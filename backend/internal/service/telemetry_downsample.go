// 文件用途：P2.3 遥测降采样作业——把旧原始数据汇总进冷层（telemetry_rollups）。
// 核心逻辑：config 门控（默认关闭，不改变既有部署行为）→ 只在直连数据库模式下工作 →
// 按 (设备,键) 对批量汇总 cutoff 之前的原始行。
//
// 关键注意事项：
//   - **默认关闭**：降采样改变查询路径（冷窗口回落冷层），必须在配置里显式开启。
//   - **不删原始数据**：冷层是加法。原始行的清理仍归保留策略（CleanSystemDataByCron）管——
//     两件事解耦，汇总错误可重跑修正，不会先删了再说。
//   - 外部 TSDB 模式（grpc.tptodb_type）下原始数据不在本库，作业显式跳过并记日志，
//     不伪造"已降采样"。
//   - 单个目标失败不中断整批：失败的 (设备,键) 记录下来，下一轮会再试。
package service

import (
	"time"

	"aetherlink-iot/backend/internal/dal"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// 降采样配置键。
const (
	telemetryDownsampleEnabledKey    = "telemetry.downsample.enabled"
	telemetryDownsampleOlderThanDays = "telemetry.downsample.older_than_days"
	telemetryDownsampleMaxKeysPerRun = "telemetry.downsample.max_keys_per_run"
)

// TelemetryDownsampleOutcome 一轮降采样作业的结果（供日志与验证引用）。
type TelemetryDownsampleOutcome struct {
	Enabled      bool     `json:"enabled"`
	Mode         string   `json:"mode"` // direct-db / external-tsdb
	Targets      int      `json:"targets"`
	Buckets      int64    `json:"buckets"`
	Failed       []string `json:"failed,omitempty"`
	CutoffUnixMs int64    `json:"cutoff_unix_ms"`
}

// RunTelemetryDownsampleByCron 执行一轮降采样。默认关闭时为 no-op。
func (*DataPolicy) RunTelemetryDownsampleByCron() TelemetryDownsampleOutcome {
	outcome := TelemetryDownsampleOutcome{Mode: "direct-db"}
	if !viper.GetBool(telemetryDownsampleEnabledKey) {
		outcome.Enabled = false
		return outcome
	}
	outcome.Enabled = true
	if !dal.TelemetryDownsamplingActive() {
		outcome.Mode = "external-tsdb"
		logrus.Info("[TelemetryDownsample] skip: telemetry raw data is managed by an external TSDB")
		return outcome
	}

	days := viper.GetInt64(telemetryDownsampleOlderThanDays)
	if days <= 0 {
		days = 90
	}
	maxKeys := viper.GetInt(telemetryDownsampleMaxKeysPerRun)
	cutoff := time.Now().UnixMilli() - days*24*3600*1000
	outcome.CutoffUnixMs = cutoff

	targets, err := dal.ListTelemetryDownsampleTargets(cutoff, maxKeys)
	if err != nil {
		logrus.Errorf("[TelemetryDownsample] list targets failed: %v", err)
		outcome.Failed = append(outcome.Failed, "list_targets")
		return outcome
	}
	outcome.Targets = len(targets)
	for _, target := range targets {
		buckets, uerr := dal.UpsertTelemetryRollups(target.DeviceID, target.Key, cutoff)
		if uerr != nil {
			logrus.Errorf("[TelemetryDownsample] upsert rollups for (%s, %s) failed: %v",
				target.DeviceID, target.Key, uerr)
			outcome.Failed = append(outcome.Failed, target.DeviceID+"/"+target.Key)
			continue
		}
		outcome.Buckets += buckets
	}
	logrus.WithField("targets", outcome.Targets).
		WithField("buckets", outcome.Buckets).
		WithField("failed", len(outcome.Failed)).
		Info("[TelemetryDownsample] rollup pass finished")
	return outcome
}
