package initialize

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
)

// withDeviceCacheLoader swaps the miss loader and forces every Redis read to
// miss (nil client), so GetDeviceCacheById always takes the fallback path.
func withDeviceCacheLoader(t *testing.T, load func(string) (*model.Device, error)) {
	t.Helper()
	prevLoader, prevRedis := deviceCacheLoader, global.REDIS
	deviceCacheLoader = load
	global.REDIS = nil
	t.Cleanup(func() {
		deviceCacheLoader = prevLoader
		global.REDIS = prevRedis
	})
}

func TestDeviceCacheSingleflightCollapsesConcurrentMisses(t *testing.T) {
	var loads atomic.Int32
	release := make(chan struct{})
	withDeviceCacheLoader(t, func(id string) (*model.Device, error) {
		loads.Add(1)
		<-release
		return &model.Device{ID: id, TenantID: "t1"}, nil
	})

	const callers = 50
	var started, finished sync.WaitGroup
	results := make([]*model.Device, callers)
	errs := make([]error, callers)
	started.Add(callers)
	finished.Add(callers)
	for i := 0; i < callers; i++ {
		go func(i int) {
			defer finished.Done()
			started.Done()
			results[i], errs[i] = GetDeviceCacheById("dev-evicted")
		}(i)
	}
	started.Wait()
	// Give every caller time to join the in-flight load before releasing it.
	time.Sleep(100 * time.Millisecond)
	close(release)
	finished.Wait()

	if n := loads.Load(); n != 1 {
		t.Fatalf("DAL loads = %d, want 1", n)
	}
	for i := 0; i < callers; i++ {
		if errs[i] != nil || results[i] == nil || results[i].ID != "dev-evicted" {
			t.Fatalf("caller %d got %+v, %v", i, results[i], errs[i])
		}
	}
	// Waiters get their own copy: mutating one must not affect another.
	results[0].IsOnline = 1
	for i := 1; i < callers; i++ {
		if results[i] == results[0] {
			t.Fatalf("caller %d shares a pointer with caller 0", i)
		}
	}
}

func TestDeviceCacheSingleflightDoesNotCacheErrorsOrMergeDevices(t *testing.T) {
	var loads atomic.Int32
	fail := errors.New("db down")
	withDeviceCacheLoader(t, func(id string) (*model.Device, error) {
		if loads.Add(1) == 1 {
			return nil, fail
		}
		return &model.Device{ID: id}, nil
	})
	if _, err := GetDeviceCacheById("d1"); !errors.Is(err, fail) {
		t.Fatalf("first call err = %v, want %v", err, fail)
	}
	if d, err := GetDeviceCacheById("d1"); err != nil || d.ID != "d1" {
		t.Fatalf("retry after error = %+v, %v", d, err)
	}
	if d, err := GetDeviceCacheById("d2"); err != nil || d.ID != "d2" {
		t.Fatalf("different device = %+v, %v", d, err)
	}
	if n := loads.Load(); n != 3 {
		t.Fatalf("loads = %d, want 3", n)
	}
}

func TestDeviceCacheSingleflightRecoversLoaderPanic(t *testing.T) {
	withDeviceCacheLoader(t, func(string) (*model.Device, error) { panic("boom") })
	if _, err := GetDeviceCacheById("d1"); err == nil {
		t.Fatal("expected error from panicking loader")
	}
	// The key must have been released: a second call runs the loader again.
	deviceCacheLoader = func(id string) (*model.Device, error) { return &model.Device{ID: id}, nil }
	if d, err := GetDeviceCacheById("d1"); err != nil || d.ID != "d1" {
		t.Fatalf("after panic = %+v, %v", d, err)
	}
}
