package initialize

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"testing"
	"time"

	"aetherlink-iot/backend/initialize/automatecache"

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

	got, removed := removeSceneAutomationFromDeviceInfos(infos, "scene-b")
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

	got, removed := removeSceneAutomationFromDeviceInfos(infos, "scene-missing")
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

	got, removed := removeSceneAutomationFromDeviceInfos(infos, "scene-a")
	if !removed {
		t.Fatal("expected target scene to be removed")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %#v", got)
	}
}

func TestDeleteCacheBySceneAutomationIdFallsBackToScanningBothDeviceDimensions(t *testing.T) {
	store := newMemoryAutomateCacheStore()
	cache := &AutomateCache{
		client:    store,
		expiredIn: time.Minute,
		device:    automatecache.NewOneDeviceCache(),
	}

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
	cache := &AutomateCache{
		client:    store,
		expiredIn: time.Minute,
		device:    automatecache.NewOneDeviceCache(),
	}

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
