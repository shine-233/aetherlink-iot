package automatecache

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"sync"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/redis/go-redis/v9"
)

type memoryAutomateCacheStore struct {
	values map[string]string
}

func newMemoryAutomateCacheStore() *memoryAutomateCacheStore {
	return &memoryAutomateCacheStore{values: make(map[string]string)}
}

func (s *memoryAutomateCacheStore) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx, "get", key)
	value, ok := s.values[key]
	if !ok {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	cmd.SetVal(value)
	return cmd
}

func (s *memoryAutomateCacheStore) Set(
	ctx context.Context,
	key string,
	value interface{},
	expiration time.Duration,
) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx, "set", key, value, expiration)
	s.values[key] = fmt.Sprint(value)
	cmd.SetVal("OK")
	return cmd
}

func (s *memoryAutomateCacheStore) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx, append([]interface{}{"del"}, stringsToInterfaces(keys)...)...)
	var removed int64
	for _, key := range keys {
		if _, ok := s.values[key]; ok {
			delete(s.values, key)
			removed++
		}
	}
	cmd.SetVal(removed)
	return cmd
}

func (s *memoryAutomateCacheStore) Scan(
	ctx context.Context,
	cursor uint64,
	match string,
	count int64,
) *redis.ScanCmd {
	cmd := redis.NewScanCmd(ctx, nil, "scan", cursor, "match", match, "count", count)
	keys := make([]string, 0)
	for key := range s.values {
		matched, err := path.Match(match, key)
		if err == nil && matched {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	cmd.SetVal(keys, 0)
	return cmd
}

func stringsToInterfaces(values []string) []interface{} {
	result := make([]interface{}, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func mustCacheJSON(t *testing.T, value interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal cache fixture: %v", err)
	}
	return string(encoded)
}

func readCachedDeviceInfos(t *testing.T, store *memoryAutomateCacheStore, key string) AutomateDeviceInfos {
	t.Helper()
	value, ok := store.values[key]
	if !ok {
		t.Fatalf("expected cache key %s", key)
	}
	var infos AutomateDeviceInfos
	if err := json.Unmarshal([]byte(value), &infos); err != nil {
		t.Fatalf("decode cache key %s: %v", key, err)
	}
	return infos
}

func requireCacheKeyMissing(t *testing.T, store *memoryAutomateCacheStore, key string) {
	t.Helper()
	if _, ok := store.values[key]; ok {
		t.Fatalf("expected cache key %s to be removed", key)
	}
}

func TestRemoveSceneAutomationFromDeviceInfosKeepsOtherScenes(t *testing.T) {
	infos := AutomateDeviceInfos{
		{SceneAutomationId: "scene-a", GroupIds: []string{"group-a"}},
		{SceneAutomationId: "scene-b", GroupIds: []string{"group-b"}},
		{SceneAutomationId: "scene-c", GroupIds: []string{"group-c"}},
	}

	got, removed := RemoveSceneAutomationFromDeviceInfos(infos, "scene-b")
	if !removed {
		t.Fatal("expected target scene to be removed")
	}
	if len(got) != 2 {
		t.Fatalf("expected two remaining scenes, got %d", len(got))
	}
	if got[0].SceneAutomationId != "scene-a" || got[1].SceneAutomationId != "scene-c" {
		t.Fatalf("unexpected remaining scenes: %#v", got)
	}
}

func TestRemoveSceneAutomationFromDeviceInfosReportsNoMatch(t *testing.T) {
	infos := AutomateDeviceInfos{
		{SceneAutomationId: "scene-a", GroupIds: []string{"group-a"}},
	}

	got, removed := RemoveSceneAutomationFromDeviceInfos(infos, "scene-missing")
	if removed {
		t.Fatal("did not expect removal for missing scene")
	}
	if len(got) != 1 || got[0].SceneAutomationId != "scene-a" {
		t.Fatalf("unexpected remaining scenes: %#v", got)
	}
}

func TestRemoveSceneAutomationFromDeviceInfosCanEmptyList(t *testing.T) {
	infos := AutomateDeviceInfos{
		{SceneAutomationId: "scene-a", GroupIds: []string{"group-a"}},
	}

	got, removed := RemoveSceneAutomationFromDeviceInfos(infos, "scene-a")
	if !removed {
		t.Fatal("expected target scene to be removed")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %#v", got)
	}
}

func TestDeleteCacheBySceneAutomationIdFallsBackToScanningBothDeviceDimensions(t *testing.T) {
	store := newMemoryAutomateCacheStore()
	cache := New(store, time.Minute)

	store.values["automate:v3:one:_:device-1"] = mustCacheJSON(t, AutomateDeviceInfos{
		{SceneAutomationId: "scene-target", GroupIds: []string{"group-one"}},
		{SceneAutomationId: "scene-keep", GroupIds: []string{"group-keep"}},
	})
	store.values["automate:v3:one:_group_:group-one"] = mustCacheJSON(t, DTConditions{
		{SceneAutomationID: "scene-target"},
	})
	store.values["automate:v3:multiple:_:config-1"] = mustCacheJSON(t, AutomateDeviceInfos{
		{SceneAutomationId: "scene-target", GroupIds: []string{"group-multiple"}},
	})
	store.values["automate:v3:multiple:_group_:group-multiple"] = mustCacheJSON(t, DTConditions{
		{SceneAutomationID: "scene-target"},
	})

	if err := cache.DeleteCacheBySceneAutomationId("scene-target"); err != nil {
		t.Fatalf("delete scene automation cache: %v", err)
	}

	oneDeviceInfos := readCachedDeviceInfos(t, store, "automate:v3:one:_:device-1")
	if len(oneDeviceInfos) != 1 || oneDeviceInfos[0].SceneAutomationId != "scene-keep" {
		t.Fatalf("expected unrelated scene cache to remain, got %#v", oneDeviceInfos)
	}
	requireCacheKeyMissing(t, store, "automate:v3:one:_group_:group-one")
	requireCacheKeyMissing(t, store, "automate:v3:multiple:_:config-1")
	requireCacheKeyMissing(t, store, "automate:v3:multiple:_group_:group-multiple")
}

func TestDeleteCacheBySceneAutomationIdFallsBackWhenGroupCacheIsMissing(t *testing.T) {
	store := newMemoryAutomateCacheStore()
	cache := New(store, time.Minute)

	store.values["automate:v3:one:_action_:scene-target"] = mustCacheJSON(t, AutomateActionInfo{
		GroupIds: []string{"missing-group"},
	})
	store.values["automate:v3:one:_:device-1"] = mustCacheJSON(t, AutomateDeviceInfos{
		{SceneAutomationId: "scene-target", GroupIds: []string{"missing-group"}},
	})

	if err := cache.DeleteCacheBySceneAutomationId("scene-target"); err != nil {
		t.Fatalf("delete scene automation cache: %v", err)
	}

	requireCacheKeyMissing(t, store, "automate:v3:one:_action_:scene-target")
	requireCacheKeyMissing(t, store, "automate:v3:one:_:device-1")
}

// lockedStore 给内存存储加锁，使其可被并发访问（真实 Redis 客户端本身并发安全）。
type lockedStore struct {
	mu    sync.Mutex
	inner *memoryAutomateCacheStore
}

func (s *lockedStore) Get(ctx context.Context, key string) *redis.StringCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Get(ctx, key)
}

func (s *lockedStore) Set(ctx context.Context, key string, value interface{}, exp time.Duration) *redis.StatusCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Set(ctx, key, value, exp)
}

func (s *lockedStore) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Del(ctx, keys...)
}

