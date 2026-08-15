// 文件用途：实现 fleet 级 device twin drift 可查询索引的纯聚合逻辑。
// 核心逻辑：把每台设备已算好的 DeviceTwinSummary(convergence 分类)聚合成一个
//
//	按 severity 排序、带分类计数的可查询索引,回答"整个 fleet 里哪些设备在漂移"。
//
// 关键注意事项：本文件是纯函数聚合,不做数据库枚举、不连 broker、不改协议契约。
//
//	设备清单的实际枚举(按租户/项目遍历并逐台取 twin)属于需运行时(PG+设备上报)
//	验证的部分,不在此实现;这里只保证"给定一组 summary,索引结论正确且可离线验证"。
//
// 重构建议：接入持久化枚举时,让枚举层复用本聚合函数,保持"drift 分类单一来源"。
package service

import (
	"sort"

	model "aetherlink-iot/backend/internal/model"
)

// DeviceTwinDriftInput 是单台设备喂给 fleet 索引的最小输入。
type DeviceTwinDriftInput struct {
	DeviceID   string
	DeviceName string
	Summary    model.DeviceTwinSummary
}

// twinConvergenceSeverity 把 convergence 状态映射为排序权重,越大越需要关注。
// needs_review(有 delta)最高,其次过期,再次等待上报,no_desired 与 ready 最低。
func twinConvergenceSeverity(status string) int {
	switch status {
	case "needs_review":
		return 40
	case "expired_desired":
		return 30
	case "waiting_reported":
		return 20
	case "no_desired":
		return 10
	case "ready":
		return 0
	default:
		return 5
	}
}

// BuildDeviceTwinDriftIndex 把一组单台 twin summary 聚合成 fleet 级 drift 索引。
// 它是纯函数:不查询数据库、不连接 broker、不下发消息,只对传入的 summary 做分类、计数与排序。
// Entries 按 severity 降序;severity 相同的按 DeviceID 升序,保证输出稳定可测。
func BuildDeviceTwinDriftIndex(inputs []DeviceTwinDriftInput) model.DeviceTwinDriftIndex {
	index := model.DeviceTwinDriftIndex{
		Entries:          make([]model.DeviceTwinDriftEntry, 0, len(inputs)),
		TotalDevices:     len(inputs),
		EvidenceBoundary: "platform_visible_evidence_only",
	}

	for _, in := range inputs {
		severity := twinConvergenceSeverity(in.Summary.ConvergenceStatus)
		index.Entries = append(index.Entries, model.DeviceTwinDriftEntry{
			DeviceID:          in.DeviceID,
			DeviceName:        in.DeviceName,
			ConvergenceStatus: in.Summary.ConvergenceStatus,
			NextAction:        in.Summary.NextAction,
			DeltaCount:        in.Summary.DeltaCount,
			UnavailableCount:  in.Summary.UnavailableCount,
			StaleDesiredCount: in.Summary.StaleDesiredCount,
			DesiredCount:      in.Summary.DesiredCount,
			Severity:          severity,
			Summary:           in.Summary,
		})

		switch in.Summary.ConvergenceStatus {
		case "needs_review":
			index.DriftDevices++
		case "expired_desired":
			index.ExpiredDevices++
		case "waiting_reported":
			index.WaitingDevices++
		case "no_desired":
			index.NoDesiredDevices++
		case "ready":
			index.ReadyDevices++
		}
	}

	sort.SliceStable(index.Entries, func(i, j int) bool {
		if index.Entries[i].Severity != index.Entries[j].Severity {
			return index.Entries[i].Severity > index.Entries[j].Severity
		}
		return index.Entries[i].DeviceID < index.Entries[j].DeviceID
	})

	return index
}
