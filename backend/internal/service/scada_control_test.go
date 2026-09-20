package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// controlExecutorStub 可控执行器。
type controlExecutorStub struct {
	calls []ControlExecution
	err   error
}

func (c *controlExecutorStub) Execute(ctx context.Context, exec ControlExecution) error {
	c.calls = append(c.calls, exec)
	return c.err
}

func setupControlTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := "scada_ctrl_" + strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.ScadaProject{}, &model.ScadaDocument{},
		&model.ScadaDocumentVersion{}, &model.ScadaControlAudit{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	prev := global.DB
	global.DB = db
	t.Cleanup(func() { global.DB = prev })
	return db
}

// ---------------------------------------------------------------------------
// 确认令牌
// ---------------------------------------------------------------------------

func TestConfirmationIssuerFailsClosedWithoutSecret(t *testing.T) {
	if _, err := NewConfirmationIssuer("", time.Minute); err != ErrConfirmationNoIssuer {
		t.Fatalf("empty secret error = %v, want %v", err, ErrConfirmationNoIssuer)
	}
	var nilIssuer *ConfirmationIssuer
	if _, err := nilIssuer.Issue("t", "d", "w", "c", "u", time.Now()); err != ErrConfirmationNoIssuer {
		t.Fatalf("nil issuer must refuse to issue, got %v", err)
	}
	if err := nilIssuer.Verify("token", "t", "d", "w", "c", "u", time.Now()); err != ErrConfirmationNoIssuer {
		t.Fatalf("nil issuer must refuse to verify, got %v", err)
	}
}

func TestConfirmationTokenIsBoundToEveryDimension(t *testing.T) {
	issuer, err := NewConfirmationIssuer("test-secret", time.Minute)
	if err != nil {
		t.Fatalf("new issuer: %v", err)
	}
	now := time.Date(2026, 9, 11, 22, 0, 0, 0, time.UTC)
	token, err := issuer.Issue("t1", "d1", "w1", "open_valve", "u1", now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := issuer.Verify(token, "t1", "d1", "w1", "open_valve", "u1", now); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}

	// 每个维度都必须参与绑定：换任何一个都要失效，
	// 否则令牌可以被挪用到别的控件/别的命令/别人身上。
	cases := []struct{ name, tenant, doc, widget, command, actor string; at time.Time; want error }{
		{name: "other tenant", tenant: "t2", doc: "d1", widget: "w1", command: "open_valve", actor: "u1", at: now, want: ErrConfirmationInvalid},
		{name: "other document", tenant: "t1", doc: "d2", widget: "w1", command: "open_valve", actor: "u1", at: now, want: ErrConfirmationInvalid},
		{name: "other widget", tenant: "t1", doc: "d1", widget: "w2", command: "open_valve", actor: "u1", at: now, want: ErrConfirmationInvalid},
		{name: "other command", tenant: "t1", doc: "d1", widget: "w1", command: "close_valve", actor: "u1", at: now, want: ErrConfirmationInvalid},
		{name: "other actor", tenant: "t1", doc: "d1", widget: "w1", command: "open_valve", actor: "u2", at: now, want: ErrConfirmationInvalid},
		{name: "expired", tenant: "t1", doc: "d1", widget: "w1", command: "open_valve", actor: "u1", at: now.Add(2 * time.Minute), want: ErrConfirmationExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := issuer.Verify(token, tc.tenant, tc.doc, tc.widget, tc.command, tc.actor, tc.at); err != tc.want {
				t.Fatalf("Verify(%s) = %v, want %v", tc.name, err, tc.want)
			}
		})
	}

	if err := issuer.Verify("", "t1", "d1", "w1", "open_valve", "u1", now); err != ErrConfirmationRequired {
		t.Fatalf("empty token = %v, want %v", err, ErrConfirmationRequired)
	}
	if err := issuer.Verify("garbage", "t1", "d1", "w1", "open_valve", "u1", now); err != ErrConfirmationInvalid {
		t.Fatalf("garbage token = %v, want %v", err, ErrConfirmationInvalid)
	}
}

// ---------------------------------------------------------------------------
// 完整控制链路
// ---------------------------------------------------------------------------

