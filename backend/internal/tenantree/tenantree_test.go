// 文件用途：tenantree 加载/缓存/作用域解析单测——不依赖真实 DB，全部走内存桩 Source。
// 覆盖：Scope 祖先方向、Descendants 子树方向、未登记退化、失效重载、冷启动失败快速失败
//
//	与退避、旧缓存保留、脏数据（重复/环/悬空父）拒绝、并发单飞、空 ID/超限拒绝。
package tenantree

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeSource 内存桩：每次 LoadTenantNodes 计数 +1，便于断言懒加载/单飞/重载次数。
type fakeSource struct {
	calls int32
	fn    func() ([]TenantNode, error)
}

func (f *fakeSource) LoadTenantNodes(context.Context) ([]TenantNode, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.fn()
}

// sampleTree：root → t1 → t1a → t1a1；t1 → t1b；root → other。
func sampleTree() []TenantNode {
	return []TenantNode{
		{ID: "root", Parent: ""},
		{ID: "t1", Parent: "root"},
		{ID: "t1a", Parent: "t1"},
		{ID: "t1b", Parent: "t1"},
		{ID: "t1a1", Parent: "t1a"},
		{ID: "other", Parent: "root"},
	}
}

func callsOf(t *testing.T, f *fakeSource) int32 {
	t.Helper()
	return atomic.LoadInt32(&f.calls)
}

func TestScopeAncestorsDirection(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()

	got, err := tree.Scope(ctx, "t1a")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"t1a", "t1", "root"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Scope(t1a)=%v, want %v", got, want)
	}
	// 根无祖先，仅自身。
	rootScope, err := tree.Scope(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"root"}; !reflect.DeepEqual(rootScope, want) {
		t.Fatalf("Scope(root)=%v, want %v", rootScope, want)
	}
}

func TestAncestorsOrder(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	got, err := tree.Ancestors(context.Background(), "t1a1")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"t1a", "t1", "root"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Ancestors=%v, want %v", got, want)
	}
}

func TestDescendantsSubtreeDirection(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()

	cases := []struct {
		id   string
		want []string
	}{
		{"root", []string{"root", "t1", "other", "t1a", "t1b", "t1a1"}}, // BFS：root→t1、other；t1→t1a、t1b；t1a→t1a1
		{"t1", []string{"t1", "t1a", "t1b", "t1a1"}},
		{"t1a", []string{"t1a", "t1a1"}},
		{"t1a1", []string{"t1a1"}},
	}
	for _, c := range cases {
		got, err := tree.Descendants(ctx, c.id)
		if err != nil {
			t.Fatalf("Descendants(%s): %v", c.id, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("Descendants(%s)=%v, want %v", c.id, got, c.want)
		}
	}
}

func TestUnregisteredTenantDegradesToSelf(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()

	if got, err := tree.Scope(ctx, "ghost"); err != nil || !reflect.DeepEqual(got, []string{"ghost"}) {
		t.Fatalf("未登记租户 Scope=%v, err=%v", got, err)
	}
	if got, err := tree.Descendants(ctx, "ghost"); err != nil || !reflect.DeepEqual(got, []string{"ghost"}) {
		t.Fatalf("未登记租户 Descendants=%v, err=%v", got, err)
	}
	if _, ok, err := tree.ParentOf(ctx, "ghost"); err != nil || ok {
		t.Fatalf("未登记租户 ParentOf: ok=%v err=%v", ok, err)
	}
}

func TestParentOf(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()

	if p, ok, err := tree.ParentOf(ctx, "t1a"); err != nil || !ok || p != "t1" {
		t.Fatalf("ParentOf(t1a)=(%q,%v,%v)", p, ok, err)
	}
	if p, ok, err := tree.ParentOf(ctx, "root"); err != nil || ok || p != "" {
		t.Fatalf("根租户应无父: ParentOf(root)=(%q,%v,%v)", p, ok, err)
	}
}

func TestLazyLoadOnceOnFirstAccess(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()

	if _, err := tree.Scope(ctx, "t1a"); err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Descendants(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	if c := callsOf(t, src); c != 1 {
		t.Fatalf("首次访问后应只加载 1 次，实际 %d", c)
	}
}

func TestInvalidateTriggersReload(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()

	if _, err := tree.Scope(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	if c := callsOf(t, src); c != 1 {
		t.Fatalf("预热后应只加载 1 次，实际 %d", c)
	}
	tree.Invalidate()
	if _, err := tree.Scope(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	if c := callsOf(t, src); c != 2 {
		t.Fatalf("Invalidate 后应重载，实际加载 %d 次", c)
	}
}

func TestInvalidateSeesUpdatedTree(t *testing.T) {
	state := sampleTree()
	src := &fakeSource{fn: func() ([]TenantNode, error) { return state, nil }}
	tree := New(src)
	ctx := context.Background()

	if got, _ := tree.Scope(ctx, "t9"); !reflect.DeepEqual(got, []string{"t9"}) {
		t.Fatalf("变更前 Scope(t9)=%v", got)
	}
	// 新增 t9 挂到 t1 下。
	state = append(state, TenantNode{ID: "t9", Parent: "t1"})
	tree.Invalidate()
	got, err := tree.Scope(ctx, "t9")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"t9", "t1", "root"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("失效重载后 Scope(t9)=%v, want %v", got, want)
	}
}

func TestColdFailureFastFailThenRefreshRecovery(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) {
		return nil, errors.New("db down")
	}}
	tree := New(src)
	ctx := context.Background()

	if _, err := tree.Scope(ctx, "t1"); err == nil {
		t.Fatal("冷启动加载失败应返回错误")
	}
	// 退避期内再次访问：快速失败，不重复打源。
	if _, err := tree.Descendants(ctx, "t1"); err == nil {
		t.Fatal("退避期内应快速失败")
	}
	if c := callsOf(t, src); c != 1 {
		t.Fatalf("退避期内不应重复加载，实际 %d 次", c)
	}
	// 数据源恢复后强制 Refresh 成功。
	src.fn = func() ([]TenantNode, error) { return sampleTree(), nil }
	if err := tree.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := tree.Scope(ctx, "t1a")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"t1a", "t1", "root"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("恢复后 Scope=%v", got)
	}
}

