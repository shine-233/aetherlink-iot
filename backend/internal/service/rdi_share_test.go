// 文件用途：验证 RDI 分享事务、令牌和接收人语义。
// 核心逻辑：覆盖分享 SQL 片段、token 有效期、回滚错误传播和接收人过滤逻辑。
// 关键注意事项：RDI 分享会暴露设备数据，测试必须保护 token 过期、SQL 通配符和跨用户访问边界。
// 重构建议：拆分 token、接收人和事务测试，补齐并发撤销、权限拒绝和审计日志边界。
package service

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func assertErrCode(t *testing.T, err error, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("expected *errcode.Error, got %T", err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("error code = %d, want %d", appErr.Code, wantCode)
	}
	if wantMessage != "" && appErr.CustomMsg != wantMessage {
		t.Fatalf("error message = %q, want %q", appErr.CustomMsg, wantMessage)
	}
}

func TestRDIShareTransactionsPropagateErrorsToDeferredRollback(t *testing.T) {
	source, err := os.ReadFile("rdi_share.go")
	if err != nil {
		t.Fatalf("read rdi_share.go: %v", err)
	}
	text := string(source)

	for _, pattern := range []string{
		"if err := assertRDIDeviceAccess",
		"if err := dal.UpdateDeviceAdditionalInfoWithTx",
		"if err := dal.Commit",
	} {
		if strings.Contains(text, pattern) {
			t.Fatalf("transaction rollback guard is bypassed by shadowed error pattern %q", pattern)
		}
	}
}

