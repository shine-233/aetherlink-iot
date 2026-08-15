// telemetry_file_spool.go provides the bounded filesystem durability boundary
// used when PostgreSQL cannot retain a failed telemetry row. It owns atomic
// record writes, crash recovery, integrity checks, replay, and capacity rules.
package storage

import (
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
	telemetryFileSpoolRecordVersion = 1
	telemetryFileSpoolExtension     = ".json"
	telemetryFileSpoolCorruptSuffix = ".corrupt"
	// Read compatibility is deliberately independent from the current write
	// limit. Operators may lower max_record_bytes without corrupting/quarantining
	// records written under an older, larger limit.
	telemetryFileSpoolReadSafetyLimit int64 = 512 * 1024 * 1024
)

// telemetryFileSpool is a bounded, process-local durable fallback for
// telemetry that cannot be written to PostgreSQL. Each finalized JSON file is
// one history row. Store uses fsync + atomic rename; replay deletes a record
// only after the idempotent database transaction succeeds.
//
// Files contain raw telemetry values. The implementation requests 0700/0600
// permissions and never logs values, but filesystem permissions are not
// encryption. Operators must place the directory on encrypted persistent
// storage when telemetry values are sensitive.
type telemetryFileSpool struct {
	directory      string
	maxBytes       int64
	maxRecords     int
	maxRecordBytes int64

	mu          sync.Mutex
	replayMu    sync.Mutex
	initialized bool
	bytes       int64
	records     int
	// Quarantined files remain part of records/bytes because they still consume
	// the bounded spool capacity. Track the subset separately so operators can
	// distinguish replayable backlog from retained integrity evidence.
	quarantinedBytes   int64
	quarantinedRecords int
}

type telemetryFileSpoolRecord struct {
	Version   int           `json:"version"`
	Identity  string        `json:"identity"`
	Checksum  string        `json:"checksum"`
	CreatedAt time.Time     `json:"created_at"`
	History   TelemetryData `json:"history"`
}

type telemetryFileSpoolUsage struct {
	Records            int
	Bytes              int64
	QuarantinedRecords int
	QuarantinedBytes   int64
}

// telemetryFileSpoolStoreResult describes the durable side effects of one
// store attempt. A healthy deterministic duplicate is a successful durability
// outcome, but it is not a newly spooled record and must not increment the
// persisted-record counter. Corrupt reports integrity failures detected during
// this attempt, while Quarantined reports how many of those records were
// actually moved aside. Keeping them separate preserves the corruption signal
// even when a filesystem error prevents quarantine.
type telemetryFileSpoolStoreResult struct {
	Stored      bool
	Duplicate   bool
	Corrupt     int
	Quarantined int
}

type telemetryFileSpoolReplayResult struct {
	Attempted int
	Replayed  int
	Corrupt   int
	Usage     telemetryFileSpoolUsage
}

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
	}
}

func (s *telemetryFileSpool) init() error {
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
		return fmt.Errorf("create telemetry spool directory: %w", err)
	}
	info, err := os.Lstat(s.directory)
	if err != nil {
		return fmt.Errorf("inspect telemetry spool directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("telemetry spool path must be a real directory")
	}
	if err := os.Chmod(s.directory, 0o700); err != nil {
		return fmt.Errorf("restrict telemetry spool directory permissions: %w", err)
	}

	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("scan telemetry spool directory: %w", err)
	}
	var bytes int64
	var records int
	var quarantinedBytes int64
	var quarantinedRecords int
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(s.directory, name)
		if isTelemetryFileSpoolTemp(name) {
			if entry.Type()&os.ModeSymlink != 0 {
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("remove telemetry spool temp symlink: %w", err)
				}
				continue
			}
			// A crash can happen after the complete record was file-fsynced but
			// before atomic rename. Promote a valid checksummed temp record instead
			// of discarding it; only incomplete/corrupt temps are removed.
			record, readErr := readTelemetryFileSpoolRecord(path, telemetryFileSpoolReadSafetyLimit, false)
			if readErr != nil {
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("remove incomplete telemetry spool temp file: %w", err)
				}
				continue
			}
			finalPath := filepath.Join(s.directory, telemetryFileSpoolFilename(record.Identity))
			if finalInfo, err := os.Lstat(finalPath); err == nil {
				if finalInfo.Mode()&os.ModeSymlink != 0 || !finalInfo.Mode().IsRegular() {
					return fmt.Errorf("recovered telemetry spool destination must be a regular file")
				}
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("remove duplicate telemetry spool temp file: %w", err)
				}
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("inspect recovered telemetry spool destination: %w", err)
			}
			entryInfo, err := entry.Info()
			if err != nil {
				return fmt.Errorf("inspect recoverable telemetry spool temp file: %w", err)
			}
			if err := os.Rename(path, finalPath); err != nil {
				return fmt.Errorf("promote recoverable telemetry spool temp file: %w", err)
			}
			if err := syncTelemetryFileSpoolDirectory(s.directory); err != nil {
				return fmt.Errorf("sync recovered telemetry spool record: %w", err)
			}
			records++
			bytes += entryInfo.Size()
			continue
		}
		if entry.IsDir() || !isTelemetryFileSpoolCommittedOrQuarantined(name) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("telemetry spool record must not be a symlink: %s", name)
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect telemetry spool record: %w", err)
		}
		records++
		bytes += entryInfo.Size()
		if isTelemetryFileSpoolQuarantined(name) {
			quarantinedRecords++
			quarantinedBytes += entryInfo.Size()
		}
	}

	// Existing committed records are never discarded merely because an
	// operator lowered a limit. Startup must stay available to replay them;
	// store remains fail-closed until usage falls below both limits.
	s.records = records
	s.bytes = bytes
	s.quarantinedRecords = quarantinedRecords
	s.quarantinedBytes = quarantinedBytes
	s.initialized = true
	return nil
}

