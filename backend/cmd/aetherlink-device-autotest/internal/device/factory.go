// 文件用途：按配置选择并创建设备测试实例。
// 核心逻辑：将配置中的 device_type 映射到直连设备或网关设备构造函数，并对未知类型返回错误。
// 关键注意事项：新增设备类型必须同步配置校验、topic 构建和测试说明，避免入口可选但测试不可用。
// 重构建议：设备类型继续增加时可改为注册表模式，降低 factory switch 与实现包的持续耦合。
package device

import (
	"fmt"

	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
)

// DeviceType 设备类型
type DeviceType string

const (
	// DeviceTypeDirect 直连设备
	DeviceTypeDirect DeviceType = "direct"
	// DeviceTypeGateway 网关设备
	DeviceTypeGateway DeviceType = "gateway"
)

// NewDevice 根据配置创建设备实例
func NewDevice(cfg *config.Config, logger *zap.Logger) (Device, error) {
	switch DeviceType(cfg.DeviceType) {
	case DeviceTypeDirect:
		return NewDirectDevice(cfg, logger), nil
	case DeviceTypeGateway:
		return NewGatewayDevice(cfg, logger), nil
	default:
		return nil, fmt.Errorf("unknown device type: %s", cfg.DeviceType)
	}
}
