// 文件用途: 覆盖实体版本差异对比（ROADMAP TB-25）——纯函数语义 diff 的四类基础用例
//           （新增/删除/嵌套修改/数组变化）与服务层 DiffEntityVersion 的租户隔离契约。
// 核心逻辑: 纯函数直接喂 JSON 字符串断言路径列表；服务层用 sqlite 内存库驱动
//           跨租户拒绝、缺失版本拒绝与两侧版本元信息回显。
// 关键注意事项: 数值经 json.Number 字面量比较（1 与 1.0 判异）；数组下标是路径段。
package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

func mustDiffSnapshots(t *testing.T, oldJSON, newJSON string) *model.EntityVersionDiffResult {
	t.Helper()
	result, err := DiffEntityVersionSnapshots(oldJSON, newJSON)
	if err != nil {
		t.Fatalf("diff snapshots: %v", err)
	}
	return result
}

// containsPath 断言路径列表中包含指定路径。
func containsPath(t *testing.T, paths []string, want string) {
	t.Helper()
	for _, path := range paths {
		if path == want {
			return
		}
	}
	t.Fatalf("path %q not found in %v", want, paths)
}

func TestDiffEntityVersionSnapshotsAddedPaths(t *testing.T) {
	result := mustDiffSnapshots(t,
		`{"name":"board-a","enabled":true}`,
		`{"name":"board-a","enabled":true,"remark":"new leaf"}`,
	)

	if len(result.Added) != 1 || result.Added[0] != "remark" {
		t.Fatalf("added = %v, want [remark]", result.Added)
	}
	if len(result.Removed) != 0 || len(result.Modified) != 0 {
		t.Fatalf("unexpected removed %v / modified %v", result.Removed, result.Modified)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
	change := result.Changes[0]
	if change.Kind != diffKindAdded || change.Path != "remark" {
		t.Fatalf("change = %+v, want added/remark", change)
	}
	if change.NewValue != "new leaf" {
		t.Fatalf("new value = %v, want %q", change.NewValue, "new leaf")
	}
	if change.OldValue != nil {
		t.Fatalf("added change must not carry old value, got %v", change.OldValue)
	}
}

func TestDiffEntityVersionSnapshotsRemovedPaths(t *testing.T) {
	result := mustDiffSnapshots(t,
		`{"name":"board-a","obsolete":"stale"}`,
		`{"name":"board-a"}`,
	)

	if len(result.Removed) != 1 || result.Removed[0] != "obsolete" {
		t.Fatalf("removed = %v, want [obsolete]", result.Removed)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
	change := result.Changes[0]
	if change.Kind != diffKindRemoved || change.OldValue != "stale" {
		t.Fatalf("change = %+v, want removed with old value stale", change)
	}
	if change.NewValue != nil {
		t.Fatalf("removed change must not carry new value, got %v", change.NewValue)
	}
}

func TestDiffEntityVersionSnapshotsNestedModified(t *testing.T) {
	result := mustDiffSnapshots(t,
		`{"config":{"a":1,"deep":{"x":1,"y":"keep"}},"name":"same"}`,
		`{"config":{"a":2,"deep":{"x":1,"y":"keep"}},"name":"same"}`,
	)

	if len(result.Modified) != 1 || result.Modified[0] != "config.a" {
		t.Fatalf("modified = %v, want [config.a]", result.Modified)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
	// 未变的兄弟子树不得出现在路径列表里。
	containsPath(t, result.Modified, "config.a")
	for _, path := range result.Modified {
		if path == "config" || path == "config.deep" {
			t.Fatalf("unchanged subtree %q must not be reported", path)
		}
	}

	// 类型不同的同名路径按 modified 记在路径本身（json.Number vs string 不跨类型递归）。
	typeChange := mustDiffSnapshots(t,
		`{"config":{"a":1}}`,
		`{"config":{"a":"1"}}`,
	)
	if len(typeChange.Modified) != 1 || typeChange.Modified[0] != "config.a" {
		t.Fatalf("type-change modified = %v, want [config.a]", typeChange.Modified)
	}
}

func TestDiffEntityVersionSnapshotsArrayChanges(t *testing.T) {
	// 数组元素替换 + 尾部追加：下标作为路径段。
	grow := mustDiffSnapshots(t,
		`{"tags":["a","b"]}`,
		`{"tags":["a","c","d"]}`,
	)
	containsPath(t, grow.Modified, "tags.1")
	containsPath(t, grow.Added, "tags.2")
	if grow.Total != 2 {
		t.Fatalf("grow total = %d, want 2", grow.Total)
	}

	// 数组收缩：尾部下标报 removed。
	shrink := mustDiffSnapshots(t,
		`{"tags":["a","b"]}`,
		`{"tags":["a"]}`,
	)
	if len(shrink.Removed) != 1 || shrink.Removed[0] != "tags.1" {
		t.Fatalf("shrink removed = %v, want [tags.1]", shrink.Removed)
	}

	// 数组与对象互变：类型不同按 modified 记在数组路径本身，不下钻。
	reshaped := mustDiffSnapshots(t,
		`{"meta":["x"]}`,
		`{"meta":{"0":"x"}}`,
	)
	if len(reshaped.Modified) != 1 || reshaped.Modified[0] != "meta" {
		t.Fatalf("reshape modified = %v, want [meta]", reshaped.Modified)
	}
}

func TestDiffEntityVersionSnapshotsIdenticalAndRootScalar(t *testing.T) {
	same := `{"name":"a","config":{"x":1},"tags":[1,2]}`
	result := mustDiffSnapshots(t, same, same)
	if result.Total != 0 || len(result.Added) != 0 || len(result.Removed) != 0 || len(result.Modified) != 0 {
		t.Fatalf("identical snapshots must yield empty diff, got %+v", result)
	}

	// 根不是容器（或两侧类型不同）时记在 "$" 占位路径。
	root := mustDiffSnapshots(t, `"scalar-a"`, `"scalar-b"`)
	if len(root.Modified) != 1 || root.Modified[0] != rootDiffPath {
		t.Fatalf("root scalar modified = %v, want [$]", root.Modified)
	}

	// 数值按 json.Number 字面量比较：1 与 1.0 判异（保精度，宁可多报不误判相同）。
	numberLiteral := mustDiffSnapshots(t, `{"n":1}`, `{"n":1.0}`)
	if len(numberLiteral.Modified) != 1 || numberLiteral.Modified[0] != "n" {
		t.Fatalf("number literal modified = %v, want [n]", numberLiteral.Modified)
	}
}

func TestDiffEntityVersionSnapshotsRejectsInvalidJSON(t *testing.T) {
	if _, err := DiffEntityVersionSnapshots(`{broken`, `{"a":1}`); errCodeOf(err) != errcode.CodeParamError {
		t.Fatalf("invalid source snapshot err = %v, want param error", err)
	}
	if _, err := DiffEntityVersionSnapshots(`{"a":1}`, `not-json`); errCodeOf(err) != errcode.CodeParamError {
		t.Fatalf("invalid target snapshot err = %v, want param error", err)
	}
}

func TestEntityVersionDiffServiceScopesAndDiff(t *testing.T) {
	db := setupEntityVersionTestDB(t)
	if err := db.Create(&model.Board{ID: "board-diff", TenantID: "tenant-a", Name: "before"}).Error; err != nil {
		t.Fatalf("seed board: %v", err)
	}
	svc := &EntityVersionService{}

	v1, err := svc.CreateEntityVersion(&model.EntityVersionCreateReq{EntityType: "board", EntityID: "board-diff"}, entityVersionClaims())
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	if err := db.Model(&model.Board{}).Where("id = ?", "board-diff").Update("name", "after").Error; err != nil {
		t.Fatalf("mutate board: %v", err)
	}
	v2, err := svc.CreateEntityVersion(&model.EntityVersionCreateReq{EntityType: "board", EntityID: "board-diff"}, entityVersionClaims())
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}

	rsp, err := svc.DiffEntityVersion(v1.ID, v2.ID, entityVersionClaims())
	if err != nil {
		t.Fatalf("diff versions: %v", err)
	}
	if rsp.Source.ID != v1.ID || rsp.Target.ID != v2.ID {
		t.Fatalf("metadata mismatch: source %s target %s", rsp.Source.ID, rsp.Target.ID)
	}
	if rsp.Source.VersionNumber != 1 || rsp.Target.VersionNumber != 2 {
		t.Fatalf("version numbers = %d/%d, want 1/2", rsp.Source.VersionNumber, rsp.Target.VersionNumber)
	}
	// 看板名称变化必须报 name 路径 modified，且旧值/新值正确。
	containsPath(t, rsp.Diff.Modified, "name")
	var nameChange *model.EntityVersionDiffChange
	for i := range rsp.Diff.Changes {
		if rsp.Diff.Changes[i].Path == "name" {
			nameChange = &rsp.Diff.Changes[i]
			break
		}
	}
	if nameChange == nil || nameChange.OldValue != "before" || nameChange.NewValue != "after" {
		t.Fatalf("name change = %+v, want before->after", nameChange)
	}

	// 同一快照自己对比自己：无差异。
	self, err := svc.DiffEntityVersion(v2.ID, v2.ID, entityVersionClaims())
	if err != nil {
		t.Fatalf("self diff: %v", err)
	}
	if self.Diff.Total != 0 {
		t.Fatalf("self diff total = %d, want 0", self.Diff.Total)
	}
}

func TestEntityVersionDiffServiceFailsClosed(t *testing.T) {
	db := setupEntityVersionTestDB(t)
	if err := db.Create(&model.Board{ID: "board-diff-b", TenantID: "tenant-a", Name: "t"}).Error; err != nil {
		t.Fatalf("seed board: %v", err)
	}
	svc := &EntityVersionService{}
	version, err := svc.CreateEntityVersion(&model.EntityVersionCreateReq{EntityType: "board", EntityID: "board-diff-b"}, entityVersionClaims())
	if err != nil {
		t.Fatalf("create version: %v", err)
	}

	// nil claims 拒绝。
	if _, err := svc.DiffEntityVersion(version.ID, version.ID, nil); errCodeOf(err) != errcode.CodeNoPermission {
		t.Fatalf("nil claims err = %v, want permission error", err)
	}
	// 空租户拒绝。
	if _, err := svc.DiffEntityVersion(version.ID, version.ID, &utils.UserClaims{ID: "u", TenantID: ""}); errCodeOf(err) != errcode.CodeNoPermission {
		t.Fatalf("empty tenant err = %v, want permission error", err)
	}
	// 路径 id 缺失拒绝。
	if _, err := svc.DiffEntityVersion("", version.ID, entityVersionClaims()); errCodeOf(err) != errcode.CodeParamError {
		t.Fatalf("empty source id err = %v, want param error", err)
	}
	// 版本不存在拒绝。
	if _, err := svc.DiffEntityVersion(uuid.New(), version.ID, entityVersionClaims()); errCodeOf(err) != errcode.CodeNotFound {
		t.Fatalf("missing source err = %v, want not found", err)
	}
	if _, err := svc.DiffEntityVersion(version.ID, uuid.New(), entityVersionClaims()); errCodeOf(err) != errcode.CodeNotFound {
		t.Fatalf("missing target err = %v, want not found", err)
	}

	// 跨租户 fail-closed：tenant-b 读不到 tenant-a 的版本，diff 与详情一致返回 not found。
	tenantB := &utils.UserClaims{ID: "user-b", TenantID: "tenant-b", Authority: "TENANT_ADMIN"}
	if _, err := svc.DiffEntityVersion(version.ID, version.ID, tenantB); errCodeOf(err) != errcode.CodeNotFound {
		t.Fatalf("cross-tenant diff err = %v, want not found", err)
	}
}
