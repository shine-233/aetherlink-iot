// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

func GetAttributeDataList(deviceId string) ([]*model.AttributeData, error) {
	data, err := query.AttributeData.
		Where(query.AttributeData.DeviceID.Eq(deviceId)).Find()
	if err != nil {
		return nil, err
	}
	return data, nil
}

/*
select ad.*,dma.data_name from attribute_datas ad
left join devices on ad.device_id = devices.id  left join  device_configs dc on devices.device_config_id = dc.id
left join device_templates dt on dt.id = dc.device_template_id
left join device_model_attributes dma on dt.id = dma.device_template_id and ad.key = dma.data_identifier
where devices.id = 'ca33926c-5ee5-3e9f-147e-94e188fde65b'
*/
// 根据设备ID获取设备属性数据列表并关联查到数据名称如以上sql
func GetAttributeDataListWithDeviceName(deviceId string) ([]map[string]interface{}, error) {
	var data []map[string]interface{}
	err := query.AttributeData.
		Select(query.AttributeData.ALL, query.DeviceModelAttribute.DataName, query.DeviceModelAttribute.Unit, query.DeviceModelAttribute.ReadWriteFlag, query.DeviceModelAttribute.DataType, query.DeviceModelAttribute.AdditionalInfo.As("enum")).
		LeftJoin(query.Device, query.AttributeData.DeviceID.EqCol(query.Device.ID)).
		LeftJoin(query.DeviceConfig, query.Device.DeviceConfigID.EqCol(query.DeviceConfig.ID)).
		LeftJoin(query.DeviceTemplate, query.DeviceConfig.DeviceTemplateID.EqCol(query.DeviceTemplate.ID)).
		LeftJoin(query.DeviceModelAttribute, query.DeviceTemplate.ID.EqCol(query.DeviceModelAttribute.DeviceTemplateID), query.AttributeData.Key.EqCol(query.DeviceModelAttribute.DataIdentifier)).
		Where(query.Device.ID.Eq(deviceId)).Scan(&data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func DeleteAttributeData(id string) error {
	_, err := query.AttributeData.
		Where(query.AttributeData.ID.Eq(id)).
		Delete()
	return err
}

func GetAttributeDataByID(id string) (*model.AttributeData, error) {
	return query.AttributeData.
		Where(query.AttributeData.ID.Eq(id)).
		First()
}

func CreateAttributeData(data *model.AttributeData) error {
	return query.AttributeData.Create(data)
}

// 更新设备属性数据，如果数据不存在，UUID生成一个ID，创建一条新的数据
func UpdateAttributeData(data *model.AttributeData) (*model.AttributeData, error) {
	// 根据新数据的数据类型，直接设置其他类型字段为null
	if data.StringV != nil {
		data.NumberV = nil
		data.BoolV = nil
	} else if data.NumberV != nil {
		data.StringV = nil
		data.BoolV = nil
	} else if data.BoolV != nil {
		data.StringV = nil
		data.NumberV = nil
	}

	// 创建包含null值的更新map，确保null字段也会被更新到数据库
	updateMap := map[string]interface{}{
		"bool_v":   data.BoolV,
		"number_v": data.NumberV,
		"string_v": data.StringV,
		"ts":       data.T,
	}

	// 尝试更新现有记录
	result, err := query.AttributeData.Where(
		query.AttributeData.DeviceID.Eq(data.DeviceID),
		query.AttributeData.TenantID.Eq(*data.TenantID),
		query.AttributeData.Key.Eq(data.Key),
	).Updates(updateMap)

	if err != nil {
		return nil, err
	} else if result.RowsAffected == 0 {
		// 数据不存在，创建新记录
		data.ID = uuid.New()
		err = query.AttributeData.Create(data)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// 获取设备单指标最新值，如果数据不存在，返回nil
func GetAttributeOneKeys(deviceId string, keys string) (interface{}, error) {
	data, err := query.AttributeData.Where(query.AttributeData.DeviceID.Eq(deviceId), query.AttributeData.Key.Eq(keys)).Order(query.AttributeData.T.Desc()).First()
	var result interface{}
	if err != nil {
		return result, err
	} else if err == gorm.ErrRecordNotFound {
		return result, nil
	}
	if data.BoolV != nil {
		// result = fmt.Sprintf("%t", *data.BoolV)
		result = *data.BoolV
	}
	if data.NumberV != nil {
		// result = fmt.Sprintf("%d", data.NumberV)
		result = *data.NumberV
	}
	if data.StringV != nil {
		result = *data.StringV
	}
	return result, nil
}

// 获取设备单指标最新值,如果数据不存在，返回nil
func GetAttributeOneKeysByDeviceId(deviceId string, keys string) (*model.AttributeData, error) {
	data, err := query.AttributeData.Where(query.AttributeData.DeviceID.Eq(deviceId), query.AttributeData.Key.Eq(keys)).Order(query.AttributeData.T.Desc()).First()
	if err != nil {
		return &model.AttributeData{}, err
	}
	return data, nil
}

// 根据设备id删除所有数据
func DeleteAttributeDataByDeviceId(deviceId string, tx *query.QueryTx) error {
	_, err := tx.AttributeData.Where(query.AttributeData.DeviceID.Eq(deviceId)).Delete()
	return err
}
