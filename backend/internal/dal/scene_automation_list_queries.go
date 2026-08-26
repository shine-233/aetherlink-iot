// 文件用途：集中场景自动化列表筛选、分页以及设备/配置到场景 ID 的解析查询。
//
// 本文件保留既有 SQL、排序和空结果契约；内部关联 helper 的错误处理仍保持兼容，
// 但设备记录不存在时会按无匹配返回，避免对 nil 查询结果解引用。scene_automations.go
// 继续负责 CRUD、启停和租户读取。
package dal

import (
	"context"
	"fmt"

	"aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/common"

	"github.com/sirupsen/logrus"
)

func GetSceneAutomationByPage(req *model.GetSceneAutomationByPageReq, tenantID string) (int64, []*model.SceneAutomation, error) {
	q := query.SceneAutomation

	var count int64
	ctx := context.Background()
	queryBuilder := q.WithContext(ctx)
	if req.Name != nil && *req.Name != "" {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *req.Name)))
	}
	if req.DeviceId != nil && *req.DeviceId != "" {
		sceneIDs, _ := getSceneAutomationIdByDeviceId(ctx, *req.DeviceId)
		// 查询设备配置 ID，并合并单类设备的触发条件和动作。设备不存在时
		// First 返回 nil 记录；保留已有直接设备命中，没有命中则按空结果返回。
		deviceInfo, _ := query.Device.Where(query.Device.ID.Eq(*req.DeviceId)).Select(query.Device.DeviceConfigID).First()
		if deviceInfo != nil && deviceInfo.DeviceConfigID != nil && *deviceInfo.DeviceConfigID != "" {
			sceneIDsByConfig, _ := getSceneAutomationIdByDeviceConfigId(ctx, *deviceInfo.DeviceConfigID)
			sceneIDs = append(sceneIDs, sceneIDsByConfig...)
		}
		if len(sceneIDs) == 0 {
			return count, nil, nil
		}
		queryBuilder = queryBuilder.Where(q.ID.In(sceneIDs...))
	}
	if req.DeviceConfigId != nil && *req.DeviceConfigId != "" {
		sceneIDs, _ := getSceneAutomationIdByDeviceConfigId(ctx, *req.DeviceConfigId)
		if len(sceneIDs) == 0 {
			return count, nil, nil
		}
		queryBuilder = queryBuilder.Where(q.ID.In(sceneIDs...))
	}

	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	if req.Page != 0 && req.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(req.PageSize)
		queryBuilder = queryBuilder.Offset((req.Page - 1) * req.PageSize)
	}
	sceneList, err := queryBuilder.Order(q.CreatedAt.Desc()).Find()
	if err != nil {
		return count, sceneList, err
	}
	return count, sceneList, nil
}

func GetSceneAutomationWithAlarmByPageReq(req *model.GetSceneAutomationsWithAlarmByPageReq, tenantID string) (int64, []*model.SceneAutomation, error) {
	q := query.SceneAutomation

	var (
		count    int64
		sceneIDs []string
	)
	ctx := context.Background()
	queryBuilder := q.WithContext(ctx)
	if !common.IsStringEmpty(req.DeviceId) {
		sceneIDs, _ = getSceneAutomationIdByDeviceId(ctx, *req.DeviceId)
		deviceConfig, err := GetDeviceByIDUnscoped(*req.DeviceId)
		if err != nil {
			return count, nil, err
		}
		if deviceConfig.DeviceConfigID != nil && *deviceConfig.DeviceConfigID != "" {
			sceneIDsByConfig, _ := getSceneAutomationIdByDeviceConfigId(ctx, *deviceConfig.DeviceConfigID)
			sceneIDs = append(sceneIDs, sceneIDsByConfig...)
		}
	} else {
		sceneIDs, _ = getSceneAutomationIdByDeviceConfigId(ctx, *req.DeviceConfigId)
	}

	if len(sceneIDs) == 0 {
		return count, nil, nil
	}
	// 查询包含告警的场景。
	sceneIDs, err := GetSceneAutomationIdWithAlartBySceneID(sceneIDs)
	if err != nil {
		return count, nil, err
	}
	if len(sceneIDs) == 0 {
		return count, nil, nil
	}

	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID), q.ID.In(sceneIDs...))
	count, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	if req.Page != 0 && req.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(req.PageSize)
		queryBuilder = queryBuilder.Offset((req.Page - 1) * req.PageSize)
	}
	sceneList, err := queryBuilder.Order(q.CreatedAt.Desc()).Find()
	if err != nil {
		return count, sceneList, err
	}
	return count, sceneList, nil
}

func getSceneAutomationIdByDeviceId(ctx context.Context, deviceID string) ([]string, error) {
	q := query.DeviceTriggerCondition
	var result []model.DeviceTriggerCondition
	var sceneIDs []string
	err := q.WithContext(ctx).Where(q.TriggerConditionType.Eq(model.DEVICE_TRIGGER_CONDITION_TYPE_ONE), q.TriggerSource.Eq(deviceID)).Scan(&result)
	if err != nil {
		return sceneIDs, err
	}

	for _, condition := range result {
		logrus.Warning(condition)
		sceneIDs = append(sceneIDs, condition.SceneAutomationID)
	}
	var actions []model.ActionInfo
	qa := query.ActionInfo
	err = qa.WithContext(ctx).Where(qa.ActionType.Eq(model.AUTOMATE_ACTION_TYPE_ONE), qa.ActionTarget.Eq(deviceID)).Scan(&actions)
	if err != nil {
		return sceneIDs, err
	}
	for _, action := range actions {
		sceneIDs = append(sceneIDs, action.SceneAutomationID)
	}
	return sceneIDs, nil
}

func getSceneAutomationIdByDeviceConfigId(ctx context.Context, deviceConfigID string) ([]string, error) {
	q := query.DeviceTriggerCondition
	var result []model.DeviceTriggerCondition
	var sceneIDs []string
	err := q.WithContext(ctx).Where(q.TriggerConditionType.Eq(model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE), q.TriggerSource.Eq(deviceConfigID)).Scan(&result)
	if err != nil {
		return sceneIDs, err
	}
	for _, condition := range result {
		sceneIDs = append(sceneIDs, condition.SceneAutomationID)
	}
	var actions []model.ActionInfo
	qa := query.ActionInfo
	err = qa.WithContext(ctx).Where(qa.ActionType.Eq(model.AUTOMATE_ACTION_TYPE_MULTIPLE), qa.ActionTarget.Eq(deviceConfigID)).Scan(&actions)
	if err != nil {
		return sceneIDs, err
	}
	for _, action := range actions {
		sceneIDs = append(sceneIDs, action.SceneAutomationID)
	}
	return sceneIDs, nil
}
