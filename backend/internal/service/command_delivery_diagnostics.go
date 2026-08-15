package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const defaultCommandDeliveryDiagnosticsLogLimit = 5

type CommandDeliveryDiagnosticsReq struct {
	DeviceID string `json:"device_id"`
	Limit    int    `json:"limit,omitempty"`
}

type CommandDeliveryDiagnosticsResp struct {
	DeviceID             string                               `json:"device_id"`
	EvaluatedAt          time.Time                            `json:"evaluated_at"`
	IsOnline             bool                                 `json:"is_online"`
	DeviceStatus         int                                  `json:"device_status"`
	LatestLog            *CommandDeliveryLogSummary           `json:"latest_log,omitempty"`
	RecentLogs           []CommandDeliveryLogSummary          `json:"recent_logs"`
	ConfirmationChannels []CommandDeliveryConfirmation        `json:"confirmation_channels"`
	Conclusion           CommandDeliveryDiagnosticsConclusion `json:"conclusion"`
}

type CommandDeliveryLogSummary struct {
	ID           string    `json:"id"`
	MessageID    string    `json:"message_id"`
	Identify     string    `json:"identify"`
	Status       string    `json:"status"`
	StatusLabel  string    `json:"status_label"`
	Data         string    `json:"data,omitempty"`
	ResponseData string    `json:"response_data,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type CommandDeliveryConfirmation struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type CommandDeliveryDiagnosticsConclusion struct {
	Level       string   `json:"level"`
	Code        string   `json:"code"`
	Summary     string   `json:"summary"`
	NextActions []string `json:"next_actions"`
	Evidence    []string `json:"evidence,omitempty"`
}

func (c *CommandData) GetCommandDeliveryDiagnostics(
	ctx context.Context,
	req CommandDeliveryDiagnosticsReq,
	claims *utils.UserClaims,
) (*CommandDeliveryDiagnosticsResp, error) {
	_ = ctx
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}

	deviceInfo, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	if err != nil {
		return nil, err
	}

	logs, _, err := dal.GetCommandSetLogsByPage(&model.GetCommandSetLogsListByPageReq{
		PageReq: model.PageReq{
			Page:     1,
			PageSize: normalizeCommandDeliveryDiagnosticsLogLimit(req.Limit),
		},
		DeviceId: deviceID,
	})
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	resp := &CommandDeliveryDiagnosticsResp{
		DeviceID:             deviceID,
		EvaluatedAt:          time.Now().UTC(),
		IsOnline:             deviceInfo.IsOnline == 1,
		DeviceStatus:         int(deviceInfo.IsOnline),
		RecentLogs:           summarizeCommandDeliveryLogs(logs),
		ConfirmationChannels: commandDeliveryConfirmationChannels(),
	}
	if len(resp.RecentLogs) > 0 {
		resp.LatestLog = &resp.RecentLogs[0]
	}
	resp.Conclusion = buildCommandDeliveryDiagnosticsConclusion(resp)
	return resp, nil
}

func normalizeCommandDeliveryDiagnosticsLogLimit(limit int) int {
	if limit <= 0 {
		return defaultCommandDeliveryDiagnosticsLogLimit
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func summarizeCommandDeliveryLogs(logs []*model.CommandSetLog) []CommandDeliveryLogSummary {
	summaries := make([]CommandDeliveryLogSummary, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		status := stringValue(log.Status)
		summaries = append(summaries, CommandDeliveryLogSummary{
			ID:           log.ID,
			MessageID:    stringValue(log.MessageID),
			Identify:     stringValue(log.Identify),
			Status:       status,
			StatusLabel:  commandDeliveryStatusLabel(status),
			Data:         stringValue(log.Datum),
			ResponseData: stringValue(log.RspDatum),
			ErrorMessage: stringValue(log.ErrorMessage),
			CreatedAt:    log.CreatedAt,
		})
	}
	return summaries
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func commandDeliveryStatusLabel(status string) string {
	switch status {
	case "0":
		return "pending"
	case strconv.Itoa(constant.StatusOK):
		return "sent"
	case strconv.Itoa(constant.StatusFailed):
		return "send_failed"
	case strconv.Itoa(constant.ResponseStatusOk):
		return "device_ack_success"
	case strconv.Itoa(constant.ResponseSStatusFailed):
		return "device_ack_failed"
	default:
		return "unknown"
	}
}

func commandDeliveryConfirmationChannels() []CommandDeliveryConfirmation {
	return []CommandDeliveryConfirmation{
		{
			Code:        "command_log",
			Label:       "命令日志",
			Description: "确认平台是否记录、发布、失败，或是否收到该命令的设备响应。",
		},
		{
			Code:        "device_response",
			Label:       "设备响应",
			Description: "确认设备是否针对该 message ID 返回成功或失败。",
		},
		{
			Code:        "device_twin",
			Label:       "设备影子",
			Description: "确认命令改变设备状态后，期望值和上报值是否收敛。",
		},
	}
}

func buildCommandDeliveryDiagnosticsConclusion(
	resp *CommandDeliveryDiagnosticsResp,
) CommandDeliveryDiagnosticsConclusion {
	if resp == nil || resp.LatestLog == nil {
		return CommandDeliveryDiagnosticsConclusion{
			Level:       "info",
			Code:        "no_command_log",
			Summary:     "这台设备还没有记录命令下发日志。",
			NextActions: []string{"先发送一条命令，再刷新本诊断视图获取 message ID 和下发状态。"},
		}
	}

	latest := resp.LatestLog
	evidence := []string{"message_id=" + emptyAsUnknown(latest.MessageID), "status=" + emptyAsUnknown(latest.Status)}
	switch latest.StatusLabel {
	case "device_ack_success":
		return CommandDeliveryDiagnosticsConclusion{
			Level:       "ok",
			Code:        "device_ack_success",
			Summary:     "最新命令已有设备成功响应。",
			NextActions: []string{"打开命令详情抽屉检查响应载荷。", "如果命令应改变设备状态，请检查设备影子或遥测页签。"},
			Evidence:    evidence,
		}
	case "device_ack_failed":
		return CommandDeliveryDiagnosticsConclusion{
			Level:       "error",
			Code:        "device_ack_failed",
			Summary:     "设备对最新命令返回了失败响应。",
			NextActions: []string{"打开命令详情抽屉，处理设备返回的错误。", "确认命令载荷和设备状态后再重试。"},
			Evidence:    append(evidence, "error="+emptyAsUnknown(latest.ErrorMessage)),
		}
	case "send_failed":
		return CommandDeliveryDiagnosticsConclusion{
			Level:       "error",
			Code:        "send_failed",
			Summary:     "平台未能把最新命令发布到 broker 或下行通道。",
			NextActions: []string{"检查 broker 连通性和下行诊断。", "重试前请确认设备主题、网关路径和凭据。"},
			Evidence:    append(evidence, "error="+emptyAsUnknown(latest.ErrorMessage)),
		}
	case "sent":
		return CommandDeliveryDiagnosticsConclusion{
			Level:       "warning",
			Code:        "awaiting_device_response",
			Summary:     "平台已接受并发布最新命令，但尚未记录设备响应。",
			NextActions: []string{"保留 message ID，并等待设备响应。", "如果命令应改变状态，请在设备影子视图对比期望值和上报值。"},
			Evidence:    evidence,
		}
	default:
		if !resp.IsOnline {
			return CommandDeliveryDiagnosticsConclusion{
				Level:       "warning",
				Code:        "offline_or_pending",
				Summary:     "最新命令仍在等待中，且设备当前离线。",
				NextActions: []string{"离线设备请使用期望消息或 Jobs 路径。", "设备重连后刷新，并检查命令日志、响应载荷和设备影子状态。"},
				Evidence:    append(evidence, "online=false"),
			}
		}
		return CommandDeliveryDiagnosticsConclusion{
			Level:       "info",
			Code:        "pending",
			Summary:     "最新命令已记录，但尚未确认设备执行结果。",
			NextActions: []string{"刷新命令日志，查看是否已有响应。", "在看到响应或状态变化前，不要把平台提交视为设备已执行。"},
			Evidence:    evidence,
		}
	}
}

func emptyAsUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
