package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	attributeEventFileSpoolRecordVersion = 1
	attributeEventFileSpoolExtension     = ".json"
	attributeEventFileSpoolCorruptSuffix = ".corrupt"
	attributeEventFileSpoolReadLimit     = int64(512 * 1024 * 1024)
)

// attributeEventFileSpool is deliberately separate from the telemetry spool:
// it has its own record schema, directory, accounting, replay lock and metrics.
// Every file contains one complete attribute/event envelope, so an attribute
// report can only be replayed as one transaction.
type attributeEventFileSpool struct {
	directory      string
	telemetryDir   string
	maxBytes       int64
	maxRecords     int
	maxRecordBytes int64

	mu          sync.Mutex
	replayMu    sync.Mutex
	initialized bool
	bytes       int64
	records     int

	quarantinedBytes   int64
	quarantinedRecords int
	startupCorrupt     int
}

type attributeEventFileSpoolRecord struct {
	Version   int                    `json:"version"`
	Identity  string                 `json:"identity"`
	Checksum  string                 `json:"checksum"`
	CreatedAt time.Time              `json:"created_at"`
	Envelope  attributeEventEnvelope `json:"envelope"`
}

type attributeEventFileSpoolUsage struct {
	Records            int
	Bytes              int64
	QuarantinedRecords int
	QuarantinedBytes   int64
}

type attributeEventFileSpoolStoreResult struct {
	Stored      bool
	Duplicate   bool
	Corrupt     int
	Quarantined int
}

type attributeEventFileSpoolReplayResult struct {
	Attempted int
	Replayed  int
	Corrupt   int
	Usage     attributeEventFileSpoolUsage
}

type attributeEventFileSpoolReplayFunc func(context.Context, attributeEventEnvelope) error

func newAttributeEventFileSpool(config Config) *attributeEventFileSpool {
	if !config.AttributeEventSpoolEnabled {
		return nil
	}
	return &attributeEventFileSpool{
		directory:      filepath.Clean(strings.TrimSpace(config.AttributeEventSpoolDirectory)),
		telemetryDir:   filepath.Clean(strings.TrimSpace(config.TelemetrySpoolDirectory)),
		maxBytes:       config.AttributeEventSpoolMaxBytes,
		maxRecords:     config.AttributeEventSpoolMaxRecords,
		maxRecordBytes: config.AttributeEventSpoolMaxRecordBytes,
	}
}

func (s *attributeEventFileSpool) init() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialized {
		return nil
	}
	if err := s.validateConfig(); err != nil {
		return err
	}
	if err := os.MkdirAll(s.directory, 0o700); err != nil {
		return fmt.Errorf("create attribute/event spool directory: %w", err)
	}
	info, err := os.Lstat(s.directory)
	if err != nil {
		return fmt.Errorf("inspect attribute/event spool directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("attribute/event spool path must be a real directory")
	}
	if err := os.Chmod(s.directory, 0o700); err != nil {
		return fmt.Errorf("restrict attribute/event spool directory permissions: %w", err)
	}

	if err := s.recoverTempsLocked(); err != nil {
		return err
	}
	if err := s.quarantineCorruptCommittedLocked(); err != nil {
		return err
	}
	if err := s.refreshUsageLocked(); err != nil {
		return err
	}
	s.initialized = true
	return nil
}

func (s *attributeEventFileSpool) validateConfig() error {
	if s.directory == "" || s.directory == "." || filepath.Dir(s.directory) == s.directory {
		return fmt.Errorf("attribute/event spool directory must be a dedicated non-root path")
	}
	spoolPath, err := filepath.Abs(s.directory)
	if err != nil {
		return fmt.Errorf("resolve attribute/event spool directory: %w", err)
	}
	publicFilesPath, err := filepath.Abs("./files")
	if err != nil {
		return fmt.Errorf("resolve public files directory: %w", err)
	}
	if pathIsWithinDirectory(publicFilesPath, spoolPath) {
		return fmt.Errorf("attribute/event spool directory must stay outside the public files directory")
	}
	if s.telemetryDir != "" && s.telemetryDir != "." {
		telemetryPath, err := filepath.Abs(s.telemetryDir)
		if err != nil {
			return fmt.Errorf("resolve telemetry spool directory: %w", err)
		}
		if pathIsWithinDirectory(telemetryPath, spoolPath) || pathIsWithinDirectory(spoolPath, telemetryPath) {
			return fmt.Errorf("attribute/event spool directory must be independent from the telemetry spool directory")
		}
	}
	if s.maxBytes < 1 {
		return fmt.Errorf("attribute/event spool max bytes must be positive")
	}
	if s.maxRecords < 1 {
		return fmt.Errorf("attribute/event spool max records must be positive")
	}
	if s.maxRecordBytes < 1 || s.maxRecordBytes > s.maxBytes || s.maxRecordBytes > attributeEventFileSpoolReadLimit {
		return fmt.Errorf("attribute/event spool max record bytes must be positive and no larger than max bytes or read safety limit")
	}
	return nil
}

