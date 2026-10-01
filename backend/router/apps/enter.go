// 文件用途：注册入口认证相关的应用路由。
// 核心逻辑：在 Gin 路由组上挂载 URL、HTTP 方法和对应 api 处理器。
// 关键注意事项：路由路径、方法和中间件会直接影响前端与自动化接口契约。
// 重构建议：路由数量继续增长时，优先按业务域抽取公共分组和权限挂载辅助函数。
package apps

type apps struct {
	User
	Role
	Casbin
	Dict
	OTA
	UpLoad
	ProtocolPlugin
	Device
	UiElements
	Board
	EventData
	TelemetryData
	AttributeData
	CommandData
	EntityRelation    // P1.1 通用实体关系
	TelemetryAnalysis // P2.2 轻量分析
	OperationLog
	Logo
	DataPolicy
	DeviceConfig
	DataScript
	NotificationGroup
	NotificationHistoryGroup
	NotificationServicesConfig
	Alarm
	SceneAutomations
	Scene
	SysFunction
	ServicePlugin
	ExpectedData
	OpenAPIKey
	MessagePush
	SystemMonitor
	DeviceAuth
	DashboardMenu
	DeviceShadow
	DeviceModbusProfile
	AiQuery
	RuleChain
	RDI
	PayloadSchema
	CalculatedField
	Product
	EntityVersion
	Asset
	UserTOTP
	OidcSso
	PluginRegistry        // PHASE-D-D9 插件框架 gRPC 网关
	ReportSchedule        // PHASE-D-D3 定时报表
	DeviceCertificate     // PHASE-D-D5 接入安全 X.509
	EdgeSync              // PHASE-D-D6 边缘计算 2.0
	AiModel               // PHASE-D-D7 AI 2.0 模型中心 + 助手
	Scada                 // P1.3 Widget 与 SCADA 基础层
	Mobile                // P1.4 移动端控制与通知
	ResourceCenter        // TP-5 资源中心
	RateLimitRouter       // TB-7 集群限流
	QueueMonitorRouter    // TB-7 队列隔离监控
	UnitsRouter           // TB-9 单位换算与物理量纲
	SecretsRouter         // TB-18 通用 Secrets Storage
	IndustrySolution      // TB-19 解决方案模板引擎
	Tenant                // P3 租户管理与自助开通
	Billing               // P3 商业化计费与用量计量
	DataConverterRouter   // ThingsBoard 核心数据转换器
	IntegrationRouter     // TB-45 统一集成实体（Integration 纳管管线）
	UserGroupRouter       // TB-46 用户组与组权限（GPE v1）
	Customer              // ThingsBoard 核心客户管理体系
	WidgetBundleRouter    // TB-04 部件库（widget_bundles）
	MediaLibraryRouter    // TB-41 文件存储与媒体库（media_files）
	MobileAppBundleRouter // TB-23 移动应用中心（mobile_app_bundles）
	WhitelabelRouter      // TB-47 白标：租户翻译覆盖 + 自定义 CSS（134.sql 登记）
	SchedulerRouter       // TB-48 统一调度器（scheduler_events + 三源聚合，137.sql 登记）
}

var Model = new(apps)
