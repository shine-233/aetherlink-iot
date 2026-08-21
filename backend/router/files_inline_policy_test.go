// 文件用途：钉死 /files 响应的内联内容类型白名单策略。
// 核心逻辑：验证 isInlineSafeFileContentType 对可执行类型（html/xhtml/svg）的排除、
// 参数剥离、大小写 fail-closed 和常见媒体类型的放行边界。
// 关键注意事项：该白名单是 /files 同源存储型 XSS 防线的一部分，改动需同步安全审查。
// 重构建议：后续可补充 handler 级集成测试覆盖完整 Content-Disposition 输出。
package router

import "testing"

func TestIsInlineSafeFileContentType(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		wantInline  bool
	}{
		{name: "html is attachment", contentType: "text/html; charset=utf-8", wantInline: false},
		{name: "xhtml is attachment", contentType: "application/xhtml+xml", wantInline: false},
		{name: "svg is attachment", contentType: "image/svg+xml", wantInline: false},
		{name: "uppercase svg fails closed", contentType: "IMAGE/SVG+XML", wantInline: false},
		{name: "png is inline", contentType: "image/png", wantInline: true},
		{name: "jpeg is inline", contentType: "image/jpeg", wantInline: true},
		{name: "plain text with params is inline", contentType: "text/plain; charset=utf-8", wantInline: true},
		{name: "pdf is inline", contentType: "application/pdf", wantInline: true},
		{name: "audio is inline", contentType: "audio/mpeg", wantInline: true},
		{name: "video is inline", contentType: "video/mp4", wantInline: true},
		{name: "octet stream is attachment", contentType: "application/octet-stream", wantInline: false},
		{name: "csv is attachment by default", contentType: "text/csv", wantInline: false},
		{name: "empty fails closed", contentType: "", wantInline: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isInlineSafeFileContentType(tc.contentType); got != tc.wantInline {
				t.Fatalf("isInlineSafeFileContentType(%q) = %v, want %v", tc.contentType, got, tc.wantInline)
			}
		})
	}
}