func (s *attributeEventFileSpool) recoverTempsLocked() error {
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("scan attribute/event spool directory: %w", err)
	}
	for _, entry := range entries {
		if !isAttributeEventFileSpoolTemp(entry.Name()) {
			continue
		}
		path := filepath.Join(s.directory, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove attribute/event spool temp symlink: %w", err)
			}
			continue
		}
		record, readErr := readAttributeEventFileSpoolRecord(path, attributeEventFileSpoolReadLimit, false)
		if readErr != nil {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove incomplete attribute/event spool temp file: %w", err)
			}
			continue
		}
		finalPath := filepath.Join(s.directory, attributeEventFileSpoolFilename(record.Identity))
		if finalInfo, inspectErr := os.Lstat(finalPath); inspectErr == nil {
			if finalInfo.Mode()&os.ModeSymlink != 0 || !finalInfo.Mode().IsRegular() {
				return fmt.Errorf("recovered attribute/event spool destination must be a regular file")
			}
			existing, existingErr := readAttributeEventFileSpoolRecord(finalPath, attributeEventFileSpoolReadLimit, true)
			if existingErr != nil {
				if _, quarantineErr := quarantineAttributeEventFile(finalPath); quarantineErr != nil {
					return fmt.Errorf("quarantine corrupt recovered attribute/event spool destination: %w", quarantineErr)
				}
				s.startupCorrupt++
			} else if !equalAttributeEventEnvelopes(existing.Envelope, record.Envelope) {
				return fmt.Errorf("attribute/event spool deterministic identity collision during temp recovery")
			} else {
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("remove duplicate attribute/event spool temp file: %w", err)
				}
				continue
			}
		} else if !errors.Is(inspectErr, os.ErrNotExist) {
			return fmt.Errorf("inspect recovered attribute/event spool destination: %w", inspectErr)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("restrict recovered attribute/event spool temp file: %w", err)
		}
		if err := os.Rename(path, finalPath); err != nil {
			return fmt.Errorf("promote recoverable attribute/event spool temp file: %w", err)
		}
		if err := syncAttributeEventSpoolDirectory(s.directory); err != nil {
			return fmt.Errorf("sync recovered attribute/event spool record: %w", err)
		}
	}
	return nil
}

func (s *attributeEventFileSpool) quarantineCorruptCommittedLocked() error {
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("scan attribute/event spool records: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), attributeEventFileSpoolExtension) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("attribute/event spool record must not be a symlink: %s", entry.Name())
		}
		path := filepath.Join(s.directory, entry.Name())
		if _, readErr := readAttributeEventFileSpoolRecord(path, attributeEventFileSpoolReadLimit, true); readErr == nil {
			continue
		}
		if _, quarantineErr := quarantineAttributeEventFile(path); quarantineErr != nil {
			return fmt.Errorf("quarantine corrupt attribute/event spool record: %w", quarantineErr)
		}
		s.startupCorrupt++
	}
	return syncAttributeEventSpoolDirectory(s.directory)
}

