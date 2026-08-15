// 文件用途：维护遥测写入和设备侧数据设置服务。
// 核心逻辑：校验设备写权限，规范化遥测键值，并把写入或设置请求交给下游存储/总线。
// 关键注意事项：遥测写入可能污染设备状态，坏 key、无权限和下游失败必须避免部分成功误报。
// 重构建议：抽出写入仓储和下行接口，补齐事务、权限、坏 payload 和外部失败测试。
// telemetry_set.go owns telemetry set/metadata service behavior.
//
// It validates telemetry definitions and coordinates telemetry configuration
// used by devices, templates, charts, and automation triggers.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/downlink"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func (*TelemetryData) GetTelemetrSetLogsDataListByPage(req *model.GetTelemetrySetLogsListByPageReq, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(req.DeviceId, claims); err != nil {
		return nil, err
	}

	count, data, err := dal.GetTelemetrySetLogsListByPage(req)
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

// TelemetryPutMessage 处理遥测数据下发
// 参数:
//
//	ctx: 上下文
//	userID: 用户ID，用于记录操作日志
//	param: 下发的消息内容
//	operationType: 操作类型
//
// 返回:
//
//	error: 处理过程中的错误
type telemetryDownlinkContext struct {
	device             *model.Device
	protocolType       string
	deviceType         string
	topicPrefix        string
	targetDeviceNumber string
}

func (t *TelemetryData) TelemetryPutMessage(ctx context.Context, userID string, param *model.PutMessage, operationType string) error {
	if err := validateTelemetryPutValue(param); err != nil {
		return err
	}

	downlinkContext, err := resolveTelemetryDownlinkContext(ctx, param.DeviceID)
	if err != nil {
		return err
	}
	if err := rewriteTelemetryPayloadForGateway(param, downlinkContext); err != nil {
		return err
	}

	logInfo, err := createTelemetrySetLog(ctx, userID, param, operationType)
	if err != nil {
		return err
	}
	if t.downlinkBus == nil {
		return markTelemetryDownlinkFailed(ctx, logInfo, "downlink bus not initialized")
	}

	t.downlinkBus.PublishTelemetry(buildTelemetryDownlinkMessage(param, logInfo.ID, downlinkContext))
	return nil
}

func validateTelemetryPutValue(param *model.PutMessage) error {
	if json.Valid([]byte(param.Value)) {
		return nil
	}
	return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
		"error": "value must be json",
	})
}

func resolveTelemetryDownlinkContext(ctx context.Context, deviceID string) (*telemetryDownlinkContext, error) {
	deviceInfo, err := initialize.GetDeviceCacheById(deviceID)
	if err != nil {
		logrus.Error(ctx, "[TelemetryPutMessage][GetDeviceCacheById]failed:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	protocolType, deviceType, err := resolveTelemetryDeviceProtocol(ctx, deviceInfo)
	if err != nil {
		return nil, err
	}
	topicPrefix, err := resolveTelemetryTopicPrefix(ctx, deviceInfo, protocolType)
	if err != nil {
		return nil, err
	}
	targetDeviceNumber, err := resolveTelemetryTargetDeviceNumber(ctx, deviceInfo, protocolType, deviceType)
	if err != nil {
		return nil, err
	}

	logrus.Info("target device number:", targetDeviceNumber)
	logrus.Info("device type:", deviceType)
	logrus.Info("topic prefix:", topicPrefix)

	return &telemetryDownlinkContext{
		device:             deviceInfo,
		protocolType:       protocolType,
		deviceType:         deviceType,
		topicPrefix:        topicPrefix,
		targetDeviceNumber: targetDeviceNumber,
	}, nil
}

func resolveTelemetryDeviceProtocol(ctx context.Context, deviceInfo *model.Device) (string, string, error) {
	if deviceInfo.DeviceConfigID == nil {
		return "MQTT", "1", nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*deviceInfo.DeviceConfigID)
	if err != nil {
		logrus.Error(ctx, "[TelemetryPutMessage][GetDeviceConfigByID]failed:", err)
		return "", "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	if deviceConfig.ProtocolType == nil {
		return "", "", errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "protocolType is nil",
		})
	}

	return *deviceConfig.ProtocolType, deviceConfig.DeviceType, nil
}

