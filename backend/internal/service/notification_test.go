// 文件用途：验证通知服务配置、分组和告警消息组装行为。
// 核心逻辑：覆盖通知目标解析、模板参数、分组过滤和服务配置读写分支。
// 关键注意事项：通知数据可能包含邮箱或 webhook，测试应同时关注租户隔离和敏感信息不落日志。
// 重构建议：按 provider 拆分测试夹具，补齐配置事务、发送失败和权限边界用例。
package service

import (
	"encoding/json"
	"errors"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
)

// --- requireNotificationServicesAdmin ---

func TestNotificationRequireAdmin(t *testing.T) {
	tests := []struct {
		name     string
		claims   *utils.UserClaims
		wantErr  bool
		wantCode int
	}{
		{
			name:     "nil claims rejected",
			claims:   nil,
			wantErr:  true,
			wantCode: errcode.CodeNoPermission,
		},
		{
			name: "non-admin rejected",
			claims: &utils.UserClaims{
				ID:        "user-1",
				Authority: "TENANT_ADMIN",
				TenantID:  "tenant-1",
			},
			wantErr:  true,
			wantCode: errcode.CodeNoPermission,
		},
		{
			name: "empty authority rejected",
			claims: &utils.UserClaims{
				ID:        "user-1",
				Authority: "",
				TenantID:  "tenant-1",
			},
			wantErr:  true,
			wantCode: errcode.CodeNoPermission,
		},
		{
			name: "tenant user rejected",
			claims: &utils.UserClaims{
				ID:        "user-2",
				Authority: constant.TENANT_USER,
				TenantID:  "tenant-1",
			},
			wantErr:  true,
			wantCode: errcode.CodeNoPermission,
		},
		{
			name: "sys admin allowed",
			claims: &utils.UserClaims{
				ID:        "admin-1",
				Authority: constant.SYS_ADMIN,
				TenantID:  "tenant-1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requireNotificationServicesAdmin(tt.claims)
			if tt.wantErr {
				assert.Error(t, err)
				appErr, ok := err.(*errcode.Error)
				assert.True(t, ok, "expected *errcode.Error, got %T", err)
				assert.Equal(t, tt.wantCode, appErr.Code)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- SendTestEmail email validation (pure validation before DB) ---

func TestNotificationSendTestEmailValidatesRecipient(t *testing.T) {
	nsc := &NotificationServicesConfig{}

	tests := []struct {
		name     string
		email    string
		wantErr  bool
		wantCode int
	}{
		{
			name:     "empty email rejected before DB access",
			email:    "",
			wantErr:  true,
			wantCode: 200014,
		},
		{
			name:     "malformed email rejected before DB access",
			email:    "not-an-email",
			wantErr:  true,
			wantCode: 200014,
		},
		{
			name:     "email missing domain rejected before DB access",
			email:    "user@",
			wantErr:  true,
			wantCode: 200014,
		},
		{
			name:     "email missing tld rejected before DB access",
			email:    "user@example",
			wantErr:  true,
			wantCode: 200014,
		},
		{
			name:     "email with spaces rejected before DB access",
			email:    "user @example.com",
			wantErr:  true,
			wantCode: 200014,
		},
		{
			name:     "email with only @ rejected before DB access",
			email:    "@",
			wantErr:  true,
			wantCode: 200014,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nsc.SendTestEmail(&model.SendTestEmailReq{
				Email: tt.email,
				Body:  "test body",
			})
			if tt.wantErr {
				assert.Error(t, err)
				appErr, ok := err.(*errcode.Error)
				assert.True(t, ok, "expected *errcode.Error, got %T", err)
				assert.Equal(t, tt.wantCode, appErr.Code)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- SendTestEmailByAdmin permission check ---

func TestNotificationSendTestEmailByAdminRequiresPermission(t *testing.T) {
	nsc := &NotificationServicesConfig{}

	err := nsc.SendTestEmailByAdmin(nil, nil)
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok, "expected *errcode.Error, got %T", err)
	assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
}

func TestNotificationSendTestEmailByAdminNonAdminRejected(t *testing.T) {
	nsc := &NotificationServicesConfig{}

	claims := &utils.UserClaims{
		ID:        "user-1",
		Authority: constant.TENANT_USER,
		TenantID:  "tenant-1",
	}
	err := nsc.SendTestEmailByAdmin(&model.SendTestEmailReq{Email: "test@example.com", Body: "body"}, claims)
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
}

// --- NotificationHistory permission check ---

func TestNotificationHistoryListNilClaimsRejected(t *testing.T) {
	nh := &NotificationHisory{}
	_, err := nh.GetNotificationHistoryListByPage(&model.GetNotificationHistoryListByPageReq{}, nil)
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
	assert.Equal(t, "no permission to query notification history", appErr.CustomMsg)
}

func TestNotificationHistoryOwnerScopeAllowsKnownRolesOnly(t *testing.T) {
	for _, authority := range []string{constant.TENANT_USER, constant.TENANT_ADMIN, constant.SYS_ADMIN} {
		t.Run(authority, func(t *testing.T) {
			err := ensureNotificationHistoryOwnerScope(&utils.UserClaims{Authority: authority})
			assert.NoError(t, err)
		})
	}

	for _, authority := range []string{"", "UNKNOWN"} {
		t.Run("reject_"+authority, func(t *testing.T) {
			err := ensureNotificationHistoryOwnerScope(&utils.UserClaims{Authority: authority})
			assert.Error(t, err)
			appErr, ok := err.(*errcode.Error)
			assert.True(t, ok)
			assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
		})
	}
}

func TestNotificationHistoryTenantUserRedactionKeepsAdminFieldsPrivate(t *testing.T) {
	content := "device alarm body"
	remark := "provider error"
	newHistory := func() *model.NotificationHistory {
		return &model.NotificationHistory{
			SendTarget:  "ops@example.com",
			SendContent: &content,
			Remark:      &remark,
		}
	}

	tenantUserHistory := newHistory()
	redactNotificationHistoryForTenantUser(
		[]*model.NotificationHistory{tenantUserHistory},
		&utils.UserClaims{Authority: constant.TENANT_USER},
	)
	assert.Empty(t, tenantUserHistory.SendTarget)
	assert.Nil(t, tenantUserHistory.SendContent)
	assert.Nil(t, tenantUserHistory.Remark)

	adminHistory := newHistory()
	redactNotificationHistoryForTenantUser(
		[]*model.NotificationHistory{adminHistory},
		&utils.UserClaims{Authority: constant.TENANT_ADMIN},
	)
	assert.Equal(t, "ops@example.com", adminHistory.SendTarget)
	assert.Equal(t, "device alarm body", *adminHistory.SendContent)
	assert.Equal(t, "provider error", *adminHistory.Remark)
}

// --- handleMemberNotification config parsing ---

func TestNotificationHandleMemberNilConfig(t *testing.T) {
	nsc := &NotificationServicesConfig{}
	notificationGroup := &model.NotificationGroup{
		ID:                 "group-1",
		NotificationConfig: nil,
	}
	err := nsc.handleMemberNotification(notificationGroup, `{"subject":"test"}`, "subject", "content", "tenant-1")
	assert.Error(t, err)
	assert.EqualError(t, err, "notification config is nil")
}

func TestNotificationHandleMemberInvalidAlertJSON(t *testing.T) {
	nsc := &NotificationServicesConfig{}
	configStr := `{"MEMBER":[]}`
	notificationGroup := &model.NotificationGroup{
		ID:                 "group-1",
		NotificationConfig: &configStr,
	}
	err := nsc.handleMemberNotification(notificationGroup, `{invalid json`, "subject", "content", "tenant-1")
	assert.Error(t, err)
	assert.EqualError(t, err, "parse alert json failed: invalid character 'i' looking for beginning of object key string")
}

func TestNotificationHandleMemberInvalidConfigJSON(t *testing.T) {
	nsc := &NotificationServicesConfig{}
	configStr := `{invalid config`
	notificationGroup := &model.NotificationGroup{
		ID:                 "group-1",
		NotificationConfig: &configStr,
	}
	err := nsc.handleMemberNotification(notificationGroup, `{"subject":"test"}`, "subject", "content", "tenant-1")
	assert.Error(t, err)
	assert.EqualError(t, err, "parse notification config failed: invalid character 'i' looking for beginning of object key string")
}

func TestNotificationHandleMemberMissingMemberConfig(t *testing.T) {
	nsc := &NotificationServicesConfig{}
	configStr := `{"EMAIL":"test@example.com"}`
	notificationGroup := &model.NotificationGroup{
		ID:                 "group-1",
		NotificationConfig: &configStr,
	}
	err := nsc.handleMemberNotification(notificationGroup, `{"subject":"test"}`, "subject", "content", "tenant-1")
	assert.Error(t, err)
	assert.EqualError(t, err, "MEMBER config not found")
}

func TestNotificationHandleMemberInvalidMemberFormat(t *testing.T) {
	nsc := &NotificationServicesConfig{}
	configStr := `{"MEMBER":"not-array-or-object"}`
	notificationGroup := &model.NotificationGroup{
		ID:                 "group-1",
		NotificationConfig: &configStr,
	}
	err := nsc.handleMemberNotification(notificationGroup, `{"subject":"test"}`, "subject", "content", "tenant-1")
	assert.Error(t, err)
	assert.EqualError(t, err, "invalid MEMBER config format")
}

func TestNotificationHandleMemberArrayMemberFormat(t *testing.T) {
	members, err := parseNotificationMemberConfig(map[string]interface{}{
		"MEMBER": []interface{}{
			map[string]interface{}{
				"name":             "user-1",
				"notificationType": []interface{}{"APP"},
			},
			map[string]interface{}{
				"name":             "user-2",
				"notificationType": "APP",
			},
		},
	})

	assert.NoError(t, err)
	if assert.Len(t, members, 2) {
		assert.Equal(t, "user-1", members[0]["name"])
		assert.Equal(t, []interface{}{"APP"}, members[0]["notificationType"])
		assert.Equal(t, "user-2", members[1]["name"])
		assert.Equal(t, "APP", members[1]["notificationType"])
	}
}

func TestNotificationHandleMemberObjectMemberFormat(t *testing.T) {
	members, err := parseNotificationMemberConfig(map[string]interface{}{
		"MEMBER": map[string]interface{}{
			"name":             "user-1",
			"notificationType": "APP",
		},
	})

	assert.NoError(t, err)
	if assert.Len(t, members, 1) {
		assert.Equal(t, "user-1", members[0]["name"])
		assert.Equal(t, "APP", members[0]["notificationType"])
	}
}

// --- sendWebhookMessage JSON validation ---

func TestNotificationSendWebhookMessageInvalidJSON(t *testing.T) {
	nsc := &NotificationServicesConfig{}
	err := nsc.sendWebhookMessage("http://example.com/webhook", "secret", "not-valid-json", "tenant-1")
	assert.Error(t, err)
}

func TestResolveWebhookEndpointPreservesDeliveryURLAndRedactsAuditTarget(t *testing.T) {
	rawURL := "https://user:password@hooks.example.com/v1/alerts?token=sensitive#fragment"
	endpoint, auditTarget, err := resolveWebhookEndpoint(rawURL)

	assert.NoError(t, err)
	assert.Equal(t, rawURL, endpoint.String())
	assert.Equal(t, "https://hooks.example.com/v1/alerts", auditTarget)
	assert.NotContains(t, auditTarget, "sensitive")
	assert.NotContains(t, auditTarget, "password")
}

func TestResolveWebhookEndpointRejectsUnavailableProviders(t *testing.T) {
	for _, rawURL := range []string{"", "/relative", "ftp://hooks.example.com/alerts", "://invalid"} {
		t.Run(rawURL, func(t *testing.T) {
			_, _, err := resolveWebhookEndpoint(rawURL)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrWebhookProviderUnavailable), "error = %v", err)
		})
	}
}

func TestNotificationSendWebhookMessageRejectsInvalidEndpointBeforePersistence(t *testing.T) {
	err := (&NotificationServicesConfig{}).sendWebhookMessage("file:///tmp/webhook", "secret", `{}`, "tenant-1")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrWebhookProviderUnavailable), "error = %v", err)
}

// --- Notification type constants ---

func TestNotificationTypeConstants(t *testing.T) {
	assert.Equal(t, "EMAIL", model.NoticeType_Email)
	assert.Equal(t, "SME_CODE", model.NoticeType_SME_CODE)
	assert.Equal(t, "MEMBER", model.NoticeType_Member)
	assert.Equal(t, "WEBHOOK", model.NoticeType_Webhook)
	assert.Equal(t, "APP", model.NoticeType_APP)
}

// --- Notification group list response contract ---

func TestNotificationGroupListResponsePreservesTotalAndListKeys(t *testing.T) {
	groups := []*model.NotificationGroup{
		{
			ID:               "group-1",
			TenantID:         "tenant-1",
			Name:             "alarm email group",
			NotificationType: model.NoticeType_Email,
			Status:           "OPEN",
		},
	}

	resp := notificationGroupListResponse(int64(1), groups)

	assert.Len(t, resp, 2)
	assert.Equal(t, int64(1), resp["total"])

	list, ok := resp["list"].([]*model.NotificationGroup)
	assert.True(t, ok, "list type = %T, want []*model.NotificationGroup", resp["list"])
	if assert.Len(t, list, 1) {
		assert.Same(t, groups[0], list[0])
		assert.Equal(t, "group-1", list[0].ID)
		assert.Equal(t, "tenant-1", list[0].TenantID)
		assert.Equal(t, "alarm email group", list[0].Name)
		assert.Equal(t, model.NoticeType_Email, list[0].NotificationType)
		assert.Equal(t, "OPEN", list[0].Status)
	}

	assert.NotContains(t, resp, "data")
}

func TestNotificationHistoryListResponsePreservesTotalAndListKeys(t *testing.T) {
	content := "alert fired"
	result := "SUCCESS"
	history := []*model.NotificationHistory{
		{
			ID:               "history-1",
			SendContent:      &content,
			SendTarget:       "ops@example.com",
			SendResult:       &result,
			NotificationType: model.NoticeType_Email,
			TenantID:         "tenant-1",
		},
	}

	resp := notificationHistoryListResponse(int64(1), history)

	assert.Len(t, resp, 2)
	assert.Equal(t, int64(1), resp["total"])

	list, ok := resp["list"].([]*model.NotificationHistory)
	assert.True(t, ok, "list type = %T, want []*model.NotificationHistory", resp["list"])
	if assert.Len(t, list, 1) {
		assert.Same(t, history[0], list[0])
		assert.Equal(t, "history-1", list[0].ID)
		assert.Equal(t, "ops@example.com", list[0].SendTarget)
		assert.Equal(t, model.NoticeType_Email, list[0].NotificationType)
		assert.Equal(t, "tenant-1", list[0].TenantID)
		assert.Equal(t, "alert fired", *list[0].SendContent)
		assert.Equal(t, "SUCCESS", *list[0].SendResult)
	}

	assert.NotContains(t, resp, "records")
}

// --- JSON encoding without HTML escape (used in webhook) ---

func TestNotificationJSONEncodingNoHTMLEscape(t *testing.T) {
	raw, err := json.Marshal(map[string]interface{}{
		"content": "<html>test</html>",
		"value":   true,
	})
	assert.NoError(t, err)
	// Standard json.Marshal escapes HTML characters
	assert.Contains(t, string(raw), `\u003c`)

	cleanJson, err := cleanWebhookAlertJSON(`{"content":"<html>test</html>","value":true}`)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"content":"<html>test</html>","value":true}`, cleanJson)
	assert.NotContains(t, cleanJson, `\u003c`)
	assert.NotContains(t, cleanJson, `\u003e`)
	assert.Contains(t, cleanJson, "<html>test</html>")
}

func TestNotificationTemplateVarsCarryDeviceScope(t *testing.T) {
	vars, ok := parseExecuteNotificationTemplateVars(`{
		"subject":"alarm",
		"content":"details",
		"device_ids":["device-a","device-b"]
	}`)

	assert.True(t, ok)
	if assert.NotNil(t, vars) {
		assert.Equal(t, "alarm", vars.subject)
		assert.Equal(t, "details", vars.content)
		assert.Equal(t, []string{"device-a", "device-b"}, vars.deviceIDs)
	}
}

func TestNotificationDeviceScopeIgnoresNonStringValues(t *testing.T) {
	deviceIDs := notificationDeviceIDsFromAlertData(map[string]interface{}{
		"device_ids": []interface{}{"device-a", float64(42), nil, "device-b"},
	})

	assert.Equal(t, []string{"device-a", "device-b"}, deviceIDs)
}