func (s *attributeEventFileSpool) store(
	ctx context.Context,
	envelope attributeEventEnvelope,
	now time.Time,
) (attributeEventFileSpoolStoreResult, error) {
	result := attributeEventFileSpoolStoreResult{}
	if s == nil {
		return result, fmt.Errorf("attribute/event file spool is disabled")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	validated, err := validateAttributeEventEnvelope(envelope)
	if err != nil {
		return result, err
	}
	if err := s.init(); err != nil {
		return result, err
	}
	record, payload, err := buildAttributeEventFileSpoolRecord(validated, now)
	if err != nil {
		return result, err
	}
	if int64(len(payload)) > s.maxRecordBytes {
		return result, fmt.Errorf("attribute/event spool record exceeds %d byte configured limit", s.maxRecordBytes)
	}
	finalPath := filepath.Join(s.directory, attributeEventFileSpoolFilename(record.Identity))

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if finalInfo, inspectErr := os.Lstat(finalPath); inspectErr == nil {
		if finalInfo.Mode()&os.ModeSymlink != 0 || !finalInfo.Mode().IsRegular() {
			return result, fmt.Errorf("attribute/event spool destination must be a regular file")
		}
		existing, readErr := readAttributeEventFileSpoolRecord(finalPath, attributeEventFileSpoolReadLimit, true)
		if readErr != nil {
			result.Corrupt++
			quarantined, quarantineErr := s.quarantineCommittedLocked(finalPath)
			if quarantined {
				result.Quarantined++
			}
			if quarantineErr != nil {
				return result, errors.Join(
					fmt.Errorf("existing attribute/event spool record is corrupt: %w", readErr),
					fmt.Errorf("quarantine existing attribute/event spool record: %w", quarantineErr),
				)
			}
		} else {
			if err := acceptExistingAttributeEventSpoolRecord(existing, record); err != nil {
				return result, err
			}
			if err := syncAttributeEventSpoolDirectory(s.directory); err != nil {
				return result, fmt.Errorf("sync existing attribute/event spool record: %w", err)
			}
			result.Duplicate = true
			return result, nil
		}
	} else if !errors.Is(inspectErr, os.ErrNotExist) {
		return result, fmt.Errorf("inspect attribute/event spool record: %w", inspectErr)
	}
	if s.records+1 > s.maxRecords || s.bytes+int64(len(payload)) > s.maxBytes {
		return result, fmt.Errorf(
			"attribute/event spool capacity exhausted: records=%d/%d bytes=%d/%d incoming_bytes=%d",
			s.records,
			s.maxRecords,
			s.bytes,
			s.maxBytes,
			len(payload),
		)
	}
	if err := writeAttributeEventFileSpoolRecord(ctx, s.directory, finalPath, payload); err != nil {
		if usageErr := s.refreshUsageLocked(); usageErr != nil {
			s.records = s.maxRecords
			s.bytes = s.maxBytes
			return result, errors.Join(err, fmt.Errorf("refresh attribute/event spool usage after failed write: %w", usageErr))
		}
		return result, err
	}
	s.records++
	s.bytes += int64(len(payload))
	result.Stored = true
	return result, nil
}

func buildAttributeEventFileSpoolRecord(
	envelope attributeEventEnvelope,
	now time.Time,
) (attributeEventFileSpoolRecord, []byte, error) {
	envelopePayload, err := json.Marshal(envelope)
	if err != nil {
		return attributeEventFileSpoolRecord{}, nil, fmt.Errorf("marshal attribute/event spool envelope: %w", err)
	}
	checksum := sha256.Sum256(envelopePayload)
	record := attributeEventFileSpoolRecord{
		Version:   attributeEventFileSpoolRecordVersion,
		Identity:  envelope.Identity,
		Checksum:  hex.EncodeToString(checksum[:]),
		CreatedAt: now.UTC(),
		Envelope:  envelope,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return attributeEventFileSpoolRecord{}, nil, fmt.Errorf("marshal attribute/event spool record: %w", err)
	}
	return record, payload, nil
}

func attributeEventFileSpoolFilename(identity string) string {
	return identity + attributeEventFileSpoolExtension
}

func isAttributeEventFileSpoolTemp(name string) bool {
	return strings.HasPrefix(name, ".attribute-event-spool-") && strings.HasSuffix(name, ".tmp")
}

func isAttributeEventFileSpoolQuarantined(name string) bool {
	return strings.Contains(name, attributeEventFileSpoolExtension+attributeEventFileSpoolCorruptSuffix)
}

func isAttributeEventFileSpoolCommittedOrQuarantined(name string) bool {
	return strings.HasSuffix(name, attributeEventFileSpoolExtension) || isAttributeEventFileSpoolQuarantined(name)
}

func writeAttributeEventFileSpoolRecord(
	ctx context.Context,
	directory string,
	finalPath string,
	payload []byte,
) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	temp, err := os.CreateTemp(directory, ".attribute-event-spool-*.tmp")
	if err != nil {
		return fmt.Errorf("create attribute/event spool temp file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if err = os.Chmod(tempPath, 0o600); err != nil {
		return fmt.Errorf("restrict attribute/event spool temp file permissions: %w", err)
	}
	if _, err = temp.Write(payload); err != nil {
		return fmt.Errorf("write attribute/event spool temp file: %w", err)
	}
	if err = temp.Sync(); err != nil {
		return fmt.Errorf("sync attribute/event spool temp file: %w", err)
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("close attribute/event spool temp file: %w", err)
	}
	if err = os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("commit attribute/event spool record: %w", err)
	}
	if err = syncAttributeEventSpoolDirectory(directory); err != nil {
		return fmt.Errorf("sync attribute/event spool directory: %w", err)
	}
	// Once rename and directory sync have succeeded, later cancellation cannot
	// turn this durable outcome into a false failure.
	return nil
}

