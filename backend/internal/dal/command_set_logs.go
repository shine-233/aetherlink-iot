// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"aetherlink-iot/backend/pkg/constant"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCommandSetLogsDataListByPage(req model.GetCommandSetLogsListByPageReq) (int64, any, error) {

	var count int64
	// P1 修复（2026-08-24，见 VALIDATION.md）：命令下发日志列表改走 raw global.DB 链，
	// 杜绝包级单例 CommandSetLog 四级 LeftJoin(devices→device_configs→device_model_commands→users)
	// 在高并发下跨请求残留 Statement 读到空/旧数据；JOIN 形态、投影列名、排序与分页语义与收敛前逐条一致。
	base := global.DB.Table("command_set_logs").
		Joins("LEFT JOIN devices ON devices.id = command_set_logs.device_id").
		Joins("LEFT JOIN device_configs ON device_configs.id = devices.device_config_id").
		Joins("LEFT JOIN device_model_commands ON device_model_commands.device_template_id = device_configs.device_template_id AND device_model_commands.data_identifier = command_set_logs.identify").
		Joins("LEFT JOIN users ON users.id = command_set_logs.user_id").
		Where("command_set_logs.device_id = ?", req.DeviceId)

	if req.Status != nil {
		base = base.Where("command_set_logs.status = ?", *req.Status)
	}
	if req.OperationType != nil {
		base = base.Where("command_set_logs.operation_type = ?", *req.OperationType)
	}
	if req.IdentifyName != nil {
		base = base.Where("device_model_commands.data_name LIKE ?", "%"+*req.IdentifyName+"%")
	}

	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	listBuilder := base.Session(&gorm.Session{}).
		Select("command_set_logs.*, device_model_commands.data_name AS identify_name, users.name AS username").
		Order("command_set_logs.created_at DESC")
	listBuilder = applyListPagination(listBuilder, req.Page, req.PageSize)
	list := make([]map[string]interface{}, 0)
	if err := listBuilder.Scan(&list).Error; err != nil {
		logrus.Error(err)
		return count, list, err
	}

	return count, list, nil

}

type CommandSetLogsQuery struct {
}

func (CommandSetLogsQuery) Create(ctx context.Context, info *model.CommandSetLog) (id string, err error) {
	command := query.CommandSetLog

	err = command.WithContext(ctx).Create(info)
	if err != nil {
		logrus.Error("[CommandSetLogsQuery]create failed:", err)
	}
	return info.ID, err
}

func (CommandSetLogsQuery) CommandResultUpdate(ctx context.Context, logId string, response model.MqttResponse) {
	command := query.CommandSetLog
	valueByte, _ := json.Marshal(response)
	values := string(valueByte)
	updates := model.CommandSetLog{
		RspDatum: &values,
	}
	if response.Result == 0 {
		status := strconv.Itoa(constant.ResponseStatusOk)
		updates.Status = &status
		updates.ErrorMessage = &response.Message
		//updates["status"] = constant.CommandStatusOk
	} else {
		//updates["status"] = constant.CommandStatusFailed
		//updates["error_message"] = response.Message
		status := strconv.Itoa(constant.ResponseSStatusFailed)
		updates.Status = &status
		updates.ErrorMessage = &response.Message
	}
	//updates["rsp_data"] = string(values)
	_, err := command.WithContext(ctx).Where(command.ID.Eq(logId)).Updates(updates)
	if err != nil {
		logrus.Error("[CommandSetLogsQuery]create failed:", err)
	}

}

func (CommandSetLogsQuery) Update(ctx context.Context, info *model.CommandSetLog) error {
	command := query.CommandSetLog

	result, err := command.WithContext(ctx).Where(command.MessageID.Eq(*info.MessageID)).Updates(info)
	if err != nil {
		logrus.Error("[CommandSetLogsQuery]update failed:", err)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no data updated")
	}
	return err
}

func (CommandSetLogsQuery) FilterOneHourByMessageID(messageId string) (*model.CommandSetLog, error) {
	command := query.CommandSetLog
	nowTime := time.Now().UTC()

	log, err := command.Where(command.MessageID.Eq(messageId)).
		Where(command.CreatedAt.Gte(nowTime.Add(-time.Hour))).
		Select().
		First()
	if err != nil {
		logrus.Error("[CommandSetLogsQuery]FilterByMessageID failed:", err)
	}
	return log, err

}

// 删除命令历史数据，带事务
func DeleteCommandSetLogsByDeviceId(deviceId string, tx *query.QueryTx) error {
	_, err := tx.CommandSetLog.Where(query.CommandSetLog.DeviceID.Eq(deviceId)).Delete()
	return err
}

// CreateCommandSetLog 创建命令日志
func CreateCommandSetLog(log *model.CommandSetLog) error {
	return query.CommandSetLog.Create(log)
}

