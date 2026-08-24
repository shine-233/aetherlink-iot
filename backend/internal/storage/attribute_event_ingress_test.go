package storage

// These are source-level durability contracts. Per the 2026-07-19 customer
// continuation constraint they were added but not executed in this pass.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var _ DurableMessagePersister = (*storage)(nil)
var _ DurableStorage = (*storage)(nil)

func TestDurableAttributeEventInputRejectsTelemetryAndInvalidPayloads(t *testing.T) {
	ingress := newAttributeEventIngress(nil, logrus.New(), testAttributeEventConfig(t), newMetricsCollector(false))
	tests := []struct {
		name string
		msg  *Message
		want string
	}{
		{name: "nil", msg: nil, want: "message is nil"},
		{
			name: "telemetry",
			msg: &Message{
				DeviceID: "device-1", TenantID: "tenant-1", DataType: DataTypeTelemetry, Timestamp: 1000,
			},
			want: "telemetry is not accepted",
		},
		{
			name: "non-positive timestamp",
			msg: &Message{
				DeviceID: "device-1", TenantID: "tenant-1", DataType: DataTypeAttribute,
				Data: []AttributeDataPoint{{Key: "mode", Value: "safe"}},
			},
			want: "timestamp must be positive",
		},
		{
			name: "empty attribute key",
			msg:  testAttributeMessage([]AttributeDataPoint{{Key: "", Value: 1}}),
			want: "empty key",
		},
		{
			name: "duplicate attribute key",
			msg:  testAttributeMessage([]AttributeDataPoint{{Key: "mode", Value: 1}, {Key: "mode", Value: 2}}),
			want: "duplicate key",
		},
		{
			name: "empty event identify",
			msg:  testEventMessage("", json.RawMessage(`{"value":1}`)),
			want: "identify is empty",
		},
		{
			name: "invalid event JSON",
			msg:  testEventMessage("alarm", json.RawMessage(`{"value":`)),
			want: "marshal event data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ingress.accept(nil, tt.msg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Accept() error = %v, want substring %q", err, tt.want)
			}
			if usage := ingress.spool.usage(); usage.Records != 0 {
				t.Fatalf("invalid input spool records = %d, want 0", usage.Records)
			}
		})
	}
}

func TestAttributeEnvelopeCanonicalizesPointOrderAndJSON(t *testing.T) {
	leftMessage := testAttributeMessage([]AttributeDataPoint{
		{Key: "zeta", Value: map[string]interface{}{"b": 2, "a": 1}},
		{Key: "alpha", Value: 7},
	})
	leftMessage.SourceMessageID = "same-protocol-report"
	left, err := buildAttributeEventEnvelope(leftMessage)
	if err != nil {
		t.Fatalf("build left envelope: %v", err)
	}
	rightMessage := testAttributeMessage([]AttributeDataPoint{
		{Key: "alpha", Value: 7},
		{Key: "zeta", Value: map[string]interface{}{"a": 1, "b": 2}},
	})
	rightMessage.SourceMessageID = "same-protocol-report"
	right, err := buildAttributeEventEnvelope(rightMessage)
	if err != nil {
		t.Fatalf("build right envelope: %v", err)
	}
	if left.Identity != right.Identity || string(left.Payload) != string(right.Payload) {
		t.Fatalf("canonical envelopes differ: left=%+v right=%+v", left, right)
	}
	if !strings.HasPrefix(string(left.Payload), `[{"key":"alpha"`) {
		t.Fatalf("attribute payload is not key sorted: %s", left.Payload)
	}
	for _, key := range []string{"alpha", "zeta"} {
		if id := deterministicAttributeEventRowID("attribute", left.Identity, key); len(id) != 36 {
			t.Fatalf("derived row ID %q has len %d, want UUID-shaped len 36", id, len(id))
		}
	}
}

