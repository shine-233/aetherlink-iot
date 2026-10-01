package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Replay passes share one claim loop (claimReplayRecords). Per pass the cost is
// a bounded directory scan (listReplayCandidates, O(n log limit)), one Lstat +
// read per claimed record under s.mu, and ONE trailing directory fsync that
// covers every removal and quarantine rename of the pass. Corrupt records are
// quarantined and never count towards limit, so they cannot starve healthy
// backlog.

// fileSpoolReplayRecord is one healthy record claimed by a replay pass.
type fileSpoolReplayRecord[T any] struct {
	value T
	name  string
}

// fileSpoolReplayPass collects the side effects that are settled once at the
// end of a pass: the grouped directory fsync and the usage reconciliation.
type fileSpoolReplayPass struct {
	result   fileSpoolReplayResult
	errs     []error
	dirDirty bool
}

// replay hands up to limit healthy records to fn one at a time and deletes each
// one only after fn succeeded. The first fn error stops the pass: a database
// outage affects every remaining record.
func (s *fileSpool[T, C]) replay(
	ctx context.Context,
	limit int,
	fn func(context.Context, T) error,
) (fileSpoolReplayResult, error) {
	if s == nil {
		return fileSpoolReplayResult{}, nil
	}
	ctx, err := s.beginReplay(ctx, limit, fn != nil)
	if err != nil {
		return fileSpoolReplayResult{}, err
	}
	s.replayMu.Lock()
	defer s.replayMu.Unlock()

	label := s.codec().label()
	pass := &fileSpoolReplayPass{}
	s.claimReplayRecords(ctx, limit, pass, func(record fileSpoolReplayRecord[T]) error {
		if err := fn(ctx, record.value); err != nil {
			return fmt.Errorf("replay %s spool record %s: %w", label, record.name, err)
		}
		removed, removeErrs := s.retireNames([]string{record.name})
		if len(removeErrs) > 0 {
			// Safe: the file may remain or reappear after a crash, and the
			// database replay is idempotent.
			return fmt.Errorf("remove replayed %s spool record %s: %w", label, record.name, errors.Join(removeErrs...))
		}
		if removed > 0 {
			pass.dirDirty = true
		}
		pass.result.Replayed++
		return nil
	})
	return s.endReplay(pass)
}

// replayBatch claims up to limit healthy records, hands them to fn in ONE call
// (one database transaction), and on success retires all of them with a single
// directory fsync. On fn failure nothing is removed, so the whole batch is
// retried on the next pass; the database side must be idempotent, as for
// replay.
func (s *fileSpool[T, C]) replayBatch(
	ctx context.Context,
	limit int,
	fn func(context.Context, []T) error,
) (fileSpoolReplayResult, error) {
	if s == nil {
		return fileSpoolReplayResult{}, nil
	}
	ctx, err := s.beginReplay(ctx, limit, fn != nil)
	if err != nil {
		return fileSpoolReplayResult{}, err
	}
	s.replayMu.Lock()
	defer s.replayMu.Unlock()

	label := s.codec().label()
	pass := &fileSpoolReplayPass{}
	records := make([]fileSpoolReplayRecord[T], 0, min(limit, 1024))
	s.claimReplayRecords(ctx, limit, pass, func(record fileSpoolReplayRecord[T]) error {
		records = append(records, record)
		return nil
	})
	if len(records) == 0 {
		return s.endReplay(pass)
	}
	if err := ctx.Err(); err != nil {
		pass.errs = append(pass.errs, err)
		return s.endReplay(pass)
	}
	values := make([]T, len(records))
	names := make([]string, len(records))
	for index, record := range records {
		values[index] = record.value
		names[index] = record.name
	}
	if err := fn(ctx, values); err != nil {
		pass.errs = append(pass.errs, fmt.Errorf("replay %d %s spool records: %w", len(records), label, err))
		return s.endReplay(pass)
	}
	// Records retired concurrently between claim and removal (write-ahead
	// release) were still replayed; only real unlink failures are errors.
	removed, removeErrs := s.retireNames(names)
	if removed > 0 {
		pass.dirDirty = true
	}
	if len(removeErrs) > 0 {
		pass.errs = append(pass.errs, fmt.Errorf("remove replayed %s spool records: %w", label, errors.Join(removeErrs...)))
	}
	pass.result.Replayed = len(records)
	return s.endReplay(pass)
}

