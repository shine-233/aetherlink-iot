package service

import (
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
	tenantID, err := sceneAutomationQueryTenantID(req.DeviceId, req.DeviceConfigId, u)
	if err != nil {
		return nil, err
	}

	total, sceneInfo, err := dal.GetSceneAutomationByPage(req, tenantID)
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
	tenantID, err := sceneAutomationQueryTenantID(req.DeviceId, req.DeviceConfigId, u)
	if err != nil {
		return nil, err
	}

	total, sceneInfo, err := dal.GetSceneAutomationWithAlarmByPageReq(req, tenantID)
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
