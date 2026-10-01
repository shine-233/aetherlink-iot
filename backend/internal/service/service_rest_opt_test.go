// 文件用途：覆盖服务层（m-z）优化/修复点——UTF-8 安全截断、告警去重注册表清理、
// market 响应体上限、租户防环判定、租户实体批量统计与更新名称校验。
package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestTruncateUTF8BytesKeepsValidUTF8(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"abc", 10, "abc"},
		{"abcdef", 3, "abc"},
		{"", 3, ""},
		{"abc", 0, ""},
		// "中" 占 3 字节：切在字符中间时必须回退到边界。
		{"a中文", 2, "a"},
		{"a中文", 4, "a中"},
		{"中文", 5, "中"},
	}
	for _, tc := range cases {
		got := truncateUTF8Bytes(tc.in, tc.max)
		if got != tc.want {
			t.Fatalf("truncateUTF8Bytes(%q,%d)=%q want %q", tc.in, tc.max, got, tc.want)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("truncateUTF8Bytes(%q,%d) produced invalid UTF-8", tc.in, tc.max)
		}
	}

	// 回归：旧实现 e[:1000] 对 1000 字节处恰好切断多字节字符的中文错误信息产生非法 UTF-8。
	long := "xy" + strings.Repeat("错", 400)
	if utf8.ValidString(long[:1000]) {
		t.Fatalf("fixture should split a rune at byte 1000")
	}
	got := truncateUTF8Bytes(long, 1000)
	if !utf8.ValidString(got) || len(got) > 1000 || len(got) < 997 {
		t.Fatalf("unexpected truncation len=%d valid=%v", len(got), utf8.ValidString(got))
	}
}

