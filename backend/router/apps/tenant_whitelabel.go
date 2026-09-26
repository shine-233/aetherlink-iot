// 文件用途：白标（TB-47）租户翻译覆盖与自定义 CSS 路由定义。
// 核心逻辑：挂载 /api/v1/whitelabel 路由组并关联 TenantWhitelabelApi：
//
//	translations（GET 列表 / PUT 批量 UPSERT / DELETE 批量删除）、
//	custom-css（GET 回显 / PUT 写入）、overrides（登录后可读的覆盖获取）。
//
// 关键注意事项：路由组挂在 CasbinRBAC 中间件之后（router_init.go），路由路径、
//
//	方法和中间件直接影响前端与自动化接口契约；新增/改名路由必须同步在迁移
//	（134.sql）做 Casbin g2/p 登记，否则启动审计 fail-fast。
//
// 重构建议：若后续把白标能力扩展到"全局行向租户级联"或菜单改名/隐藏
//
//	（tenant_dashboard_menus），另立子组并单独评审授权，不动本管理面路由。
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

// WhitelabelRouter 白标路由组。
type WhitelabelRouter struct{}

// InitWhitelabel 挂载白标路由（/api/v1/whitelabel/*，Casbin 保护，134.sql 登记）。
func (*WhitelabelRouter) InitWhitelabel(Router *gin.RouterGroup) {
	r := Router.Group("whitelabel")
	{
		r.GET("translations", api.Controllers.TenantWhitelabelApi.ListTenantTranslations)
		r.PUT("translations", api.Controllers.TenantWhitelabelApi.UpsertTenantTranslations)
		r.DELETE("translations", api.Controllers.TenantWhitelabelApi.DeleteTenantTranslations)
		r.GET("custom-css", api.Controllers.TenantWhitelabelApi.GetTenantCustomCSS)
		r.PUT("custom-css", api.Controllers.TenantWhitelabelApi.UpsertTenantCustomCSS)
		r.GET("overrides", api.Controllers.TenantWhitelabelApi.GetWhitelabelOverrides)
	}
}
