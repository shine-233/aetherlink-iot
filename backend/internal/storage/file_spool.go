// file_spool.go is the single bounded filesystem durability boundary shared by
// the telemetry and attribute/event spools. It owns atomic record writes,
// file and directory fsync, crash recovery of interrupted temp files,
// integrity quarantine, replay, and capacity accounting. Record schemas live in
// small codecs (telemetry_file_spool.go, attribute_event_file_spool.go), so the
// on-disk format of each spool is unchanged: one JSON file per record named
// "<identity>.json", temp files ".<prefix>-*.tmp", quarantine "*.json.corrupt*".
//
// Durability and group commit: a record is acknowledged only after its temp
// file was fsynced, atomically renamed, and a directory fsync that STARTED
// after the rename completed. Directory fsyncs are coalesced across concurrent
// callers (leader/follower) and across all records of one storeBatch call, so N
// records cost N file fsyncs plus roughly one directory fsync instead of 2N.
// File I/O runs outside the accounting mutex; capacity is reserved up front so
// concurrent writers can never overshoot the configured bounds.
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	fileSpoolExtension     = ".json"
	fileSpoolCorruptSuffix = ".corrupt"
	// Read compatibility is deliberately independent from the current write
	// limit. Operators may lower max_record_bytes without corrupting or
	// quarantining records written under an older, larger limit.
	fileSpoolReadSafetyLimit int64 = 512 * 1024 * 1024
)

// fileSpoolCodec is the record schema of one spool. Implementations are
// zero-size value types so a spool literal works without wiring.
type fileSpoolCodec[T any] interface {
	// label names the spool in errors ("telemetry", "attribute/event").
	label() string
	// tempPrefix is the temp-file prefix (".telemetry-spool-").
	tempPrefix() string
	// prepare validates/normalizes a value and returns its deterministic identity.
	prepare(value T) (T, string, error)
	// encode renders the complete on-disk record.
	encode(value T, identity string, now time.Time) ([]byte, error)
	// decode parses and integrity-checks a record, returning value and identity.
	decode(payload []byte) (T, string, error)
	// equivalent reports whether an existing durable record satisfies incoming.
	equivalent(existing, incoming T) bool
	// validateOnInit quarantines corrupt committed records during startup.
	validateOnInit() bool
	// orderByModTime replays oldest-mtime first instead of by filename.
	orderByModTime() bool
}

type fileSpoolUsage struct {
	Records            int
	Bytes              int64
	QuarantinedRecords int
	QuarantinedBytes   int64
}

// fileSpoolStoreResult describes the durable side effects of one store. A
// healthy deterministic duplicate is a successful durability outcome but not a
// newly spooled record. Corrupt counts integrity failures detected during the
// attempt; Quarantined counts how many were actually moved aside.
type fileSpoolStoreResult struct {
	Stored      bool
	Duplicate   bool
	Corrupt     int
	Quarantined int
}

type fileSpoolReplayResult struct {
	Attempted int
	Replayed  int
	Corrupt   int
	Usage     fileSpoolUsage
}

type fileSpoolReplayFile struct {
	name    string
	path    string
	modTime time.Time
}

// fileSpool is a bounded, process-local durable fallback. Files contain raw
// values; 0700/0600 permissions are requested but are not encryption, so the
// directory belongs on encrypted persistent storage for sensitive data.
type fileSpool[T any, C fileSpoolCodec[T]] struct {
	directory      string
	maxBytes       int64
	maxRecords     int
	maxRecordBytes int64
	// independentOf lists directories this spool must neither contain nor sit in.
	independentOf []string
	// syncDir overrides the directory fsync (tests); nil uses the platform one.
	syncDir func(string) error
	// commitWindow optionally delays a group-commit leader to gather more
	// followers. Zero keeps natural (latency-free) coalescing only.
	commitWindow time.Duration

	mu       sync.Mutex
	cond     *sync.Cond
	replayMu sync.Mutex
	syncer   fileSpoolDirSyncer

	initialized bool
	bytes       int64
	records     int
	// Quarantined files remain part of records/bytes because they still consume
	// bounded capacity; the subset is tracked separately for operators.
	quarantinedBytes   int64
	quarantinedRecords int
	startupCorrupt     int
	reservedBytes      int64
	reservedRecords    int
	inflight           map[string]struct{}
}

func (s *fileSpool[T, C]) codec() C {
	var codec C
	return codec
}

func (s *fileSpool[T, C]) condLocked() *sync.Cond {
	if s.cond == nil {
		s.cond = sync.NewCond(&s.mu)
	}
	return s.cond
}