func syncAttributeEventSpoolDirectory(directory string) error {
	// Windows has no portable directory fsync through os.File.Sync. Production
	// Alpine executes the fsync below. Windows retains file fsync + atomic rename
	// and still needs runtime power-loss fault injection before that claim is made.
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *attributeEventFileSpool) replay(
	ctx context.Context,
	limit int,
	replay attributeEventFileSpoolReplayFunc,
) (attributeEventFileSpoolReplayResult, error) {
	result := attributeEventFileSpoolReplayResult{}
	if s == nil {
		return result, nil
	}
	if replay == nil {
		return result, fmt.Errorf("attribute/event spool replay callback is nil")
	}
	if limit < 1 {
		return result, fmt.Errorf("attribute/event spool replay limit must be positive")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.init(); err != nil {
		return result, err
	}

	s.replayMu.Lock()
	defer s.replayMu.Unlock()
	files, err := s.listReplayFiles()
	if err != nil {
		result.Usage = s.usage()
		return result, err
	}
	var replayErrors []error
	validAttempts := 0
	for _, file := range files {
		if validAttempts >= limit {
			break
		}
		if err := ctx.Err(); err != nil {
			replayErrors = append(replayErrors, err)
			break
		}
		result.Attempted++

		s.mu.Lock()
		info, inspectErr := os.Lstat(file.path)
		if inspectErr != nil {
			s.mu.Unlock()
			if errors.Is(inspectErr, os.ErrNotExist) {
				continue
			}
			replayErrors = append(replayErrors, fmt.Errorf("inspect attribute/event spool record %s: %w", file.name, inspectErr))
			break
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			s.mu.Unlock()
			replayErrors = append(replayErrors, fmt.Errorf("attribute/event spool replay record must be a regular file: %s", file.name))
			break
		}
		record, readErr := readAttributeEventFileSpoolRecord(file.path, attributeEventFileSpoolReadLimit, true)
		if readErr != nil {
			result.Corrupt++
			quarantined, quarantineErr := s.quarantineCommittedLocked(file.path)
			s.mu.Unlock()
			if quarantineErr != nil || !quarantined {
				replayErrors = append(replayErrors, fmt.Errorf(
					"read attribute/event spool record %s: %w; quarantine failed: %v",
					file.name,
					readErr,
					quarantineErr,
				))
				break
			}
			replayErrors = append(replayErrors, fmt.Errorf("read attribute/event spool record %s and quarantined it: %w", file.name, readErr))
			continue
		}
		currentSize := info.Size()
		s.mu.Unlock()

		validAttempts++
		if err := replay(ctx, record.Envelope); err != nil {
			replayErrors = append(replayErrors, fmt.Errorf("replay attribute/event spool record %s: %w", file.name, err))
			break
		}
		if err := s.removeCommitted(file.path, currentSize); err != nil {
			replayErrors = append(replayErrors, fmt.Errorf("remove replayed attribute/event spool record %s: %w", file.name, err))
			break
		}
		result.Replayed++
	}
	result.Usage = s.usage()
	return result, errors.Join(replayErrors...)
}

type attributeEventFileSpoolReplayFile struct {
	name string
	path string
}

func (s *attributeEventFileSpool) listReplayFiles() ([]attributeEventFileSpoolReplayFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return nil, fmt.Errorf("scan attribute/event spool for replay: %w", err)
	}
	files := make([]attributeEventFileSpoolReplayFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), attributeEventFileSpoolExtension) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("attribute/event spool record must not be a symlink: %s", entry.Name())
		}
		files = append(files, attributeEventFileSpoolReplayFile{
			name: entry.Name(),
			path: filepath.Join(s.directory, entry.Name()),
		})
	}
	// Identity filenames are content-derived and therefore give a stable order
	// across processes, restarts and filesystems whose mtimes have low precision.
	sort.Slice(files, func(left, right int) bool { return files[left].name < files[right].name })
	return files, nil
}

