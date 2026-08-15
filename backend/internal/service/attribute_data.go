// 文件用途：维护设备属性读写、属性模型和属性下行业务。
// 核心逻辑：校验设备访问权限，解析属性定义与当前值，并把写入请求转换为下行总线调用。
// 关键注意事项：属性写入可能影响真实设备状态，权限失败和参数错误必须在下行副作用前返回。
// 重构建议：抽出属性仓储与下行发送接口，补齐跨租户、协议差异、事务和下行失败测试。
// attribute_data.go owns device attribute service behavior.
//
// It handles attribute reads/writes, publication metadata, and device-facing
// attribute state used by detail pages, MQTT downlink paths, and automation.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/downlink"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type AttributeData struct {
	downlinkBus *downlink.Bus
}

func (*AttributeData) GetAttributeDataList(device_id string, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(device_id, claims); err != nil {
		return nil, err
	}

	data, err := dal.GetAttributeDataListWithDeviceName(device_id)
	if err != nil {
		return nil, err
	}

	var easyData []map[string]interface{}
	for _, v := range data {
		d := make(map[string]interface{})
		d["id"] = v["id"]
		d["device_id"] = device_id
		d["ts"] = v["ts"]
		d["key"] = v["key"]
		d["data_name"] = v["data_name"]
		d["unit"] = v["unit"]
		if v["string_v"] != nil {
			d["value"] = v["string_v"]
		}

		if v["bool_v"] != nil {
			d["value"] = v["bool_v"]
		}

		if v["number_v"] != nil {
			d["value"] = v["number_v"]
		}

		if v["read_write_flag"] != nil {
			d["read_write_flag"] = v["read_write_flag"]
		}

		easyData = append(easyData, d)
	}

	return easyData, nil
}

func (*AttributeData) DeleteAttributeData(id string, claims *utils.UserClaims) error {
	data, err := dal.GetAttributeDataByID(id)
	if err != nil {
		return err
	}
	if _, err := ensureTelemetryDeviceWriteAccess(data.DeviceID, claims); err != nil {
		return err
	}

	err = dal.DeleteAttributeData(id)
	return err
}

func (*AttributeData) GetAttributeSetLogsDataListByPage(req model.GetAttributeSetLogsListByPageReq, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(req.DeviceId, claims); err != nil {
		return nil, err
	}

	count, data, err := dal.GetAttributeSetLogsDataListByPage(req)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	dataMap := make(map[string]interface{})
	dataMap["count"] = count
	dataMap["list"] = data

	return dataMap, nil
}

// 根据key查询设备属性
func (*AttributeData) GetAttributeDataByKey(req model.GetDataListByKeyReq, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(req.DeviceId, claims); err != nil {
		return nil, err
	}

	dataMap := make(map[string]interface{})

	data, err := dal.GetAttributeDataByKey(req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dataMap, nil
		}
		return dataMap, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	dataMap["id"] = data.ID
	dataMap["key"] = data.Key
	dataMap["device_id"] = data.DeviceID
	dataMap["ts"] = data.T
	if data.BoolV != nil {
		dataMap["value"] = data.BoolV
	} else if data.NumberV != nil {
		dataMap["value"] = data.NumberV
	} else if data.StringV != nil {
		dataMap["value"] = *data.StringV
	} else {
		dataMap["value"] = nil
	}

	return dataMap, nil
}

// SetDownlinkBus 设置 downlink Bus（在 Application 初始化时调用）
func (a *AttributeData) SetDownlinkBus(bus *downlink.Bus) {
	a.downlinkBus = bus
}

// AttributePutMessage 属性设置下发（改造为异步模式，支持多层网关）
func (a *AttributeData) AttributePutMessage(ctx context.Context, operatorID string, putMessageReq *model.AttributePutMessage, operationType string, claimsOpt ...*utils.UserClaims) error {
	if err := ensureAttributeWriteAccess(putMessageReq.DeviceID, claimsOpt...); err != nil {
		return err
	}

	profile, err := loadAttributeSetDeviceProfile(putMessageReq.DeviceID)
	if err != nil {
		return err
	}

	jsonData, err := buildAttributeSetPayload(putMessageReq, profile.device, profile.deviceType)
	if err != nil {
		return err
	}

	targetDevice, targetDeviceNumber, topicPrefix, err := a.resolveDeviceInfo(profile.device, profile.deviceType, profile.protocolType)
	if err != nil {
		return err
	}

	messageID := uuid.New()[:8]
	if err := a.createAttributeLog(profile.device, messageID, putMessageReq.Value, operationType); err != nil {
		logrus.WithError(err).Error("Failed to create attribute log")
		// 不阻塞发送流程
	}

	return a.publishAttributeSet(profile.device, targetDevice, targetDeviceNumber, profile.deviceType, topicPrefix, messageID, jsonData)
}

