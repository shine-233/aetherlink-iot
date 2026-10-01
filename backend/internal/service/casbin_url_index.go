// 文件用途：g2 资源登记（GetUrl）的内存索引——精确集合 + 分段前缀树，替代每请求全量模式枚举。
// 核心逻辑：
//
//	· exact：g2 第 0 列原串集合（精确通道，O(1)）；
//	· trie：按段逐层分叉（字面子节点 map + 单一 ":param" 子节点），请求路径沿树下降，
//	  参数分支回溯，复杂度与路径段数相关而与模式总数（~300）无关；
//	· fuzzy：含 U+FFFD 字面段的模式（需 rune 级比较，极少见）单独线性兜底。
//
// 一致性：索引快照保存构建时的 g2 第 0 列串序列；每次查询在 enforcer 读锁下逐项比对
//
//	（Go 字符串比较先比指针，策略未变时近乎零成本），任何变化（LoadPolicy 整体换模型、
//	Add/Remove/Update 增量写、测试直接替换 global.CasbinEnforcer）都会在下次查询时
//	自动重建。因此无需在各处 LoadPolicy 调用点（role_permission/tenant/sys_user_* 等）
//	显式失效，也不存在"漏调一处重建 → 资源登记陈旧 → 鉴权放行/误拒"的风险。
//
// 关键注意事项：语义须与 utils.MatchURLPattern 逐位一致（含尾斜杠宽容、单独 ':' 为字面、
//
//	参数段非空），casbin_url_index_test.go 以旧全量扫描实现为 oracle 做等价性测试。
package service

import (
	"strings"
	"sync/atomic"

	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

// urlTrieNode 是分段前缀树节点。
type urlTrieNode struct {
	lit      map[string]*urlTrieNode
	param    *urlTrieNode
	terminal bool
}

func (n *urlTrieNode) insert(segs []utils.URLPatternSegment) {
	cur := n
	for _, s := range segs {
		if s.Param {
			if cur.param == nil {
				cur.param = &urlTrieNode{}
			}
			cur = cur.param
			continue
		}
		if cur.lit == nil {
			cur.lit = make(map[string]*urlTrieNode)
		}
		next := cur.lit[s.Literal]
		if next == nil {
			next = &urlTrieNode{}
			cur.lit[s.Literal] = next
		}
		cur = next
	}
	cur.terminal = true
}

// match 在已归一化路径的剩余部分 rest 上下降；零分配。
func (n *urlTrieNode) match(rest string) bool {
	j := strings.IndexByte(rest, '/')
	seg, tail, last := rest, "", true
	if j >= 0 {
		seg, tail, last = rest[:j], rest[j+1:], false
	}
	if child := n.lit[seg]; child != nil {
		if last {
			if child.terminal {
				return true
			}
		} else if child.match(tail) {
			return true
		}
	}
	if n.param != nil && seg != "" {
		if last {
			return n.param.terminal
		}
		return n.param.match(tail)
	}
	return false
}

// casbinURLIndex 是某一时刻 g2 策略的不可变快照索引。
type casbinURLIndex struct {
	enforcer *casbin.SyncedEnforcer
	ast      *model.Assertion
	keys     []string // 构建时 g2 各行第 0 列（空行记 ""），用于失效判定
	exact    map[string]struct{}
	root     *urlTrieNode
	fuzzy    []*utils.CompiledURLPattern
}

var casbinURLIndexPtr atomic.Pointer[casbinURLIndex]

func buildCasbinURLIndex(e *casbin.SyncedEnforcer, ast *model.Assertion) *casbinURLIndex {
	idx := &casbinURLIndex{
		enforcer: e,
		ast:      ast,
		keys:     make([]string, len(ast.Policy)),
		exact:    make(map[string]struct{}, len(ast.Policy)),
		root:     &urlTrieNode{},
	}
	for i, rule := range ast.Policy {
		if len(rule) == 0 {
			continue
		}
		key := rule[0]
		idx.keys[i] = key
		idx.exact[key] = struct{}{}
		if key == "" {
			continue // 与旧实现一致：空模式不参与模式通道
		}
		p := utils.CompileURLPattern(key)
		switch {
		case p.Never():
		case p.Fuzzy():
			idx.fuzzy = append(idx.fuzzy, p)
		default:
			idx.root.insert(p.Segments())
		}
	}
	return idx
}

// validFor 判断快照是否仍对应当前 g2 策略（调用方须持有 enforcer 读锁）。
func (idx *casbinURLIndex) validFor(e *casbin.SyncedEnforcer, ast *model.Assertion) bool {
	if idx == nil || idx.enforcer != e || idx.ast != ast || len(idx.keys) != len(ast.Policy) {
		return false
	}
	for i, rule := range ast.Policy {
		key := ""
		if len(rule) > 0 {
			key = rule[0]
		}
		if idx.keys[i] != key {
			return false
		}
	}
	return true
}

// lookup 判断 url 是否已登记：精确通道 → 前缀树 → fuzzy 兜底。
func (idx *casbinURLIndex) lookup(url string) bool {
	if _, ok := idx.exact[url]; ok {
		return true
	}
	path := utils.NormalizeURLPath(url)
	if idx.root.match(path) {
		return true
	}
	for _, p := range idx.fuzzy {
		if p.MatchNormalized(path) {
			return true
		}
	}
	return false
}

// casbinURLIndexFor 返回与 e 当前 g2 策略一致的索引；不存在 g2 定义时返回 nil。
// 在 enforcer 读锁内完成校验/构建（构建是纯读操作，可与其他读者并发），
// 并发构建者各自产出一致快照，后写者覆盖无害；陈旧快照下次查询即被重建。
func casbinURLIndexFor(e *casbin.SyncedEnforcer) *casbinURLIndex {
	lock := e.GetLock()
	lock.RLock()
	defer lock.RUnlock()
	ast := e.GetModel()["g"]["g2"]
	if ast == nil {
		return nil
	}
	if idx := casbinURLIndexPtr.Load(); idx.validFor(e, ast) {
		return idx
	}
	idx := buildCasbinURLIndex(e, ast)
	casbinURLIndexPtr.Store(idx)
	return idx
}
