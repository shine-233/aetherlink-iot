package service

import (
	"fmt"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestValidateSceneAutomationEventParamTriggerValueRejectsInvalidMatchConfigs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		triggerValue string
		wantMessage  string
	}{
		{
			name:         "empty conditions",
			triggerValue: `{"match_mode":"field","conditions":[]}`,
			wantMessage:  "event trigger_value must contain at least one condition",
		},
		{
			name:         "unknown operator",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":"contains","value":"hot"}]}`,
			wantMessage:  "event trigger_value operator [contains] is not supported",
		},
		{
			name:         "between requires ordered numeric values",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":"between","value":[20,10]}]}`,
			wantMessage:  "event trigger_value between operator requires two ordered numeric values",
		},
		{
			name:         "in requires non-empty list",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"code","operator":"in","value":[]}]}`,
			wantMessage:  "event trigger_value in operator requires a non-empty list",
		},
		{
			name:         "exists requires boolean",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"online","operator":"exists","value":"true"}]}`,
			wantMessage:  "event trigger_value exists operator requires a boolean value",
		},
		{
			name:         "numeric operator requires number",
			triggerValue: `{"match_mode":"field","conditions":[{"field":"level","operator":">=","value":"warn"}]}`,
			wantMessage:  "event trigger_value numeric operator requires a number",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateSceneAutomationEventParamTriggerValue(model.Condition{
				TriggerParamType: pureHelperStringPtr("event"),
				TriggerValue:     &tc.triggerValue,
			})

			assertErrcodeError(t, err, tc.name, errcode.CodeParamError, tc.wantMessage)
		})
	}
}

func TestValidateSceneAutomationEventParamTriggerValueAcceptsStructuredMatchConfig(t *testing.T) {
	t.Parallel()

	triggerValue := `{
		"match_mode":"field",
		"conditions":[
			{"field":"level","operator":">=","value":80},
			{"field":"code","operator":"in","value":["A","B"]},
			{"field":"online","operator":"exists","value":true},
			{"field":"temperature","operator":"between","value":[10,20]}
		]
	}`

	err := validateSceneAutomationEventParamTriggerValue(model.Condition{
		TriggerParamType: pureHelperStringPtr("event"),
		TriggerValue:     &triggerValue,
	})
	if err != nil {
		t.Fatalf("valid event param match config should pass validation, got %v", err)
	}
}

// 动作 20「激活场景」的运行期对象是 scenes 表（GetSceneInfo + ActiveSceneExecute），
// 校验对象必须与运行期一致。回归锚点：2026-09-19 活栈暴露——校验此前去查
// scene_automations，指向真实场景的合法动作在创建期即被 record not found 拒绝，
// strict 模式的 31 号运行时用例第一次跑通正是靠这次修复。
func setupSceneReferenceValidationDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SceneInfo{}); err != nil {
		t.Fatalf("migrate scene_info: %v", err)
	}
	if err := db.Exec("INSERT INTO scene_info (id, tenant_id, name, description, creator, created_at) VALUES ('scene-1', 'tenant-1', 'nested scene', 'fixture', 'fixture-user', CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatalf("seed scene: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	oldQ := query.Q
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
		_ = oldQ
	})
}

func TestSceneAction20ReferenceValidatesAgainstScenesTable(t *testing.T) {
	setupSceneReferenceValidationDB(t)

	tenantAdmin := &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}

	// 同租户的真实场景：修复前这里因查 scene_automations 而 record not found。
	if err := validateSceneAutomationActionSceneReference("scene-1", tenantAdmin, "tenant-1"); err != nil {
		t.Fatalf("valid scene target must pass, got %v", err)
	}

	// 他租户场景必须拒绝（租户边界不因修复而放松）。
	otherTenant := &utils.UserClaims{TenantID: "tenant-2", Authority: constant.TENANT_ADMIN}
	if err := validateSceneAutomationActionSceneReference("scene-1", otherTenant, "tenant-2"); err == nil {
		t.Fatal("cross-tenant scene target must be rejected")
	} else if !strings.Contains(err.Error(), "no permission") {
		t.Fatalf("cross-tenant rejection should be a permission error, got %v", err)
	}

	// 不存在的场景保持拒绝。
	if err := validateSceneAutomationActionSceneReference("scene-missing", tenantAdmin, "tenant-1"); err == nil {
		t.Fatal("missing scene target must be rejected")
	}
}
