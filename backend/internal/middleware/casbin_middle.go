// 文件用途：提供 HTTP 请求链路中的 casbin middle 中间件能力。
// 核心逻辑：在 Gin 请求处理前后执行认证、鉴权、跨域、指标、响应包装或操作日志处理，主要围绕 func CasbinRBAC 等声明展开。
// 关键注意事项：中间件位于安全与兼容边界，修改需保持状态码、上下文键和响应格式稳定。
// 重构建议：后续可将外部依赖抽成接口，便于独立测试和不同部署模式复用。

package middleware

import (
	"net/http"

	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// 采用casbin，如果资源在表中，就需要校验，不在表中不做校验
// RBAC：用户-角色-功能-资源-动作

func CasbinRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		url := strings.TrimLeft(c.Request.URL.Path, "/")
		// 判断接口是否注册进 casbin 资源表（GetUrl 未注册返回 false，未注册的接口不做校验）
		isVerify := service.GroupApp.Casbin.GetUrl(url)
		if isVerify {
			// claims 由前置 JWT 中间件写入；缺失或类型不符时 fail-closed，
			// 避免像 MustGet 硬断言那样让中间件顺序错乱时直接 panic。
			claimsValue, exists := c.Get("claims")
			if !exists {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			userClaims, ok := claimsValue.(*utils.UserClaims)
			if !ok || userClaims == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			isSuccess := service.GroupApp.Casbin.Verify(userClaims.ID, url)
			if !isSuccess {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "非法访问"})
				return
			}
		}
	}
}
