package service

import (
	"context"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupScadaTestDB 为每个用例建一份隔离的内存库，并恢复原 global.DB / query 默认库。
func setupScadaTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := global.DB
	dbName := "scada_" + strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.ScadaProject{},
		&model.ScadaDocument{},
		&model.ScadaDocumentVersion{},
		&model.ScadaControlAudit{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func newScadaSvc() *ScadaDocumentService { return &ScadaDocumentService{} }

func strp(s string) *string { return &s }

// TestScadaProjectCRUD 锁定门禁"项目 CRUD 不再返回 unsupported"：
// 此前看板没有项目这一层，项目 CRUD 无从实现。
func TestScadaProjectCRUD(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	created, err := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "产线 A", CreatedBy: strp("u1")})
	if err != nil || created == nil || created.ID == "" {
		t.Fatalf("CreateProject = (%v, %v)", created, err)
	}
	got, err := svc.GetProject(ctx, created.ID, "t1")
	if err != nil || got.Name != "产线 A" {
		t.Fatalf("GetProject = (%v, %v)", got, err)
	}

	// 跨租户读取必须表现为 not found，而不是 403（403 会泄漏 ID 存在性）。
	if _, err := svc.GetProject(ctx, created.ID, "t2"); err == nil || !strings.Contains(err.Error(), "100404") {
		t.Fatalf("cross-tenant GetProject error = %v, want not found", err)
	}
	if _, err := svc.GetProject(ctx, created.ID, "t2"); err == nil {
		t.Fatal("cross-tenant read must fail")
	}

	list, err := svc.ListProjects(ctx, "t1", 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListProjects = (%d rows, %v), want 1 row", len(list), err)
	}

	// 同名冲突：同租户拒绝，跨租户允许。
	if _, err := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "产线 A"}); err == nil {
		t.Fatal("duplicate project name in same tenant must be rejected")
	}
	if _, err := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t2", Name: "产线 A"}); err != nil {
		t.Fatalf("same name under another tenant must be allowed: %v", err)
	}

	if err := svc.DeleteProject(ctx, created.ID, "t1"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if err := svc.DeleteProject(ctx, created.ID, "t1"); err == nil {
		t.Fatal("deleting twice must report not found")
	}
}

// TestScadaCanvasSaveLoadRoundtrip 锁定门禁"画布保存/加载可往返"。
func TestScadaCanvasSaveLoadRoundtrip(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	project, err := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "产线 A"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	canvas := `{"widgets":[{"id":"w1","widget_type":"gauge","version":"1","layout":{"x":0,"y":0,"w":2,"h":2}}]}`
	doc, err := svc.CreateDocument(ctx, ScadaDocumentCreate{
		TenantID: "t1", ProjectID: project.ID, Name: "总览", Canvas: canvas,
	})
	if err != nil || doc == nil {
		t.Fatalf("CreateDocument = (%v, %v)", doc, err)
	}
	if doc.CurrentVersion != 1 || doc.Status != model.ScadaStatusDraft {
		t.Fatalf("new doc version=%d status=%q, want 1/DRAFT", doc.CurrentVersion, doc.Status)
	}

	loaded, err := svc.LoadDocument(ctx, doc.ID, "t1")
	if err != nil {
		t.Fatalf("LoadDocument: %v", err)
	}
	if loaded.JSONData == nil || *loaded.JSONData != canvas {
		t.Fatalf("roundtrip canvas = %v, want %q", loaded.JSONData, canvas)
	}

	// 再保存一次：版本推进，内容往返仍然一致。
	updated := `{"widgets":[{"id":"w1","widget_type":"gauge","version":"1"},{"id":"w2","widget_type":"chart","version":"1"}]}`
	saved, err := svc.SaveDocument(ctx, doc.ID, "t1", loaded.CurrentVersion, updated, strp("u1"))
	if err != nil || saved == nil {
		t.Fatalf("SaveDocument = (%v, %v)", saved, err)
	}
	if saved.CurrentVersion != 2 {
		t.Fatalf("version after save = %d, want 2", saved.CurrentVersion)
	}
	if saved.JSONData == nil || *saved.JSONData != updated {
		t.Fatalf("saved canvas = %v, want %q", saved.JSONData, updated)
	}
}

