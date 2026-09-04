package service

import (
	"strings"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type SceneAutomation struct{}

func ensureSceneAutomationReadAccess(sceneAutomationID string, claims *utils.UserClaims) (*model.SceneAutomation, error) {
	sceneAutomation, err := dal.GetSceneAutomation(sceneAutomationID, nil)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query scene automation")
	}
	if claims.Authority != constant.SYS_ADMIN && sceneAutomation.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query scene automation")
	}
	return sceneAutomation, nil
}

func ensureSceneAutomationWriteAccess(sceneAutomationID string, claims *utils.UserClaims) (*model.SceneAutomation, error) {
	sceneAutomation, err := ensureSceneAutomationReadAccess(sceneAutomationID, claims)
	if err != nil {
		return nil, err
	}
	if claims.Authority != constant.SYS_ADMIN && sceneAutomation.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify scene automation")
	}
	return sceneAutomation, nil
}

func sceneAutomationSwitchTarget(currentEnabled, requestedTarget string) string {
	if requestedTarget != "" {
		return requestedTarget
	}
	if currentEnabled == "Y" {
		return "N"
	}
	return "Y"
}

// CreateSceneAutomation creates the persisted rule and refreshes runtime cache
// only after tenant/reference validation and transaction writes succeed.
func (s *SceneAutomation) CreateSceneAutomation(req *model.CreateSceneAutomationReq, u *utils.UserClaims) (string, error) {
	if u == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to create scene automation")
	}
	if err := validateSceneAutomationReferences(req.TriggerConditionGroups, req.Actions, u, u.TenantID); err != nil {
		return "", err
	}

	enabled := normalizeSceneAutomationEnabled(req.Enabled)
	var sceneAutomationID string
	if err := withSceneAutomationTransaction(func(tx *query.QueryTx) error {
		var err error
		sceneAutomationID, err = createSceneAutomationRecord(tx, req, u, enabled)
		if err != nil {
			return err
		}
		if err := writeSceneAutomationTriggerGroups(tx, sceneAutomationID, enabled, u.TenantID, req.TriggerConditionGroups); err != nil {
			return err
		}
		return writeSceneAutomationActions(tx, sceneAutomationID, req.Actions)
	}); err != nil {
		return "", err
	}

	go refreshCreatedSceneAutomationCache(s, sceneAutomationID, enabled)
	return sceneAutomationID, nil
}

func refreshCreatedSceneAutomationCache(s *SceneAutomation, sceneAutomationID string, enabled string) {
	if enabled != "Y" {
		return
	}
	if err := s.AutomateCacheSet(sceneAutomationID); err != nil {
		logrus.Error("create scene automation cache refresh failed: ", err)
	}
}