func newControlFixture(t *testing.T) (*ScadaControlService, *ScadaDocumentService, *controlExecutorStub, *model.ScadaDocument, *ConfirmationIssuer) {
	t.Helper()
	setupControlTestDB(t)

	docSvc := &ScadaDocumentService{}
	ctx := context.Background()
	project, err := docSvc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "p"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc, err := docSvc.CreateDocument(ctx, ScadaDocumentCreate{
		TenantID: "t1", ProjectID: project.ID, Name: "d", Canvas: `{}`,
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	reg := NewWidgetRegistry()
	if err := reg.Register(WidgetDefinition{
		Type: "valve", Version: "1", Schema: `{"type":"object"}`,
		Capabilities: []string{WidgetCapability2D},
		Commands: []CommandDefinition{
			{Name: "refresh", RequiresConfirmation: false},
			{Name: "open_valve", RequiresConfirmation: true},
		},
	}); err != nil {
		t.Fatalf("register widget: %v", err)
	}

	issuer, err := NewConfirmationIssuer("control-secret", time.Minute)
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	exec := &controlExecutorStub{}
	return NewScadaControlService(reg, issuer, exec), docSvc, exec, doc, issuer
}

func adminActor() ControlActor {
	return ControlActor{UserID: "u1", TenantID: "t1", Authority: "TENANT_ADMIN"}
}

// TestControlDenialsAreAudited 锁定审计的核心：**被拒绝的命令也要留痕**。
func TestControlDenialsAreAudited(t *testing.T) {
	svc, _, exec, doc, issuer := newControlFixture(t)
	ctx := context.Background()

	// 未注册的 Widget
	_, err := svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "unknown", Version: "1",
		Command: "open_valve", Actor: adminActor(),
	})
	if err == nil {
		t.Fatal("unknown widget must be rejected")
	}
	if len(exec.calls) != 0 {
		t.Fatal("rejected command must not reach the executor")
	}

	// Widget 未声明该命令
	_, err = svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "valve", Version: "1",
		Command: "self_destruct", Actor: adminActor(),
	})
	if err == nil {
		t.Fatal("undeclared command must be rejected")
	}

	// 需要确认但没有令牌
	_, err = svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "valve", Version: "1",
		Command: "open_valve", Actor: adminActor(),
	})
	if err == nil {
		t.Fatal("command requiring confirmation must be rejected without a token")
	}
	_ = issuer

	// 权限不足
	_, err = svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "valve", Version: "1",
		Command: "refresh", Actor: ControlActor{UserID: "u9", TenantID: "t1", Authority: "VIEWER"},
	})
	if err == nil {
		t.Fatal("actor without control authority must be rejected")
	}

	// 跨租户操作者
	_, err = svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "valve", Version: "1",
		Command: "refresh", Actor: ControlActor{UserID: "u9", TenantID: "t2", Authority: "TENANT_ADMIN"},
	})
	if err == nil {
		t.Fatal("cross-tenant actor must be rejected")
	}

	// 上述全部拒绝都必须落审计，否则"谁在反复尝试越权控制"从记录里消失。
	audits, aerr := dal.ListScadaControlAudits("t1", doc.ID, 0)
	if aerr != nil {
		t.Fatalf("list audits: %v", aerr)
	}
	if len(audits) != 5 {
		t.Fatalf("audit count = %d, want 5 (every denial recorded)", len(audits))
	}
	for _, a := range audits {
		if a.Outcome != model.ControlOutcomeDenied {
			t.Fatalf("audit outcome = %q, want denied", a.Outcome)
		}
	}
}

// TestControlSuccessPathAuditsPendingThenSuccess 锁定审计时序：
// 先落 pending 再执行，成功后推进为 success。
func TestControlSuccessPathAuditsPendingThenSuccess(t *testing.T) {
	svc, _, exec, doc, issuer := newControlFixture(t)
	ctx := context.Background()
	now := time.Now()

	token, err := issuer.Issue("t1", doc.ID, "w1", "open_valve", "u1", now)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	outcome, err := svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "valve", Version: "1",
		Command: "open_valve", Actor: adminActor(), ConfirmationToken: token,
		Params: map[string]interface{}{"value": 1},
	})
	if err != nil || outcome != model.ControlOutcomeSuccess {
		t.Fatalf("ExecuteControl = (%q, %v), want success", outcome, err)
	}
	if len(exec.calls) != 1 || exec.calls[0].DeviceID != "dev1" {
		t.Fatalf("executor calls = %+v, want exactly one targeting dev1", exec.calls)
	}

	audits, err := dal.ListScadaControlAudits("t1", doc.ID, 0)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("audit count = %d, want 1 (pending updated in place, not appended)", len(audits))
	}
	if audits[0].Outcome != model.ControlOutcomeSuccess {
		t.Fatalf("audit outcome = %q, want success", audits[0].Outcome)
	}
	if audits[0].ConfirmationToken != token {
		t.Fatal("audit must retain the confirmation token actually used")
	}
}

