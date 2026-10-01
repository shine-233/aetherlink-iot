package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// recoverTempsLocked handles temps left behind by a crash. A crash can land
// after the complete record was file-fsynced but before the atomic rename, so
// a valid checksummed temp is promoted; incomplete or corrupt temps are removed.
func (s *fileSpool[T, C]) recoverTempsLocked() error {
	label := s.codec().label()
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("scan %s spool directory: %w", label, err)
	}
	changed := false
	for _, entry := range entries {
		if !s.isTemp(entry.Name()) {
			continue
		}
		path := filepath.Join(s.directory, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove %s spool temp symlink: %w", label, err)
			}
			continue
		}
		value, identity, readErr := s.readRecord(path, false)
		if readErr != nil {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove incomplete %s spool temp file: %w", label, err)
			}
			continue
		}
		finalPath := filepath.Join(s.directory, fileSpoolFilename(identity))
		if finalInfo, inspectErr := os.Lstat(finalPath); inspectErr == nil {
			if finalInfo.Mode()&os.ModeSymlink != 0 || !finalInfo.Mode().IsRegular() {
				return fmt.Errorf("recovered %s spool destination must be a regular file", label)
			}
			existing, _, existingErr := s.readRecord(finalPath, true)
			switch {
			case existingErr != nil:
				if _, err := quarantineFileSpoolFile(finalPath, label); err != nil {
					return fmt.Errorf("quarantine corrupt recovered %s spool destination: %w", label, err)
				}
				s.startupCorrupt++
			case !s.codec().equivalent(existing, value):
				return fmt.Errorf("%s spool deterministic identity collision during temp recovery", label)
			default:
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("remove duplicate %s spool temp file: %w", label, err)
				}
				continue
			}
		} else if !errors.Is(inspectErr, os.ErrNotExist) {
			return fmt.Errorf("inspect recovered %s spool destination: %w", label, inspectErr)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("restrict recovered %s spool temp file: %w", label, err)
		}
		if err := os.Rename(path, finalPath); err != nil {
			return fmt.Errorf("promote recoverable %s spool temp file: %w", label, err)
		}
		changed = true
	}
	if changed {
		if err := s.syncDirectoryNow(); err != nil {
			return fmt.Errorf("sync recovered %s spool record: %w", label, err)
		}
	}
	return nil
}

// quarantineCorruptCommittedLocked validates every committed record at init
// with a streamed directory scan (no whole-backlog name sort) and moves corrupt
// ones aside. Quarantined files stay counted by the refresh that follows.
func (s *fileSpool[T, C]) quarantineCorruptCommittedLocked() error {
	label := s.codec().label()
	changed := false
	err := scanFileSpoolDirectory(s.directory, func(entry os.DirEntry) error {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, fileSpoolExtension) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s spool record must not be a symlink: %s", label, name)
		}
		path := filepath.Join(s.directory, name)
		if _, _, readErr := s.readRecord(path, true); readErr == nil {
			return nil
		}
		if _, err := quarantineFileSpoolFile(path, label); err != nil {
			return fmt.Errorf("quarantine corrupt %s spool record: %w", label, err)
		}
		s.startupCorrupt++
		changed = true
		return nil
	})
	if err != nil {
		var visitErr *fileSpoolVisitError
		if errors.As(err, &visitErr) {
			return visitErr.err
		}
		return fmt.Errorf("scan %s spool records: %w", label, err)
	}
	if !changed {
		return nil
	}
	return s.syncDirectoryNow()
}

// fileSpoolPending is one record admitted by the store preflight: capacity is
// reserved and the identity is marked in flight until finish releases both.
type fileSpoolPending struct {
	index     int
	finalPath string
	identity  string
	payload   []byte
	written   bool
	err       error
}

// store durably writes one record. See storeBatch.
func (s *fileSpool[T, C]) store(ctx context.Context, value T, now time.Time) (fileSpoolStoreResult, error) {
	results, errs := s.storeBatch(ctx, []T{value}, now)
	return results[0], errs[0]
}

