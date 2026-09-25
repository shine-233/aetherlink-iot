// 文件用途：edgemqttbroker 的边缘本地规则执行器接线（TB-21 scoped v1，可选开关默认关闭）。
// 核心逻辑：捕获的每条 PUBLISH 按 topic 分流——快照命令主题（devices/command/ 前缀）解析为
//
//	edge_sync 快照信封交给 executor.LoadSnapshot（修订号去重）；遥测主题（过滤器匹配）
//	解析为 {device_id,payload} 信封交给 ProcessTelemetry，命中的本地告警打日志并按行追加 JSONL。
//
// 关键注意事项：
//   - 本文件仅是验证栈的演示接线（无鉴权最小 broker）；生产边缘运行时应直接复用
//     internal/edgerules 包，订阅同样形状的快照/遥测主题。
//   - 遥测时间戳取 broker 接收时刻（断云缓冲重投后按"边缘实际见到"计），信封 ts 不参与去重窗口。
//   - 解析失败一律忽略并记 debug 日志：非规则快照/非遥测载荷本就可能与其它用途共用主题。
package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/edgerules"

	"github.com/sirupsen/logrus"
)

// edgeRuleConfig 边缘规则接线配置（全部来自命令行 flag，默认关闭）。
type edgeRuleConfig struct {
	Enabled        bool
	TelemetryTopic string // 遥测主题过滤器（MQTT +/# 通配），对照 edgeforward 的转发主题 {prefix}/{type}/{device_id}
	SnapshotPrefix string // 快照命令主题前缀，对照 mqtt 配置 commands.publish_topic（devices/command/）
	AlarmLogPath   string // 本地告警 JSONL 追加文件；空=只打日志
}

// edgeRuleRunner 把 broker 捕获的 PUBLISH 接到 internal/edgerules 执行器。
type edgeRuleRunner struct {
	exec     *edgerules.Executor
	cfg      edgeRuleConfig
	alarmLog *os.File
	log      *logrus.Logger
}

// newEdgeRuleRunner 构造接线器；Enabled=false 返回 nil（调用方按 nil 判开关）。
func newEdgeRuleRunner(cfg edgeRuleConfig, log *logrus.Logger) *edgeRuleRunner {
	if !cfg.Enabled {
		return nil
	}
	if log == nil {
		log = logrus.StandardLogger()
	}
	if cfg.SnapshotPrefix == "" {
		cfg.SnapshotPrefix = "devices/command/"
	}
	if cfg.TelemetryTopic == "" {
		cfg.TelemetryTopic = "aetherlink/edge/+/+"
	}
	runner := &edgeRuleRunner{
		exec: edgerules.NewExecutor(edgerules.WithSink(edgerules.LogSink{Log: log}), edgerules.WithLogger(log)),
		cfg:  cfg,
		log:  log,
	}
	if cfg.AlarmLogPath != "" {
		f, err := os.OpenFile(cfg.AlarmLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.WithError(err).Warn("edgerules: 打开告警日志文件失败，退化为仅日志输出")
		} else {
			runner.alarmLog = f
		}
	}
	log.WithFields(logrus.Fields{
		"telemetry_topic": cfg.TelemetryTopic,
		"snapshot_prefix": cfg.SnapshotPrefix,
		"alarm_log":       cfg.AlarmLogPath,
	}).Info("edgerules: 边缘本地规则执行器已启用（TB-21 scoped v1）")
	return runner
}

// close 释放告警日志文件句柄。
func (r *edgeRuleRunner) close() {
	if r != nil && r.alarmLog != nil {
		_ = r.alarmLog.Close()
	}
}

// edgeTelemetryEnvelope 边缘转发遥测信封（对照 internal/edgeforward/forwarder.go envelope 的读取子集）。
type edgeTelemetryEnvelope struct {
	DeviceID string          `json:"device_id"`
	Type     string          `json:"type"`
	TS       int64           `json:"ts"`
	Payload  json.RawMessage `json:"payload"`
}

// ingest 按主题分流捕获的 PUBLISH；解析失败静默忽略（主题可能承载其它用途载荷）。
func (r *edgeRuleRunner) ingest(topic string, payload []byte) {
	switch {
	case strings.HasPrefix(topic, r.cfg.SnapshotPrefix):
		r.ingestSnapshot(payload)
	case matchTopic(r.cfg.TelemetryTopic, topic):
		r.ingestTelemetry(topic, payload)
	}
}

// ingestSnapshot 快照命令主题：rule_chain 信封装载进执行器（修订号去重在执行器内）。
func (r *edgeRuleRunner) ingestSnapshot(payload []byte) {
	result := r.exec.LoadSnapshot(payload)
	fields := logrus.Fields{
		"status":      string(result.Status),
		"resource_id": result.ResourceID,
		"revision":    result.Revision,
		"reason":      result.Reason,
		"unsupported": strings.Join(result.UnsupportedTypes, ","),
	}
	switch result.Status {
	case edgerules.LoadStatusLoaded:
		r.log.WithFields(fields).Info("edgerules: 规则快照装载/升级")
	case edgerules.LoadStatusUnchanged:
		r.log.WithFields(fields).Debug("edgerules: 快照修订号未变，重连去重跳过")
	case edgerules.LoadStatusRegression:
		r.log.WithFields(fields).Warn("edgerules: 快照修订号回退（边缘超前于云端），已跳过")
	default:
		r.log.WithFields(fields).Warn("edgerules: 快照拒收（fail-closed，保留当前版本）")
	}
}

// ingestTelemetry 遥测主题：解析信封后对执行器求值，命中即记本地告警事件。
func (r *edgeRuleRunner) ingestTelemetry(topic string, payload []byte) {
	var env edgeTelemetryEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		r.log.WithError(err).WithField("topic", topic).Debug("edgerules: 遥测信封解析失败，忽略")
		return
	}
	if len(env.Payload) == 0 || env.Payload[0] != '{' {
		return // 非对象载荷（字符串化非 JSON 等）不参与阈值求值
	}
	var values map[string]any
	if err := json.Unmarshal(env.Payload, &values); err != nil {
		r.log.WithError(err).WithField("topic", topic).Debug("edgerules: 遥测载荷非对象，忽略")
		return
	}
	deviceID := env.DeviceID
	if deviceID == "" {
		deviceID = topic // 信封缺 device_id 时退化为主题标识，保证去重键可分
	}
	events := r.exec.ProcessTelemetry(deviceID, values, time.Now())
	for _, ev := range events {
		r.writeAlarmEvent(ev)
	}
}

// writeAlarmEvent 单条本地告警落地：JSONL 追加（可配）+ 结构化日志。
func (r *edgeRuleRunner) writeAlarmEvent(ev edgerules.AlarmEvent) {
	if r.alarmLog != nil {
		line, err := json.Marshal(ev)
		if err == nil {
			_, _ = r.alarmLog.Write(append(line, '\n'))
		}
	}
	r.log.WithFields(logrus.Fields{
		"alarm_name": ev.AlarmName,
		"severity":   ev.Severity,
		"device_id":  ev.DeviceID,
		"revision":   ev.Revision,
		"node_id":    ev.NodeID,
		"condition":  ev.Condition,
	}).Warn("edgerules: 边缘本地告警命中")
}
