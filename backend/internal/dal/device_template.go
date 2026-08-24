// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

const (
	DEVICE_TEMPLATE_PRIVATE = int16(1)
	DEVICE_TEMPLATE_PUBLIC  = int16(2)
)

func CreateDeviceTemplate(device *model.DeviceTemplate) (*model.DeviceTemplate, error) {

	return device, query.DeviceTemplate.Create(device)
}

func GetDeviceTemplateById(id string) (*model.DeviceTemplate, error) {
	template, err := query.DeviceTemplate.Where(query.DeviceTemplate.ID.Eq(id)).First()
	if err != nil {
		return template, err
	}
	if template == nil {
		return nil, fmt.Errorf("thing model not found: id=%s", id)
	}
	return template, err
}

// GetDeviceTemplateForTenant returns a thing model within an explicit tenant
// boundary. It is used by outbound publishing paths that must not trust a bare ID.
func GetDeviceTemplateForTenant(id, tenantID string) (*model.DeviceTemplate, error) {
	q := query.DeviceTemplate
	template, err := q.Where(q.ID.Eq(id), q.TenantID.Eq(tenantID)).First()
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("thing model not found: id=%s tenant_id=%s", id, tenantID)
	}
	return template, nil
}

func GetDeviceTemplateChartConfigByID(id, tenantID string) (*model.DeviceTemplate, error) {
	q := query.DeviceTemplate
	template, err := q.Select(q.ID, q.WebChartConfig, q.AppChartConfig).Where(q.ID.Eq(id), q.TenantID.Eq(tenantID)).First()
	if err != nil {
		return template, err
	}
	if template == nil {
		return nil, fmt.Errorf("thing model chart config not found: id=%s", id)
	}
	return template, nil
}

// GetDeviceTemplateByDeviceId 根据设备ID获取物模型。
// gen 继承面收敛（2026-08-24）：原 query.Device/DeviceConfig/DeviceTemplate 三单例链在
// 并发下继承残留 Statement（同 devices 列表读旧快照家族，CI 实证 /device/template/chart
// 间歇 101001），改为 global.DB raw 链重建等价三表 LEFT JOIN；Scan 空行/nil id 的
// 兼容分支逐字节保留。
func GetDeviceTemplateByDeviceId(deviceId string) (any, error) {
	var rsp map[string]interface{}
	err := global.DB.Table("devices").
		Joins("LEFT JOIN device_configs ON device_configs.id = devices.device_config_id").
		Joins("LEFT JOIN device_templates ON device_templates.id = device_configs.device_template_id").
		Where("devices.id = ?", deviceId).
		Select("device_templates.*").
		Scan(&rsp).Error
	if err != nil {
		return nil, err
	}
	// 判断rsp是否有key为id
	if v, ok := rsp["id"]; ok {
		if v == nil {
			//返回{}，而不是nil
			return map[string]interface{}{}, nil
		}
	}
	return rsp, err
}

func UpdateDeviceTemplate(data *model.DeviceTemplate) (*model.DeviceTemplate, error) {
	info, err := query.DeviceTemplate.Where(query.DeviceTemplate.ID.Eq(data.ID)).Updates(data)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	if info.RowsAffected == 0 {
		return nil, fmt.Errorf("update thing model failed, no rows affected")
	}
	return data, err
}

func DeleteDeviceTemplate(id string) error {
	_, err := query.DeviceTemplate.Where(query.DeviceTemplate.ID.Eq(id)).Delete()
	return err
}

func GetDeviceTemplateListByPage(req *model.GetDeviceTemplateListByPageReq, claims *utils.UserClaims) (int64, interface{}, error) {

	if req.Page <= 0 || req.PageSize <= 0 {
		return 0, nil, fmt.Errorf("page and pageSize must be greater than 0")
	}

	q := query.DeviceTemplate
	var count int64
	queryBuilder := q.WithContext(context.Background())
	if req.Name != nil {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *req.Name)))
	}
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(claims.TenantID))
	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	// toal_pages向上取整
	// total_pages := int64(math.Ceil(float64(count) / float64(req.PageSize)))

	queryBuilder = queryBuilder.Limit(req.PageSize)
	queryBuilder = queryBuilder.Offset((req.Page - 1) * req.PageSize)
	queryBuilder = queryBuilder.Order(q.CreatedAt.Desc())
	datalist, err := queryBuilder.Find()
	if err != nil {
		logrus.Error("queryBuilder.Find error: ", err)
	}
	return count, datalist, err

}

