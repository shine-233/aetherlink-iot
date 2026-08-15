// 文件用途：验证 backend 向 MQTT broker 发布设备会话撤销通知的合同。
// 核心逻辑：锁定 v1 命令/ACK、awaiting_ack 超时重投、required broker 聚合和快速 ACK 竞态。
// 关键注意事项：该合同跨越 backend 与 mqtt-broker 两个 Go module，字符串不可单边修改。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func TestRequestMQTTDeviceSessionTerminationPublishesContract(t *testing.T) {
	previousPublish := publishMQTTSessionRevocation
	t.Cleanup(func() { publishMQTTSessionRevocation = previousPublish })

	var gotChannel string
	var gotDeviceID string
	publishMQTTSessionRevocation = func(_ context.Context, channel string, deviceID string) (int64, error) {
		gotChannel = channel
		gotDeviceID = deviceID
		return 1, nil
	}

	if err := requestMQTTDeviceSessionTermination(context.Background(), " device-1 "); err != nil {
		t.Fatalf("requestMQTTDeviceSessionTermination() error = %v", err)
	}
	if gotChannel != mqttDeviceSessionRevocationChannel {
		t.Fatalf("channel = %q, want %q", gotChannel, mqttDeviceSessionRevocationChannel)
	}
	if gotDeviceID != "device-1" {
		t.Fatalf("device ID = %q, want device-1", gotDeviceID)
	}
}

func TestRequestMQTTDeviceSessionTerminationPropagatesPublishFailure(t *testing.T) {
	previousPublish := publishMQTTSessionRevocation
	t.Cleanup(func() { publishMQTTSessionRevocation = previousPublish })
	wantErr := errors.New("redis unavailable")
	publishMQTTSessionRevocation = func(context.Context, string, string) (int64, error) { return 0, wantErr }

	if err := requestMQTTDeviceSessionTermination(context.Background(), "device-1"); !errors.Is(err, wantErr) {
		t.Fatalf("requestMQTTDeviceSessionTermination() error = %v, want %v", err, wantErr)
	}
}

func TestRequestMQTTDeviceSessionTerminationRejectsMissingBrokerSubscriber(t *testing.T) {
	previousPublish := publishMQTTSessionRevocation
	t.Cleanup(func() { publishMQTTSessionRevocation = previousPublish })
	publishMQTTSessionRevocation = func(context.Context, string, string) (int64, error) { return 0, nil }

	if err := requestMQTTDeviceSessionTermination(context.Background(), "device-1"); err == nil {
		t.Fatal("requestMQTTDeviceSessionTermination() error = nil, want missing subscriber error")
	}
}

