// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
)

func CreateActionInfo(d model.ActionInfo, tx *query.QueryTx) error {
	if tx != nil {
		return tx.ActionInfo.Create(&d)
	} else {
		return query.ActionInfo.Create(&d)
	}
}

// tenant-scope: parent-owned?2026-08-26 ?????
func GetActionInfo(sceneAutomationId string) ([]*model.ActionInfo, error) {
	data, err := query.ActionInfo.Where(query.ActionInfo.SceneAutomationID.Eq(sceneAutomationId)).Find()
	return data, err
}

func DeleteActionInfo(sceneAutomationId string, tx *query.QueryTx) error {
	if tx != nil {
		_, err := tx.ActionInfo.Where(tx.ActionInfo.SceneAutomationID.Eq(sceneAutomationId)).Delete()
		return err
	} else {
		_, err := query.ActionInfo.Where(query.ActionInfo.SceneAutomationID.Eq(sceneAutomationId)).Delete()
		return err
	}
}

// tenant-scope: parent-owned?2026-08-26 ?????
func GetActionInfoListBySceneAutomationId(sceneAutomationIds []string) ([]model.ActionInfo, error) {
	var actionInfos []model.ActionInfo
	qa := query.ActionInfo
	return actionInfos, qa.Where(qa.SceneAutomationID.In(sceneAutomationIds...)).Scan(&actionInfos)
}

// 获取场景动作
// tenant-scope: parent-owned?2026-08-26 ?????
func GetActionInfoListBySceneId(sceneIds []string) ([]model.ActionInfo, error) {
	var (
		result      []model.ActionInfo
		actionInfos []model.SceneActionInfo
	)
	qa := query.SceneActionInfo

	err := qa.Where(qa.SceneID.In(sceneIds...)).Scan(&actionInfos)
	if err != nil {
		return result, err
	}
	for i := range actionInfos {
		result = append(result, model.ActionInfo{
			SceneAutomationID: actionInfos[i].SceneID,
			ActionTarget:      &actionInfos[i].ActionTarget,
			ActionType:        actionInfos[i].ActionType,
			ActionParamType:   actionInfos[i].ActionParamType,
			ActionParam:       actionInfos[i].ActionParam,
			ActionValue:       actionInfos[i].ActionValue,
			Remark:            actionInfos[i].Remark,
		})
	}
	return result, nil
}

// tenant-scope: parent-owned?2026-08-26 ?????
func GetSceneAutomationIdWithAlartBySceneID(sceneIds []string) ([]string, error) {
	var resultSceneIds []string
	result, err := query.ActionInfo.Where(query.ActionInfo.SceneAutomationID.In(sceneIds...), query.ActionInfo.ActionType.Eq(model.AUTOMATE_ACTION_TYPE_ALARM)).Distinct(query.ActionInfo.SceneAutomationID).Find()
	if err != nil {
		return resultSceneIds, nil
	}
	for _, v := range result {
		resultSceneIds = append(resultSceneIds, v.SceneAutomationID)
	}
	return resultSceneIds, nil
}
