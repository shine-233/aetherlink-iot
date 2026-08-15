// 文件用途：持久化并发布设备 MQTT 会话撤销通知，供独立 broker 进程终止解绑前的在线连接。
// 核心逻辑：物理解绑事务写入 PostgreSQL outbox；后台 worker 租约式发布并在 ACK 超时后重试。
// 关键注意事项：Redis subscriber count 只用于诊断；只有满足 required broker 快照的 processed ACK 才能进入 acknowledged 终态。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"github.com/go-basic/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	mqttDeviceSessionRevocationChannel = "aetherlink:mqtt:device-session:terminate"

	mqttSessionRevocationStatusPending      = "pending"
	mqttSessionRevocationStatusProcessing   = "processing"
	mqttSessionRevocationStatusAwaitingAck  = "awaiting_ack"
	mqttSessionRevocationStatusAcknowledged = "acknowledged"

	mqttSessionRevocationClaimLease        = 30 * time.Second
	mqttSessionRevocationDefaultAckTimeout = 30 * time.Second
	mqttSessionRevocationRetryBase         = 5 * time.Second
	mqttSessionRevocationRetryMax          = 5 * time.Minute

	mqttSessionRevocationBrokerIDMaxLength        = 128
	mqttSessionRevocationPolicyBackfillMarker     = "__migration_policy_backfill_required__"
	mqttSessionRevocationPolicyBackfillMarkerJSON = `["__migration_policy_backfill_required__"]`
)

var mqttSessionRevocationBrokerIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

type mqttSessionRevocationOutbox struct {
	ID                      string     `gorm:"column:id;primaryKey"`
	DeviceID                string     `gorm:"column:device_id;not null"`
	RevokedAt               time.Time  `gorm:"column:revoked_at;not null"`
	Status                  string     `gorm:"column:status;not null"`
	ClaimToken              *string    `gorm:"column:claim_token"`
	Attempts                int        `gorm:"column:attempts;not null"`
	LastError               *string    `gorm:"column:last_error"`
	NextRetryAt             *time.Time `gorm:"column:next_retry_at"`
	PublishedAt             *time.Time `gorm:"column:published_at"`
	SubscriberCount         *int64     `gorm:"column:subscriber_count"`
	RequiredBrokerIDs       string     `gorm:"column:required_broker_ids;not null"`
	AcknowledgedAt          *time.Time `gorm:"column:acknowledged_at"`
	AcknowledgedBrokerCount int        `gorm:"column:acknowledged_broker_count;not null"`
	CreatedAt               time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt               time.Time  `gorm:"column:updated_at;not null"`
}

func (mqttSessionRevocationOutbox) TableName() string {
	return "mqtt_session_revocation_outbox"
}

type mqttSessionRevocationMessage struct {
	Version   int       `json:"version"`
	EventID   string    `json:"event_id"`
	DeviceID  string    `json:"device_id"`
	RevokedAt time.Time `json:"revoked_at"`
}

type mqttSessionRevocationAckMessage struct {
	Version            int       `json:"version"`
	EventID            string    `json:"event_id"`
	DeviceID           string    `json:"device_id"`
	RevokedAt          time.Time `json:"revoked_at"`
	BrokerID           string    `json:"broker_id"`
	Status             string    `json:"status"`
	ProcessedAt        time.Time `json:"processed_at"`
	TerminatedSessions int       `json:"terminated_sessions"`
}

type mqttSessionRevocationAck struct {
	EventID            string    `gorm:"column:event_id;primaryKey"`
	BrokerID           string    `gorm:"column:broker_id;primaryKey"`
	DeviceID           string    `gorm:"column:device_id;not null"`
	RevokedAt          time.Time `gorm:"column:revoked_at;not null"`
	ProcessedAt        time.Time `gorm:"column:processed_at;not null"`
	TerminatedSessions int       `gorm:"column:terminated_sessions;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;not null"`
}

func (mqttSessionRevocationAck) TableName() string {
	return "mqtt_session_revocation_acks"
}

// MQTTSessionRevocationOutboxDrainResult reports worker progress. Published is
// the number of commands accepted by Redis and moved to awaiting_ack; only a
// persisted broker acknowledgement can move an event to acknowledged.
type MQTTSessionRevocationOutboxDrainResult struct {
	Claimed   int
	Published int
	Retried   int
}

