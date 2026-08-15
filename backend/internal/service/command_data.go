// 文件用途：维护设备命令数据建模、校验和下发服务。
// 核心逻辑：读取命令模型和设备配置，校验命令参数后组装下行 payload 交给设备通道。
// 关键注意事项：命令下发会影响真实设备，参数错误、无权限和设备离线时必须避免外部副作用。
// 重构建议：拆分命令 schema 校验与下行发送接口，补齐事务、权限、超时和协议差异测试。
// command_data.go owns device command service behavior.
//
// It builds command payloads, validates command metadata, and coordinates
// command delivery for device detail pages, MQTT downlink, and RDI flows.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/downlink"
	"aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type CommandData struct {
	downlinkBus *downlink.Bus // ✨ 依赖注入
}

type CommandDeliveryTracking struct {
	MessageID     string `json:"message_id"`
	Status        string `json:"status"`
	DeviceID      string `json:"device_id"`
	Identify      string `json:"identify"`
	OperationType string `json:"operation_type"`
	LogRecorded   bool   `json:"log_recorded"`
}

type CommandDeliveryOption func(*commandDeliveryOptions)

type commandDeliveryOptions struct {
	messageID          string
	requireOnline      bool
	requireLogRecorded bool
}

func WithCommandDeliveryMessageID(messageID string) CommandDeliveryOption {
	return func(options *commandDeliveryOptions) {
		options.messageID = strings.TrimSpace(messageID)
	}
}

func requireAuditableOnlineCommandDelivery() CommandDeliveryOption {
	return func(options *commandDeliveryOptions) {
		options.requireOnline = true
		options.requireLogRecorded = true
	}
}

type commandDeviceProfile struct {
	device       *model.Device
	deviceType   string
	protocolType string
}

type commandDispatchPlan struct {
	sourceDevice       *model.Device
	targetDevice       *model.Device
	targetDeviceNumber string
	deviceType         string
	topicPrefix        string
	payload            []byte
	payloadText        string
}

type preparedCommandDelivery struct {
	profile       *commandDeviceProfile
	plan          *commandDispatchPlan
	messageID     string
	identify      string
	operationType string
	logRecorded   bool
}

// SetDownlinkBus 设置 downlink Bus（在 Application 初始化时调用）
func (c *CommandData) SetDownlinkBus(bus *downlink.Bus) {
	c.downlinkBus = bus
}

// PutMessage 下发命令（改造为异步模式，支持多层网关）
// 保持原有的 CommandPutMessage 接口签名
func (c *CommandData) CommandPutMessage(ctx context.Context, operatorID string, putMessageReq *model.PutMessageForCommand, operationType string, claimsOpt ...*utils.UserClaims) error {
	_, err := c.CommandPutMessageWithTracking(ctx, operatorID, putMessageReq, operationType, commandDeliveryClaimArgs(claimsOpt...)...)
	return err
}

func (c *CommandData) CommandPutMessageWithTracking(ctx context.Context, operatorID string, putMessageReq *model.PutMessageForCommand, operationType string, args ...interface{}) (*CommandDeliveryTracking, error) {
	options, claimsOpt := commandDeliveryArgs(args...)
	delivery, err := c.prepareCommandDelivery(operatorID, putMessageReq, operationType, options, claimsOpt)
	if err != nil {
		return nil, err
	}

	if err := c.publishCommand(delivery.plan, delivery.messageID, delivery.identify); err != nil {
		c.recordCommandDeliveryPublishFailure(delivery, err)
		return nil, err
	}

	return commandDeliveryTrackingFromPrepared(delivery), nil
}

