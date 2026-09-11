// 文件用途：设备影子消息的状态机词汇与重试退避（ROADMAP P0.2）。
// 手写文件：device_shadow.gen.go 由生成器产出且已不在当前生成清单内，故新增列与状态语义放此处，
// 写入一律走 DAL 的 map 式 Updates，不修改生成结构体。
package model

import "time"

// 影子消息状态（P0.2 ACK 闭环）。
const (
	// ShadowStatusPending 已入队，尚未下发。
	ShadowStatusPending = "pending"
	// ShadowStatusSent 已下发，等待设备 ACK；此状态不得对外宣称“已送达”。
	ShadowStatusSent = "sent"
	// ShadowStatusDelivered 设备已 ACK，确认送达。
	ShadowStatusDelivered = "delivered"
	// ShadowStatusFailed 超过最大尝试次数仍未收到 ACK，终态。
	ShadowStatusFailed = "failed"
	// ShadowStatusExpired 超过 TTL 仍未送达，终态。
	ShadowStatusExpired = "expired"
	// ShadowStatusCanceled 被显式取消，终态。
	ShadowStatusCanceled = "canceled"
)

// 终态集合：不可再投递、不可 ACK、不可取消。
var shadowTerminalStatuses = map[string]bool{
	ShadowStatusDelivered: true,
	ShadowStatusFailed:    true,
	ShadowStatusExpired:   true,
	ShadowStatusCanceled:  true,
}

// IsShadowTerminalStatus 判断状态是否为终态。
func IsShadowTerminalStatus(status string) bool { return shadowTerminalStatuses[status] }

// 可被设备 ACK 的状态：只有 pending（尚未下发但设备已收到）与 sent 允许确认。
func IsShadowAckableStatus(status string) bool {
	return status == ShadowStatusPending || status == ShadowStatusSent
}

const (
	// ShadowMaxAttempts 未收到 ACK 时的最大下发次数，超出即 failed。
	ShadowMaxAttempts = 3
	// shadowBaseBackoff 退避基数，第 n 次重试等待 base * 2^(n-1)。
	shadowBaseBackoff = 30 * time.Second
	// shadowMaxBackoff 退避上限，避免长尾设备被无限推迟。
	shadowMaxBackoff = 10 * time.Minute
)

// ShadowRetryBackoff 返回第 attempts 次（从 1 开始）下发后的等待时长。
// 采用有上限的指数退避，避免无界重试与惊群。
func ShadowRetryBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	backoff := shadowBaseBackoff
	for i := 1; i < attempts; i++ {
		backoff *= 2
		if backoff >= shadowMaxBackoff {
			return shadowMaxBackoff
		}
	}
	if backoff > shadowMaxBackoff {
		return shadowMaxBackoff
	}
	return backoff
}

// ShadowNextAttemptAt 计算下一次允许重投的时间点。
func ShadowNextAttemptAt(now time.Time, attempts int) time.Time {
	return now.UTC().Add(ShadowRetryBackoff(attempts))
}
