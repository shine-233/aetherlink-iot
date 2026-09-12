// 文件用途：锁定设备凭证候选串展开的跨服务契约测试。
// 核心逻辑：校验两种稳定 JSON 编码（结构体序 / 字典序）都能被展开出来，且非凭证 JSON
// 不被无端展开——这是 backend 与 broker 对同一份凭证能否得出同一匹配面的前提。
// 关键注意事项：期望值与 broker 侧契约测试保持字面一致：
// mqtt-broker/plugin/aetherlink/db_test.go（TestDeviceVoucherLookupCandidatesSupportBothJSONKeyOrders）。

package utils

import (
	"reflect"
	"testing"
)

// TestDeviceVoucherLookupCandidatesSupportBothJSONKeyOrders 与 broker 侧同名契约用例字面
// 对齐：任一稳定编码顺序都必须出现在候选集中，且不得产生重复候选（重复只会放大查询
// 次数，不增加匹配面）。
func TestDeviceVoucherLookupCandidatesSupportBothJSONKeyOrders(t *testing.T) {
	exact := `{"password":"device-pass","username":"device-user"}`
	got := DeviceVoucherLookupCandidates(exact)

	want := map[string]bool{
		`{"password":"device-pass","username":"device-user"}`: false,
		`{"username":"device-user","password":"device-pass"}`: false,
	}
	for _, candidate := range got {
		if _, ok := want[candidate]; ok {
			want[candidate] = true
		}
	}
	for candidate, found := range want {
		if !found {
			t.Fatalf("DeviceVoucherLookupCandidates() missing %q; got %v", candidate, got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("DeviceVoucherLookupCandidates() returned duplicate/unexpected candidates: %v", got)
	}
	// 原始串必须排在第一位：命中精确值时不应先付出额外查询代价。
	if got[0] != exact {
		t.Fatalf("first candidate = %q, want the original voucher %q", got[0], exact)
	}
}

// TestDeviceVoucherLookupCandidatesCoversStructOrderInput 覆盖反向输入：设备按结构体序
// 上报凭证、库中存的是字典序（更新接口 map 序列化产物）时仍须展开出两种编码。
func TestDeviceVoucherLookupCandidatesCoversStructOrderInput(t *testing.T) {
	got := DeviceVoucherLookupCandidates(`{"username":"device-user","password":"device-pass"}`)
	want := []string{
		`{"username":"device-user","password":"device-pass"}`,
		`{"password":"device-pass","username":"device-user"}`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeviceVoucherLookupCandidates() = %v, want %v", got, want)
	}
}

// TestDeviceVoucherLookupCandidatesNonCredentialPayloads 锁定"不做无端展开"：非凭证形态
// 只能原样返回，否则会凭空造出匹配项、放宽凭证匹配强度。
func TestDeviceVoucherLookupCandidatesNonCredentialPayloads(t *testing.T) {
	cases := []struct {
		name    string
		voucher string
	}{
		{name: "empty string", voucher: ""},
		{name: "not json", voucher: "legacy-plaintext"},
		{name: "json without username", voucher: `{"default":"0f1c2e3a"}`},
		{name: "json array", voucher: `["username"]`},
		{name: "username empty", voucher: `{"username":"","password":"pw"}`},
		{name: "truncated json", voucher: `{"username":"u"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeviceVoucherLookupCandidates(tc.voucher)
			if len(got) != 1 || got[0] != tc.voucher {
				t.Fatalf("DeviceVoucherLookupCandidates(%q) = %v, want exactly the original string", tc.voucher, got)
			}
		})
	}
}

// TestDeviceVoucherLookupCandidatesOmitsPasswordWhenEmpty 锁定无密码凭证的候选形态：
// password 带 omitempty，候选为 {"username":"x"}，不得产出带空 password 的字典序串。
func TestDeviceVoucherLookupCandidatesOmitsPasswordWhenEmpty(t *testing.T) {
	cases := []struct {
		name    string
		voucher string
		want    []string
	}{
		{
			name:    "explicit empty password collapses to canonical",
			voucher: `{"username":"device-user","password":""}`,
			want: []string{
				`{"username":"device-user","password":""}`,
				`{"username":"device-user"}`,
			},
		},
		{
			name:    "bare username needs no extra candidate",
			voucher: `{"username":"device-user"}`,
			want:    []string{`{"username":"device-user"}`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeviceVoucherLookupCandidates(tc.voucher)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("DeviceVoucherLookupCandidates(%q) = %v, want %v", tc.voucher, got, tc.want)
			}
		})
	}
}
