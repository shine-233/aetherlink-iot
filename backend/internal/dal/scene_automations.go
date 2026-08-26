// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"context"

	"github.com/sirupsen/logrus"
)

func CreateSceneAutomation(d *model.SceneAutomation, tx *query.QueryTx) error {
	if tx != nil {
		return tx.SceneAutomation.Create(d)
	} else {
		return query.SceneAutomation.Create(d)
	}
}

func SaveSceneAutomation(d *model.SceneAutomation, tx *query.QueryTx) error {
	if tx != nil {
		return tx.SceneAutomation.Save(d)
	} else {
		return query.SceneAutomation.Save(d)
	}
}

func DeleteSceneAutomation(id string, tx *query.QueryTx) error {
	if tx != nil {
		_, err := tx.SceneAutomation.Where(tx.SceneAutomation.ID.Eq(id)).Delete()
		return err
	} else {
		_, err := query.SceneAutomation.Where(query.SceneAutomation.ID.Eq(id)).Delete()
		return err
	}

}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetSceneAutomation(id string, tx *query.QueryTx) (*model.SceneAutomation, error) {
	if tx != nil {
		data, err := tx.SceneAutomation.Where(tx.SceneAutomation.ID.Eq(id)).First()
		return data, err
	} else {
		data, err := query.SceneAutomation.Where(query.SceneAutomation.ID.Eq(id)).First()
		if err != nil {
			logrus.Error(err)
		}
		return data, err
	}
}

func SwitchSceneAutomation(id, enabled string, tx *query.QueryTx) error {
	_, err := tx.SceneAutomation.Where(tx.SceneAutomation.ID.Eq(id)).Update(tx.SceneAutomation.Enabled, enabled)
	return err
}

func CheckSceneAutomationHasClose(id string) bool {
	activeCount, err := query.SceneAutomation.Where(query.SceneAutomation.ID.Eq(id), query.SceneAutomation.Enabled.Eq("Y")).Count()
	if err != nil {
		logrus.WithError(err).WithField("scene_automation_id", id).Warn("failed to check scene automation active state")
		return true
	}
	return activeCount != 1
}

func GetSceneAutomationTenantID(ctx context.Context, scene_id string) (string, error) {
	// Keep this query uncached so tenant ownership changes are visible immediately.
	var tenantID string
	if err := query.SceneAutomation.WithContext(ctx).Where(query.SceneAutomation.ID.Eq(scene_id)).Select(query.SceneAutomation.TenantID).Scan(&tenantID); err != nil {
		logrus.WithError(err).WithField("scene_automation_id", scene_id).Warn("resolve scene automation tenant failed")
		return "", err
	}
	return tenantID, nil
}