func TestRDIPhysicalUnbindPersistsInactiveStateBeforePublishingSessionRevocation(t *testing.T) {
	db := setupMQTTSessionRevocationTestDB(t)
	ownerUserID := "owner-1"
	additionalInfo := `{"rdi_share_tokens":[{"token_hash":"secret"}]}`
	device := &model.Device{
		ID:             "device-1",
		Voucher:        "voucher-1",
		TenantID:       "tenant-1",
		OwnerUserID:    &ownerUserID,
		IsEnabled:      "enabled",
		ActivateFlag:   "active",
		DeviceNumber:   "rdi-001",
		IsOnline:       1,
		AdditionalInfo: &additionalInfo,
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}
	if err := db.Create(&model.RGroupDevice{GroupID: "group-1", DeviceID: device.ID, TenantID: device.TenantID}).Error; err != nil {
		t.Fatalf("seed group relation: %v", err)
	}

	previousPublish := publishMQTTSessionRevocation
	t.Cleanup(func() { publishMQTTSessionRevocation = previousPublish })
	publishMQTTSessionRevocation = func(_ context.Context, channel string, payload string) (int64, error) {
		var event mqttSessionRevocationMessage
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			t.Fatalf("decode durable revocation payload: %v", err)
		}
		if channel != mqttDeviceSessionRevocationChannel || event.DeviceID != device.ID || event.EventID == "" || event.RevokedAt.IsZero() {
			t.Fatalf("publish contract = (%q, %#v)", channel, event)
		}
		var persisted model.Device
		if err := db.Where("id = ?", device.ID).First(&persisted).Error; err != nil {
			t.Fatalf("read persisted device before publish: %v", err)
		}
		if persisted.TenantID != "" || persisted.OwnerUserID != nil || persisted.ActivateFlag != "inactive" || persisted.IsEnabled != "disabled" || persisted.IsOnline != 0 {
			t.Fatalf("device was not fully unbound before publish: %#v", persisted)
		}
		return 1, nil
	}

	if err := (&RDI{}).HandlePhysicalUnbindEvent(device, &model.EventInfo{Method: rdiSW3ShortPressEvent}); err != nil {
		t.Fatalf("HandlePhysicalUnbindEvent() error = %v", err)
	}
	var relationCount int64
	if err := db.Model(&model.RGroupDevice{}).Where("device_id = ?", device.ID).Count(&relationCount).Error; err != nil {
		t.Fatalf("count group relations: %v", err)
	}
	if relationCount != 0 {
		t.Fatalf("group relation count = %d, want 0", relationCount)
	}
	var outbox mqttSessionRevocationOutbox
	if err := db.Where("device_id = ?", device.ID).First(&outbox).Error; err != nil {
		t.Fatalf("read durable revocation outbox: %v", err)
	}
	if outbox.Status != mqttSessionRevocationStatusAwaitingAck || outbox.SubscriberCount == nil || *outbox.SubscriberCount != 1 || outbox.NextRetryAt == nil {
		t.Fatalf("outbox publication state = %#v", outbox)
	}
}

func TestMQTTSessionRevocationPublishesOldCutoffAfterDeviceReactivation(t *testing.T) {
	db := setupMQTTSessionRevocationTestDB(t)
	reactivatedAt := time.Now().UTC()
	device := &model.Device{
		ID:           "device-reactivated",
		Voucher:      "voucher-reactivated",
		TenantID:     "tenant-new-owner",
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		DeviceNumber: "rdi-reactivated",
		UpdateAt:     &reactivatedAt,
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("seed reactivated device: %v", err)
	}

	revokedAt := reactivatedAt.Add(-time.Minute)
	event := newMQTTSessionRevocationOutbox(device.ID, revokedAt)
	claimToken := "reactivated-claim"
	event.Status = mqttSessionRevocationStatusProcessing
	event.ClaimToken = &claimToken
	event.Attempts = 2
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("seed delayed revocation event: %v", err)
	}

	previousPublish := publishMQTTSessionRevocation
	t.Cleanup(func() { publishMQTTSessionRevocation = previousPublish })
	var published mqttSessionRevocationMessage
	publishMQTTSessionRevocation = func(_ context.Context, channel string, payload string) (int64, error) {
		if channel != mqttDeviceSessionRevocationChannel {
			t.Fatalf("channel = %q, want %q", channel, mqttDeviceSessionRevocationChannel)
		}
		if err := json.Unmarshal([]byte(payload), &published); err != nil {
			t.Fatalf("decode delayed revocation payload: %v", err)
		}
		return 1, nil
	}

	outcome, err := deliverClaimedMQTTSessionRevocationOutboxEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("deliver delayed revocation: %v", err)
	}
	if outcome != mqttSessionRevocationStatusAwaitingAck {
		t.Fatalf("outcome = %q, want awaiting_ack", outcome)
	}
	if published.DeviceID != device.ID || !published.RevokedAt.Equal(revokedAt) {
		t.Fatalf("published event = %#v, want old generation cutoff %s", published, revokedAt)
	}

	var persisted mqttSessionRevocationOutbox
	if err := db.First(&persisted, "id = ?", event.ID).Error; err != nil {
		t.Fatalf("read published delayed revocation: %v", err)
	}
	if persisted.Status != mqttSessionRevocationStatusAwaitingAck {
		t.Fatalf("persisted status = %q, want awaiting_ack", persisted.Status)
	}
	if persisted.ClaimToken != nil {
		t.Fatalf("published claim token = %q, want cleared", *persisted.ClaimToken)
	}
}

