// 文件用途：租户父子树（ROADMAP C2 ①）的加载/缓存/作用域解析服务。
//
//	上游 internal/hierarchy 提供纯语义（BuildParentMap/Ancestors/Scope/ValidateTree），
//	本包在其之上补齐运行时部分：数据源抽象（Source）、内存缓存（Tree）、并发安全与失效控制，
//	向上层 DAL / RBAC（Casbin）暴露共享的 Scope 服务。
//
// 核心逻辑：
//   - Refresh 从 Source 拉全量 tenants(id, parent_tenant_id)，经 hierarchy.ValidateTree
//     校验后建立 parent 索引与 children 索引并原子换入缓存；
//   - Scope/Ancestors 走"祖先方向"（=hierarchy.Scope，self∪祖先），供 RBAC 角色链（②）使用；
//   - Descendants 走"子树方向"（self∪全部后代），供 DAL 逐租户过滤条件 =→IN(Scope)（③）使用；
//   - 冷启动/失效后首次访问自动加载（写锁内刷新，天然合并并发击穿）；刷新失败保留上一份
//     健康缓存继续服务，并按最小间隔（retryAfter）退避重试，避免对 DB 的打点放大。
//
// 关键注意事项：
//   - 本包不感知业务表结构：Source 负责把 NULL/空串两种"根"约定归一为 Parent=""；
//   - 未登记进 tenants 表的租户 ID 退化为"仅自身"（与 hierarchy.Ancestors 对缺失节点的语义一致）；
//   - 数据源出现重复 ID/环/悬空父时 Refresh 直接报错且不污染缓存（fail-fast，数据完整性红线）。
//
// 边界（诚实标注）：
//   - DBSource 读取 public.tenants（parent_tenant_id 列由 60.sql 提供）；表/列缺失在首次
//     Refresh 时报错，不在启动期崩溃；
//   - 单例的装配（由谁持有哪些 Service 引用、何时 Invalidate）属 ②③ 接入阶段工作，本包只交付组件。
package tenantree

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/hierarchy"
)

// MaxTenantNodes 防止异常数据源放大内存占用（平台合理上限）。
const MaxTenantNodes = 100000

// retryAfter 刷新失败后的最小重试间隔（避免对 DB 打点放大）。
const retryAfter = 2 * time.Second

// ErrEmptyTenant 租户 ID 为空或全空白时返回。
var ErrEmptyTenant = errors.New("tenantree: 租户 ID 为空")

// ErrNilSource 未注入数据源时返回。
var ErrNilSource = errors.New("tenantree: 未注入 Source")

// ErrTooManyNodes 数据源节点数超过 MaxTenantNodes 时返回。
var ErrTooManyNodes = errors.New("tenantree: tenants 节点数超限")

// TenantNode 一条租户边：ID 的直接父为 Parent（Parent="" 表示根租户，由 Source 负责归一）。
type TenantNode struct {
	ID     string
	Parent string
}

// Source 租户树数据源：一次性返回全量租户父子边。
type Source interface {
	LoadTenantNodes(ctx context.Context) ([]TenantNode, error)
}

// LoadFunc 函数型 Source 适配器，便于注入 DB 查询或测试桩。
type LoadFunc func(ctx context.Context) ([]TenantNode, error)

// LoadTenantNodes 实现 Source。
func (f LoadFunc) LoadTenantNodes(ctx context.Context) ([]TenantNode, error) {
	return f(ctx)
}

// Stats 缓存运行状态（供观测/调试）。
type Stats struct {
	Loaded      bool
	Nodes       int
	RefreshedAt time.Time
	LastErr     error
}

// Tree 租户树缓存：并发安全，Refresh 整体换入（copy-on-write）。
type Tree struct {
	source Source

	mu sync.RWMutex

	parent   map[string]string // child -> parent（hierarchy 语义）
	children map[string][]string

	loaded      bool
	dirty       bool // Invalidate 后置位：下次访问需重载
	refreshErr  error
	lastAttempt time.Time

	nodeCount   int
	refreshedAt time.Time

	loadMu sync.Mutex // 单飞：并发读路径只允许一次在途加载
}