func TestRecordRuleChainNodeTraceTruncatesErrorOnRuneBoundary(t *testing.T) {
	captured := make(chan *model.RuleChainNodeTrace, 1)
	origEnabled, origWriter := ruleChainTraceEnabled, ruleChainTraceWriter
	defer func() { ruleChainTraceEnabled, ruleChainTraceWriter = origEnabled, origWriter }()
	ruleChainTraceEnabled = func() bool { return true }
	ruleChainTraceWriter = func(trace *model.RuleChainNodeTrace) error {
		captured <- trace
		return nil
	}
	e := &ruleChainExecution{ctx: context.Background(), graph: &RuleChainGraph{ChainID: "chain-u"}, execID: "exec-u"}
	node := &RuleChainNode{ID: "n-u", Type: RuleChainFilterThreshold}
	msg := ruleChainMessage{Payload: map[string]any{}, Metadata: map[string]any{}, Rcc: &RuleChainContext{TenantID: "tenant-1"}}
	nodeErr := errors.New("y" + strings.Repeat("失", 300))
	recordRuleChainNodeTrace(e, node, msg, ruleChainNodeResult{}, nodeErr, time.Millisecond)

	select {
	case trace := <-captured:
		if trace.ErrorMsg == nil {
			t.Fatalf("expected error message")
		}
		if !utf8.ValidString(*trace.ErrorMsg) || len(*trace.ErrorMsg) > ruleChainTraceErrorMax {
			t.Fatalf("error msg invalid: len=%d valid=%v", len(*trace.ErrorMsg), utf8.ValidString(*trace.ErrorMsg))
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("trace not written")
	}
}

func TestInMemoryAlarmDedupSweepsExpiredEntries(t *testing.T) {
	alarmDedupMu.Lock()
	origRegistry, origMax := alarmDedupRegistry, alarmDedupMaxWindow
	alarmDedupRegistry = map[string]time.Time{}
	alarmDedupMaxWindow = 0
	origFloor := alarmDedupSweepFloor
	alarmDedupSweepFloor = 0
	alarmDedupMu.Unlock()
	defer func() {
		alarmDedupMu.Lock()
		alarmDedupRegistry, alarmDedupMaxWindow = origRegistry, origMax
		alarmDedupSweepFloor = origFloor
		alarmDedupMu.Unlock()
	}()

	store := ruleChainInMemoryAlarmDedup{}
	window := 50 * time.Millisecond
	if store.SeenWithin("fresh", window) {
		t.Fatalf("unseen key reported as seen")
	}
	stale := time.Now().Add(-time.Hour)
	alarmDedupMu.Lock()
	for i := 0; i <= alarmDedupSweepLimit; i++ {
		alarmDedupRegistry[fmt.Sprintf("stale-%d", i)] = stale
	}
	alarmDedupMu.Unlock()

	store.MarkSeen("fresh")
	alarmDedupMu.Lock()
	size := len(alarmDedupRegistry)
	alarmDedupMu.Unlock()
	if size != 1 {
		t.Fatalf("expected stale entries swept, registry size=%d", size)
	}
	if !store.SeenWithin("fresh", window) {
		t.Fatalf("fresh key must remain deduped inside window")
	}
}

func TestInMemoryAlarmDedupKeepsEntriesInsideLargestWindow(t *testing.T) {
	alarmDedupMu.Lock()
	origRegistry, origMax := alarmDedupRegistry, alarmDedupMaxWindow
	alarmDedupRegistry = map[string]time.Time{}
	alarmDedupMaxWindow = 0
	origFloor := alarmDedupSweepFloor
	alarmDedupSweepFloor = 0
	alarmDedupMu.Unlock()
	defer func() {
		alarmDedupMu.Lock()
		alarmDedupRegistry, alarmDedupMaxWindow = origRegistry, origMax
		alarmDedupSweepFloor = origFloor
		alarmDedupMu.Unlock()
	}()

	store := ruleChainInMemoryAlarmDedup{}
	// 大窗口曾被查询过：比小窗口旧、但仍在大窗口内的条目不得被清理。
	_ = store.SeenWithin("probe", 2*time.Hour)
	recent := time.Now().Add(-time.Hour)
	alarmDedupMu.Lock()
	for i := 0; i <= alarmDedupSweepLimit; i++ {
		alarmDedupRegistry[fmt.Sprintf("k-%d", i)] = recent
	}
	alarmDedupMu.Unlock()
	store.MarkSeen("new")
	if !store.SeenWithin("k-0", 2*time.Hour) {
		t.Fatalf("entry inside the largest observed window was swept")
	}
}

// 存活条目超过上限时不得每次 MarkSeen 都全表扫描：清理后阈值随存活数翻倍。
func TestInMemoryAlarmDedupSweepIsAmortizedWhenLiveEntriesExceedLimit(t *testing.T) {
	alarmDedupMu.Lock()
	origRegistry, origMax, origFloor := alarmDedupRegistry, alarmDedupMaxWindow, alarmDedupSweepFloor
	alarmDedupRegistry = map[string]time.Time{}
	alarmDedupMaxWindow = time.Hour
	alarmDedupSweepFloor = 0
	recent := time.Now()
	for i := 0; i <= alarmDedupSweepLimit; i++ {
		alarmDedupRegistry[fmt.Sprintf("live-%d", i)] = recent
	}
	alarmDedupMu.Unlock()
	defer func() {
		alarmDedupMu.Lock()
		alarmDedupRegistry, alarmDedupMaxWindow, alarmDedupSweepFloor = origRegistry, origMax, origFloor
		alarmDedupMu.Unlock()
	}()

	store := ruleChainInMemoryAlarmDedup{}
	store.MarkSeen("a")
	alarmDedupMu.Lock()
	floor := alarmDedupSweepFloor
	alarmDedupMu.Unlock()
	if floor != alarmDedupSweepLimit+1 {
		t.Fatalf("sweep floor = %d, want %d (all live entries survive)", floor, alarmDedupSweepLimit+1)
	}
	// 下一次 MarkSeen 未超过 2*floor，不应再次清理（floor 不变）。
	store.MarkSeen("b")
	alarmDedupMu.Lock()
	floorAfter := alarmDedupSweepFloor
	alarmDedupMu.Unlock()
	if floorAfter != floor {
		t.Fatalf("sweep re-ran below amortized threshold: floor %d -> %d", floor, floorAfter)
	}
}

func TestReadMarketResponseRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 1<<20)
		for written := int64(0); written <= marketMaxResponseBytes; written += int64(len(chunk)) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	client := &MarketClient{baseURL: server.URL, httpClient: server.Client()}
	_, err := client.CheckUserExists(context.Background(), "fixture@example.com")
	if !errors.Is(err, ErrMarketInvalidResponse) {
		t.Fatalf("expected ErrMarketInvalidResponse for oversized body, got %v", err)
	}
}