func (c *CommandData) prepareCommandDelivery(
	operatorID string,
	putMessageReq *model.PutMessageForCommand,
	operationType string,
	options commandDeliveryOptions,
	claimsOpt []*utils.UserClaims,
) (*preparedCommandDelivery, error) {
	if putMessageReq == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "command request is required")
	}
	if err := ensureCommandWriteAccess(putMessageReq.DeviceID, claimsOpt...); err != nil {
		return nil, err
	}

	profile, err := loadCommandDeviceProfile(putMessageReq.DeviceID)
	if err != nil {
		return nil, err
	}
	if options.requireOnline && profile.device.IsOnline != 1 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "direct method requires an online device")
	}

	plan, err := c.buildCommandDispatchPlan(putMessageReq, profile)
	if err != nil {
		return nil, err
	}

	messageID := options.messageID
	if messageID == "" {
		messageID = uuid.New()[:8]
	}
	logRecorded := true
	if err := c.createCommandLogForPut(profile.device, operatorID, messageID, putMessageReq.Identify, &plan.payloadText, operationType); err != nil {
		logRecorded = false
		logrus.WithError(err).Error("Failed to create command log")
		if options.requireLogRecorded {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"reason": "direct method command log must be recorded before publish",
			})
		}
		// 普通异步下发保持兼容：日志失败不会阻断原有发送流程。
	}

	return &preparedCommandDelivery{
		profile:       profile,
		plan:          plan,
		messageID:     messageID,
		identify:      putMessageReq.Identify,
		operationType: operationType,
		logRecorded:   logRecorded,
	}, nil
}

func (c *CommandData) recordCommandDeliveryPublishFailure(delivery *preparedCommandDelivery, err error) {
	if delivery == nil || delivery.profile == nil || delivery.profile.device == nil || !delivery.logRecorded {
		return
	}
	c.markCommandPublishFailed(delivery.profile.device.ID, delivery.messageID, err)
}

func commandDeliveryTrackingFromPrepared(delivery *preparedCommandDelivery) *CommandDeliveryTracking {
	return &CommandDeliveryTracking{
		MessageID:     delivery.messageID,
		Status:        "0",
		DeviceID:      delivery.profile.device.ID,
		Identify:      delivery.identify,
		OperationType: delivery.operationType,
		LogRecorded:   delivery.logRecorded,
	}
}

func commandDeliveryArgs(args ...interface{}) (commandDeliveryOptions, []*utils.UserClaims) {
	options := commandDeliveryOptions{}
	claims := make([]*utils.UserClaims, 0, len(args))
	for _, arg := range args {
		switch value := arg.(type) {
		case CommandDeliveryOption:
			if value != nil {
				value(&options)
			}
		case *utils.UserClaims:
			claims = append(claims, value)
		}
	}
	return options, claims
}

func commandDeliveryClaimArgs(claims ...*utils.UserClaims) []interface{} {
	args := make([]interface{}, 0, len(claims))
	for _, claim := range claims {
		args = append(args, claim)
	}
	return args
}

func ensureCommandWriteAccess(deviceID string, claimsOpt ...*utils.UserClaims) error {
	if len(claimsOpt) == 0 {
		return nil
	}
	_, err := ensureTelemetryDeviceWriteAccess(deviceID, claimsOpt[0])
	return err
}

func loadCommandDeviceProfile(deviceID string) (*commandDeviceProfile, error) {
	device, err := initialize.GetDeviceCacheById(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	profile := &commandDeviceProfile{
		device:       device,
		deviceType:   "1",
		protocolType: "MQTT",
	}
	if device.DeviceConfigID == nil {
		return profile, nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(*device.DeviceConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device config: %w", err)
	}
	profile.deviceType = deviceConfig.DeviceType
	if deviceConfig.ProtocolType != nil {
		profile.protocolType = *deviceConfig.ProtocolType
	}
	return profile, nil
}

func (c *CommandData) buildCommandDispatchPlan(req *model.PutMessageForCommand, profile *commandDeviceProfile) (*commandDispatchPlan, error) {
	if profile == nil || profile.device == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "command device profile is required")
	}
	jsonData, transformedDataStr, err := buildCommandPayload(req, profile.device, profile.deviceType)
	if err != nil {
		return nil, err
	}
	targetDevice, targetDeviceNumber, topicPrefix, err := c.resolveDeviceInfo(profile.device, profile.deviceType, profile.protocolType)
	if err != nil {
		return nil, err
	}

	return &commandDispatchPlan{
		sourceDevice:       profile.device,
		targetDevice:       targetDevice,
		targetDeviceNumber: targetDeviceNumber,
		deviceType:         profile.deviceType,
		topicPrefix:        topicPrefix,
		payload:            jsonData,
		payloadText:        transformedDataStr,
	}, nil
}