// storeBatch durably writes values with one grouped directory fsync. Each
// result/error pair describes the value at the same index; a value is durable
// (acknowledged) only when its error is nil, which is never before the
// directory fsync covering its rename has completed. Deterministic duplicates
// are Duplicate once a directory fsync proves the earlier rename is durable.
func (s *fileSpool[T, C]) storeBatch(ctx context.Context, values []T, now time.Time) ([]fileSpoolStoreResult, []error) {
	results := make([]fileSpoolStoreResult, len(values))
	errs := make([]error, len(values))
	if len(values) == 0 {
		return results, errs
	}
	fail := func(err error) ([]fileSpoolStoreResult, []error) {
		for index := range errs {
			if errs[index] == nil {
				errs[index] = err
			}
		}
		return results, errs
	}
	if s == nil {
		return fail(fmt.Errorf("%s file spool is disabled", s.codec().label()))
	}
	label := s.codec().label()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := s.init(); err != nil {
		return fail(err)
	}

	pending := make([]*fileSpoolPending, 0, len(values))
	batchIdentities := make(map[string]fileSpoolBatchEntry[T], len(values))
	var aliases []fileSpoolAlias
	needsDirSync := false
	for index, value := range values {
		prepared, identity, err := s.codec().prepare(value)
		if err != nil {
			errs[index] = err
			continue
		}
		payload, err := s.codec().encode(prepared, identity, now)
		if err != nil {
			errs[index] = err
			continue
		}
		if int64(len(payload)) > s.maxRecordBytes {
			errs[index] = fmt.Errorf("%s spool record exceeds %d byte configured limit", label, s.maxRecordBytes)
			continue
		}
		// A repeated identity inside one batch must not wait on its own
		// in-flight reservation; it shares the earlier item's outcome.
		if earlier, ok := batchIdentities[identity]; ok {
			if !s.codec().equivalent(earlier.value, prepared) {
				errs[index] = fmt.Errorf("%s spool deterministic identity collision inside one batch: identity=%s", label, identity)
				continue
			}
			aliases = append(aliases, fileSpoolAlias{index: index, of: earlier.index})
			continue
		}
		batchIdentities[identity] = fileSpoolBatchEntry[T]{index: index, value: prepared}
		item := &fileSpoolPending{
			index:     index,
			finalPath: filepath.Join(s.directory, fileSpoolFilename(identity)),
			identity:  identity,
			payload:   payload,
		}
		duplicate, err := s.admit(ctx, item, prepared, &results[index])
		if err != nil {
			errs[index] = err
			continue
		}
		if duplicate {
			needsDirSync = true
			continue
		}
		pending = append(pending, item)
	}

	// File I/O runs without the accounting mutex so concurrent stores overlap
	// their data fsyncs and share the grouped directory fsync below.
	for _, item := range pending {
		if err := writeFileSpoolTemp(s.directory, s.codec().tempPrefix(), label, item.finalPath, item.payload); err != nil {
			item.err = err
			continue
		}
		item.written = true
		needsDirSync = true
	}
	var dirErr error
	if needsDirSync {
		if err := s.commitDirectory(); err != nil {
			dirErr = fmt.Errorf("sync %s spool directory: %w", label, err)
		}
	}
	s.finish(pending, dirErr)
	// A corrupt predecessor quarantined by admit may have been resized by the
	// corruption; reconcile once per batch (rare path) so capacity is exact.
	// A failed scan saturates usage (fail-closed) but cannot un-durable the
	// records already committed, so it does not fail them.
	_ = s.reconcileUsage()
	if dirErr != nil {
		for index := range values {
			if errs[index] == nil && results[index].Duplicate {
				results[index].Duplicate = false
				errs[index] = fmt.Errorf("sync existing %s spool record: %w", label, dirErr)
			}
		}
	}
	for _, item := range pending {
		if item.err != nil {
			errs[item.index] = item.err
			continue
		}
		results[item.index].Stored = true
	}
	for _, alias := range aliases {
		// The later copy is a deterministic duplicate of the earlier one and is
		// durable exactly when the earlier one is.
		if errs[alias.of] != nil {
			errs[alias.index] = errs[alias.of]
			continue
		}
		results[alias.index].Duplicate = true
	}
	return results, errs
}

type fileSpoolBatchEntry[T any] struct {
	index int
	value T
}