func TestEventIdentityDoesNotCollapseEqualEventsInSameMillisecond(t *testing.T) {
	firstMessage := testEventMessage("door-open", json.RawMessage(`{"floor":2}`))
	secondMessage := testEventMessage("door-open", json.RawMessage(`{"floor":2}`))
	first, err := buildAttributeEventEnvelope(firstMessage)
	if err != nil {
		t.Fatalf("build first event: %v", err)
	}
	second, err := buildAttributeEventEnvelope(secondMessage)
	if err != nil {
		t.Fatalf("build second event: %v", err)
	}
	if first.Identity == second.Identity {
		t.Fatalf("independent equal events share message_id %q", first.Identity)
	}

	retryMessage := testEventMessage("door-open", json.RawMessage(`{"floor":2}`))
	retryMessage.SourceMessageID = "mqtt-source-1"
	retry, err := buildAttributeEventEnvelope(retryMessage)
	if err != nil {
		t.Fatalf("build source event: %v", err)
	}
	sameSource := testEventMessage("door-open", json.RawMessage(`{"floor":2}`))
	sameSource.SourceMessageID = "mqtt-source-1"
	sameSource.Timestamp += 60_000
	replayed, err := buildAttributeEventEnvelope(sameSource)
	if err != nil {
		t.Fatalf("build source retry: %v", err)
	}
	if retry.Identity != replayed.Identity || retry.Fingerprint != replayed.Fingerprint {
		t.Fatalf("protocol retry identity changed: first=%+v retry=%+v", retry, replayed)
	}
}

func TestDurableAttributeEnvelopeUsesOneDatabaseTransaction(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	if err := db.Exec(`
		CREATE TRIGGER reject_attribute_envelope_point
		BEFORE INSERT ON attribute_datas
		WHEN NEW."key" = 'reject-me'
		BEGIN
			SELECT RAISE(FAIL, 'forced envelope rollback');
		END
	`).Error; err != nil {
		t.Fatalf("create attribute rejection trigger: %v", err)
	}
	cfg := testAttributeEventConfig(t)
	cfg.AttributeEventSpoolEnabled = false
	ingress := newAttributeEventIngress(db, logrus.New(), cfg, newMetricsCollector(false))
	envelope, buildErr := buildAttributeEventEnvelope(testAttributeMessage([]AttributeDataPoint{
		{Key: "accepted-first", Value: 1},
		{Key: "reject-me", Value: 2},
	}))
	if buildErr != nil {
		t.Fatalf("build transactional attribute envelope: %v", buildErr)
	}
	err := ingress.persistEnvelope(context.Background(), envelope, true)
	if err == nil {
		t.Fatal("persistEnvelope() error = nil after a point failed")
	}
	var count int64
	if err := db.Model(&AttributeData{}).Count(&count).Error; err != nil {
		t.Fatalf("count attributes after rollback: %v", err)
	}
	if count != 0 {
		t.Fatalf("attribute rows after failed envelope = %d, want transaction rollback to 0", count)
	}
	if err := db.Model(&uplinkStorageReceipt{}).Count(&count).Error; err != nil {
		t.Fatalf("count receipts after rollback: %v", err)
	}
	if count != 0 {
		t.Fatalf("attribute receipts after failed envelope = %d, want transaction rollback to 0", count)
	}
}

