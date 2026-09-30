// alarm_config.go 告警规则（alarm_config）聚合的服务编排：创建/删除/更新/分页列表，
// 以及更新载荷构建、通知组租户一致性校验后的通知载荷执行与级别归一。
// 核心逻辑：写路径先过 ensureAlarmConfigWriteAccess，更新走 buildAlarmConfigUpdate 差量组装，
// TriggerDuration/SLA 的零值补写对齐结构体 Updates 跳零值的 ORM 行为。
// 关键注意事项：删除只清规则与名称缓存，历史按保留审计契约不删（见 DeleteAlarmHistory）。
package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func buildAlarmConfigUpdate(req *model.UpdateAlarmConfigReq, oldConfig *model.AlarmConfig, claims *utils.UserClaims) (*model.AlarmConfig, error) {
	data := &model.AlarmConfig{
		ID:        req.ID,
		UpdatedAt: time.Now().UTC(),
		TenantID:  oldConfig.TenantID,
		Remark:    req.Remark,
	}

	if req.Name != nil {
		data.Name = *req.Name
	}
	if req.Description != nil {
		data.Description = req.Description
	}
	if req.AlarmLevel != nil {
		normalizedLevel, err := normalizeAlarmConfigLevel(*req.AlarmLevel)
		if err != nil {
			return nil, err
		}
		data.AlarmLevel = normalizedLevel
	}
	if req.NotificationGroupID != nil {
		data.NotificationGroupID = *req.NotificationGroupID
		if err := validateAlarmNotificationGroupTenant(data.NotificationGroupID, oldConfig.TenantID, claims); err != nil {
			return nil, err
		}
	}
	if req.Enabled != nil {
		data.Enabled = *req.Enabled
	}
	if req.TriggerDuration != nil {
		if err := validateAlarmTriggerDuration(req.TriggerDuration); err != nil {
			return nil, err
		}
		data.TriggerDuration = normalizeAlarmTriggerDuration(req.TriggerDuration)
	} else {
		data.TriggerDuration = oldConfig.TriggerDuration
	}
	// TB-27：SLA 时限更新。req.SlaHours 为 nil（缺省/JSON null）= 沿用旧值；
	// 0 = 关闭 SLA（normalize 折叠为 nil，落库 NULL），由下方单列更新兜底写入。
	if req.SlaHours != nil {
		if err := validateAlarmSlaHours(req.SlaHours); err != nil {
			return nil, err
		}
		data.SlaHours = normalizeAlarmSlaHours(req.SlaHours)
	} else {
		data.SlaHours = oldConfig.SlaHours
	}

	return data, nil
}

func executeAlarmNotificationPayload(alarmConfig *model.AlarmConfig, alertData map[string]interface{}) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(alertData); err != nil {
		logrus.Error("encode alarm notification payload failed: ", err)
		return
	}

	alertJSON := strings.TrimSpace(buffer.String())
	GroupApp.NotificationServicesConfig.ExecuteNotification(alarmConfig.NotificationGroupID, alertJSON, alarmConfig.TenantID)
}

func (*Alarm) CreateAlarmConfig(req *model.CreateAlarmConfigReq, claims *utils.UserClaims) (data *model.AlarmConfig, err error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to create alarm config")
	}
	data = &model.AlarmConfig{}
	t := time.Now().UTC()
	data.ID = uuid.New()
	data.Name = req.Name
	data.Description = req.Description
	normalizedLevel, err := normalizeAlarmConfigLevel(req.AlarmLevel)
	if err != nil {
		return nil, err
	}
	data.AlarmLevel = normalizedLevel
	data.NotificationGroupID = req.NotificationGroupID
	if err := validateAlarmNotificationGroupTenant(data.NotificationGroupID, claims.TenantID, claims); err != nil {
		return nil, err
	}
	data.CreatedAt = t
	data.UpdatedAt = t
	data.TenantID = claims.TenantID
	data.Remark = req.Remark
	data.Enabled = req.Enabled
	if err := validateAlarmTriggerDuration(req.TriggerDuration); err != nil {
		return nil, err
	}
	data.TriggerDuration = normalizeAlarmTriggerDuration(req.TriggerDuration)
	// TB-27：SLA 时限校验与归一（nil/0 → NULL=不启用）。
	if err := validateAlarmSlaHours(req.SlaHours); err != nil {
		return nil, err
	}
	data.SlaHours = normalizeAlarmSlaHours(req.SlaHours)

	err = dal.CreateAlarmConfig(data)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	return
}