type fileSpoolAlias struct {
	index int
	of    int
}

// admit is the locked preflight for one record: wait out an in-flight write of
// the same identity, detect duplicates, quarantine a corrupt predecessor, and
// reserve capacity. It returns duplicate=true for a healthy existing record.
func (s *fileSpool[T, C]) admit(ctx context.Context, item *fileSpoolPending, value T, result *fileSpoolStoreResult) (bool, error) {
	label := s.codec().label()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waitInflightLocked(item.identity)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if finalInfo, inspectErr := os.Lstat(item.finalPath); inspectErr == nil {
		if finalInfo.Mode()&os.ModeSymlink != 0 || !finalInfo.Mode().IsRegular() {
			return false, fmt.Errorf("%s spool destination must be a regular file", label)
		}
		existing, _, readErr := s.readRecord(item.finalPath, true)
		if readErr != nil {
			result.Corrupt++
			quarantined, quarantineErr := s.quarantineCommittedLocked(item.finalPath)
			if quarantined {
				result.Quarantined++
			}
			if quarantineErr != nil {
				return false, errors.Join(
					fmt.Errorf("existing %s spool record is corrupt: %w", label, readErr),
					fmt.Errorf("quarantine existing %s spool record: %w", label, quarantineErr),
				)
			}
			// Continue to the capacity check and write a valid replacement. The
			// quarantined evidence stays counted and is never auto-deleted. The
			// rename joins this batch's grouped directory fsync; if no replacement
			// is written, a crash merely re-detects and re-quarantines the file.
		} else {
			if !s.codec().equivalent(existing, value) {
				return false, fmt.Errorf(
					"%s spool deterministic identity collision: identity=%s already contains a different payload",
					label,
					item.identity,
				)
			}
			// First committed record wins, matching ON CONFLICT DO NOTHING.
			result.Duplicate = true
			return true, nil
		}
	} else if !errors.Is(inspectErr, os.ErrNotExist) {
		return false, fmt.Errorf("inspect %s spool record: %w", label, inspectErr)
	}
	usedRecords := s.records + s.reservedRecords
	usedBytes := s.bytes + s.reservedBytes
	if usedRecords+1 > s.maxRecords || usedBytes+int64(len(item.payload)) > s.maxBytes {
		return false, fmt.Errorf(
			"%s spool capacity exhausted: records=%d/%d bytes=%d/%d incoming_bytes=%d",
			label,
			usedRecords,
			s.maxRecords,
			usedBytes,
			s.maxBytes,
			len(item.payload),
		)
	}
	s.reservedRecords++
	s.reservedBytes += int64(len(item.payload))
	if s.inflight == nil {
		s.inflight = make(map[string]struct{})
	}
	s.inflight[item.identity] = struct{}{}
	return false, nil
}

func (s *fileSpool[T, C]) waitInflightLocked(identity string) {
	for {
		if _, busy := s.inflight[identity]; !busy {
			return
		}
		s.condLocked().Wait()
	}
}

// finish converts reservations into accounted usage and releases identities.
func (s *fileSpool[T, C]) finish(pending []*fileSpoolPending, dirErr error) {
	if len(pending) == 0 {
		return
	}
	label := s.codec().label()
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.condLocked().Broadcast()
	anyFailure := false
	for _, item := range pending {
		s.reservedRecords--
		s.reservedBytes -= int64(len(item.payload))
		delete(s.inflight, item.identity)
		if item.written && dirErr != nil {
			item.err = dirErr
		}
		if item.err != nil {
			anyFailure = true
			continue
		}
		s.records++
		s.bytes += int64(len(item.payload))
	}
	if !anyFailure {
		return
	}
	// A failure may happen before rename or after the final file became
	// visible. Re-scan so a visible record is never omitted from capacity
	// accounting; if that fails too, saturate to stay fail-closed.
	if usageErr := s.refreshUsageLocked(); usageErr != nil {
		s.records = s.maxRecords
		s.bytes = s.maxBytes
		for _, item := range pending {
			if item.err != nil {
				item.err = errors.Join(item.err, fmt.Errorf("refresh %s spool usage after failed write: %w", label, usageErr))
			}
		}
	}
}
