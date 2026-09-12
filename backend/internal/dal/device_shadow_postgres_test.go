// 文件用途：设备影子 ACK 闭环的运行期证据（ROADMAP P0.2）。
// 覆盖：下发转 sent（而非直接 delivered）、ACK 才转 delivered 并写 ack_at、
// 终态行重复 ACK 被拒、退避重试次数耗尽转 failed、TTL 到期转 expired。
// 说明：需要真实 PostgreSQL。缺 DSN 或缺表一律 Skip，不得把 Skip 当作通过。
package dal

import (
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"github.com/go-basic/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openShadowPostgres 打开数据库并校验影子表存在。
func openShadowPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; device shadow tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	global.DB = db
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='device_shadow_messages')").Scan(&exists).Error; err != nil {
		t.Fatalf("probe shadow table: %v", err)
	}
	if !exists {
		t.Skip("device_shadow_messages missing; apply migrations first")
	}
	return db
}

// seedShadowRow 写入一条影子消息；device_id 用自建设备 ID，避免依赖既有设备。
func seedShadowRow(t *testing.T, db *gorm.DB, deviceID string, status string, expiresAt time.Time) *model.DeviceShadowMessage {
	t.Helper()
	id := uuid.New()
	payload := `{"method":"set","params":{}}`
	now := time.Now().UTC()
	msg := &model.DeviceShadowMessage{
		ID:          id,
		DeviceID:    deviceID,
		MessageType: "command",
		Payload:     &payload,
		TTLSeconds:  3600,
		Status:      status,
		CreatedAt:   &now,
		ExpiresAt:   expiresAt,
	}
	if err := CreateShadowMessage(msg); err != nil {
		t.Fatalf("create shadow message: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM device_shadow_messages WHERE id = ?", id).Error
	})
	return msg
}

func shadowStatusOf(t *testing.T, db *gorm.DB, id string) string {
	t.Helper()
	var status string
	if err := db.Raw("SELECT status FROM device_shadow_messages WHERE id = ?", id).Scan(&status).Error; err != nil {
		t.Fatalf("read status: %v", err)
	}
	return status
}

// TestShadowAckRequiresDeviceConfirmation 核心不变式：下发只能到 sent，
// 设备 ACK 才转 delivered 并写 ack_at——不得把"已下发"写成"设备已确认"。
func TestShadowAckRequiresDeviceConfirmation(t *testing.T) {
	db := openShadowPostgres(t)
	deviceID := uuid.New()
	msg := seedShadowRow(t, db, deviceID, model.ShadowStatusPending, time.Now().UTC().Add(time.Hour))

	if _, err := MarkShadowMessageSent(msg.ID); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if got := shadowStatusOf(t, db, msg.ID); got != model.ShadowStatusSent {
		t.Fatalf("after dispatch status = %s, want %s", got, model.ShadowStatusSent)
	}

	if err := AckShadowMessage(deviceID, msg.ID); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if got := shadowStatusOf(t, db, msg.ID); got != model.ShadowStatusDelivered {
		t.Fatalf("after ack status = %s, want %s", got, model.ShadowStatusDelivered)
	}
	var ackAt *time.Time
	if err := db.Raw("SELECT ack_at FROM device_shadow_messages WHERE id = ?", msg.ID).Scan(&ackAt).Error; err != nil {
		t.Fatalf("read ack_at: %v", err)
	}
	if ackAt == nil {
		t.Fatalf("ack_at must be written on device confirmation")
	}
}

// TestShadowTerminalAckRejected 终态行不可被再次确认，
// 否则历史消息会被改写成"已确认"，等于伪造送达。
func TestShadowTerminalAckRejected(t *testing.T) {
	db := openShadowPostgres(t)
	deviceID := uuid.New()
	msg := seedShadowRow(t, db, deviceID, model.ShadowStatusPending, time.Now().UTC().Add(time.Hour))

	if _, err := MarkShadowMessageSent(msg.ID); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if err := AckShadowMessage(deviceID, msg.ID); err != nil {
		t.Fatalf("first ack: %v", err)
	}
	if err := AckShadowMessage(deviceID, msg.ID); err == nil {
		t.Fatalf("acking a terminal shadow message must be rejected")
	}
}

// TestShadowRetryExhaustionMarksFailed 达到最大尝试次数后必须转 failed，
// 不能无限重试，也不能停在 sent 假装还在投。
func TestShadowRetryExhaustionMarksFailed(t *testing.T) {
	db := openShadowPostgres(t)
	deviceID := uuid.New()
	msg := seedShadowRow(t, db, deviceID, model.ShadowStatusPending, time.Now().UTC().Add(time.Hour))

	if _, err := MarkShadowMessageSent(msg.ID); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	// 直接把退避时间推到过去并让 attempts 达到上限，触发收口分支。
	if err := db.Exec(
		"UPDATE device_shadow_messages SET attempts = ?, next_attempt_at = ? WHERE id = ?",
		model.ShadowMaxAttempts, time.Now().UTC().Add(-time.Minute), msg.ID).Error; err != nil {
		t.Fatalf("force retry exhaustion: %v", err)
	}

	if _, _, _, err := ExpireAndRetryShadowMessages(); err != nil {
		t.Fatalf("expire and retry: %v", err)
	}
	if got := shadowStatusOf(t, db, msg.ID); got != model.ShadowStatusFailed {
		t.Fatalf("exhausted shadow status = %s, want %s", got, model.ShadowStatusFailed)
	}
}

// TestShadowTTLIsHardStop TTL 到期必须转 expired，且不得因重试而延长。
func TestShadowTTLIsHardStop(t *testing.T) {
	db := openShadowPostgres(t)
	deviceID := uuid.New()
	msg := seedShadowRow(t, db, deviceID, model.ShadowStatusPending, time.Now().UTC().Add(-time.Minute))

	if _, _, _, err := ExpireAndRetryShadowMessages(); err != nil {
		t.Fatalf("expire and retry: %v", err)
	}
	if got := shadowStatusOf(t, db, msg.ID); got != model.ShadowStatusExpired {
		t.Fatalf("expired shadow status = %s, want %s", got, model.ShadowStatusExpired)
	}
}
