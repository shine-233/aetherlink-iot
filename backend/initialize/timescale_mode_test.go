// 文件用途：TimescaleDB 显式开关（ROADMAP C1 收尾）的纯逻辑单测。
// 核心逻辑：归一化取值（空→auto、未知→错误）、决策表（off 跳过、on 需扩展、auto 沿用现状）。
package initialize

import "testing"

func TestNormalizeTimescaleMode(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		bad  bool
	}{
		{"", "auto", false},
		{"auto", "auto", false},
		{"AUTO", "auto", false},
		{"on", "on", false},
		{"OFF", "off", false},
		{"  off  ", "off", false},
		{"banana", "", true},
		{"on;DROP", "", true},
	}
	for _, c := range cases {
		got, err := normalizeTimescaleMode(c.raw)
		if c.bad {
			if err == nil {
				t.Errorf("normalizeTimescaleMode(%q) 应报错", c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeTimescaleMode(%q) 意外报错: %v", c.raw, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeTimescaleMode(%q)=%q, want %q", c.raw, got, c.want)
		}
	}
}

func TestDecideTimescaleMigration(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		installed  bool
		wantRun    bool
		wantFatal  bool
	}{
		{"auto+未装扩展: 由57自行检测, 执行", "auto", false, true, false},
		{"auto+已装扩展: 执行转换", "auto", true, true, false},
		{"off+已装扩展: 显式关闭, 跳过", "off", true, false, false},
		{"off+未装扩展: 跳过", "off", false, false, false},
		{"on+已装扩展: 执行转换", "on", true, true, false},
		{"on+未装扩展: fail-fast 报可操作指引", "on", false, false, true},
	}
	for _, c := range cases {
		run, failMsg := decideTimescaleMigration(c.mode, c.installed)
		if run != c.wantRun {
			t.Errorf("%s: run=%v, want %v", c.name, run, c.wantRun)
		}
		if (failMsg != "") != c.wantFatal {
			t.Errorf("%s: failMsg=%q, wantFatal=%v", c.name, failMsg, c.wantFatal)
		}
		if c.wantFatal && failMsg == "" {
			t.Errorf("%s: on+缺扩展必须给可操作指引", c.name)
		}
	}
}