func GetDeviceTemplateMenu(req *model.GetDeviceTemplateMenuReq, claims *utils.UserClaims) (interface{}, error) {

	q := query.DeviceTemplate
	queryBuilder := q.WithContext(context.Background())
	if req.Name != nil {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *req.Name)))
	}
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(claims.TenantID))
	var data []map[string]interface{}
	err := queryBuilder.Select(q.ID, q.Name).Order(q.CreatedAt.Desc()).Scan(&data)
	if err != nil {
		logrus.Error("queryBuilder.Find error: ", err)
	}
	return data, err

}

// GetDeviceTemplateStats 获取设备物模型统计信息
func GetDeviceTemplateStats(deviceTemplateID string, tenantID string) (*model.GetDeviceTemplateStatsRsp, error) {
	ctx := context.Background()

	// 查询物模型基本信息
	dt := query.DeviceTemplate
	template, err := dt.WithContext(ctx).
		Where(dt.ID.Eq(deviceTemplateID), dt.TenantID.Eq(tenantID)).
		First()
	if err != nil {
		logrus.Error("query thing model error: ", err)
		return nil, err
	}

	// 统计关联设备总数和在线设备数
	// 通过 device_configs 表关联 devices 表
	dc := query.DeviceConfig
	d := query.Device

	// 统计总设备数
	totalDevices, err := d.WithContext(ctx).
		Join(dc, dc.ID.EqCol(d.DeviceConfigID)).
		Where(dc.DeviceTemplateID.Eq(deviceTemplateID), d.TenantID.Eq(tenantID)).
		Count()
	if err != nil {
		logrus.Error("query total devices error: ", err)
		return nil, err
	}

	// 统计在线设备数
	onlineDevices, err := d.WithContext(ctx).
		Join(dc, dc.ID.EqCol(d.DeviceConfigID)).
		Where(dc.DeviceTemplateID.Eq(deviceTemplateID), d.TenantID.Eq(tenantID), d.IsOnline.Eq(1)).
		Count()
	if err != nil {
		logrus.Error("query online devices error: ", err)
		return nil, err
	}

	// 构造返回结果
	label := ""
	if template.Label != nil {
		label = *template.Label
	}

	result := &model.GetDeviceTemplateStatsRsp{
		DeviceTemplateID: template.ID,
		Name:             template.Name,
		Label:            label,
		TotalDevices:     totalDevices,
		OnlineDevices:    onlineDevices,
	}

	return result, nil
}

// GetDeviceTemplateSelector 获取设备物模型选择器列表（不分页）
func GetDeviceTemplateSelector(req *model.GetDeviceTemplateSelectorReq, tenantID string) ([]*model.GetDeviceTemplateSelectorRsp, error) {
	ctx := context.Background()
	q := query.DeviceTemplate

	queryBuilder := q.WithContext(ctx).Where(q.TenantID.Eq(tenantID))

	// 物模型ID精确查询
	if req.DeviceTemplateID != nil && *req.DeviceTemplateID != "" {
		queryBuilder = queryBuilder.Where(q.ID.Eq(*req.DeviceTemplateID))
	}

	// 物模型名称模糊匹配
	if req.Name != nil && *req.Name != "" {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *req.Name)))
	}

	// 标签模糊匹配
	if req.Label != nil && *req.Label != "" {
		queryBuilder = queryBuilder.Where(q.Label.Like(fmt.Sprintf("%%%s%%", *req.Label)))
	}

	// 查询ID、Name和Label字段
	var results []*model.GetDeviceTemplateSelectorRsp
	err := queryBuilder.Select(q.ID, q.Name, q.Label).Order(q.UpdatedAt.Desc()).Scan(&results)
	if err != nil {
		logrus.Error("query thing model selector error: ", err)
		return nil, err
	}

	return results, nil
}
