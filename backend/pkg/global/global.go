// 文件用途：声明后端进程级全局状态和事件通道。
// 核心逻辑：集中保存版本号、数据库、Redis、Casbin、响应处理器、OTA 地址和 SSE 管理器引用。
// 关键注意事项：全局变量依赖初始化顺序，测试和后台任务使用前必须确认对应对象已被设置。
// 重构建议：后续可逐步迁移到应用上下文或依赖注入，减少跨包隐式耦合。
package global

import (
	"aetherlink-iot/backend/internal/middleware/response"

	"github.com/casbin/casbin/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	VERSION         = "0.0.23"
	VERSION_NUMBER  = 57
	SYSTEM_VERSION  = "v1.2.3"
	DB              *gorm.DB
	REDIS           *redis.Client
	STATUS_REDIS    *redis.Client
	CasbinEnforcer  *casbin.Enforcer
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