// New 以任意 Source 构建租户树缓存。
func New(source Source) *Tree {
	return &Tree{source: source}
}

// Refresh 强制从数据源重新加载并原子换入缓存（等价 refreshUnlocked）。
// 失败语义：加载/校验失败时若存在上一份健康缓存则继续保留并返回错误；
// 若从未成功加载则记录 refreshErr，Scope 等读操作返回该错误（fast-fail）。
func (t *Tree) Refresh(ctx context.Context) error {
	return t.refreshUnlocked(ctx)
}

// recordFailure 记录一次刷新失败：保留旧健康缓存；更新错误与退避时间戳。
func (t *Tree) recordFailure(err error) {
	now := time.Now()
	t.mu.Lock()
	t.refreshErr = err
	t.lastAttempt = now
	t.mu.Unlock()
}

// Invalidate 标记缓存失效：下一次 Scope/Ancestors/Descendants/ParentOf 会触发重载。
// 租户新增/变更/删除（含改父）后应调用，由上层在写路径统一挂接（②③ 接入阶段）。
func (t *Tree) Invalidate() {
	t.mu.Lock()
	t.dirty = true
	t.mu.Unlock()
}

// refreshDecisionLocked 判定读路径是否需要触发一次重载。调用方需持有读锁。
// 返回 (needRefresh, coldErr)：needRefresh=false 时 coldErr 非 nil 表示"冷失败退避期，
// 直接快速失败"；返回 nil 表示缓存可直接服务（健康缓存，或旧缓存仍在退避期服务）。
func (t *Tree) refreshDecisionLocked(now time.Time) (bool, error) {
	// 冷启动（从未成功加载）且最近一次尝试刚失败：退避期内快速失败，不打 DB。
	if !t.loaded && t.refreshErr != nil && now.Sub(t.lastAttempt) < retryAfter {
		return false, t.refreshErr
	}
	// 健康且未失效：直接服务。
	if t.loaded && !t.dirty && t.refreshErr == nil {
		return false, nil
	}
	// 显式失效（租户写路径刚发生）：立即重载，不受退避约束。
	if t.dirty {
		return true, nil
	}
	// 有旧健康缓存但上次刷新失败：退避期内继续用旧缓存，到期后重试。
	if t.refreshErr != nil && now.Sub(t.lastAttempt) < retryAfter {
		return false, nil
	}
	return true, nil
}

// loadIfNeeded 冷启动/失效/退避到期时加载一次；loadMu 单飞保证并发读只触发一次在途加载。
// 返回 nil 表示缓存可服务（含"旧健康缓存继续服务"）；否则返回需上抛的错误（冷失败退避期）。
func (t *Tree) loadIfNeeded(ctx context.Context) error {
	t.mu.RLock()
	need, coldErr := t.refreshDecisionLocked(time.Now())
	t.mu.RUnlock()
	if !need {
		return coldErr
	}

	t.loadMu.Lock()
	defer t.loadMu.Unlock()
	// 双检：等待 loadMu 期间可能已被其他调用方刷新。
	t.mu.RLock()
	need, coldErr = t.refreshDecisionLocked(time.Now())
	t.mu.RUnlock()
	if !need {
		return coldErr
	}

	if err := t.refreshUnlocked(ctx); err != nil {
		t.mu.RLock()
		defer t.mu.RUnlock()
		if !t.loaded {
			return t.refreshErr
		}
		return nil // 旧缓存仍健康，读路径继续服务
	}
	return nil
}

// refreshUnlocked 加载并换入缓存（锁外调用，内部自行加写锁换入）。
func (t *Tree) refreshUnlocked(ctx context.Context) error {
	if t == nil || t.source == nil {
		// 必须记录失败：否则冷启动错误丢失，读路径会静默退化为"仅自身"。
		t.recordFailure(ErrNilSource)
		return ErrNilSource
	}
	nodes, err := t.source.LoadTenantNodes(ctx)
	if err != nil {
		t.recordFailure(err)
		return err
	}
	if len(nodes) > MaxTenantNodes {
		err := ErrTooManyNodes
		t.recordFailure(err)
		return err
	}
	hnodes := toHierarchyNodes(nodes)
	if err := hierarchy.ValidateTree(hnodes); err != nil {
		t.recordFailure(err)
		return err
	}
	parent, err := hierarchy.BuildParentMap(hnodes)
	if err != nil {
		t.recordFailure(err)
		return err
	}
	children := buildChildren(nodes)

	now := time.Now()
	t.mu.Lock()
	t.parent = parent
	t.children = children
	t.nodeCount = len(nodes)
	t.loaded = true
	t.dirty = false
	t.refreshErr = nil
	t.refreshedAt = now
	t.lastAttempt = now
	t.mu.Unlock()
	return nil
}

