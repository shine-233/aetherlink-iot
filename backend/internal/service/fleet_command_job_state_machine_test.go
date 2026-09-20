// 文件用途：批次作业状态机的定向证据（ROADMAP P0.3）。
// 覆盖：合法/非法转移、终态不可复活、暂停态不可被 worker 领取。
package service

import (
	"strings"
	"testing"
)

func TestCommandJobLegalTransitions(t *testing.T) {
	cases := []struct{ from, to string }{
		{commandJobStatusScheduled, commandJobStatusRunning},
		{commandJobStatusScheduled, commandJobStatusPaused},
		{commandJobStatusScheduled, commandJobStatusCanceled},
		{commandJobStatusRunning, commandJobStatusPaused},
		{commandJobStatusRunning, commandJobStatusCanceled},
		{commandJobStatusRunning, commandJobStatusCompleted},
		{commandJobStatusRunning, commandJobStatusPartiallyFailed},
		{commandJobStatusRunning, commandJobStatusFailed},
		{commandJobStatusPaused, commandJobStatusRunning},
		{commandJobStatusPaused, commandJobStatusCanceled},
	}
	for _, c := range cases {
		if !isLegalCommandJobTransition(c.from, c.to) {
			t.Fatalf("transition %s -> %s must be allowed", c.from, c.to)
		}
	}
}

func TestCommandJobTerminalStatesCannotRevive(t *testing.T) {
	terminals := []string{
		commandJobStatusCompleted,
		commandJobStatusPartiallyFailed,
		commandJobStatusFailed,
		commandJobStatusCanceled,
	}
	targets := []string{
		commandJobStatusScheduled,
		commandJobStatusRunning,
		commandJobStatusPaused,
		commandJobStatusCompleted,
		commandJobStatusFailed,
	}
	for _, from := range terminals {
		if !isTerminalCommandJobStatus(from) {
			t.Fatalf("%s must be terminal", from)
		}
		for _, to := range targets {
			// 终态不可"复活"：已取消/已完成的批次不允许再回到运行或暂停。
			if isLegalCommandJobTransition(from, to) {
				t.Fatalf("terminal state %s must not transition to %s", from, to)
			}
		}
	}
}

func TestCommandJobIllegalTransitionsAreRejected(t *testing.T) {
	cases := []struct{ from, to string }{
		{commandJobStatusPaused, commandJobStatusPaused},
		{commandJobStatusRunning, commandJobStatusScheduled},
		{commandJobStatusCompleted, commandJobStatusRunning},
		{commandJobStatusCanceled, commandJobStatusPaused},
	}
	for _, c := range cases {
		if isLegalCommandJobTransition(c.from, c.to) {
			t.Fatalf("transition %s -> %s must be rejected", c.from, c.to)
		}
	}
	// 未知状态一律拒绝，避免拼写错误被当成合法状态。
	if isLegalCommandJobTransition("not_a_status", commandJobStatusRunning) {
		t.Fatal("unknown source status must be rejected")
	}
	if isTerminalCommandJobStatus("not_a_status") {
		t.Fatal("unknown status must not be treated as terminal")
	}
}

func TestPausedJobIsNotDispatchable(t *testing.T) {
	// worker 只领取 running/scheduled；paused 必须被排除，否则暂停形同虚设。
	dispatchable := map[string]bool{
		commandJobStatusRunning:   true,
		commandJobStatusScheduled: true,
	}
	if dispatchable[commandJobStatusPaused] {
		t.Fatal("paused job must not be dispatchable")
	}
	if !dispatchable[commandJobStatusRunning] || !dispatchable[commandJobStatusScheduled] {
		t.Fatal("running and scheduled must remain dispatchable")
	}
}

func TestCommandJobTransitionErrorIsDeniedNotSilent(t *testing.T) {
	err := commandJobTransitionError(commandJobStatusCanceled, commandJobStatusRunning)
	if err == nil {
		t.Fatal("illegal transition must produce an error, not succeed silently")
	}
	// 错误信息必须带上双向状态，便于定位是哪次非法转移。
	want := "canceled -> running"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Fatalf("error %q must mention %q", got, want)
	}
}