func TestRefreshFailureKeepsOldCache(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()

	if _, err := tree.Scope(ctx, "t1a"); err != nil {
		t.Fatal(err)
	}
	// 源故障：Refresh 报错但旧缓存继续服务。
	src.fn = func() ([]TenantNode, error) { return nil, errors.New("db down") }
	if err := tree.Refresh(ctx); err == nil {
		t.Fatal("Refresh 应返回错误")
	}
	got, err := tree.Scope(ctx, "t1a")
	if err != nil {
		t.Fatalf("旧缓存应继续服务: %v", err)
	}
	if want := []string{"t1a", "t1", "root"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Scope=%v, want %v", got, want)
	}
	if st := tree.Stats(); st.LastErr == nil || !st.Loaded {
		t.Fatalf("Stats 应记录刷新失败: %+v", st)
	}
}

func TestCorruptSourceRejectedWithoutCachePollution(t *testing.T) {
	// 重复 ID：建树失败；无旧缓存时应报错，有旧缓存时应继续服务。
	dup := func() ([]TenantNode, error) {
		return []TenantNode{
			{ID: "a", Parent: ""},
			{ID: "a", Parent: "b"},
		}, nil
	}
	tree := New(&fakeSource{fn: dup})
	if _, err := tree.Scope(context.Background(), "a"); err == nil {
		t.Fatal("重复 ID 应被拒绝")
	}

	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree = New(src)
	ctx := context.Background()
	if _, err := tree.Scope(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	src.fn = dup
	if err := tree.Refresh(ctx); err == nil {
		t.Fatal("脏数据 Refresh 应报错")
	}
	got, err := tree.Scope(ctx, "t1a")
	if err != nil || !reflect.DeepEqual(got, []string{"t1a", "t1", "root"}) {
		t.Fatalf("脏数据不应污染旧缓存: got=%v err=%v", got, err)
	}
}

func TestCycleRejected(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) {
		return []TenantNode{
			{ID: "a", Parent: "b"},
			{ID: "b", Parent: "a"},
		}, nil
	}}
	tree := New(src)
	if _, err := tree.Scope(context.Background(), "a"); err == nil {
		t.Fatal("成环数据应被拒绝")
	}
}

func TestDanglingParentRejected(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) {
		return []TenantNode{
			{ID: "a", Parent: "ghost-parent"},
		}, nil
	}}
	tree := New(src)
	if _, err := tree.Descendants(context.Background(), "a"); err == nil {
		t.Fatal("悬空父应被拒绝")
	}
}

func TestConcurrentScopeSingleFlight(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()

	const n = 32
	var wg sync.WaitGroup
	errs := make([]error, n)
	res := make([][]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res[i], errs[i] = tree.Scope(ctx, "t1a")
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if !reflect.DeepEqual(res[i], []string{"t1a", "t1", "root"}) {
			t.Fatalf("goroutine %d 结果不一致: %v", i, res[i])
		}
	}
	if c := callsOf(t, src); c != 1 {
		t.Fatalf("并发冷启动应只加载 1 次，实际 %d", c)
	}
}

func TestEmptyTenantIDRejected(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return sampleTree(), nil }}
	tree := New(src)
	ctx := context.Background()
	for _, id := range []string{"", "   "} {
		if _, err := tree.Scope(ctx, id); err == nil {
			t.Fatalf("空租户 ID %q 应报错", id)
		}
		if _, err := tree.Descendants(ctx, id); err == nil {
			t.Fatalf("空租户 ID %q Descendants 应报错", id)
		}
		if _, _, err := tree.ParentOf(ctx, id); err == nil {
			t.Fatalf("空租户 ID %q ParentOf 应报错", id)
		}
	}
}

func TestZeroNodesTree(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) { return nil, nil }}
	tree := New(src)
	ctx := context.Background()
	if got, err := tree.Scope(ctx, "x"); err != nil || !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("空树 Scope(x)=%v err=%v", got, err)
	}
	if st := tree.Stats(); !st.Loaded || st.Nodes != 0 {
		t.Fatalf("空树 Stats=%+v", st)
	}
}

func TestMaxNodesGuard(t *testing.T) {
	src := &fakeSource{fn: func() ([]TenantNode, error) {
		nodes := make([]TenantNode, MaxTenantNodes+1)
		for i := range nodes {
			nodes[i] = TenantNode{ID: "t"}
		}
		return nodes, nil
	}}
	tree := New(src)
	if _, err := tree.Scope(context.Background(), "t"); err == nil {
		t.Fatal("超限应被拒绝")
	}
}

func TestNilSource(t *testing.T) {
	tree := New(nil)
	if _, err := tree.Scope(context.Background(), "a"); err == nil {
		t.Fatal("nil Source 应报错")
	}
	if err := NewDBTree(nil).Refresh(context.Background()); err == nil {
		t.Fatal("nil DB 的 Refresh 应报错")
	}
	if _, err := NewDBSource(nil).LoadTenantNodes(context.Background()); err == nil {
		t.Fatal("nil DB 的 LoadTenantNodes 应报错")
	}
}

// 编译期断言：LoadFunc 满足 Source 接口。
var _ Source = LoadFunc(nil)
var _ Source = (*fakeSource)(nil)
