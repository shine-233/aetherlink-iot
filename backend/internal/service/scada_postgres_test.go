package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// scadaPGTenant 本测试专用租户，便于清理。
const scadaPGTenant = "pg-scada-verify"

// setupScadaPostgres 连接真实 PostgreSQL。
//
// 为什么必须有这份真库证据：同包的其余用例跑在 SQLite 内存库上，而 SQLite 不执行
// 迁移里定义的 CHECK / 复合唯一约束 / jsonb 语义 / ON CONFLICT 行为。
// 只靠 SQLite 绿就宣称"已验证"，正是本项目此前踩过的坑（用 SQLite 冒充 PG 证据）。
func setupScadaPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; SCADA PostgreSQL verification skipped")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	for _, table := range []string{
		model.TableNameScadaProject, model.TableNameScadaDocument,
		model.TableNameScadaDocumentVersion, model.TableNameScadaControlAudit,
		model.TableNamePushDeviceRegistration, model.TableNamePushDelivery,
	} {
		var count int64
		if err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?",
			table,
		).Scan(&count).Error; err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s is missing; apply backend/sql/88.sql to the test database first", table)
		}
	}

	prev := global.DB
	global.DB = db
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM "+model.TableNamePushDelivery+" WHERE tenant_id = ?", scadaPGTenant).Error
		_ = db.Exec("DELETE FROM "+model.TableNamePushDeviceRegistration+" WHERE tenant_id = ?", scadaPGTenant).Error
		_ = db.Exec("DELETE FROM "+model.TableNameScadaControlAudit+" WHERE tenant_id = ?", scadaPGTenant).Error
		global.DB = prev
	})
	return db
}

// TestScadaDocumentAgainstPostgres 真库上的项目/文档往返与乐观并发。
func TestScadaDocumentAgainstPostgres(t *testing.T) {
	db := setupScadaPostgres(t)
	ctx := context.Background()
	svc := &ScadaDocumentService{}

	// 每个用例用唯一项目名，避免与历史数据互撞。
	suffix := time.Now().Format("150405.000000000")
	project, err := svc.CreateProject(ctx, ScadaProjectCreate{
		TenantID: scadaPGTenant, Name: "pg-project-" + suffix,
	})
	if err != nil || project == nil || project.ID == "" {
		t.Fatalf("CreateProject = (%v, %v)", project, err)
	}

	// 复合唯一约束在真库上生效：同租户同名必须被拒。
	if _, err := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: scadaPGTenant, Name: project.Name}); err == nil {
		t.Fatal("duplicate project name must be rejected by the real unique constraint")
	}
	// 跨租户同名必须允许（唯一约束含 tenant_id）。
	otherTenant := scadaPGTenant + "-b"
	if _, err := svc.CreateProject(ctx, ScadaProjectCreate{TenantID: otherTenant, Name: project.Name}); err != nil {
		t.Fatalf("same name under another tenant must be allowed: %v", err)
	}
	defer func() {
		_ = db.Exec("DELETE FROM "+model.TableNameScadaProject+" WHERE tenant_id = ?", otherTenant).Error
	}()

	canvas := `{"widgets":[{"id":"w1","widget_type":"gauge","version":"1"}]}`
	doc, err := svc.CreateDocument(ctx, ScadaDocumentCreate{
		TenantID: scadaPGTenant, ProjectID: project.ID, Name: "pg-doc-" + suffix, Canvas: canvas,
	})
	if err != nil || doc == nil {
		t.Fatalf("CreateDocument = (%v, %v)", doc, err)
	}

	// jsonb 往返：真库会以 jsonb 存储并可能重排键，内容语义必须保持一致。
	loaded, err := svc.LoadDocument(ctx, doc.ID, scadaPGTenant)
	if err != nil || loaded.JSONData == nil || !strings.Contains(*loaded.JSONData, `"w1"`) {
		t.Fatalf("roundtrip on postgres = (%v, %v)", loaded, err)
	}

	// 乐观并发：过期版本一条都不改。
	affected, err := dal.SaveScadaDocumentInTenant(doc.ID, scadaPGTenant, loaded.CurrentVersion+7, `{"x":1}`, nil)
	if err != nil || affected != 0 {
		t.Fatalf("stale save affected = %d (%v), want 0", affected, err)
	}
	affected, err = dal.SaveScadaDocumentInTenant(doc.ID, scadaPGTenant, loaded.CurrentVersion, `{"x":1}`, nil)
	if err != nil || affected != 1 {
		t.Fatalf("matching save affected = %d (%v), want 1", affected, err)
	}
}