func TestTenantParentWouldCycle(t *testing.T) {
	parent := map[string]string{"root": "", "a": "root", "b": "a", "c": "b"}
	if !tenantParentWouldCycle("a", "c", parent) {
		t.Fatalf("attaching a under its descendant c must be a cycle")
	}
	if tenantParentWouldCycle("c", "root", parent) {
		t.Fatalf("attaching c under root is not a cycle")
	}
	if tenantParentWouldCycle("x", "unknown", parent) {
		t.Fatalf("unknown parent chain terminates and is not a cycle")
	}
	// 脏数据已有环：fail-closed。
	loop := map[string]string{"p": "q", "q": "p"}
	if !tenantParentWouldCycle("z", "p", loop) {
		t.Fatalf("pre-existing loop must be treated as cycle")
	}

	// 回归：旧实现用 hierarchy.Descendants（最多遍历 64 个子孙），宽树下会漏判。
	wide := map[string]string{"top": ""}
	for i := 0; i < 100; i++ {
		wide[fmt.Sprintf("child-%03d", i)] = "top"
	}
	wide["deep"] = "child-099"
	if !tenantParentWouldCycle("top", "deep", wide) {
		t.Fatalf("descendant beyond 64 siblings must still be detected as cycle")
	}
}

func setupTenantCountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, stmt := range []string{
		"CREATE TABLE tenants (id TEXT PRIMARY KEY, name TEXT NOT NULL, parent_tenant_id TEXT NOT NULL DEFAULT '', created_at DATETIME, updated_at DATETIME)",
		"CREATE TABLE devices (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL)",
		"CREATE TABLE users (id TEXT PRIMARY KEY, tenant_id TEXT)",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestListTenantsBatchesEntityCounts(t *testing.T) {
	db := setupTenantCountTestDB(t)
	now := time.Now().UTC()
	for i, id := range []string{"t1", "t2", "t3"} {
		if err := db.Exec("INSERT INTO tenants (id, name, parent_tenant_id, created_at, updated_at) VALUES (?, ?, '', ?, ?)",
			id, "tenant-"+id, now.Add(time.Duration(i)*time.Second), now).Error; err != nil {
			t.Fatalf("seed tenant: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		db.Exec("INSERT INTO devices (id, tenant_id) VALUES (?, 't1')", fmt.Sprintf("d1-%d", i))
	}
	db.Exec("INSERT INTO devices (id, tenant_id) VALUES ('d2-0', 't2')")
	db.Exec("INSERT INTO devices (id, tenant_id) VALUES ('dx-0', 'other')")
	db.Exec("INSERT INTO users (id, tenant_id) VALUES ('u1', 't1'), ('u2', 't3'), ('u3', 't3')")

	list, total, err := (&TenantService{}).ListTenants(context.Background(), 1, 10, "",
		&utils.UserClaims{ID: "admin", Authority: "SYS_ADMIN"})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("total=%d len=%d", total, len(list))
	}
	want := map[string][2]int64{"t1": {3, 1}, "t2": {1, 0}, "t3": {0, 2}}
	for _, vo := range list {
		w := want[vo.ID]
		if vo.DeviceCount != w[0] || vo.UserCount != w[1] {
			t.Fatalf("tenant %s counts device=%d user=%d want %v", vo.ID, vo.DeviceCount, vo.UserCount, w)
		}
	}
}

func TestTenantScopeForTenantAdmin(t *testing.T) {
	db := setupTenantCountTestDB(t)
	now := time.Now().UTC()
	for _, row := range [][2]string{{"hq", ""}, {"branch", "hq"}, {"other", ""}} {
		db.Exec("INSERT INTO tenants (id, name, parent_tenant_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			row[0], row[0], row[1], now, now)
	}
	claims := &utils.UserClaims{ID: "ta", Authority: "TENANT_ADMIN", TenantID: "hq"}
	svc := &TenantService{}

	list, total, err := svc.ListTenants(context.Background(), 1, 10, "", claims)
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("tenant admin list: total=%d len=%d err=%v", total, len(list), err)
	}
	if _, err := svc.GetTenant(context.Background(), "branch", claims); err != nil {
		t.Fatalf("descendant must be visible: %v", err)
	}
	if _, err := svc.GetTenant(context.Background(), "other", claims); err == nil {
		t.Fatalf("unrelated tenant must not be visible")
	}
}

func TestUpdateTenantRejectsBlankName(t *testing.T) {
	db := setupTenantCountTestDB(t)
	now := time.Now().UTC()
	db.Exec("INSERT INTO tenants (id, name, parent_tenant_id, created_at, updated_at) VALUES ('t1', 'keep-me', '', ?, ?)", now, now)

	_, err := (&TenantService{}).UpdateTenant(context.Background(), "t1", &model.UpdateTenantReq{Name: "   "},
		&utils.UserClaims{ID: "admin", Authority: "SYS_ADMIN"})
	if err == nil {
		t.Fatalf("blank tenant name must be rejected")
	}
	var name string
	db.Raw("SELECT name FROM tenants WHERE id = 't1'").Scan(&name)
	if name != "keep-me" {
		t.Fatalf("tenant name was overwritten to %q", name)
	}
}