// GetCommandSetLogByMessageID 根据 message_id 和 device_id 查询日志。
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCommandSetLogByMessageID(messageID string, deviceID string) (*model.CommandSetLog, error) {
	return GetCommandSetLogByMessageIDWithContext(context.Background(), messageID, deviceID)
}

// GetCommandSetLogByMessageIDWithContext lets short request-response callers
// cancel an in-flight lookup when the HTTP request or wait deadline ends.
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCommandSetLogByMessageIDWithContext(ctx context.Context, messageID string, deviceID string) (*model.CommandSetLog, error) {
	return query.CommandSetLog.WithContext(ctx).
		Where(query.CommandSetLog.MessageID.Eq(messageID)).
		Where(query.CommandSetLog.DeviceID.Eq(deviceID)).
		First()
}

// UpdateCommandSetLogDeliveryStatus updates the platform publish state only
// while no device response has reached the durable log. The predicate and
// update execute in one statement so a fast status 3/4 response cannot be
// overwritten by a later status 1/2 delivery callback.
func UpdateCommandSetLogDeliveryStatus(messageID, deviceID, status, errorMessage string) (bool, error) {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}

	result, err := query.CommandSetLog.
		Where(query.CommandSetLog.MessageID.Eq(messageID)).
		Where(query.CommandSetLog.DeviceID.Eq(deviceID)).
		Where(query.CommandSetLog.Status.NotIn(
			strconv.Itoa(constant.ResponseStatusOk),
			strconv.Itoa(constant.ResponseSStatusFailed),
		)).
		Updates(updates)
	if err != nil {
		return false, err
	}
	return result.RowsAffected > 0, nil
}

type CommandSetLogLookup struct {
	DeviceID  string
	MessageID string
}

func CommandSetLogLookupKey(deviceID, messageID string) string {
	return deviceID + "\x00" + messageID
}

// GetCommandSetLogsByDeviceMessageIDs returns command logs keyed by device_id + message_id.
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCommandSetLogsByDeviceMessageIDs(lookups []CommandSetLogLookup) (map[string]*model.CommandSetLog, error) {
	result := make(map[string]*model.CommandSetLog, len(lookups))
	if len(lookups) == 0 {
		return result, nil
	}

	messageIDs := make([]string, 0, len(lookups))
	deviceIDs := make([]string, 0, len(lookups))
	seenMessages := map[string]struct{}{}
	seenDevices := map[string]struct{}{}
	wanted := map[string]struct{}{}
	for _, lookup := range lookups {
		if lookup.DeviceID == "" || lookup.MessageID == "" {
			continue
		}
		wanted[CommandSetLogLookupKey(lookup.DeviceID, lookup.MessageID)] = struct{}{}
		if _, ok := seenMessages[lookup.MessageID]; !ok {
			seenMessages[lookup.MessageID] = struct{}{}
			messageIDs = append(messageIDs, lookup.MessageID)
		}
		if _, ok := seenDevices[lookup.DeviceID]; !ok {
			seenDevices[lookup.DeviceID] = struct{}{}
			deviceIDs = append(deviceIDs, lookup.DeviceID)
		}
	}
	if len(wanted) == 0 {
		return result, nil
	}

	logs, err := query.CommandSetLog.
		Where(query.CommandSetLog.MessageID.In(messageIDs...)).
		Where(query.CommandSetLog.DeviceID.In(deviceIDs...)).
		Find()
	if err != nil {
		return result, err
	}
	for _, log := range logs {
		if log == nil || log.MessageID == nil || *log.MessageID == "" || log.DeviceID == "" {
			continue
		}
		key := CommandSetLogLookupKey(log.DeviceID, *log.MessageID)
		if _, ok := wanted[key]; ok {
			result[key] = log
		}
	}
	return result, nil
}

// GetCommandSetLogsByPage 分页查询命令下发日志
// tenant-scope: caller-enforced?2026-08-26 ?????
func GetCommandSetLogsByPage(req *model.GetCommandSetLogsListByPageReq) ([]*model.CommandSetLog, int64, error) {
	q := query.CommandSetLog.Order(query.CommandSetLog.CreatedAt.Desc())

	// 设备ID过滤（字段名是 DeviceId，不是 DeviceID）
	if req.DeviceId != "" {
		q = q.Where(query.CommandSetLog.DeviceID.Eq(req.DeviceId))
	}

	// 状态过滤
	if req.Status != nil && *req.Status != "" {
		q = q.Where(query.CommandSetLog.Status.Eq(*req.Status))
	}

	// 分页
	offset := (req.Page - 1) * req.PageSize
	logs, total, err := q.FindByPage(offset, req.PageSize)

	return logs, total, err
}

// UpdateCommandSetLog 更新命令日志
func UpdateCommandSetLog(log *model.CommandSetLog) error {
	return query.CommandSetLog.Save(log)
}
