// 文件用途：创建并装配后端 Gin HTTP 路由。
// 核心逻辑：挂载 Swagger、metrics、静态文件、公共 API、JWT、Casbin、SSE 和各业务模块路由。
// 关键注意事项：路径字符串是前后端和自动化共享契约，`/files/*filepath` 必须继续走安全 resolver。
// 重构建议：后续可拆分公开路由、鉴权路由、业务模块路由和运维路由注册函数，降低单文件复杂度。
package router

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	middleware "aetherlink-iot/backend/internal/middleware"
	"aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/metrics"
	"aetherlink-iot/backend/router/apps"
	"aetherlink-iot/backend/router/publicfiles"
	"aetherlink-iot/backend/static"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	// gin-swagger middleware
	_ "aetherlink-iot/backend/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	api "aetherlink-iot/backend/internal/api"
	service "aetherlink-iot/backend/internal/service"
)

// swagger embed files

// isInlineSafeFileContentType 返回允许浏览器内联渲染的内容类型白名单。
// 明确排除 text/html、application/xhtml+xml 与 image/svg+xml：它们可执行脚本，
// 一旦落入同源 /files 路径就等于存储型 XSS 落点。
func isInlineSafeFileContentType(contentType string) bool {
	base := strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch {
	case base == "text/plain", base == "application/pdf":
		return true
	case strings.HasPrefix(base, "audio/"), strings.HasPrefix(base, "video/"):
		return true
	case strings.HasPrefix(base, "image/"):
		return base != "image/svg+xml"
	default:
		return false
	}
}

