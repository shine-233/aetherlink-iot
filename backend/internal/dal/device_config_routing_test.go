package dal

import (
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"gorm.io/gorm"
)

// resetDeviceConfigRoutingCache 清空全部路由缓存并推进代际，使在途加载结果作废（仅测试用）。
func resetDeviceConfigRoutingCache() {
	deviceConfigRoutingMu.Lock()
	deviceConfigRoutingCache = map[string]deviceConfigRoutingEntry{}
	deviceConfigRoutingGen++
	deviceConfigRoutingMu.Unlock()
}

func stubDeviceConfigRouting(t *testing.T, load func(string) (*model.DeviceConfig, error)) *time.Time {
	t.Helper()
	prevLoad, prevNow := deviceConfigRoutingLoad, deviceConfigRoutingNow
	now := time.Unix(1_000, 0)
	deviceConfigRoutingLoad = load
	deviceConfigRoutingNow = func() time.Time { return now }
	resetDeviceConfigRoutingCache()
	t.Cleanup(func() {
		deviceConfigRoutingLoad, deviceConfigRoutingNow = prevLoad, prevNow
		resetDeviceConfigRoutingCache()
	})
	return &now
}

func TestDeviceConfigRoutingCachesAndInvalidates(t *testing.T) {
	calls := 0
	chain := " chain-1 "
	stubDeviceConfigRouting(t, func(id string) (*model.DeviceConfig, error) {
		calls++
		return &model.DeviceConfig{ID: id, TenantID: "t1", DefaultRuleChainID: &chain}, nil
	})
	for i := 0; i < 3; i++ {
		r, found, err := GetDeviceConfigRouting("cfg")
		if err != nil || !found || r.TenantID != "t1" || r.DefaultRuleChainID != "chain-1" {
			t.Fatalf("routing = %+v %v %v", r, found, err)
		}
	}
	if calls != 1 {
		t.Fatalf("loads = %d, want 1", calls)
	}
	chain = "chain-2"
	InvalidateDeviceConfigRouting("cfg")
	if r, _, _ := GetDeviceConfigRouting("cfg"); r.DefaultRuleChainID != "chain-2" || calls != 2 {
		t.Fatalf("after invalidate: %+v calls=%d", r, calls)
	}
}

func TestDeviceConfigRoutingTTLAndNegativeCache(t *testing.T) {
	calls := 0
	now := stubDeviceConfigRouting(t, func(string) (*model.DeviceConfig, error) {
		calls++
		return nil, gorm.ErrRecordNotFound
	})
	for i := 0; i < 2; i++ {
		// First call surfaces the not-found error; cached hits report found=false, err=nil.
		_, found, err := GetDeviceConfigRouting("missing")
		if found || (i == 0 && !errors.Is(err, gorm.ErrRecordNotFound)) || (i > 0 && err != nil) {
			t.Fatalf("call %d: found=%v err=%v", i, found, err)
		}
	}
	if calls != 1 {
		t.Fatalf("negative result not cached: loads=%d", calls)
	}
	*now = now.Add(deviceConfigRoutingTTL + time.Second)
	GetDeviceConfigRouting("missing")
	if calls != 2 {
		t.Fatalf("TTL not honored: loads=%d", calls)
	}
}

func TestDeviceConfigRoutingDoesNotCacheTransientErrors(t *testing.T) {
	calls := 0
	stubDeviceConfigRouting(t, func(string) (*model.DeviceConfig, error) {
		calls++
		return nil, errors.New("db down")
	})
	GetDeviceConfigRouting("cfg")
	GetDeviceConfigRouting("cfg")
	if calls != 2 {
		t.Fatalf("transient error cached: loads=%d", calls)
	}
}

func TestDeviceConfigRoutingDropsLoadRacingInvalidation(t *testing.T) {
	calls := 0
	stubDeviceConfigRouting(t, func(id string) (*model.DeviceConfig, error) {
		calls++
		if calls == 1 {
			// A writer invalidates while this (stale) read is in flight.
			InvalidateDeviceConfigRouting(id)
		}
		return &model.DeviceConfig{ID: id, TenantID: "t1"}, nil
	})
	GetDeviceConfigRouting("cfg")
	GetDeviceConfigRouting("cfg")
	if calls != 2 {
		t.Fatalf("stale load raced with invalidation was cached: loads=%d", calls)
	}
}

func TestDeviceConfigRoutingDBLoaderAndWritePathInvalidation(t *testing.T) {
	setupDeviceConfigDALTestDB(t)
	prevNow := deviceConfigRoutingNow
	t.Cleanup(func() { deviceConfigRoutingNow = prevNow; resetDeviceConfigRoutingCache() })
	resetDeviceConfigRoutingCache()

	chain := "chain-a"
	if err := CreateDeviceConfig(&model.DeviceConfig{ID: "cfg-db", Name: "n", TenantID: "t1", DefaultRuleChainID: &chain}); err != nil {
		t.Fatal(err)
	}
	r, found, err := GetDeviceConfigRouting("cfg-db")
	if err != nil || !found || r.DefaultRuleChainID != "chain-a" || r.TenantID != "t1" {
		t.Fatalf("routing = %+v %v %v", r, found, err)
	}
	if err := UpdateDeviceConfigDefaultRuleChainID("cfg-db", nil); err != nil {
		t.Fatal(err)
	}
	if r, _, _ := GetDeviceConfigRouting("cfg-db"); r.DefaultRuleChainID != "" {
		t.Fatalf("unbind not visible: %+v", r)
	}
	if err := DeleteDeviceConfigForTenant("cfg-db", "t1"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := GetDeviceConfigRouting("cfg-db"); found {
		t.Fatal("deleted config still routed")
	}
}
