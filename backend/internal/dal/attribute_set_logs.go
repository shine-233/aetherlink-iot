// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"aetherlink-iot/backend/pkg/constant"
	"context"
	"encoding/json"
	"strconv"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
)

func GetAttributeSetLogsDataListByPage(req model.GetAttributeSetLogsListByPageReq) (int64, []*model.AttributeSetLog, error) {

	var count int64
	q := query.AttributeSetLog
	u := query.User
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.DeviceID.Eq(req.DeviceId))
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(q.Status.Eq(*req.Status))
	}
	if req.OperationType != nil {
		queryBuilder = queryBuilder.Where(q.OperationType.Eq(*req.OperationType))
	}
	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	queryBuilder = applyListPagination(queryBuilder, req.Page, req.PageSize)
	queryBuilder = queryBuilder.Order(q.CreatedAt.Desc())
	queryBuilder = queryBuilder.LeftJoin(u, u.ID.EqCol(q.UserID))
	list, err := queryBuilder.Select(q.ALL, u.Name.As("username")).Find()
	if err != nil {
		logrus.Error(err)
		return count, list, err
	}

	return count, list, nil

}

type AttributeSetLogsQuery struct {
}

func (AttributeSetLogsQuery) Create(ctx context.Context, info *model.AttributeSetLog) (id string, err error) {
	attribute := query.AttributeSetLog

	err = attribute.WithContext(ctx).Create(info)
	if err != nil {
		logrus.Error("[AttributeSetLogsQuery]create failed:", err)
	}
	return info.ID, err
}

func (AttributeSetLogsQuery) SetAttributeResultUpdate(ctx context.Context, logId string, response model.MqttResponse) {
	attribute := query.AttributeSetLog
	valueByte, _ := json.Marshal(response)
	values := string(valueByte)
	updates := model.AttributeSetLog{
		RspDatum: &values,
	}
	if response.Result == 0 {
		status := strconv.Itoa(constant.ResponseStatusOk)
		updates.Status = &status
	} else {
		status := strconv.Itoa(constant.ResponseSStatusFailed)
		updates.Status = &status
		updates.ErrorMessage = &response.Message
	}
	//updates["rsp_data"] = string(values)
	_, err := attribute.WithContext(ctx).Where(attribute.ID.Eq(logId)).Updates(updates)
	if err != nil {
		logrus.Error("[CommandSetLogsQuery]create failed:", err)
	}

}

// 根据key查询设备属性
func GetAttributeDataByKey(req model.GetDataListByKeyReq) (*model.AttributeData, error) {
	data, err := query.AttributeData.WithContext(context.Background()).Where(query.AttributeData.DeviceID.Eq(req.DeviceId), query.AttributeData.Key.Eq(req.Key)).First()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return data, nil
}

// 删除属性下发历史数据，带事务
func DeleteAttributeDataByDeviceIdTx(deviceId string, tx *query.QueryTx) error {
	_, err := tx.AttributeData.WithContext(context.Background()).Where(query.AttributeData.DeviceID.Eq(deviceId)).Delete()
	return err
}

// CreateAttributeSetLog 创建属性设置日志
func CreateAttributeSetLog(log *model.AttributeSetLog) error {
	return query.AttributeSetLog.Create(log)
}

// GetAttributeSetLogByMessageID 根据 message_id 和 device_id 查询日志（提升性能）
func GetAttributeSetLogByMessageID(messageID string, deviceID string) (*model.AttributeSetLog, error) {
	return query.AttributeSetLog.
		Where(query.AttributeSetLog.MessageID.Eq(messageID)).
		Where(query.AttributeSetLog.DeviceID.Eq(deviceID)). // ✨ 添加 device_id
		First()
}

// UpdateAttributeSetLog 更新属性设置日志
func UpdateAttributeSetLog(log *model.AttributeSetLog) error {
	return query.AttributeSetLog.Save(log)
}
