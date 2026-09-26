// 文件用途：TP-22 画布壳层契约在 SCADA 文档服务写路径上的接线测试。
// 核心逻辑：验证创建/保存/发布/回滚四条写路径都执行 display_mode 契约，
// 且旧画布（无 display_mode 字段）零改动通过——向后兼容是本契约的硬要求。
// 关键注意事项：服务层用内存 SQLite 起隔离库，跑在 global.DB 上（与 dal/scada.go
// 的取库方式一致）；用例只锁行为不锁实现，改错误文案时同步调整断言子串。
// 重构建议：若后续新增画布写路径（如模板实例化），必须同步在此表加一行，
// 否则该路径会绕过壳层契约。
package service

import (
	"context"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
)

// fixed1080 合法画布样例（snake_case 别名键，与缺口任务书原文写法一致）。
const canvasFixed1080OK = `{"schemaVersion":1,"display_mode":"fixed1080","width":1920,"height":1080,"nodes":[]}`

// TestScadaDisplayModeCreateAndSave 锁定创建/保存路径上的 display_mode 契约。
func TestScadaDisplayModeCreateAndSave(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	created, err := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "大屏项目"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	t.Run("fixed1080 合法画布可创建", func(t *testing.T) {
		doc, err := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: created.ID, Name: "大屏", Canvas: canvasFixed1080OK})
		if err != nil || doc == nil {
			t.Fatalf("CreateDocument(fixed1080) = (%v, %v)", doc, err)
		}
	})

	t.Run("fixed1080 尺寸不符拒绝", func(t *testing.T) {
		bad := `{"display_mode":"fixed1080","width":1280,"height":720,"nodes":[]}`
		_, err := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: created.ID, Name: "坏尺寸", Canvas: bad})
		if err == nil || !strings.Contains(err.Error(), "1920") {
			t.Fatalf("err = %v, want fixed1080 size contract error", err)
		}
	})

	t.Run("未知显示模式拒绝", func(t *testing.T) {
		bad := `{"displayMode":"curved","width":1920,"height":1080}`
		_, err := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: created.ID, Name: "坏模式", Canvas: bad})
		if err == nil || !strings.Contains(err.Error(), "display_mode") {
			t.Fatalf("err = %v, want display_mode oneof error", err)
		}
	})

	t.Run("双键冲突拒绝", func(t *testing.T) {
		bad := `{"displayMode":"responsive","display_mode":"fixed1080","width":1920,"height":1080}`
		if _, err := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: created.ID, Name: "双键", Canvas: bad}); err == nil {
			t.Fatal("conflicting displayMode/display_mode must be rejected")
		}
	})

	t.Run("旧画布无该字段仍可保存", func(t *testing.T) {
		doc, err := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: created.ID, Name: "旧画布", Canvas: `{"schemaVersion":1,"width":1280,"height":720,"nodes":[]}`})
		if err != nil {
			t.Fatalf("legacy canvas save must pass: %v", err)
		}
		// 再次保存（乐观并发）同样通过。
		if _, err := svc.SaveDocument(ctx, doc.ID, "t1", doc.CurrentVersion, `{"schemaVersion":1,"width":1280,"height":720,"nodes":[{"id":"n1","kind":"shape","ref":"rect","x":1,"y":1,"width":10,"height":10}]}`, nil); err != nil {
			t.Fatalf("legacy canvas re-save must pass: %v", err)
		}
	})

	t.Run("保存路径切换到非法模式拒绝", func(t *testing.T) {
		doc, err := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: created.ID, Name: "切换", Canvas: canvasFixed1080OK})
		if err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
		bad := `{"display_mode":"fixed1080","width":1920,"height":900,"nodes":[]}`
		if _, err := svc.SaveDocument(ctx, doc.ID, "t1", doc.CurrentVersion, bad, nil); err == nil {
			t.Fatal("save must enforce the fixed1080 size contract")
		}
	})
}

// TestScadaDisplayModePublishAndRollback 锁定发布/回滚路径：契约是全写路径底线。
func TestScadaDisplayModePublishAndRollback(t *testing.T) {
	setupScadaTestDB(t)
	svc := newScadaSvc()
	ctx := context.Background()

	project, err := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "发布项目"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	t.Run("fixed1080 可发布且回滚", func(t *testing.T) {
		doc, err := svc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: project.ID, Name: "投屏大屏", Canvas: canvasFixed1080OK})
		if err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
		published, err := svc.PublishDocument(ctx, doc.ID, "t1", nil)
		if err != nil || published.PublishedVersion == nil {
			t.Fatalf("PublishDocument = (%v, %v)", published, err)
		}
		// 草稿改成 responsive 后回滚：快照内容（fixed1080）写回新草稿必须再次过契约。
		if _, err := svc.SaveDocument(ctx, doc.ID, "t1", published.CurrentVersion, `{"display_mode":"responsive","width":1280,"height":720,"nodes":[]}`, nil); err != nil {
			t.Fatalf("SaveDocument: %v", err)
		}
		rolled, err := svc.RollbackDocument(ctx, doc.ID, "t1", 1, nil)
		if err != nil {
			t.Fatalf("RollbackDocument: %v", err)
		}
		if !strings.Contains(stringPtrValue(rolled.JSONData), "fixed1080") {
			t.Fatalf("rollback content = %s, want fixed1080 snapshot", stringPtrValue(rolled.JSONData))
		}
	})

	t.Run("坏壳层草稿拒绝发布", func(t *testing.T) {
		// 直接经 DAL 种一份"契约上线前"的坏画布，模拟存量数据：
		// 发布必须 fail-closed，把坏模式冻进不可变历史比拒绝发布更糟。
		bad := `{"display_mode":"fixed4k","width":3840,"height":2160,"nodes":[]}`
		legacy := &model.ScadaDocument{
			ID:             "legacy-bad-doc",
			TenantID:       "t1",
			ProjectID:      project.ID,
			Name:           "存量坏稿",
			Status:         model.ScadaStatusDraft,
			CurrentVersion: 1,
			JSONData:       &bad,
		}
		if err := dal.CreateScadaDocument(legacy); err != nil {
			t.Fatalf("seed legacy doc: %v", err)
		}
		if _, err := svc.PublishDocument(ctx, legacy.ID, "t1", nil); err == nil || !strings.Contains(err.Error(), "display_mode") {
			t.Fatalf("err = %v, want display_mode contract error on publish", err)
		}
	})
}