// TestControlWithoutExecutorNeverSucceeds 锁定：执行器未接线时记 failed，绝不报 success。
func TestControlWithoutExecutorNeverSucceeds(t *testing.T) {
	setupControlTestDB(t)
	ctx := context.Background()
	docSvc := &ScadaDocumentService{}
	project, _ := docSvc.CreateProject(ctx, ScadaProjectCreate{TenantID: "t1", Name: "p"})
	doc, _ := docSvc.CreateDocument(ctx, ScadaDocumentCreate{TenantID: "t1", ProjectID: project.ID, Name: "d", Canvas: `{}`})

	reg := NewWidgetRegistry()
	_ = reg.Register(WidgetDefinition{
		Type: "valve", Version: "1", Schema: `{"type":"object"}`,
		Capabilities: []string{WidgetCapability2D},
		Commands:     []CommandDefinition{{Name: "refresh"}},
	})
	// executor 为 nil
	svc := NewScadaControlService(reg, nil, nil)

	outcome, err := svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "valve", Version: "1",
		Command: "refresh", Actor: adminActor(),
	})
	if err == nil {
		t.Fatal("missing executor must not report success")
	}
	if outcome != model.ControlOutcomeFailed {
		t.Fatalf("outcome = %q, want failed", outcome)
	}
	audits, _ := dal.ListScadaControlAudits("t1", doc.ID, 0)
	if len(audits) != 1 || audits[0].Outcome != model.ControlOutcomeFailed {
		t.Fatalf("audits = %+v, want one failed record", audits)
	}
}

// TestControlRecordsFailureFromExecutor 锁定：执行失败记 failed 并留下原因。
func TestControlRecordsFailureFromExecutor(t *testing.T) {
	svc, _, exec, doc, _ := newControlFixture(t)
	exec.err = errors.New("device offline")
	ctx := context.Background()

	outcome, err := svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "valve", Version: "1",
		Command: "refresh", Actor: adminActor(),
	})
	if err == nil || outcome != model.ControlOutcomeFailed {
		t.Fatalf("ExecuteControl = (%q, %v), want failed", outcome, err)
	}
	audits, _ := dal.ListScadaControlAudits("t1", doc.ID, 0)
	if len(audits) != 1 || audits[0].Outcome != model.ControlOutcomeFailed {
		t.Fatalf("audits = %+v, want one failed record", audits)
	}
	if audits[0].Detail == nil || !strings.Contains(*audits[0].Detail, "device offline") {
		t.Fatalf("audit detail = %v, want the underlying error recorded", audits[0].Detail)
	}
}

// TestControlRejectsArchivedDocument 锁定：归档文档拒绝控制。
func TestControlRejectsArchivedDocument(t *testing.T) {
	svc, docSvc, exec, doc, _ := newControlFixture(t)
	ctx := context.Background()

	if _, err := docSvc.ArchiveDocument(ctx, doc.ID, "t1", strp("u1")); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, err := svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DeviceID: "dev1", DocumentID: doc.ID,
		WidgetID: "w1", WidgetType: "valve", Version: "1",
		Command: "refresh", Actor: adminActor(),
	})
	if err == nil {
		t.Fatal("archived document must reject control commands")
	}
	if len(exec.calls) != 0 {
		t.Fatal("archived document must not reach the executor")
	}
	audits, _ := dal.ListScadaControlAudits("t1", doc.ID, 0)
	if len(audits) != 1 || audits[0].Outcome != model.ControlOutcomeDenied {
		t.Fatalf("audits = %+v, want one denied record", audits)
	}
}

func TestControlRejectsIncompleteRequest(t *testing.T) {
	svc, _, _, _, _ := newControlFixture(t)
	ctx := context.Background()

	if _, err := svc.ExecuteControl(ctx, ControlRequest{TenantID: "t1", DocumentID: "d"}); err == nil {
		t.Fatal("request without actor must be rejected")
	}
	// 缺少目标设备：没有目标的命令无从执行。
	_, err := svc.ExecuteControl(ctx, ControlRequest{
		TenantID: "t1", DocumentID: "d1", WidgetID: "w1",
		WidgetType: "valve", Version: "1", Command: "refresh", Actor: adminActor(),
	})
	if err == nil {
		t.Fatal("request without a target device must be rejected")
	}
}