// TestScadaSaveDistinguishesConflictFromMissing 锁定：RowsAffected=0 必须区分
// "版本冲突"与"文档不存在"。统一报冲突会让"文档早被删了"伪装成"你保存太慢了"。
func TestScadaSaveDistinguishesConflictFromMissing(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	project, _ := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "p"})
	doc, _ := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: project.ID, Name: "d", Canvas: `{}`})

	// 用过期版本号保存 => 冲突（不是 not found）。
	_, err := svc.SaveDocument(ctx, doc.ID, "t1", 99, `{"a":1}`, nil)
	if err == nil || strings.Contains(err.Error(), "100404") {
		t.Fatalf("stale version error = %v, want a conflict, not not-found", err)
	}

	// 不存在的文档 => not found。
	_, err = svc.SaveDocument(ctx, "no-such-doc", "t1", 1, `{"a":1}`, nil)
	if err == nil || !strings.Contains(err.Error(), "100404") {
		t.Fatalf("missing doc error = %v, want not found", err)
	}

	// 跨租户保存 => not found（不泄漏存在性）。
	if _, err := svc.SaveDocument(ctx, doc.ID, "t2", doc.CurrentVersion, `{"a":1}`, nil); err == nil {
		t.Fatal("cross-tenant save must fail")
	}
}

// TestScadaPublishAndRollbackDoesNotRewriteHistory 锁定回滚语义：
// 回滚把目标版本内容写成新的草稿版本，不动已发布历史。
func TestScadaPublishAndRollbackDoesNotRewriteHistory(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	project, _ := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "p"})
	doc, _ := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: project.ID, Name: "d", Canvas: `{"v":1}`})

	// v1 发布
	published, err := svc.PublishDocument(ctx, doc.ID, "t1", strp("u1"))
	if err != nil || published.Status != model.ScadaStatusPublished {
		t.Fatalf("PublishDocument = (%v, %v)", published, err)
	}
	if published.PublishedVersion == nil || *published.PublishedVersion != 1 {
		t.Fatalf("published_version = %v, want 1", published.PublishedVersion)
	}
	// 同一版本重复发布必须拒绝：版本历史里出现重复项会让人误以为发生过两次变更。
	if _, err := svc.PublishDocument(ctx, doc.ID, "t1", strp("u1")); err == nil {
		t.Fatal("republishing the same version must be rejected")
	}

	// 改一版并发布 v2
	v2, err := svc.SaveDocument(ctx, doc.ID, "t1", published.CurrentVersion, `{"v":2}`, strp("u1"))
	if err != nil {
		t.Fatalf("save v2: %v", err)
	}
	if _, err := svc.PublishDocument(ctx, doc.ID, "t1", strp("u1")); err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	_ = v2

	// 回滚到 v1：产生新的草稿版本（v3），历史不动。
	rolled, err := svc.RollbackDocument(ctx, doc.ID, "t1", 1, strp("u1"))
	if err != nil || rolled == nil {
		t.Fatalf("RollbackDocument = (%v, %v)", rolled, err)
	}
	if rolled.JSONData == nil || *rolled.JSONData != `{"v":1}` {
		t.Fatalf("rolled back canvas = %v, want v1 content", rolled.JSONData)
	}
	if rolled.CurrentVersion < 3 {
		t.Fatalf("rollback must create a NEW draft version (>=3), got %d", rolled.CurrentVersion)
	}
	// 已发布版本仍是 v2：回滚不偷偷改发布状态。
	if rolled.PublishedVersion == nil || *rolled.PublishedVersion != 2 {
		t.Fatalf("published_version after rollback = %v, want still 2", rolled.PublishedVersion)
	}
	// 历史快照仍然是 2 条，没有被改写。
	versions, err := svc.ListDocumentVersions(ctx, doc.ID, "t1", 0)
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions = %d (%v), want 2 immutable snapshots", len(versions), err)
	}

	// 回滚到从未发布的版本必须拒绝。
	if _, err := svc.RollbackDocument(ctx, doc.ID, "t1", 99, strp("u1")); err == nil {
		t.Fatal("rollback to an unpublished version must be rejected")
	}
}

