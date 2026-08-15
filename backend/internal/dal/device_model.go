// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"context"

	"github.com/sirupsen/logrus"
)

// create
func CreateDeviceModelTelemetry(d *model.DeviceModelTelemetry) (err error) {
	return query.DeviceModelTelemetry.Create(d)
}

func CreateDeviceModelAttribute(d *model.DeviceModelAttribute) (err error) {
	return query.DeviceModelAttribute.Create(d)
}

func CreateDeviceModelEvent(d *model.DeviceModelEvent) (err error) {
	return query.DeviceModelEvent.Create(d)
}

func CreateDeviceModelCommand(d *model.DeviceModelCommand) (err error) {
	return query.DeviceModelCommand.Create(d)
}

// delete
func DeleteDeviceModelTelemetry(id string) (err error) {
	r, err := query.DeviceModelTelemetry.Where(query.DeviceModelTelemetry.ID.Eq(id)).Delete()
	if r.RowsAffected == 0 {
		return nil
	}
	return err
}

func DeleteDeviceModelAttribute(id string) (err error) {
	r, err := query.DeviceModelAttribute.Where(query.DeviceModelAttribute.ID.Eq(id)).Delete()
	if r.RowsAffected == 0 {
		return nil
	}
	return err
}

func DeleteDeviceModelEvent(id string) (err error) {
	r, err := query.DeviceModelEvent.Where(query.DeviceModelEvent.ID.Eq(id)).Delete()
	if r.RowsAffected == 0 {
		return nil
	}
	return err
}

func DeleteDeviceModelCommand(id string) (err error) {
	r, err := query.DeviceModelCommand.Where(query.DeviceModelCommand.ID.Eq(id)).Delete()
	if r.RowsAffected == 0 {
		return nil
	}
	return err
}

// update
func UpdateDeviceModelTelemetry(d *model.DeviceModelTelemetry) (err error) {
	p := query.DeviceModelTelemetry
	r, err := query.DeviceModelTelemetry.Where(p.ID.Eq(d.ID)).Updates(d)
	if r.RowsAffected == 0 {
		return nil
	} else {
		return err
	}
}

func UpdateDeviceModelAttribute(d *model.DeviceModelAttribute) (err error) {
	p := query.DeviceModelAttribute
	r, err := query.DeviceModelAttribute.Where(p.ID.Eq(d.ID)).Updates(d)
	if r.RowsAffected == 0 {
		return nil
	} else {
		return err
	}
}

func UpdateDeviceModelEvent(d *model.DeviceModelEvent) (err error) {
	p := query.DeviceModelEvent
	r, err := query.DeviceModelEvent.Where(p.ID.Eq(d.ID)).Updates(d)
	if r.RowsAffected == 0 {
		return nil
	} else {
		return err
	}
}

func UpdateDeviceModelCommand(d *model.DeviceModelCommand) (err error) {
	p := query.DeviceModelCommand
	r, err := query.DeviceModelCommand.Where(p.ID.Eq(d.ID)).Updates(d)
	if r.RowsAffected == 0 {
		return nil
	} else {
		return err
	}
}

func GetDeviceModelTelemetryListByPage(r model.GetDeviceModelListByPageReq, tenant_id string) (count int64, data []*model.DeviceModelTelemetry, err error) {
	q := query.DeviceModelTelemetry
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenant_id))
	queryBuilder = queryBuilder.Where(q.DeviceTemplateID.Eq(r.DeviceTemplateId))
	count, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, data, err
	}
	if r.Page != 0 && r.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(r.PageSize)
		queryBuilder = queryBuilder.Offset((r.Page - 1) * r.PageSize)
	}
	data, err = queryBuilder.Select().Find()
	if err != nil {
		logrus.Error(err)

	}
	return count, data, err
}

func GetDeviceModelAttributesListByPage(r model.GetDeviceModelListByPageReq, tenant_id string) (count int64, data []*model.DeviceModelAttribute, err error) {
	q := query.DeviceModelAttribute
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenant_id))
	queryBuilder = queryBuilder.Where(q.DeviceTemplateID.Eq(r.DeviceTemplateId))
	count, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, data, err
	}
	if r.Page != 0 && r.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(r.PageSize)
		queryBuilder = queryBuilder.Offset((r.Page - 1) * r.PageSize)
	}
	data, err = queryBuilder.Select().Find()
	if err != nil {
		logrus.Error(err)

	}
	return count, data, err
}

func GetDeviceModelEventsListByPage(r model.GetDeviceModelListByPageReq, tenant_id string) (count int64, data []*model.DeviceModelEvent, err error) {
	q := query.DeviceModelEvent
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenant_id))
	queryBuilder = queryBuilder.Where(q.DeviceTemplateID.Eq(r.DeviceTemplateId))
	count, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, data, err
	}
	if r.Page != 0 && r.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(r.PageSize)
		queryBuilder = queryBuilder.Offset((r.Page - 1) * r.PageSize)
	}
	data, err = queryBuilder.Select().Find()
	if err != nil {
		logrus.Error(err)

	}
	return count, data, err
}

func GetDeviceModelCommandsListByPage(r model.GetDeviceModelListByPageReq, tenant_id string) (count int64, data []*model.DeviceModelCommand, err error) {
	q := query.DeviceModelCommand
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenant_id))
	queryBuilder = queryBuilder.Where(q.DeviceTemplateID.Eq(r.DeviceTemplateId))
	count, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, data, err
	}
	if r.Page != 0 && r.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(r.PageSize)
		queryBuilder = queryBuilder.Offset((r.Page - 1) * r.PageSize)
	}
	data, err = queryBuilder.Select().Find()
	if err != nil {
		logrus.Error(err)

	}
	return count, data, err
}

