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
	RDI
	PayloadSchema
}

var Model = new(apps)
