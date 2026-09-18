package calcfield

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	types "aetherlink-iot/backend/internal/calcfield/types"

	"github.com/casbin/govaluate"
	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

var severityRank = map[string]int{
	"H": 3,
	"M": 2,
	"L": 1,
}

func sortRulesBySeverity(rules []types.AlarmSeverityRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		rI := severityRank[strings.ToUpper(strings.TrimSpace(rules[i].Severity))]
		rJ := severityRank[strings.ToUpper(strings.TrimSpace(rules[j].Severity))]
		return rI > rJ
	})
}

// evaluateAlarmRule 对告警规则计算字段统一求值，涵盖多严重度阶梯、严重度升级、自动清除与拓扑传播。
func evaluateAlarmRule(rule compiledRule, payload map[string]interface{}, ts int64, deviceID, tenantID string) (interface{}, []string, error) {
	cfg := rule.advanced
	if cfg == nil || len(cfg.AlarmRules) == 0 {
		return nil, nil, nil
	}

	alarmName := cfg.AlarmName
	if strings.TrimSpace(alarmName) == "" {
		alarmName = rule.outputKey
	}

	// 1. 按严重度从高到低排序规则
	rules := make([]types.AlarmSeverityRule, len(cfg.AlarmRules))
	copy(rules, cfg.AlarmRules)
	sortRulesBySeverity(rules)

	var (
		matchedRule *types.AlarmSeverityRule
		matchedVars map[string]interface{}
	)

	for i := range rules {
		r := &rules[i]
		expr, err := govaluate.NewEvaluableExpression(r.Expression)
		if err != nil {
			continue
		}
		vars := expr.Vars()
		params := make(map[string]interface{}, len(vars))
		hasAllVars := true
		for _, v := range vars {
			val, exists := payload[v]
			if !exists {
				hasAllVars = false
				break
			}
			params[v] = val
		}
		if !hasAllVars {
			continue
		}
		res, err := expr.Evaluate(params)
		if err != nil {
			continue
		}
		if boolVal, ok := res.(bool); ok && boolVal {
			matchedRule = r
			matchedVars = params
			break
		}
	}

	activeAlarm, _ := dal.GetActiveAlarmHistoryForDeviceAndName(tenantID, deviceID, alarmName)

	if matchedRule != nil {
		matchedSeverity := strings.ToUpper(strings.TrimSpace(matchedRule.Severity))
		detailContent := fmt.Sprintf("condition '%s' satisfied (params: %v)", matchedRule.Expression, matchedVars)

		if activeAlarm == nil {
			// 新增活动告警
			histID := uuid.New()
			newHist := &model.AlarmHistory{
				ID:                histID,
				AlarmConfigID:     rule.id,
				GroupID:           "calculated_field",
				SceneAutomationID: rule.id,
				Name:              alarmName,
				Content:           &detailContent,
				AlarmStatus:       matchedSeverity,
				TenantID:          tenantID,
				CreateAt:          time.Now().UTC(),
				AlarmDeviceList:   fmt.Sprintf(`["%s"]`, deviceID),
			}
			if err := dal.CreateAlarmHistoryRow(newHist); err != nil {
				logrus.WithError(err).Warn("Failed to create alarm history from calcfield")
			} else {
				logrus.Infof("Calcfield alarm created: device=%s, alarm=%s, severity=%s", deviceID, alarmName, matchedSeverity)
			}

			// 拓扑传播：向关联实体/父网关广播
			if cfg.Propagate {
				targets := resolveTargets(cfg, tenantID, deviceID)
				for _, target := range targets {
					if target == deviceID || target == "" {
						continue
					}
					propHistID := uuid.New()
					propContent := fmt.Sprintf("[Propagated from device %s] %s", deviceID, detailContent)
					remark := fmt.Sprintf(`{"propagated_from":"%s","origin_alarm_id":"%s"}`, deviceID, histID)
					propHist := &model.AlarmHistory{
						ID:                propHistID,
						AlarmConfigID:     rule.id,
						GroupID:           "calculated_field",
						SceneAutomationID: rule.id,
						Name:              alarmName,
						Content:           &propContent,
						AlarmStatus:       matchedSeverity,
						TenantID:          tenantID,
						CreateAt:          time.Now().UTC(),
						AlarmDeviceList:   fmt.Sprintf(`["%s"]`, target),
						Remark:            &remark,
					}
					_ = dal.CreateAlarmHistoryRow(propHist)
				}
			}
		} else {
			// 已存在活动告警：检查严重度升级
			currRank := severityRank[strings.ToUpper(strings.TrimSpace(activeAlarm.AlarmStatus))]
			newRank := severityRank[matchedSeverity]
			if newRank > currRank {
				escalatedContent := fmt.Sprintf("%s (escalated from %s to %s)", detailContent, activeAlarm.AlarmStatus, matchedSeverity)
				if err := dal.EscalateAlarmHistorySeverity(activeAlarm.ID, tenantID, matchedSeverity, escalatedContent); err != nil {
					logrus.WithError(err).Warn("Failed to escalate alarm severity")
				} else {
					logrus.Infof("Calcfield alarm escalated: device=%s, alarm=%s, %s -> %s", deviceID, alarmName, activeAlarm.AlarmStatus, matchedSeverity)
				}
			}
		}
		return matchedSeverity, nil, nil
	}

	// 2. 未命中任何告警规则：检查自动清除规则
	if cfg.ClearRule != nil && strings.TrimSpace(cfg.ClearRule.Expression) != "" && activeAlarm != nil {
		clearExpr, err := govaluate.NewEvaluableExpression(cfg.ClearRule.Expression)
		if err == nil {
			vars := clearExpr.Vars()
			params := make(map[string]interface{}, len(vars))
			hasAllVars := true
			for _, v := range vars {
				val, exists := payload[v]
				if !exists {
					hasAllVars = false
					break
				}
				params[v] = val
			}
			if hasAllVars {
				res, err := clearExpr.Evaluate(params)
				if err == nil {
					if boolVal, ok := res.(bool); ok && boolVal {
						note := fmt.Sprintf("auto-cleared by rule: %s", cfg.ClearRule.Expression)
						if err := dal.AutoClearAlarmHistoryRecord(activeAlarm.ID, tenantID, note); err != nil {
							logrus.WithError(err).Warn("Failed to auto clear alarm")
						} else {
							logrus.Infof("Calcfield alarm auto-cleared: device=%s, alarm=%s", deviceID, alarmName)
						}
						return "NORMAL", nil, nil
					}
				}
			}
		}
	}

	return nil, nil, nil
}