func (s *fileSpool[T, C]) syncDirectoryNow() error {
	if s.syncDir != nil {
		return s.syncDir(s.directory)
	}
	return syncFileSpoolDirectory(s.directory)
}

// commitDirectory joins the next group directory fsync and returns once a sync
// that started after this call has completed.
func (s *fileSpool[T, C]) commitDirectory() error {
	return s.syncer.sync(s.commitWindow, s.syncDirectoryNow)
}

func (s *fileSpool[T, C]) init() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialized {
		return nil
	}
	label := s.codec().label()
	if err := s.validateConfig(); err != nil {
		return err
	}
	if err := os.MkdirAll(s.directory, 0o700); err != nil {
		return fmt.Errorf("create %s spool directory: %w", label, err)
	}
	info, err := os.Lstat(s.directory)
	if err != nil {
		return fmt.Errorf("inspect %s spool directory: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s spool path must be a real directory", label)
	}
	if err := os.Chmod(s.directory, 0o700); err != nil {
		return fmt.Errorf("restrict %s spool directory permissions: %w", label, err)
	}
	if err := s.recoverTempsLocked(); err != nil {
		return err
	}
	if s.codec().validateOnInit() {
		if err := s.quarantineCorruptCommittedLocked(); err != nil {
			return err
		}
	}
	// Existing committed records are never discarded because an operator
	// lowered a limit. Startup stays available to replay them; store remains
	// fail-closed until usage falls below both limits.
	if err := s.refreshUsageLocked(); err != nil {
		return err
	}
	s.initialized = true
	return nil
}

func (s *fileSpool[T, C]) validateConfig() error {
	label := s.codec().label()
	if s.directory == "" || s.directory == "." || filepath.Dir(s.directory) == s.directory {
		return fmt.Errorf("%s spool directory must be a dedicated non-root path", label)
	}
	spoolPath, err := filepath.Abs(s.directory)
	if err != nil {
		return fmt.Errorf("resolve %s spool directory: %w", label, err)
	}
	publicFilesPath, err := filepath.Abs("./files")
	if err != nil {
		return fmt.Errorf("resolve public files directory: %w", err)
	}
	if pathIsWithinDirectory(publicFilesPath, spoolPath) {
		return fmt.Errorf("%s spool directory must stay outside the public files directory", label)
	}
	for _, other := range s.independentOf {
		if other == "" || other == "." {
			continue
		}
		otherPath, err := filepath.Abs(other)
		if err != nil {
			return fmt.Errorf("resolve peer spool directory: %w", err)
		}
		if pathIsWithinDirectory(otherPath, spoolPath) || pathIsWithinDirectory(spoolPath, otherPath) {
			return fmt.Errorf("%s spool directory must be independent from the telemetry spool directory", label)
		}
	}
	if s.maxBytes < 1 {
		return fmt.Errorf("%s spool max bytes must be positive", label)
	}
	if s.maxRecords < 1 {
		return fmt.Errorf("%s spool max records must be positive", label)
	}
	if s.maxRecordBytes < 1 || s.maxRecordBytes > s.maxBytes || s.maxRecordBytes > fileSpoolReadSafetyLimit {
		return fmt.Errorf("%s spool max record bytes must be positive and no larger than max bytes or read safety limit", label)
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

func (s *fileSpool[T, C]) isTemp(name string) bool {
	return strings.HasPrefix(name, s.codec().tempPrefix()) && strings.HasSuffix(name, ".tmp")
}

func isFileSpoolQuarantined(name string) bool {
	return strings.Contains(name, fileSpoolExtension+fileSpoolCorruptSuffix)
}

func isFileSpoolCommittedOrQuarantined(name string) bool {
	return strings.HasSuffix(name, fileSpoolExtension) || isFileSpoolQuarantined(name)
}

func fileSpoolFilename(identity string) string {
	return identity + fileSpoolExtension
}

// readRecord reads and integrity-checks one record file.
func (s *fileSpool[T, C]) readRecord(path string, verifyFilename bool) (T, string, error) {
	var zero T
	payload, err := readFileSpoolPayload(path, fileSpoolReadSafetyLimit)
	if err != nil {
		return zero, "", err
	}
	value, identity, err := s.codec().decode(payload)
	if err != nil {
		return zero, "", err
	}
	if verifyFilename && filepath.Base(path) != fileSpoolFilename(identity) {
		return zero, "", fmt.Errorf("record filename identity mismatch")
	}
	return value, identity, nil
}

func readFileSpoolPayload(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, fmt.Errorf("record exceeds %d byte safety limit", limit)
	}
	return payload, nil
}