func (s *telemetryFileSpool) validateConfig() error {
	if s.directory == "" || s.directory == "." || filepath.Dir(s.directory) == s.directory {
		return fmt.Errorf("telemetry spool directory must be a dedicated non-root path")
	}
	spoolPath, err := filepath.Abs(s.directory)
	if err != nil {
		return fmt.Errorf("resolve telemetry spool directory: %w", err)
	}
	publicFilesPath, err := filepath.Abs("./files")
	if err != nil {
		return fmt.Errorf("resolve public files directory: %w", err)
	}
	if pathIsWithinDirectory(publicFilesPath, spoolPath) {
		return fmt.Errorf("telemetry spool directory must stay outside the public files directory")
	}
	if s.maxBytes < 1 {
		return fmt.Errorf("telemetry spool max bytes must be positive")
	}
	if s.maxRecords < 1 {
		return fmt.Errorf("telemetry spool max records must be positive")
	}
	if s.maxRecordBytes < 1 || s.maxRecordBytes > s.maxBytes || s.maxRecordBytes > telemetryFileSpoolReadSafetyLimit {
		return fmt.Errorf("telemetry spool max record bytes must be positive and no larger than max bytes or read safety limit")
	}
	return nil
}

func pathIsWithinDirectory(base, candidate string) bool {
	relative, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func (s *telemetryFileSpool) store(history TelemetryData, now time.Time) (telemetryFileSpoolStoreResult, error) {
	result := telemetryFileSpoolStoreResult{}
	if s == nil {
		return result, fmt.Errorf("telemetry file spool is disabled")
	}
	if !telemetryDataReplayable(history) {
		return result, fmt.Errorf("telemetry row is missing replay identity")
	}
	if err := s.init(); err != nil {
		return result, err
	}

	record, payload, err := buildTelemetryFileSpoolRecord(history, now)
	if err != nil {
		return result, err
	}
	if int64(len(payload)) > s.maxRecordBytes {
		return result, fmt.Errorf("telemetry spool record exceeds %d byte configured limit", s.maxRecordBytes)
	}
	filename := telemetryFileSpoolFilename(record.Identity)
	finalPath := filepath.Join(s.directory, filename)

	s.mu.Lock()
	defer s.mu.Unlock()
	if finalInfo, err := os.Lstat(finalPath); err == nil {
		if finalInfo.Mode()&os.ModeSymlink != 0 || !finalInfo.Mode().IsRegular() {
			return result, fmt.Errorf("telemetry spool destination must be a regular file")
		}
		existingRecord, readErr := readTelemetryFileSpoolRecord(finalPath, telemetryFileSpoolReadSafetyLimit, true)
		if readErr != nil {
			result.Corrupt++
			quarantined, quarantineErr := s.quarantineCommittedLocked(finalPath)
			if quarantined {
				result.Quarantined++
			}
			if quarantineErr != nil {
				return result, errors.Join(
					fmt.Errorf("existing telemetry spool record is corrupt: %w", readErr),
					fmt.Errorf("quarantine existing telemetry spool record: %w", quarantineErr),
				)
			}
			// Continue to the normal capacity check and write a valid replacement.
			// The quarantined evidence remains counted and is never auto-deleted.
		} else {
			if existingRecord.Checksum != record.Checksum {
				return result, fmt.Errorf(
					"telemetry spool identity collision: identity=%s already contains a different payload",
					record.Identity,
				)
			}
			// The database history key and spool identity are deterministic. Keeping
			// the first committed record matches PostgreSQL ON CONFLICT DO NOTHING.
			// Re-sync the directory before declaring it durable: a prior attempt may
			// have renamed this healthy file successfully but failed directory fsync.
			if syncErr := syncTelemetryFileSpoolDirectory(s.directory); syncErr != nil {
				return result, fmt.Errorf("sync existing telemetry spool record: %w", syncErr)
			}
			result.Duplicate = true
			return result, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("inspect telemetry spool record: %w", err)
	}
	if s.records+1 > s.maxRecords || s.bytes+int64(len(payload)) > s.maxBytes {
		return result, fmt.Errorf(
			"telemetry spool capacity exhausted: records=%d/%d bytes=%d/%d incoming_bytes=%d",
			s.records,
			s.maxRecords,
			s.bytes,
			s.maxBytes,
			len(payload),
		)
	}

	if err := writeTelemetryFileSpoolRecord(s.directory, finalPath, payload); err != nil {
		// A failure may happen either before rename or after the final file became
		// visible but directory fsync failed. Re-scan the bounded directory so a
		// visible record is never omitted from capacity accounting. If even that
		// scan fails, saturate both limits to keep subsequent writes fail-closed
		// until restart/operator recovery instead of under-counting disk usage.
		if usageErr := s.refreshUsageLocked(); usageErr != nil {
			s.records = s.maxRecords
			s.bytes = s.maxBytes
			return result, errors.Join(err, fmt.Errorf("refresh telemetry spool usage after failed write: %w", usageErr))
		}
		return result, err
	}
	s.records++
	s.bytes += int64(len(payload))
	result.Stored = true
	return result, nil
}

func buildTelemetryFileSpoolRecord(
	history TelemetryData,
	now time.Time,
) (telemetryFileSpoolRecord, []byte, error) {
	identity := telemetryFileSpoolIdentity(history)
	historyPayload, err := json.Marshal(history)
	if err != nil {
		return telemetryFileSpoolRecord{}, nil, fmt.Errorf("marshal telemetry spool history: %w", err)
	}
	checksum := sha256.Sum256(historyPayload)
	record := telemetryFileSpoolRecord{
		Version:   telemetryFileSpoolRecordVersion,
		Identity:  identity,
		Checksum:  hex.EncodeToString(checksum[:]),
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
	return identity + telemetryFileSpoolExtension
}

func isTelemetryFileSpoolTemp(name string) bool {
	return strings.HasPrefix(name, ".telemetry-spool-") && strings.HasSuffix(name, ".tmp")
}

func writeTelemetryFileSpoolRecord(directory, finalPath string, payload []byte) (err error) {
	temp, err := os.CreateTemp(directory, ".telemetry-spool-*.tmp")
	if err != nil {
		return fmt.Errorf("create telemetry spool temp file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if err = os.Chmod(tempPath, 0o600); err != nil {
		return fmt.Errorf("restrict telemetry spool temp file permissions: %w", err)
	}
	if _, err = temp.Write(payload); err != nil {
		return fmt.Errorf("write telemetry spool temp file: %w", err)
	}
	if err = temp.Sync(); err != nil {
		return fmt.Errorf("sync telemetry spool temp file: %w", err)
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("close telemetry spool temp file: %w", err)
	}
	if err = os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("commit telemetry spool record: %w", err)
	}
	if err = syncTelemetryFileSpoolDirectory(directory); err != nil {
		return fmt.Errorf("sync telemetry spool directory: %w", err)
	}
	return nil
}

func syncTelemetryFileSpoolDirectory(directory string) error {
	// Windows does not expose a portable directory fsync through os.File.Sync.
	// The production Alpine deployment executes the fsync path below; Windows
	// still gets file fsync plus atomic rename and must be covered by runtime
	// fault-injection before claiming power-loss proof there.
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

func (s *telemetryFileSpool) replay(
	ctx context.Context,
	limit int,
	replay telemetryFileSpoolReplayFunc,
) (telemetryFileSpoolReplayResult, error) {
	result := telemetryFileSpoolReplayResult{}
	if s == nil {
		return result, nil
	}
	if replay == nil {
		return result, fmt.Errorf("telemetry spool replay callback is nil")
	}
	if limit < 1 {
		return result, fmt.Errorf("telemetry spool replay limit must be positive")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.init(); err != nil {
		return result, err
	}

	// Only one replay pass may claim files in this process. Store is not held
	// while PostgreSQL is contacted, so an outage cannot block new disk writes.
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

		// Read and, when necessary, quarantine one deterministic filename while
		// holding the same mutex used by store. Without this critical section a
		// concurrent store could replace a corrupt file between our failed read
		// and quarantine call, causing replay to quarantine the new healthy file.
		// Capture the current size under the same lock as well; the directory scan
		// may have observed an older corrupt file that store has since replaced.
		s.mu.Lock()
		currentInfo, inspectErr := os.Lstat(file.path)
		if inspectErr != nil {
			s.mu.Unlock()
			replayErrors = append(replayErrors, fmt.Errorf("inspect telemetry spool record %s before replay: %w", file.name, inspectErr))
			break
		}
		if currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.Mode().IsRegular() {
			s.mu.Unlock()
			replayErrors = append(replayErrors, fmt.Errorf("telemetry spool replay record must be a regular file: %s", file.name))
			break
		}
		record, readErr := readTelemetryFileSpoolRecord(file.path, telemetryFileSpoolReadSafetyLimit, true)
		if readErr != nil {
			result.Corrupt++
			quarantined, quarantineErr := s.quarantineCommittedLocked(file.path)
			s.mu.Unlock()
			if quarantineErr != nil || !quarantined {
				replayErrors = append(replayErrors, fmt.Errorf(
					"read telemetry spool record %s: %w; quarantine failed: %v",
					file.name,
					readErr,
					quarantineErr,
				))
				break
			}
			replayErrors = append(replayErrors, fmt.Errorf("read telemetry spool record %s and quarantined it: %w", file.name, readErr))
			continue
		}
		currentSize := currentInfo.Size()
		s.mu.Unlock()

		validAttempts++
		if err := replay(ctx, record.History); err != nil {
			// A database outage generally affects every remaining file. Stop this
			// pass after the first database error to avoid an avoidable retry storm.
			replayErrors = append(replayErrors, fmt.Errorf("replay telemetry spool record %s: %w", file.name, err))
			break
		}
		if err := s.removeCommitted(file.path, currentSize); err != nil {
			// A delete or directory-fsync failure is safe: the file may remain or
			// reappear after a crash, and the database replay is idempotent.
			replayErrors = append(replayErrors, fmt.Errorf("remove replayed telemetry spool record %s: %w", file.name, err))
			break
		}
		result.Replayed++
	}
	result.Usage = s.usage()
	return result, errors.Join(replayErrors...)
}

type telemetryFileSpoolReplayFile struct {
	name    string
	path    string
	modTime time.Time
}

func (s *telemetryFileSpool) listReplayFiles() ([]telemetryFileSpoolReplayFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return nil, fmt.Errorf("scan telemetry spool for replay: %w", err)
	}
	files := make([]telemetryFileSpoolReplayFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), telemetryFileSpoolExtension) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("telemetry spool record must not be a symlink: %s", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect telemetry spool replay record: %w", err)
		}
		files = append(files, telemetryFileSpoolReplayFile{
			name:    entry.Name(),
			path:    filepath.Join(s.directory, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].name < files[j].name
		}
		return files[i].modTime.Before(files[j].modTime)
	})
	return files, nil
}

func isTelemetryFileSpoolCommittedOrQuarantined(name string) bool {
	return strings.HasSuffix(name, telemetryFileSpoolExtension) || isTelemetryFileSpoolQuarantined(name)
}

func isTelemetryFileSpoolQuarantined(name string) bool {
	return strings.Contains(name, telemetryFileSpoolExtension+telemetryFileSpoolCorruptSuffix)
}

func (s *telemetryFileSpool) quarantineCommittedLocked(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("telemetry spool quarantine source must be a regular file")
	}
	targetPath := path + telemetryFileSpoolCorruptSuffix
	if _, err := os.Lstat(targetPath); err == nil {
		targetPath = fmt.Sprintf("%s.%d", targetPath, time.Now().UTC().UnixNano())
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.Rename(path, targetPath); err != nil {
		return false, err
	}
	s.quarantinedRecords++
	s.quarantinedBytes += info.Size()
	// A file may have been externally truncated while becoming corrupt. Refresh
	// the logical totals after the rename so both the overall capacity gauges and
	// the quarantine subset report the bytes that are actually retained.
	usageErr := s.refreshUsageLocked()
	// Quarantined bytes remain part of capacity accounting and are never
	// deleted automatically. Operators can inspect/archive them without letting
	// corrupt oldest records starve healthy replay work.
	return true, errors.Join(usageErr, syncTelemetryFileSpoolDirectory(s.directory))
}

func (s *telemetryFileSpool) refreshUsageLocked() error {
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("scan telemetry spool usage: %w", err)
	}
	var bytes int64
	var records int
	var quarantinedBytes int64
	var quarantinedRecords int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isTelemetryFileSpoolCommittedOrQuarantined(name) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("telemetry spool record must not be a symlink: %s", name)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect telemetry spool usage record: %w", err)
		}
		records++
		bytes += info.Size()
		if isTelemetryFileSpoolQuarantined(name) {
			quarantinedRecords++
			quarantinedBytes += info.Size()
		}
	}
	s.records = records
	s.bytes = bytes
	s.quarantinedRecords = quarantinedRecords
	s.quarantinedBytes = quarantinedBytes
	return nil
}