func RouterInit() *gin.Engine {
	// 生产默认 Release 模式；测试进程会自行 SetMode(gin.TestMode) 覆盖。
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	// 必须在任何路由注册前挂载，确保 Swagger、metrics、静态文件和 404 都可关联且带基础安全头。
	router.Use(middleware.RequestID())
	router.Use(middleware.SecurityHeaders())
	// 运维暴露面收敛（P3，2026-08-24，见 VALIDATION.md）：/swagger、/metrics、/metrics-viewer
	// 均无业务认证，生产部署（GOTP_ENV=production）下跳过注册，避免接口契约与指标数据对外暴露；
	// 非生产环境保持原样，供开发调试与自动化验证使用。
	opsEndpointsEnabled := os.Getenv("GOTP_ENV") != "production"
	if opsEndpointsEnabled {
		// Swagger文档路由
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// 创建 metrics 收集器
	m := metrics.NewMetrics("AetherLinkIoT")
	// 创建内存存储实现
	memStorage := metrics.NewMemoryStorage()
	// 设置存储实现
	m.SetHistoryStorage(memStorage)
	// 开始定期收集系统指标(每15秒)
	m.StartMetricsCollection(15 * time.Second)
	// 注册 metrics 中间件
	router.Use(middleware.MetricsMiddleware(m))
	if opsEndpointsEnabled {
		// 注册 prometheus metrics 接口
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	// 设置metrics管理器到系统监控服务
	service.SetMetricsManager(m)

	// 添加静态文件路由（嵌入二进制，不依赖运行时工作目录）
	if opsEndpointsEnabled {
		router.GET("/metrics-viewer", func(c *gin.Context) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", static.MetricsViewerHTML)
		})
		router.GET("/metrics-viewer/echarts.min.js", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/javascript; charset=utf-8", static.MetricsViewerEChartsJS)
		})
	}

	// 处理文件访问请求
	router.GET("/files/*filepath", func(c *gin.Context) {
		relativePath, err := publicfiles.ResolveRelativePath(c.Param("filepath"))
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		root, err := os.OpenRoot("./files")
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer root.Close()
		file, err := root.Open(filepath.FromSlash(relativePath))
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		contentType := mime.TypeByExtension(filepath.Ext(relativePath))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		// 上传目录内容不可全信：禁止嗅探，且除内联安全类型外一律按附件下发，
		// 防止 d_plugin 等免签名校验类型被同源渲染成 HTML/SVG 造成存储型 XSS。
		c.Header("X-Content-Type-Options", "nosniff")
		disposition := "attachment"
		if isInlineSafeFileContentType(contentType) {
			disposition = "inline"
		}
		if cd := mime.FormatMediaType(disposition, map[string]string{"filename": filepath.Base(relativePath)}); cd != "" {
			c.Header("Content-Disposition", cd)
		} else {
			c.Header("Content-Disposition", disposition)
		}
		c.DataFromReader(http.StatusOK, info.Size(), contentType, file, nil)
	})

	// router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.Use(middleware.Cors())
	// 初始化响应处理器
	handler, err := response.NewHandler("configs/messages.yaml", "configs/messages_str.yaml")
	if err != nil {
		logrus.Fatalf("初始化响应处理器失败: %v", err)
	}

	// 记录操作日志
	router.Use(middleware.OperationLogs())
	// 全局使用
	global.ResponseHandler = handler
	// 使用中间件
	router.Use(handler.Middleware())

	controllers := new(api.Controller)
	// /health is process liveness; /ready checks required local/core dependencies.
	router.GET("/health", controllers.SystemApi.HealthCheck)
	router.GET("/ready", controllers.SystemApi.Readiness)
	router.GET("/deployment/health", controllers.SystemApi.DeploymentHealth)

	api := router.Group("api")
	// 启动期 Casbin 覆盖审计的基线容器：在挂载 CasbinRBAC 前声明，快照在挂载点后写入。
	var casbinBaselineRoutes []string
	{
		// 无需权限校验
		v1 := api.Group("v1")
		{
			// v1.GET("notice/test", controllers.NoticeTest)
			// 协议插件接入端点：边界认证见 middleware.PluginAuth
			// （配置 plugin.service.key 后全来源严格校验 X-Plugin-Key；未配置仅放行回环/私网）。
			plugin := v1.Group("", middleware.PluginAuth())
			{
				plugin.POST("plugin/heartbeat", controllers.Heartbeat)
				plugin.POST("plugin/device/config", controllers.HandleDeviceConfigForProtocolPlugin)
				plugin.POST("plugin/devices", controllers.HandleDeviceConfigForProtocolPluginByProtocolType)
				plugin.POST("plugin/service/access/list", controllers.HandlePluginServiceAccessList)
				plugin.POST("plugin/service/access", controllers.HandlePluginServiceAccess)
			}
			v1.POST("login", controllers.Login)
			v1.GET("verification/code", controllers.HandleVerificationCode)
			v1.POST("reset/password/link", controllers.RequestPasswordResetLink)
			v1.POST("reset/password", controllers.ResetPassword)
			v1.GET("logo", controllers.HandleLogoList)
			// 设备遥测（ws）
			v1.GET("telemetry/datas/current/ws", controllers.TelemetryDataApi.ServeCurrentDataByWS)
			// 设备在线离线状态（ws） - 兼容旧实现
			v1.GET("device/online/status/ws", controllers.TelemetryDataApi.ServeDeviceStatusByWS)
			// 设备在线离线状态（ws） - 新批量订阅实现（首次消息鉴权，支持 device_ids）
			v1.GET("device/online/status/ws/batch", controllers.TelemetryDataApi.ServeDeviceOnlineStatusWS)
			// 设备遥测keys（ws）
			v1.GET("telemetry/datas/current/keys/ws", controllers.TelemetryDataApi.ServeCurrentDataByKey)
			v1.GET("ota/download/files/upgradePackage/:path/:file", controllers.OTAApi.DownloadOTAUpgradePackage)
			v1.GET("rdi/shared/:token", controllers.RDIApi.SharedDeviceConfig)
			v1.GET("board/shared/:token", controllers.BoardApi.GetPublishedBoardByShareToken)
			// 获取系统时间
			v1.GET("systime", controllers.SystemApi.HandleSystime)
			v1.GET("deployment/health", controllers.SystemApi.DeploymentHealth)
			// 查询系统功能设置
			v1.GET("sys_function", controllers.SysFunctionApi.HandleSysFcuntion)
			// 租户邮箱注册
			v1.POST("/tenant/email/register", controllers.UserApi.EmailRegister)
			// 检查是否存在超管
			v1.GET("/tenant/has-admin", controllers.UserApi.HasAdmin)
			// 首次安装状态
			v1.GET("/tenant/setup-state", controllers.UserApi.SetupState)
			// 首次安装超管初始化（语义化新接口）
			v1.POST("/tenant/super-admin/init", controllers.UserApi.InitSuperAdmin)
			// 超管注册（联动市场）
			v1.POST("/tenant/market-register", controllers.UserApi.MarketRegister)
			// 网关自动注册
			v1.POST("/device/gateway-register", controllers.DeviceApi.GatewayRegister)
			// 网关子设备注册
			v1.POST("/device/gateway-sub-register", controllers.DeviceApi.GatewaySubRegister)
			// 获取系统版本
			v1.GET("sys_version", controllers.SystemApi.HandleSysVersion)
			// 设备动态认证（一型一密）
			v1.POST("/device/auth", controllers.DeviceAuthApi.DeviceAuth)
		}

		// 需要权限校验
		v1.Use(middleware.JWTAuth())

		// per-tenant API 限流（对标 TB PE 能力）：JWT 后全量业务接口统一计数；
		// 阈值 api-rate-limit.requests-per-minute（默认 600，<=0 关闭），超限返回 429+Retry-After。
		v1.Use(middleware.TenantRateLimit())

		// 设备诊断需要登录态 claims，但不走 Casbin 菜单权限。
		v1.GET("/devices/:device_id/diagnostics", controllers.DeviceApi.GetDeviceDiagnostics)

		// 需要权限校验
		v1.Use(middleware.CasbinRBAC())
		// 启动期 Casbin 覆盖审计基线：此快照之后注册的一切路由都视为"受 Casbin 保护"，
		// 必须登记进资源表，否则 auditCasbinRouteCoverage 会在启动期阻断（见 casbin_audit.go）。
		casbinBaselineRoutes = ginRoutePaths(router)
		// SSE服务
		SSERouter(v1)

		{
			apps.Model.User.InitUser(v1) // 用户模块

			apps.Model.Role.Init(v1) // 角色管理

			apps.Model.Casbin.Init(v1) // 权限管理

			apps.Model.Dict.InitDict(v1) // 字典模块

			apps.Model.OTA.InitOTA(v1) // OTA模块

			apps.Model.UpLoad.Init(v1) // 文件上传

			apps.Model.ProtocolPlugin.InitProtocolPlugin(v1) // 协议插件模块

			apps.Model.Device.InitDevice(v1) // 设备

			apps.Model.UiElements.Init(v1) // UI元素控制

			apps.Model.Board.InitBoard(v1) // 首页

			apps.Model.DashboardMenu.Init(v1) // 仪表盘菜单绑定

			apps.Model.EventData.InitEventData(v1) // 事件数据

			apps.Model.TelemetryData.InitTelemetryData(v1) // 遥测数据

			apps.Model.AttributeData.InitAttributeData(v1) // 属性数据

			apps.Model.CommandData.InitCommandData(v1) // 命令数据

			apps.Model.OperationLog.Init(v1) // 操作日志

			apps.Model.Logo.Init(v1) // logo

			apps.Model.DataPolicy.Init(v1) // 数据清理

			apps.Model.DeviceConfig.Init(v1) // 设备配置

			apps.Model.Product.Init(v1) // 产品选择列表

			apps.Model.DataScript.Init(v1) // 数据处理脚本

			apps.Model.NotificationGroup.InitNotificationGroup(v1) // 通知组

			apps.Model.NotificationHistoryGroup.InitNotificationHistory(v1) // 通知组

			apps.Model.NotificationServicesConfig.Init(v1) // 通知服务配置

			apps.Model.Alarm.Init(v1) // 告警模块

			apps.Model.Scene.Init(v1) // 场景

			apps.Model.SceneAutomations.Init(v1) // 场景联动

			apps.Model.SysFunction.Init(v1) // 功能设置

			apps.Model.ServicePlugin.Init(v1) // 插件管理

			apps.Model.ExpectedData.InitExpectedData(v1)

			apps.Model.DeviceShadow.InitDeviceShadow(v1) // 设备影子（离线命令缓存）

			apps.Model.AiQuery.InitAiQuery(v1) // AI 集成（自然语言查询遥测）

			apps.Model.DeviceModbusProfile.InitDeviceModbusProfile(v1) // Modbus 点表配置

			apps.Model.RuleChain.InitRuleChain(v1) // 规则链（可视化 DAG 编排）

			apps.Model.OpenAPIKey.InitOpenAPIKey(v1)

			apps.Model.MessagePush.Init(v1)

			apps.Model.RDI.InitRDI(v1)

			apps.Model.PayloadSchema.InitPayloadSchema(v1) // payload schema 静态校验

			apps.Model.CalculatedField.InitCalculatedField(v1) // 计算字段（遥测派生指标）

			apps.Model.Tenant.InitTenant(v1) // 租户客户层级（组织架构管理）

			// 初始化系统监控路由
			apps.Model.SystemMonitor.InitSystemMonitor(v1, m)
		}
	}

	// 启动期 Casbin 覆盖检查：默认 fail-fast（P1 批交付，casbin.route-audit-mode 可配 warn/off）；
	// 运维显式选择 off 时退回 #178 引入的只警报报告，保底可观测性。
	if strings.EqualFold(strings.TrimSpace(viper.GetString("casbin.route-audit-mode")), "off") {
		LogCasbinRegistrationGaps(router)
	} else {
		auditCasbinRouteCoverage(router, casbinBaselineRoutes)
	}

	return router
}
