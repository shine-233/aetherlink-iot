// 文件用途：遥测模拟器读侧切换凭证测试缓存后的行为回归（凭证哈希存储 Phase 2b）。
// 核心逻辑：锁定三处入口（ServeEchoData / GetSimulationInit / SimulationSend）的
// miss 分支错误码与文案，以及 hit 分支从真实创建路径写入的缓存中取到明文凭证。
// 关键注意事项：Redis 依赖经 dal 包级 seam 注入假实现；miss 断言使用精确业务码
// errcode.CodeNotFound 与固定英文 message（前端/E2E 可依赖的稳定契约）。

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const simulationVoucherCacheMissMessage = "device credential test cache expired or absent; rotate the voucher to regenerate test credentials"

func setupTelemetrySimulationTestDB(t *testing.T) {
	t.Helper()

	oldDB := global.DB
	dbName := fmt.Sprintf("%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Device{}, &model.Group{}, &model.RGroupDevice{}); err != nil {
		t.Fatalf("migrate telemetry simulation test tables: %v", err)
	}
	// 凭证哈希存储 Phase 1：voucher_hash 列由 50.sql 迁移负责，gen 模型无该字段，
	// 内存库手工补列以匹配生产 schema（CreateDevice 收口写入需要）。
	if err := db.Exec(`ALTER TABLE devices ADD COLUMN voucher_hash varchar(64)`).Error; err != nil {
		t.Fatalf("add voucher_hash column: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
}

func seedSimulationDevice(t *testing.T, id, voucher string) {
	t.Helper()

	if err := dal.CreateDevice(&model.Device{
		ID:           id,
		Voucher:      voucher,
		TenantID:     "tenant-simulation",
		DeviceNumber: id,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
	}); err != nil {
		t.Fatalf("create simulation device %s: %v", id, err)
	}
}

func simulationAdminClaims() *utils.UserClaims {
	return &utils.UserClaims{Authority: constant.SYS_ADMIN, TenantID: "tenant-simulation"}
}

func assertSimulationCacheMissError(t *testing.T, err error) {
	t.Helper()

	var ec *errcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("error = %v, want *errcode.Error", err)
	}
	if ec.Code != errcode.CodeNotFound {
		t.Fatalf("error code = %d, want %d (CodeNotFound)", ec.Code, errcode.CodeNotFound)
	}
	if ec.CustomMsg != simulationVoucherCacheMissMessage {
		t.Fatalf("error message = %q, want %q", ec.CustomMsg, simulationVoucherCacheMissMessage)
	}
}

// TestSimulationEndpointsFailClosedWhenTestCacheMissing 锁定 miss 分支：
// 设备存在但测试缓存未命中（未创建过缓存 / 过期 / Redis 故障）时，
// 三处模拟器入口都必须返回 CodeNotFound + 固定轮换提示，而不是回退读明文列。
func TestSimulationEndpointsFailClosedWhenTestCacheMissing(t *testing.T) {
	setupTelemetrySimulationTestDB(t)
	// 不注入 seam：默认适配器在无 Redis 的单测环境按 fail-closed 归一为 miss。
	seedSimulationDevice(t, "sim-miss-device", `{"username":"db-only-user","password":"pw"}`)

	telemetry := &TelemetryData{}

	echoResp, err := telemetry.ServeEchoData(&model.ServeEchoDataReq{DeviceId: "sim-miss-device"}, "127.0.0.1", simulationAdminClaims())
	if err == nil {
		t.Fatalf("ServeEchoData must fail closed on cache miss, got resp=%v", echoResp)
	}
	assertSimulationCacheMissError(t, err)

	initResp, err := telemetry.GetSimulationInit("sim-miss-device", simulationAdminClaims())
	if err == nil {
		t.Fatalf("GetSimulationInit must fail closed on cache miss, got resp=%v", initResp)
	}
	assertSimulationCacheMissError(t, err)

	err = telemetry.SimulationSend(&model.SimulationSendReq{
		DeviceID: "sim-miss-device",
		Data:     `{"temperature":25.5}`,
	}, simulationAdminClaims())
	if err == nil {
		t.Fatalf("SimulationSend must fail closed on cache miss")
	}
	assertSimulationCacheMissError(t, err)
}

// recordingSimulationCacheStore 是 dal.DeviceCredentialCacheStore 的假实现：
// 记录真实创建路径写入的逐设备缓存，供 hit 分支断言。
type recordingSimulationCacheStore struct {
	values map[string]string
}

func (s *recordingSimulationCacheStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	s.values[key] = value
	return nil
}

func (s *recordingSimulationCacheStore) Get(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", dal.ErrCredentialCacheMiss
	}
	return value, nil
}

func useRecordingSimulationCacheStore(t *testing.T) *recordingSimulationCacheStore {
	t.Helper()

	fake := &recordingSimulationCacheStore{values: map[string]string{}}
	old := dal.DeviceCredentialCacheStore
	dal.DeviceCredentialCacheStore = fake
	t.Cleanup(func() { dal.DeviceCredentialCacheStore = old })
	return fake
}

// TestSimulationReadsCredentialsFromTestCacheHit 锁定 hit 分支：
// 走真实创建路径（dal.CreateDevice → hash 收口点写缓存）后，模拟器能取到明文凭证，
// 且不再读取 devices.voucher 列（该列停写明文后为空串）。
func TestSimulationReadsCredentialsFromTestCacheHit(t *testing.T) {
	setupTelemetrySimulationTestDB(t)
	useRecordingSimulationCacheStore(t)
	voucher := `{"username":"sim-cache-user","password":"sim-cache-pass"}`
	seedSimulationDevice(t, "sim-hit-device", voucher)

	storedPlaintext := ""
	if err := global.DB.Raw(`SELECT voucher FROM devices WHERE id = ?`, "sim-hit-device").Scan(&storedPlaintext).Error; err != nil {
		t.Fatalf("read voucher column: %v", err)
	}
	if storedPlaintext != "" {
		t.Fatalf("devices.voucher must stop persisting plaintext, got %q", storedPlaintext)
	}

	oldBroker := viper.GetString("mqtt.broker")
	viper.Set("mqtt.broker", "127.0.0.1:1883")
	t.Cleanup(func() { viper.Set("mqtt.broker", oldBroker) })

	telemetry := &TelemetryData{}

	initResp, err := telemetry.GetSimulationInit("sim-hit-device", simulationAdminClaims())
	if err != nil {
		t.Fatalf("GetSimulationInit on cache hit: %v", err)
	}
	if initResp == nil || initResp.Username != "sim-cache-user" || initResp.Password != "sim-cache-pass" {
		t.Fatalf("GetSimulationInit credentials = %+v, want cached voucher credentials", initResp)
	}
	if initResp.Port != 1883 {
		t.Fatalf("GetSimulationInit port = %d, want 1883", initResp.Port)
	}

	echoResp, err := telemetry.ServeEchoData(&model.ServeEchoDataReq{DeviceId: "sim-hit-device"}, "127.0.0.1", simulationAdminClaims())
	if err != nil {
		t.Fatalf("ServeEchoData on cache hit: %v", err)
	}
	command, ok := echoResp.(string)
	if !ok || !strings.Contains(command, "sim-cache-user") {
		t.Fatalf("ServeEchoData command = %#v, want mosquitto command embedding cached voucher username", echoResp)
	}
}
