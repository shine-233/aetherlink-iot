// telemetry_file_spool.go is the telemetry record codec for the shared
// fileSpool. Each record file holds one history row; the on-disk schema
// (version 1, identity, checksum, created_at, history) is unchanged, so files
// written by earlier releases remain readable and replay on startup.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	telemetryFileSpoolRecordVersion   = 1
	telemetryFileSpoolCorruptSuffix   = fileSpoolCorruptSuffix
	telemetryFileSpoolReadSafetyLimit = fileSpoolReadSafetyLimit
	telemetryFileSpoolTempPrefix      = ".telemetry-spool-"
)

type telemetryFileSpoolRecord struct {
	Version   int           `json:"version"`
	Identity  string        `json:"identity"`
	Checksum  string        `json:"checksum"`
	CreatedAt time.Time     `json:"created_at"`
	History   TelemetryData `json:"history"`
}

// telemetrySpoolCodec keeps the historical telemetry rules: lenient JSON
// decoding, lazy corruption detection (on store/replay), oldest-mtime replay.
type telemetrySpoolCodec struct{}

func (telemetrySpoolCodec) label() string        { return "telemetry" }
func (telemetrySpoolCodec) tempPrefix() string   { return telemetryFileSpoolTempPrefix }
func (telemetrySpoolCodec) validateOnInit() bool { return false }
func (telemetrySpoolCodec) orderByModTime() bool { return true }

func (telemetrySpoolCodec) prepare(history TelemetryData) (TelemetryData, string, error) {
	if !telemetryDataReplayable(history) {
		return history, "", fmt.Errorf("telemetry row is missing replay identity")
	}
	return history, telemetryFileSpoolIdentity(history), nil
}

func (telemetrySpoolCodec) encode(history TelemetryData, _ string, now time.Time) ([]byte, error) {
	_, payload, err := buildTelemetryFileSpoolRecord(history, now)
	return payload, err
}

func (telemetrySpoolCodec) decode(payload []byte) (TelemetryData, string, error) {
	var record telemetryFileSpoolRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return TelemetryData{}, "", err
	}
	if record.Version != telemetryFileSpoolRecordVersion {
		return TelemetryData{}, "", fmt.Errorf("unsupported record version %d", record.Version)
	}
	if !telemetryDataReplayable(record.History) {
		return TelemetryData{}, "", fmt.Errorf("record is missing replay identity")
	}
	if record.Identity != telemetryFileSpoolIdentity(record.History) {
		return TelemetryData{}, "", fmt.Errorf("record identity mismatch")
	}
	checksum, err := telemetryHistoryChecksum(record.History)
	if err != nil {
		return TelemetryData{}, "", err
	}
	if record.Checksum != checksum {
		return TelemetryData{}, "", fmt.Errorf("record checksum mismatch")
	}
	return record.History, record.Identity, nil
}

func (telemetrySpoolCodec) equivalent(existing, incoming TelemetryData) bool {
	left, leftErr := telemetryHistoryChecksum(existing)
	right, rightErr := telemetryHistoryChecksum(incoming)
	return leftErr == nil && rightErr == nil && left == right
}

// telemetryFileSpool is the telemetry instance of the shared spool.
type telemetryFileSpool = fileSpool[TelemetryData, telemetrySpoolCodec]

type telemetryFileSpoolUsage = fileSpoolUsage
type telemetryFileSpoolStoreResult = fileSpoolStoreResult
type telemetryFileSpoolReplayResult = fileSpoolReplayResult
type telemetryFileSpoolReplayFunc func(context.Context, TelemetryData) error

func newTelemetryFileSpool(config Config) *telemetryFileSpool {
	if !config.TelemetrySpoolEnabled {
		return nil
	}
	return &telemetryFileSpool{
		directory:      filepath.Clean(strings.TrimSpace(config.TelemetrySpoolDirectory)),
		maxBytes:       config.TelemetrySpoolMaxBytes,
		maxRecords:     config.TelemetrySpoolMaxRecords,
		maxRecordBytes: config.TelemetrySpoolMaxRecordBytes,
		commitWindow:   config.TelemetrySpoolGroupCommitWindow,
	}
}

func telemetryHistoryChecksum(history TelemetryData) (string, error) {
	historyPayload, err := json.Marshal(history)
	if err != nil {
		return "", fmt.Errorf("marshal telemetry spool history: %w", err)
	}
	checksum := sha256.Sum256(historyPayload)
	return hex.EncodeToString(checksum[:]), nil
}

func buildTelemetryFileSpoolRecord(history TelemetryData, now time.Time) (telemetryFileSpoolRecord, []byte, error) {
	checksum, err := telemetryHistoryChecksum(history)
	if err != nil {
		return telemetryFileSpoolRecord{}, nil, err
	}
	record := telemetryFileSpoolRecord{
		Version:   telemetryFileSpoolRecordVersion,
		Identity:  telemetryFileSpoolIdentity(history),
		Checksum:  checksum,
		CreatedAt: now.UTC(),
		History:   history,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return telemetryFileSpoolRecord{}, nil, fmt.Errorf("marshal telemetry spool record: %w", err)
	}
	return record, payload, nil
}

func telemetryFileSpoolIdentity(history TelemetryData) string {
	// Match the authoritative PostgreSQL history uniqueness constraint exactly.
	// If a logically identical point arrives again, the first durable value wins.
	identity := fmt.Sprintf("%s\x00%s\x00%d", history.DeviceID, history.Key, history.TS)
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

func telemetryFileSpoolFilename(identity string) string {
	return fileSpoolFilename(identity)
}

func isTelemetryFileSpoolTemp(name string) bool {
	return strings.HasPrefix(name, telemetryFileSpoolTempPrefix) && strings.HasSuffix(name, ".tmp")
}

// readTelemetryFileSpoolRecord is kept for tests and tooling that inspect a
// single record file directly.
func readTelemetryFileSpoolRecord(path string, maxRecordBytes int64, verifyFilename bool) (telemetryFileSpoolRecord, error) {
	payload, err := readFileSpoolPayload(path, maxRecordBytes)
	if err != nil {
		return telemetryFileSpoolRecord{}, err
	}
	history, identity, err := telemetrySpoolCodec{}.decode(payload)
	if err != nil {
		return telemetryFileSpoolRecord{}, err
	}
	if verifyFilename && filepath.Base(path) != fileSpoolFilename(identity) {
		return telemetryFileSpoolRecord{}, fmt.Errorf("record identity mismatch")
	}
	var record telemetryFileSpoolRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return telemetryFileSpoolRecord{}, err
	}
	record.History = history
	return record, nil
}

// removeWriteAheadReceipt retires the receipt of history after its database
// flush. A missing file is not an error.
func removeTelemetryWriteAheadReceipt(s *telemetryFileSpool, history TelemetryData) error {
	if s == nil || !telemetryDataReplayable(history) {
		return nil
	}
	return s.removeIdentity(telemetryFileSpoolIdentity(history))
}

// removeTelemetryWriteAheadReceipts retires the receipts of a whole flushed
// batch with one grouped directory fsync. Non-replayable rows never had a
// receipt and are skipped; duplicates are harmless (missing files are not
// errors).
func removeTelemetryWriteAheadReceipts(s *telemetryFileSpool, histories []TelemetryData) error {
	if s == nil || len(histories) == 0 {
		return nil
	}
	identities := make([]string, 0, len(histories))
	for _, history := range histories {
		if telemetryDataReplayable(history) {
			identities = append(identities, telemetryFileSpoolIdentity(history))
		}
	}
	return s.removeIdentities(identities)
}
