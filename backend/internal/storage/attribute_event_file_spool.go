// attribute_event_file_spool.go is the attribute/event record codec for the
// shared fileSpool. It keeps its own directory, accounting, replay lock and
// metrics (separate spool instance), and every file contains one complete
// envelope so an attribute report can only be replayed as one transaction. The
// on-disk schema (version 1, identity, checksum, created_at, envelope) and the
// strict decoding rules are unchanged, so records written by earlier releases
// remain readable and replay on startup.
package storage

import (
	"bytes"
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
	attributeEventFileSpoolRecordVersion = 1
	attributeEventFileSpoolExtension     = fileSpoolExtension
	attributeEventFileSpoolCorruptSuffix = fileSpoolCorruptSuffix
	attributeEventFileSpoolReadLimit     = fileSpoolReadSafetyLimit
	attributeEventFileSpoolTempPrefix    = ".attribute-event-spool-"
)

type attributeEventFileSpoolRecord struct {
	Version   int                    `json:"version"`
	Identity  string                 `json:"identity"`
	Checksum  string                 `json:"checksum"`
	CreatedAt time.Time              `json:"created_at"`
	Envelope  attributeEventEnvelope `json:"envelope"`
}

// attributeEventSpoolCodec keeps the historical attribute/event rules: strict
// JSON decoding, eager startup quarantine, identity-ordered replay.
type attributeEventSpoolCodec struct{}

func (attributeEventSpoolCodec) label() string        { return "attribute/event" }
func (attributeEventSpoolCodec) tempPrefix() string   { return attributeEventFileSpoolTempPrefix }
func (attributeEventSpoolCodec) validateOnInit() bool { return true }
func (attributeEventSpoolCodec) orderByModTime() bool { return false }

func (attributeEventSpoolCodec) prepare(envelope attributeEventEnvelope) (attributeEventEnvelope, string, error) {
	validated, err := validateAttributeEventEnvelope(envelope)
	if err != nil {
		return envelope, "", err
	}
	return validated, validated.Identity, nil
}

func (attributeEventSpoolCodec) encode(envelope attributeEventEnvelope, _ string, now time.Time) ([]byte, error) {
	_, payload, err := buildAttributeEventFileSpoolRecord(envelope, now)
	return payload, err
}

func (attributeEventSpoolCodec) decode(payload []byte) (attributeEventEnvelope, string, error) {
	record, err := decodeAttributeEventFileSpoolRecord(payload)
	if err != nil {
		return attributeEventEnvelope{}, "", err
	}
	return record.Envelope, record.Identity, nil
}

func (attributeEventSpoolCodec) equivalent(existing, incoming attributeEventEnvelope) bool {
	return equalAttributeEventEnvelopes(existing, incoming)
}

// attributeEventFileSpool is the attribute/event instance of the shared spool.
type attributeEventFileSpool = fileSpool[attributeEventEnvelope, attributeEventSpoolCodec]

type attributeEventFileSpoolUsage = fileSpoolUsage
type attributeEventFileSpoolStoreResult = fileSpoolStoreResult
type attributeEventFileSpoolReplayResult = fileSpoolReplayResult
type attributeEventFileSpoolReplayFunc func(context.Context, attributeEventEnvelope) error

func newAttributeEventFileSpool(config Config) *attributeEventFileSpool {
	if !config.AttributeEventSpoolEnabled {
		return nil
	}
	return &attributeEventFileSpool{
		directory:      filepath.Clean(strings.TrimSpace(config.AttributeEventSpoolDirectory)),
		independentOf:  []string{filepath.Clean(strings.TrimSpace(config.TelemetrySpoolDirectory))},
		maxBytes:       config.AttributeEventSpoolMaxBytes,
		maxRecords:     config.AttributeEventSpoolMaxRecords,
		maxRecordBytes: config.AttributeEventSpoolMaxRecordBytes,
		commitWindow:   config.AttributeEventSpoolGroupCommitWindow,
	}
}