func (s *fileSpool[T, C]) beginReplay(ctx context.Context, limit int, haveCallback bool) (context.Context, error) {
	label := s.codec().label()
	if !haveCallback {
		return ctx, fmt.Errorf("%s spool replay callback is nil", label)
	}
	if limit < 1 {
		return ctx, fmt.Errorf("%s spool replay limit must be positive", label)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return ctx, s.init()
}

// endReplay settles the pass: one grouped directory fsync for every removal
// and quarantine rename, then at most one usage reconciliation scan.
func (s *fileSpool[T, C]) endReplay(pass *fileSpoolReplayPass) (fileSpoolReplayResult, error) {
	label := s.codec().label()
	if pass.dirDirty {
		if err := s.commitDirectory(); err != nil {
			pass.errs = append(pass.errs, fmt.Errorf("sync %s spool directory after replay: %w", label, err))
		}
	}
	if err := s.reconcileUsage(); err != nil {
		pass.errs = append(pass.errs, err)
	}
	pass.result.Usage = s.usage()
	return pass.result, errors.Join(pass.errs...)
}

// claimReplayRecords walks committed records in replay order and hands up to
// limit healthy ones to visit. A visit error stops the pass. Selection runs in
// rounds: each round lists only as many candidates as healthy records are still
// needed, and a further round runs only when corrupt or concurrently retired
// candidates left the quota unfilled and the listing was truncated. Names seen
// in earlier rounds are excluded, so records that stay on disk until the end of
// the pass (replayBatch) are never claimed twice and every round makes
// progress.
func (s *fileSpool[T, C]) claimReplayRecords(
	ctx context.Context,
	limit int,
	pass *fileSpoolReplayPass,
	visit func(fileSpoolReplayRecord[T]) error,
) {
	seen := make(map[string]struct{}, min(limit, 1024))
	valid := 0
	for valid < limit {
		candidates, truncated, err := s.listReplayCandidates(limit-valid, seen)
		if err != nil {
			pass.errs = append(pass.errs, err)
			return
		}
		for _, file := range candidates {
			if err := ctx.Err(); err != nil {
				pass.errs = append(pass.errs, err)
				return
			}
			seen[file.name] = struct{}{}
			pass.result.Attempted++
			record, healthy, stop := s.claimReplayFile(file, pass)
			if stop {
				return
			}
			if !healthy {
				continue
			}
			valid++
			if err := visit(record); err != nil {
				pass.errs = append(pass.errs, err)
				return
			}
		}
		if !truncated || len(candidates) == 0 {
			return
		}
	}
}

// claimReplayFile reads one candidate under the store mutex so a concurrent
// store cannot replace a corrupt file between the failed read and the
// quarantine rename. In-flight writes of the same identity are waited out so
// replay never observes a half-committed replacement. healthy=false with
// stop=false means skip (retired concurrently, or quarantined).
func (s *fileSpool[T, C]) claimReplayFile(
	file fileSpoolReplayFile,
	pass *fileSpoolReplayPass,
) (record fileSpoolReplayRecord[T], healthy bool, stop bool) {
	label := s.codec().label()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waitInflightLocked(strings.TrimSuffix(file.name, fileSpoolExtension))
	info, err := os.Lstat(file.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Retired concurrently (write-ahead release or another pass).
			return record, false, false
		}
		pass.errs = append(pass.errs, fmt.Errorf("inspect %s spool record %s before replay: %w", label, file.name, err))
		return record, false, true
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		pass.errs = append(pass.errs, fmt.Errorf("%s spool replay record must be a regular file: %s", label, file.name))
		return record, false, true
	}
	value, _, readErr := s.readRecord(file.path, true)
	if readErr == nil {
		return fileSpoolReplayRecord[T]{value: value, name: file.name}, true, false
	}
	pass.result.Corrupt++
	quarantined, quarantineErr := s.quarantineCommittedLocked(file.path)
	if quarantineErr != nil || !quarantined {
		pass.errs = append(pass.errs, fmt.Errorf(
			"read %s spool record %s: %w; quarantine failed: %v",
			label, file.name, readErr, quarantineErr,
		))
		return record, false, true
	}
	pass.dirDirty = true
	pass.errs = append(pass.errs, fmt.Errorf("read %s spool record %s and quarantined it: %w", label, file.name, readErr))
	return record, false, false
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
