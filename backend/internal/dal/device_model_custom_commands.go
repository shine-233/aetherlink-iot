// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

func CreateDeviceModelCustomCommand(data *model.DeviceModelCustomCommand) error {
	return query.DeviceModelCustomCommand.Create(data)
}

func GetDeviceModelCustomCommandById(id string) (*model.DeviceModelCustomCommand, error) {
	return query.DeviceModelCustomCommand.Where(query.DeviceModelCustomCommand.ID.Eq(id)).First()
}

func UpdateDeviceModelCustomCommand(data *model.DeviceModelCustomCommand) (*model.DeviceModelCustomCommand, error) {
	info, err := query.DeviceModelCustomCommand.Where(query.DeviceModelCustomCommand.ID.Eq(data.ID)).Updates(data)
	if err != nil {
		return nil, err
	} else if info.RowsAffected == 0 {
		return nil, fmt.Errorf("update device model custom command failed, no rows affected")
	}
	return data, err
}

func DeleteDeviceModelCustomCommandById(id string) error {
	info, err := query.DeviceModelCustomCommand.Where(query.DeviceModelCustomCommand.ID.Eq(id)).Delete()
	if err != nil {
		logrus.Error(err)
	}

	if info.RowsAffected == 0 {
		return fmt.Errorf("no data deleted")
	}

	return err

}

func GetDeviceModelCustomCommandsByPage(page model.GetDeviceModelListByPageReq, tenantID string) (int64, []*model.DeviceModelCustomCommand, error) {
	var count int64
	q := query.DeviceModelCustomCommand
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))
	queryBuilder = queryBuilder.Where(q.DeviceTemplateID.Eq(page.DeviceTemplateId))
	if page.EnableStatus != nil {
		queryBuilder = queryBuilder.Where(q.EnableStatus.Eq(*page.EnableStatus))
	}
	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	if page.Page != 0 && page.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(page.PageSize)
		queryBuilder = queryBuilder.Offset((page.Page - 1) * page.PageSize)
	}

	data, err := queryBuilder.Select(q.ALL).Find()
	if err != nil {
		logrus.Error(err)
		return count, data, err
	}

	return count, data, nil

}

func GetDeviceModelCustomCommandsByDeviceId(deviceId, tenantId string) ([]*model.DeviceModelCustomCommand, error) {
	d, err := GetDeviceByIDUnscoped(deviceId)
	if err != nil {
		return nil, err
	}
	if d.DeviceConfigID == nil {
		return nil, nil
	}
	dc, err := GetDeviceConfigByID(*d.DeviceConfigID)
	if err != nil {
		return nil, err
	}
	if dc.DeviceTemplateID == nil {
		return nil, nil
	}
	data, err := query.DeviceModelCustomCommand.
		Where(query.DeviceModelCustomCommand.DeviceTemplateID.Eq(*dc.DeviceTemplateID)).
		Where(query.DeviceModelCustomCommand.TenantID.Eq(tenantId)).
		Find()
	if err != nil {
		logrus.Error(err)
	}

	return data, nil
}