func buildCommandPayload(req *model.PutMessageForCommand, device *model.Device, deviceType string) ([]byte, string, error) {
	commandData, err := buildRawCommandData(req)
	if err != nil {
		return nil, "", err
	}
	transformedData, err := transformCommandDataForMultiLevelGateway(commandData, device, deviceType)
	if err != nil {
		return nil, "", fmt.Errorf("failed to transform command data: %w", err)
	}
	jsonData, err := json.Marshal(transformedData)
	if err != nil {
		return nil, "", errcode.NewWithMessage(errcode.CodeParamError, "command data is not a valid JSON")
	}
	return jsonData, string(jsonData), nil
}

func buildRawCommandData(req *model.PutMessageForCommand) (map[string]interface{}, error) {
	commandData := map[string]interface{}{
		"method": req.Identify,
	}
	if req.Value == nil {
		return commandData, nil
	}
	valueStr := strings.TrimSpace(*req.Value)
	if valueStr == "" {
		return commandData, nil
	}
	if !json.Valid([]byte(valueStr)) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "value is not a valid JSON")
	}
	commandData["params"] = json.RawMessage(valueStr)
	return commandData, nil
}

func (c *CommandData) publishCommand(plan *commandDispatchPlan, messageID, identify string) error {
	if c.downlinkBus == nil {
		return fmt.Errorf("downlink service not available")
	}
	if plan == nil || plan.sourceDevice == nil || plan.targetDevice == nil {
		return fmt.Errorf("command dispatch plan is incomplete")
	}
	msg := &downlink.Message{
		DeviceID:       plan.sourceDevice.ID,
		DeviceNumber:   plan.targetDeviceNumber,
		DeviceType:     plan.deviceType,
		DeviceConfigID: c.getDeviceConfigID(plan.targetDevice),
		Type:           downlink.MessageTypeCommand,
		Data:           plan.payload,
		TopicPrefix:    plan.topicPrefix,
		MessageID:      messageID,
	}
	c.downlinkBus.PublishCommand(msg)

	logrus.WithFields(logrus.Fields{
		"device_id":            plan.sourceDevice.ID,
		"target_device_id":     plan.targetDevice.ID,
		"target_device_number": plan.targetDeviceNumber,
		"device_type":          plan.deviceType,
		"message_id":           messageID,
		"identify":             identify,
	}).Info("Command sent via downlink")
	return nil
}

func (c *CommandData) markCommandPublishFailed(deviceID, messageID string, publishErr error) {
	if deviceID == "" || messageID == "" || publishErr == nil {
		return
	}

	log, err := dal.GetCommandSetLogByMessageID(messageID, deviceID)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"device_id":  deviceID,
			"message_id": messageID,
		}).Warn("Failed to find command log after publish failure")
		return
	}

	status := strconv.Itoa(constant.StatusFailed)
	errorMessage := fmt.Sprintf("publish failed: %v", publishErr)
	log.Status = &status
	log.ErrorMessage = &errorMessage

	if err := dal.UpdateCommandSetLog(log); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"device_id":  deviceID,
			"message_id": messageID,
		}).Warn("Failed to mark command log as publish failed")
	}
}

// createCommandLogForPut 创建命令日志（for PutMessageForCommand）
func (c *CommandData) createCommandLogForPut(device *model.Device, operatorID, messageId, identify string, value *string, operationType string) error {
	return dal.CreateCommandSetLog(newCommandSetLog(device, operatorID, messageId, identify, value, operationType))
}

