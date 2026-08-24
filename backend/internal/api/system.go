// system.go 负责系统基础信息相关的 HTTP 接口。
// 主要覆盖系统时间、健康检查、版本号这类基础能力，它们通常被前端启动探活、
// 运维监控或部署核验流程直接调用，因此该文件刻意保持“无数据库、无复杂业务、直接返回结果”的薄处理器风格。
// 当前实现不依赖 claims，也不做请求体绑定，边界非常清晰：只从工具函数或全局常量读取值后写回统一响应上下文。
// 静态审查建议：
//  1. 这类基础接口适合作为最轻量的健康与冒烟验证入口，但若后续新增更多系统信息字段，建议统一抽成 system info DTO，
//     避免 map[string]interface{} 分散扩张。
//  2. HealthCheck 目前只表示“HTTP 处理器可用”，并不证明数据库、Redis、MQTT 等依赖已就绪；若后续要承载发布探针，
//     应单独区分轻量存活检查与重依赖就绪检查，避免误判系统状态。
package api

import (
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type SystemApi struct{}

// HandleSystime 返回秒级系统时间戳。
// 绑定/claims：不绑定请求参数，也不依赖登录 claims，任何能访问该路由的调用方都能拿到当前服务节点时间。
// 边界说明：这里返回的是应用节点本地时间，不保证与数据库时间、外部设备时间或多节点集群时间绝对一致。
// 静态审查建议：当前直接使用 map 返回单字段结果，若后续再叠加时区、毫秒精度或时钟源信息，建议改为显式响应结构体。
// 路由：/api/v1/systime
func (*SystemApi) HandleSystime(c *gin.Context) {
	c.Set("data", map[string]interface{}{"systime": utils.GetSecondTimestamp()})
}

// HealthCheck 提供最基础的存活探针。
// 绑定/claims：不读取请求体、不读取 claims，只要 HTTP 服务可达就会返回成功。
// 边界说明：该接口只证明 Gin 路由链和当前进程仍在响应，请不要把它等同于“系统全部依赖健康”。
// 静态审查建议：如果未来接入 Kubernetes 或网关探针，建议把 /health 保持为轻量检查，再单独提供 readiness 接口。
// 路由：/health
func (*SystemApi) HealthCheck(c *gin.Context) {
	c.Set("data", nil)
}

// HandleSysVersion 返回后端编译时注入的系统版本号。
// 绑定/claims：不绑定参数，不依赖 claims，直接从全局常量读取版本信息。
// 边界说明：这里展示的是当前运行包的版本标识，是否与前端版本、数据库迁移版本或协议插件版本匹配，需要由上层发布流程额外保证。
// 静态审查建议：若后续要补充 commit、构建时间或构建目标平台，建议统一封装版本元数据结构，避免接口字段零散演进。
// 路由：/api/v1/sys_version
func (*SystemApi) HandleSysVersion(c *gin.Context) {
	c.Set("data", map[string]interface{}{"version": global.SYSTEM_VERSION})
}
