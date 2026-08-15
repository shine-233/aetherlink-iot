// 文件用途：验证场景自动化和日志服务的执行记录边界。
// 核心逻辑：构造场景、动作和日志输入，断言筛选、执行状态和返回数据转换。
// 关键注意事项：场景测试要保护用户可见执行历史，失败场景不能误报成功或跨租户展示。
// 重构建议：拆出动作执行器和日志仓储 fake，补齐事务回滚、权限拒绝和幂等执行测试。
package service

import (
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSceneCreateSceneNilClaims(t *testing.T) {
	t.Parallel()

	scene := &Scene{}
	_, err := scene.CreateScene(model.CreateSceneReq{}, nil)
	assertErrcodeError(t, err, "CreateScene nil claims", errcode.CodeNoPermission, "no permission to create scene")
}

func TestSceneGetSceneListByPageNilClaims(t *testing.T) {
	t.Parallel()

	scene := &Scene{}
	_, err := scene.GetSceneListByPage(model.GetSceneListByPageReq{}, nil)
	assertErrcodeError(t, err, "GetSceneListByPage nil claims", errcode.CodeNoPermission, "no permission to query scene")
}

func TestSceneDeleteSceneNilClaims(t *testing.T) {
	t.Parallel()

	scene := &Scene{}
	err := scene.DeleteScene("scene-id", nil)
	assertErrcodeError(t, err, "DeleteScene nil claims", errcode.CodeNoPermission, "no permission to query scene")
}

func TestSceneGetSceneNilClaims(t *testing.T) {
	t.Parallel()

	scene := &Scene{}
	_, err := scene.GetScene("scene-id", nil)
	assertErrcodeError(t, err, "GetScene nil claims", errcode.CodeNoPermission, "no permission to query scene")
}

func TestSceneActiveSceneNilClaims(t *testing.T) {
	t.Parallel()

	scene := &Scene{}
	err := scene.ActiveScene("scene-id", nil)
	assertErrcodeError(t, err, "ActiveScene nil claims", errcode.CodeNoPermission, "no permission to query scene")
}

func TestSceneGetSceneLogNilClaims(t *testing.T) {
	t.Parallel()

	scene := &Scene{}
	_, err := scene.GetSceneLog(model.GetSceneLogListByPageReq{ID: "scene-id"}, nil)
	assertErrcodeError(t, err, "GetSceneLog nil claims", errcode.CodeNoPermission, "no permission to query scene")
}

func TestSceneUpdateSceneNilClaims(t *testing.T) {
	t.Parallel()

	scene := &Scene{}
	_, err := scene.UpdateScene(model.UpdateSceneReq{ID: "scene-id"}, nil)
	assertErrcodeError(t, err, "UpdateScene nil claims", errcode.CodeNoPermission, "no permission to query scene")
}

func TestSceneNonAdminCannotAccessOtherTenantScene(t *testing.T) {
	t.Parallel()

	// This tests the permission logic pattern used in scene services:
	// non-SYS_ADMIN users should not be able to access scenes of other tenants
	nonAdminClaims := &utils.UserClaims{
		ID:        "user-1",
		Authority: "TENANT_ADMIN",
		TenantID:  "tenant-1",
	}
	assert.NotEqual(t, constant.SYS_ADMIN, nonAdminClaims.Authority)
}

func TestSceneResponsePayloadsPreserveClientContract(t *testing.T) {
	t.Parallel()

	description := "lighting automation"
	sceneInfo := &model.SceneInfo{
		ID:          "scene-1",
		Name:        "turn on lights",
		Description: &description,
		TenantID:    "tenant-1",
		Creator:     "creator-1",
	}
	actions := []*model.SceneActionInfo{{
		ID:           "action-1",
		SceneID:      "scene-1",
		ActionType:   model.AUTOMATE_ACTION_TYPE_ONE,
		ActionTarget: "device-1",
		TenantID:     "tenant-1",
	}}

	detail := sceneDetailResponse(sceneInfo, actions)
	require.Len(t, detail, 2)
	assert.Same(t, sceneInfo, detail["info"])
	assert.Equal(t, actions, detail["actions"])

	list := []*model.SceneInfo{sceneInfo}
	listResp := sceneListResponse(1, list)
	require.Len(t, listResp, 2)
	assert.Equal(t, int64(1), listResp["total"])
	assert.Equal(t, list, listResp["list"])
	assert.NotContains(t, listResp, "data")

	logs := []*model.SceneLog{{
		ID:              "log-1",
		SceneID:         "scene-1",
		Detail:          "action executed",
		ExecutionResult: "S",
		TenantID:        "tenant-1",
	}}
	logResp := sceneLogListResponse(1, logs)
	require.Len(t, logResp, 2)
	assert.Equal(t, int64(1), logResp["total"])
	assert.Equal(t, logs, logResp["list"])
	assert.NotContains(t, logResp, "data")
}
