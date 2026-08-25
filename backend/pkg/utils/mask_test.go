// 文件用途：MaskVoucher 展示掩码契约单测。
// 核心逻辑：锁定 Phase 2a 统一掩码格式的三个分支——正常长凭证（>12）取前 10 字符加省略号、
// 短凭证（≤12，含边界值 12 与空串）整体替换为固定占位 "******"。
// 关键注意事项：前端以「… 结尾」识别掩码形态，格式变更需同步 join.vue 降级判定与本测试。
package utils

import "testing"

func TestMaskVoucher(t *testing.T) {
	cases := []struct {
		name    string
		voucher string
		want    string
	}{
		{
			name:    "正常长度凭证保留前10字符并追加省略号",
			voucher: `{"username":"mqtt_device_001","password":"p"}`,
			want:    `{"username` + "…",
		},
		{
			name:    "恰好13字符走前缀掩码",
			voucher: "abcdefghijXYZ",
			want:    "abcdefghij…",
		},
		{
			name:    "12字符边界整体替换占位",
			voucher: "abcdefghijkl",
			want:    "******",
		},
		{
			name:    "短凭证整体替换占位",
			voucher: "short",
			want:    "******",
		},
		{
			name:    "空串整体替换占位",
			voucher: "",
			want:    "******",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskVoucher(tc.voucher); got != tc.want {
				t.Fatalf("MaskVoucher(%q) = %q, want %q", tc.voucher, got, tc.want)
			}
		})
	}
}

func TestMaskVoucherNeverReturnsPlaintext(t *testing.T) {
	// 契约护栏：任何输入下掩码结果都不得包含完整明文（防未来把截断阈值改穿）。
	plain := `{"username":"abc","password":"top-secret-value"}`
	masked := MaskVoucher(plain)
	if masked == plain {
		t.Fatal("masked output must never equal the plaintext voucher")
	}
	if len(masked) >= len(plain) {
		t.Fatalf("masked output %q must be shorter than plaintext", masked)
	}
}
