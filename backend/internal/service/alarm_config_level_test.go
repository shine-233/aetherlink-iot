// alarm_config_level_test.go 锁定 alarm_config.alarm_level 的枚举边界。
//
// 该列是 varchar(10) 且此前只有 required 校验，任意字符串都能落库；前端
// alarmSeverityLabel 匹配不到选项时会原样回显，等于把脏数据带到界面上。
// 这里锁定三件事：只接受 H/M/L、trim 后再判定、其余输入一律拒绝。
package service

import (
	"testing"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeAlarmConfigLevelAcceptsOnlyHighMediumLow(t *testing.T) {
	for _, level := range []string{"H", "M", "L"} {
		normalized, err := normalizeAlarmConfigLevel(level)
		if err != nil {
			t.Fatalf("normalizeAlarmConfigLevel(%q) returned error: %v", level, err)
		}
		if normalized != level {
			t.Fatalf("normalizeAlarmConfigLevel(%q) = %q, want %q", level, normalized, level)
		}
	}
}

func TestNormalizeAlarmConfigLevelTrimsSurroundingWhitespace(t *testing.T) {
	normalized, err := normalizeAlarmConfigLevel("  H  ")
	if err != nil {
		t.Fatalf("padded level returned error: %v", err)
	}
	if normalized != "H" {
		t.Fatalf("padded level = %q, want %q", normalized, "H")
	}
}

func TestNormalizeAlarmConfigLevelRejectsUnsupportedValues(t *testing.T) {
	// "N" 表示恢复正常，属于 alarm_history 的状态口径，不是可配置的规则级别。
	// "high" 是前端测试里出现过的写法，任何 UI 选项都匹配不上它。
	for _, level := range []string{"", "N", "high", "HIGH", "h", "critical", "1", "H;DROP"} {
		normalized, err := normalizeAlarmConfigLevel(level)
		if err == nil {
			t.Fatalf("normalizeAlarmConfigLevel(%q) accepted an unsupported level", level)
		}
		if normalized != "" {
			t.Fatalf("rejected level %q still returned %q, want empty", level, normalized)
		}
		// 断言错误码而不是文案：errcode.Error 的 Error() 只渲染 code，
		// 与包内既有断言口径（assert.Equal(errcode.CodeParamError, appErr.Code)）保持一致。
		appErr, ok := err.(*errcode.Error)
		if !assert.True(t, ok, "level %q: expected *errcode.Error, got %T", level, err) {
			continue
		}
		assert.Equal(t, errcode.CodeParamError, appErr.Code, "level %q", level)
	}
}
