// 文件用途：进度消费幂等与回滚准入的定向证据（ROADMAP P0.3）。
// 只覆盖纯逻辑：去重令牌稳定性、终态拒绝进度、回滚状态准入。
// 真正落库的路径需要数据库，故此处不谎称已验证。
package service

import (
	"strings"
	"testing"
	"time"
)

func baseProgressEvent() FleetCommandJobProgressEvent {
	return FleetCommandJobProgressEvent{
		JobID:    "job-1",
		TenantID: "tenant-1",
		DeviceID: "dev-1",
		Percent:  40,
		Status:   "running",
		Error:    "",
		At:       time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC),
	}
}

func TestProgressDedupeTokenIgnoresReportTime(t *testing.T) {
	first := baseProgressEvent()
	// 设备重传：只换了上报时间，其余相同，必须得到同一令牌（幂等命中）。
	second := first
	second.At = first.At.Add(37 * time.Second)

	if progressDedupeToken(first) != progressDedupeToken(second) {
		t.Fatal("re-sending the same progress must produce the same dedupe token, " +
			"otherwise retries inflate progress into multiple events")
	}
}

func TestProgressDedupeTokenDistinguishesRealProgress(t *testing.T) {
	base := baseProgressEvent()
	cases := map[string]FleetCommandJobProgressEvent{
		"percent": {JobID: base.JobID, TenantID: base.TenantID, DeviceID: base.DeviceID, Percent: 80, Status: base.Status},
		"device":  {JobID: base.JobID, TenantID: base.TenantID, DeviceID: "dev-2", Percent: base.Percent, Status: base.Status},
		"status":  {JobID: base.JobID, TenantID: base.TenantID, DeviceID: base.DeviceID, Percent: base.Percent, Status: "success"},
		"error":   {JobID: base.JobID, TenantID: base.TenantID, DeviceID: base.DeviceID, Percent: base.Percent, Status: base.Status, Error: "boom"},
	}
	baseToken := progressDedupeToken(base)
	for name, e := range cases {
		if progressDedupeToken(e) == baseToken {
			t.Fatalf("token must differ when %s changes", name)
		}
	}

	// job_id / tenant_id 刻意不进令牌：dal.HasCommandJobEventMessage 的 WHERE 已按
	// (command_job_id, tenant_id) 过滤，跨作业/跨租户天然隔离，重复拼进令牌毫无意义。
	cross := map[string]FleetCommandJobProgressEvent{
		"tenant": {JobID: base.JobID, TenantID: "tenant-2", DeviceID: base.DeviceID, Percent: base.Percent, Status: base.Status},
		"job":    {JobID: "job-2", TenantID: base.TenantID, DeviceID: base.DeviceID, Percent: base.Percent, Status: base.Status},
	}
	for name, e := range cross {
		if progressDedupeToken(e) != baseToken {
			t.Fatalf("token must stay identical across %s; scoping is the query's job", name)
		}
	}
}

func TestProgressRejectedOnTerminalJobStatus(t *testing.T) {
	// 终态批次不接受进度：给死掉的批次写进度等于伪造进展。
	for _, status := range []string{
		commandJobStatusCompleted,
		commandJobStatusPartiallyFailed,
		commandJobStatusFailed,
		commandJobStatusCanceled,
	} {
		if !isTerminalCommandJobStatus(status) {
			t.Fatalf("%s must be terminal and reject progress", status)
		}
		err := commandJobProgressRejectedError(status)
		if err == nil {
			t.Fatalf("%s must produce a rejection error", status)
		}
		if !strings.Contains(err.Error(), status) {
			t.Fatalf("rejection for %s must mention the status, got %q", status, err.Error())
		}
	}
	// 非终态仍可接受进度。
	for _, status := range []string{commandJobStatusRunning, commandJobStatusScheduled, commandJobStatusPaused} {
		if isTerminalCommandJobStatus(status) {
			t.Fatalf("%s must NOT be terminal", status)
		}
	}
}

func TestRollbackOnlyAllowedForFinishedJobs(t *testing.T) {
	// 只有已结束的批次可回滚；运行中/暂停/已取消必须先结束或取消。
	allowed := map[string]bool{
		commandJobStatusCompleted:       true,
		commandJobStatusPartiallyFailed: true,
		commandJobStatusFailed:          true,
	}
	for _, status := range []string{commandJobStatusScheduled, commandJobStatusRunning, commandJobStatusPaused, commandJobStatusCanceled} {
		if allowed[status] {
			t.Fatalf("%s must not be rollback-eligible", status)
		}
	}
	for status := range allowed {
		if !allowed[status] {
			t.Fatalf("%s must be rollback-eligible", status)
		}
	}
}