// Scope 返回 tenantID 的数据作用域 = {self} ∪ 祖先（近→远），self 恒在首位。
// 供 RBAC 角色集扩展（②）与需要"向上一并纳入"的过滤场景使用；语义与 hierarchy.Scope 一致。
// 未登记租户退化为 [tenantID]（仅自身，保持与既有隔离行为兼容）。
func (t *Tree) Scope(ctx context.Context, tenantID string) ([]string, error) {
	id, err := normalizeID(tenantID)
	if err != nil {
		return nil, err
	}
	if err := t.loadIfNeeded(ctx); err != nil {
		return nil, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return hierarchy.Scope(id, t.parent)
}

// Ancestors 返回 tenantID 的祖先链（近→远，不含自身）。
func (t *Tree) Ancestors(ctx context.Context, tenantID string) ([]string, error) {
	id, err := normalizeID(tenantID)
	if err != nil {
		return nil, err
	}
	if err := t.loadIfNeeded(ctx); err != nil {
		return nil, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return hierarchy.Ancestors(id, t.parent)
}

// Descendants 返回 tenantID 的子树作用域 = {self} ∪ 全部后代（BFS 序，self 在首位）。
// 供 DAL 把"租户过滤 = 某 ID"改写为 IN (Descendants(me))（③），使上级租户可见自身+后代数据。
// 未登记租户退化为 [tenantID]（无后代）。
func (t *Tree) Descendants(ctx context.Context, tenantID string) ([]string, error) {
	id, err := normalizeID(tenantID)
	if err != nil {
		return nil, err
	}
	if err := t.loadIfNeeded(ctx); err != nil {
		return nil, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := []string{id}
	queue := []string{id}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, child := range t.children[cur] {
			out = append(out, child)
			queue = append(queue, child)
		}
	}
	return out, nil
}

// ParentOf 返回 tenantID 的直接父租户；未登记返回 ("", false)。
func (t *Tree) ParentOf(ctx context.Context, tenantID string) (string, bool, error) {
	id, err := normalizeID(tenantID)
	if err != nil {
		return "", false, err
	}
	if err := t.loadIfNeeded(ctx); err != nil {
		return "", false, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	p, ok := t.parent[id]
	return p, ok && p != "", nil
}

// Stats 返回当前缓存运行状态（幂等只读）。
func (t *Tree) Stats() Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	s := Stats{
		Loaded:      t.loaded,
		Nodes:       t.nodeCount,
		RefreshedAt: t.refreshedAt,
		LastErr:     t.refreshErr,
	}
	if !t.loaded && t.refreshErr == nil && t.source == nil {
		s.LastErr = ErrNilSource
	}
	return s
}

// normalizeID 校验并裁剪租户 ID。
func normalizeID(tenantID string) (string, error) {
	id := strings.TrimSpace(tenantID)
	if id == "" {
		return "", ErrEmptyTenant
	}
	if len(id) > 64 {
		return "", errors.New("tenantree: 租户 ID 超长")
	}
	return id, nil
}

func toHierarchyNodes(nodes []TenantNode) []hierarchy.Node {
	out := make([]hierarchy.Node, len(nodes))
	for i, n := range nodes {
		out[i] = hierarchy.Node{ID: n.ID, Parent: n.Parent}
	}
	return out
}

// buildChildren 由父子边构建 child 邻接（保持数据源顺序，供 Descendants BFS 使用）。
func buildChildren(nodes []TenantNode) map[string][]string {
	children := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		if n.Parent == "" {
			continue
		}
		children[n.Parent] = append(children[n.Parent], n.ID)
	}
	return children
}
