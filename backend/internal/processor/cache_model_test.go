package processor

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

// data_scripts.content 可为 NULL：转换不得 panic，按空脚本处理。
func TestCachedScriptFromModelHandlesNullContent(t *testing.T) {
	got := cachedScriptFromModel(&model.DataScript{ID: "s1", EnableFlag: "Y", ScriptType: "A"})
	if got.Content != "" || got.ID != "s1" || got.EnableFlag != "Y" || got.ScriptType != "A" {
		t.Fatalf("unexpected cached script: %+v", got)
	}

	body := "return 1"
	got = cachedScriptFromModel(&model.DataScript{ID: "s2", Content: &body})
	if got.Content != body {
		t.Fatalf("Content = %q, want %q", got.Content, body)
	}
}
