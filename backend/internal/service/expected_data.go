// expected_data.go 负责设备期望数据的创建、分页、过期判断与主动下发，
// 支持 telemetry、attribute、command 三类待发送消息。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type ExpectedData struct{}

const (
	expectedDataStatusPending  = "pending"
	expectedDataStatusSent     = "sent"
	expectedDataStatusExpired  = "expired"
	expectedDataSendSuccess    = "send success"
	expectedDataIdentifyNeeded = "identify is required"
)

type expectedDataDispatchResult struct {
	status  string
	message *string
}

func parseExpectedDataPayloadJSON(payload string) (any, error) {
	var params any
	if err := json.Unmarshal([]byte(payload), &params); err != nil {
		return nil, fmt.Errorf("error parsing payload JSON: %v", err)
	}
	return params, nil
}

func marshalExpectedDataPayloadJSON(value any) (string, error) {
	mergedJSON, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("error marshaling merged data to JSON: %v", err)
	}
	return string(mergedJSON), nil
}

// mergeIdentifyAndPayload 会把 command 类请求重写成统一的 method/params JSON，
// 方便后续发送链路按同一协议解析。
func mergeIdentifyAndPayload(identify string, paramsStr *string) (string, error) {
	mergedData := map[string]interface{}{
		"method": identify,
	}
	if paramsStr != nil {
		params, err := parseExpectedDataPayloadJSON(*paramsStr)
		if err != nil {
			return "", err
		}
		mergedData["params"] = params
	}

	return marshalExpectedDataPayloadJSON(mergedData)
}

func (e *ExpectedData) Create(ctx context.Context, req *model.CreateExpectedDataReq, userClaims *utils.UserClaims) (*model.ExpectedData, error) {
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(req.DeviceID, userClaims)
	if err != nil {
		return nil, err
	}

	// command 类型必须显式带 identify，且会在写库前被封装成标准命令 JSON。
	if req.SendType == "command" {
		if req.Identify == nil {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"identify": "identify is required",
			})
		}
		payload, err := mergeIdentifyAndPayload(*req.Identify, req.Payload)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"payload": err.Error(),
			})
		}
		req.Payload = &payload
	} else if req.Payload == nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"payload": "payload is required",
		})
	}

	ed := &model.ExpectedData{
		ID:         uuid.New(),
		DeviceID:   req.DeviceID,
		SendType:   req.SendType,
		Payload:    *req.Payload,
		CreatedAt:  time.Now(),
		Status:     expectedDataStatusPending,
		ExpiryTime: req.Expiry,
		Label:      req.Label,
		TenantID:   deviceInfo.TenantID,
	}
	expectedDataDal := dal.ExpectedDataDal{}
	err = expectedDataDal.Create(ctx, ed)
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	expectedData, err := expectedDataDal.GetByID(ctx, ed.ID)
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	deviceStatus, err := GroupApp.Device.GetDeviceOnlineStatus(req.DeviceID, userClaims)
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	// 设备在线时立即触发一次发送，离线则保留为 pending 等待后续补发。
	if deviceStatus["is_online"] == 1 {
		err := e.Send(ctx, req.DeviceID)
		if err != nil {
			logrus.Error(err)
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
	}

	return expectedData, nil
}

func (*ExpectedData) Delete(ctx context.Context, id string, userClaims *utils.UserClaims) error {
	expectedDataDal := dal.ExpectedDataDal{}
	expectedData, err := expectedDataDal.GetByID(ctx, id)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(expectedData.DeviceID, userClaims)
	if err != nil {
		return err
	}
	if expectedData.TenantID != deviceInfo.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "expected data tenant mismatch")
	}
	if err := expectedDataDal.Delete(ctx, id); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*ExpectedData) PageList(ctx context.Context, req *model.GetExpectedDataPageReq, userClaims *utils.UserClaims) (map[string]interface{}, error) {
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(req.DeviceID, userClaims)
	if err != nil {
		return nil, err
	}
	total, list, err := dal.ExpectedDataDal{}.PageList(ctx, req, deviceInfo.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}, nil
}

func (*ExpectedData) Send(ctx context.Context, deviceID string) error {
	expectedDataDal := dal.ExpectedDataDal{}
	ed, err := expectedDataDal.GetAllByDeviceID(ctx, deviceID)
	if err != nil {
		logrus.WithError(err).Error("query expected data failed")
		return err
	}
	logrus.Debug("expected data loaded")

	for _, v := range ed {
		// 每条期望数据独立判断是否过期或可发送，避免一条失败阻塞其余记录状态推进。
		result, ok := dispatchExpectedDataItem(ctx, deviceID, v)
		if !ok {
			continue
		}
		if err := updateStatus(ctx, v.ID, result.status, result.message); err != nil {
			return err
		}
	}

	return nil
}