// TestScadaControlAuditAgainstPostgres 真库上的审计时序：pending → success。
func TestScadaControlAuditAgainstPostgres(t *testing.T) {
	setupScadaPostgres(t)
	ctx := context.Background()
	docSvc := &ScadaDocumentService{}

	suffix := time.Now().Format("150405.000000000")
	project, err := docSvc.CreateProject(ctx, ScadaProjectCreate{TenantID: scadaPGTenant, Name: "pg-audit-p-" + suffix})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc, err := docSvc.CreateDocument(ctx, ScadaDocumentCreate{
		TenantID: scadaPGTenant, ProjectID: project.ID, Name: "pg-audit-d-" + suffix, Canvas: `{}`,
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	reg := NewWidgetRegistry()
	if err := reg.Register(WidgetDefinition{
		Type: "pgvalve", Version: "1", Schema: `{"type":"object"}`,
		Capabilities: []string{WidgetCapability2D},
		Commands:     []CommandDefinition{{Name: "refresh"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	exec := &controlExecutorStub{}
	svc := NewScadaControlService(reg, nil, exec)

	outcome, err := svc.ExecuteControl(ctx, ControlRequest{
		TenantID: scadaPGTenant, DeviceID: "pg-dev", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "pgvalve", Version: "1",
		Command: "refresh", Actor: ControlActor{UserID: "u1", TenantID: scadaPGTenant, Authority: "TENANT_ADMIN"},
	})
	if err != nil || outcome != model.ControlOutcomeSuccess {
		t.Fatalf("ExecuteControl = (%q, %v), want success", outcome, err)
	}

	// 审计必须只有一条且已推进到 success（pending 原地更新，不是再插一条）。
	audits, err := dal.ListScadaControlAudits(scadaPGTenant, doc.ID, 0)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(audits))
	}
	if audits[0].Outcome != model.ControlOutcomeSuccess {
		t.Fatalf("audit outcome = %q, want success", audits[0].Outcome)
	}

	// outcome CHECK 在真库上生效：非法值必须被拒。
	if err := global.DB.Exec(
		"INSERT INTO "+model.TableNameScadaControlAudit+
			" (id,tenant_id,document_id,widget_id,command,actor_user_id,confirmation_token,outcome) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		model.ScadaControlAudit{}.ID+"x", scadaPGTenant, doc.ID, "w", "c", "u", "tok", "maybe",
	).Error; err == nil {
		t.Fatal("illegal outcome must be rejected by the CHECK constraint")
	}
}

// TestPushRetryAgainstPostgres 真库上的 upsert 与重试到终态。
func TestPushRetryAgainstPostgres(t *testing.T) {
	db := setupScadaPostgres(t)
	ctx := context.Background()

	// 幂等登记：同一 (租户,用户,平台,令牌) 重复登记不产生第二行，
	// 否则一次推送会被放大成 N 条。
	token := "pg-token-" + time.Now().Format("150405.000000000")
	for i := 0; i < 3; i++ {
		reg := &model.PushDeviceRegistration{
			ID: uuid.New().String(), TenantID: scadaPGTenant, UserID: "u1",
			Platform: model.PushPlatformAndroid, Token: token, Provider: "fcm", Enabled: true,
		}
		if err := dal.UpsertPushRegistration(reg); err != nil {
			t.Fatalf("upsert #%d: %v", i, err)
		}
	}
	var regCount int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM "+model.TableNamePushDelivery+" WHERE tenant_id = ?", scadaPGTenant,
	).Scan(&regCount).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	var rows int64
	if err := db.Model(&model.PushDeviceRegistration{}).
		Where("tenant_id = ? AND token = ?", scadaPGTenant, token).Count(&rows).Error; err != nil {
		t.Fatalf("count registrations: %v", err)
	}
	if rows != 1 {
		t.Fatalf("registration rows = %d, want 1 (upsert must not duplicate)", rows)
	}

	// 推送失败 → 重试 → 用尽转 dead 终态。
	provider := &stubPushProvider{name: "fcm", platform: []string{"android"}, err: errors.New("upstream 503")}
	registry := NewPushProviderRegistry()
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	pushSvc := NewPushService(registry, PushRetryPolicy{MaxAttempts: 2, BaseBackoff: time.Second, MaxBackoff: time.Second})

	deliveries, err := pushSvc.Enqueue(ctx, scadaPGTenant, "u1", "告警", "设备离线", nil, time.Now())
	if err != nil || len(deliveries) == 0 {
		t.Fatalf("Enqueue = (%d, %v), want at least one delivery", len(deliveries), err)
	}

	// 第 1 次：failed（仍可重试）
	if _, _, err := pushSvc.DeliverDue(ctx, time.Now(), 10); err != nil {
		t.Fatalf("deliver #1: %v", err)
	}
	got, err := dal.GetPushDeliveryInTenant(deliveries[0].ID, scadaPGTenant)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != model.PushStatusFailed || got.AttemptCount != 1 {
		t.Fatalf("after #1: status=%q attempts=%d, want failed/1", got.Status, got.AttemptCount)
	}
	if got.LastError == nil || !strings.Contains(*got.LastError, "503") {
		t.Fatalf("last_error = %v, want the underlying failure recorded", got.LastError)
	}

	// 第 2 次：用尽 → dead 终态，且不再安排重试时间。
	if _, _, err := pushSvc.DeliverDue(ctx, time.Now().Add(time.Second), 10); err != nil {
		t.Fatalf("deliver #2: %v", err)
	}
	got, err = dal.GetPushDeliveryInTenant(deliveries[0].ID, scadaPGTenant)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != model.PushStatusDead {
		t.Fatalf("after #2: status = %q, want dead", got.Status)
	}
	if got.NextAttemptAt != nil {
		t.Fatal("terminal delivery must not carry a retry time; otherwise it would be picked up again")
	}
	// 终态不得再被捞起。
	due, err := dal.ListRetryablePushDeliveries(time.Now().Add(time.Hour), 50)
	if err != nil {
		t.Fatalf("list retryable: %v", err)
	}
	for _, d := range due {
		if d.ID == deliveries[0].ID {
			t.Fatal("dead delivery must never be picked up again")
		}
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2 (exactly MaxAttempts)", provider.calls)
	}
}

// TestPushDeliveryStatusCheckAgainstPostgres 真库上的状态 CHECK。
func TestPushDeliveryStatusCheckAgainstPostgres(t *testing.T) {
	setupScadaPostgres(t)
	if err := global.DB.Exec(
		"INSERT INTO "+model.TableNamePushDelivery+" (id,tenant_id,user_id,title,body,status) VALUES (?,?,?,?,?,?)",
		"pg-bad-status", scadaPGTenant, "u1", "t", "b", "SENT-ISH",
	).Error; err == nil {
		t.Fatal("illegal push status must be rejected by the CHECK constraint")
	}
	_ = global.DB.Exec("DELETE FROM "+model.TableNamePushDelivery+" WHERE id = ?", "pg-bad-status").Error
}
