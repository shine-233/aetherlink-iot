package service

import (
	"strings"
	"testing"
)

func TestNormalizeFleetCommandJobRowsSearchPreservesUTF8Boundaries(t *testing.T) {
	input := strings.Repeat("上线", 40)
	got := normalizeFleetCommandJobRowsSearch(input)
	if !strings.Contains(got, "上线") {
		t.Fatalf("normalized search lost UTF-8 content: %q", got)
	}
	if got != strings.Repeat("上线", 32) {
		t.Fatalf("normalized search should retain 64 runes, got %d runes", len([]rune(got)))
	}
}

func TestNormalizeFleetCommandJobListSearchPreservesUTF8Boundaries(t *testing.T) {
	input := strings.Repeat("失败", 40)
	got := normalizeFleetCommandJobListSearch(input)
	if got != strings.Repeat("失败", 32) {
		t.Fatalf("normalized list search should retain 64 runes, got %d runes", len([]rune(got)))
	}
}