func dispatchExpectedDataItem(ctx context.Context, deviceID string, expectedData *model.ExpectedData) (*expectedDataDispatchResult, bool) {
	// 过期数据不再下发，但仍会把状态推进到 expired，防止持续重复尝试。
	if expectedData.ExpiryTime != nil && expectedData.ExpiryTime.Before(time.Now()) {
		logrus.WithField("dataID", expectedData.ID).Debug("expected data expired")
		return &expectedDataDispatchResult{status: expectedDataStatusExpired}, true
	}

	message, err, ok := sendExpectedDataByType(ctx, deviceID, expectedData)
	if !ok {
		return nil, false
	}

	return buildExpectedDataDispatchResult(expectedData.SendType, message, err), true
}

func sendExpectedDataByType(ctx context.Context, deviceID string, expectedData *model.ExpectedData) (string, error, bool) {
	switch expectedData.SendType {
	case "telemetry":
		message, err := sendTelemetry(ctx, deviceID, expectedData.Payload)
		return message, err, true
	case "attribute":
		message, err := sendAttribute(ctx, deviceID, expectedData.Payload)
		return message, err, true
	case "command":
		message, err := sendCommand(ctx, deviceID, expectedData.Payload)
		return message, err, true
	default:
		logrus.Error("unknown expected data send type")
		return "", nil, false
	}
}

func buildExpectedDataDispatchResult(sendType, message string, err error) *expectedDataDispatchResult {
	if err != nil {
		logrus.Error("send expected data failed")
		return &expectedDataDispatchResult{
			// 现有语义里发送失败直接记为 expired，表示该条待发送任务本轮终止，不再自动重试。
			status:  expectedDataStatusExpired,
			message: &message,
		}
	}

	return &expectedDataDispatchResult{
		status:  expectedDataStatusSent,
		message: &message,
	}
}

func sendTelemetry(ctx context.Context, deviceID, payload string) (string, error) {
	putMessage := &model.PutMessage{
		DeviceID: deviceID,
		Value:    payload,
	}
	err := GroupApp.TelemetryData.TelemetryPutMessage(ctx, "", putMessage, "2")
	if err != nil {
		return err.Error(), err
	}
	return expectedDataSendSuccess, nil
}

func sendAttribute(ctx context.Context, deviceID, payload string) (string, error) {
	putMessage := &model.AttributePutMessage{
		DeviceID: deviceID,
		Value:    payload,
	}
	err := GroupApp.AttributeData.AttributePutMessage(ctx, "", putMessage, "2")
	if err != nil {
		return err.Error(), err
	}
	return expectedDataSendSuccess, nil
}

func parseExpectedDataCommandMethod(rawMethod json.RawMessage) (string, error) {
	if len(rawMethod) == 0 {
		return "", fmt.Errorf(expectedDataIdentifyNeeded)
	}

	var method string
	if err := json.Unmarshal(rawMethod, &method); err != nil || method == "" {
		return "", fmt.Errorf(expectedDataIdentifyNeeded)
	}
	return method, nil
}

func parseExpectedDataCommandParams(rawParams json.RawMessage) *string {
	if len(rawParams) == 0 {
		return nil
	}

	params := string(rawParams)
	return &params
}

func decodeExpectedDataCommandPayload(payload string) (string, *string, string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		message := fmt.Sprintf("Error parsing JSON payload: %s", err.Error())
		return "", nil, message, err
	}

	// 这里按弱类型 map 先拆 method/params，兼容 params 为对象、数组或基础类型的情况。
	method, err := parseExpectedDataCommandMethod(raw["method"])
	if err != nil {
		return "", nil, err.Error(), err
	}

	return method, parseExpectedDataCommandParams(raw["params"]), "", nil
}

func sendCommand(ctx context.Context, deviceID, payload string) (string, error) {
	method, paramsStr, message, err := decodeExpectedDataCommandPayload(payload)
	if err != nil {
		return message, err
	}

	putMessage := &model.PutMessageForCommand{
		DeviceID: deviceID,
		Identify: method,
		Value:    paramsStr,
	}
	err = GroupApp.CommandData.CommandPutMessage(ctx, "", putMessage, "2")
	if err != nil {
		return err.Error(), err
	}

	return expectedDataSendSuccess, nil
}

func updateStatus(ctx context.Context, id string, status string, message *string) error {
	var sendTime time.Time
	if status == expectedDataStatusSent {
		sendTime = time.Now()
	}

	err := dal.ExpectedDataDal{}.UpdateStatus(ctx, id, status, message, &sendTime)
	if err != nil {
		logrus.Error("update expected data status failed")
	}
	return err
}