func readTelemetryFileSpoolRecord(path string, maxRecordBytes int64, verifyFilename bool) (telemetryFileSpoolRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return telemetryFileSpoolRecord{}, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, maxRecordBytes+1))
	if err != nil {
		return telemetryFileSpoolRecord{}, err
	}
	if int64(len(payload)) > maxRecordBytes {
		return telemetryFileSpoolRecord{}, fmt.Errorf("record exceeds %d byte configured limit", maxRecordBytes)
	}
	var record telemetryFileSpoolRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return telemetryFileSpoolRecord{}, err
	}
	if record.Version != telemetryFileSpoolRecordVersion {
		return telemetryFileSpoolRecord{}, fmt.Errorf("unsupported record version %d", record.Version)
	}
	if !telemetryDataReplayable(record.History) {
		return telemetryFileSpoolRecord{}, fmt.Errorf("record is missing replay identity")
	}
	wantIdentity := telemetryFileSpoolIdentity(record.History)
	if record.Identity != wantIdentity || (verifyFilename && filepath.Base(path) != telemetryFileSpoolFilename(wantIdentity)) {
		return telemetryFileSpoolRecord{}, fmt.Errorf("record identity mismatch")
	}
	historyPayload, err := json.Marshal(record.History)
	if err != nil {
		return telemetryFileSpoolRecord{}, err
	}
	checksum := sha256.Sum256(historyPayload)
	if record.Checksum != hex.EncodeToString(checksum[:]) {
		return telemetryFileSpoolRecord{}, fmt.Errorf("record checksum mismatch")
	}
	return record, nil
}

