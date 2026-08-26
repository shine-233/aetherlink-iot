// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"context"

	"github.com/sirupsen/logrus"
)

func BatchCreateRGroupDevice(r []*model.RGroupDevice) error {
	return query.RGroupDevice.CreateInBatches(r, len(r))
}

func DeleteRGroupDevice(group_id, device_id string) error {
	_, err := query.RGroupDevice.
		Where(query.RGroupDevice.GroupID.Eq(group_id)).
		Where(query.RGroupDevice.DeviceID.Eq(device_id)).
		Delete()
	return err
}

func DeleteRGroupDeviceByDeviceID(deviceID string) error {
	_, err := query.RGroupDevice.
		Where(query.RGroupDevice.DeviceID.Eq(deviceID)).
		Delete()
	return err
}

func GetRGroupDeviceByGroupId(req model.GetDeviceListByGroup, tenantID string, ownerUserID *string) (int64, interface{}, error) {
	// 获取分组下设备,分页返回
	q := query.RGroupDevice
	var devicesList []model.GetDeviceListByGroupRsp
	queryBuilder := q.WithContext(context.Background())
	d := query.Device
	c := query.DeviceConfig
	queryBuilder = queryBuilder.
		LeftJoin(d, d.ID.EqCol(q.DeviceID)).
		LeftJoin(c, c.ID.EqCol(d.DeviceConfigID)).
		Where(q.GroupID.Eq(req.GroupId)).
		Where(d.TenantID.Eq(tenantID)).
		Where(d.ActivateFlag.Eq("active"))
	if ownerUserID != nil && *ownerUserID != "" {
		queryBuilder = queryBuilder.Where(d.OwnerUserID.Eq(*ownerUserID))
	}
	var count int64
	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, devicesList, err
	}

	queryBuilder = applyListPagination(queryBuilder, req.Page, req.PageSize)
	err = queryBuilder.Select(q.GroupID, d.ID, d.DeviceNumber, d.Name, d.DeviceConfigID.As("device_config_id"), c.Name.As("device_config_name")).
		Order(d.CreatedAt.Desc()).
		Scan(&devicesList)
	if err != nil {
		logrus.Error(err)
		return count, devicesList, err
	}

	return count, devicesList, err
}

// 获取分组下设备下拉菜单
// 返回设备id、设备名称、设备配置id、设备配置名称
func GetDeviceSelectByGroupId(tenantId string, group_id string, deviceName string, bindConfig int, ownerUserID *string) ([]map[string]interface{}, error) {
	var data []map[string]interface{}
	rgd := query.RGroupDevice
	d := query.Device
	dc := query.DeviceConfig
	query := rgd.
		Select(rgd.DeviceID.As("id"), d.Name, d.DeviceConfigID, dc.Name.As("device_config_name")).
		Join(d, d.ID.EqCol(rgd.DeviceID)).
		Join(dc, d.DeviceConfigID.EqCol(dc.ID)).
		Where(rgd.GroupID.Eq(group_id)).
		Where(d.TenantID.Eq(tenantId)).
		Where(d.ActivateFlag.Eq("active")). // 激活状态
		Where(d.Name.Like("%" + deviceName + "%")).Order(d.CreatedAt.Desc())
	switch bindConfig {
	case 1:
		query = query.Where(d.DeviceConfigID.IsNotNull())
	case 2:
		query = query.Where(d.DeviceConfigID.IsNull())
	}
	if ownerUserID != nil && *ownerUserID != "" {
		query = query.Where(d.OwnerUserID.Eq(*ownerUserID))
	}
	return data, query.Scan(&data)
}

func GetRGroupDeviceByDeviceId(device_id string) ([]*model.RGroupDevice, error) {
	data, err := query.RGroupDevice.Where(query.RGroupDevice.DeviceID.Eq(device_id)).Find()
	return data, err
}

func GetDeviceIdsByGroupIds(group_ids []string) ([]string, error) {
	data, err := query.RGroupDevice.Where(query.RGroupDevice.GroupID.In(group_ids...)).Select(query.RGroupDevice.DeviceID).Find()
	var deviceIds []string
	for i := range data {
		deviceIds = append(deviceIds, data[i].DeviceID)
	}
	return deviceIds, err
}

func GetDeviceIdsByDeviceConfigId(deviceConfigIds []string) ([]string, error) {
	var result []string
	err := query.Device.Where(query.Device.DeviceConfigID.In(deviceConfigIds...)).Pluck(query.Device.ID, &result)

	return result, err
}