func TestMQTTSessionRevocationStaleClaimCannotFinishNewLease(t *testing.T) {
	db := setupMQTTSessionRevocationTestDB(t)
	oldToken := "old-claim"
	newToken := "new-claim"
	event := newMQTTSessionRevocationOutbox("device-fenced", time.Now().UTC())
	event.Status = mqttSessionRevocationStatusProcessing
	event.ClaimToken = &oldToken
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("seed fenced revocation event: %v", err)
	}
	if err := db.Model(&mqttSessionRevocationOutbox{}).
		Where("id = ?", event.ID).
		Update("claim_token", newToken).Error; err != nil {
		t.Fatalf("replace claim owner: %v", err)
	}

	_, err := markMQTTSessionRevocationAwaitingAck(
		context.Background(),
		event,
		1,
	)
	if err == nil || !strings.Contains(err.Error(), "lost claim ownership") {
		t.Fatalf("stale claim completion error = %v, want ownership loss", err)
	}

	var persisted mqttSessionRevocationOutbox
	if err := db.First(&persisted, "id = ?", event.ID).Error; err != nil {
		t.Fatalf("read fenced revocation event: %v", err)
	}
	if persisted.Status != mqttSessionRevocationStatusProcessing || persisted.ClaimToken == nil || *persisted.ClaimToken != newToken {
		t.Fatalf("new claim was overwritten: %#v", persisted)
	}
}

func TestMQTTSessionRevocationSnapshotsRequiredBrokerIDs(t *testing.T) {
	key := "mqtt_session_revocations.required_broker_ids"
	previousValue := viper.Get(key)
	previouslySet := viper.IsSet(key)
	viper.Set(key, []string{" broker-b ", "broker-a", "broker-a"})
	t.Cleanup(func() {
		if previouslySet {
			viper.Set(key, previousValue)
		} else {
			viper.Set(key, []string{})
		}
	})

	event := newMQTTSessionRevocationOutbox("device-required-brokers", time.Now().UTC())
	if event.RequiredBrokerIDs != `["broker-a","broker-b"]` {
		t.Fatalf("required broker snapshot = %s", event.RequiredBrokerIDs)
	}
}

func TestMQTTSessionRevocationRejectsInvalidRequiredBrokerID(t *testing.T) {
	key := "mqtt_session_revocations.required_broker_ids"
	previous := viper.Get(key)
	viper.Set(key, []string{"broker/invalid"})
	t.Cleanup(func() { viper.Set(key, previous) })

	if got := snapshotMQTTSessionRevocationRequiredBrokerIDs(); got != mqttSessionRevocationPolicyBackfillMarkerJSON {
		t.Fatalf("invalid broker policy snapshot = %s, want fail-closed marker", got)
	}
	if _, err := decodeMQTTSessionRevocationRequiredBrokerIDs(mqttSessionRevocationPolicyBackfillMarkerJSON); err == nil {
		t.Fatal("migration policy marker decoded without an error")
	}
}