var publishMQTTSessionRevocation = func(ctx context.Context, channel string, payload string) (int64, error) {
	if global.REDIS == nil {
		return 0, fmt.Errorf("redis is not initialized")
	}
	return global.REDIS.Publish(ctx, channel, payload).Result()
}

// requestMQTTDeviceSessionTermination keeps the legacy plain-device-ID contract
// for callers outside the durable SW3 path. New outbox deliveries use the
// versioned JSON envelope below so retrying an old unbind cannot target a later
// authentication generation.
func requestMQTTDeviceSessionTermination(ctx context.Context, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return fmt.Errorf("mqtt session revocation device id is required")
	}
	subscriberCount, err := publishMQTTSessionRevocation(ctx, mqttDeviceSessionRevocationChannel, deviceID)
	if err != nil {
		return fmt.Errorf("publish mqtt session revocation for device %s: %w", deviceID, err)
	}
	if subscriberCount == 0 {
		return fmt.Errorf("publish mqtt session revocation for device %s: no broker subscriber", deviceID)
	}
	return nil
}

func newMQTTSessionRevocationOutbox(deviceID string, revokedAt time.Time) *mqttSessionRevocationOutbox {
	now := time.Now().UTC()
	return &mqttSessionRevocationOutbox{
		ID:                uuid.New(),
		DeviceID:          strings.TrimSpace(deviceID),
		RevokedAt:         revokedAt.UTC(),
		Status:            mqttSessionRevocationStatusPending,
		RequiredBrokerIDs: snapshotMQTTSessionRevocationRequiredBrokerIDs(),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func snapshotMQTTSessionRevocationRequiredBrokerIDs() string {
	brokerIDs, err := configuredMQTTSessionRevocationRequiredBrokerIDs()
	if err != nil {
		// Fail closed if an event is somehow created before application startup
		// rejects the invalid configuration.
		return mqttSessionRevocationPolicyBackfillMarkerJSON
	}
	encoded, err := json.Marshal(brokerIDs)
	if err != nil {
		return mqttSessionRevocationPolicyBackfillMarkerJSON
	}
	return string(encoded)
}

func configuredMQTTSessionRevocationRequiredBrokerIDs() ([]string, error) {
	seen := make(map[string]struct{})
	brokerIDs := make([]string, 0)
	for _, rawBrokerID := range viper.GetStringSlice("mqtt_session_revocations.required_broker_ids") {
		brokerID, err := normalizeMQTTSessionRevocationBrokerID(rawBrokerID)
		if err != nil {
			return nil, fmt.Errorf("invalid mqtt_session_revocations.required_broker_ids entry: %w", err)
		}
		if _, exists := seen[brokerID]; exists {
			continue
		}
		seen[brokerID] = struct{}{}
		brokerIDs = append(brokerIDs, brokerID)
	}
	sort.Strings(brokerIDs)
	return brokerIDs, nil
}

func normalizeMQTTSessionRevocationBrokerID(raw string) (string, error) {
	brokerID := strings.TrimSpace(raw)
	if brokerID == "" {
		return "", fmt.Errorf("broker id is required")
	}
	if brokerID == mqttSessionRevocationPolicyBackfillMarker {
		return "", fmt.Errorf("broker id is reserved for migration policy backfill")
	}
	if len(brokerID) > mqttSessionRevocationBrokerIDMaxLength {
		return "", fmt.Errorf("broker id must be at most %d characters", mqttSessionRevocationBrokerIDMaxLength)
	}
	if !mqttSessionRevocationBrokerIDPattern.MatchString(brokerID) {
		return "", fmt.Errorf("broker id may contain only letters, digits, dot, underscore, colon, and hyphen")
	}
	return brokerID, nil
}

func decodeMQTTSessionRevocationRequiredBrokerIDs(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var brokerIDs []string
	if err := json.Unmarshal([]byte(raw), &brokerIDs); err != nil {
		return nil, fmt.Errorf("decode required mqtt session revocation broker ids: %w", err)
	}
	normalized := make([]string, 0, len(brokerIDs))
	seen := make(map[string]struct{}, len(brokerIDs))
	for _, rawBrokerID := range brokerIDs {
		brokerID, err := normalizeMQTTSessionRevocationBrokerID(rawBrokerID)
		if err != nil {
			return nil, fmt.Errorf("invalid persisted mqtt session revocation broker id: %w", err)
		}
		if _, exists := seen[brokerID]; exists {
			continue
		}
		seen[brokerID] = struct{}{}
		normalized = append(normalized, brokerID)
	}
	sort.Strings(normalized)
	return normalized, nil
}

// PrepareMQTTSessionRevocationOutboxForWorker validates the configured broker
// policy and snapshots it into legacy rows tagged by migration 33 before the
// worker performs its first drain. Other delivery paths fail closed while the
// marker remains because SQL cannot infer a deployment's broker roster safely.
func PrepareMQTTSessionRevocationOutboxForWorker(ctx context.Context) (int64, error) {
	if global.DB == nil {
		return 0, fmt.Errorf("database is not initialized for mqtt session revocation policy backfill")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	brokerIDs, err := configuredMQTTSessionRevocationRequiredBrokerIDs()
	if err != nil {
		return 0, err
	}
	encoded, err := json.Marshal(brokerIDs)
	if err != nil {
		return 0, fmt.Errorf("encode mqtt session revocation broker policy: %w", err)
	}
	now := time.Now().UTC()
	result := global.DB.WithContext(ctx).
		Model(&mqttSessionRevocationOutbox{}).
		Where("CAST(required_broker_ids AS TEXT) = ?", mqttSessionRevocationPolicyBackfillMarkerJSON).
		Updates(map[string]interface{}{
			"required_broker_ids": string(encoded),
			"updated_at":          now,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("backfill mqtt session revocation broker policy: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// outboxScopedDB returns a session with a fresh statement so a handle previously bound
// to another model (e.g. tx.Device.UnderlyingDB()) resolves the outbox table from its own
// TableName() rather than inheriting the caller's Statement.Schema. NewDB keeps the same
// (transaction) ConnPool, so this stays inside the caller's transaction.
func outboxScopedDB(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{NewDB: true})
}

func createMQTTSessionRevocationOutboxWithDB(db *gorm.DB, event *mqttSessionRevocationOutbox) error {
	if db == nil {
		return fmt.Errorf("database is not initialized for mqtt session revocation outbox")
	}
	if event == nil || strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.DeviceID) == "" {
		return fmt.Errorf("invalid mqtt session revocation outbox event")
	}
	// The caller hands us tx.Device.UnderlyingDB(), whose Statement.Schema is already
	// parsed as the Device model. A NewDB session drops that stale schema (re-parsing the
	// outbox model from the value) while keeping the transaction's ConnPool, so the INSERT
	// targets mqtt_session_revocation_outbox instead of devices.
	return outboxScopedDB(db).Create(event).Error
}

func findActionableMQTTSessionRevocationOutboxWithDB(db *gorm.DB, deviceID string) (*mqttSessionRevocationOutbox, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized for mqtt session revocation outbox")
	}
	var event mqttSessionRevocationOutbox
	err := outboxScopedDB(db).Where(
		"device_id = ? AND status IN ?",
		strings.TrimSpace(deviceID),
		[]string{
			mqttSessionRevocationStatusPending,
			mqttSessionRevocationStatusProcessing,
			mqttSessionRevocationStatusAwaitingAck,
		},
	).Order("revoked_at DESC").First(&event).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func deliverMQTTSessionRevocationOutboxEvent(ctx context.Context, eventID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	events, err := claimMQTTSessionRevocationOutbox(ctx, 1, strings.TrimSpace(eventID))
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	_, err = deliverClaimedMQTTSessionRevocationOutboxEvent(ctx, &events[0])
	return err
}

// DrainMQTTSessionRevocationOutboxForWorker claims due rows with SKIP LOCKED,
// so multiple backend instances can retry without concurrently publishing the
// same lease. A crash after Redis Publish but before the database update can
// still cause at-least-once delivery; the broker's revocation-generation cutoff
// makes that duplicate safe.
func DrainMQTTSessionRevocationOutboxForWorker(ctx context.Context, limit int) (*MQTTSessionRevocationOutboxDrainResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	events, err := claimMQTTSessionRevocationOutbox(ctx, limit, "")
	if err != nil {
		return nil, err
	}
	result := &MQTTSessionRevocationOutboxDrainResult{Claimed: len(events)}
	for i := range events {
		outcome, deliveryErr := deliverClaimedMQTTSessionRevocationOutboxEvent(ctx, &events[i])
		switch outcome {
		case mqttSessionRevocationStatusAwaitingAck, mqttSessionRevocationStatusAcknowledged:
			result.Published++
		default:
			result.Retried++
		}
		if deliveryErr != nil && ctx != nil && ctx.Err() != nil {
			return result, ctx.Err()
		}
	}
	return result, nil
}

func claimMQTTSessionRevocationOutbox(ctx context.Context, limit int, eventID string) ([]mqttSessionRevocationOutbox, error) {
	if global.DB == nil {
		return nil, fmt.Errorf("database is not initialized for mqtt session revocation outbox")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	leaseUntil := now.Add(mqttSessionRevocationClaimLease)
	claimed := make([]mqttSessionRevocationOutbox, 0, limit)
	err := global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(
				`((status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?))
					OR (status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?))
					OR (status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)))`,
				mqttSessionRevocationStatusPending,
				now,
				mqttSessionRevocationStatusProcessing,
				now,
				mqttSessionRevocationStatusAwaitingAck,
				now,
			)
		if eventID != "" {
			query = query.Where("id = ?", eventID)
		}
		if err := query.Order("created_at ASC").Limit(limit).Find(&claimed).Error; err != nil {
			return err
		}
		for i := range claimed {
			claimToken := uuid.New()
			if err := tx.Model(&mqttSessionRevocationOutbox{}).
				Where("id = ?", claimed[i].ID).
				Updates(map[string]interface{}{
					"status":        mqttSessionRevocationStatusProcessing,
					"claim_token":   claimToken,
					"attempts":      gorm.Expr("attempts + 1"),
					"next_retry_at": leaseUntil,
					"updated_at":    now,
				}).Error; err != nil {
				return err
			}
			claimed[i].Status = mqttSessionRevocationStatusProcessing
			claimed[i].ClaimToken = &claimToken
			claimed[i].Attempts++
			claimed[i].NextRetryAt = &leaseUntil
		}
		return nil
	})
	return claimed, err
}

func deliverClaimedMQTTSessionRevocationOutboxEvent(ctx context.Context, event *mqttSessionRevocationOutbox) (string, error) {
	if event == nil {
		return "", fmt.Errorf("mqtt session revocation outbox event is nil")
	}
	// Always publish the original generation cutoff, even if the device has
	// already been reactivated. An old broker session can survive the outage
	// that delayed this row; the broker uses RevokedAt to terminate only that
	// old generation while preserving sessions authenticated after reactivation.
	payload, err := json.Marshal(mqttSessionRevocationMessage{
		Version:   1,
		EventID:   event.ID,
		DeviceID:  strings.TrimSpace(event.DeviceID),
		RevokedAt: event.RevokedAt.UTC(),
	})
	if err != nil {
		return mqttSessionRevocationStatusPending, retryMQTTSessionRevocationOutbox(ctx, event, err)
	}
	subscriberCount, err := publishMQTTSessionRevocation(ctx, mqttDeviceSessionRevocationChannel, string(payload))
	if err != nil {
		return mqttSessionRevocationStatusPending, retryMQTTSessionRevocationOutbox(
			ctx,
			event,
			fmt.Errorf("publish mqtt session revocation for device %s: %w", event.DeviceID, err),
		)
	}
	if subscriberCount == 0 {
		return mqttSessionRevocationStatusPending, retryMQTTSessionRevocationOutbox(
			ctx,
			event,
			fmt.Errorf("publish mqtt session revocation for device %s: no broker subscriber", event.DeviceID),
		)
	}
	return markMQTTSessionRevocationAwaitingAck(ctx, event, subscriberCount)
}

func retryMQTTSessionRevocationOutbox(ctx context.Context, event *mqttSessionRevocationOutbox, deliveryErr error) error {
	if event == nil || event.ClaimToken == nil || strings.TrimSpace(*event.ClaimToken) == "" {
		return fmt.Errorf("%v; mqtt session revocation claim token is missing", deliveryErr)
	}
	now := time.Now().UTC()
	nextRetryAt := now.Add(mqttSessionRevocationRetryDelay(event.Attempts))
	message := deliveryErr.Error()
	if global.DB == nil {
		return deliveryErr
	}
	result := global.DB.WithContext(ctx).
		Model(&mqttSessionRevocationOutbox{}).
		Where("id = ? AND status = ? AND claim_token = ?", event.ID, mqttSessionRevocationStatusProcessing, *event.ClaimToken).
		Updates(map[string]interface{}{
			"status":        mqttSessionRevocationStatusPending,
			"claim_token":   nil,
			"last_error":    message,
			"next_retry_at": nextRetryAt,
			"updated_at":    now,
		})
	if result.Error != nil {
		return fmt.Errorf("%v; persist mqtt session revocation retry: %w", deliveryErr, result.Error)
	}
	if result.RowsAffected != 1 {
		status, statusErr := loadMQTTSessionRevocationStatus(ctx, event.ID)
		if statusErr == nil && status == mqttSessionRevocationStatusAcknowledged {
			return nil
		}
		return fmt.Errorf("%v; mqtt session revocation retry lost claim ownership", deliveryErr)
	}
	return deliveryErr
}

func markMQTTSessionRevocationAwaitingAck(ctx context.Context, event *mqttSessionRevocationOutbox, subscriberCount int64) (string, error) {
	if global.DB == nil {
		return mqttSessionRevocationStatusProcessing, fmt.Errorf("database is not initialized for mqtt session revocation outbox")
	}
	if event == nil || event.ClaimToken == nil || strings.TrimSpace(*event.ClaimToken) == "" {
		return mqttSessionRevocationStatusProcessing, fmt.Errorf("mqtt session revocation claim token is missing")
	}
	now := time.Now().UTC()
	ackDeadline := now.Add(mqttSessionRevocationAckTimeout())
	result := global.DB.WithContext(ctx).
		Model(&mqttSessionRevocationOutbox{}).
		Where("id = ? AND status = ? AND claim_token = ?", event.ID, mqttSessionRevocationStatusProcessing, *event.ClaimToken).
		Updates(map[string]interface{}{
			"status":           mqttSessionRevocationStatusAwaitingAck,
			"claim_token":      nil,
			"last_error":       nil,
			"next_retry_at":    ackDeadline,
			"published_at":     now,
			"subscriber_count": subscriberCount,
			"updated_at":       now,
		})
	if result.Error != nil {
		return mqttSessionRevocationStatusProcessing, result.Error
	}
	if result.RowsAffected == 1 {
		return mqttSessionRevocationStatusAwaitingAck, nil
	}
	status, err := loadMQTTSessionRevocationStatus(ctx, event.ID)
	if err != nil {
		return mqttSessionRevocationStatusProcessing, err
	}
	if status == mqttSessionRevocationStatusAcknowledged {
		if err := global.DB.WithContext(ctx).Model(&mqttSessionRevocationOutbox{}).
			Where("id = ? AND status = ?", event.ID, mqttSessionRevocationStatusAcknowledged).
			Updates(map[string]interface{}{
				"published_at":     now,
				"subscriber_count": subscriberCount,
				"updated_at":       now,
			}).Error; err != nil {
			return status, err
		}
		return status, nil
	}
	return status, fmt.Errorf("mqtt session revocation awaiting-ack transition lost claim ownership")
}

func loadMQTTSessionRevocationStatus(ctx context.Context, eventID string) (string, error) {
	if global.DB == nil {
		return "", fmt.Errorf("database is not initialized for mqtt session revocation outbox")
	}
	var event mqttSessionRevocationOutbox
	if err := global.DB.WithContext(ctx).Select("status").First(&event, "id = ?", eventID).Error; err != nil {
		return "", err
	}
	return event.Status, nil
}

func mqttSessionRevocationAckTimeout() time.Duration {
	if value := viper.GetDuration("mqtt_session_revocations.ack_timeout"); value > 0 {
		return value
	}
	return mqttSessionRevocationDefaultAckTimeout
}

func acknowledgeMQTTSessionRevocation(ctx context.Context, ack *mqttSessionRevocationAckMessage) error {
	if global.DB == nil {
		return fmt.Errorf("database is not initialized for mqtt session revocation acknowledgement")
	}
	if err := validateMQTTSessionRevocationAck(ack); err != nil {
		return err
	}
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event mqttSessionRevocationOutbox
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&event, "id = ?", ack.EventID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		if strings.TrimSpace(event.DeviceID) != ack.DeviceID || !event.RevokedAt.UTC().Equal(ack.RevokedAt.UTC()) {
			return fmt.Errorf("mqtt session revocation acknowledgement does not match persisted event")
		}
		if event.Status == mqttSessionRevocationStatusAcknowledged {
			return nil
		}

		persistedAck := &mqttSessionRevocationAck{
			EventID:            ack.EventID,
			BrokerID:           ack.BrokerID,
			DeviceID:           ack.DeviceID,
			RevokedAt:          ack.RevokedAt.UTC(),
			ProcessedAt:        ack.ProcessedAt.UTC(),
			TerminatedSessions: ack.TerminatedSessions,
			CreatedAt:          time.Now().UTC(),
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(persistedAck).Error; err != nil {
			return err
		}

		requiredBrokerIDs, err := decodeMQTTSessionRevocationRequiredBrokerIDs(event.RequiredBrokerIDs)
		if err != nil {
			return err
		}
		complete, acknowledgedBrokerCount, err := mqttSessionRevocationRequiredAcksComplete(tx, event.ID, requiredBrokerIDs)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if !complete {
			return tx.Model(&mqttSessionRevocationOutbox{}).
				Where("id = ? AND status <> ?", event.ID, mqttSessionRevocationStatusAcknowledged).
				Updates(map[string]interface{}{
					"acknowledged_broker_count": acknowledgedBrokerCount,
					"updated_at":                now,
				}).Error
		}
		return tx.Model(&mqttSessionRevocationOutbox{}).
			Where("id = ? AND status IN ?", event.ID, []string{
				mqttSessionRevocationStatusPending,
				mqttSessionRevocationStatusProcessing,
				mqttSessionRevocationStatusAwaitingAck,
			}).
			Updates(map[string]interface{}{
				"status":                    mqttSessionRevocationStatusAcknowledged,
				"claim_token":               nil,
				"last_error":                nil,
				"next_retry_at":             nil,
				"acknowledged_at":           now,
				"acknowledged_broker_count": acknowledgedBrokerCount,
				"updated_at":                now,
			}).Error
	})
}

func validateMQTTSessionRevocationAck(ack *mqttSessionRevocationAckMessage) error {
	if ack == nil {
		return fmt.Errorf("mqtt session revocation acknowledgement is nil")
	}
	ack.EventID = strings.TrimSpace(ack.EventID)
	ack.DeviceID = strings.TrimSpace(ack.DeviceID)
	brokerID, err := normalizeMQTTSessionRevocationBrokerID(ack.BrokerID)
	if err != nil {
		return fmt.Errorf("invalid mqtt session revocation acknowledgement broker id: %w", err)
	}
	ack.BrokerID = brokerID
	ack.Status = strings.TrimSpace(ack.Status)
	if ack.Version != 1 {
		return fmt.Errorf("mqtt session revocation acknowledgement version must be 1")
	}
	if ack.EventID == "" || ack.DeviceID == "" {
		return fmt.Errorf("mqtt session revocation acknowledgement identifiers are required")
	}
	if ack.Status != "processed" {
		return fmt.Errorf("mqtt session revocation acknowledgement status must be processed")
	}
	if ack.RevokedAt.IsZero() || ack.ProcessedAt.IsZero() {
		return fmt.Errorf("mqtt session revocation acknowledgement timestamps are required")
	}
	if ack.TerminatedSessions < 0 {
		return fmt.Errorf("mqtt session revocation terminated_sessions cannot be negative")
	}
	return nil
}

func mqttSessionRevocationRequiredAcksComplete(tx *gorm.DB, eventID string, requiredBrokerIDs []string) (bool, int, error) {
	query := tx.Model(&mqttSessionRevocationAck{}).Where("event_id = ?", eventID)
	if len(requiredBrokerIDs) > 0 {
		query = query.Where("broker_id IN ?", requiredBrokerIDs)
	}
	var acknowledgedBrokerCount int64
	if err := query.Distinct("broker_id").Count(&acknowledgedBrokerCount).Error; err != nil {
		return false, 0, err
	}
	if len(requiredBrokerIDs) == 0 {
		return acknowledgedBrokerCount >= 1, int(acknowledgedBrokerCount), nil
	}
	return acknowledgedBrokerCount == int64(len(requiredBrokerIDs)), int(acknowledgedBrokerCount), nil
}

// AcknowledgeMQTTSessionRevocation validates and persists one broker ACK
// payload. Subscription ownership stays in the application worker so there is
// exactly one Redis lifecycle owner per backend process.
func AcknowledgeMQTTSessionRevocation(ctx context.Context, payload string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var ack mqttSessionRevocationAckMessage
	if err := json.Unmarshal([]byte(payload), &ack); err != nil {
		return fmt.Errorf("decode mqtt session revocation acknowledgement: %w", err)
	}
	return acknowledgeMQTTSessionRevocation(ctx, &ack)
}

func mqttSessionRevocationRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := mqttSessionRevocationRetryBase
	for i := 1; i < attempts && delay < mqttSessionRevocationRetryMax; i++ {
		delay *= 2
		if delay > mqttSessionRevocationRetryMax {
			return mqttSessionRevocationRetryMax
		}
	}
	return delay
}