func readAttributeEventFileSpoolRecord(
	path string,
	maxRecordBytes int64,
	verifyFilename bool,
) (attributeEventFileSpoolRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, maxRecordBytes+1))
	if err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	if int64(len(payload)) > maxRecordBytes {
		return attributeEventFileSpoolRecord{}, fmt.Errorf("record exceeds %d byte safety limit", maxRecordBytes)
	}
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
	if verifyFilename && filepath.Base(path) != attributeEventFileSpoolFilename(record.Identity) {
		return attributeEventFileSpoolRecord{}, fmt.Errorf("record filename identity mismatch")
	}
	envelopePayload, err := json.Marshal(validated)
	if err != nil {
		return attributeEventFileSpoolRecord{}, err
	}
	checksum := sha256.Sum256(envelopePayload)
	if record.Checksum != hex.EncodeToString(checksum[:]) {
		return attributeEventFileSpoolRecord{}, fmt.Errorf("record checksum mismatch")
	}
	record.Envelope = validated
	return record, nil
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

func (s *attributeEventFileSpool) quarantineCommittedLocked(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	target, err := quarantineAttributeEventFile(path)
	if err != nil {
		return false, err
	}
	if target == "" {
		return false, fmt.Errorf("attribute/event spool quarantine target is empty")
	}
	s.quarantinedRecords++
	s.quarantinedBytes += info.Size()
	return true, errors.Join(s.refreshUsageLocked(), syncAttributeEventSpoolDirectory(s.directory))
}

func quarantineAttributeEventFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("attribute/event spool quarantine source must be a regular file")
	}
	target := path + attributeEventFileSpoolCorruptSuffix
	if _, inspectErr := os.Lstat(target); inspectErr == nil {
		target = fmt.Sprintf("%s.%d", target, time.Now().UTC().UnixNano())
	} else if !errors.Is(inspectErr, os.ErrNotExist) {
		return "", inspectErr
	}
	if err := os.Rename(path, target); err != nil {
		return "", err
	}
	return target, nil
}

func (s *attributeEventFileSpool) refreshUsageLocked() error {
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("scan attribute/event spool usage: %w", err)
	}
	var bytesUsed int64
	var records int
	var quarantinedBytes int64
	var quarantinedRecords int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isAttributeEventFileSpoolCommittedOrQuarantined(name) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("attribute/event spool record must not be a symlink: %s", name)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect attribute/event spool usage record: %w", err)
		}
		records++
		bytesUsed += info.Size()
		if isAttributeEventFileSpoolQuarantined(name) {
			quarantinedRecords++
			quarantinedBytes += info.Size()
		}
	}
	s.records = records
	s.bytes = bytesUsed
	s.quarantinedRecords = quarantinedRecords
	s.quarantinedBytes = quarantinedBytes
	return nil
}

func (s *attributeEventFileSpool) removeCommitted(path string, size int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(path); err != nil {
		return err
	}
	if s.records > 0 {
		s.records--
	}
	s.bytes -= size
	if s.bytes < 0 {
		s.bytes = 0
	}
	return syncAttributeEventSpoolDirectory(s.directory)
}

func (s *attributeEventFileSpool) usage() attributeEventFileSpoolUsage {
	if s == nil {
		return attributeEventFileSpoolUsage{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return attributeEventFileSpoolUsage{
		Records:            s.records,
		Bytes:              s.bytes,
		QuarantinedRecords: s.quarantinedRecords,
		QuarantinedBytes:   s.quarantinedBytes,
	}
}

func (s *attributeEventFileSpool) startupCorruptCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startupCorrupt
}
