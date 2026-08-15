package service

import (
	"sort"
	"strings"

	"aetherlink-iot/backend/internal/model"
)

func attachFleetCommandJobPreviewGuidance(result *model.FleetCommandJobPreviewResult) *model.FleetCommandJobPreviewResult {
	if result == nil {
		return nil
	}

	result.PathCounts = buildFleetCommandJobPreviewPathCounts(result.Rows)
	result.Blockers = buildFleetCommandJobPreviewBlockers(result.Rows)
	result.NextAction = buildFleetCommandJobPreviewNextAction(result)
	result.Governance = buildFleetCommandJobPreviewGovernanceSummary(result)
	return result
}

func buildFleetCommandJobPreviewPathCounts(rows []model.FleetCommandJobPreviewRow) model.FleetCommandJobPreviewPathCounts {
	counts := model.FleetCommandJobPreviewPathCounts{}
	for _, row := range rows {
		switch row.RecommendedPath {
		case "immediate":
			counts.Immediate++
		case "jobs":
			counts.Jobs++
		default:
			counts.Blocked++
		}
		if row.TelemetryCurrentCount > 0 {
			counts.Telemetry++
		}
	}
	return counts
}

func buildFleetCommandJobPreviewBlockers(rows []model.FleetCommandJobPreviewRow) []model.FleetCommandJobPreviewBlocker {
	type blockerKey struct {
		reason string
		advice string
	}

	grouped := map[blockerKey]int{}
	for _, row := range rows {
		if row.Eligible && row.RecommendedPath != "blocked" {
			continue
		}
		reason := strings.TrimSpace(row.Reason)
		if reason == "" {
			reason = strings.TrimSpace(row.Status)
		}
		if reason == "" {
			reason = "预览行被阻断。"
		}
		grouped[blockerKey{
			reason: reason,
			advice: strings.TrimSpace(row.Advice),
		}]++
	}

	blockers := make([]model.FleetCommandJobPreviewBlocker, 0, len(grouped))
	for key, count := range grouped {
		blockers = append(blockers, model.FleetCommandJobPreviewBlocker{
			Reason: key.reason,
			Advice: key.advice,
			Count:  count,
		})
	}
	sort.Slice(blockers, func(i, j int) bool {
		if blockers[i].Count != blockers[j].Count {
			return blockers[i].Count > blockers[j].Count
		}
		return blockers[i].Reason < blockers[j].Reason
	})
	if len(blockers) > 5 {
		return blockers[:5]
	}
	return blockers
}

func buildFleetCommandJobPreviewNextAction(result *model.FleetCommandJobPreviewResult) string {
	if result.EligibleCount <= 0 {
		return "提交批量命令前，请先处理预览中显示的阻断项。"
	}
	if result.RequestedCount > len(result.Rows) {
		return "当前只是预览子集证据；请缩小筛选范围或提高上限，直到预览覆盖已确认的目标范围。"
	}
	if result.BlockedCount > 0 {
		return "请先核对被阻断设备的原因，再只提交可执行设备。"
	}
	return "预览已覆盖确认范围；可以提交可执行设备，并持续观察任务进度。"
}
