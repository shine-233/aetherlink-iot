// 文件用途：告警 SLA 计时与超时升级（TB-27）纯函数的表驱动单测。
// 核心逻辑：锚定 alarmSlaEscalationTarget（L→M→H 一档升级，H 到顶保持，N/未知透传）、
//
//	alarmSlaEscalationDue（到期判定）、alarmSlaDueAt（起算）与 validate/normalize（取值收口）。
//
// 关键注意事项：全部为无副作用纯函数，不需要数据库/Redis，可随任意定向单测运行。
// 重构建议：升级目标改为查表策略后，本文件的表驱动用例直接迁移为策略表断言即可。
package service

import (
	"testing"
	"time"
)

func TestAlarmSlaEscalationTarget(t *testing.T) {
	cases := []struct {
		name    string
		current string
		want    string
	}{
		{"low escalates to medium", "L", "M"},
		{"medium escalates to high", "M", "H"},
		{"high stays at top", "H", "H"},
		{"recovered rows untouched", "N", "N"},
		{"empty untouched", "", ""},
		{"unknown level passed through", "X", "X"},
		{"lowercase low escalates", "l", "M"},
		{"whitespace padded medium escalates", " M ", "H"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := alarmSlaEscalationTarget(tc.current); got != tc.want {
				t.Fatalf("alarmSlaEscalationTarget(%q) = %q, want %q", tc.current, got, tc.want)
			}
		})
	}
}

func TestAlarmSlaEscalationDue(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	pastDue := now.Add(-1 * time.Hour)
	futureDue := now.Add(1 * time.Hour)

	cases := []struct {
		name        string
		alarmStatus string
		slaDueAt    *time.Time
		slaBreached bool
		now         time.Time
		want        bool
	}{
		{"active past-due unbreached is due", "M", &pastDue, false, now, true},
		{"low severity past-due is due", "L", &pastDue, false, now, true},
		{"high severity past-due is due (breach marker only)", "H", &pastDue, false, now, true},
		{"future due date not yet due", "M", &futureDue, false, now, false},
		{"already breached never re-escalates", "M", &pastDue, true, now, false},
		{"no sla configured never due", "M", nil, false, now, false},
		{"recovered rows excluded", "N", &pastDue, false, now, false},
		{"unknown status excluded", "X", &pastDue, false, now, false},
		{"exactly-due boundary not breached yet", "M", &now, false, now, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := alarmSlaEscalationDue(tc.alarmStatus, tc.slaDueAt, tc.slaBreached, tc.now)
			if got != tc.want {
				t.Fatalf("alarmSlaEscalationDue(%q, %v, %v) = %v, want %v",
					tc.alarmStatus, tc.slaDueAt, tc.slaBreached, got, tc.want)
			}
		})
	}
}

func TestAlarmSlaDueAt(t *testing.T) {
	triggerAt := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	zero := int32(0)
	four := int32(4)

	if got := alarmSlaDueAt(nil, triggerAt); got != nil {
		t.Fatalf("alarmSlaDueAt(nil) = %v, want nil", got)
	}
	if got := alarmSlaDueAt(&zero, triggerAt); got != nil {
		t.Fatalf("alarmSlaDueAt(0) = %v, want nil", got)
	}
	got := alarmSlaDueAt(&four, triggerAt)
	if got == nil {
		t.Fatal("alarmSlaDueAt(4) = nil, want due at +4h")
	}
	want := triggerAt.Add(4 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("alarmSlaDueAt(4) = %v, want %v", got, want)
	}
}

func TestValidateAndNormalizeAlarmSlaHours(t *testing.T) {
	one := int32(1)
	negative := int32(-1)
	zero := int32(0)
	tooBig := int32(24*365*10 + 1)

	// validate：nil 放行；[0, 上限] 合法；负数与超上限参数错误。
	if err := validateAlarmSlaHours(nil); err != nil {
		t.Fatalf("validateAlarmSlaHours(nil) = %v, want nil", err)
	}
	if err := validateAlarmSlaHours(&zero); err != nil {
		t.Fatalf("validateAlarmSlaHours(0) = %v, want nil", err)
	}
	if err := validateAlarmSlaHours(&negative); err == nil {
		t.Fatal("validateAlarmSlaHours(-1) = nil, want param error")
	}
	if err := validateAlarmSlaHours(&tooBig); err == nil {
		t.Fatal("validateAlarmSlaHours(too big) = nil, want param error")
	}

	// normalize：nil/0 → nil（NULL=不启用）；正值保留副本。
	if got := normalizeAlarmSlaHours(nil); got != nil {
		t.Fatalf("normalizeAlarmSlaHours(nil) = %v, want nil", got)
	}
	if got := normalizeAlarmSlaHours(&zero); got != nil {
		t.Fatalf("normalizeAlarmSlaHours(0) = %v, want nil", got)
	}
	normalized := normalizeAlarmSlaHours(&one)
	if normalized == nil || *normalized != one {
		t.Fatalf("normalizeAlarmSlaHours(1) = %v, want 1", normalized)
	}
}
