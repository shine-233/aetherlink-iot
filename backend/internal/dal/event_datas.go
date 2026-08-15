// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"errors"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func GetEventDatasListByPage(req *model.GetEventDatasListByPageReq) (int64, []map[string]interface{}, error) {

	var count int64
	q := query.EventData
	d := query.Device
	dc := query.DeviceConfig
	dme := query.DeviceModelEvent

	queryBuilder := q.WithContext(context.Background())

	queryBuilder = queryBuilder.LeftJoin(d, q.DeviceID.EqCol(d.ID)).
		LeftJoin(dc, d.DeviceConfigID.EqCol(dc.ID)).
		LeftJoin(dme, dc.DeviceTemplateID.EqCol(dme.DeviceTemplateID), dme.DataIdentifier.EqCol(q.Identify)).
		Where(q.DeviceID.Eq(req.DeviceId))

	if req.Identify != nil && *req.Identify != "" {
		queryBuilder = queryBuilder.Where(q.Identify.Eq(*req.Identify))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	if req.Page != 0 && req.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(req.PageSize)
		queryBuilder = queryBuilder.Offset((req.Page - 1) * req.PageSize)
	}
	queryBuilder = queryBuilder.Order(q.T.Desc())
	var list []map[string]interface{}
	err = queryBuilder.Select(q.ALL, dme.DataName).Scan(&list)
	if err != nil {
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
