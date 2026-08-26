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

func CreatePeriodicTask(d model.PeriodicTask, tx *query.QueryTx) error {
	if tx != nil {
		return tx.PeriodicTask.Create(&d)
	} else {
		return query.PeriodicTask.Create(&d)
	}
}

func SwitchPeriodicTask(sceneAutomationId, enabled string, tx *query.QueryTx) error {
	_, err := tx.PeriodicTask.
		Where(tx.PeriodicTask.SceneAutomationID.Eq(sceneAutomationId)).
		Update(tx.PeriodicTask.Enabled, enabled)
	return err
}

// tenant-scope: system-internal?2026-08-26 ?????
func GetPeriodicTask(sceneAutomationId string) ([]*model.PeriodicTask, error) {
	data, err := query.PeriodicTask.Where(query.PeriodicTask.SceneAutomationID.Eq(sceneAutomationId)).Find()
	return data, err
}

func DeletePeriodicTask(sceneAutomationId string, tx *query.QueryTx) error {
	if tx != nil {
		_, err := tx.PeriodicTask.Where(tx.PeriodicTask.SceneAutomationID.Eq(sceneAutomationId)).Delete()
		return err
	} else {
		_, err := query.PeriodicTask.Where(query.PeriodicTask.SceneAutomationID.Eq(sceneAutomationId)).Delete()
		return err
	}
}

// tenant-scope: system-internal?2026-08-26 ?????
func GetPeriodicTaskListWithLock(limit int) ([]*model.PeriodicTask, error) {
	key := "aetherlink-iot:periodicTask"
	lockToken := common.AcquireLockToken(key, time.Second*5)
	if lockToken == "" {
		return nil, errors.New("未获取到锁")
	}
	defer func() { _ = common.ReleaseLockToken(key, lockToken) }()
	q := query.PeriodicTask
	result, err := q.Where(q.ExecutionTime.Lte(time.Now()), q.Enabled.Eq("Y")).Order(q.ExecutionTime.Asc()).Limit(limit).Find()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	var executeResult []*model.PeriodicTask
	for _, v := range result {
		nextExecuteTime, err := common.GetSceneExecuteTime(v.TaskType, v.Param)
		if err != nil {
			return result, err
		}
		if !v.ExecutionTime.IsZero() {
			executeResult = append(executeResult, v)
		}
		_, _ = q.Where(q.ID.Eq(v.ID)).UpdateColumn(q.ExecutionTime, nextExecuteTime.UTC())
	}

	return executeResult, nil
}
