// 文件用途：场景/Flow 的执行窗口与冲突停止语义（ROADMAP P0.4）。
// 核心逻辑：把"开始/结束时间、时区、过期、停止其他 Flow、重复触发幂等"收敛到一处。
// 关键注意事项：
//  1. 时区非法一律 fail closed（不可执行），绝不悄悄按 UTC 放行——那会让窗口边界整体偏移。
//  2. 边界采用 [starts_at, expires_at) 左闭右开，避免同一时刻被两个窗口同时命中。
//  3. 停止其他 Flow 必须留下审计事件，禁止静默停止。
package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrInvalidExecutionTimezone 时区非法；调用方必须视为不可执行。
var ErrInvalidExecutionTimezone = errors.New("execution window timezone is invalid")

// ExecutionWindow 场景执行窗口。StartsAt/ExpiresAt 为 nil 表示该侧无界。
type ExecutionWindow struct {
	StartsAt  *time.Time
	ExpiresAt *time.Time
	Timezone  string
}

// FlowEngine 场景执行引擎，持有可注入的运行态注册表以便测试与替换实现。
type FlowEngine struct {
	registry FlowRunRegistry
	audit    FlowAuditSink
}

// FlowRunRegistry 运行中的 Flow 注册表抽象。
type FlowRunRegistry interface {
	// ListRunningFlows 返回指定设备上正在运行、且不属于 excludeFlowID 的 flow id。
	ListRunningFlows(ctx context.Context, deviceID, excludeFlowID string) ([]string, error)
	// StopFlow 停止指定 flow，返回是否确实发生了状态变化。
	StopFlow(ctx context.Context, deviceID, flowID string) (bool, error)
}

// FlowAuditSink 审计落点抽象，保证"停止动作可审计"。
type FlowAuditSink interface {
	RecordFlowStopped(ctx context.Context, deviceID, flowID, reason string) error
}

// NewFlowEngine 构造场景执行引擎。
func NewFlowEngine(registry FlowRunRegistry, audit FlowAuditSink) *FlowEngine {
	return &FlowEngine{registry: registry, audit: audit}
}

// CanRun 判断 now 是否落在执行窗口内。
// 语义：
//   - 时区为空视为 UTC；时区非法返回错误（fail closed）。
//   - StartsAt 为 nil 表示无下界；ExpiresAt 为 nil 表示无上界；两者皆 nil 表示始终可执行。
//   - 区间为左闭右开 [starts_at, expires_at)：恰好等于 expires_at 的时刻不再执行。
func (e FlowEngine) CanRun(now time.Time, w ExecutionWindow) (bool, error) {
	zone := w.Timezone
	if zone == "" {
		zone = "UTC"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		// 非法时区不做"按 UTC 兜底"的静默降级：窗口边界会整体偏移，属于伪造可执行性。
		return false, fmt.Errorf("%w: %q", ErrInvalidExecutionTimezone, w.Timezone)
	}
	current := now.In(loc)
	if w.StartsAt != nil && current.Before(*w.StartsAt) {
		return false, nil
	}
	if w.ExpiresAt != nil && !current.Before(*w.ExpiresAt) {
		return false, nil
	}
	return true, nil
}

// IsExpired 判断窗口是否已过期（expires_at 非空且 now 已达或超过）。
func (e FlowEngine) IsExpired(now time.Time, w ExecutionWindow) (bool, error) {
	if w.ExpiresAt == nil {
		return false, nil
	}
	zone := w.Timezone
	if zone == "" {
		zone = "UTC"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return false, fmt.Errorf("%w: %q", ErrInvalidExecutionTimezone, w.Timezone)
	}
	return !now.In(loc).Before(*w.ExpiresAt), nil
}

// FlowTriggerIdentity 同一次触发的幂等身份。
// 同一 (flow, device, 触发时刻) 重复到达只应执行一次，避免定时器抖动把一次触发放大成多次。
type FlowTriggerIdentity struct {
	FlowID   string
	DeviceID string
	// TriggerAt 触发时刻，按窗口粒度对齐（默认截断到秒）。
	TriggerAt time.Time
}

// FlowTriggerKey 生成幂等键；同一身份必然得到同一键。
func FlowTriggerKey(id FlowTriggerIdentity) string {
	return fmt.Sprintf("flow=%s|device=%s|at=%s",
		id.FlowID, id.DeviceID, id.TriggerAt.UTC().Truncate(time.Second).Format(time.RFC3339))
}

// StopConflictingFlows 停止同一设备上除自身以外的运行中 Flow，并逐条留下审计事件。
// 返回被停止的 flow id 列表；注册表或审计缺失时返回错误，不做静默停止。
func (e FlowEngine) StopConflictingFlows(ctx context.Context, deviceID, flowID string) ([]string, error) {
	if e.registry == nil {
		return nil, errors.New("flow run registry is not configured")
	}
	if e.audit == nil {
		return nil, errors.New("flow audit sink is not configured; stopping flows requires an audit trail")
	}
	running, err := e.registry.ListRunningFlows(ctx, deviceID, flowID)
	if err != nil {
		return nil, err
	}
	stopped := make([]string, 0, len(running))
	for _, other := range running {
		if other == flowID {
			continue
		}
		changed, err := e.registry.StopFlow(ctx, deviceID, other)
		if err != nil {
			return stopped, err
		}
		if !changed {
			// 已被其他路径停止：不重复审计，也不计入本次停止列表。
			continue
		}
		if err := e.audit.RecordFlowStopped(ctx, deviceID, other, fmt.Sprintf("conflicting flow %s started", flowID)); err != nil {
			return stopped, err
		}
		stopped = append(stopped, other)
	}
	return stopped, nil
}
