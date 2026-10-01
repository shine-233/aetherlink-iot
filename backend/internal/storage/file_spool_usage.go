package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Capacity accounting is incremental: store adds the bytes it wrote, removal
// subtracts the size Lstat reported under the mutex, and quarantine moves a
// record into the quarantined subset without touching the totals. The only
// full directory scan (refreshUsageLocked) runs at init, after a failed write,
// and as a reconciliation fallback when the incremental state is known to be
// inexact (usageDirty).

// fileSpoolRemoveChunk bounds how many unlinks run per s.mu critical section
// so a huge receipt release never stalls concurrent stores for long.
const fileSpoolRemoveChunk = 256

// refreshUsageLocked recomputes usage from the directory. Final files of
// identities still in flight are skipped: their writer accounts for them in
// finish, and counting them here as well would permanently over-count.
func (s *fileSpool[T, C]) refreshUsageLocked() error {
	label := s.codec().label()
	var bytesUsed int64
	var records int
	var quarantinedBytes int64
	var quarantinedRecords int
	err := scanFileSpoolDirectory(s.directory, func(entry os.DirEntry) error {
		name := entry.Name()
		if entry.IsDir() || !isFileSpoolCommittedOrQuarantined(name) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s spool record must not be a symlink: %s", label, name)
		}
		quarantined := isFileSpoolQuarantined(name)
		if !quarantined {
			if _, busy := s.inflight[strings.TrimSuffix(name, fileSpoolExtension)]; busy {
				return nil
			}
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("inspect %s spool usage record: %w", label, err)
		}
		records++
		bytesUsed += info.Size()
		if quarantined {
			quarantinedRecords++
			quarantinedBytes += info.Size()
		}
		return nil
	})
	if err != nil {
		var visitErr *fileSpoolVisitError
		if errors.As(err, &visitErr) {
			return visitErr.err
		}
		return fmt.Errorf("scan %s spool usage: %w", label, err)
	}
	s.records = records
	s.bytes = bytesUsed
	s.quarantinedRecords = quarantinedRecords
	s.quarantinedBytes = quarantinedBytes
	s.usageDirty = false
	return nil
}

// reconcileUsage runs the fallback full scan once if incremental accounting
// was marked inexact. Callers invoke it at the end of a replay pass, never per
// record. A failed scan saturates usage so store stays fail-closed.
func (s *fileSpool[T, C]) reconcileUsage() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.usageDirty {
		return nil
	}
	if err := s.refreshUsageLocked(); err != nil {
		s.records = max(s.records, s.maxRecords)
		s.bytes = max(s.bytes, s.maxBytes)
		return fmt.Errorf("reconcile %s spool usage: %w", s.codec().label(), err)
	}
	return nil
}

// quarantineCommittedLocked moves a corrupt committed record aside. The file
// keeps consuming bounded capacity, so only the quarantined subset changes,
// using the size of the Lstat taken here. The size this record was accounted
// with when it was written is unknown (corruption may have resized it), so
// usage is marked for one reconciliation scan later instead of rescanning the
// whole directory now while holding s.mu.
//
// No directory fsync runs here: the caller groups it (replay ends its pass
// with one commitDirectory, store joins the batch commit). Losing the rename
// in a crash only means the corrupt file is detected and quarantined again.
func (s *fileSpool[T, C]) quarantineCommittedLocked(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if _, err := quarantineFileSpoolFile(path, s.codec().label()); err != nil {
		return false, err
	}
	s.quarantinedRecords++
	s.quarantinedBytes += info.Size()
	s.usageDirty = true
	return true, nil
}

// unlinkCommittedLocked removes the committed record at path and releases its
// accounted usage. It reports removed=false with os.ErrNotExist when the file
// is already gone. Never unlinks through a symlink or irregular entry; the
// anomaly is left for the integrity/quarantine path to report.
func (s *fileSpool[T, C]) unlinkCommittedLocked(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%s spool record is not a regular file", s.codec().label())
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	s.records--
	s.bytes -= info.Size()
	if s.records < 0 || s.bytes < 0 {
		// The directory changed behind our back. Never let a negative counter
		// widen capacity; rescan now (rare path) to restore exact usage.
		s.usageDirty = true
		if refreshErr := s.refreshUsageLocked(); refreshErr != nil {
			s.records = max(s.records, 0)
			s.bytes = max(s.bytes, 0)
		}
	}
	return true, nil
}

// removeIdentities retires the committed records of identities (write-ahead
// receipts or replayed rows) with ONE grouped directory fsync for the whole set
// instead of one per record. Missing files are not errors. Per-identity
// failures are joined and do not stop the rest.
//
// Deletion durability is not required for safety (replay is idempotent), but
// the trailing fsync keeps the directory metadata promptly bounded.
func (s *fileSpool[T, C]) removeIdentities(identities []string) error {
	if s == nil || len(identities) == 0 {
		return nil
	}
	label := s.codec().label()
	var removeErrors []error
	names := make([]string, 0, len(identities))
	for _, identity := range identities {
		if !validFileSpoolIdentity(identity) {
			removeErrors = append(removeErrors, fmt.Errorf("%s spool identity is invalid: %q", label, identity))
			continue
		}
		names = append(names, fileSpoolFilename(identity))
	}
	removed, retireErrors := s.retireNames(names)
	removeErrors = append(removeErrors, retireErrors...)
	if removed > 0 {
		if err := s.commitDirectory(); err != nil {
			removeErrors = append(removeErrors, fmt.Errorf("sync %s spool directory after removal: %w", label, err))
		}
	}
	return errors.Join(removeErrors...)
}

// retireNames unlinks committed record files (base names inside the spool
// directory) without any directory fsync; callers group exactly one. In-flight
// writes of the same identity are waited out so a fresh rename is never
// unlinked half way. Unlinks run in bounded chunks under s.mu so concurrent
// stores interleave. Already-missing files are skipped silently.
func (s *fileSpool[T, C]) retireNames(names []string) (int, []error) {
	label := s.codec().label()
	var retireErrors []error
	removed := 0
	for start := 0; start < len(names); start += fileSpoolRemoveChunk {
		end := min(start+fileSpoolRemoveChunk, len(names))
		s.mu.Lock()
		for _, name := range names[start:end] {
			s.waitInflightLocked(strings.TrimSuffix(name, fileSpoolExtension))
			ok, err := s.unlinkCommittedLocked(filepath.Join(s.directory, name))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				retireErrors = append(retireErrors, fmt.Errorf("remove %s spool record %s: %w", label, name, err))
			}
			if ok {
				removed++
			}
		}
		s.mu.Unlock()
	}
	return removed, retireErrors
}

// validFileSpoolIdentity rejects identities that could escape the spool
// directory or alias a temp/quarantine name.
func validFileSpoolIdentity(identity string) bool {
	return identity != "" &&
		identity != "." && identity != ".." &&
		!strings.ContainsAny(identity, `/\`) &&
		!strings.HasPrefix(identity, ".") &&
		!strings.Contains(identity, fileSpoolExtension)
}

func (s *fileSpool[T, C]) usage() fileSpoolUsage {
	if s == nil {
		return fileSpoolUsage{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fileSpoolUsage{
		Records:            s.records,
		Bytes:              s.bytes,
		QuarantinedRecords: s.quarantinedRecords,
		QuarantinedBytes:   s.quarantinedBytes,
	}
}

func (s *fileSpool[T, C]) startupCorruptCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startupCorrupt
}
