// 文件用途：锁定设备配置缓存的 TTL 契约（不能写永久键）。
// 核心逻辑：用 miniredis 断言 GetDeviceConfigByID 回填缓存时带 constant.CacheFallbackTTL，
//          而不是历史实现里的 0（永久）。
// 关键注意事项：
//   1. 写路径主动失效（service/device_config.go 的 initialize.DelDeviceConfigCache）仍是
//      主机制；本测试锁的是**兜底**——只要有一条写路径漏了失效，永久键就会造成永久脏读。
//   2. 设备缓存（GetDeviceCacheById）与脚本缓存早已在 P2 修复（2026-08-25）改用
//      CacheFallbackTTL，唯独设备配置缓存漏掉；本测试防止它退回永久键。

package dal

import (
	"context"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// useDeviceConfigCacheRedis 用 miniredis 顶替 global.REDIS，返回服务端与客户端。
func useDeviceConfigCacheRedis(t *testing.T) (*miniredis.Miniredis, *goredis.Client) {
	t.Helper()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})

	old := global.REDIS
	global.REDIS = client
	t.Cleanup(func() {
		_ = client.Close()
		global.REDIS = old
	})
	return server, client
}

func TestGetDeviceConfigByIDCachesWithFallbackTTL(t *testing.T) {
	db := setupDeviceConfigDALTestDB(t)
	_, client := useDeviceConfigCacheRedis(t)

	now := time.Now().UTC()
	config := &model.DeviceConfig{
		ID:               "cfg-cache-ttl",
		Name:             "cache-ttl-config",
		DeviceType:       "1",
		TenantID:         "tenant-a",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(config).Error; err != nil {
		t.Fatalf("seed device config: %v", err)
	}

	// 首次读取：缓存未命中 -> 回源 DB -> 写回缓存
	if _, err := GetDeviceConfigByID("cfg-cache-ttl"); err != nil {
		t.Fatalf("GetDeviceConfigByID: %v", err)
	}

	const cacheKey = "cfg-cache-ttl_config"
	ttl, err := client.TTL(context.Background(), cacheKey).Result()
	if err != nil {
		t.Fatalf("read cache TTL: %v", err)
	}

	if ttl <= 0 {
		t.Fatalf("设备配置缓存必须是带过期的键，实测 TTL=%v（<=0 表示永久键，漏失效即永久脏读）", ttl)
	}
	if ttl > constant.CacheFallbackTTL {
		t.Fatalf("缓存 TTL=%v 超过兜底上限 %v", ttl, constant.CacheFallbackTTL)
	}
}

// TestGetDeviceConfigByIDServesFromCache 确认回填后确实走缓存（TTL 改动没有把缓存写坏）。
func TestGetDeviceConfigByIDServesFromCache(t *testing.T) {
	db := setupDeviceConfigDALTestDB(t)
	server, _ := useDeviceConfigCacheRedis(t)

	now := time.Now().UTC()
	config := &model.DeviceConfig{
		ID:         "cfg-cache-hit",
		Name:       "cache-hit-config",
		DeviceType: "1",
		TenantID:   "tenant-a",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(config).Error; err != nil {
		t.Fatalf("seed device config: %v", err)
	}

	if _, err := GetDeviceConfigByID("cfg-cache-hit"); err != nil {
		t.Fatalf("first read: %v", err)
	}
	if !server.Exists("cfg-cache-hit_config") {
		t.Fatalf("首次读取后应回填缓存键")
	}

	// 改库但不改缓存：第二次读取必须返回缓存里的旧值，证明命中路径未被破坏。
	if err := db.Model(&model.DeviceConfig{}).
		Where("id = ?", "cfg-cache-hit").
		Update("name", "renamed-in-db").Error; err != nil {
		t.Fatalf("update row: %v", err)
	}

	got, err := GetDeviceConfigByID("cfg-cache-hit")
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if got.Name != "cache-hit-config" {
		t.Fatalf("缓存命中应返回旧值，实测 Name=%q（说明回填/命中路径已坏）", got.Name)
	}
}
