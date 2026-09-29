package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// replay hands up to limit healthy records to fn and deletes each one only
// after fn succeeded. Corrupt records are quarantined and do not count towards
// limit, so they cannot starve healthy backlog. The first fn error stops the
// pass: a database outage affects every remaining record.
func (s *fileSpool[T, C]) replay(
	ctx context.Context,
	limit int,
	fn func(context.Context, T) error,
) (fileSpoolReplayResult, error) {
	result := fileSpoolReplayResult{}
	if s == nil {
		return result, nil
	}
	label := s.codec().label()
	if fn == nil {
		return result, fmt.Errorf("%s spool replay callback is nil", label)
	}
	if limit < 1 {
		return result, fmt.Errorf("%s spool replay limit must be positive", label)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.init(); err != nil {
		return result, err
	}

	// Only one replay pass may claim files in this process. Store is not held
	// while the database is contacted, so an outage never blocks disk writes.
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

		// Read and, when necessary, quarantine under the store mutex so a
		// concurrent store cannot replace a corrupt file between the failed read
		// and the quarantine rename. In-flight writes of the same identity are
		// waited out so replay never observes a half-committed replacement.
		s.mu.Lock()
		identity := strings.TrimSuffix(file.name, fileSpoolExtension)
		s.waitInflightLocked(identity)
		info, inspectErr := os.Lstat(file.path)
		if inspectErr != nil {
			s.mu.Unlock()
			if errors.Is(inspectErr, os.ErrNotExist) {
				// Retired concurrently (write-ahead release or replay elsewhere).
				continue
			}
			replayErrors = append(replayErrors, fmt.Errorf("inspect %s spool record %s before replay: %w", label, file.name, inspectErr))
			break
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			s.mu.Unlock()
			replayErrors = append(replayErrors, fmt.Errorf("%s spool replay record must be a regular file: %s", label, file.name))
			break
		}
		value, _, readErr := s.readRecord(file.path, true)
		if readErr != nil {
			result.Corrupt++
			quarantined, quarantineErr := s.quarantineCommittedLocked(file.path)
			s.mu.Unlock()
			if quarantineErr != nil || !quarantined {
				replayErrors = append(replayErrors, fmt.Errorf(
					"read %s spool record %s: %w; quarantine failed: %v",
					label,
					file.name,
					readErr,
					quarantineErr,
				))
				break
			}
			replayErrors = append(replayErrors, fmt.Errorf("read %s spool record %s and quarantined it: %w", label, file.name, readErr))
			continue
		}
		s.mu.Unlock()

		validAttempts++
		if err := fn(ctx, value); err != nil {
			replayErrors = append(replayErrors, fmt.Errorf("replay %s spool record %s: %w", label, file.name, err))
			break
		}
		if err := s.removeCommitted(file.path); err != nil {
			// Safe: the file may remain or reappear after a crash, and the
			// database replay is idempotent.
			replayErrors = append(replayErrors, fmt.Errorf("remove replayed %s spool record %s: %w", label, file.name, err))
			break
		}
		result.Replayed++
	}
	result.Usage = s.usage()
	return result, errors.Join(replayErrors...)
}

func (s *fileSpool[T, C]) listReplayFiles() ([]fileSpoolReplayFile, error) {
	label := s.codec().label()
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return nil, fmt.Errorf("scan %s spool for replay: %w", label, err)
	}
	byModTime := s.codec().orderByModTime()
	files := make([]fileSpoolReplayFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), fileSpoolExtension) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s spool record must not be a symlink: %s", label, entry.Name())
		}
		file := fileSpoolReplayFile{
			name: entry.Name(),
			path: filepath.Join(s.directory, entry.Name()),
		}
		if byModTime {
			info, err := entry.Info()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("inspect %s spool replay record: %w", label, err)
			}
			file.modTime = info.ModTime()
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		if byModTime && !files[i].modTime.Equal(files[j].modTime) {
			return files[i].modTime.Before(files[j].modTime)
		}
		// Identity filenames are content-derived: a stable order across
		// processes and filesystems with low mtime precision.
		return files[i].name < files[j].name
	})
	return files, nil
}

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
	// A file may have been truncated externally while becoming corrupt; refresh
	// so the gauges report the bytes actually retained. Quarantined evidence is
	// never deleted automatically and keeps consuming capacity.
	return true, errors.Join(s.refreshUsageLocked(), s.syncDirectoryNow())
}

func quarantineFileSpoolFile(path, label string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s spool quarantine source must be a regular file", label)
	}
	target := path + fileSpoolCorruptSuffix
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

// refreshUsageLocked recomputes usage from the directory. Final files of
// identities still in flight are skipped: their writer accounts for them in
// finish, and counting them here as well would permanently over-count.
func (s *fileSpool[T, C]) refreshUsageLocked() error {
	label := s.codec().label()
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("scan %s spool usage: %w", label, err)
	}
	var bytesUsed int64
	var records int
	var quarantinedBytes int64
	var quarantinedRecords int
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isFileSpoolCommittedOrQuarantined(name) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s spool record must not be a symlink: %s", label, name)
		}
		quarantined := isFileSpoolQuarantined(name)
		if !quarantined {
			if _, busy := s.inflight[strings.TrimSuffix(name, fileSpoolExtension)]; busy {
				continue
			}
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("inspect %s spool usage record: %w", label, err)
		}
		records++
		bytesUsed += info.Size()
		if quarantined {
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

// removeCommitted unlinks a committed record and releases its accounted
// usage. The size is taken under the mutex so accounting always matches the
// file actually removed. Deletion durability is not required for safety
// (replay is idempotent), so the directory fsync joins the grouped commit
// outside the mutex.
func (s *fileSpool[T, C]) removeCommitted(path string) error {
	s.mu.Lock()
	s.waitInflightLocked(strings.TrimSuffix(filepath.Base(path), fileSpoolExtension))
	info, err := os.Lstat(path)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	if !info.Mode().IsRegular() {
		s.mu.Unlock()
		// Never unlink through a symlink or irregular entry; leave the anomaly
		// for the integrity/quarantine path to report.
		return fmt.Errorf("%s spool record is not a regular file", s.codec().label())
	}
	if err := os.Remove(path); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.records > 0 {
		s.records--
	}
	s.bytes -= info.Size()
	if s.bytes < 0 {
		s.bytes = 0
	}
	s.mu.Unlock()
	return s.commitDirectory()
}

// removeIdentity deletes the committed record of identity if still present.
// A missing file is not an error: replay may already have retired it.
func (s *fileSpool[T, C]) removeIdentity(identity string) error {
	if s == nil {
		return nil
	}
	err := s.removeCommitted(filepath.Join(s.directory, fileSpoolFilename(identity)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
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
