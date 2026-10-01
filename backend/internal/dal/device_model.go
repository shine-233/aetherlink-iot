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
	"gorm.io/gen"
	"gorm.io/gen/field"
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

// deviceModelListDo 抽象四类物模型（遥测/属性/事件/命令）gen 构建器的共同形状，
// 让分页查询共用一份实现：Where/Limit/Offset/Select 返回自身类型，Count/Find 终结。
type deviceModelListDo[Q any, T any] interface {
	Where(conds ...gen.Condition) Q
	Limit(limit int) Q
	Offset(offset int) Q
	Select(conds ...field.Expr) Q
	Count() (int64, error)
	Find() ([]*T, error)
}

// listDeviceModelByPage 按作用域 + 模板 id 分页查询物模型定义。空作用域 fail-closed
// 返回空结果；单元素作用域走 Eq（与旧单租户语义一致），多元素走 In。
func listDeviceModelByPage[Q deviceModelListDo[Q, T], T any](qb Q, tenantID, templateID field.String, r model.GetDeviceModelListByPageReq, scopes []string) (count int64, data []*T, err error) {
	if len(scopes) == 0 {
		return count, data, nil
	}
	if len(scopes) == 1 {
		qb = qb.Where(tenantID.Eq(scopes[0]))
	} else {
		qb = qb.Where(tenantID.In(scopes...))
	}
	qb = qb.Where(templateID.Eq(r.DeviceTemplateId))
	count, err = qb.Count()
	if err != nil {
		logrus.Error(err)
		return count, data, err
	}
	data, err = applyListPagination(qb, r.Page, r.PageSize).Select().Find()
	if err != nil {
		logrus.Error(err)
	}
	return count, data, err
}

// GetDeviceModelTelemetryListByPage 分页查询某模板的遥测物模型定义（tenant-scope: caller-enforced；
// scopes 由 service 层展开——总部/父级可见 self∪子孙模板的物模型）。
func GetDeviceModelTelemetryListByPage(r model.GetDeviceModelListByPageReq, scopes []string) (int64, []*model.DeviceModelTelemetry, error) {
	q := query.DeviceModelTelemetry
	return listDeviceModelByPage[query.IDeviceModelTelemetryDo, model.DeviceModelTelemetry](q.WithContext(context.Background()), q.TenantID, q.DeviceTemplateID, r, scopes)
}

// GetDeviceModelAttributesListByPage 分页查询某模板的属性物模型定义（tenant-scope: caller-enforced）。
func GetDeviceModelAttributesListByPage(r model.GetDeviceModelListByPageReq, scopes []string) (int64, []*model.DeviceModelAttribute, error) {
	q := query.DeviceModelAttribute
	return listDeviceModelByPage[query.IDeviceModelAttributeDo, model.DeviceModelAttribute](q.WithContext(context.Background()), q.TenantID, q.DeviceTemplateID, r, scopes)
}

// GetDeviceModelEventsListByPage 分页查询某模板的事件物模型定义（tenant-scope: caller-enforced）。
func GetDeviceModelEventsListByPage(r model.GetDeviceModelListByPageReq, scopes []string) (int64, []*model.DeviceModelEvent, error) {
	q := query.DeviceModelEvent
	return listDeviceModelByPage[query.IDeviceModelEventDo, model.DeviceModelEvent](q.WithContext(context.Background()), q.TenantID, q.DeviceTemplateID, r, scopes)
}

// GetDeviceModelCommandsListByPage 分页查询某模板的命令物模型定义（tenant-scope: caller-enforced）。
func GetDeviceModelCommandsListByPage(r model.GetDeviceModelListByPageReq, scopes []string) (int64, []*model.DeviceModelCommand, error) {
	q := query.DeviceModelCommand
	return listDeviceModelByPage[query.IDeviceModelCommandDo, model.DeviceModelCommand](q.WithContext(context.Background()), q.TenantID, q.DeviceTemplateID, r, scopes)
}

// tenant-scope: parent-owned?2026-08-26 ?????
func GetDeviceModelEventDataList(device_template_id string) ([]*model.DeviceModelEvent, error) {
	data, err := query.DeviceModelEvent.
		Where(query.DeviceModelEvent.DeviceTemplateID.Eq(device_template_id)).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: parent-owned?2026-08-26 ?????
func GetDeviceModelCommandDataList(device_template_id string) ([]*model.DeviceModelCommand, error) {
	data, err := query.DeviceModelCommand.
		Where(query.DeviceModelCommand.DeviceTemplateID.Eq(device_template_id)).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: parent-owned?2026-08-26 ?????
func GetDeviceModelTelemetryDataList(device_template_id string) ([]*model.DeviceModelTelemetry, error) {
	data, err := query.DeviceModelTelemetry.
		Where(query.DeviceModelTelemetry.DeviceTemplateID.Eq(device_template_id)).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// tenant-scope: parent-owned?2026-08-26 ?????
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

// tenant-scope: parent-owned?2026-08-26 ?????
func GetIdentifierNameTelemetry() func(device_template_id, identifier string) string {
	return func(device_template_id, identifier string) string {
		q := query.DeviceModelTelemetry
		var result model.DeviceModelTelemetry
		if err := q.Where(q.DeviceTemplateID.Eq(device_template_id), q.DataIdentifier.Eq(identifier)).Select(q.DataName).Scan(&result); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"kind": "telemetry", "device_template_id": device_template_id, "identifier": identifier}).Warn("resolve identifier display name failed")
		}
		if result.DataName == nil {
			return identifier
		}
		return *result.DataName
	}
}

// tenant-scope: parent-owned?2026-08-26 ?????
func GetIdentifierNameAttribute() func(device_template_id, identifier string) string {
	return func(device_template_id, identifier string) string {
		q := query.DeviceModelAttribute
		var result model.DeviceModelAttribute
		if err := q.Where(q.DeviceTemplateID.Eq(device_template_id), q.DataIdentifier.Eq(identifier)).Select(q.DataName).Scan(&result); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"kind": "attribute", "device_template_id": device_template_id, "identifier": identifier}).Warn("resolve identifier display name failed")
		}
		if result.DataName == nil {
			return identifier
		}
		return *result.DataName
	}
}

// tenant-scope: parent-owned?2026-08-26 ?????
func GetIdentifierNameEvent() func(device_template_id, identifier string) string {
	return func(device_template_id, identifier string) string {
		q := query.DeviceModelEvent
		var result model.DeviceModelEvent
		if err := q.Where(q.DeviceTemplateID.Eq(device_template_id), q.DataIdentifier.Eq(identifier)).Select(q.DataName).Scan(&result); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"kind": "event", "device_template_id": device_template_id, "identifier": identifier}).Warn("resolve identifier display name failed")
		}
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
