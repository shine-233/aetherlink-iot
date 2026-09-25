// 文件用途：边缘本地规则执行器（TB-21 scoped v1）——快照装载（修订号去重）与遥测求值调度。
// 核心逻辑：LoadSnapshot 收 edge_sync 快照信封，按 (resource_id, revision) 四态去重——
//
//	修订号未变（重连重放同一任务）跳过不重复执行、回退跳过并暴露、升级则整图替换重装载；
//	ProcessTelemetry 对全部已装载图求值，命中经 retrigger 去重窗口后交给 EventSink 落日志/本地存储。
//
// 关键注意事项：
//   - 全程 mutex 串行化，边缘 broker 多连接并发投递安全；时钟可注入（去重窗口测试不 sleep）。
//   - retrigger 窗口键=resource|node|device（对照云端 action.alarm 的 node+device 语义），
//     窗口内重复触发不落新事件也不计入返回值。
//   - 本地告警只进 EventSink（进程内结构+日志/本地存储接口），云端告警收敛去重演练在 v1 范围外。
package edgerules

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// LoadStatus 快照装载结果。
type LoadStatus string

const (
	// LoadStatusLoaded 已装载/升级重装载成功。
	LoadStatusLoaded LoadStatus = "loaded"
	// LoadStatusUnchanged 修订号未变：重连去重跳过（不重复执行）。
	LoadStatusUnchanged LoadStatus = "unchanged"
	// LoadStatusRegression 修订号回退：异常跳过（边缘超前于云端，需人工确认）。
	LoadStatusRegression LoadStatus = "regression"
	// LoadStatusRejected 拒收：解析失败/非法修订号/非法图（fail-closed，保留当前版本继续跑）。
	LoadStatusRejected LoadStatus = "rejected"
)

// LoadResult 一次 LoadSnapshot 的结果。
type LoadResult struct {
	Status     LoadStatus `json:"status"`
	ResourceID string     `json:"resource_id,omitempty"`
	Revision   int64      `json:"revision,omitempty"` // 拒收且读不出修订号时为 0
	Reason     string     `json:"reason,omitempty"`   // rejected/regression 时的原因
	// UnsupportedTypes 快照中边缘 v1 不支持的节点类型（装载成功也可能带）；
	// 这些节点求值时阻断其路径（fail-closed），供运维评估边缘/云端语义差。
	UnsupportedTypes []string `json:"unsupported_types,omitempty"`
}

// AlarmEvent 本地告警事件（结构体即本地存储接口的最小契约；JSON 标签供日志/落盘序列化）。
type AlarmEvent struct {
	ID          string         `json:"id"`                   // 事件唯一 ID（uuid）
	ResourceID  string         `json:"resource_id"`          // 规则链资源 ID（快照 resource_id）
	ChainName   string         `json:"chain_name,omitempty"` // 快照 name
	Revision    int64          `json:"revision"`             // 装载时快照修订号（可追溯执行的是哪版规则）
	NodeID      string         `json:"node_id"`              // 命中的 action.alarm 节点
	AlarmName   string         `json:"alarm_name"`
	Severity    string         `json:"severity"` // L/M/H（对照云端语义）
	Description string         `json:"description,omitempty"`
	Content     string         `json:"content,omitempty"`
	Condition   string         `json:"condition,omitempty"` // 命中路径上最后通过的阈值条件（留痕，对照 calcfield detail）
	DeviceID    string         `json:"device_id"`           // 命中设备（边缘本地标识）
	TenantID    string         `json:"tenant_id,omitempty"`
	Values      map[string]any `json:"values,omitempty"` // 命中时遥测快照
	TriggeredAt time.Time      `json:"triggered_at"`
}

// EventSink 本地告警落地接口：日志/本地文件/边缘存储实现之。同步调用，实现方不得阻塞过久。
type EventSink interface {
	RecordAlarm(event AlarmEvent) error
}

// LogSink 默认落地：logrus 结构化日志（边缘无头环境的最小可用实现）。
type LogSink struct {
	Log *logrus.Logger
}

// RecordAlarm 以 warn 级别输出结构化告警日志。
func (s LogSink) RecordAlarm(event AlarmEvent) error {
	log := s.Log
	if log == nil {
		log = logrus.StandardLogger()
	}
	log.WithFields(logrus.Fields{
		"event":       "edge_local_alarm",
		"alarm_id":    event.ID,
		"resource_id": event.ResourceID,
		"revision":    event.Revision,
		"node_id":     event.NodeID,
		"alarm_name":  event.AlarmName,
		"severity":    event.Severity,
		"device_id":   event.DeviceID,
		"tenant_id":   event.TenantID,
	}).Warn("边缘本地告警命中")
	return nil
}

