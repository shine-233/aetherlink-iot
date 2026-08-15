// 文件用途：把 Command Job 下发的「配额/限速准入」判定从数据库事务里抽成纯函数。
// 核心逻辑：给定当前全局/租户在飞数量、两个限速游标和策略，决定这次领取是放行还是推迟，
// 并给出推迟到的时间点与归因码。
// 关键注意事项：
//  1. 判定顺序必须是「全局并发 → 租户并发 → 限速游标」。运行事务里也是这个顺序，
//     顺序变化会改变归因码，运维看板据此判断该调哪个旋钮。
//  2. 并发用 >= 比较：计数是已经占用的槽位数，等于上限时已经没有余量。多实例下
//     若写成 >，每个实例都会各自多放一行出去。
//  3. 并发触顶用固定的 ContentionRetryInterval 重试，不沿用可能很远的限速游标，
//     否则一次拥塞会把后续下发拖停很久。
//  4. 限速推迟取两个游标里更晚的那个，且不额外加间隔——游标本身就是「下一个可发时刻」。
//  5. 本函数不读时钟：Now 必须由调用方从数据库时钟取，保证多实例判定同源。
package dal

import "time"

// Command Job 下发推迟的四种归因码。抽成常量避免调用方手写字符串。
const (
	CommandJobDispatchGateGlobalConcurrency = "global_concurrency"
	CommandJobDispatchGateTenantConcurrency = "tenant_concurrency"
	CommandJobDispatchGateGlobalRate        = "global_rate"
	CommandJobDispatchGateTenantRate        = "tenant_rate"
)

// CommandJobDispatchGateInput 是一次准入判定所需的全部输入。
// 用具名字段而不是位置参数，避免两个 int64 计数和两个 time.Time 游标被调错顺序。
type CommandJobDispatchGateInput struct {
	Policy CommandJobDispatchPolicy
	// Now 必须来自数据库时钟（clock_timestamp()），不能用进程本地时间。
	Now time.Time
	// GlobalDispatching / TenantDispatching 是仍在租约内的 dispatching 行数。
	GlobalDispatching int64
	TenantDispatching int64
	// GlobalNextDispatchAt / TenantNextDispatchAt 是持久化的限速游标。
	GlobalNextDispatchAt time.Time
	TenantNextDispatchAt time.Time
}

// CommandJobDispatchGateDecision 是准入判定的结果。
type CommandJobDispatchGateDecision struct {
	// Allow 为 true 时可以继续去锁 detail 行；为 false 时必须按 RetryAt 推迟。
	Allow bool
	// RetryAt 只在 Allow=false 时有意义。
	RetryAt time.Time
	// Reason 为空表示放行，否则是上面四种归因码之一。
	Reason string
}

// EvaluateCommandJobDispatchGate 判定一次下发领取是否放行。
func EvaluateCommandJobDispatchGate(input CommandJobDispatchGateInput) CommandJobDispatchGateDecision {
	// 并发上限优先于限速：并发满时就算限速游标已到也没有可用槽位。
	if input.GlobalDispatching >= int64(input.Policy.GlobalMaxConcurrent) {
		return CommandJobDispatchGateDecision{
			RetryAt: input.Now.Add(input.Policy.ContentionRetryInterval),
			Reason:  CommandJobDispatchGateGlobalConcurrency,
		}
	}
	if input.TenantDispatching >= int64(input.Policy.TenantMaxConcurrent) {
		return CommandJobDispatchGateDecision{
			RetryAt: input.Now.Add(input.Policy.ContentionRetryInterval),
			Reason:  CommandJobDispatchGateTenantConcurrency,
		}
	}

	rateEligibleAt := laterCommandJobDispatchTime(input.GlobalNextDispatchAt, input.TenantNextDispatchAt)
	if rateEligibleAt.After(input.Now) {
		// 两个游标相等时归因到 global：全局是更强的约束，运维应先看全局限速。
		reason := CommandJobDispatchGateTenantRate
		if !input.GlobalNextDispatchAt.Before(input.TenantNextDispatchAt) {
			reason = CommandJobDispatchGateGlobalRate
		}
		return CommandJobDispatchGateDecision{RetryAt: rateEligibleAt, Reason: reason}
	}

	return CommandJobDispatchGateDecision{Allow: true}
}
