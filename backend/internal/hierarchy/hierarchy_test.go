// 文件用途：层级语义核心单测——祖先链推导、成环/自引用/悬空父拒绝、Scope 语义、
// 全树校验、超长 id 拒绝。
package hierarchy

import (
	"fmt"
	"reflect"
	"testing"
)

func sampleNodes() []Node {
	return []Node{
		{ID: "root", Parent: ""},
		{ID: "t1", Parent: "root"},
		{ID: "t1a", Parent: "t1"},
		{ID: "t1b", Parent: "t1"},
		{ID: "other", Parent: "root"},
	}
}

func TestAncestorsOrderAndRoot(t *testing.T) {
	parent, err := BuildParentMap(sampleNodes())
	if err != nil {
		t.Fatal(err)
	}
	anc, err := Ancestors("t1a", parent)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(anc, []string{"t1", "root"}) {
		t.Fatalf("祖先链=%v", anc)
	}
	if a, _ := Ancestors("root", parent); len(a) != 0 {
		t.Fatalf("根应无祖先: %v", a)
	}
	if a, _ := Ancestors("ghost", parent); len(a) != 0 {
		t.Fatalf("不存在节点应空: %v", a)
	}
}

func TestScopeIncludesSelfFirst(t *testing.T) {
	parent, _ := BuildParentMap(sampleNodes())
	s, err := Scope("t1b", parent)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, []string{"t1b", "t1", "root"}) {
		t.Fatalf("scope=%v", s)
	}
}

func TestValidateTreeRejectsCycle(t *testing.T) {
	cycle := []Node{
		{ID: "a", Parent: "b"},
		{ID: "b", Parent: "a"},
	}
	if err := ValidateTree(cycle); err == nil {
		t.Fatal("成环应拒绝")
	}
	self := []Node{{ID: "a", Parent: "a"}}
	if err := ValidateTree(self); err == nil {
		t.Fatal("自引用应拒绝")
	}
	dangling := []Node{{ID: "a", Parent: "nope"}}
	if err := ValidateTree(dangling); err == nil {
		t.Fatal("悬空父应拒绝")
	}
	dup := []Node{{ID: "a"}, {ID: "a"}}
	if err := ValidateTree(dup); err == nil {
		t.Fatal("重复节点应拒绝")
	}
	good := sampleNodes()
	if err := ValidateTree(good); err != nil {
		t.Fatalf("合法树不应拒绝: %v", err)
	}
}

func TestRejectsOversizedID(t *testing.T) {
	big := string(make([]byte, 80))
	if _, err := BuildParentMap([]Node{{ID: big}}); err == nil {
		t.Fatal("超长 id 应拒绝")
	}
}

func TestDeepChainBounded(t *testing.T) {
	nodes := make([]Node, 0, MaxAncestors+2)
	for i := 0; i < MaxAncestors+2; i++ {
		p := ""
		if i > 0 {
			p = nodeName(i - 1)
		}
		nodes = append(nodes, Node{ID: nodeName(i), Parent: p})
	}
	parent, err := BuildParentMap(nodes)
	if err != nil {
		t.Fatal(err)
	}
	anc, err := Ancestors(nodeName(len(nodes)-1), parent)
	if err != nil {
		t.Fatalf("长链不应报环: %v", err)
	}
	if len(anc) > MaxAncestors {
		t.Fatalf("祖先链超上限: %d", len(anc))
	}
}

func nodeName(i int) string {
	return fmt.Sprintf("n%d", i)
}
