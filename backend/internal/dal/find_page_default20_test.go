package dal

import "testing"

// TestNormalizeLegacyListPage 覆盖 page/pageSize 的边界归一（与合并前四处内联实现一致）。
func TestNormalizeLegacyListPage(t *testing.T) {
	cases := []struct{ page, size, wantPage, wantSize int }{
		{0, 0, 1, 20},
		{-3, -1, 1, 20},
		{1, 1, 1, 1},
		{2, 200, 2, 200},
		{5, 201, 5, 20},
		{3, 50, 3, 50},
	}
	for _, tc := range cases {
		p, s := normalizeLegacyListPage(tc.page, tc.size)
		if p != tc.wantPage || s != tc.wantSize {
			t.Errorf("normalize(%d,%d)=(%d,%d) want (%d,%d)", tc.page, tc.size, p, s, tc.wantPage, tc.wantSize)
		}
	}
}