// MemorySink 进程内收集器（测试/调试用；非持久化，勿在生产使用）。
type MemorySink struct {
	mu     sync.Mutex
	Events []AlarmEvent
}

// RecordAlarm 追加事件（并发安全）。
func (s *MemorySink) RecordAlarm(event AlarmEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Events = append(s.Events, event)
	return nil
}

// Snapshot 追加事件的拷贝（并发安全读取）。
func (s *MemorySink) Snapshot() []AlarmEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AlarmEvent, len(s.Events))
	copy(out, s.Events)
	return out
}

// loadedChain 一张已装载的图及其快照上下文。
type loadedChain struct {
	snapshot *Snapshot
	graph    *Graph
	loadedAt time.Time
}

// Executor 边缘本地规则执行器。
type Executor struct {
	mu       sync.Mutex
	sink     EventSink
	log      *logrus.Logger
	tenantID string
	now      func() time.Time
	chains   map[string]*loadedChain // resource_id -> loaded
	// retrigger 窗口状态：dedupKey -> 上次触发时间（对照云端 action.alarm 进程内去重）。
	lastFired map[string]time.Time
}

// Option 执行器构造选项。
type Option func(*Executor)

// WithSink 替换告警落地实现（默认 LogSink）。
func WithSink(sink EventSink) Option {
	return func(x *Executor) { x.sink = sink }
}

// WithLogger 注入 logger（默认 logrus.StandardLogger）。
func WithLogger(log *logrus.Logger) Option {
	return func(x *Executor) { x.log = log }
}

// WithTenantID 设置本地事件的租户归属（边缘网关归属租户；空表示未知）。
func WithTenantID(tenantID string) Option {
	return func(x *Executor) { x.tenantID = tenantID }
}

// WithNow 注入时钟（测试去重窗口用）。
func WithNow(now func() time.Time) Option {
	return func(x *Executor) { x.now = now }
}

// NewExecutor 构造执行器。
func NewExecutor(opts ...Option) *Executor {
	x := &Executor{
		sink:      LogSink{},
		log:       logrus.StandardLogger(),
		now:       time.Now,
		chains:    map[string]*loadedChain{},
		lastFired: map[string]time.Time{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(x)
		}
	}
	return x
}

// LoadSnapshot 处理一份 edge_sync 快照信封：解析 → 修订号四态去重 → 升级则整图替换。
// 任何情况下都不返回 error（结果在 LoadResult 里，边缘主循环只记日志不中断）。
func (x *Executor) LoadSnapshot(raw []byte) LoadResult {
	snap, graph, unsupported, err := ParseSnapshot(raw)
	if err != nil {
		return LoadResult{Status: LoadStatusRejected, Reason: err.Error()}
	}
	result := LoadResult{ResourceID: snap.ResourceID, Revision: snap.Revision, UnsupportedTypes: unsupported}
	x.mu.Lock()
	defer x.mu.Unlock()
	current := int64(0)
	if loaded, ok := x.chains[snap.ResourceID]; ok {
		current = loaded.snapshot.Revision
	}
	switch ClassifyRevision(current, snap.Revision) {
	case RevisionStateInvalid:
		result.Status = LoadStatusRejected
		result.Reason = fmt.Sprintf("revision %d is not a valid version", snap.Revision)
		return result
	case RevisionStateUnchanged:
		// 重连/重放去重：修订号未变不重复执行，保留当前装载。
		result.Status = LoadStatusUnchanged
		result.Reason = "revision unchanged, deduplicated"
		return result
	case RevisionStateRegression:
		result.Status = LoadStatusRegression
		result.Reason = fmt.Sprintf("revision %d behind loaded %d (edge ahead of cloud)", snap.Revision, current)
		return result
	default: // RevisionStateNewer
		x.chains[snap.ResourceID] = &loadedChain{snapshot: snap, graph: graph, loadedAt: x.now()}
		result.Status = LoadStatusLoaded
		return result
	}
}

// LoadedRevision 返回某资源当前装载的修订号（未装载 ok=false）。
func (x *Executor) LoadedRevision(resourceID string) (int64, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	loaded, ok := x.chains[resourceID]
	if !ok {
		return 0, false
	}
	return loaded.snapshot.Revision, true
}

