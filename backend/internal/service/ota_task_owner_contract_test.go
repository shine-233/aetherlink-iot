// 文件用途：锁定 OTA 任务对普通租户用户的设备 owner 隔离源码合同。
// 核心逻辑：确认列表和单任务入口都把 owner 范围传到 DAL，并对混合或孤儿设备失败关闭。
// 关键注意事项：这是轻量源码合同，不替代真实 PostgreSQL、账号和 API 运行验证。
package service

import (
	"os"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func TestOTATaskOwnerScopeRejectsUnknownAuthority(t *testing.T) {
	_, err := otaTaskOwnerUserIDForClaims(&utils.UserClaims{
		ID:        "unexpected-role-1",
		TenantID:  "tenant-1",
		Authority: "UNEXPECTED_ROLE",
	})
	appErr, ok := err.(*errcode.Error)
	if !ok || appErr.Code != errcode.CodeNoPermission {
		t.Fatalf("unexpected authority error = %#v, want no-permission", err)
	}

	for _, authority := range []string{constant.TENANT_ADMIN, constant.SYS_ADMIN} {
		ownerUserID, adminErr := otaTaskOwnerUserIDForClaims(&utils.UserClaims{Authority: authority})
		if adminErr != nil || ownerUserID != nil {
			t.Fatalf("manager authority %s returned owner=%v err=%v", authority, ownerUserID, adminErr)
		}
	}
}

func TestOTATaskTenantUserOwnerScopeContract(t *testing.T) {
	serviceSource := readOTATaskOwnerContractFile(t, "ota.go")
	listSource := readOTATaskOwnerContractFile(t, "ota_task.go")
	dalSource := readOTATaskOwnerContractFile(t, "../dal/ota_upgrade_tasks.go")

	for _, required := range []string{
		"otaTaskOwnerUserIDForClaims(claims)",
		"dal.OTAUpgradeTaskDevicesOwnedBy(task.ID, *ownerUserID)",
		"no permission to access ota task",
	} {
		if !strings.Contains(serviceSource, required) {
			t.Fatalf("ota task direct access is missing owner contract %q", required)
		}
	}

	for _, required := range []string{
		"func (*OTA) DeleteOTAUpgradeTask(id string, claims *utils.UserClaims) error {\n\tif _, err := ensureOTATaskAccess(id, claims); err != nil",
		"func (*OTA) GetOTAUpgradeTaskDetailListByPage(req *model.GetOTAUpgradeTaskDetailReq, claims *utils.UserClaims) (map[string]interface{}, error) {\n\tif _, err := ensureOTATaskAccess(req.OtaUpgradeTaskId, claims); err != nil",
		"func (*OTA) GetOTAUpgradeTaskSupportBundle(taskID string, claims *utils.UserClaims) (*model.OTAUpgradeTaskSupportBundle, error) {\n\ttask, err := ensureOTATaskAccess(taskID, claims)",
		"dal.GetOtaUpgradeTaskListByPage(req, ownerUserID)",
	} {
		if !strings.Contains(listSource, required) {
			t.Fatalf("ota task entrypoint is missing owner contract %q", required)
		}
	}

	for _, required := range []string{
		"AND EXISTS (",
		"AND NOT EXISTS (",
		"scoped_device.owner_user_id IS DISTINCT FROM ?",
		"func OTAUpgradeTaskDevicesOwnedBy(",
		"counts.TotalCount > 0 && counts.OwnedCount == counts.TotalCount",
	} {
		if !strings.Contains(dalSource, required) {
			t.Fatalf("ota task DAL is missing fail-closed owner contract %q", required)
		}
	}
}

func readOTATaskOwnerContractFile(t *testing.T, path string) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(source)
}