func TestRDIShareAdditionalInfoFragmentEscapesSQLLikeWildcards(t *testing.T) {
	cases := map[string]string{
		`tenant_100%`:      `tenant\_100\%`,
		`path\with_marker`: `path\\with\_marker`,
		`plain-token-hash`: `plain-token-hash`,
		`100%_match\exact`: `100\%\_match\\exact`,
	}

	for input, want := range cases {
		if got := escapeSQLLikeFragment(input); got != want {
			t.Fatalf("escapeSQLLikeFragment(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRDIShareServiceGuards(t *testing.T) {
	validClaims := &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Email: "user-a@example.com"}

	t.Run("create share token requires permission", func(t *testing.T) {
		_, err := GroupApp.RDI.CreateShareToken("device-a", &model.RDIShareTokenReq{}, nil)
		assertErrCode(t, err, errcode.CodeNoPermission, "")
	})

	t.Run("create share token requires device id", func(t *testing.T) {
		_, err := GroupApp.RDI.CreateShareToken("   ", &model.RDIShareTokenReq{}, validClaims)
		assertErrCode(t, err, errcode.CodeParamError, "device id is required")
	})

	t.Run("create share token tolerates nil request and still validates device id", func(t *testing.T) {
		_, err := GroupApp.RDI.CreateShareToken("   ", nil, validClaims)
		assertErrCode(t, err, errcode.CodeParamError, "device id is required")
	})

	t.Run("accept shared device requires permission", func(t *testing.T) {
		_, err := GroupApp.RDI.AcceptSharedDevice("share-token", nil)
		assertErrCode(t, err, errcode.CodeNoPermission, "")
	})

	t.Run("accept shared device requires token", func(t *testing.T) {
		_, err := GroupApp.RDI.AcceptSharedDevice("   ", validClaims)
		assertErrCode(t, err, errcode.CodeParamError, "share token is required")
	})

	t.Run("shared devices requires permission", func(t *testing.T) {
		_, err := GroupApp.RDI.SharedDevices(&model.RDISharedDeviceListReq{}, nil)
		assertErrCode(t, err, errcode.CodeNoPermission, "")
	})

	t.Run("shared device config requires token", func(t *testing.T) {
		_, err := GroupApp.RDI.SharedDeviceConfig("   ")
		assertErrCode(t, err, errcode.CodeParamError, "share token is required")
	})
}

func TestRDIShareRecipientSemantics(t *testing.T) {
	t.Run("shared-with-me lookup depends on explicit recipient metadata", func(t *testing.T) {
		additional, err := json.Marshal(map[string]interface{}{
			rdiShareRecipientsKey: []model.RDIShareRecipientRecord{
				{
					UserID:     "user-b",
					Email:      "user-b@example.com",
					TenantID:   "tenant-b",
					TokenHash:  "hash-b",
					AcceptedAt: 1718900000,
				},
			},
		})
		if err != nil {
			t.Fatalf("marshal additional info: %v", err)
		}

		device := &model.Device{
			ID:       "dev-1",
			TenantID: "tenant-a",
			AdditionalInfo: func() *string {
				value := string(additional)
				return &value
			}(),
		}
		sameTenantClaims := &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Email: "user-a@example.com"}
		recipientClaims := &utils.UserClaims{ID: "user-b", TenantID: "tenant-b", Email: "user-b@example.com"}
		sameRecipientTenantDifferentUserClaims := &utils.UserClaims{ID: "user-c", TenantID: "tenant-b", Email: "user-c@example.com"}
		sameRecipientEmailDifferentUserClaims := &utils.UserClaims{ID: "user-c", TenantID: "tenant-c", Email: "user-b@example.com"}

		if recipient, ok := rdiShareRecipientForUser(device, sameTenantClaims); ok {
			t.Fatalf("same-tenant owner without recipient metadata must not appear in shared-with-me: %#v", recipient)
		}
		if recipient, ok := rdiShareRecipientForUser(device, sameRecipientTenantDifferentUserClaims); ok {
			t.Fatalf("same recipient tenant but different user must not appear in shared-with-me: %#v", recipient)
		}
		if recipient, ok := rdiShareRecipientForUser(device, sameRecipientEmailDifferentUserClaims); ok {
			t.Fatalf("same recipient email but different user must not appear in shared-with-me: %#v", recipient)
		}

		recipient, ok := rdiShareRecipientForUser(device, recipientClaims)
		if !ok {
			t.Fatal("expected explicit recipient metadata to be discoverable")
		}
		if recipient.UserID != "user-b" || recipient.Email != "user-b@example.com" || recipient.TenantID != "tenant-b" || recipient.TokenHash != "hash-b" || recipient.AcceptedAt != 1718900000 {
			t.Fatalf("recipient metadata = %#v, want exact accepted share recipient", recipient)
		}
	})
}

func TestRDIShareSameTenantRecipientKeepsReadOnlyAccess(t *testing.T) {
	ownerUserID := "user-owner"
	additional := `{"rdi_config":{"data_collection_interval":45,"sensor_alarm_emails":"alarm@example.com","switch_alarm_emails":"alarm@example.com","warranty_alarm_emails":"alarm@example.com","sensor_1_alarm_emails":"alarm@example.com","sensor_2_alarm_emails":"alarm@example.com","switch_1_alarm_emails":"alarm@example.com","switch_2_alarm_emails":"alarm@example.com"},"rdi_share_recipients":[{"user_id":"user-recipient","email":"recipient@example.com","tenant_id":"tenant-a","token_hash":"hash-a","accepted_at":1718900000}]}`
	device := &model.Device{
		ID:             "device-a",
		TenantID:       "tenant-a",
		OwnerUserID:    &ownerUserID,
		AdditionalInfo: &additional,
	}
	recipientClaims := &utils.UserClaims{
		ID:        "user-recipient",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
		Email:     "recipient@example.com",
	}

	if !hasTelemetryTenantAccess(device, recipientClaims, true) {
		t.Fatal("same-tenant accepted recipient should have shared read access")
	}
	if !canReadDeviceOnlineStatus(device, recipientClaims) {
		t.Fatal("same-tenant accepted recipient should read the shared device online status")
	}
	if hasTelemetryTenantAccess(device, recipientClaims, false) {
		t.Fatal("same-tenant accepted recipient must not gain owner/write access")
	}
	response := rdiAcceptShareFastPath(device, recipientClaims)
	if response == nil || !response.AlreadyAccepted || !response.SharedWithMe {
		t.Fatalf("same-tenant accepted recipient fast path = %#v, want accepted shared response", response)
	}
	rdiTestAssertAlarmEmails(t, response.Device.Config, "")
	if response.Device.Config.DataCollectionInterval != 45 {
		t.Fatalf("shared response changed collection interval: %#v", response.Device.Config)
	}

	nonRecipientClaims := &utils.UserClaims{
		ID:        "user-other",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	}
	if response := rdiAcceptShareFastPath(device, nonRecipientClaims); response != nil {
		t.Fatalf("same-tenant non-owner without recipient metadata bypassed acceptance: %#v", response)
	}
	if canReadDeviceOnlineStatus(device, nonRecipientClaims) {
		t.Fatal("same-tenant non-owner without recipient metadata read device online status")
	}

	ownerClaims := &utils.UserClaims{
		ID:        ownerUserID,
		TenantID:  "tenant-a",
		Authority: constant.TENANT_USER,
	}
	ownerResponse := rdiAcceptShareFastPath(device, ownerClaims)
	if ownerResponse == nil || !ownerResponse.AlreadyAccepted || ownerResponse.SharedWithMe {
		t.Fatalf("device owner fast path = %#v, want existing owner access", ownerResponse)
	}
	rdiTestAssertAlarmEmails(t, ownerResponse.Device.Config, "alarm@example.com")

	tenantAdminResponse := rdiAcceptShareFastPath(device, &utils.UserClaims{
		ID:        "tenant-admin",
		TenantID:  "tenant-a",
		Authority: constant.TENANT_ADMIN,
	})
	if tenantAdminResponse == nil || tenantAdminResponse.SharedWithMe {
		t.Fatalf("tenant admin fast path = %#v, want privileged existing access", tenantAdminResponse)
	}
	rdiTestAssertAlarmEmails(t, tenantAdminResponse.Device.Config, "alarm@example.com")
}

func TestRDIConfigDisclosureFollowsDeviceOwnershipScope(t *testing.T) {
	ownerUserID := "device-owner"
	// 七个告警邮箱字段都要设值：rdiTestAssertAlarmEmails 会逐个校验，
	// 只设 sensor_alarm_emails 会让未设置的字段与"已脱敏"无法区分。
	additional := `{"private_note":"owner-only","rdi_config":{"data_collection_interval":45,"sensor_alarm_emails":"alarm@example.com","switch_alarm_emails":"alarm@example.com","warranty_alarm_emails":"alarm@example.com","sensor_1_alarm_emails":"alarm@example.com","sensor_2_alarm_emails":"alarm@example.com","switch_1_alarm_emails":"alarm@example.com","switch_2_alarm_emails":"alarm@example.com"},"rdi_system_info":{"address":"Shared installation","extra_fields":{"private_system_note":"owner-only-system-note"}},"rdi_share_recipients":[{"user_id":"shared-user","tenant_id":"tenant-b","accepted_at":1718900000}]}`
	device := &model.Device{
		ID:             "shared-device",
		TenantID:       "tenant-a",
		OwnerUserID:    &ownerUserID,
		AdditionalInfo: &additional,
	}
	tests := []struct {
		name              string
		claims            *utils.UserClaims
		includeAdditional bool
		exposeEmails      bool
	}{
		{
			name:              "same tenant owner",
			claims:            &utils.UserClaims{ID: ownerUserID, TenantID: "tenant-a", Authority: constant.TENANT_USER},
			includeAdditional: true,
			exposeEmails:      true,
		},
		{
			name:              "same tenant admin",
			claims:            &utils.UserClaims{ID: "tenant-admin", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN},
			includeAdditional: true,
			exposeEmails:      true,
		},
		{
			name:              "system admin",
			claims:            &utils.UserClaims{ID: "system-admin", TenantID: "system", Authority: constant.SYS_ADMIN},
			includeAdditional: true,
			exposeEmails:      true,
		},
		{
			name:              "same tenant recipient",
			claims:            &utils.UserClaims{ID: "shared-user", TenantID: "tenant-a", Authority: constant.TENANT_USER},
			includeAdditional: false,
			exposeEmails:      false,
		},
		{
			name:              "cross tenant recipient",
			claims:            &utils.UserClaims{ID: "shared-user", TenantID: "tenant-b", Authority: constant.TENANT_USER},
			includeAdditional: false,
			exposeEmails:      false,
		},
		{
			name:              "cross tenant admin recipient",
			claims:            &utils.UserClaims{ID: "shared-user", TenantID: "tenant-b", Authority: constant.TENANT_ADMIN},
			includeAdditional: false,
			exposeEmails:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := rdiDeviceConfigResponseOptionsForClaims(device, tt.claims)
			if options.IncludeAdditionalInfo != tt.includeAdditional || options.ExposeAlarmEmails != tt.exposeEmails {
				t.Fatalf("disclosure options = %#v, want includeAdditional=%v exposeEmails=%v", options, tt.includeAdditional, tt.exposeEmails)
			}
			response := rdiDeviceConfigResponse(device, options)
			if tt.includeAdditional {
				if response.AdditionalInfo["private_note"] != "owner-only" {
					t.Fatalf("owner/admin response lost allowed additional info: %#v", response.AdditionalInfo)
				}
				if _, exposed := response.AdditionalInfo[rdiShareRecipientsKey]; exposed {
					t.Fatalf("owner/admin response exposed share recipient metadata: %#v", response.AdditionalInfo)
				}
				if response.SystemInfo.ExtraFields["private_system_note"] != "owner-only-system-note" {
					t.Fatalf("owner/admin response lost allowed system-info extensions: %#v", response.SystemInfo.ExtraFields)
				}
			} else if len(response.AdditionalInfo) != 0 {
				t.Fatalf("share-only response exposed raw additional info: %#v", response.AdditionalInfo)
			}
			if !tt.includeAdditional && len(response.SystemInfo.ExtraFields) != 0 {
				t.Fatalf("share-only response exposed arbitrary system-info extensions: %#v", response.SystemInfo.ExtraFields)
			}
			if response.SystemInfo.Address != "Shared installation" {
				t.Fatalf("modeled shared system information was removed: %#v", response.SystemInfo)
			}
			expectedEmail := ""
			if tt.exposeEmails {
				expectedEmail = "alarm@example.com"
			}
			rdiTestAssertAlarmEmails(t, response.Config, expectedEmail)
		})
	}
}

