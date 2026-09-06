package service

import (
	"strings"
	"testing"
)

// PHASE-D-D2 BEGIN 模板变量引擎表驱动单测

func TestRenderNotificationTemplateD2(t *testing.T) {
	cases := []struct {
		name string
		tpl  string
		vars map[string]interface{}
		want string
	}{
		{
			name: "基础替换",
			tpl:  "【{{subject}}】{{content}}",
			vars: map[string]interface{}{"subject": "高温告警", "content": "温度 45 度"},
			want: "【高温告警】温度 45 度",
		},
		{
			name: "未知占位符原样保留",
			tpl:  "{{subject}}/{{unknown_key}}",
			vars: map[string]interface{}{"subject": "s"},
			want: "s/{{unknown_key}}",
		},
		{
			name: "非字符串值字符串化",
			tpl:  "count={{count}} ok={{ok}}",
			vars: map[string]interface{}{"count": 3, "ok": true},
			want: "count=3 ok=true",
		},
		{
			name: "占位符两侧空白被容忍",
			tpl:  "{{  subject  }}",
			vars: map[string]interface{}{"subject": "s"},
			want: "s",
		},
		{
			name: "值中的占位符不递归展开",
			tpl:  "{{a}}",
			vars: map[string]interface{}{"a": "{{b}}", "b": "X"},
			want: "{{b}}",
		},
		{
			name: "空模板返回空",
			tpl:  "",
			vars: map[string]interface{}{"a": "1"},
			want: "",
		},
		{
			name: "nil 值按未知处理",
			tpl:  "{{nilkey}}",
			vars: map[string]interface{}{"nilkey": nil},
			want: "{{nilkey}}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderNotificationTemplate(tc.tpl, tc.vars)
			if got != tc.want {
				t.Fatalf("renderNotificationTemplate(%q) = %q, want %q", tc.tpl, got, tc.want)
			}
		})
	}
}

func TestBuildIMTextContentD2(t *testing.T) {
	t.Run("subject 与 content 注入渲染", func(t *testing.T) {
		vars := &executeNotificationTemplateVars{
			alertData: map[string]interface{}{"subject": "离线告警", "content": "设备 dev-1 离线", "device_name": "dev-1"},
			subject:   "离线告警",
			content:   "设备 dev-1 离线",
		}
		got := buildIMTextContent(vars)
		if !strings.Contains(got, "【离线告警】") || !strings.Contains(got, "设备 dev-1 离线") {
			t.Fatalf("buildIMTextContent = %q, 缺少主题或正文", got)
		}
	})
	t.Run("nil 安全", func(t *testing.T) {
		got := buildIMTextContent(nil)
		if strings.Contains(got, "{{") {
			t.Fatalf("nil 输入不应残留占位符: %q", got)
		}
	})
}

// PHASE-D-D2 END