// DeleteAlarmConfig deletes one alarm configuration and clears related caches.
// Existing alarm history is retained when the rule is removed.
func (*Alarm) DeleteAlarmConfig(id string, claims *utils.UserClaims) (err error) {
	if _, err = ensureAlarmConfigWriteAccess(id, claims); err != nil {
		return err
	}
	err = dal.DeleteAlarmConfig(id)
	if err != nil {
		return dbError(err)
	}
	_ = dal.DeleteAlarmNameCache(id)
	go func() {
		if err := initialize.NewAlarmCache().DeleteByAlarmId(id); err != nil {
			logrus.Error("DeleteAlarmConfig failed to clear alarm cache: ", err)
		}
	}()
	return
}

// UpdateAlarmConfig updates alarm rule fields after tenant and notification-group checks.
func (*Alarm) UpdateAlarmConfig(req *model.UpdateAlarmConfigReq, claims *utils.UserClaims) (data *model.AlarmConfig, err error) {
	oldConfig, err := ensureAlarmConfigWriteAccess(req.ID, claims)
	if err != nil {
		return nil, err
	}
	data, err = buildAlarmConfigUpdate(req, oldConfig, claims)
	if err != nil {
		return nil, err
	}
	err = dal.UpdateAlarmConfig(data)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	// 结构体 Updates 会跳过零值，把持续时长显式改回 0（立即触发）时需要补一次单列更新。
	if req.TriggerDuration != nil && data.TriggerDuration == 0 {
		if err := dal.UpdateAlarmConfigTriggerDuration(req.ID, 0); err != nil {
			return nil, wrapAlarmDBError(err)
		}
	}
	// 结构体 Updates 会跳过 nil 指针：把 SLA 关闭（req 提交 0 → 归一为 NULL）时
	// 需要补一次单列更新，否则旧时限残留（TB-27，对齐 TriggerDuration 先例）。
	if req.SlaHours != nil && data.SlaHours == nil {
		if err := dal.UpdateAlarmConfigSlaHours(req.ID, nil); err != nil {
			return nil, wrapAlarmDBError(err)
		}
	}
	go func() {
		if err := dal.DeleteAlarmNameCache(req.ID); err != nil {
			logrus.Error("UpdateAlarmConfig failed to clear alarm cache: ", err)
		}
	}()
	data, err = dal.GetAlarmByID(req.ID)
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	return data, nil
}

// GetAlarmConfigListByPage returns paged alarm rule configuration for the caller tenant scope.
func (*Alarm) GetAlarmConfigListByPage(req *model.GetAlarmConfigListByPageReq, claims *utils.UserClaims) (data map[string]interface{}, err error) {
	tenantID, err := normalizeAlarmListTenantID(req.TenantID, claims)
	if err != nil {
		return nil, err
	}
	req.TenantID = tenantID
	// ROADMAP A1：SYS_ADMIN 未指定租户时显式授权全租户视角，其余空租户在 DAL fail-closed。
	allTenants := authz.IsSysAdmin(claims) && strings.TrimSpace(tenantID) == ""
	total, list, err := dal.GetAlarmConfigListByPageForScopes(req, allTenants, alarmListScopes(allTenants, tenantID))
	if err != nil {
		return nil, wrapAlarmDBError(err)
	}
	data = make(map[string]interface{})
	data["total"] = total
	data["list"] = list
	return
}

// normalizeAlarmConfigLevel 校验并归一告警规则的级别。
// alarm_config.alarm_level 是 varchar(10) 且此前只有 required 校验，任意字符串都能落库；
// 前端 alarmSeverityLabel 在匹配不到选项时会原样回显该值，等于把脏数据带到界面上。
// 这里与 alarm_history 的 H/M/L 口径保持一致（配置侧不接受表示"恢复正常"的 N）。
func normalizeAlarmConfigLevel(alarmLevel string) (string, error) {
	normalized := strings.TrimSpace(alarmLevel)
	switch normalized {
	case "H", "M", "L":
		return normalized, nil
	default:
		return "", errcode.NewWithMessage(errcode.CodeParamError, "unsupported alarm_level")
	}
}