func newCommandSetLog(device *model.Device, operatorID, messageId, identify string, value *string, operationType string) *model.CommandSetLog {
	status := "0" // pending
	var userID *string
	if normalizedOperatorID := strings.TrimSpace(operatorID); normalizedOperatorID != "" {
		userID = &normalizedOperatorID
	}
	log := &model.CommandSetLog{
		ID:            uuid.New(),
		DeviceID:      device.ID,
		OperationType: &operationType,
		MessageID:     &messageId,
		Identify:      &identify,
		Datum:         value, // 直接使用 *string
		Status:        &status,
		ErrorMessage:  nil,
		CreatedAt:     time.Now(),
		UserID:        userID,
	}
	return log
}

// resolveDeviceInfo 处理多层网关，返回目标设备、目标设备编号和Topic前缀
func (c *CommandData) resolveDeviceInfo(device *model.Device, deviceType, protocolType string) (*model.Device, string, string, error) {
	var targetDevice *model.Device
	var targetDeviceNumber string
	var topicPrefix string

	// 根据协议类型和设备类型确定目标设备
	// MQTT协议：网关/子设备需要查找顶层网关
	// 非MQTT协议（协议插件）：直接使用设备自己，插件会处理层级关系
	if protocolType == "MQTT" && (device.ParentID != nil && *device.ParentID != "") {
		// MQTT 网关/子设备：查找顶层网关
		topGateway, err := findTopLevelGatewayForCommand(device, deviceType)
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

// getDeviceConfigID 获取设备配置ID
func (c *CommandData) getDeviceConfigID(device *model.Device) string {
	if device.DeviceConfigID == nil {
		return ""
	}
	return *device.DeviceConfigID
}

// GetCommonList 获取命令列表（保留原有方法）
func (*CommandData) GetCommonList(ctx context.Context, id string, claims *utils.UserClaims) ([]model.GetCommandListRes, error) {
	list := make([]model.GetCommandListRes, 0)
	if _, err := ensureTelemetryDeviceReadAccess(id, claims); err != nil {
		return list, err
	}

	deviceInfo, err := dal.DeviceQuery{}.First(ctx, query.Device.ID.Eq(id))
	if err != nil {
		logrus.Error(ctx, "[GetCommonList]device failed:", err)
		return list, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	if deviceInfo.DeviceConfigID == nil || common.CheckEmpty(*deviceInfo.DeviceConfigID) {
		logrus.Debug("device.device_config_id is empty")
		return list, nil
	}

	deviceConfigsInfo, err := dal.DeviceConfigQuery{}.First(ctx, query.DeviceConfig.ID.Eq(*deviceInfo.DeviceConfigID))
	if err != nil {
		logrus.Debug(ctx, "[GetCommonList]device_configs failed:", err)
		return list, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	if deviceConfigsInfo.DeviceTemplateID == nil || common.CheckEmpty(*deviceConfigsInfo.DeviceTemplateID) {
		logrus.Debug("device_configs.device_template_id is empty")
		return list, nil
	}

	commandList, err := dal.DeviceModelCommandsQuery{}.Find(ctx, query.DeviceModelCommand.DeviceTemplateID.Eq(*deviceConfigsInfo.DeviceTemplateID))
	if err != nil {
		logrus.Error(ctx, "[GetCommonList]device_model_command failed:", err)
		return list, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	for _, info := range commandList {
		commandRes := model.GetCommandListRes{
			Identifier: info.DataIdentifier,
		}
		if info.DataName != nil {
			commandRes.Name = *info.DataName
		}
		if info.Param != nil {
			commandRes.Params = *info.Param
		}
		if info.Description != nil {
			commandRes.Description = *info.Description
		}
		list = append(list, commandRes)
	}

	return list, err
}

// GetCommandSetLogsDataListByPage 获取命令下发日志（分页）
func (c *CommandData) GetCommandSetLogsDataListByPage(req model.GetCommandSetLogsListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(req.DeviceId, claims); err != nil {
		return nil, err
	}

	// 查询日志列表
	logs, total, err := dal.GetCommandSetLogsByPage(&req)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return map[string]interface{}{
		"list":  logs,
		"total": total,
	}, nil
}
