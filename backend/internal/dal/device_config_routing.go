// 文件用途: device_config 路由缓存子聚合（9-28 DAL 拆分，自 device_config.go 按聚合迁出）。
// 核心逻辑: 上行热路径（规则链解析）的档案路由读取——(租户, 默认规则链) 的进程内 TTL 缓存，
//   miss 时只取两列直查 DB，写路径与绕过 DAL 的修改通过 Invalidate 显式失效。
// 关键注意事项: 缓存写入以 generation 防止"加载期间发生失效"的旧值回填；查询失败不缓存正值，
//   由调用方按 fail-closed 处理。导出符号与签名与拆分前完全一致。
// 重构建议: 若未来多副本部署出现路由漂移投诉，可把 TTL 收敛为配置面或接入失效广播。

package dal

import (
	"errors"
	"strings"
	"sync"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// DeviceConfigRouting 是上行热路径（规则链解析）真正需要的档案字段子集。
type DeviceConfigRouting struct {
	TenantID           string
	DefaultRuleChainID string // 已 TrimSpace；空串表示未绑定
}

// deviceConfigRoutingTTL 兜底 TTL：本进程写路径会主动失效；其它副本/直接改库的
// 陈旧窗口以此为上限。
const deviceConfigRoutingTTL = 30 * time.Second

type deviceConfigRoutingEntry struct {
	routing   DeviceConfigRouting
	found     bool
	expiresAt time.Time
}

var (
	deviceConfigRoutingMu    sync.RWMutex
	deviceConfigRoutingCache = map[string]deviceConfigRoutingEntry{}
	deviceConfigRoutingGen   uint64
	deviceConfigRoutingNow   = time.Now
	// deviceConfigRoutingLoad 可在测试中替换。默认直查 DB 的两列而不是走 GetDeviceConfigByID：
	// 后者读永久 Redis 键 "<id>_config"，而 service 层在 DAL 写返回之后才删该键，
	// 窗口内的并发 miss 会把旧的默认规则链重新写进本缓存（持续一个 TTL）。
	deviceConfigRoutingLoad = loadDeviceConfigRoutingFromDB
)

// loadDeviceConfigRoutingFromDB 只取路由需要的列；每档案每 TTL 至多一次，
// 比整份档案 JSON 反序列化更轻。
func loadDeviceConfigRoutingFromDB(id string) (*model.DeviceConfig, error) {
	var row model.DeviceConfig
	err := global.DB.Model(&model.DeviceConfig{}).
		Select("id", "tenant_id", "default_rule_chain_id").
		Where("id = ?", id).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

const deviceConfigRoutingMaxEntries = 65536

// GetDeviceConfigRouting 返回档案的 (租户, 默认规则链)；进程内 TTL 缓存命中时零 Redis/DB 往返。
// 查询失败（含不存在）不缓存正值，返回 found=false 与原始错误，由调用方按 fail-closed 处理；
// "不存在"会做短期负缓存，避免坏绑定每条消息打一次 Redis。
func GetDeviceConfigRouting(id string) (DeviceConfigRouting, bool, error) {
	now := deviceConfigRoutingNow()
	deviceConfigRoutingMu.RLock()
	entry, ok := deviceConfigRoutingCache[id]
	gen := deviceConfigRoutingGen
	deviceConfigRoutingMu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.routing, entry.found, nil
	}

	config, err := deviceConfigRoutingLoad(id)
	notFound := err != nil && errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !notFound {
		return DeviceConfigRouting{}, false, err
	}
	next := deviceConfigRoutingEntry{expiresAt: now.Add(deviceConfigRoutingTTL)}
	if config != nil && err == nil {
		next.found = true
		next.routing.TenantID = config.TenantID
		if config.DefaultRuleChainID != nil {
			next.routing.DefaultRuleChainID = strings.TrimSpace(*config.DefaultRuleChainID)
		}
	}
	deviceConfigRoutingMu.Lock()
	// 加载期间发生过失效则放弃写入，避免把失效前读到的旧值写回缓存。
	if gen == deviceConfigRoutingGen {
		if len(deviceConfigRoutingCache) >= deviceConfigRoutingMaxEntries {
			deviceConfigRoutingCache = map[string]deviceConfigRoutingEntry{}
		}
		deviceConfigRoutingCache[id] = next
	}
	deviceConfigRoutingMu.Unlock()
	return next.routing, next.found, err
}

// InvalidateDeviceConfigRouting 丢弃指定档案的进程内路由缓存；device_config 写路径均会调用，
// service 层在绕过 DAL 写档案时也应调用。
func InvalidateDeviceConfigRouting(id string) {
	deviceConfigRoutingMu.Lock()
	delete(deviceConfigRoutingCache, id)
	deviceConfigRoutingGen++
	deviceConfigRoutingMu.Unlock()
}

// ResetDeviceConfigRoutingCache 清空全部路由缓存（测试与运维兜底）。
func ResetDeviceConfigRoutingCache() {
	deviceConfigRoutingMu.Lock()
	deviceConfigRoutingCache = map[string]deviceConfigRoutingEntry{}
	deviceConfigRoutingGen++
	deviceConfigRoutingMu.Unlock()
}