// resolveDeviceInfo 处理多层网关，返回目标设备、目标设备编号和Topic前缀
func (a *AttributeData) resolveDeviceInfo(device *model.Device, deviceType, protocolType string) (*model.Device, string, string, error) {
	var targetDevice *model.Device
	var targetDeviceNumber string
	var topicPrefix string

	// 根据协议类型和设备类型确定目标设备
	// MQTT协议：网关/子设备需要查找顶层网关
	// 非MQTT协议（协议插件）：直接使用设备自己，插件会处理层级关系
	if protocolType == "MQTT" && (device.ParentID != nil && *device.ParentID != "") {
		// MQTT 网关/子设备：查找顶层网关
		topGateway, err := findTopLevelGatewayForAttribute(device, deviceType)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to find top level gateway: %w", err)
		}
		targetDevice = topGateway
		targetDeviceNumber = topGateway.DeviceNumber
	} else {
		// 直连设备 或 非MQTT协议：使用设备自己
		targetDevice = device
		targetDeviceNumber = device.DeviceNumber
	}

	// 检查是否有协议插件前缀（仅非MQTT协议需要）
	if protocolType != "MQTT" && targetDevice.DeviceConfigID != nil {
		// 使用 service_plugins 表获取主题前缀
		var err error
		topicPrefix, err = dal.GetServicePluginSubTopicPrefixByDeviceConfigID(*targetDevice.DeviceConfigID)
		if err != nil {
			logrus.WithError(err).Warn("failed to get sub topic prefix from service_plugins")
		}
	}

	return targetDevice, targetDeviceNumber, topicPrefix, nil
}

// createAttributeLog 创建属性设置日志
func (a *AttributeData) createAttributeLog(device *model.Device, messageId, value, operationType string) error {
	status := "0" // pending
	log := &model.AttributeSetLog{
		ID:            uuid.New(),
		DeviceID:      device.ID,
		OperationType: &operationType,
		MessageID:     &messageId,
		Datum:         &value,
		Status:        &status,
		ErrorMessage:  nil,
		CreatedAt:     time.Now(),
	}

	return dal.CreateAttributeSetLog(log)
}

// getDeviceConfigID 获取设备配置ID
func (a *AttributeData) getDeviceConfigID(device *model.Device) string {
	if device.DeviceConfigID == nil {
		return ""
	}
	return *device.DeviceConfigID
}

func (a *AttributeData) AttributeGetMessage(claims *utils.UserClaims, req *model.AttributeGetMessageReq) error {
	logrus.Debug("AttributeGetMessage")
	if _, err := ensureTelemetryDeviceWriteAccess(req.DeviceID, claims); err != nil {
		return err
	}

	// 1. 获取设备信息
	device, err := dal.GetDeviceByID(req.DeviceID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	if device.DeviceNumber == "" {
		// 没有设备编号，不支持获取属性
		return nil
	}

	// 2. 获取设备类型和协议类型
	var deviceType string
	var protocolType string
	if device.DeviceConfigID != nil {
		deviceConfig, err := dal.GetDeviceConfigByID(*device.DeviceConfigID)
		if err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		deviceType = deviceConfig.DeviceType
		if deviceConfig.ProtocolType != nil {
			protocolType = *deviceConfig.ProtocolType
		} else {
			protocolType = "MQTT" // 默认为MQTT
		}
	} else {
		protocolType = "MQTT" // 无配置时默认为MQTT
		deviceType = "1"      // 默认为直连设备
	}

	// 3. 处理网关层级，获取目标设备信息
	targetDevice, targetDeviceNumber, topicPrefix, err := a.resolveDeviceInfo(device, deviceType, protocolType)
	if err != nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"system_error": err.Error(),
		})
	}

	// 4. 组装属性获取 payload。
	payload, err := buildAttributeGetPayload(req.Keys)
	if err != nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"system_error": err.Error(),
		})
	}

	// 5. 通过 Downlink Bus 发送属性获取请求
	return a.publishAttributeGet(device, targetDevice, targetDeviceNumber, deviceType, topicPrefix, payload)
}

