// 文件用途：实现单设备、在线限定、可审计的即时命令请求/响应等待模块。
// 核心逻辑：复用现有命令下发与 command_set_logs message ID，短轮询设备终态并区分成功、失败、下发失败和超时。
// 关键注意事项：平台 publish 成功不是设备执行成功；HTTP 取消只停止等待，不能撤回已发布命令或删除持久日志。
// 维护建议：真实设备协议仍由 downlink/uplink adapter 负责，本文件不要增加第二套 topic、内存回执总线或 Fleet Jobs 语义。
package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const (
	directMethodDefaultTimeoutSeconds = 10
	directMethodMinTimeoutSeconds     = 1
	directMethodMaxTimeoutSeconds     = 30
	directMethodPollInterval          = 200 * time.Millisecond
)

const (
	directMethodOutcomeAwaitingResponse = "awaiting_response"
	directMethodOutcomeDeviceSucceeded  = "device_succeeded"
	directMethodOutcomeDeviceFailed     = "device_failed"
	directMethodOutcomeDeliveryFailed   = "delivery_failed"
	directMethodOutcomeTimeout          = "timeout"
)

// DirectMethodResult separates platform publish acceptance from the correlated
// device outcome. The same message ID remains the durable audit key in
// command_set_logs regardless of success, failure, timeout, or client cancel.
type DirectMethodResult struct {
	MessageID       string `json:"message_id"`
	DeviceID        string `json:"device_id"`
	Identify        string `json:"identify"`
	Status          string `json:"status"`
	Outcome         string `json:"outcome"`
	Published       bool   `json:"published"`
	LogRecorded     bool   `json:"log_recorded"`
	DeviceResponded bool   `json:"device_responded"`
	DeviceSucceeded bool   `json:"device_succeeded"`
	TimedOut        bool   `json:"timed_out"`
	ResponsePayload string `json:"response_payload,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	ElapsedMS       int64  `json:"elapsed_ms"`
}

type directMethodLogLookup func(context.Context, string, string) (*model.CommandSetLog, error)

// InvokeDirectMethod publishes one command only after the device is confirmed
// online and its audit log is durable, then waits for status 3/4 on that same
// log. It does not create a second message channel or reinterpret publish
// acceptance as device success.
func (c *CommandData) InvokeDirectMethod(
	ctx context.Context,
	operatorID string,
	req *model.DirectMethodCommandReq,
	claimsOpt ...*utils.UserClaims,
) (*DirectMethodResult, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "direct method request is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	timeoutSeconds, err := normalizeDirectMethodTimeout(req.TimeoutSeconds)
	if err != nil {
		return nil, err
	}

	args := []interface{}{requireAuditableOnlineCommandDelivery()}
	args = append(args, commandDeliveryClaimArgs(claimsOpt...)...)
	tracking, err := c.CommandPutMessageWithTracking(
		ctx,
		operatorID,
		&model.PutMessageForCommand{
			DeviceID: req.DeviceID,
			Value:    req.Value,
			Identify: req.Identify,
		},
		strconv.Itoa(constant.Manual),
		args...,
	)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	result, err := waitForDirectMethodResult(
		ctx,
		&DirectMethodResult{
			MessageID:      tracking.MessageID,
			DeviceID:       tracking.DeviceID,
			Identify:       tracking.Identify,
			Status:         tracking.Status,
			Outcome:        directMethodOutcomeAwaitingResponse,
			Published:      true,
			LogRecorded:    tracking.LogRecorded,
			TimeoutSeconds: timeoutSeconds,
		},
		time.Duration(timeoutSeconds)*time.Second,
		directMethodPollInterval,
		dal.GetCommandSetLogByMessageIDWithContext,
	)
	if result != nil {
		result.ElapsedMS = time.Since(startedAt).Milliseconds()
	}
	return result, err
}

func normalizeDirectMethodTimeout(timeoutSeconds int) (int, error) {
	if timeoutSeconds == 0 {
		return directMethodDefaultTimeoutSeconds, nil
	}
	if timeoutSeconds < directMethodMinTimeoutSeconds || timeoutSeconds > directMethodMaxTimeoutSeconds {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, "timeout_seconds must be between 1 and 30")
	}
	return timeoutSeconds, nil
}

func waitForDirectMethodResult(
	ctx context.Context,
	base *DirectMethodResult,
	timeout time.Duration,
	pollInterval time.Duration,
	lookup directMethodLogLookup,
) (*DirectMethodResult, error) {
	if base == nil || strings.TrimSpace(base.MessageID) == "" || strings.TrimSpace(base.DeviceID) == "" {
		return nil, fmt.Errorf("direct method tracking identity is incomplete")
	}
	if lookup == nil {
		return nil, fmt.Errorf("direct method command log lookup is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("direct method wait timeout must be positive")
	}
	if pollInterval <= 0 {
		pollInterval = directMethodPollInterval
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	latest := *base
	for {
		log, err := lookup(waitCtx, base.MessageID, base.DeviceID)
		if err != nil {
			if waitCtx.Err() != nil {
				return finishDirectMethodWait(ctx, &latest)
			}
			return nil, fmt.Errorf("read direct method command log: %w", err)
		}

		current, terminal := directMethodResultFromLog(base, log)
		latest = *current
		if terminal {
			return current, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-waitCtx.Done():
			return finishDirectMethodWait(ctx, &latest)
		case <-ticker.C:
		}
	}
}

func finishDirectMethodWait(parent context.Context, latest *DirectMethodResult) (*DirectMethodResult, error) {
	if err := parent.Err(); err != nil {
		return nil, err
	}
	result := *latest
	result.Outcome = directMethodOutcomeTimeout
	result.TimedOut = true
	result.DeviceResponded = false
	result.DeviceSucceeded = false
	return &result, nil
}

func directMethodResultFromLog(base *DirectMethodResult, log *model.CommandSetLog) (*DirectMethodResult, bool) {
	result := *base
	if log == nil {
		return &result, false
	}
	result.Status = strings.TrimSpace(stringValue(log.Status))
	result.ResponsePayload = stringValue(log.RspDatum)
	result.ErrorMessage = stringValue(log.ErrorMessage)

	switch result.Status {
	case strconv.Itoa(constant.ResponseStatusOk):
		result.Outcome = directMethodOutcomeDeviceSucceeded
		result.DeviceResponded = true
		result.DeviceSucceeded = true
		return &result, true
	case strconv.Itoa(constant.ResponseSStatusFailed):
		result.Outcome = directMethodOutcomeDeviceFailed
		result.DeviceResponded = true
		result.DeviceSucceeded = false
		return &result, true
	case strconv.Itoa(constant.StatusFailed):
		result.Outcome = directMethodOutcomeDeliveryFailed
		result.DeviceResponded = false
		result.DeviceSucceeded = false
		return &result, true
	default:
		result.Outcome = directMethodOutcomeAwaitingResponse
		return &result, false
	}
}
