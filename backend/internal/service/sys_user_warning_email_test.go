// 文件用途：验证用户告警邮箱归一化和 additional_info 解析。
// 核心逻辑：覆盖邮箱去重、格式校验、JSON 解析和空值回退。
// 关键注意事项：告警邮箱会接收设备告警，测试需保证无效地址被拒绝且历史配置不被误清空。
// 重构建议：抽出邮箱集合 value object，补齐跨租户读取、更新事务和通知发送边界。
package service

import (
	"reflect"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func TestGetWarningEmailsHidesTenantGlobalRecipientsFromOrdinaryUsers(t *testing.T) {
	emails, err := (&User{}).GetWarningEmails(&utils.UserClaims{
		ID:        "user-1",
		TenantID:  "tenant-1",
		Authority: constant.TENANT_USER,
	})
	if err != nil {
		t.Fatalf("GetWarningEmails returned error: %v", err)
	}
	if len(emails) != 0 {
		t.Fatalf("ordinary user warning emails = %#v, want hidden tenant recipients", emails)
	}
}

func TestNormalizeWarningEmails(t *testing.T) {
	got, err := normalizeWarningEmails([]string{" Ops@Example.com ", "ops@example.com", "", "Other@Example.com"})
	if err != nil {
		t.Fatalf("normalizeWarningEmails returned error: %v", err)
	}
	want := []string{"ops@example.com", "other@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeWarningEmails = %#v, want %#v", got, want)
	}
}

func TestNormalizeWarningEmailsRejectsInvalid(t *testing.T) {
	_, err := normalizeWarningEmails([]string{"not-an-email"})
	assertErrcodeError(t, err, "invalid warning email", errcode.CodeParamError, "emails contains an invalid email address")
}

func TestWarningEmailsFromAdditionalInfo(t *testing.T) {
	raw := StringPtr(`{"warning_emails":["A@Example.com","a@example.com","b@example.com"]}`)
	got := warningEmailsFromAdditionalInfo(raw)
	want := []string{"a@example.com", "b@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warningEmailsFromAdditionalInfo = %#v, want %#v", got, want)
	}

	raw = StringPtr(`{"warning_emails":"c@example.com, D@example.com"}`)
	got = warningEmailsFromAdditionalInfo(raw)
	want = []string{"c@example.com", "d@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warningEmailsFromAdditionalInfo string = %#v, want %#v", got, want)
	}
}

func TestWarningEmailsFromUserPrefersConfiguredWarningEmails(t *testing.T) {
	raw := StringPtr(`{"warning_emails":["ops@example.com"]}`)
	user := &model.User{Email: "registered@example.com", AdditionalInfo: raw}

	got := warningEmailsFromUser(user)
	want := []string{"ops@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warningEmailsFromUser = %#v, want %#v", got, want)
	}
}

func TestWarningEmailsFromUserFallsBackToRegisteredEmail(t *testing.T) {
	user := &model.User{Email: " Registered@Example.com "}

	got := warningEmailsFromUser(user)
	want := []string{"registered@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warningEmailsFromUser fallback = %#v, want %#v", got, want)
	}
}

func TestWarningEmailsFromUserIgnoresInvalidRegisteredEmail(t *testing.T) {
	user := &model.User{Email: "not-an-email"}

	got := warningEmailsFromUser(user)
	if len(got) != 0 {
		t.Fatalf("warningEmailsFromUser invalid fallback = %#v, want empty", got)
	}
}

func TestPickWarningEmailOwnerUserPrefersTenantAdminForTenantScopedClaims(t *testing.T) {
	claims := &utils.UserClaims{ID: "user-1", TenantID: "tenant-1"}
	tenantAdmin := &model.User{ID: "admin-1", Email: "admin@example.com"}
	currentUser := &model.User{ID: "user-1", Email: "user@example.com"}

	got := pickWarningEmailOwnerUser(claims, tenantAdmin, currentUser)
	if got != tenantAdmin {
		t.Fatalf("pickWarningEmailOwnerUser = %#v, want tenant admin", got)
	}
}

func TestPickWarningEmailOwnerUserReturnsNilWithoutTenantAdminInTenantScope(t *testing.T) {
	claims := &utils.UserClaims{ID: "user-1", TenantID: "tenant-1"}

	got := pickWarningEmailOwnerUser(claims, nil, &model.User{ID: "user-1", Email: "user@example.com"})
	if got != nil {
		t.Fatalf("pickWarningEmailOwnerUser missing tenant admin = %#v, want nil", got)
	}
}

func TestPickWarningEmailOwnerUserUsesCurrentUserOutsideTenantScope(t *testing.T) {
	currentUser := &model.User{ID: "sys-1", Email: "sys@example.com"}

	got := pickWarningEmailOwnerUser(&utils.UserClaims{ID: "sys-1"}, &model.User{ID: "tenant-admin"}, currentUser)
	if got != currentUser {
		t.Fatalf("pickWarningEmailOwnerUser non-tenant = %#v, want current user", got)
	}
}
