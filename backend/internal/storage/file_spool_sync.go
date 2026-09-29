package storage

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

// writeFileSpoolTemp writes payload to a private temp file, fsyncs it, and
// atomically renames it over finalPath. The directory fsync is left to the
// caller so it can be grouped with other records.
func writeFileSpoolTemp(directory, prefix, label, finalPath string, payload []byte) (err error) {
	temp, err := os.CreateTemp(directory, prefix+"*.tmp")
	if err != nil {
		return fmt.Errorf("create %s spool temp file: %w", label, err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if err = os.Chmod(tempPath, 0o600); err != nil {
		return fmt.Errorf("restrict %s spool temp file permissions: %w", label, err)
	}
	if _, err = temp.Write(payload); err != nil {
		return fmt.Errorf("write %s spool temp file: %w", label, err)
	}
	if err = temp.Sync(); err != nil {
		return fmt.Errorf("sync %s spool temp file: %w", label, err)
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("close %s spool temp file: %w", label, err)
	}
	if err = os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("commit %s spool record: %w", label, err)
	}
	return nil
}

func syncFileSpoolDirectory(directory string) error {
	// Windows has no portable directory fsync through os.File.Sync. Production
	// Alpine executes the fsync below; Windows keeps file fsync + atomic rename
	// and needs runtime power-loss fault injection before that claim is made.
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

// fileSpoolDirSyncer coalesces directory fsyncs (group commit). sync returns
// only after a sync that BEGAN after the call has completed, so every rename
// performed before the call is covered by it. One caller leads, concurrent
// callers wait and share the leader's result.
type fileSpoolDirSyncer struct {
	mu        sync.Mutex
	cond      *sync.Cond
	running   bool
	started   uint64
	completed uint64
	lastErr   error
	// syncs counts executed fsyncs; tests use it to prove coalescing.
	syncs uint64
}

func (d *fileSpoolDirSyncer) sync(window time.Duration, fn func() error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cond == nil {
		d.cond = sync.NewCond(&d.mu)
	}
	// A sync numbered started+1 or later has not begun yet, so it covers us.
	need := d.started + 1
	for d.completed < need {
		if d.running {
			d.cond.Wait()
			continue
		}
		d.running = true
		if window > 0 {
			// Give concurrent writers a few milliseconds to join this commit.
			d.mu.Unlock()
			time.Sleep(window)
			d.mu.Lock()
		}
		d.started++
		generation := d.started
		d.mu.Unlock()
		err := safeFileSpoolSync(fn)
		d.mu.Lock()
		d.syncs++
		d.completed = generation
		d.lastErr = err
		d.running = false
		d.cond.Broadcast()
	}
	// The latest completed sync started after this call, so its outcome applies.
	return d.lastErr
}

func (d *fileSpoolDirSyncer) count() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.syncs
}

// safeFileSpoolSync converts a panicking sync into an error so a leader can
// never leave followers waiting forever.
func safeFileSpoolSync(fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.New(fmt.Sprint("spool directory sync panicked: ", recovered))
		}
	}()
	return fn()
}
