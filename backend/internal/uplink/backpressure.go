// backpressure.go 落实摄取回压短期 B 方案（P2.3，决策备忘录
// docs/design/2026-09-19-ingest-backpressure-decision-memo.md）：
// 把"满队列即阻塞、paho 入站队列静默丢已 PUBACK 消息"的最危险属性——静默——消掉。
//
// 计数语义（进程内账本，received = accepted + 三类 dropped 严格成立）：
//   received               通过发布闸门且类型可路由的消息数（含最终被丢的）
//   accepted               成功进入对应分类队列的消息数
//   dropped_channel_full   响应链路满队列直接丢弃（该链路的显式丢弃策略）
//   dropped_caller_context 阻塞等待期间调用方 ctx 取消/超时，消息未入队
//   dropped_bus_closed     阻塞等待期间总线关闭中止，消息未入队
//   rejected_admission     发布闸门（closing/closed）在准入前拒绝的消息数（不占 received）
//   dropped_unknown_type   未知消息类型拒绝（不占 received）
//   blocked                队列满后进入阻塞等待的事件数与累计耗时——
//                          阻塞发生在订阅者回调线程上，是 paho 入站队列
//                          溢出丢失"已 PUBACK"消息的先行指标
//
// paho 库内部的丢失（到达回调之前）在本进程不可见，端到端总账需结合
// broker 侧计数（GMQTT $SYS / 探针 offered）与落库行数推导；这属于
// 中期 C 方案（反压到 broker）与容量模型的配套工作，不在 B 范围内。
package uplink

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// 账本按消息在总线上的去向分类；响应类五种消息类型共享同一响应队列，合并计数。
const (
	acctKindTelemetry = iota
	acctKindAttribute
	acctKindEvent
	acctKindStatus
	acctKindResponse
	acctKindCount
)

var acctKindNames = [acctKindCount]string{"telemetry", "attribute", "event", "status", "response"}

// accountingKindFor 把消息类型映射为账本分类；第二个返回值为 false 表示未知类型。
func accountingKindFor(msgType string) (int, bool) {
	switch msgType {
	case MessageTypeTelemetry, "gateway_telemetry":
		return acctKindTelemetry, true
	case MessageTypeAttribute, "gateway_attribute":
		return acctKindAttribute, true
	case MessageTypeEvent, "gateway_event":
		return acctKindEvent, true
	case MessageTypeStatus:
		return acctKindStatus, true
	}
	if isResponseMessageType(msgType) {
		return acctKindResponse, true
	}
	return 0, false
}

// UplinkDroppedTotalKey 是快照中总丢弃计数的键名，与决策备忘录验收口径中的
// `uplink_dropped_total` 对应。
const UplinkDroppedTotalKey = "uplink_dropped_total"

type kindCounters [acctKindCount]atomic.Uint64

// busAccounting 是总线进程内的摄取账本。全部原子计数，热路径零锁。
type busAccounting struct {
	receivedTotal atomic.Uint64
	acceptedTotal atomic.Uint64
	droppedTotal  atomic.Uint64
	rejectedTotal atomic.Uint64

	droppedUnknownType atomic.Uint64

	received             kindCounters
	accepted             kindCounters
	droppedChannelFull   kindCounters
	droppedCallerContext kindCounters
	droppedBusClosed     kindCounters

	blockedEvents kindCounters
	blockedNanos  kindCounters
}

func (a *busAccounting) accountReceived(kind int) {
	a.receivedTotal.Add(1)
	a.received[kind].Add(1)
}

// accountPublishOutcome 在消息进入队列或确定被丢时记账。err 为 nil 即接受。
func (a *busAccounting) accountPublishOutcome(kind int, err error) {
	if err == nil {
		a.acceptedTotal.Add(1)
		a.accepted[kind].Add(1)
		return
	}
	a.droppedTotal.Add(1)
	switch {
	case errors.Is(err, ErrChannelFull):
		a.droppedChannelFull[kind].Add(1)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		a.droppedCallerContext[kind].Add(1)
	case errors.Is(err, ErrBusClosed):
		a.droppedBusClosed[kind].Add(1)
	}
}

func perKindSnapshot(c *kindCounters) map[string]uint64 {
	m := make(map[string]uint64, acctKindCount)
	for i, name := range acctKindNames {
		m[name] = c[i].Load()
	}
	return m
}

// snapshot 输出账本全量快照。received 与 accepted/dropped 的分账在同一进程内
// 严格自洽（received = accepted + channel_full + caller_context + bus_closed）。
func (a *busAccounting) snapshot() map[string]interface{} {
	return map[string]interface{}{
		"received_total":       a.receivedTotal.Load(),
		"accepted_total":       a.acceptedTotal.Load(),
		UplinkDroppedTotalKey:  a.droppedTotal.Load(),
		"rejected_admission":   a.rejectedTotal.Load(),
		"dropped_unknown_type": a.droppedUnknownType.Load(),
		"received":             perKindSnapshot(&a.received),
		"accepted":             perKindSnapshot(&a.accepted),
		"dropped_channel_full":   perKindSnapshot(&a.droppedChannelFull),
		"dropped_caller_context": perKindSnapshot(&a.droppedCallerContext),
		"dropped_bus_closed":     perKindSnapshot(&a.droppedBusClosed),
		"blocked_events":         perKindSnapshot(&a.blockedEvents),
		"blocked_seconds_total":  blockedSecondsSnapshot(&a.blockedNanos),
	}
}