func TestDurableAttributeEventFallsBackToIndependentSpool(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	if err := db.Migrator().DropTable(&AttributeData{}); err != nil {
		t.Fatalf("drop attribute table: %v", err)
	}
	if err := db.Migrator().DropTable(&uplinkStorageDeadLetter{}); err != nil {
		t.Fatalf("drop attribute/event dead-letter table: %v", err)
	}
	metrics := newMetricsCollector(false)
	ingress := newAttributeEventIngress(db, logrus.New(), testAttributeEventConfig(t), metrics)
	msg := testAttributeMessage([]AttributeDataPoint{{Key: "mode", Value: "auto"}})
	receipt, err := ingress.accept(nil, msg)
	if err != nil {
		t.Fatalf("Accept() with database down and healthy spool: %v", err)
	}
	if receipt.Tier != DurabilityTierFileSpool || receipt.MessageID == "" {
		t.Fatalf("file-spool receipt = %+v", receipt)
	}
	if _, err := ingress.accept(context.Background(), msg); err != nil {
		t.Fatalf("idempotent Accept() duplicate: %v", err)
	}
	usage := ingress.spool.usage()
	if usage.Records != 1 || usage.QuarantinedRecords != 0 {
		t.Fatalf("spool usage = %+v, want one healthy deterministic record", usage)
	}
	got := metrics.GetMetrics()
	if got.AttributeEventSpooled != 1 || got.AttributeEventSpoolBacklog != 1 || got.AttributeEventSpoolBytes == 0 || got.AttributeEventDeadLetterFailed != 2 {
		t.Fatalf("attribute/event fallback metrics = %+v", got)
	}
}