// TestScadaArchiveIsTerminal 锁定：归档是终态，不是装饰性标签。
func TestScadaArchiveIsTerminal(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	project, _ := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "p"})
	doc, _ := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: project.ID, Name: "d", Canvas: `{}`})

	archived, err := svc.ArchiveDocument(ctx, doc.ID, "t1", strp("u1"))
	if err != nil || archived.Status != model.ScadaStatusArchived {
		t.Fatalf("ArchiveDocument = (%v, %v)", archived, err)
	}
	if _, err := svc.SaveDocument(ctx, doc.ID, "t1", archived.CurrentVersion, `{"x":1}`, nil); err == nil {
		t.Fatal("archived document must reject saves")
	}
	if _, err := svc.PublishDocument(ctx, doc.ID, "t1", nil); err == nil {
		t.Fatal("archived document must reject publishing")
	}
}

// TestScadaRejectsOversizedCanvas 锁定：超限拒绝，绝不静默截断。
func TestScadaRejectsOversizedCanvas(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	project, _ := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "p"})
	oversized := `{"w":"` + strings.Repeat("x", model.ScadaMaxCanvasBytes) + `"}`
	if _, err := svc.CreateDocument(ctx, ScadaDocumentCreate{
		TenantID: "t1", ProjectID: project.ID, Name: "big", Canvas: oversized,
	}); err == nil {
		t.Fatal("oversized canvas must be rejected, not truncated")
	}

	// 非对象载荷同样拒绝：数组/标量无法承载画布布局结构。
	if _, err := svc.CreateDocument(ctx, ScadaDocumentCreate{
		TenantID: "t1", ProjectID: project.ID, Name: "arr", Canvas: `[1,2,3]`,
	}); err == nil {
		t.Fatal("non-object canvas must be rejected")
	}
}

func TestScadaDocumentBelongsToProjectTenant(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	project, _ := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "p"})
	// 用 t2 的身份在 t1 的项目下建文档：项目对 t2 不可见，必须 not found。
	if _, err := svc.CreateDocument(ctx, ScadaDocumentCreate{
		TenantID: "t2", ProjectID: project.ID, Name: "d", Canvas: `{}`,
	}); err == nil {
		t.Fatal("creating a document under another tenant's project must fail")
	}
	// 项目不存在时同样 not found。
	if _, err := svc.CreateDocument(ctx, ScadaDocumentCreate{
		TenantID: "t1", ProjectID: "missing", Name: "d", Canvas: `{}`,
	}); err == nil {
		t.Fatal("creating a document under a missing project must fail")
	}
}

// TestScadaDALSaveUsesConditionalUpdate 直接验证 DAL 的条件更新语义：
// 版本不匹配时一条都不该被改。
func TestScadaDALSaveUsesConditionalUpdate(t *testing.T) {
	db := setupScadaTestDB(t)
	ctx := context.Background()
	svc := newScadaSvc()

	project, _ := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "p"})
	doc, _ := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: project.ID, Name: "d", Canvas: `{"a":1}`})

	affected, err := dal.SaveScadaDocumentInTenant(doc.ID, "t1", 1, `{"a":2}`, nil)
	if err != nil || affected != 1 {
		t.Fatalf("matching version save = (%d, %v), want (1, nil)", affected, err)
	}
	affected, err = dal.SaveScadaDocumentInTenant(doc.ID, "t1", 1, `{"a":3}`, nil)
	if err != nil || affected != 0 {
		t.Fatalf("stale version save = (%d, %v), want (0, nil)", affected, err)
	}
	// 文档确实没被第二次写入改坏。
	var fresh model.ScadaDocument
	if err := db.Where("id = ?", doc.ID).First(&fresh).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if fresh.JSONData == nil || *fresh.JSONData != `{"a":2}` {
		t.Fatalf("canvas = %v, want the first save's content", fresh.JSONData)
	}
}
