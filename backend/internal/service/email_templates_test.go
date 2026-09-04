// 文件用途：锁定邮件模板变量白名单、作用域权限和原文包装行为。
// 核心逻辑：使用纯函数验证模板渲染，不依赖 SMTP、PostgreSQL 或外部服务。
// 关键注意事项：本文件不证明真实迁移、模板 CRUD 或告警发送运行链路。
package service

import (
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"
)

func TestRenderAlarmEmailTemplatePairUsesOnlyDocumentedData(t *testing.T) {
	data := buildAlarmEmailTemplateData("High temperature", "T1 exceeded 8 C", "tenant-1", []string{"dev-1", "dev-2"}, time.Unix(0, 0))
	subject, body, err := renderAlarmEmailTemplatePair(
		"[{{.TenantID}}] {{.Subject}}",
		"{{.Message}}\nDevices: {{.DeviceIDs}} ({{.DeviceCount}})\n{{.SentAt}}",
		data,
	)
	if err != nil {
		t.Fatalf("render template: %v", err)
	}
	if subject != "[tenant-1] High temperature" || !strings.Contains(body, "dev-1, dev-2 (2)") {
		t.Fatalf("unexpected rendered template: subject=%q body=%q", subject, body)
	}
	if _, _, err := renderAlarmEmailTemplatePair("{{.Unknown}}", "{{.Message}}", data); err == nil {
		t.Fatal("unknown template field should fail")
	}
	if _, _, err := renderAlarmEmailTemplatePair("{{printf \"%s\" .Subject}}", "{{.Message}}", data); err == nil {
		t.Fatal("template functions should fail")
	}
	injected := buildAlarmEmailTemplateData("High\r\nBcc: attacker@example.invalid", "body", "tenant-1", nil, time.Unix(0, 0))
	if _, _, err := renderAlarmEmailTemplatePair("{{.Subject}}", "{{.Message}}", injected); err == nil {
		t.Fatal("rendered subject line breaks should fail")
	}
}

func TestValidateAlarmEmailTemplateRejectsBlankName(t *testing.T) {
	req := &model.EmailTemplateUpsertReq{
		Name:            "   ",
		SubjectTemplate: "Alarm",
		BodyTemplate:    "{{.Message}}",
		Enabled:         true,
	}
	if err := validateAlarmEmailTemplate(req); err == nil {
		t.Fatal("blank template name should fail")
	}
}

func TestPreviewEmailTemplateRejectsNilRequest(t *testing.T) {
	_, err := (&NotificationServicesConfig{}).PreviewEmailTemplate(nil, &utils.UserClaims{Authority: constant.SYS_ADMIN})
	if err == nil {
		t.Fatal("nil preview request should fail")
	}
}

func TestEmailTemplateScopeForClaims(t *testing.T) {
	if scope, err := emailTemplateScopeForClaims(&utils.UserClaims{Authority: constant.SYS_ADMIN}); err != nil || scope != "" {
		t.Fatalf("system scope = %q, err=%v", scope, err)
	}
	if scope, err := emailTemplateScopeForClaims(&utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}); err != nil || scope != "tenant-1" {
		t.Fatalf("tenant scope = %q, err=%v", scope, err)
	}
	if _, err := emailTemplateScopeForClaims(&utils.UserClaims{Authority: constant.TENANT_USER, TenantID: "tenant-1"}); err == nil {
		t.Fatal("tenant user should not manage email templates")
	}
}

// TestEmailTemplateListScopes 管理列表读作用域：空租户(SYS_ADMIN 平台默认模板)映射 [""]，
// 非空租户回退 self-only（无层级链接时至少包含自身，等价旧单租户）。
func TestEmailTemplateListScopes(t *testing.T) {
	if got := emailTemplateListScopes(""); len(got) != 1 || got[0] != "" {
		t.Fatalf("platform scope = %#v, want [\"\"]", got)
	}
	if got := emailTemplateListScopes("tenant-1"); len(got) != 1 || got[0] != "tenant-1" {
		t.Fatalf("tenant fallback scope = %#v, want [tenant-1]", got)
	}
}
