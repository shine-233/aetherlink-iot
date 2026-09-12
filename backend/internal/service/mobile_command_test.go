package service

import (
	"context"
	"errors"
	"testing"

	utils "aetherlink-iot/backend/pkg/utils"
)

// fakeCommandSender 记录下发次数，用于验证弱网重试不会重复下发。
type fakeCommandSender struct {
	calls    int
	failWith error
}

func (f *fakeCommandSender) Send(ctx context.Context, exec ControlExecution) error {
	f.calls++
	return f.failWith
}

// TestMobileCapabilitiesReflectActualWiring 锁定"能力矩阵由实际接线决定"：
// 未注入的依赖必须报 false，且调用时报错，绝不能假装可用。
func TestMobileCapabilitiesReflectActualWiring(t *testing.T) {
	svc := NewMobileService(MobileServiceDeps{})
	caps := svc.Capabilities(context.Background())
	if caps.Commands || caps.Shadow || caps.Alarms || caps.OTA || caps.Dashboards || caps.Push || caps.Telemetry {
		t.Fatalf("empty service must report everything unwired, got %+v", caps)
	}
	// 离线缓存是客户端能力，服务端无从得知，恒为 false。
	if caps.OfflineCache {
		t.Fatal("offline cache must never be claimed by the server")
	}

	if _, _, err := svc.ListDevices(context.Background(), &utils.UserClaims{ID: "u", TenantID: "t"}, "", 1, 10); err == nil {
		t.Fatal("ListDevices on unwired service must fail, not return an empty list")
	}
	if err := svc.AcknowledgeAlarm(context.Background(), &utils.UserClaims{ID: "u", TenantID: "t"}, "a"); err == nil {
		t.Fatal("AcknowledgeAlarm on unwired service must fail")
	}
	if _, err := svc.GetShadow(context.Background(), &utils.UserClaims{ID: "u", TenantID: "t"}, "d"); err == nil {
		t.Fatal("GetShadow on unwired service must fail")
	}
	if _, err := svc.OTAStatus(context.Background(), &utils.UserClaims{ID: "u", TenantID: "t"}, "d"); err == nil {
		t.Fatal("OTAStatus on unwired service must fail")
	}
	if _, err := svc.ListDashboards(context.Background(), &utils.UserClaims{ID: "u", TenantID: "t"}); err == nil {
		t.Fatal("ListDashboards on unwired service must fail")
	}
	if _, err := svc.SendCommand(context.Background(), "t", "u", "k", ControlExecution{}); err == nil {
		t.Fatal("SendCommand on unwired service must fail")
	}

	sender := &fakeCommandSender{}
	wired := NewMobileService(MobileServiceDeps{Commands: sender})
	if !wired.Capabilities(context.Background()).Commands {
		t.Fatal("commands must be reported available once a sender is wired")
	}
}

// TestMobileSendCommandIdempotentAcrossRetries 锁定门禁"弱网重试不重复命令"：
// 同一幂等键重复提交只真正下发一次，且第二次返回的是第一次的收据。
func TestMobileSendCommandIdempotentAcrossRetries(t *testing.T) {
	sender := &fakeCommandSender{}
	svc := NewMobileService(MobileServiceDeps{Commands: sender, Idempotency: NewInMemoryCommandIdempotencyStore(0)})
	ctx := context.Background()
	exec := ControlExecution{TenantID: "t1", DeviceID: "d1", Command: "open_valve", Params: `{"v":1}`}

	first, err := svc.SendCommand(ctx, "t1", "u1", "key-1", exec)
	if err != nil || first == nil || !first.Accepted {
		t.Fatalf("first send = (%v, %v), want accepted", first, err)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls)
	}

	// 弱网重试：客户端没收到响应，用同一个键再发一次。
	second, err := svc.SendCommand(ctx, "t1", "u1", "key-1", exec)
	if err != nil || second == nil || !second.Accepted {
		t.Fatalf("retry = (%v, %v), want the original accepted receipt", second, err)
	}
	if sender.calls != 1 {
		t.Fatalf("retry must not re-dispatch: sender calls = %d, want 1", sender.calls)
	}
	if second.FinishedAt != first.FinishedAt {
		t.Fatal("retry must return the original receipt verbatim")
	}
}