func buildAttributeEventFileSpoolRecord(
	envelope attributeEventEnvelope,
	now time.Time,
) (attributeEventFileSpoolRecord, []byte, error) {
	checksum, err := attributeEventEnvelopeChecksum(envelope)
	if err != nil {
		return attributeEventFileSpoolRecord{}, nil, err
	}
	record := attributeEventFileSpoolRecord{
		Version:   attributeEventFileSpoolRecordVersion,
		Identity:  envelope.Identity,
		Checksum:  checksum,
		CreatedAt: now.UTC(),
		Envelope:  envelope,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return attributeEventFileSpoolRecord{}, nil, fmt.Errorf("marshal attribute/event spool record: %w", err)
	}
	return record, payload, nil
}

func attributeEventEnvelopeChecksum(envelope attributeEventEnvelope) (string, error) {
	envelopePayload, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("marshal attribute/event spool envelope: %w", err)
	}
	checksum := sha256.Sum256(envelopePayload)
	return hex.EncodeToString(checksum[:]), nil
}

// decodeAttributeEventFileSpoolRecord strictly parses and integrity-checks one
// record: unknown fields, trailing data, version, envelope validity, identity
// and checksum are all enforced.
func decodeAttributeEventFileSpoolRecord(payload []byte) (attributeEventFileSpoolRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var record attributeEventFileSpoolRecord
	if err := decoder.Decode(&record); err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	if record.Version != attributeEventFileSpoolRecordVersion {
		return attributeEventFileSpoolRecord{}, fmt.Errorf("unsupported record version %d", record.Version)
	}
	validated, err := validateAttributeEventEnvelope(record.Envelope)
	if err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	if record.Identity != validated.Identity {
		return attributeEventFileSpoolRecord{}, fmt.Errorf("record identity mismatch")
	}
	checksum, err := attributeEventEnvelopeChecksum(validated)
	if err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	if record.Checksum != checksum {
		return attributeEventFileSpoolRecord{}, fmt.Errorf("record checksum mismatch")
	}
	record.Envelope = validated
	return record, nil
}

// readAttributeEventFileSpoolRecord is kept for tests and tooling that inspect
// a single record file directly.
func readAttributeEventFileSpoolRecord(
	path string,
	maxRecordBytes int64,
	verifyFilename bool,
) (attributeEventFileSpoolRecord, error) {
	payload, err := readFileSpoolPayload(path, maxRecordBytes)
	if err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	record, err := decodeAttributeEventFileSpoolRecord(payload)
	if err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	if verifyFilename && filepath.Base(path) != attributeEventFileSpoolFilename(record.Identity) {
		return attributeEventFileSpoolRecord{}, fmt.Errorf("record filename identity mismatch")
	}
	return record, nil
}

func attributeEventFileSpoolFilename(identity string) string {
	return fileSpoolFilename(identity)
}

func equalAttributeEventEnvelopes(left, right attributeEventEnvelope) bool {
	// A trusted protocol retry reuses message_id but receives a fresh adapter
	// timestamp. The first durable writer owns that timestamp; all other identity
	// and canonical payload fields must still match exactly.
	return left.Version == right.Version &&
		strings.EqualFold(left.Identity, right.Identity) &&
		strings.EqualFold(left.Fingerprint, right.Fingerprint) &&
		left.DeviceID == right.DeviceID &&
		left.TenantID == right.TenantID &&
		left.Kind == right.Kind &&
		bytes.Equal(left.Payload, right.Payload)
}

func acceptExistingAttributeEventSpoolRecord(
	existing attributeEventFileSpoolRecord,
	incoming attributeEventFileSpoolRecord,
) error {
	if existing.Identity != incoming.Identity ||
		!equalAttributeEventEnvelopes(existing.Envelope, incoming.Envelope) {
		return fmt.Errorf("attribute/event spool deterministic identity collision")
	}
	return nil
}