func GetDeviceModelEventDataList(device_template_id string) ([]*model.DeviceModelEvent, error) {
	data, err := query.DeviceModelEvent.
		Where(query.DeviceModelEvent.DeviceTemplateID.Eq(device_template_id)).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetDeviceModelCommandDataList(device_template_id string) ([]*model.DeviceModelCommand, error) {
	data, err := query.DeviceModelCommand.
		Where(query.DeviceModelCommand.DeviceTemplateID.Eq(device_template_id)).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetDeviceModelTelemetryDataList(device_template_id string) ([]*model.DeviceModelTelemetry, error) {
	data, err := query.DeviceModelTelemetry.
		Where(query.DeviceModelTelemetry.DeviceTemplateID.Eq(device_template_id)).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetDeviceModelAttributeDataList(device_template_id string) ([]*model.DeviceModelAttribute, error) {
	data, err := query.DeviceModelAttribute.
		Where(query.DeviceModelAttribute.DeviceTemplateID.Eq(device_template_id)).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// The Market-specific readers keep every exported thing-model definition inside
// the same tenant boundary as its parent configuration and template.
func GetDeviceModelEventDataForTenant(deviceTemplateID, tenantID string) ([]*model.DeviceModelEvent, error) {
	q := query.DeviceModelEvent
	return q.Where(q.DeviceTemplateID.Eq(deviceTemplateID), q.TenantID.Eq(tenantID)).Find()
}

func GetDeviceModelCommandDataForTenant(deviceTemplateID, tenantID string) ([]*model.DeviceModelCommand, error) {
	q := query.DeviceModelCommand
	return q.Where(q.DeviceTemplateID.Eq(deviceTemplateID), q.TenantID.Eq(tenantID)).Find()
}

func GetDeviceModelTelemetryDataForTenant(deviceTemplateID, tenantID string) ([]*model.DeviceModelTelemetry, error) {
	q := query.DeviceModelTelemetry
	return q.Where(q.DeviceTemplateID.Eq(deviceTemplateID), q.TenantID.Eq(tenantID)).Find()
}

func GetDeviceModelAttributeDataForTenant(deviceTemplateID, tenantID string) ([]*model.DeviceModelAttribute, error) {
	q := query.DeviceModelAttribute
	return q.Where(q.DeviceTemplateID.Eq(deviceTemplateID), q.TenantID.Eq(tenantID)).Find()
}

func GetIdentifierNameTelemetry() func(device_template_id, identifier string) string {
	return func(device_template_id, identifier string) string {
		q := query.DeviceModelTelemetry
		var result model.DeviceModelTelemetry
		q.Where(q.DeviceTemplateID.Eq(device_template_id), q.DataIdentifier.Eq(identifier)).Select(q.DataName).Scan(&result)
		if result.DataName == nil {
			return identifier
		}
		return *result.DataName
	}
}

func GetIdentifierNameAttribute() func(device_template_id, identifier string) string {
	return func(device_template_id, identifier string) string {
		q := query.DeviceModelAttribute
		var result model.DeviceModelAttribute
		q.Where(q.DeviceTemplateID.Eq(device_template_id), q.DataIdentifier.Eq(identifier)).Select(q.DataName).Scan(&result)
		if result.DataName == nil {
			return identifier
		}
		return *result.DataName
	}
}
func GetIdentifierNameEvent() func(device_template_id, identifier string) string {
	return func(device_template_id, identifier string) string {
		q := query.DeviceModelEvent
		var result model.DeviceModelEvent
		q.Where(q.DeviceTemplateID.Eq(device_template_id), q.DataIdentifier.Eq(identifier)).Select(q.DataName).Scan(&result)
		if result.DataName == nil {
			return identifier
		}
		return *result.DataName
	}
}

// Check for duplicate DataIdentifier functions
func CheckTelemetryDataIdentifierExists(deviceTemplateID, tenantID, dataIdentifier string) (bool, error) {
	count, err := query.DeviceModelTelemetry.Where(
		query.DeviceModelTelemetry.DeviceTemplateID.Eq(deviceTemplateID),
		query.DeviceModelTelemetry.TenantID.Eq(tenantID),
		query.DeviceModelTelemetry.DataIdentifier.Eq(dataIdentifier),
	).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func CheckAttributeDataIdentifierExists(deviceTemplateID, tenantID, dataIdentifier string) (bool, error) {
	count, err := query.DeviceModelAttribute.Where(
		query.DeviceModelAttribute.DeviceTemplateID.Eq(deviceTemplateID),
		query.DeviceModelAttribute.TenantID.Eq(tenantID),
		query.DeviceModelAttribute.DataIdentifier.Eq(dataIdentifier),
	).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func CheckEventDataIdentifierExists(deviceTemplateID, tenantID, dataIdentifier string) (bool, error) {
	count, err := query.DeviceModelEvent.Where(
		query.DeviceModelEvent.DeviceTemplateID.Eq(deviceTemplateID),
		query.DeviceModelEvent.TenantID.Eq(tenantID),
		query.DeviceModelEvent.DataIdentifier.Eq(dataIdentifier),
	).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func CheckCommandDataIdentifierExists(deviceTemplateID, tenantID, dataIdentifier string) (bool, error) {
	count, err := query.DeviceModelCommand.Where(
		query.DeviceModelCommand.DeviceTemplateID.Eq(deviceTemplateID),
		query.DeviceModelCommand.TenantID.Eq(tenantID),
		query.DeviceModelCommand.DataIdentifier.Eq(dataIdentifier),
	).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
