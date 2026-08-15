// 文件用途：锁定即时命令的超时边界、审计前置条件、终态映射和请求取消合同。
// 验证边界：这些测试使用本地 fake lookup；真实 PostgreSQL、broker、设备响应和 HTTP 代理超时仍需集成验证。
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func directMethodString(value string) *string {
	return &value
}

func TestNormalizeDirectMethodTimeout(t *testing.T) {
	if got, err := normalizeDirectMethodTimeout(0); err != nil || got != directMethodDefaultTimeoutSeconds {
		t.Fatalf("default timeout = %d, %v", got, err)
	}
	for _, value := range []int{-1, directMethodMaxTimeoutSeconds + 1} {
		if _, err := normalizeDirectMethodTimeout(value); err == nil {
			t.Fatalf("timeout %d should be rejected", value)
		}
	}
}

func TestAuditableOnlineCommandDeliveryOption(t *testing.T) {
	options := commandDeliveryOptions{}
	requireAuditableOnlineCommandDelivery()(&options)
	if !options.requireOnline || !options.requireLogRecorded {
		t.Fatalf("direct method requirements = %#v", options)
	}
}

func TestNewCommandSetLogRecordsOperator(t *testing.T) {
	payload := `{"method":"reboot"}`
	log := newCommandSetLog(&model.Device{ID: "device-1"}, " operator-1 ", "msg-1", "reboot", &payload, "1")
	if log.DeviceID != "device-1" || log.MessageID == nil || *log.MessageID != "msg-1" || log.UserID == nil || *log.UserID != "operator-1" {
		t.Fatalf("auditable command log = %#v", log)
	}

	withoutOperator := newCommandSetLog(&model.Device{ID: "device-1"}, " ", "msg-2", "reboot", &payload, "2")
	if withoutOperator.UserID != nil {
		t.Fatalf("automation command without operator should keep user_id null: %#v", withoutOperator.UserID)
	}
}

func TestDirectMethodResultFromLog(t *testing.T) {
	base := &DirectMethodResult{MessageID: "msg-1", DeviceID: "device-1", Published: true, LogRecorded: true}
	tests := []struct {
		status          string
		outcome         string
		terminal        bool
		deviceResponded bool
		deviceSucceeded bool
	}{
		{status: "1", outcome: directMethodOutcomeAwaitingResponse},
		{status: "2", outcome: directMethodOutcomeDeliveryFailed, terminal: true},
		{status: "3", outcome: directMethodOutcomeDeviceSucceeded, terminal: true, deviceResponded: true, deviceSucceeded: true},
		{status: "4", outcome: directMethodOutcomeDeviceFailed, terminal: true, deviceResponded: true},
	}

	for _, test := range tests {
		result, terminal := directMethodResultFromLog(base, &model.CommandSetLog{
			Status:       directMethodString(test.status),
			RspDatum:     directMethodString(`{"result":0}`),
			ErrorMessage: directMethodString("device error"),
		})
		if result.Outcome != test.outcome || terminal != test.terminal || result.DeviceResponded != test.deviceResponded || result.DeviceSucceeded != test.deviceSucceeded {
			t.Fatalf("status %s result = %#v, terminal=%v", test.status, result, terminal)
		}
	}
}

func TestWaitForDirectMethodResultTimesOutWithTracking(t *testing.T) {
	base := &DirectMethodResult{MessageID: "msg-1", DeviceID: "device-1", Published: true, LogRecorded: true}
	result, err := waitForDirectMethodResult(
		context.Background(),
		base,
		2*time.Millisecond,
		time.Millisecond,
		func(context.Context, string, string) (*model.CommandSetLog, error) {
			return &model.CommandSetLog{Status: directMethodString("1")}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != directMethodOutcomeTimeout || !result.TimedOut || result.MessageID != "msg-1" || result.Status != "1" {
		t.Fatalf("timeout result = %#v", result)
	}
}

func TestWaitForDirectMethodResultStopsOnParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := waitForDirectMethodResult(
		ctx,
		&DirectMethodResult{MessageID: "msg-1", DeviceID: "device-1"},
		time.Second,
		time.Millisecond,
		func(context.Context, string, string) (*model.CommandSetLog, error) {
			t.Fatal("lookup should not run after parent cancellation")
			return nil, nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
}
