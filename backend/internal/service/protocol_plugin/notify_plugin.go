// 文件用途：封装协议插件侧的设备配置变更和断连通知。
// 核心逻辑：根据设备配置、设备快照和插件 HTTP 地址调用插件管理接口，协调设备删除或配置更新后的外部副作用。
// 关键注意事项：插件调用属于跨进程副作用，设备快照、协议类型和地址为空时必须保持 fail-safe，避免误断连其他设备。
// 重构建议：抽出插件客户端接口，补齐插件不可达、部分失败、重复通知和删除事务之后副作用顺序的测试。
package protocolplugin

import (
	"fmt"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/pluginruntime"

	"github.com/sirupsen/logrus"
)

// 设备配置更新后主动断开设备连接
func DeviceConfigUpdateAndDisconnect(deviceConfigID string, protocolType string, deviceType string) error {

	// 根据协议类型获取协议信息
	servicePlugin, err := dal.GetServicePluginByServiceIdentifier(protocolType)
	if err != nil {
		return err
	}
	// 获取协议插件host:
	_, host, err := dal.GetServicePluginHttpAddressByID(servicePlugin.ID)
	if err != nil {
		return err
	}
	// 通知所有相关网关断开连接
	if deviceType == "3" {
		// 获取已绑定网关的关联的子设备列表
		deviceIDs, err := dal.GetGatewayDevicesBySubDeviceConfigID(deviceConfigID)
		if err != nil {
			return err
		}
		// 断开设备连接
		for _, deviceID := range deviceIDs {
			DisconnectDevice(deviceID, host)
		}
	} else if deviceType == "1" || deviceType == "2" {
		// 根据设备配置ID获取设备列表
		devices, err := dal.GetDevicesByDeviceConfigID(deviceConfigID)
		if err != nil {
			return err
		}
		// 断开设备连接
		for _, device := range devices {
			DisconnectDevice(device.ID, host)
		}
		return nil
	}
	return nil

}

// 通知协议插件
func DisconnectDevice(deviceID string, httpAddress string) error {
	err := pluginruntime.Current().DisconnectDevice(httpAddress, deviceID)
	if err != nil {
		logrus.Warnf("update succeeded, but connect plugin failed: %s", err)
	}
	return err
}

// 根据设备ID通知协议插件
// 修改设备调用
// 删除设备调用
// 新增网关子设备的时候使用（deviceID送网关设备ID）
// 移除网关子设备调用
func DisconnectDeviceByDeviceID(deviceID string) error {
	// 获取设备信息
	device, err := dal.GetDeviceByID(deviceID)
	if err != nil {
		return err
	}
	return DisconnectDeviceByDeviceSnapshot(device)
}

// DisconnectDeviceByDeviceSnapshot notifies the protocol plugin using a device
// row loaded before delete commit, avoiding a post-delete reload.
func DisconnectDeviceByDeviceSnapshot(device *model.Device) error {
	if device == nil {
		return nil
	}
	if device.DeviceConfigID == nil {
		return nil
	}
	// 获取设备配置
	deviceConfig, err := dal.GetDeviceConfigByID(*device.DeviceConfigID)
	if err != nil {
		return err
	}
	if deviceConfig == nil {
		return nil
	}
	if deviceConfig.ProtocolType == nil {
		return fmt.Errorf("protocol type not found")
	}
	if *deviceConfig.ProtocolType == "MQTT" {
		return nil
	}
	// 根据协议类型获取协议信息
	servicePlugin, err := dal.GetServicePluginByServiceIdentifier(*deviceConfig.ProtocolType)
	if err != nil {
		return err
	}
	// 获取协议插件host:
	_, host, err := dal.GetServicePluginHttpAddressByID(servicePlugin.ID)
	if err != nil {
		return err
	}
	// 断开设备连接
	if deviceConfig.DeviceType == "3" {
		if device.ParentID == nil {
			return nil
		}
		err = DisconnectDevice(*device.ParentID, host)
		if err != nil {
			return err
		}
	} else {
		err = DisconnectDevice(device.ID, host)
		if err != nil {
			return err
		}
	}
	return nil
}