// findTopLevelGatewayForAttribute 递归查找顶层网关（用于属性设置）
func findTopLevelGatewayForAttribute(deviceInfo *model.Device, deviceType string) (*model.Device, error) {
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

// transformAttributeDataForMultiLevelGateway 为多层网关构建属性数据格式
func transformAttributeDataForMultiLevelGateway(param *model.AttributePutMessage, deviceInfo *model.Device, deviceType string) error {
	// 解析JSON
	var inputData map[string]interface{}
	if err := json.Unmarshal([]byte(param.Value), &inputData); err != nil {
		return fmt.Errorf("解析输入JSON失败: %v", err)
	}

	// 根据设备类型和是否有父网关构建不同的输出数据结构
	var outputData map[string]interface{}

	if deviceType == "3" { // 子设备
		if deviceInfo.SubDeviceAddr == nil {
			return fmt.Errorf("子设备的SubDeviceAddr为空")
		}

		// 查找子设备的直接父网关（可能是子网关）
		parentGateway, err := initialize.GetDeviceCacheById(*deviceInfo.ParentID)
		if err != nil {
			return fmt.Errorf("获取父设备信息失败: %v", err)
		}

		// 如果父网关是子网关（有parent_id），需要嵌套结构
		if parentGateway.ParentID != nil {
			// 父网关是子网关，需要构建嵌套的sub_gateway_data结构
			if parentGateway.SubDeviceAddr == nil {
				return fmt.Errorf("父网关的SubDeviceAddr为空")
			}
			outputData = buildNestedSubGatewayDataForAttribute(parentGateway, *deviceInfo.SubDeviceAddr, inputData)
		} else {
			// 父网关是顶层网关，直接构建sub_device_data
			outputData = map[string]interface{}{
				"sub_device_data": map[string]interface{}{
					*deviceInfo.SubDeviceAddr: inputData,
				},
			}
		}
	} else if deviceType == "2" { // 网关设备
		if deviceInfo.ParentID != nil {
			// 子网关：构建为sub_gateway_data格式
			if deviceInfo.SubDeviceAddr == nil {
				return fmt.Errorf("子网关的SubDeviceAddr为空")
			}
			outputData = map[string]interface{}{
				"sub_gateway_data": map[string]interface{}{
					*deviceInfo.SubDeviceAddr: map[string]interface{}{
						"gateway_data": inputData,
					},
				},
			}
		} else {
			// 顶层网关：构建为gateway_data格式
			outputData = map[string]interface{}{
				"gateway_data": inputData,
			}
		}
	} else {
		// 直连设备（deviceType == "1" 或其他）：不需要嵌套，直接返回原始数据
		outputData = inputData
	}

	// 重新构建payload
	output, err := json.Marshal(outputData)
	if err != nil {
		return fmt.Errorf("生成输出JSON失败: %v", err)
	}
	param.Value = string(output)

	return nil
}

// buildNestedSubGatewayDataForAttribute 递归构建多层子网关的嵌套属性数据结构
func buildNestedSubGatewayDataForAttribute(gateway *model.Device, subDeviceAddr string, inputData map[string]interface{}) map[string]interface{} {
	if gateway.ParentID == nil {
		// 到达顶层网关，构建最内层结构
		return map[string]interface{}{
			"sub_device_data": map[string]interface{}{
				subDeviceAddr: inputData,
			},
		}
	}

	// 递归查找父网关并构建嵌套结构
	parentGateway, err := initialize.GetDeviceCacheById(*gateway.ParentID)
	if err != nil {
		// 如果出错，返回当前层级的结构
		return map[string]interface{}{
			"sub_gateway_data": map[string]interface{}{
				*gateway.SubDeviceAddr: map[string]interface{}{
					"sub_device_data": map[string]interface{}{
						subDeviceAddr: inputData,
					},
				},
			},
		}
	}

	// 构建当前层级的嵌套结构
	innerData := buildNestedSubGatewayDataForAttribute(parentGateway, subDeviceAddr, inputData)

	// 如果父网关也是子网关，继续嵌套
	if parentGateway.ParentID != nil {
		return map[string]interface{}{
			"sub_gateway_data": map[string]interface{}{
				*gateway.SubDeviceAddr: innerData,
			},
		}
	} else {
		// 父网关是顶层网关
		return map[string]interface{}{
			"sub_gateway_data": map[string]interface{}{
				*gateway.SubDeviceAddr: map[string]interface{}{
					"sub_device_data": map[string]interface{}{
						subDeviceAddr: inputData,
					},
				},
			},
		}
	}
}