func (*SceneAutomation) AutomateCacheSet(sceneAutomationID string) error {
	logrus.Info("start persisting scene automation cache")
	groupInfoPtrs, err := dal.GetDeviceTriggerCondition(sceneAutomationID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	actionInfoPtrs, err := dal.GetActionInfo(sceneAutomationID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	var groupInfos []model.DeviceTriggerCondition
	for _, groupInfo := range groupInfoPtrs {
		if groupInfo != nil && groupInfo.Enabled == "Y" {
			groupInfos = append(groupInfos, *groupInfo)
		}
	}
	var actionInfos []model.ActionInfo
	for _, actionInfo := range actionInfoPtrs {
		if actionInfo != nil {
			actionInfos = append(actionInfos, *actionInfo)
		}
	}
	err = initialize.NewAutomateCache().SetCacheBySceneAutomationId(sceneAutomationID, groupInfos, actionInfos)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*SceneAutomation) DeleteSceneAutomation(sceneAutomationID string, claims *utils.UserClaims) error {
	if _, err := ensureSceneAutomationWriteAccess(sceneAutomationID, claims); err != nil {
		return err
	}
	err := dal.DeleteSceneAutomation(sceneAutomationID, nil)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	clearSceneAutomationCaches(sceneAutomationID)
	return nil
}

func (*SceneAutomation) SwitchSceneAutomation(sceneAutomationID, target string, claims *utils.UserClaims) error {
	sceneAutomation, err := ensureSceneAutomationWriteAccess(sceneAutomationID, claims)
	if err != nil {
		return err
	}

	target = sceneAutomationSwitchTarget(sceneAutomation.Enabled, target)
	if err := switchSceneAutomationDefinition(sceneAutomationID, target); err != nil {
		return err
	}

	refreshSwitchedSceneAutomationCache(sceneAutomationID, target)
	return nil
}

func refreshSwitchedSceneAutomationCache(sceneAutomationID string, target string) {
	if target == "Y" {
		go func() {
			sa := SceneAutomation{}
			if err := sa.AutomateCacheSet(sceneAutomationID); err != nil {
				logrus.Error("refresh enabled scene automation cache failed: ", err)
			}
		}()
	}
	if target == "N" {
		clearSceneAutomationCaches(sceneAutomationID)
	}
}

func (*SceneAutomation) GetSceneAutomationByPageReq(req *model.GetSceneAutomationByPageReq, u *utils.UserClaims) (interface{}, error) {
	scopes, err := sceneAutomationListScopes(req.DeviceId, req.DeviceConfigId, u)
	if err != nil {
		return nil, err
	}

	total, sceneInfo, err := dal.GetSceneAutomationByPage(req, scopes)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	sceneListMap := make(map[string]interface{})
	sceneListMap["total"] = total
	sceneListMap["list"] = sceneInfo
	return sceneListMap, nil
}

func (*SceneAutomation) GetSceneAutomationWithAlarmByPageReq(req *model.GetSceneAutomationsWithAlarmByPageReq, u *utils.UserClaims) (interface{}, error) {
	scopes, err := sceneAutomationListScopes(req.DeviceId, req.DeviceConfigId, u)
	if err != nil {
		return nil, err
	}

	total, sceneInfo, err := dal.GetSceneAutomationWithAlarmByPageReq(req, scopes)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	sceneListMap := make(map[string]interface{})
	sceneListMap["total"] = total
	sceneListMap["list"] = sceneInfo
	return sceneListMap, nil
}

func sceneAutomationQueryTenantID(deviceID *string, deviceConfigID *string, claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query scene automation")
	}
	tenantID := claims.TenantID
	if deviceID != nil && *deviceID != "" {
		device, err := ensureTelemetryDeviceReadAccess(*deviceID, claims)
		if err != nil {
			return "", err
		}
		tenantID = device.TenantID
	}
	if deviceConfigID != nil && *deviceConfigID != "" {
		deviceConfig, err := ensureDeviceConfigReadAccess(*deviceConfigID, claims)
		if err != nil {
			return "", err
		}
		if tenantID != "" && deviceConfig.TenantID != tenantID {
			return "", errcode.NewWithMessage(errcode.CodeNoPermission, "device config tenant mismatch")
		}
		tenantID = deviceConfig.TenantID
	}
	return tenantID, nil
}

// sceneAutomationListScopes 解析场景自动化管理读作用域（ROADMAP C2 自上而下）：
// 先按 device/config 锚点做资源读访问校验并收敛到锚点租户，再映射为作用域；
// 无锚点时锚点租户即声明租户。nil 声明一律返回错误（与旧签名一致）。
func sceneAutomationListScopes(deviceID *string, deviceConfigID *string, claims *utils.UserClaims) ([]string, error) {
	tenantID, err := sceneAutomationQueryTenantID(deviceID, deviceConfigID, claims)
	if err != nil {
		return nil, err
	}
	return sceneAutomationReadScopes(tenantID, claims), nil
}

// sceneAutomationReadScopes 将解析后的租户映射为场景自动化读作用域：
// TENANT_USER 保持 self-only（场景为租户级资源、无 per-user 可见性维度），空租户
// 返回 nil（fail-closed）；空租户管理员（SYS_ADMIN 平台行）→ [""] 保持旧行为；
// 其余非空租户管理员 → expandTenantIDScope（self∪子孙，链接缺失回退 self-only）。
func sceneAutomationReadScopes(tenantID string, claims *utils.UserClaims) []string {
	if claims == nil {
		return nil
	}
	if claims.Authority == constant.TENANT_USER {
		if tenantID := strings.TrimSpace(tenantID); tenantID != "" {
			return []string{tenantID}
		}
		return nil
	}
	if strings.TrimSpace(tenantID) == "" {
		return []string{""}
	}
	return expandTenantIDScope(tenantID)
}

func (*SceneAutomation) UpdateSceneAutomation(req *model.UpdateSceneAutomationReq, u *utils.UserClaims) (string, error) {
	var sceneAutomationID string
	oldSceneAutomation, err := ensureSceneAutomationWriteAccess(req.ID, u)
	if err != nil {
		return sceneAutomationID, err
	}
	if err := validateSceneAutomationReferences(req.TriggerConditionGroups, req.Actions, u, oldSceneAutomation.TenantID); err != nil {
		return sceneAutomationID, err
	}

	sceneAutomationID = req.ID
	if err := withSceneAutomationTransaction(func(tx *query.QueryTx) error {
		return replaceSceneAutomationDefinition(tx, sceneAutomationID, req, oldSceneAutomation.TenantID, u.ID)
	}); err != nil {
		return "", err
	}

	refreshUpdatedSceneAutomationCaches(sceneAutomationID, req.Enabled)
	return sceneAutomationID, nil
}

func refreshUpdatedSceneAutomationCaches(sceneAutomationID string, enabled string) {
	clearSceneAutomationCaches(sceneAutomationID)

	if enabled == "Y" {
		go func() {
			sa := SceneAutomation{}
			if err := sa.AutomateCacheSet(sceneAutomationID); err != nil {
				logrus.Error("rebuild updated scene automation cache failed: ", err)
			}
		}()
	}
}

func clearSceneAutomationCaches(sceneAutomationID string) {
	if err := initialize.NewAutomateCache().DeleteCacheBySceneAutomationId(sceneAutomationID); err != nil {
		logrus.Error("delete scene automation cache failed: ", err)
	}

	alarmCache := initialize.NewAlarmCache()
	groupIDs, err := alarmCache.GetBySceneAutomationId(sceneAutomationID)
	if err != nil {
		return
	}
	for _, groupID := range groupIDs {
		if err := alarmCache.DeleteBygroupId(groupID); err != nil {
			logrus.Error("delete alarm cache failed: ", err)
		}
	}
}
