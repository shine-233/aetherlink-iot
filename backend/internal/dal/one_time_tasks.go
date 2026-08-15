// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/common"
	"errors"
	"time"
)

func CreateOneTimeTask(d model.OneTimeTask, tx *query.QueryTx) error {
	if tx != nil {
		return tx.OneTimeTask.Create(&d)
	} else {
		return query.OneTimeTask.Create(&d)
	}
}

func SwitchOneTimeTask(sceneAutomationId, enabled string, tx *query.QueryTx) error {
	_, err := tx.OneTimeTask.
		Where(tx.OneTimeTask.SceneAutomationID.Eq(sceneAutomationId)).
		Update(tx.OneTimeTask.Enabled, enabled)
	return err
}

func GetOneTimeTask(sceneAutomationId string) ([]*model.OneTimeTask, error) {
	data, err := query.OneTimeTask.Where(query.OneTimeTask.SceneAutomationID.Eq(sceneAutomationId)).Find()
	return data, err
}

func DeleteOneTimeTask(sceneAutomationId string, tx *query.QueryTx) error {
	if tx != nil {
		_, err := tx.OneTimeTask.Where(tx.OneTimeTask.SceneAutomationID.Eq(sceneAutomationId)).Delete()
		return err
	} else {
		_, err := query.OneTimeTask.Where(query.OneTimeTask.SceneAutomationID.Eq(sceneAutomationId)).Delete()
		return err
	}
}

func GetOnceTaskListWithLock(limit int) ([]*model.OneTimeTask, error) {
	key := "aetherlink-iot:onceTask"
	lockToken := common.AcquireLockToken(key, time.Second*5)
	if lockToken == "" {
		return nil, errors.New("未获取到锁")
	}
	defer func() { _ = common.ReleaseLockToken(key, lockToken) }()
	q := query.OneTimeTask
	result, err := q.Where(q.ExecutionTime.Lte(time.Now()), q.Enabled.Eq("Y"), q.ExecutingState.Eq("NEX")).Order(q.ExecutionTime.Asc()).Limit(limit).Find()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	var taskId []string
	for _, v := range result {
		taskId = append(taskId, v.ID)
	}
	_, err = q.Where(q.ID.In(taskId...)).UpdateColumn(q.ExecutingState, "EXE")
	if err != nil {
		return nil, err
	}
	return result, nil
}

func TaskExpirationSave(ids []string) error {
	_, err := query.OneTimeTask.Where(query.OneTimeTask.ID.In(ids...)).UpdateColumn(query.OneTimeTask.ExecutingState, "EXP")

	return err
}
