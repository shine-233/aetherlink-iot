// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
)

func CreateDeviceTriggerCondition(d model.DeviceTriggerCondition, tx *query.QueryTx) error {
	if tx != nil {
		return tx.DeviceTriggerCondition.Create(&d)
	} else {
		return query.DeviceTriggerCondition.Create(&d)
	}
}

func SwitchDeviceTriggerCondition(sceneAutomationId, enabled string, tx *query.QueryTx) error {
	_, err := tx.DeviceTriggerCondition.
		Where(tx.DeviceTriggerCondition.SceneAutomationID.Eq(sceneAutomationId)).
		Update(tx.DeviceTriggerCondition.Enabled, enabled)
	return err
}

func GetDeviceTriggerCondition(sceneAutomationId string) ([]*model.DeviceTriggerCondition, error) {
	data, err := query.DeviceTriggerCondition.
		Where(query.DeviceTriggerCondition.SceneAutomationID.Eq(sceneAutomationId)).
		Find()
	return data, err
}

func DeleteDeviceTriggerCondition(sceneAutomationId string, tx *query.QueryTx) error {
	if tx != nil {
		_, err := tx.DeviceTriggerCondition.Where(tx.DeviceTriggerCondition.SceneAutomationID.Eq(sceneAutomationId)).Delete()
		return err
	} else {
		_, err := query.DeviceTriggerCondition.Where(query.DeviceTriggerCondition.SceneAutomationID.Eq(sceneAutomationId)).Delete()
		return err
	}
}

func GetDeviceTriggerConditionByDeviceId(deviceId string, conditionType string) ([]model.DeviceTriggerCondition, error) {
	var condtionds []model.DeviceTriggerCondition
	qd := query.DeviceTriggerCondition
	err := qd.Where(qd.TriggerConditionType.Eq(conditionType), qd.TriggerSource.Eq(deviceId), qd.Enabled.Eq("Y")).Scan(&condtionds)
	return condtionds, err
}

func GetDeviceTriggerConditionListByDeviceId(deviceId string) ([]model.DeviceTriggerCondition, error) {
	var condtionds []model.DeviceTriggerCondition
	qd := query.DeviceTriggerCondition
	err := qd.Where(qd.TriggerSource.Eq(deviceId)).Scan(&condtionds)
	return condtionds, err
}

func GetDeviceTriggerConditionByGroupIds(groupIds []string) ([]model.DeviceTriggerCondition, error) {
	var condtionds []model.DeviceTriggerCondition
	qd := query.DeviceTriggerCondition
	err := qd.Where(qd.GroupID.In(groupIds...), qd.Enabled.Eq("Y")).Scan(&condtionds)
	return condtionds, err
}
