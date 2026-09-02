package hierarchy

import "testing"

// TestDescendantsAndScopeDown 覆盖自上而下语义（总部/父级可见子树，子级仅自身）。
func TestDescendantsAndScopeDown(t *testing.T) {
	parent := map[string]string{
		"t-root":   "",
		"t-hq":     "t-root",
		"t-east":   "t-hq",
		"t-east-s": "t-east",
		"t-west":   "t-hq",
		"t-other":  "t-root",
	}
	// 子孙集合（不含自身，BFS 扁平）
	desc, err := Descendants("t-hq", parent)
	if err != nil {
		t.Fatalf("descendants: %v", err)
	}
	if len(desc) != 3 {
		t.Fatalf("t-hq descendants len=%d want 3 (%v)", len(desc), desc)
	}
	seen := map[string]bool{}
	for _, d := range desc {
		seen[d] = true
	}
	if !seen["t-east"] || !seen["t-west"] || !seen["t-east-s"] {
		t.Fatalf("t-hq descendants missing east/west/east-s: %v", desc)
	}
	// 叶节点无子孙
	if leaf, _ := Descendants("t-east-s", parent); len(leaf) != 0 {
		t.Fatalf("leaf must have no descendants: %v", leaf)
	}
	// ScopeDown：self 在首位 + 全部子孙
	scope, err := ScopeDown("t-hq", parent)
	if err != nil {
		t.Fatalf("scopeDown: %v", err)
	}
	if len(scope) != 4 || scope[0] != "t-hq" {
		t.Fatalf("scopeDown(hq) len=%d head=%q want 4/hq", len(scope), scope[0])
	}
	// 未知节点：scope 仅自身
	if sc, _ := ScopeDown("t-ghost", parent); len(sc) != 1 || sc[0] != "t-ghost" {
		t.Fatalf("scopeDown(ghost)=%v want [t-ghost]", sc)
	}
	// 环防御：Descendants 不应死循环
	cyclic := map[string]string{"a": "b", "b": "a"}
	cyc, err := Descendants("a", cyclic)
	if err != nil {
		t.Fatalf("cyclic descendants returned error: %v", err)
	}
	if len(cyc) > MaxAncestors {
		t.Fatalf("cyclic traversal runaway: %v", cyc)
	}
	_ = cyc
}
