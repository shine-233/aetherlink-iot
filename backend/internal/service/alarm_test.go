// 文件用途：验证告警服务的规则、状态和通知边界。
// 核心逻辑：构造告警规则、历史和租户上下文，断言告警计算、权限拒绝和通知触发条件。
// 关键注意事项：告警行为会影响前端提示和外部消息，测试应同时覆盖正常、异常和跨租户输入。
// 重构建议：拆分规则判定与通知副作用夹具，补齐事务回滚、通知失败和权限边界用例。
package service

import (
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
)

// --- alarmHistoryDeviceIDsForAccess ---

func TestAlarmHistoryDeviceIDsForAccess(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "empty string returns empty slice",
			raw:  "",
			want: nil,
		},
		{
			name: "whitespace only returns empty slice",
			raw:  "   ",
			want: nil,
		},
		{
			name: "valid JSON array of IDs",
			raw:  `["device-1","device-2"]`,
			want: []string{"device-1", "device-2"},
		},
		{
			name: "single device ID",
			raw:  `["device-1"]`,
			want: []string{"device-1"},
		},
		{
			name: "empty JSON array",
			raw:  `[]`,
			want: []string{},
		},
		{
			name: "invalid JSON returns nil",
			raw:  `{invalid}`,
			want: nil,
		},
		{
			name: "JSON object instead of array returns nil",
			raw:  `{"id":"device-1"}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alarmHistoryDeviceIDsForAccess(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- normalizeAlarmListTenantID ---

func TestAlarmNormalizeListTenantID(t *testing.T) {
	tests := []struct {
		name            string
		requestTenantID string
		claims          *utils.UserClaims
		wantTenantID    string
		wantErr         bool
		wantCode        int
	}{
		{
			name:            "nil claims rejected",
			requestTenantID: "",
			claims:          nil,
			wantErr:         true,
			wantCode:        errcode.CodeNoPermission,
		},
		{
			name:            "sys admin with empty request tenant ID",
			requestTenantID: "",
			claims: &utils.UserClaims{
				ID:        "admin-1",
				Authority: constant.SYS_ADMIN,
				TenantID:  "tenant-1",
			},
			wantTenantID: "",
		},
		{
			name:            "sys admin with specific request tenant ID",
			requestTenantID: "tenant-2",
			claims: &utils.UserClaims{
				ID:        "admin-1",
				Authority: constant.SYS_ADMIN,
				TenantID:  "tenant-1",
			},
			wantTenantID: "tenant-2",
		},
		{
			name:            "sys admin with whitespace request tenant ID",
			requestTenantID: "  tenant-2  ",
			claims: &utils.UserClaims{
				ID:        "admin-1",
				Authority: constant.SYS_ADMIN,
				TenantID:  "tenant-1",
			},
			wantTenantID: "tenant-2",
		},
		{
			name:            "non-admin with matching tenant ID",
			requestTenantID: "tenant-1",
			claims: &utils.UserClaims{
				ID:        "user-1",
				Authority: constant.TENANT_USER,
				TenantID:  "tenant-1",
			},
			wantTenantID: "tenant-1",
		},
		{
			name:            "non-admin with empty request tenant ID uses claims tenant",
			requestTenantID: "",
			claims: &utils.UserClaims{
				ID:        "user-1",
				Authority: constant.TENANT_USER,
				TenantID:  "tenant-1",
			},
			wantTenantID: "tenant-1",
		},
		{
			name:            "non-admin with whitespace request tenant ID uses claims tenant",
			requestTenantID: "   ",
			claims: &utils.UserClaims{
				ID:        "user-1",
				Authority: constant.TENANT_USER,
				TenantID:  "tenant-1",
			},
			wantTenantID: "tenant-1",
		},
		{
			name:            "non-admin with different tenant ID rejected",
			requestTenantID: "tenant-2",
			claims: &utils.UserClaims{
				ID:        "user-1",
				Authority: constant.TENANT_USER,
				TenantID:  "tenant-1",
			},
			wantErr:  true,
			wantCode: errcode.CodeNoPermission,
		},
		{
			name:            "non-admin with empty claims tenant ID rejected",
			requestTenantID: "",
			claims: &utils.UserClaims{
				ID:        "user-1",
				Authority: constant.TENANT_USER,
				TenantID:  "",
			},
			wantErr:  true,
			wantCode: errcode.CodeNoPermission,
		},
		{
			name:            "tenant admin with matching tenant ID",
			requestTenantID: "tenant-1",
			claims: &utils.UserClaims{
				ID:        "admin-1",
				Authority: constant.TENANT_ADMIN,
				TenantID:  "tenant-1",
			},
			wantTenantID: "tenant-1",
		},
		{
			name:            "tenant admin with different tenant ID rejected",
			requestTenantID: "tenant-2",
			claims: &utils.UserClaims{
				ID:        "admin-1",
				Authority: constant.TENANT_ADMIN,
				TenantID:  "tenant-1",
			},
			wantErr:  true,
			wantCode: errcode.CodeNoPermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAlarmListTenantID(tt.requestTenantID, tt.claims)
			if tt.wantErr {
				assert.Error(t, err)
				appErr, ok := err.(*errcode.Error)
				if !assert.True(t, ok, "expected *errcode.Error, got %T", err) {
					return
				}
				assert.Equal(t, tt.wantCode, appErr.Code)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTenantID, got)
			}
		})
	}
}

// --- validateAlarmHistoryType ---

func TestAlarmValidateHistoryType(t *testing.T) {
	tests := []struct {
		name      string
		alarmType *string
		wantErr   bool
	}{
		{
			name:      "nil alarm type is valid",
			alarmType: nil,
			wantErr:   false,
		},
		{
			name:      "empty alarm type is valid",
			alarmType: alarmTestStrPtr(""),
			wantErr:   false,
		},
		{
			name:      "whitespace alarm type is valid (trimmed to empty)",
			alarmType: alarmTestStrPtr("   "),
			wantErr:   false,
		},
		{
			name:      "temperature_alarm is valid",
			alarmType: alarmTestStrPtr("temperature_alarm"),
			wantErr:   false,
		},
		{
			name:      "switch_alarm is valid",
			alarmType: alarmTestStrPtr("switch_alarm"),
			wantErr:   false,
		},
		{
			name:      "warranty_alarm is valid",
			alarmType: alarmTestStrPtr("warranty_alarm"),
			wantErr:   false,
		},
		{
			name:      "pressure_alarm is valid",
			alarmType: alarmTestStrPtr("pressure_alarm"),
			wantErr:   false,
		},
		{
			name:      "PT is valid",
			alarmType: alarmTestStrPtr("PT"),
			wantErr:   false,
		},
		{
			name:      "unsupported alarm type rejected",
			alarmType: alarmTestStrPtr("unknown_alarm"),
			wantErr:   true,
		},
		{
			name:      "alarm type with trailing space is trimmed and valid",
			alarmType: alarmTestStrPtr("temperature_alarm "),
			wantErr:   false,
		},
		{
			name:      "case sensitive: TEMPERATURE_ALARM rejected",
			alarmType: alarmTestStrPtr("TEMPERATURE_ALARM"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &model.GetAlarmHisttoryListByPage{AlarmType: tt.alarmType}
			err := validateAlarmHistoryType(req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAlarmValidateHistoryTypeNilReq(t *testing.T) {
	err := validateAlarmHistoryType(nil)
	assert.NoError(t, err)
}

func TestAlarmValidateHistoryTypeTrimsValue(t *testing.T) {
	trimmed := "  temperature_alarm  "
	req := &model.GetAlarmHisttoryListByPage{AlarmType: &trimmed}
	err := validateAlarmHistoryType(req)
	assert.NoError(t, err)
	assert.Equal(t, "temperature_alarm", *req.AlarmType)
}

func TestAlarmValidateHistoryStatusAllowsStoredValuesAndActiveQueryAlias(t *testing.T) {
	for _, raw := range []string{"", "   ", "H", " M ", "L", "N", model.AlarmHistoryQueryStatusActive} {
		req := &model.GetAlarmHisttoryListByPage{AlarmStatus: alarmTestStrPtr(raw)}
		assert.NoError(t, validateAlarmHistoryStatus(req), "alarm status %q should be accepted", raw)
		assert.Equal(t, strings.TrimSpace(raw), *req.AlarmStatus)
	}
	assert.NoError(t, validateAlarmHistoryStatus(nil))
	assert.NoError(t, validateAlarmHistoryStatus(&model.GetAlarmHisttoryListByPage{}))
}

func TestAlarmValidateHistoryStatusRejectsUnsupportedOrWrongCaseValues(t *testing.T) {
	for _, raw := range []string{"ALL", "active", "A", "RESET", "unknown"} {
		req := &model.GetAlarmHisttoryListByPage{AlarmStatus: alarmTestStrPtr(raw)}
		err := validateAlarmHistoryStatus(req)
		assert.Error(t, err, "alarm status %q should be rejected", raw)
		appErr, ok := err.(*errcode.Error)
		assert.True(t, ok)
		assert.Equal(t, errcode.CodeParamError, appErr.Code)
		assert.Equal(t, "unsupported alarm_status", appErr.CustomMsg)
	}
}

// --- Alarm.AcknowledgeAlarmHistory ID validation ---

func TestAlarmAcknowledgeHistoryEmptyID(t *testing.T) {
	alarm := &Alarm{}
	_, err := alarm.AcknowledgeAlarmHistory("", &utils.UserClaims{
		ID:        "user-1",
		Authority: constant.SYS_ADMIN,
		TenantID:  "tenant-1",
	})
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeParamError, appErr.Code)
}

func TestAlarmAcknowledgeHistoryWhitespaceID(t *testing.T) {
	alarm := &Alarm{}
	_, err := alarm.AcknowledgeAlarmHistory("   ", &utils.UserClaims{
		ID:        "user-1",
		Authority: constant.SYS_ADMIN,
		TenantID:  "tenant-1",
	})
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeParamError, appErr.Code)
}

// --- Alarm.ResetAlarmHistory ID validation ---

func TestAlarmResetHistoryEmptyID(t *testing.T) {
	alarm := &Alarm{}
	_, err := alarm.ResetAlarmHistory("", &utils.UserClaims{
		ID:        "user-1",
		Authority: constant.SYS_ADMIN,
		TenantID:  "tenant-1",
	})
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeParamError, appErr.Code)
}

func TestAlarmResetHistoryWhitespaceID(t *testing.T) {
	alarm := &Alarm{}
	_, err := alarm.ResetAlarmHistory("   ", &utils.UserClaims{
		ID:        "user-1",
		Authority: constant.SYS_ADMIN,
		TenantID:  "tenant-1",
	})
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeParamError, appErr.Code)
}

// --- Alarm.UpdateAlarmInfoBatch empty ID validation ---

func TestAlarmUpdateInfoBatchEmptyIDs(t *testing.T) {
	alarm := &Alarm{}
	err := alarm.UpdateAlarmInfoBatch(&model.UpdateAlarmInfoBatchReq{
		Id: []string{},
	}, &utils.UserClaims{
		ID:        "user-1",
		Authority: constant.SYS_ADMIN,
		TenantID:  "tenant-1",
	})
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeParamError, appErr.Code)
}

func TestAlarmUpdateInfoBatchNilClaims(t *testing.T) {
	alarm := &Alarm{}
	err := alarm.UpdateAlarmInfoBatch(&model.UpdateAlarmInfoBatchReq{
		Id: []string{"id-1"},
	}, nil)
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
}

// --- Alarm.CreateAlarmConfig nil claims ---

func TestAlarmCreateConfigNilClaims(t *testing.T) {
	alarm := &Alarm{}
	_, err := alarm.CreateAlarmConfig(&model.CreateAlarmConfigReq{}, nil)
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
}

// --- Alarm.GetAlarmHisttoryListByPage validation ---

func TestAlarmGetHisttoryListByPageInvalidType(t *testing.T) {
	alarm := &Alarm{}
	alarmType := "invalid_type"
	_, err := alarm.GetAlarmHisttoryListByPage(&model.GetAlarmHisttoryListByPage{
		AlarmType: &alarmType,
	}, &utils.UserClaims{
		ID:        "user-1",
		Authority: constant.SYS_ADMIN,
		TenantID:  "tenant-1",
	})
	assert.Error(t, err)
}

func TestAlarmGetHisttoryListByPageNilClaims(t *testing.T) {
	alarm := &Alarm{}
	_, err := alarm.GetAlarmHisttoryListByPage(&model.GetAlarmHisttoryListByPage{}, nil)
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
}

func TestValidateAlarmHistoryMonthlyTrendYear(t *testing.T) {
	assert.NoError(t, validateAlarmHistoryMonthlyTrendYear(2026))
	assert.Error(t, validateAlarmHistoryMonthlyTrendYear(1999))
	assert.Error(t, validateAlarmHistoryMonthlyTrendYear(2101))
}

func TestResolveAlarmHistoryMonthlyTrendLocation(t *testing.T) {
	location, timezone, err := resolveAlarmHistoryMonthlyTrendLocation("Asia/Shanghai")
	assert.NoError(t, err)
	assert.NotNil(t, location)
	assert.Equal(t, "Asia/Shanghai", timezone)

	_, _, err = resolveAlarmHistoryMonthlyTrendLocation("not/a-timezone")
	assert.Error(t, err)
}

func TestResetLoadedAlarmHistoryRejectsNormalStatus(t *testing.T) {
	_, err := dal.ResetLoadedAlarmHistoryWithNote(&model.AlarmHistory{AlarmStatus: "N"}, "user-1", "")
	assert.Error(t, err)
}

func TestAlarmGetHistoryMonthlyTrendNilClaims(t *testing.T) {
	alarm := &Alarm{}
	_, err := alarm.GetAlarmHistoryMonthlyTrend(&model.AlarmHistoryMonthlyTrendReq{Year: 2026}, nil)
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
}

func TestAlarmGetDeviceCountsNilClaims(t *testing.T) {
	alarm := &Alarm{}
	_, err := alarm.GetAlarmDeviceCountsByTenant(nil)
	assert.Error(t, err)
	appErr, ok := err.(*errcode.Error)
	assert.True(t, ok)
	assert.Equal(t, errcode.CodeNoPermission, appErr.Code)
}

func TestTenantUserAlarmHistoryAccessUsesDeviceOwner(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "alarm-owner-config", "tenant-a", "1")
	createDeviceServiceOwnedDevice(t, db, "alarm-device-a", "alarm-owner-a", "tenant-a", "user-a", configID, now)
	createDeviceServiceOwnedDevice(t, db, "alarm-device-b", "alarm-owner-b", "tenant-a", "user-b", configID, now)

	for _, history := range []*model.AlarmHistory{
		{
			ID: "alarm-history-a", AlarmConfigID: "config-a", GroupID: "group-a", SceneAutomationID: "scene-a",
			Name: "owner a alarm", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now,
			AlarmDeviceList: `["alarm-device-a"]`,
		},
		{
			ID: "alarm-history-b", AlarmConfigID: "config-b", GroupID: "group-b", SceneAutomationID: "scene-b",
			Name: "owner b alarm", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now,
			AlarmDeviceList: `["alarm-device-b"]`,
		},
		{
			ID: "alarm-history-mixed", AlarmConfigID: "config-mixed", GroupID: "group-mixed", SceneAutomationID: "scene-mixed",
			Name: "mixed owner alarm", AlarmStatus: "H", TenantID: "tenant-a", CreateAt: now,
			AlarmDeviceList: `["alarm-device-a","alarm-device-b"]`,
		},
	} {
		if err := db.Create(history).Error; err != nil {
			t.Fatalf("create alarm history %s: %v", history.ID, err)
		}
	}

	tenantUser := &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Authority: constant.TENANT_USER}
	if _, err := ensureAlarmHistoryReadAccess("alarm-history-a", tenantUser); err != nil {
		t.Fatalf("owner read was rejected: %v", err)
	}
	if _, err := ensureAlarmHistoryWriteAccess("alarm-history-a", tenantUser); err != nil {
		t.Fatalf("owner write was rejected: %v", err)
	}
	for _, access := range []struct {
		name string
		call func() error
	}{
		{name: "read", call: func() error { _, err := ensureAlarmHistoryReadAccess("alarm-history-b", tenantUser); return err }},
		{name: "write", call: func() error { _, err := ensureAlarmHistoryWriteAccess("alarm-history-b", tenantUser); return err }},
	} {
		t.Run(access.name, func(t *testing.T) {
			err := access.call()
			appErr, ok := err.(*errcode.Error)
			if !ok || appErr.Code != errcode.CodeNoPermission {
				t.Fatalf("foreign owner access error = %#v, want no-permission", err)
			}
		})
	}
	if _, err := ensureAlarmHistoryReadAccess("alarm-history-mixed", tenantUser); err != nil {
		t.Fatalf("mixed alarm read should remain visible to an involved owner: %v", err)
	}
	if _, err := ensureAlarmHistoryWriteAccess("alarm-history-mixed", tenantUser); err == nil {
		t.Fatal("mixed alarm write should be rejected when any linked device has another owner")
	}
}

func TestPreloadedAlarmHistoryBatchActionEnforcesEveryDeviceOwner(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	configID := createDeviceServiceConfig(t, db, "alarm-batch-owner-config", "tenant-a", "1")
	createDeviceServiceOwnedDevice(t, db, "alarm-batch-device-a", "alarm-batch-owner-a", "tenant-a", "user-a", configID, now)
	createDeviceServiceOwnedDevice(t, db, "alarm-batch-device-b", "alarm-batch-owner-b", "tenant-a", "user-b", configID, now)

	histories := map[string]*model.AlarmHistory{
		"alarm-batch-owner": {
			ID: "alarm-batch-owner", TenantID: "tenant-a", AlarmDeviceList: `["alarm-batch-device-a"]`,
		},
		"alarm-batch-foreign": {
			ID: "alarm-batch-foreign", TenantID: "tenant-a", AlarmDeviceList: `["alarm-batch-device-b"]`,
		},
		"alarm-batch-mixed": {
			ID: "alarm-batch-mixed", TenantID: "tenant-a", AlarmDeviceList: `["alarm-batch-device-a","alarm-batch-device-b"]`,
		},
	}
	var appliedIDs []string
	plan := &alarmHistoryBatchActionPlan{
		action: "acknowledge",
		applyLoaded: func(history *model.AlarmHistory, operatorID, note string) (*model.AlarmHistoryActionResp, error) {
			appliedIDs = append(appliedIDs, history.ID)
			return &model.AlarmHistoryActionResp{}, nil
		},
	}
	claims := &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Authority: constant.TENANT_USER}

	if _, err := applyPreloadedAlarmHistoryBatchAction("alarm-batch-owner", claims, plan, histories, nil); err != nil {
		t.Fatalf("owner batch action was rejected: %v", err)
	}
	for _, id := range []string{"alarm-batch-foreign", "alarm-batch-mixed"} {
		if _, err := applyPreloadedAlarmHistoryBatchAction(id, claims, plan, histories, nil); err == nil {
			t.Fatalf("batch action %s should reject non-owned linked devices", id)
		} else if appErr, ok := err.(*errcode.Error); !ok || appErr.Code != errcode.CodeNoPermission {
			t.Fatalf("batch action %s error = %#v, want no-permission", id, err)
		}
	}
	assert.Equal(t, []string{"alarm-batch-owner"}, appliedIDs)
}

func TestTenantUserAlarmHistoryMutationEntrypointsRejectForeignOwner(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	description := "before"
	configID := createDeviceServiceConfig(t, db, "alarm-entry-owner-config", "tenant-a", "1")
	createDeviceServiceOwnedDevice(t, db, "alarm-entry-device-b", "alarm-entry-owner-b", "tenant-a", "user-b", configID, now)
	history := &model.AlarmHistory{
		ID:                "alarm-entry-foreign",
		AlarmConfigID:     "alarm-entry-config",
		GroupID:           "alarm-entry-group",
		SceneAutomationID: "alarm-entry-scene",
		Name:              "foreign owner alarm",
		Description:       &description,
		AlarmStatus:       "H",
		TenantID:          "tenant-a",
		CreateAt:          now,
		AlarmDeviceList:   `["alarm-entry-device-b"]`,
	}
	if err := db.Create(history).Error; err != nil {
		t.Fatalf("create foreign alarm history: %v", err)
	}

	claims := &utils.UserClaims{ID: "user-a", TenantID: "tenant-a", Authority: constant.TENANT_USER}
	alarmService := &Alarm{}
	for _, mutation := range []struct {
		name string
		call func() error
	}{
		{
			name: "description update",
			call: func() error {
				return alarmService.AlarmHistoryDescUpdate(&model.AlarmHistoryDescUpdateReq{
					AlarmHistoryId: history.ID,
					Description:    "after",
				}, claims)
			},
		},
		{
			name: "acknowledge",
			call: func() error {
				_, err := alarmService.AcknowledgeAlarmHistory(history.ID, claims)
				return err
			},
		},
		{
			name: "reset",
			call: func() error {
				_, err := alarmService.ResetAlarmHistory(history.ID, claims)
				return err
			},
		},
		{
			name: "delete",
			call: func() error {
				return alarmService.DeleteAlarmHistory(history.ID, claims)
			},
		},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			err := mutation.call()
			appErr, ok := err.(*errcode.Error)
			if !ok || appErr.Code != errcode.CodeNoPermission {
				t.Fatalf("foreign owner mutation error = %#v, want no-permission", err)
			}
		})
	}

	batchResp, err := alarmService.BatchAlarmHistoryAction(&model.AlarmHistoryBatchActionReq{
		IDs:    []string{history.ID},
		Action: "acknowledge",
	}, claims)
	if err != nil {
		t.Fatalf("foreign owner batch action returned request error: %v", err)
	}
	if batchResp.SuccessCount != 0 || batchResp.FailureCount != 1 || len(batchResp.Results) != 1 || batchResp.Results[0].OK {
		t.Fatalf("foreign owner batch response = %#v, want one item-level failure", batchResp)
	}

	var persisted model.AlarmHistory
	if err := db.First(&persisted, "id = ?", history.ID).Error; err != nil {
		t.Fatalf("foreign alarm history should remain after rejected mutations: %v", err)
	}
	if persisted.Description == nil {
		t.Fatal("foreign alarm history description was cleared by a rejected mutation")
	}
	assert.Equal(t, "before", *persisted.Description)
	assert.Equal(t, "H", persisted.AlarmStatus)
}

func TestDeleteAlarmHistoryRetainsAuthorizedAuditRecord(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	now := time.Now().UTC()
	history := &model.AlarmHistory{
		ID:                "alarm-history-retained",
		AlarmConfigID:     "alarm-retention-config",
		GroupID:           "alarm-retention-group",
		SceneAutomationID: "alarm-retention-scene",
		Name:              "retained alarm",
		AlarmStatus:       "N",
		TenantID:          "tenant-a",
		CreateAt:          now,
		AlarmDeviceList:   `[]`,
	}
	if err := db.Create(history).Error; err != nil {
		t.Fatalf("create retained alarm history: %v", err)
	}

	claims := &utils.UserClaims{ID: "tenant-admin", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN}
	err := (&Alarm{}).DeleteAlarmHistory(history.ID, claims)
	appErr, ok := err.(*errcode.Error)
	if !ok || appErr.Code != errcode.CodeOpDenied || appErr.CustomMsg != alarmHistoryRetentionMessage {
		t.Fatalf("delete alarm history error = %#v, want retention denial", err)
	}

	var persisted model.AlarmHistory
	if err := db.First(&persisted, "id = ?", history.ID).Error; err != nil {
		t.Fatalf("retained alarm history should remain queryable: %v", err)
	}
	assert.Equal(t, history.ID, persisted.ID)
}

// helper
func alarmTestStrPtr(s string) *string {
	return &s
}
