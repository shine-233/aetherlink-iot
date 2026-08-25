// 文件用途：数据转发规则的 service 层 CRUD 与脱敏装配。
// 核心逻辑：校验来源/目标类型合法性与目标配置完备性，落库经 dal，出参统一走掩码。
// 关键注意事项：mqtt_password 仅创建/更新时入库，任何查询响应一律不回传明文。

package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"github.com/go-basic/uuid"
)

// ForwardRuleService 数据转发规则业务入口。
type ForwardRuleService struct{}

var validForwardSourceTypes = map[string]bool{"telemetry": true, "property": true, "event": true, "status": true}
var validForwardTargetTypes = map[string]bool{"http": true, "mqtt": true}

func validateForwardRule(rule *model.ForwardRule) error {
	if strings.TrimSpace(rule.Name) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "规则名称不能为空")
	}
	if !validForwardSourceTypes[rule.SourceType] {
		return errcode.NewWithMessage(errcode.CodeParamError, "来源类型必须为 telemetry/property/event/status")
	}
	if !validForwardTargetTypes[rule.TargetType] {
		return errcode.NewWithMessage(errcode.CodeParamError, "目标类型必须为 http 或 mqtt")
	}
	switch rule.TargetType {
	case "http":
		if rule.HttpURL == nil || !strings.HasPrefix(strings.TrimSpace(*rule.HttpURL), "http") {
			return errcode.NewWithMessage(errcode.CodeParamError, "HTTP 目标必须提供合法 URL")
		}
		if rule.HttpMethod == nil || *rule.HttpMethod == "" {
			m := "POST"
			rule.HttpMethod = &m
		}
	case "mqtt":
		if rule.MqttBroker == nil || strings.TrimSpace(*rule.MqttBroker) == "" ||
			rule.MqttTopic == nil || strings.TrimSpace(*rule.MqttTopic) == "" {
			return errcode.NewWithMessage(errcode.CodeParamError, "MQTT 目标必须提供 broker 地址与 topic")
		}
	}
	return nil
}

// CreateForwardRule 新建规则。
func (*ForwardRuleService) CreateForwardRule(rule *model.ForwardRule, tenantID string) (*model.ForwardRule, error) {
	rule.ID = uuid.New()
	rule.TenantID = tenantID
	rule.CreatedAt = time.Now().UTC()
	rule.UpdatedAt = rule.CreatedAt
	if err := validateForwardRule(rule); err != nil {
		return nil, err
	}
	if err := dal.CreateForwardRule(rule); err != nil {
		return nil, err
	}
	return maskedForwardRule(rule), nil
}

// UpdateForwardRule 更新规则。
func (*ForwardRuleService) UpdateForwardRule(rule *model.ForwardRule, tenantID string) (*model.ForwardRule, error) {
	if strings.TrimSpace(rule.ID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "规则 ID 不能为空")
	}
	rule.UpdatedAt = time.Now().UTC()
	if err := validateForwardRule(rule); err != nil {
		return nil, err
	}
	if err := dal.UpdateForwardRule(rule, tenantID); err != nil {
		return nil, err
	}
	fresh, err := dal.GetForwardRuleByID(rule.ID, tenantID)
	if err != nil {
		return nil, err
	}
	return maskedForwardRule(fresh), nil
}

// DeleteForwardRule 删除规则。
func (*ForwardRuleService) DeleteForwardRule(id, tenantID string) error {
	return dal.DeleteForwardRule(id, tenantID)
}

// ToggleForwardRule 启停。
func (*ForwardRuleService) ToggleForwardRule(id, tenantID string, enabled bool) error {
	return dal.SetForwardRuleEnabled(id, tenantID, enabled)
}

// GetForwardRuleByID 详情。
func (*ForwardRuleService) GetForwardRuleByID(id, tenantID string) (*model.ForwardRule, error) {
	rule, err := dal.GetForwardRuleByID(id, tenantID)
	if err != nil {
		return nil, err
	}
	return maskedForwardRule(rule), nil
}

// GetForwardRuleListByPage 分页。
func (*ForwardRuleService) GetForwardRuleListByPage(req *model.GetForwardRuleListByPageReq, tenantID string) (map[string]interface{}, error) {
	total, rows, err := dal.GetForwardRuleListByPage(req, tenantID)
	if err != nil {
		return nil, err
	}
	masked := make([]*model.ForwardRule, 0, len(rows))
	for _, row := range rows {
		masked = append(masked, maskedForwardRule(row))
	}
	return map[string]interface{}{"total": total, "list": masked}, nil
}

// maskedForwardRule 出参脱敏：密码列以固定掩码替代（存储侧加密挂账 Phase 2）。
func maskedForwardRule(rule *model.ForwardRule) *model.ForwardRule {
	if rule == nil {
		return nil
	}
	out := *rule
	if out.MqttPassword != nil && *out.MqttPassword != "" {
		m := "******"
		out.MqttPassword = &m
	}
	return &out
}
