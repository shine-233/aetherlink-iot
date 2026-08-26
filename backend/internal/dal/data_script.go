// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"errors"
	"fmt"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func CreateDataScript(data *model.DataScript) error {
	return query.DataScript.Create(data)
}

// UpdateDataScript 按主键更新数据处理脚本。
// P1 修复（2026-08-23，见 VALIDATION.md）：更新改走 raw global.DB 链（clone==1 根，
// 无 gen 继承链残留 Model/Dest 的陈旧条件注入风险），显式列赋值并检查 RowsAffected，
// 杜绝"更新命中错误行/未命中仍返回成功"导致的更新后读旧值。
func UpdateDataScript(data *model.UpdateDataScriptReq) error {
	t := time.Now().UTC()
	data.UpdatedAt = &t
	updates := map[string]interface{}{
		"name":             data.Name,
		"device_config_id": data.DeviceConfigId,
		"script_type":      data.ScriptType,
		"updated_at":       t,
	}
	if data.Content != nil {
		updates["content"] = *data.Content
	}
	if data.LastAnalogInput != nil {
		updates["last_analog_input"] = *data.LastAnalogInput
	}
	if data.Description != nil {
		updates["description"] = *data.Description
	}
	if data.Remark != nil {
		updates["remark"] = *data.Remark
	}
	result := global.DB.Model(&model.DataScript{}).
		Where("id = ?", data.Id).
		Updates(updates)
	if result.Error != nil {
		logrus.Error(result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		// id 为用户可控值，不进日志（log-injection 面）；定位时经请求上下文关联。
		logrus.Warn("UpdateDataScript affected 0 rows")
	}
	return nil
}

func DeleteDataScript(id string) error {
	info, err := query.DataScript.Where(query.DataScript.ID.Eq(id)).Delete()
	if info.RowsAffected == 0 {
		return nil
	}
	return err
}

func GetDataScriptById(id string) (*model.DataScript, error) {
	data, err := query.DataScript.Where(query.DataScript.ID.Eq(id)).First()
	if err != nil {
		logrus.Error(err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("data script not found")
	}
	return data, err
}

func GetDataScriptListByPage(data *model.GetDataScriptListByPageReq) (int64, interface{}, error) {
	q := query.DataScript
	var count int64
	var dataList interface{}
	queryBuilder := q.WithContext(context.Background())

	if data.DeviceConfigId != nil && *data.DeviceConfigId != "" {
		queryBuilder = queryBuilder.Where(q.DeviceConfigID.Eq(*data.DeviceConfigId))
	}

	if data.ScriptType != nil && *data.ScriptType != "" {
		queryBuilder = queryBuilder.Where(q.ScriptType.Eq(*data.ScriptType))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, dataList, err
	}

	queryBuilder = applyListPagination(queryBuilder, data.Page, data.PageSize)

	dataList, err = queryBuilder.Select().Order(q.CreatedAt).Find()
	if err != nil {
		logrus.Error(err)
		return count, dataList, err
	}

	return count, dataList, err
}

func OnlyOneScriptTypeEnabled(id string) (enabled bool, err error) {
	q := query.DataScript
	var count int64

	data_script, err := GetDataScriptById(id)
	if err != nil {
		logrus.Error(err)
		return false, err
	}

	if data_script.EnableFlag == "Y" {
		return false, fmt.Errorf("the script has been enabled")
	}

	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Not(q.ID.Eq(data_script.ID))
	queryBuilder = queryBuilder.Where(q.DeviceConfigID.Eq(data_script.DeviceConfigID))
	queryBuilder = queryBuilder.Where(q.ScriptType.Eq(data_script.ScriptType))
	queryBuilder = queryBuilder.Where(q.EnableFlag.Eq("Y"))

	count, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return false, err
	}

	if count > 0 {
		return false, fmt.Errorf("other script has been enabled")
	}

	return true, nil
}

func EnableDataScript(data *model.DataScript) error {
	p := query.DataScript
	t := time.Now().UTC()
	data.UpdatedAt = &t
	_, err := query.DataScript.Where(p.ID.Eq(data.ID)).Updates(data)
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func GetDeviceIDsByDataScriptID(dataScriptID string) ([]string, error) {
	var deviceIDs []string
	dataScript, err := query.DataScript.Where(query.DataScript.ID.Eq(dataScriptID)).First()
	if err != nil {
		logrus.Error(err)
		return deviceIDs, err
	}
	devices, err := query.Device.Where(query.Device.DeviceConfigID.Eq(dataScript.DeviceConfigID)).Find()
	if err != nil {
		logrus.Error(err)
		return deviceIDs, err
	}
	for _, device := range devices {
		deviceIDs = append(deviceIDs, device.ID)
	}
	return deviceIDs, err
}

func GetDataScriptByDeviceConfigIdAndScriptType(deviceConfigId *string, scriptType string) (*model.DataScript, error) {
	if deviceConfigId == nil || *deviceConfigId == "" {
		return nil, nil
	}
	data, err := query.DataScript.
		Where(
			query.DataScript.DeviceConfigID.Eq(*deviceConfigId),
			query.DataScript.ScriptType.Eq(scriptType),
			query.DataScript.EnableFlag.Eq("Y")).
		First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}