func TestMQTTSessionRevocationFastAckWinsAwaitingAckTransition(t *testing.T) {
	db := setupMQTTSessionRevocationTestDB(t)
	event := newMQTTSessionRevocationOutbox("device-fast-ack", time.Now().UTC())
	claimToken := "fast-ack-claim"
	event.Status = mqttSessionRevocationStatusProcessing
	event.ClaimToken = &claimToken
	event.Attempts = 1
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("seed fast-ack event: %v", err)
	}

	previousPublish := publishMQTTSessionRevocation
	t.Cleanup(func() { publishMQTTSessionRevocation = previousPublish })
	publishMQTTSessionRevocation = func(ctx context.Context, _ string, payload string) (int64, error) {
		var request mqttSessionRevocationMessage
		if err := json.Unmarshal([]byte(payload), &request); err != nil {
			t.Fatalf("decode fast-ack request: %v", err)
		}
		ackPayload := mqttSessionRevocationAckPayload(t, request, "broker-fast", 1)
		if err := AcknowledgeMQTTSessionRevocation(ctx, ackPayload); err != nil {
			t.Fatalf("persist fast acknowledgement: %v", err)
		}
		return 7, nil
	}

	outcome, err := deliverClaimedMQTTSessionRevocationOutboxEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("deliver fast-ack event: %v", err)
	}
	if outcome != mqttSessionRevocationStatusAcknowledged {
		t.Fatalf("outcome = %q, want acknowledged", outcome)
	}
	var persisted mqttSessionRevocationOutbox
	if err := db.First(&persisted, "id = ?", event.ID).Error; err != nil {
		t.Fatalf("read fast-ack event: %v", err)
	}
	if persisted.Status != mqttSessionRevocationStatusAcknowledged || persisted.AcknowledgedAt == nil || persisted.AcknowledgedBrokerCount != 1 || persisted.SubscriberCount == nil || *persisted.SubscriberCount != 7 {
		t.Fatalf("fast-ack state = %#v", persisted)
	}
}

func TestMQTTSessionRevocationRequiredBrokerAcksAreIdempotent(t *testing.T) {
	db := setupMQTTSessionRevocationTestDB(t)
	event := newMQTTSessionRevocationOutbox("device-two-brokers", time.Now().UTC())
	event.Status = mqttSessionRevocationStatusAwaitingAck
	event.RequiredBrokerIDs = `["broker-a","broker-b"]`
	deadline := time.Now().UTC().Add(time.Minute)
	event.NextRetryAt = &deadline
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("seed required-broker event: %v", err)
	}
	request := mqttSessionRevocationMessage{Version: 1, EventID: event.ID, DeviceID: event.DeviceID, RevokedAt: event.RevokedAt}
	ackA := mqttSessionRevocationAckPayload(t, request, "broker-a", 1)
	if err := AcknowledgeMQTTSessionRevocation(context.Background(), ackA); err != nil {
		t.Fatalf("ack broker-a: %v", err)
	}
	if err := AcknowledgeMQTTSessionRevocation(context.Background(), ackA); err != nil {
		t.Fatalf("duplicate ack broker-a: %v", err)
	}
	var intermediate mqttSessionRevocationOutbox
	if err := db.First(&intermediate, "id = ?", event.ID).Error; err != nil {
		t.Fatalf("read intermediate event: %v", err)
	}
	if intermediate.Status != mqttSessionRevocationStatusAwaitingAck || intermediate.AcknowledgedBrokerCount != 1 {
		t.Fatalf("intermediate required-broker state = %#v", intermediate)
	}
	if err := AcknowledgeMQTTSessionRevocation(context.Background(), mqttSessionRevocationAckPayload(t, request, "broker-b", 0)); err != nil {
		t.Fatalf("ack broker-b: %v", err)
	}
	var persisted mqttSessionRevocationOutbox
	if err := db.First(&persisted, "id = ?", event.ID).Error; err != nil {
		t.Fatalf("read acknowledged event: %v", err)
	}
	if persisted.Status != mqttSessionRevocationStatusAcknowledged || persisted.AcknowledgedBrokerCount != 2 || persisted.AcknowledgedAt == nil {
		t.Fatalf("required-broker terminal state = %#v", persisted)
	}
	var ackCount int64
	if err := db.Model(&mqttSessionRevocationAck{}).Where("event_id = ?", event.ID).Count(&ackCount).Error; err != nil {
		t.Fatalf("count persisted acknowledgements: %v", err)
	}
	if ackCount != 2 {
		t.Fatalf("ack row count = %d, want 2", ackCount)
	}
}