func TestRDISharedWithMeListRedactsEveryRecipientScope(t *testing.T) {
	additional := `{"rdi_config":{"data_collection_interval":45,"sensor_alarm_emails":"alarm@example.com","switch_alarm_emails":"alarm@example.com","warranty_alarm_emails":"alarm@example.com","sensor_1_alarm_emails":"alarm@example.com","sensor_2_alarm_emails":"alarm@example.com","switch_1_alarm_emails":"alarm@example.com","switch_2_alarm_emails":"alarm@example.com"},"rdi_share_recipients":[{"user_id":"shared-user","email":"shared@example.com","tenant_id":"tenant-b","token_hash":"hash-b","accepted_at":1718900000}]}`
	device := &model.Device{
		ID:             "shared-device",
		TenantID:       "tenant-a",
		AdditionalInfo: &additional,
	}
	req := &model.RDISharedDeviceListReq{}

	recipientClaims := &utils.UserClaims{
		ID:        "shared-user",
		TenantID:  "tenant-b",
		Authority: constant.TENANT_USER,
	}
	records := filterRDISharedDevices([]*model.Device{device}, req, recipientClaims)
	if len(records) != 1 || records[0].AcceptedAt != 1718900000 {
		t.Fatalf("ordinary shared-with-me records = %#v, want one accepted record", records)
	}
	rdiTestAssertAlarmEmails(t, records[0].Device.Config, "")
	if len(records[0].Device.AdditionalInfo) != 0 {
		t.Fatalf("ordinary shared-with-me response exposed raw additional info: %#v", records[0].Device.AdditionalInfo)
	}
	if !canReadDeviceOnlineStatus(device, recipientClaims) {
		t.Fatal("cross-tenant accepted recipient should read device online status")
	}
	if records[0].Device.Config.DataCollectionInterval != 45 {
		t.Fatalf("ordinary shared-with-me response changed non-email config: %#v", records[0].Device.Config)
	}

	adminClaims := &utils.UserClaims{
		ID:        "shared-user",
		TenantID:  "tenant-b",
		Authority: constant.TENANT_ADMIN,
	}
	adminRecords := filterRDISharedDevices([]*model.Device{device}, req, adminClaims)
	if len(adminRecords) != 1 {
		t.Fatalf("tenant admin shared-with-me records = %#v, want one accepted record", adminRecords)
	}
	rdiTestAssertAlarmEmails(t, adminRecords[0].Device.Config, "")
	if len(adminRecords[0].Device.AdditionalInfo) != 0 {
		t.Fatalf("cross-tenant admin share response exposed raw additional info: %#v", adminRecords[0].Device.AdditionalInfo)
	}
	serialized, err := json.Marshal(adminRecords[0])
	if err != nil {
		t.Fatalf("marshal shared record: %v", err)
	}
	for _, forbidden := range []string{"owner_id", "owner_email", "tenant-a"} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("shared-with-me response exposed owner/tenant identity %q: %s", forbidden, serialized)
		}
	}
}

func TestRDIShareTokenActiveRequiresUnexpiredLockedToken(t *testing.T) {
	now := int64(1718900000)
	activeHash := hashRDIShareToken("active-token")
	expiredHash := hashRDIShareToken("expired-token")
	additional := map[string]interface{}{
		rdiShareTokensKey: []model.RDIShareTokenRecord{
			{
				TokenHash: activeHash,
				ExpiresAt: now + 60,
			},
			{
				TokenHash: expiredHash,
				ExpiresAt: now,
			},
			{
				TokenHash: "",
				ExpiresAt: now + 60,
			},
		},
	}

	if !rdiShareTokenActive(additional, activeHash, now) {
		t.Fatal("active token hash was rejected")
	}
	for _, tokenHash := range []string{"", expiredHash, hashRDIShareToken("missing-token")} {
		if rdiShareTokenActive(additional, tokenHash, now) {
			t.Fatalf("token hash %q should not be active", tokenHash)
		}
	}
	if rdiShareTokenActive(map[string]interface{}{}, activeHash, now) {
		t.Fatal("missing locked token metadata should reject the token")
	}
}