func (s *telemetryFileSpool) removeCommitted(path string, size int64) error {
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
	return syncTelemetryFileSpoolDirectory(s.directory)
}

// removeWriteAheadReceipt deletes the record identified by history, if it is
// still present. It is used after a successful database flush to retire a
// write-ahead receipt. A missing file is not an error: background replay may
// have already committed and removed the same deterministic identity.
func (s *telemetryFileSpool) removeWriteAheadReceipt(history TelemetryData) error {
	if s == nil {
		return nil
	}
	if !telemetryDataReplayable(history) {
		return nil
	}
	path := filepath.Join(s.directory, telemetryFileSpoolFilename(telemetryFileSpoolIdentity(history)))
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.Mode().IsRegular() {
		// Never unlink through a symlink or irregular entry; leave the anomaly
		// in place so the existing integrity/quarantine path can report it.
		return fmt.Errorf("telemetry spool receipt is not a regular file")
	}
	if err := s.removeCommitted(path, info.Size()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func (s *telemetryFileSpool) usage() telemetryFileSpoolUsage {
	if s == nil {
		return telemetryFileSpoolUsage{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return telemetryFileSpoolUsage{
		Records:            s.records,
		Bytes:              s.bytes,
		QuarantinedRecords: s.quarantinedRecords,
		QuarantinedBytes:   s.quarantinedBytes,
	}
}
