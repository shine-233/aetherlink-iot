package service

import (
	"fmt"
	"strconv"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
)

// findTopLevelGatewayForCommand 递归查找顶层网关（保留原有方法）
func findTopLevelGatewayForCommand(deviceInfo *model.Device, deviceType string) (*model.Device, error) {
	currentDevice := deviceInfo

	// 如果是子设备(3)，先找到它的父设备
	if deviceType == "3" {
		if deviceInfo.ParentID == nil {
			return nil, fmt.Errorf("子设备的parentID为空")
		}
		parentDevice, err := initialize.GetDeviceCacheById(*deviceInfo.ParentID)
		if err != nil {
			return nil, fmt.Errorf("获取父设备信息失败: %v", err)
		}
		currentDevice = parentDevice
	}

	// 递归查找顶层网关（parent_id为空的设备）
	maxDepth := 10 // 防止无限循环
	depth := 0

	for currentDevice.ParentID != nil && depth < maxDepth {
		parentDevice, err := initialize.GetDeviceCacheById(*currentDevice.ParentID)
		if err != nil {
			return nil, fmt.Errorf("获取父设备信息失败: %v", err)
		}
		currentDevice = parentDevice
		depth++
	}

	if depth >= maxDepth {
		return nil, fmt.Errorf("网关层级过深，超过最大深度限制")
	}

	// 确保找到的是网关设备（device_type=2）
	if currentDevice.DeviceConfigID != nil {
		deviceConfig, err := dal.GetDeviceConfigByID(*currentDevice.DeviceConfigID)
		if err != nil {
			return nil, fmt.Errorf("获取设备配置失败: %v", err)
		}
		if deviceConfig.DeviceType != strconv.Itoa(constant.GATEWAY_DEVICE) {
			return nil, fmt.Errorf("顶层设备不是网关类型")
		}
	}

	return currentDevice, nil
}

// transformCommandDataForMultiLevelGateway 为多层网关构建命令数据格式（保留原有方法）
func transformCommandDataForMultiLevelGateway(payloadMap map[string]interface{}, deviceInfo *model.Device, deviceType string) (map[string]interface{}, error) {
	if deviceInfo == nil {
		return nil, fmt.Errorf("设备信息为空")
	}
	switch deviceType {
	case "3":
		return buildSubDeviceCommandPayload(deviceInfo, payloadMap)
	case "2":
		return buildGatewayCommandPayload(deviceInfo, payloadMap)
	default:
		return payloadMap, nil
	}
}

func buildSubDeviceCommandPayload(deviceInfo *model.Device, payloadMap map[string]interface{}) (map[string]interface{}, error) {
	if deviceInfo.ParentID == nil {
		return nil, fmt.Errorf("子设备的parentID为空")
	}
	subDeviceAddr, err := commandDeviceAddress(deviceInfo, "子设备")
	if err != nil {
		return nil, err
	}

	parentGateway, err := initialize.GetDeviceCacheById(*deviceInfo.ParentID)
	if err != nil {
		return nil, fmt.Errorf("获取父设备信息失败: %v", err)
	}
	if parentGateway.ParentID != nil {
		if _, err := commandDeviceAddress(parentGateway, "父网关"); err != nil {
			return nil, err
		}
		return buildNestedSubGatewayDataForCommand(parentGateway, subDeviceAddr, payloadMap)
	}
	return commandSubDeviceEnvelope(subDeviceAddr, payloadMap), nil
}

func buildGatewayCommandPayload(deviceInfo *model.Device, payloadMap map[string]interface{}) (map[string]interface{}, error) {
	if deviceInfo.ParentID == nil {
		return commandGatewayEnvelope(payloadMap), nil
	}
	gatewayAddr, err := commandDeviceAddress(deviceInfo, "子网关")
	if err != nil {
		return nil, err
	}
	return commandSubGatewayEnvelope(gatewayAddr, commandGatewayEnvelope(payloadMap)), nil
}

// buildNestedSubGatewayDataForCommand 递归构建多层子网关的嵌套命令数据结构（保留原有方法）
func buildNestedSubGatewayDataForCommand(gateway *model.Device, subDeviceAddr string, payloadMap map[string]interface{}) (map[string]interface{}, error) {
	if gateway.ParentID == nil {
		return commandSubDeviceEnvelope(subDeviceAddr, payloadMap), nil
	}

	parentGateway, err := initialize.GetDeviceCacheById(*gateway.ParentID)
	if err != nil {
		gatewayAddr, addrErr := commandDeviceAddress(gateway, "子网关")
		if addrErr != nil {
			return nil, addrErr
		}
		return commandSubGatewayEnvelope(gatewayAddr, commandSubDeviceEnvelope(subDeviceAddr, payloadMap)), nil
	}

	gatewayAddr, err := commandDeviceAddress(gateway, "子网关")
	if err != nil {
		return nil, err
	}
	innerData, err := buildNestedSubGatewayDataForCommand(parentGateway, subDeviceAddr, payloadMap)
	if err != nil {
		return nil, err
	}
	if parentGateway.ParentID != nil {
		return commandSubGatewayEnvelope(gatewayAddr, innerData), nil
	}
	return commandSubGatewayEnvelope(gatewayAddr, commandSubDeviceEnvelope(subDeviceAddr, payloadMap)), nil
}

func commandDeviceAddress(device *model.Device, role string) (string, error) {
	if device == nil || device.SubDeviceAddr == nil {
		return "", fmt.Errorf("%s的SubDeviceAddr为空", role)
	}
	return *device.SubDeviceAddr, nil
}

func commandSubDeviceEnvelope(subDeviceAddr string, payloadMap map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"sub_device_data": map[string]interface{}{
			subDeviceAddr: payloadMap,
		},
	}
}

func commandGatewayEnvelope(payloadMap map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"gateway_data": payloadMap,
	}
}

func commandSubGatewayEnvelope(subGatewayAddr string, payloadMap map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"sub_gateway_data": map[string]interface{}{
			subGatewayAddr: payloadMap,
		},
	}
}