// TestMobileSendCommandReleasesKeyOnFailure 锁定"失败必须释放幂等键"：
// 否则客户端重试会被判为已处理，命令永远发不出去。
func TestMobileSendCommandReleasesKeyOnFailure(t *testing.T) {
	sender := &fakeCommandSender{failWith: errors.New("broker unreachable")}
	svc := NewMobileService(MobileServiceDeps{Commands: sender, Idempotency: NewInMemoryCommandIdempotencyStore(0)})
	ctx := context.Background()
	exec := ControlExecution{TenantID: "t1", DeviceID: "d1", Command: "open_valve"}

	if _, err := svc.SendCommand(ctx, "t1", "u1", "key-2", exec); err == nil {
		t.Fatal("failing send must return an error")
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls)
	}

	// 故障排除后重试必须真的能发出去。
	sender.failWith = nil
	got, err := svc.SendCommand(ctx, "t1", "u1", "key-2", exec)
	if err != nil || got == nil || !got.Accepted {
		t.Fatalf("retry after failure = (%v, %v), want accepted", got, err)
	}
	if sender.calls != 2 {
		t.Fatalf("failed command must be retryable: sender calls = %d, want 2", sender.calls)
	}
}

// TestMobileSendCommandRejectsKeyReuseWithDifferentParams 锁定：同一幂等键换参数必须拒绝。
// 否则第二条命令会返回第一条的结果，用户以为生效其实没生效。
func TestMobileSendCommandRejectsKeyReuseWithDifferentParams(t *testing.T) {
	sender := &fakeCommandSender{}
	svc := NewMobileService(MobileServiceDeps{Commands: sender, Idempotency: NewInMemoryCommandIdempotencyStore(0)})
	ctx := context.Background()

	if _, err := svc.SendCommand(ctx, "t1", "u1", "key-3", ControlExecution{DeviceID: "d1", Command: "open"}); err != nil {
		t.Fatalf("first send: %v", err)
	}
	_, err := svc.SendCommand(ctx, "t1", "u1", "key-3", ControlExecution{DeviceID: "d2", Command: "close"})
	if err == nil {
		t.Fatal("reusing a key with different parameters must be rejected")
	}
}

func TestMobileSendCommandRequiresIdempotencyKey(t *testing.T) {
	sender := &fakeCommandSender{}
	svc := NewMobileService(MobileServiceDeps{Commands: sender, Idempotency: NewInMemoryCommandIdempotencyStore(0)})
	if _, err := svc.SendCommand(context.Background(), "t1", "u1", "  ", ControlExecution{DeviceID: "d1"}); err == nil {
		t.Fatal("empty idempotency key must be rejected")
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

func TestMobileIdempotencyIsScopedPerTenantAndUser(t *testing.T) {
	sender := &fakeCommandSender{}
	svc := NewMobileService(MobileServiceDeps{Commands: sender, Idempotency: NewInMemoryCommandIdempotencyStore(0)})
	ctx := context.Background()
	exec := ControlExecution{DeviceID: "d1", Command: "open"}

	if _, err := svc.SendCommand(ctx, "t1", "u1", "shared-key", exec); err != nil {
		t.Fatalf("tenant t1: %v", err)
	}
	if _, err := svc.SendCommand(ctx, "t2", "u1", "shared-key", exec); err != nil {
		t.Fatalf("same key under another tenant must be independent: %v", err)
	}
	if sender.calls != 2 {
		t.Fatalf("sender calls = %d, want 2 (key scope must include tenant)", sender.calls)
	}
}
