package storage

import (
	"container/heap"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// fileSpoolScanChunk bounds how many directory entries are materialized at a
// time. os.ReadDir would allocate and name-sort the whole backlog first.
const fileSpoolScanChunk = 512

// fileSpoolVisitError marks an error produced by a scan visitor (a policy
// failure such as a symlinked record) as opposed to a directory I/O error, so
// callers keep their historical, distinct error messages.
type fileSpoolVisitError struct{ err error }

func (e *fileSpoolVisitError) Error() string { return e.err.Error() }
func (e *fileSpoolVisitError) Unwrap() error { return e.err }

// scanFileSpoolDirectory streams directory entries in unsorted chunks and
// stops at the first visitor error, which is returned as *fileSpoolVisitError.
func scanFileSpoolDirectory(directory string, visit func(os.DirEntry) error) error {
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		entries, readErr := dir.ReadDir(fileSpoolScanChunk)
		for _, entry := range entries {
			if err := visit(entry); err != nil {
				return &fileSpoolVisitError{err: err}
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		if len(entries) == 0 {
			return nil
		}
	}
}

// fileSpoolReplayBefore is the replay order: oldest mtime first when the codec
// asks for it, then the content-derived filename, which is stable across
// processes and filesystems with low mtime precision.
func fileSpoolReplayBefore(left, right fileSpoolReplayFile, byModTime bool) bool {
	if byModTime && !left.modTime.Equal(right.modTime) {
		return left.modTime.Before(right.modTime)
	}
	return left.name < right.name
}

// fileSpoolReplayHeap is a max-heap (the "newest" candidate at the root) so a
// bounded selection evicts the worst candidate in O(log limit).
type fileSpoolReplayHeap struct {
	files     []fileSpoolReplayFile
	byModTime bool
}

func (h *fileSpoolReplayHeap) Len() int { return len(h.files) }
func (h *fileSpoolReplayHeap) Less(i, j int) bool {
	return fileSpoolReplayBefore(h.files[j], h.files[i], h.byModTime)
}
func (h *fileSpoolReplayHeap) Swap(i, j int) { h.files[i], h.files[j] = h.files[j], h.files[i] }
func (h *fileSpoolReplayHeap) Push(x any)    { h.files = append(h.files, x.(fileSpoolReplayFile)) }
func (h *fileSpoolReplayHeap) Pop() any {
	last := h.files[len(h.files)-1]
	h.files = h.files[:len(h.files)-1]
	return last
}

// listReplayCandidates selects the first limit committed records in replay
// order (limit < 1 means all), skipping names in exclude. It costs one
// streamed directory scan, one stat per committed record only when ordering by
// mtime, and O(n log limit) ordering work. truncated reports that more
// eligible records exist beyond the selection.
//
// The store mutex is deliberately NOT held: a directory listing is only a
// candidate set, and every candidate is re-inspected under the mutex before it
// is read, replayed, quarantined, or removed. Concurrent stores therefore
// never stall behind a large backlog scan.
func (s *fileSpool[T, C]) listReplayCandidates(limit int, exclude map[string]struct{}) ([]fileSpoolReplayFile, bool, error) {
	label := s.codec().label()
	byModTime := s.codec().orderByModTime()
	selection := &fileSpoolReplayHeap{byModTime: byModTime}
	if limit > 0 {
		selection.files = make([]fileSpoolReplayFile, 0, limit)
	}
	truncated := false
	err := scanFileSpoolDirectory(s.directory, func(entry os.DirEntry) error {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, fileSpoolExtension) {
			return nil
		}
		if _, skip := exclude[name]; skip {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s spool record must not be a symlink: %s", label, name)
		}
		file := fileSpoolReplayFile{name: name}
		if byModTime {
			info, err := entry.Info()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return fmt.Errorf("inspect %s spool replay record: %w", label, err)
			}
			file.modTime = info.ModTime()
		}
		if limit < 1 || selection.Len() < limit {
			heap.Push(selection, file)
			return nil
		}
		truncated = true
		if fileSpoolReplayBefore(file, selection.files[0], byModTime) {
			selection.files[0] = file
			heap.Fix(selection, 0)
		}
		return nil
	})
	if err != nil {
		var visitErr *fileSpoolVisitError
		if errors.As(err, &visitErr) {
			return nil, false, visitErr.err
		}
		return nil, false, fmt.Errorf("scan %s spool for replay: %w", label, err)
	}
	// Popping a max-heap yields newest first; fill from the back.
	files := make([]fileSpoolReplayFile, selection.Len())
	for index := len(files) - 1; index >= 0; index-- {
		file := heap.Pop(selection).(fileSpoolReplayFile)
		file.path = filepath.Join(s.directory, file.name)
		files[index] = file
	}
	return files, truncated, nil
}
