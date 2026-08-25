// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"errors"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func GetEventDatasListByPage(req *model.GetEventDatasListByPageReq) (int64, []map[string]interface{}, error) {

	var count int64
	// P1 修复（2026-08-24，见 VALIDATION.md）：事件数据列表改走 raw global.DB 链，
	// 杜绝包级单例 EventData 三级 LeftJoin(devices→device_configs→device_model_events)
	// 在高并发下跨请求残留 Statement 读到空/旧数据；JOIN 形态、投影列名、
	// 排序与分页语义与收敛前逐条一致。
	base := global.DB.Table("event_datas").
		Joins("LEFT JOIN devices ON devices.id = event_datas.device_id").
		Joins("LEFT JOIN device_configs ON devices.device_config_id = device_configs.id").
		Joins("LEFT JOIN device_model_events ON device_configs.device_template_id = device_model_events.device_template_id AND device_model_events.data_identifier = event_datas.identify").
		Where("event_datas.device_id = ?", req.DeviceId)

	if req.Identify != nil && *req.Identify != "" {
		base = base.Where("event_datas.identify = ?", *req.Identify)
	}

	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	listBuilder := base.Session(&gorm.Session{}).
		Select("event_datas.*, device_model_events.data_name").
		Order("event_datas.ts DESC")
	listBuilder = applyListPagination(listBuilder, req.Page, req.PageSize)
	list := make([]map[string]interface{}, 0)
	if err := listBuilder.Scan(&list).Error; err != nil {
		logrus.Error(err)
		return count, list, err
	}

	return count, list, nil

}

// CreateEventData 创建事件数据
func GetDeviceEventOneKeys(deviceId string, keys string) (string, error) {
	data, err := query.EventData.Where(query.EventData.DeviceID.Eq(deviceId), query.EventData.Identify.Eq(keys)).Order(query.EventData.T.Desc()).First()
	var result string
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	} else if err != nil {
		return result, err
	}

	if data.Datum != nil {
		result = *data.Datum
	}
	return result, nil
}

// 删除命令历史数据，带事务
func DeleteEventDataByDeviceId(deviceId string, tx *query.QueryTx) error {
	_, err := tx.EventData.Where(query.EventData.DeviceID.Eq(deviceId)).Delete()
	return err
}

// 获取设备单指标最新值,如果数据不存在，返回nil
func GetEventDataOneKeysByDeviceId(deviceId string, keys string) (*model.EventData, error) {
	data, err := query.EventData.Where(query.EventData.DeviceID.Eq(deviceId), query.EventData.Identify.Eq(keys)).Order(query.EventData.T.Desc()).First()
	if err != nil {
		return &model.EventData{}, err
	}
	return data, nil
}