func (s *lockedStore) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Scan(ctx, cursor, match, count)
}

// 旧实现把 one/multiple 维度写在单例字段上，并发的单设备与设备配置读写会拿对方的前缀拼键。
// 维度改为逐调用参数后，大量 goroutine 交错读写必须各自落在正确的键上。
func TestCacheDimensionIsPerCallUnderConcurrency(t *testing.T) {
	store := &lockedStore{inner: newMemoryAutomateCacheStore()}
	cache := New(store, time.Minute)
	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				_ = cache.SetCacheByDeviceIdWithNoTask(fmt.Sprintf("dev-%d", i), "")
			} else {
				_ = cache.SetCacheByDeviceIdWithNoTask(fmt.Sprintf("dev-%d", i), fmt.Sprintf("cfg-%d", i))
			}
		}()
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		var key, cfg string
		if i%2 == 0 {
			key = fmt.Sprintf("automate:v3:one:_:dev-%d", i)
		} else {
			cfg = fmt.Sprintf("cfg-%d", i)
			key = "automate:v3:multiple:_:" + cfg
		}
		if store.inner.values[key] != ContentNoTask {
			t.Fatalf("expected %s = NOTASK, got %q", key, store.inner.values[key])
		}
		_, result, err := cache.GetCacheByDeviceId(fmt.Sprintf("dev-%d", i), cfg)
		if err != nil || result != ResultNoTask {
			t.Fatalf("GetCacheByDeviceId(%d) = %d, %v", i, result, err)
		}
	}
	if got := len(store.inner.values); got != n {
		t.Fatalf("cache holds %d keys, want %d (a dimension leak would write cross-prefixed keys)", got, n)
	}
}

// 键格式是线上存量缓存的契约，锁死以防回归。
func TestCacheKeyFormatIsStable(t *testing.T) {
	cases := map[string]string{
		keyBase(dimensionOne, "d1"):           "automate:v3:one:_:d1",
		keyGroup(dimensionMultiple, "g1"):     "automate:v3:multiple:_group_:g1",
		keyAction(dimensionOne, "s1"):         "automate:v3:one:_action_:s1",
		keyBase(dimensionFor("cfg"), "cfg"):   "automate:v3:multiple:_:cfg",
		keyBase(dimensionFor(""), "device-x"): "automate:v3:one:_:device-x",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("key = %s, want %s", got, want)
		}
	}
}

func TestSetThenGetCacheByDeviceIdRoundTrip(t *testing.T) {
	store := newMemoryAutomateCacheStore()
	cache := New(store, time.Minute)
	source := "dev-1"
	conditions := []model.DeviceTriggerCondition{{SceneAutomationID: "s1", GroupID: "g1", TriggerSource: &source, TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_ONE}}
	actions := []model.ActionInfo{{SceneAutomationID: "s1", ActionType: "x"}}
	if err := cache.SetCacheByDeviceId("dev-1", "", conditions, actions); err != nil {
		t.Fatal(err)
	}
	got, result, err := cache.GetCacheByDeviceId("dev-1", "")
	if err != nil || result != ResultOK {
		t.Fatalf("get = %d, %v", result, err)
	}
	if len(got.AutomateExecteSceeInfos) != 1 || got.AutomateExecteSceeInfos[0].SceneAutomationId != "s1" ||
		len(got.AutomateExecteSceeInfos[0].GroupsCondition) != 1 || len(got.AutomateExecteSceeInfos[0].Actions) != 1 {
		t.Fatalf("unexpected params %#v", got)
	}
	if _, result, _ := cache.GetCacheByDeviceId("dev-1", "cfg-other"); result != ResultNotFound {
		t.Fatalf("multiple-dimension lookup must miss, got %d", result)
	}
}
