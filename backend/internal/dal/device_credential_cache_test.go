// 文件用途：设备凭证网页测试缓存（Phase 2b）helper 单测。
// 核心逻辑：锁定键格式、24h TTL、逐设备写入、miss/故障 fail-closed 归一为
// ErrCredentialCacheMiss 的边界；Redis 依赖经包级 seam 注入假实现（无 miniredis）。
// 关键注意事项：Store 是 best-effort（失败仅 Warn），Load 不区分"过期"与"Redis 故障"。

package dal

import (
	"context"
	"errors"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

type stubCredentialCacheStore struct {
	setKey    string
	setValue  string
	setTTL    time.Duration
	getValues map[string]string
	getErr    error
}

func (s *stubCredentialCacheStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	s.setKey = key
	s.setValue = value
	s.setTTL = ttl
	return nil
}

func (s *stubCredentialCacheStore) Get(_ context.Context, key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	value, ok := s.getValues[key]
	if !ok {
		return "", ErrCredentialCacheMiss
	}
	return value, nil
}

func useStubCredentialCacheStore(t *testing.T, stub *stubCredentialCacheStore) {
	t.Helper()

	old := DeviceCredentialCacheStore
	DeviceCredentialCacheStore = stub
	t.Cleanup(func() { DeviceCredentialCacheStore = old })
}

func TestStoreDeviceCredentialTestCacheKeyTTLAndValue(t *testing.T) {
	stub := &stubCredentialCacheStore{}
	useStubCredentialCacheStore(t, stub)

	voucher := `{"username":"cache-user","password":"pw"}`
	StoreDeviceCredentialTestCache("dev-cache-1", voucher)

	if stub.setKey != "aetherlink:device_cred_test_cache:dev-cache-1" {
		t.Fatalf("cache key = %q, want prefixed device key", stub.setKey)
	}
	if stub.setValue != voucher {
		t.Fatalf("cached value = %q, want %q", stub.setValue, voucher)
	}
	if stub.setTTL != DeviceCredentialTestCacheTTL || DeviceCredentialTestCacheTTL != 24*time.Hour {
		t.Fatalf("cache ttl = %v, want 24h", stub.setTTL)
	}
}

func TestStoreDeviceCredentialTestCacheSkipsEmptyInputs(t *testing.T) {
	stub := &stubCredentialCacheStore{}
	useStubCredentialCacheStore(t, stub)

	StoreDeviceCredentialTestCache("", `{"username":"u"}`)
	StoreDeviceCredentialTestCache("dev-cache-empty", "   ")

	if stub.setKey != "" {
		t.Fatalf("empty inputs must not touch the store, got key %q", stub.setKey)
	}
}

func TestStoreDeviceCredentialTestCacheFailureIsBestEffort(t *testing.T) {
	stub := &stubCredentialCacheStore{getErr: errors.New("store down")}
	useStubCredentialCacheStore(t, stub)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("store failure must not panic (best-effort contract): %v", r)
		}
	}()
	StoreDeviceCredentialTestCache("dev-cache-fail", `{"username":"u"}`)
}

func TestLoadDeviceCredentialTestCacheHitAndMiss(t *testing.T) {
	t.Run("hit returns stored voucher", func(t *testing.T) {
		stub := &stubCredentialCacheStore{getValues: map[string]string{
			deviceCredentialTestCacheKey("dev-hit"): `{"username":"hit-user"}`,
		}}
		useStubCredentialCacheStore(t, stub)

		got, err := LoadDeviceCredentialTestCache("dev-hit")
		if err != nil || got != `{"username":"hit-user"}` {
			t.Fatalf("load = (%q, %v), want hit value with nil error", got, err)
		}
	})

	t.Run("absent key maps to sentinel miss", func(t *testing.T) {
		stub := &stubCredentialCacheStore{}
		useStubCredentialCacheStore(t, stub)

		got, err := LoadDeviceCredentialTestCache("dev-missing")
		if got != "" || !errors.Is(err, ErrCredentialCacheMiss) {
			t.Fatalf("load = (%q, %v), want sentinel miss", got, err)
		}
	})

	t.Run("redis failure fail-closed to sentinel miss", func(t *testing.T) {
		stub := &stubCredentialCacheStore{getErr: errors.New("redis connection refused")}
		useStubCredentialCacheStore(t, stub)

		got, err := LoadDeviceCredentialTestCache("dev-err")
		if got != "" || !errors.Is(err, ErrCredentialCacheMiss) {
			t.Fatalf("load = (%q, %v), want fail-closed sentinel miss", got, err)
		}
	})

	t.Run("blank device id maps to sentinel miss", func(t *testing.T) {
		stub := &stubCredentialCacheStore{}
		useStubCredentialCacheStore(t, stub)

		got, err := LoadDeviceCredentialTestCache("   ")
		if got != "" || !errors.Is(err, ErrCredentialCacheMiss) {
			t.Fatalf("load = (%q, %v), want sentinel miss", got, err)
		}
	})

	t.Run("blank stored value maps to sentinel miss", func(t *testing.T) {
		stub := &stubCredentialCacheStore{getValues: map[string]string{
			deviceCredentialTestCacheKey("dev-blank"): " ",
		}}
		useStubCredentialCacheStore(t, stub)

		got, err := LoadDeviceCredentialTestCache("dev-blank")
		if got != "" || !errors.Is(err, ErrCredentialCacheMiss) {
			t.Fatalf("load = (%q, %v), want sentinel miss for blank value", got, err)
		}
	})
}

// TestWriteVoucherHashStoresTestCachePerDevice 锁定 hash 收口点与测试缓存的联动：
// writeVoucherHashWithTx 对每台非空 voucher 设备各写一次缓存，空 voucher 行跳过。
func TestWriteVoucherHashStoresTestCachePerDevice(t *testing.T) {
	db := setupDeviceVoucherDualModeTestDB(t)
	cache := useRecordingCredentialCacheStore(t)

	devices := []*model.Device{
		{ID: "vh-cache-a", Voucher: `{"username":"cache-a"}`},
		{ID: "vh-cache-b", Voucher: `{"username":"cache-b"}`},
		{ID: "vh-cache-skip", Voucher: ""},
	}

	if err := writeVoucherHashWithTx(db, devices); err != nil {
		t.Fatalf("writeVoucherHashWithTx: %v", err)
	}

	for _, id := range []string{"vh-cache-a", "vh-cache-b"} {
		key := deviceCredentialTestCacheKey(id)
		if _, ok := cache.values[key]; !ok {
			t.Fatalf("test cache missing for device %s", id)
		}
	}
	if _, ok := cache.values[deviceCredentialTestCacheKey("vh-cache-skip")]; ok {
		t.Fatalf("empty voucher row must not populate test cache")
	}
}
