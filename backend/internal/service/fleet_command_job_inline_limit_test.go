// 文件用途：覆盖 Command Job 详情/支持包内联行集的截断标记行为。
// 核心逻辑：状态计数总和作为真实行数基准，验证 rows_truncated 与提示信息的正确性。
// 关键注意事项：不依赖真实数据库；支持包测试使用无 MessageID 的行以跳过日志联查。
package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestCommandJobResultMarksRowsTruncatedFromStatusCounts(t *testing.T) {
	job := &model.CommandJob{
		ID:             "job-truncated",
		JobType:        "command",
		ScopeType:      fleetCommandScopeSelectedDevices,
		Status:         commandJobStatusCompleted,
		RequestedCount: 3,
	}
	details := []*model.CommandJobDetail{
		{ID: "d-1", CommandJobID: job.ID, DeviceID: "dev-1", Status: commandJobDetailStatusSubmitted},
	}

	result := commandJobResultFromPersistence(job, details, map[string]int{
		commandJobDetailStatusSubmitted: 3,
	}, nil)

	if !result.RowsTruncated {
		t.Fatalf("rows_truncated = false, want true when inline rows are fewer than status counts")
	}
	if result.RowsTotal != 3 {
		t.Fatalf("rows_total = %d, want 3 from status counts", result.RowsTotal)
	}
	foundTruncatedWarning := false
	for _, warning := range result.Warnings {
		if warning == "逐设备行数超过单次内联上限，当前仅返回部分行；完整结果请使用分页 rows 接口查询。" {
			foundTruncatedWarning = true
		}
	}
	if !foundTruncatedWarning {
		t.Fatalf("expected truncated warning in %#v", result.Warnings)
	}
}

func TestCommandJobResultDoesNotMarkTruncatedWhenCountsMatch(t *testing.T) {
	job := &model.CommandJob{
		ID:             "job-complete",
		JobType:        "command",
		ScopeType:      fleetCommandScopeSelectedDevices,
		Status:         commandJobStatusCompleted,
		RequestedCount: 1,
	}
	details := []*model.CommandJobDetail{
		{ID: "d-1", CommandJobID: job.ID, DeviceID: "dev-1", Status: commandJobDetailStatusSubmitted},
	}

	result := commandJobResultFromPersistence(job, details, map[string]int{
		commandJobDetailStatusSubmitted: 1,
	}, nil)

	if result.RowsTruncated {
		t.Fatalf("rows_truncated = true, want false when inline rows match status counts")
	}
	if result.RowsTotal != 1 {
		t.Fatalf("rows_total = %d, want 1", result.RowsTotal)
	}
	for _, warning := range result.Warnings {
		if warning == "逐设备行数超过单次内联上限，当前仅返回部分行；完整结果请使用分页 rows 接口查询。" {
			t.Fatalf("truncated warning must be absent when rows are complete, got %#v", result.Warnings)
		}
	}
}

func buildInlineSupportDetails(jobID string, count int) []*model.CommandJobDetail {
	rows := make([]*model.CommandJobDetail, 0, count)
	for i := 0; i < count; i++ {
		rows = append(rows, &model.CommandJobDetail{
			ID:           "detail",
			CommandJobID: jobID,
			DeviceID:     "device",
			Status:       commandJobDetailStatusFailed,
			CanRetry:     true,
		})
	}
	return rows
}

func TestCommandJobSupportBundleMarksRowsTruncatedAtInlineCap(t *testing.T) {
	job := &model.CommandJob{
		ID:        "job-support-cap",
		JobType:   "command",
		ScopeType: fleetCommandScopeSelectedDevices,
		Status:    commandJobStatusPartiallyFailed,
	}

	bundle := commandJobSupportBundleFromPersistence(
		job,
		buildInlineSupportDetails(job.ID, commandJobInlineRowLimit),
		map[string]int{commandJobDetailStatusFailed: commandJobInlineRowLimit + 10},
		0, 0, 0, 0, 0,
		nil,
	)
	if !bundle.RowsTruncated {
		t.Fatalf("rows_truncated = false, want true when support rows reach the inline cap")
	}
	foundHint := false
	for _, action := range bundle.NextActions {
		if action == "支持包证据行数已达到单次内联上限，可能缺少部分设备行；请结合分页 rows 接口复核完整结果。" {
			foundHint = true
		}
	}
	if !foundHint {
		t.Fatalf("expected truncation hint in next actions %#v", bundle.NextActions)
	}

	belowCap := commandJobSupportBundleFromPersistence(
		job,
		buildInlineSupportDetails(job.ID, commandJobInlineRowLimit-1),
		map[string]int{commandJobDetailStatusFailed: commandJobInlineRowLimit - 1},
		0, 0, 0, 0, 0,
		nil,
	)
	if belowCap.RowsTruncated {
		t.Fatalf("rows_truncated = true, want false when support rows stay below the cap")
	}
}