func TestMQTTSessionRevocationLostAckTimesOutAndRepublishes(t *testing.T) {
	db := setupMQTTSessionRevocationTestDB(t)
	event := newMQTTSessionRevocationOutbox("device-lost-ack", time.Now().UTC())
	event.Status = mqttSessionRevocationStatusAwaitingAck
	event.Attempts = 1
	expired := time.Now().UTC().Add(-time.Second)
	event.NextRetryAt = &expired
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("seed lost-ack event: %v", err)
	}

	claimed, err := claimMQTTSessionRevocationOutbox(context.Background(), 1, event.ID)
	if err != nil {
		t.Fatalf("reclaim expired awaiting-ack event: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Status != mqttSessionRevocationStatusProcessing || claimed[0].Attempts != 2 || claimed[0].ClaimToken == nil {
		t.Fatalf("reclaimed events = %#v", claimed)
	}
	previousPublish := publishMQTTSessionRevocation
	t.Cleanup(func() { publishMQTTSessionRevocation = previousPublish })
	publishCalls := 0
	publishMQTTSessionRevocation = func(context.Context, string, string) (int64, error) {
		publishCalls++
		return 1, nil
	}
	outcome, err := deliverClaimedMQTTSessionRevocationOutboxEvent(context.Background(), &claimed[0])
	if err != nil {
		t.Fatalf("republish expired awaiting-ack event: %v", err)
	}
	if publishCalls != 1 || outcome != mqttSessionRevocationStatusAwaitingAck {
		t.Fatalf("republish result = calls:%d outcome:%q", publishCalls, outcome)
	}
}

func TestMQTTSessionRevocationMissingLeaseIsReclaimable(t *testing.T) {
	db := setupMQTTSessionRevocationTestDB(t)
	for _, status := range []string{
		mqttSessionRevocationStatusProcessing,
		mqttSessionRevocationStatusAwaitingAck,
	} {
		event := newMQTTSessionRevocationOutbox("device-missing-lease-"+status, time.Now().UTC())
		event.Status = status
		event.Attempts = 2
		event.NextRetryAt = nil
		if status == mqttSessionRevocationStatusProcessing {
			staleToken := "stale-missing-lease"
			event.ClaimToken = &staleToken
		}
		if err := db.Create(event).Error; err != nil {
			t.Fatalf("seed %s event without lease: %v", status, err)
		}

		claimed, err := claimMQTTSessionRevocationOutbox(context.Background(), 1, event.ID)
		if err != nil {
			t.Fatalf("reclaim %s event without lease: %v", status, err)
		}
		if len(claimed) != 1 ||
			claimed[0].Status != mqttSessionRevocationStatusProcessing ||
			claimed[0].Attempts != 3 ||
			claimed[0].ClaimToken == nil ||
			claimed[0].NextRetryAt == nil {
			t.Fatalf("reclaimed %s event = %#v", status, claimed)
		}
	}
}

func mqttSessionRevocationAckPayload(t *testing.T, request mqttSessionRevocationMessage, brokerID string, terminatedSessions int) string {
	t.Helper()
	payload, err := json.Marshal(mqttSessionRevocationAckMessage{
		Version:            1,
		EventID:            request.EventID,
		DeviceID:           request.DeviceID,
		RevokedAt:          request.RevokedAt,
		BrokerID:           brokerID,
		Status:             "processed",
		ProcessedAt:        time.Now().UTC(),
		TerminatedSessions: terminatedSessions,
	})
	if err != nil {
		t.Fatalf("encode acknowledgement payload: %v", err)
	}
	return string(payload)
}

func setupMQTTSessionRevocationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	oldRedis := global.REDIS
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Device{}, &model.RGroupDevice{}, &mqttSessionRevocationOutbox{}, &mqttSessionRevocationAck{}); err != nil {
		t.Fatalf("migrate mqtt session revocation tables: %v", err)
	}
	global.DB = db
	global.REDIS = nil
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		global.REDIS = oldRedis
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}
