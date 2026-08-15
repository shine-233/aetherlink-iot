// 文件用途：描述网关、子网关和子设备的拓扑结构。
// 核心逻辑：使用 YAML 标签承载多层网关配置，供网关测试构造嵌套上报数据和目标设备断言。
// 关键注意事项：当前结构与 config 包中的网关配置存在重复定义，审查时需确认两处字段保持一致。
// 重构建议：可合并到单一拓扑模型或由 config 包导出复用，避免配置字段漂移。
package device

// SubDeviceConfig 子设备配置
type SubDeviceConfig struct {
	SubDeviceNumber string `yaml:"sub_device_number"` // 子设备编号
	DeviceID        string `yaml:"device_id"`         // 设备ID
	Description     string `yaml:"description"`       // 描述
}

// SubGatewayConfig 子网关配置
type SubGatewayConfig struct {
	SubGatewayNumber string            `yaml:"sub_gateway_number"` // 子网关编号
	DeviceID         string            `yaml:"device_id"`          // 设备ID
	Description      string            `yaml:"description"`        // 描述
	SubDevices       []SubDeviceConfig `yaml:"sub_devices"`        // 子网关下的子设备
}

// GatewayTopology 网关拓扑结构
type GatewayTopology struct {
	SubDevices  []SubDeviceConfig  `yaml:"sub_devices"`  // 直连子设备
	SubGateways []SubGatewayConfig `yaml:"sub_gateways"` // 子网关
}