func resolveTelemetryTopicPrefix(ctx context.Context, deviceInfo *model.Device, protocolType string) (string, error) {
	if protocolType == "MQTT" {
		return "", nil
	}

	subTopicPrefix, err := dal.GetServicePluginSubTopicPrefixByDeviceConfigID(*deviceInfo.DeviceConfigID)
	if err != nil {
		logrus.Error(ctx, "failed to get sub topic prefix", err)
		return "", errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return subTopicPrefix, nil
}

func resolveTelemetryTargetDeviceNumber(ctx context.Context, deviceInfo *model.Device, protocolType string, deviceType string) (string, error) {
	if protocolType != "MQTT" || (deviceType != "2" && deviceType != "3") {
		return deviceInfo.DeviceNumber, nil
	}

	topGateway, err := findTopLevelGateway(deviceInfo, deviceType)
	if err != nil {
		logrus.Error(ctx, "failed to find top level gateway", err)
		return "", errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return topGateway.DeviceNumber, nil
}

func rewriteTelemetryPayloadForGateway(param *model.PutMessage, downlinkContext *telemetryDownlinkContext) error {
	if downlinkContext.protocolType != "MQTT" || (downlinkContext.deviceType != "3" && downlinkContext.deviceType != "2") {
		return nil
	}

	var inputData map[string]interface{}
	if err := json.Unmarshal([]byte(param.Value), &inputData); err != nil {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	outputData, err := buildTelemetryGatewayPayload(downlinkContext.device, downlinkContext.deviceType, inputData)
	if err != nil {
		return err
	}
	output, err := json.Marshal(outputData)
	if err != nil {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	param.Value = string(output)
	return nil
}

func buildTelemetryGatewayPayload(deviceInfo *model.Device, deviceType string, inputData map[string]interface{}) (map[string]interface{}, error) {
	switch deviceType {
	case "3":
		return buildSubDeviceTelemetryPayload(deviceInfo, inputData)
	case "2":
		return buildGatewayTelemetryPayload(deviceInfo, inputData)
	default:
		return inputData, nil
	}
}

func buildSubDeviceTelemetryPayload(deviceInfo *model.Device, inputData map[string]interface{}) (map[string]interface{}, error) {
	if deviceInfo.SubDeviceAddr == nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "subDeviceAddr is nil",
		})
	}
	parentGateway, err := initialize.GetDeviceCacheById(*deviceInfo.ParentID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	if parentGateway.ParentID != nil {
		if parentGateway.SubDeviceAddr == nil {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"error": "parent gateway subDeviceAddr is nil",
			})
		}
		return buildNestedSubGatewayData(parentGateway, *deviceInfo.SubDeviceAddr, inputData), nil
	}
	return map[string]interface{}{
		"sub_device_data": map[string]interface{}{
			*deviceInfo.SubDeviceAddr: inputData,
		},
	}, nil
}

func buildGatewayTelemetryPayload(deviceInfo *model.Device, inputData map[string]interface{}) (map[string]interface{}, error) {
	if deviceInfo.ParentID == nil {
		return map[string]interface{}{
			"gateway_data": inputData,
		}, nil
	}
	if deviceInfo.SubDeviceAddr == nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "sub gateway subDeviceAddr is nil",
		})
	}
	return map[string]interface{}{
		"sub_gateway_data": map[string]interface{}{
			*deviceInfo.SubDeviceAddr: map[string]interface{}{
				"gateway_data": inputData,
			},
		},
	}, nil
}

func createTelemetrySetLog(ctx context.Context, userID string, param *model.PutMessage, operationType string) (*model.TelemetrySetLog, error) {
	description := "下发遥测日志记录"
	logInfo := &model.TelemetrySetLog{
		ID:            uuid.New(),
		DeviceID:      param.DeviceID,
		OperationType: &operationType,
		Datum:         &(param.Value),
		Status:        nil,
		ErrorMessage:  nil,
		CreatedAt:     time.Now().UTC(),
		Description:   &description,
		UserID:        &userID,
	}
	if userID == "" {
		logInfo.UserID = nil
	}

	if _, err := (dal.TelemetrySetLogsQuery{}).Create(ctx, logInfo); err != nil {
		logrus.Error(ctx, "failed to create telemetry set log", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}
	return logInfo, nil
}

func markTelemetryDownlinkFailed(ctx context.Context, logInfo *model.TelemetrySetLog, errorMessage string) error {
	logrus.Error(ctx, "下发失败: ", errorMessage)
	status := strconv.Itoa(constant.StatusFailed)
	logInfo.Status = &status
	logInfo.ErrorMessage = &errorMessage
	if updateErr := dal.UpdateTelemetrySetLog(logInfo); updateErr != nil {
		logrus.Error(ctx, "failed to update telemetry set log", updateErr)
	}
	return errors.New(errorMessage)
}

func buildTelemetryDownlinkMessage(param *model.PutMessage, logID string, downlinkContext *telemetryDownlinkContext) *downlink.Message {
	return &downlink.Message{
		DeviceID:       downlinkContext.device.ID,
		DeviceNumber:   downlinkContext.targetDeviceNumber,
		DeviceType:     downlinkContext.deviceType,
		DeviceConfigID: getDeviceConfigID(downlinkContext.device),
		Type:           downlink.MessageTypeTelemetry,
		Data:           []byte(param.Value),
		TopicPrefix:    downlinkContext.topicPrefix,
		MessageID:      logID,
	}
}

// findTopLevelGateway 递归查找顶层网关（parent_id为空的网关）
func findTopLevelGateway(deviceInfo *model.Device, deviceType string) (*model.Device, error) {
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

// buildNestedSubGatewayData 递归构建多层子网关的嵌套数据结构
func buildNestedSubGatewayData(gateway *model.Device, subDeviceAddr string, inputData map[string]interface{}) map[string]interface{} {
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
	innerData := buildNestedSubGatewayData(parentGateway, subDeviceAddr, inputData)

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

// getDeviceConfigID 获取设备配置ID（辅助函数）
func getDeviceConfigID(device *model.Device) string {
	if device.DeviceConfigID == nil {
		return ""
	}
	return *device.DeviceConfigID
}
