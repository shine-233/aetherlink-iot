// 文件用途：数据转发规则 DAL——raw 链 CRUD、分页与启用规则加载。
// 核心逻辑：全部查询以 global.DB/global.DB.Table 为 clone==1 干净起点（gen 继承面收敛
// 家族约定），租户隔离条件在每条语句内显式携带。
// 关键注意事项：本文件不做脚本执行与网络投递，仅负责规则持久化读取。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
)

// CreateForwardRule 新建规则。
func CreateForwardRule(rule *model.ForwardRule) error {
	if err := global.DB.Create(rule).Error; err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return nil
}

// UpdateForwardRule 更新规则（按 id + 租户双条件，未命中报错）。
func UpdateForwardRule(rule *model.ForwardRule, tenantID string) error {
	res := global.DB.Model(&model.ForwardRule{}).
		Where("id = ? AND tenant_id = ?", rule.ID, tenantID).
		Updates(map[string]interface{}{
			"name":               rule.Name,
			"enabled":            rule.Enabled,
			"source_type":        rule.SourceType,
			"device_template_id": rule.DeviceTemplateID,
			"script":             rule.Script,
			"target_type":        rule.TargetType,
			"http_url":           rule.HttpURL,
			"http_method":        rule.HttpMethod,
			"http_headers":       rule.HttpHeaders,
			"mqtt_broker":        rule.MqttBroker,
			"mqtt_topic":         rule.MqttTopic,
			"mqtt_username":      rule.MqttUsername,
			"mqtt_password":      rule.MqttPassword,
			"remark":             rule.Remark,
			"updated_at":         rule.UpdatedAt,
		})
	if res.Error != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": res.Error.Error()})
	}
	if res.RowsAffected == 0 {
		return errcode.NewWithMessage(errcode.CodeNotFound, "forward rule not found")
	}
	return nil
}

// DeleteForwardRule 删除规则。
func DeleteForwardRule(id, tenantID string) error {
	res := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&model.ForwardRule{})
	if res.Error != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": res.Error.Error()})
	}
	if res.RowsAffected == 0 {
		return errcode.NewWithMessage(errcode.CodeNotFound, "forward rule not found")
	}
	return nil
}

// SetForwardRuleEnabled 启停切换。
func SetForwardRuleEnabled(id, tenantID string, enabled bool) error {
	res := global.DB.Model(&model.ForwardRule{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Update("enabled", enabled)
	if res.Error != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": res.Error.Error()})
	}
	if res.RowsAffected == 0 {
		return errcode.NewWithMessage(errcode.CodeNotFound, "forward rule not found")
	}
	return nil
}

// GetForwardRuleByID 单条读取。
func GetForwardRuleByID(id, tenantID string) (*model.ForwardRule, error) {
	var rule model.ForwardRule
	err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetForwardRuleListByPage 分页查询（租户隔离 + 可选过滤）。
func GetForwardRuleListByPage(req *model.GetForwardRuleListByPageReq, tenantID string) (int64, []*model.ForwardRule, error) {
	db := global.DB.Table(model.TableNameForwardRule).Where("tenant_id = ?", tenantID)
	if req.Name != nil && *req.Name != "" {
		// 跨库大小写不敏感匹配（PG 支持 ILIKE，SQLite 不支持，统一用 LOWER+LIKE）。
		db = db.Where("LOWER(name) LIKE LOWER(?)", "%"+*req.Name+"%")
	}
	if req.Enabled != nil {
		db = db.Where("enabled = ?", *req.Enabled)
	}
	if req.SourceType != nil && *req.SourceType != "" {
		db = db.Where("source_type = ?", *req.SourceType)
	}
	if req.TargetType != nil && *req.TargetType != "" {
		db = db.Where("target_type = ?", *req.TargetType)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return 0, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}

	page, pageSize := req.Page, req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	rows := make([]*model.ForwardRule, 0, pageSize)
	if err := db.Select("id", "tenant_id", "name", "enabled", "source_type", "target_type",
		"http_url", "http_method", "mqtt_broker", "mqtt_topic", "remark", "created_at", "updated_at").
		Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return 0, nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return total, rows, nil
}

// ListEnabledForwardRules 全量加载某来源类型的启用规则（引擎缓存刷新用，跨租户：
// 消息自带 tenant_id，投递时按规则配置原样使用；规则本身按租户创建隔离管理）。
func ListEnabledForwardRules(sourceType string) ([]*model.ForwardRule, error) {
	rows := make([]*model.ForwardRule, 0, 8)
	err := global.DB.Table(model.TableNameForwardRule).
		Where("enabled = true AND source_type = ?", sourceType).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