func TestDurableAttributeEventUsesPostgreSQLDeadLetterBeforeFileSpool(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	if err := db.Migrator().DropTable(&AttributeData{}); err != nil {
		t.Fatalf("drop attribute table: %v", err)
	}
	metrics := newMetricsCollector(false)
	ingress := newAttributeEventIngress(db, logrus.New(), testAttributeEventConfig(t), metrics)
	message := testAttributeMessage([]AttributeDataPoint{{Key: "mode", Value: "safe"}})
	receipt, err := ingress.accept(context.Background(), message)
	if err != nil {
		t.Fatalf("Accept() with healthy PostgreSQL dead letter: %v", err)
	}
	if receipt.Tier != DurabilityTierPostgresDeadLetter || receipt.MessageID == "" {
		t.Fatalf("PostgreSQL dead-letter receipt = %+v", receipt)
	}
	if usage := ingress.spool.usage(); usage.Records != 0 {
		t.Fatalf("file spool records = %d, want PostgreSQL dead letter to win", usage.Records)
	}
	var deadLetters []uplinkStorageDeadLetter
	if err := db.Find(&deadLetters).Error; err != nil {
		t.Fatalf("load PostgreSQL dead letter: %v", err)
	}
	if len(deadLetters) != 1 || deadLetters[0].Status != uplinkDeadLetterStatusPending || len(deadLetters[0].ID) != 36 {
		t.Fatalf("dead letters = %+v", deadLetters)
	}
	if got := metrics.GetMetrics(); got.AttributeEventDeadLettered != 1 || got.AttributeEventDeadLetterFailed != 0 {
		t.Fatalf("dead-letter metrics after fallback = %+v", got)
	}

	if err := db.AutoMigrate(&AttributeData{}); err != nil {
		t.Fatalf("restore attribute table: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS attribute_event_device_key_test ON attribute_datas(device_id, key)").Error; err != nil {
		t.Fatalf("restore attribute unique index: %v", err)
	}
	if err := ingress.replayDeadLetters(context.Background()); err != nil {
		t.Fatalf("replay PostgreSQL dead letter: %v", err)
	}
	if got := metrics.GetMetrics(); got.AttributeEventDeadLetterReplayed != 1 {
		t.Fatalf("dead-letter replay metrics = %+v", got)
	}
	var attribute AttributeData
	if err := db.Where("device_id = ? AND key = ?", message.DeviceID, "mode").Take(&attribute).Error; err != nil {
		t.Fatalf("load dead-letter-replayed attribute: %v", err)
	}
	var resolved uplinkStorageDeadLetter
	if err := db.Where("id = ?", deadLetters[0].ID).Take(&resolved).Error; err != nil {
		t.Fatalf("load resolved dead letter: %v", err)
	}
	if resolved.Status != uplinkDeadLetterStatusResolved {
		t.Fatalf("dead-letter status = %q, want resolved", resolved.Status)
	}
	var receipts int64
	if err := db.Model(&uplinkStorageReceipt{}).Count(&receipts).Error; err != nil || receipts != 1 {
		t.Fatalf("attribute receipt count = %d err=%v, want 1", receipts, err)
	}
}

func TestAttributeEventDeadLetterClaimUsesLeaseFencing(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	firstIngress := newAttributeEventIngress(db, logrus.New(), testAttributeEventConfig(t), newMetricsCollector(false))
	secondIngress := newAttributeEventIngress(db, logrus.New(), testAttributeEventConfig(t), newMetricsCollector(false))
	envelope, err := buildAttributeEventEnvelope(testEventMessage("lease", json.RawMessage(`{"ok":true}`)))
	if err != nil {
		t.Fatalf("build lease envelope: %v", err)
	}
	if err := firstIngress.persistDeadLetter(context.Background(), envelope); err != nil {
		t.Fatalf("persist lease dead letter: %v", err)
	}

	claimedAt := time.Unix(100, 0).UTC()
	first, claimed, err := firstIngress.claimDeadLetter(context.Background(), envelope.Identity, claimedAt)
	if err != nil || !claimed || first.ClaimToken == nil || first.LeaseUntil == nil {
		t.Fatalf("first claim = %+v claimed=%t err=%v", first, claimed, err)
	}
	if _, claimed, err := secondIngress.claimDeadLetter(context.Background(), envelope.Identity, claimedAt); err != nil || claimed {
		t.Fatalf("concurrent claim claimed=%t err=%v, want fenced rejection", claimed, err)
	}

	second, claimed, err := secondIngress.claimDeadLetter(
		context.Background(),
		envelope.Identity,
		first.LeaseUntil.Add(time.Millisecond),
	)
	if err != nil || !claimed || second.ClaimToken == nil || *second.ClaimToken == *first.ClaimToken {
		t.Fatalf("expired lease recovery = %+v claimed=%t err=%v", second, claimed, err)
	}
	staleErr := firstIngress.updateClaimedDeadLetter(context.Background(), first, map[string]interface{}{
		"status":      uplinkDeadLetterStatusResolved,
		"claim_token": nil,
		"lease_until": nil,
	})
	if staleErr == nil || !strings.Contains(staleErr.Error(), "lease was lost") {
		t.Fatalf("stale owner update error = %v, want fencing conflict", staleErr)
	}
	if err := secondIngress.updateClaimedDeadLetter(context.Background(), second, map[string]interface{}{
		"status":        uplinkDeadLetterStatusResolved,
		"last_error":    "",
		"next_retry_at": nil,
		"claim_token":   nil,
		"lease_until":   nil,
		"updated_at":    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("current owner resolve: %v", err)
	}
}

func TestDurableAttributeEventPrimaryReceiptIsExplicit(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	ingress := newAttributeEventIngress(db, logrus.New(), testAttributeEventConfig(t), newMetricsCollector(false))
	receipt, err := ingress.accept(context.Background(), testEventMessage("door-open", json.RawMessage(`{"floor":2}`)))
	if err != nil {
		t.Fatalf("persist primary event: %v", err)
	}
	if receipt.Tier != DurabilityTierPrimary || receipt.MessageID == "" {
		t.Fatalf("primary receipt = %+v", receipt)
	}
}

func TestAttributeEventSpoolReplayCommitsThenDeletesAndIsIdempotent(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	if err := db.Migrator().DropTable(&EventDataModel{}); err != nil {
		t.Fatalf("drop event table: %v", err)
	}
	if err := db.Migrator().DropTable(&uplinkStorageDeadLetter{}); err != nil {
		t.Fatalf("drop attribute/event dead-letter table: %v", err)
	}
	metrics := newMetricsCollector(false)
	ingress := newAttributeEventIngress(db, logrus.New(), testAttributeEventConfig(t), metrics)
	msg := testEventMessage("overheat", json.RawMessage(`{"unit":"C","value":81}`))
	if _, err := ingress.accept(context.Background(), msg); err != nil {
		t.Fatalf("spool event while database table is absent: %v", err)
	}
	if err := db.AutoMigrate(&EventDataModel{}); err != nil {
		t.Fatalf("restore event table: %v", err)
	}
	ingress.replayOnce(context.Background())
	if usage := ingress.spool.usage(); usage.Records != 0 {
		t.Fatalf("spool records after replay = %d, want 0", usage.Records)
	}
	var rows []EventDataModel
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("load replayed event: %v", err)
	}
	if len(rows) != 1 || rows[0].Identify != "overheat" {
		t.Fatalf("replayed event rows = %+v", rows)
	}
	// A second pass has nothing to replay and cannot duplicate the event.
	ingress.replayOnce(context.Background())
	var count int64
	if err := db.Model(&EventDataModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count idempotent event rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("event rows after second replay = %d, want 1", count)
	}
	got := metrics.GetMetrics()
	if got.AttributeEventSpoolReplayed != 1 || got.AttributeEventSpoolBacklog != 0 {
		t.Fatalf("replay metrics = %+v", got)
	}
}

func TestEventDeterministicIdentityKeepsFirstWriterAndDetectsCollision(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	ingress := newAttributeEventIngress(db, logrus.New(), testAttributeEventConfig(t), newMetricsCollector(false))
	firstMessage := testEventMessage(
		"door-open",
		json.RawMessage(`{"floor":2}`),
	)
	firstMessage.SourceMessageID = "mqtt-source-first-writer"
	envelope, err := buildAttributeEventEnvelope(firstMessage)
	if err != nil {
		t.Fatalf("build event envelope: %v", err)
	}
	if err := ingress.persistEnvelope(context.Background(), envelope, true); err != nil {
		t.Fatalf("persist first event: %v", err)
	}
	retryMessage := testEventMessage("door-open", json.RawMessage(`{"floor":2}`))
	retryMessage.SourceMessageID = firstMessage.SourceMessageID
	retryMessage.Timestamp = firstMessage.Timestamp + 999
	retryEnvelope, err := buildAttributeEventEnvelope(retryMessage)
	if err != nil {
		t.Fatalf("build event retry envelope: %v", err)
	}
	if err := ingress.persistEnvelope(context.Background(), retryEnvelope, true); err != nil {
		t.Fatalf("persist protocol retry: %v", err)
	}
	var count int64
	if err := db.Model(&EventDataModel{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("event count after retry = %d err=%v, want 1", count, err)
	}
	var firstWriter EventDataModel
	if err := db.Where("id = ?", envelope.Identity).Take(&firstWriter).Error; err != nil {
		t.Fatalf("load first-writer event: %v", err)
	}
	if firstWriter.TS.UnixMilli() != firstMessage.Timestamp {
		t.Fatalf("event retry changed first-writer timestamp: got %d want %d", firstWriter.TS.UnixMilli(), firstMessage.Timestamp)
	}

	rowID := envelope.Identity
	if err := db.Model(&EventDataModel{}).Where("id = ?", rowID).Update("data", json.RawMessage(`{"floor":99}`)).Error; err != nil {
		t.Fatalf("prepare deterministic identity collision: %v", err)
	}
	err = ingress.persistEnvelope(context.Background(), envelope, true)
	if err == nil || !strings.Contains(err.Error(), "event deterministic identity collision") {
		t.Fatalf("collision error = %v", err)
	}
	var existing EventDataModel
	if err := db.Where("id = ?", rowID).Take(&existing).Error; err != nil {
		t.Fatalf("load first-writer row: %v", err)
	}
	if !strings.Contains(string(existing.Data), "99") {
		t.Fatalf("collision overwrote first-writer row: %s", existing.Data)
	}
}

func TestAttributeEnvelopeKeepsCurrentTimestampMonotonic(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	ingress := newAttributeEventIngress(db, logrus.New(), testAttributeEventConfig(t), newMetricsCollector(false))
	newer := testAttributeMessage([]AttributeDataPoint{{Key: "mode", Value: "new"}})
	newer.Timestamp = 2000
	older := testAttributeMessage([]AttributeDataPoint{{Key: "mode", Value: "old"}})
	older.Timestamp = 1000
	if _, err := ingress.accept(context.Background(), newer); err != nil {
		t.Fatalf("accept newer attribute: %v", err)
	}
	if _, err := ingress.accept(context.Background(), older); err != nil {
		t.Fatalf("accept stale attribute: %v", err)
	}
	var row AttributeData
	if err := db.Where("device_id = ? AND key = ?", newer.DeviceID, "mode").Take(&row).Error; err != nil {
		t.Fatalf("load current attribute: %v", err)
	}
	if row.TS.UnixMilli() != newer.Timestamp || row.StringV == nil || *row.StringV != "new" {
		t.Fatalf("current attribute regressed: %+v", row)
	}
}

func TestStorageStopStopsAttributeEventIngress(t *testing.T) {
	db := setupAttributeEventTestDB(t)
	cfg := testAttributeEventConfig(t)
	service := New(db, logrus.New(), cfg).(*storage)
	input := make(chan *Message)
	if err := service.Start(context.Background(), input); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	close(input)
	select {
	case <-service.doneCh:
	case <-time.After(time.Second):
		t.Fatal("storage main loop did not observe closed input")
	}
	if err := service.Stop(time.Second); err != nil {
		t.Fatalf("Stop(): %v", err)
	}
	if _, err := service.PersistDurably(context.Background(), testEventMessage("late", json.RawMessage(`{}`))); !errorsIsAttributeEventStopped(err) {
		t.Fatalf("PersistDurably() after Stop = %v, want stopped error", err)
	}
}

func errorsIsAttributeEventStopped(err error) bool {
	return err != nil && strings.Contains(err.Error(), errAttributeEventIngressStopped.Error())
}

func testAttributeMessage(points []AttributeDataPoint) *Message {
	return &Message{
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		DataType:  DataTypeAttribute,
		Timestamp: 1000,
		Data:      points,
	}
}

func testEventMessage(identify string, data json.RawMessage) *Message {
	return &Message{
		DeviceID:  "device-1",
		TenantID:  "tenant-1",
		DataType:  DataTypeEvent,
		Timestamp: 1000,
		Data: EventData{
			Identify: identify,
			Data:     data,
		},
	}
}

func testAttributeEventConfig(t *testing.T) Config {
	t.Helper()
	cfg := DefaultConfig()
	root := t.TempDir()
	cfg.EnableMetrics = false
	cfg.TelemetrySpoolDirectory = filepath.Join(root, "telemetry-spool")
	cfg.AttributeEventSpoolDirectory = filepath.Join(root, "attribute-event-spool")
	cfg.AttributeEventSpoolReplayInterval = time.Hour
	cfg.AttributeEventSpoolReplayTimeout = time.Second
	return cfg
}

func setupAttributeEventTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite attribute/event stand-in: %v", err)
	}
	if err := db.AutoMigrate(
		&AttributeData{},
		&EventDataModel{},
		&uplinkStorageReceipt{},
		&uplinkStorageDeadLetter{},
	); err != nil {
		t.Fatalf("migrate attribute/event stand-in: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS attribute_event_device_key_test ON attribute_datas(device_id, key)").Error; err != nil {
		t.Fatalf("create attribute unique index: %v", err)
	}
	return db
}

func sortedStrings(values []string) []string {
	copyOfValues := append([]string(nil), values...)
	sort.Strings(copyOfValues)
	return copyOfValues
}
