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
	EntityRelation // P1.1 通用实体关系
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
	PluginRegistry // PHASE-D-D9 插件框架 gRPC 网关
	ReportSchedule  // PHASE-D-D3 定时报表
	DeviceCertificate // PHASE-D-D5 接入安全 X.509
	EdgeSync // PHASE-D-D6 边缘计算 2.0
	AiModel  // PHASE-D-D7 AI 2.0 模型中心 + 助手
	Scada    // P1.3 Widget 与 SCADA 基础层
	Mobile   // P1.4 移动端控制与通知
}

var Model = new(apps)
