// 文件用途：资产/租户层级（ROADMAP C2）的语义核心——纯逻辑、无存储依赖。
// 核心逻辑：parent 指针链的合法性校验（自引用/成环拒绝）、由 node 向上推导祖先链
//   （Ancestors）、以及"层级作用域"决策（scope=node∪ancestors，用于数据隔离级联过滤）。
// 关键注意事项：
//   - 本包不读写 DB；上层把 tenants.parent_tenant_id / assets.parent_id 喂进来即可复用；
//   - 拒绝自引用与环（含悬空父指针由上层按外键保证）；
//   - 入参长度钳制（id ≤64），防放大；结果按从近到远排序。
// 重构建议：RBAC 继承（子租户继承父租户角色策略）与各 DAL raw 查询的级联 WHERE
//   在接入阶段按本包导出的 Scope 语义逐模块替换，见交付说明。
package hierarchy

import (
	"fmt"
	"strings"
)

// Edge 一条父子关系：Child 的直接父为 Parent。
type Edge struct {
	Child  string
	Parent string
}

// Node 用于校验/推导的节点视图。
type Node struct {
	ID     string
	Parent string // 根节点为 ""
}

// MaxAncestors 防止恶意深链放大（平台合理上限）。
const MaxAncestors = 64

// BuildParentMap 由节点列表构建 child→parent 索引（重复 child 报错）。
func BuildParentMap(nodes []Node) (map[string]string, error) {
	m := map[string]string{}
	for _, n := range nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" || len(id) > 64 {
			return nil, fmt.Errorf("hierarchy: 非法节点 id")
		}
		if _, dup := m[id]; dup {
			return nil, fmt.Errorf("hierarchy: 重复节点 %q", id)
		}
		m[id] = strings.TrimSpace(n.Parent)
	}
	return m, nil
}

// Ancestors 返回 nodeID 的祖先链（近→远，不含自身）；根或缺失返回空切片。
// 遇到环/自引用返回错误（数据完整性红线）。
func Ancestors(nodeID string, parent map[string]string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{nodeID: true}
	cur := strings.TrimSpace(nodeID)
	for i := 0; i < MaxAncestors; i++ {
		p, ok := parent[cur]
		if !ok || p == "" {
			break
		}
		if seen[p] {
			return nil, fmt.Errorf("hierarchy: 检测到环（节点 %q）", p)
		}
		if len(p) > 64 {
			return nil, fmt.Errorf("hierarchy: 父 id 超长")
		}
		out = append(out, p)
		seen[p] = true
		cur = p
		if len(out) >= MaxAncestors {
			break
		}
	}
	return out, nil
}

// Scope 返回某节点的有效作用域 = {self} ∪ Ancestors；上层用它拼级联过滤条件
// （如 tenant_id IN (scope…) 或按资产子树展开）。self 恒在首位。
func Scope(nodeID string, parent map[string]string) ([]string, error) {
	anc, err := Ancestors(nodeID, parent)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, 1+len(anc))
	out = append(out, nodeID)
	out = append(out, anc...)
	return out, nil
}

// ValidateTree 全量校验：无重复父索引、逐节点无环、父指针要么为空要么存在。
func ValidateTree(nodes []Node) error {
	parent, err := BuildParentMap(nodes)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		id := strings.TrimSpace(n.ID)
		if n.Parent == "" {
			continue
		}
		if _, exists := parent[n.Parent]; !exists {
			return fmt.Errorf("hierarchy: 节点 %q 的父 %q 不存在", id, n.Parent)
		}
		if _, err := Ancestors(id, parent); err != nil {
			return err
		}
	}
	return nil
}
