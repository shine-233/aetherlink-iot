// 文件用途：声明后端进程级全局状态和事件通道。
// 核心逻辑：集中保存版本号、数据库、Redis、Casbin、响应处理器、OTA 地址和 SSE 管理器引用。
// 关键注意事项：全局变量依赖初始化顺序，测试和后台任务使用前必须确认对应对象已被设置。
// 重构建议：后续可逐步迁移到应用上下文或依赖注入，减少跨包隐式耦合。
package global

import (
	"aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/internal/tenantree"

	"github.com/casbin/casbin/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	// VERSION_NUMBER 必须与 backend/sql/ 下最大编号一致：pg_init.go 的迁移循环上界即本值，
	// 低于已合入迁移编号会导致新迁移永不执行（2026-09-06 Phase D 集成修复）。
	// 69/71/72/73 为补齐号位的空迁移（循环对缺文件 fail-fast，故不可跳过）。
	VERSION        = "0.0.23"
	VERSION_NUMBER = 82
	SYSTEM_VERSION = "v1.2.3"
	DB             *gorm.DB
	REDIS          *redis.Client
	STATUS_REDIS   *redis.Client
	// CasbinEnforcer 切换为 SyncedEnforcer（2026-09-04，ROADMAP C7+ watcher 配套）：
	// watcher 跨实例通知触发的 LoadPolicy 与 HTTP 并发 Enforce 共存需锁保护；
	// 同时消除既有"手工 LoadPolicy vs 并发 Enforce"的无锁竞态。
	CasbinEnforcer *casbin.SyncedEnforcer
	// TenantTree is the process-wide lazy tenant hierarchy cache.
	TenantTree      *tenantree.Tree
	OtaAddress      string
	TPSSEManager    *SSEManager
	ResponseHandler *response.Handler
)

type EventData struct {
	Name    string
	Message string
}

// 事件通道
var EventChan chan EventData