// LoadedResources 返回已装载资源 ID（排序稳定，便于观测）。
func (x *Executor) LoadedResources() []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	ids := make([]string, 0, len(x.chains))
	for id := range x.chains {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ProcessTelemetry 用一份遥测对所有已装载图求值；命中的告警经去重窗口后写 EventSink。
// 返回本次真正落地的告警事件（去重跳过的不在列）。设备 ID 是边缘本地标识，不参与图选择——
// v1 快照不区分目标设备，网关域内所有遥测都参与求值。
func (x *Executor) ProcessTelemetry(deviceID string, values map[string]any, ts time.Time) []AlarmEvent {
	if len(values) == 0 {
		return nil
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if ts.IsZero() {
		ts = x.now()
	}
	// 固定遍历顺序：资源 ID 排序，保证多图命中顺序确定（可测试/可审计）。
	resources := make([]string, 0, len(x.chains))
	for id := range x.chains {
		resources = append(resources, id)
	}
	sort.Strings(resources)

	fired := make([]AlarmEvent, 0, 1)
	for _, resourceID := range resources {
		loaded := x.chains[resourceID]
		result := Evaluate(loaded.graph, values)
		for _, blocked := range result.Blocked {
			x.log.WithFields(logrus.Fields{
				"resource_id": resourceID,
				"node_id":     blocked,
			}).Debug("edge rules: path blocked by unsupported node")
		}
		for _, hit := range result.Fired {
			dedupKey := retriggerKey(resourceID, hit.NodeID, deviceID)
			if windowMs := x.dedupWindowMs(loaded, hit.NodeID); windowMs > 0 {
				if last, ok := x.lastFired[dedupKey]; ok {
					if ts.Sub(last) < time.Duration(windowMs)*time.Millisecond {
						x.log.WithFields(logrus.Fields{
							"resource_id": resourceID,
							"node_id":     hit.NodeID,
							"device_id":   deviceID,
						}).Debug("edge rules: retrigger deduplicated within window")
						continue
					}
				}
				x.lastFired[dedupKey] = ts
			}
			content := hit.Content
			if content == "" && hit.Condition != "" {
				// 对照 calcfield 的 detail content：无显式 content 时以命中的条件文本留痕。
				content = fmt.Sprintf("condition %q satisfied", hit.Condition)
			}
			event := AlarmEvent{
				ID:          uuid.NewString(),
				ResourceID:  resourceID,
				ChainName:   loaded.snapshot.Name,
				Revision:    loaded.snapshot.Revision,
				NodeID:      hit.NodeID,
				AlarmName:   hit.AlarmName,
				Severity:    hit.Severity,
				Description: hit.Description,
				Content:     content,
				Condition:   hit.Condition,
				DeviceID:    deviceID,
				TenantID:    x.tenantID,
				Values:      hit.Values,
				TriggeredAt: ts,
			}
			if err := x.sink.RecordAlarm(event); err != nil {
				// 落地失败不吞事件：仍返回给调用方，由调用方决定补救；只记错误日志。
				x.log.WithError(err).WithField("alarm_id", event.ID).Error("edge rules: sink record alarm failed")
			}
			fired = append(fired, event)
		}
	}
	if len(fired) == 0 {
		return nil
	}
	return fired
}

// dedupWindowMs 取命中节点的 retrigger_dedup_ms（节点配置按 NodeID 现查；
// 图在装载期已校验，配置损坏时按不去重处理并告警日志）。
func (x *Executor) dedupWindowMs(loaded *loadedChain, nodeID string) float64 {
	node := loaded.graph.NodeByID(nodeID)
	if node == nil {
		return 0
	}
	cfg, err := parseAlarmConfig(node.Config)
	if err != nil {
		x.log.WithFields(logrus.Fields{
			"resource_id": loaded.snapshot.ResourceID,
			"node_id":     nodeID,
		}).Warn("edge rules: alarm config damaged at eval time, dedup disabled")
		return 0
	}
	return cfg.RetriggerDedupMs
}

// retriggerKey 去重窗口键：resource|node|device（对照云端"同 node+device 窗口内不重触发"）。
func retriggerKey(resourceID, nodeID, deviceID string) string {
	return strings.Join([]string{"edge-alarm", resourceID, nodeID, deviceID}, "|")
}
