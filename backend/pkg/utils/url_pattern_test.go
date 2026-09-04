// 文件用途：MatchURLPattern 锚定模式匹配的表驱动测试——重点防"子串误命中"回归。
package utils

import "testing"

func TestMatchURLPattern(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		pattern string
		want    bool
	}{
		// 参数段命中
		{"param route hit", "api/v1/device/123", "api/v1/device/:id", true},
		{"param route uuid", "api/v1/device/6f04df62-6704", "api/v1/device/:id", true},
		{"multi param", "api/v1/device/1/msg/2", "api/v1/device/:id/msg/:msgId", true},
		// 字面段不误伤
		{"sibling prefix no hit", "api/v1/devices", "api/v1/device/:id", false},
		{"trailing junk no hit", "api/v1/devicesXYZ", "api/v1/devices", false},
		{"longer path no hit", "api/v1/device/123/extra", "api/v1/device/:id", false},
		{"different tail no hit", "api/v1/device/123", "api/v1/device/:id/msg", false},
		// 正则元字符按字面
		{"dot literal", "api/v1/v1.2/x", "api/v1/v1.2/:id", true},
		{"dot not wildcard", "api/v1/v1x2/x", "api/v1/v1.2/:id", false},
		// 空模式与空路径
		{"empty pattern", "api/v1/x", "", false},
		{"empty url vs literal", "", "api/v1/x", false},
		// 尾斜杠宽容（中间件 TrimLeft 口径下两侧均无尾斜杠，但登记方可能带）
		{"pattern trailing slash", "api/v1/devices", "api/v1/devices/", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchURLPattern(tc.url, tc.pattern); got != tc.want {
				t.Fatalf("MatchURLPattern(%q, %q) = %v, want %v", tc.url, tc.pattern, got, tc.want)
			}
		})
	}
}