func blockedSecondsSnapshot(c *kindCounters) map[string]float64 {
	m := make(map[string]float64, acctKindCount)
	for i, name := range acctKindNames {
		m[name] = math.Round(float64(c[i].Load())/1e9*1e6) / 1e6
	}
	return m
}

// BackpressureAlertConfig 是丢弃率告警采样器的配置。
type BackpressureAlertConfig struct {
	Enabled     bool
	Window      time.Duration // 采样窗口长度
	DropRatio   float64       // 窗口内 dropped/received 超过该比例即告警
	MinReceived uint64        // 窗口内接收量低于该值不评估，避免低流量误报
}

// DefaultBackpressureAlertConfig 给出默认告警参数。阈值依据决策备忘录的
// 实测数据：健康稳态丢弃应为 0，1% 即视为异常。
func DefaultBackpressureAlertConfig() BackpressureAlertConfig {
	return BackpressureAlertConfig{
		Enabled:     true,
		Window:      60 * time.Second,
		DropRatio:   0.01,
		MinReceived: 100,
	}
}

// 告警配置的 viper 键。uplink 包不直接依赖 viper，由应用装配层读取后注入。
const (
	AlertEnabledKey     = "telemetry.uplink_backpressure_alert.enabled"
	AlertWindowKey      = "telemetry.uplink_backpressure_alert.window_seconds"
	AlertDropRatioKey   = "telemetry.uplink_backpressure_alert.drop_ratio"
	AlertMinReceivedKey = "telemetry.uplink_backpressure_alert.min_received"
)

// AlertLogMarker 是告警日志的稳定检索标记，供日志告警管道按字面量订阅。
const AlertLogMarker = "UPLINK_BACKPRESSURE_ALERT"

// evaluateBackpressureDropRatio 是告警判定的纯函数：窗口接收量达到下限且
// 丢弃率严格高于阈值才告警。返回 (ratio, shouldAlert)。
func evaluateBackpressureDropRatio(received, dropped, minReceived uint64, threshold float64) (float64, bool) {
	if received == 0 || received < minReceived {
		return 0, false
	}
	ratio := float64(dropped) / float64(received)
	return ratio, ratio > threshold
}

type backpressureAlertState struct {
	lastAlertAtUnixNano atomic.Int64
	lastRatioBits       atomic.Uint64
	alertCount          atomic.Uint64
}

func (s *backpressureAlertState) snapshot(cfg BackpressureAlertConfig) map[string]interface{} {
	out := map[string]interface{}{
		"enabled":              true,
		"window_seconds":       int(cfg.Window.Seconds()),
		"drop_ratio_threshold": cfg.DropRatio,
		"min_received":         cfg.MinReceived,
		"alert_count":          s.alertCount.Load(),
		"last_alert_at":        nil,
		"last_drop_ratio":      nil,
	}
	if at := s.lastAlertAtUnixNano.Load(); at != 0 {
		out["last_alert_at"] = time.Unix(0, at).UTC().Format(time.RFC3339Nano)
		if bits := s.lastRatioBits.Load(); bits != 0 {
			out["last_drop_ratio"] = math.Float64frombits(bits)
		}
	}
	return out
}

// startBackpressureAlertSampler 启动窗口采样 goroutine；随 abortPublish 关闭退出。
// 每个窗口把账本增量与阈值比较，越限即输出带稳定标记的 ERROR 日志并记录状态，
// 状态随 GetChannelStats 快照暴露，可演练（压低窗口/阈值即可触发）。
func (b *Bus) startBackpressureAlertSampler(cfg BackpressureAlertConfig) {
	prevReceived := b.acct.receivedTotal.Load()
	prevDropped := b.acct.droppedTotal.Load()

	go func() {
		ticker := time.NewTicker(cfg.Window)
		defer ticker.Stop()
		for {
			select {
			case <-b.abortPublish:
				return
			case <-ticker.C:
				curReceived := b.acct.receivedTotal.Load()
				curDropped := b.acct.droppedTotal.Load()
				winReceived := curReceived - prevReceived
				winDropped := curDropped - prevDropped
				prevReceived, prevDropped = curReceived, curDropped

				ratio, fire := evaluateBackpressureDropRatio(winReceived, winDropped, cfg.MinReceived, cfg.DropRatio)
				if !fire {
					continue
				}
				b.alertState.alertCount.Add(1)
				b.alertState.lastAlertAtUnixNano.Store(time.Now().UnixNano())
				b.alertState.lastRatioBits.Store(math.Float64bits(ratio))
				b.logger.WithFields(logrus.Fields{
					"marker":          AlertLogMarker,
					"window":          cfg.Window.String(),
					"window_received": winReceived,
					"window_dropped":  winDropped,
					"drop_ratio":      ratio,
					"threshold":       cfg.DropRatio,
				}).Error("上行摄取回压告警：窗口内丢弃率超过阈值，已 PUBACK 消息可能被静默丢弃")
			}
		}
	}()
}

// diagnosticsBus 持有应用装配层注册的诊断用总线实例，供 /queue/stats 暴露。
var diagnosticsBus atomic.Pointer[Bus]

// RegisterDiagnosticsBus 注册（或替换）诊断总线。进程内单例语义，最后注册者生效。
func RegisterDiagnosticsBus(b *Bus) {
	diagnosticsBus.Store(b)
}

// DefaultBusSnapshot 返回已注册诊断总线的快照；未注册时返回 nil。
func DefaultBusSnapshot() map[string]interface{} {
	b := diagnosticsBus.Load()
	if b == nil {
		return nil
	}
	return b.GetChannelStats()
}
