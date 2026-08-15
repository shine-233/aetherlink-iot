package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
)

type rdiAlarmEventPlanOptions struct {
	HistoryID string
	EventTime time.Time
}

type rdiAlarmEventPlan struct {
	AlarmHistory *model.AlarmHistory
	Email        *rdiAlarmEventEmailPlan
}

type rdiAlarmEventEmailPlan struct {
	Subject                      string
	Body                         string
	Recipients                   []string
	NeedsTenantWarningRecipients bool
}

func buildRDIAlarmEventPlan(device *model.Device, eventInfo *model.EventInfo, opts rdiAlarmEventPlanOptions) rdiAlarmEventPlan {
	if device == nil || eventInfo == nil {
		return rdiAlarmEventPlan{}
	}

	plan := rdiAlarmEventPlan{
		AlarmHistory: buildRDIAlarmHistoryDraft(device, eventInfo, opts),
	}

	cfg := configFromAdditionalInfo(parseAdditionalInfo(device.AdditionalInfo))
	eventType, recipients := rdiAlarmEmailTargetsForParams(cfg, eventInfo.Method, eventInfo.Params)
	if eventType == "" {
		return plan
	}

	plan.Email = buildRDIAlarmEventEmailPlan(device, eventInfo, eventType, recipients, opts.EventTime)
	return plan
}

func buildRDIAlarmEventEmailPlan(device *model.Device, eventInfo *model.EventInfo, eventType string, recipients []string, eventTime time.Time) *rdiAlarmEventEmailPlan {
	return &rdiAlarmEventEmailPlan{
		Subject:                      fmt.Sprintf("[RDI Alarm] %s - %s", SafeDeref(device.Name), eventType),
		Body:                         rdiAlarmEventEmailBody(device, eventInfo, eventType, eventTime),
		Recipients:                   recipients,
		NeedsTenantWarningRecipients: len(recipients) == 0,
	}
}

func rdiAlarmEventEmailBody(device *model.Device, eventInfo *model.EventInfo, eventType string, eventTime time.Time) string {
	return fmt.Sprintf(`RDI alarm event
Device ID: %s
Device Name: %s
PID: %s
Alarm Type: %s
Time: %s
Params: %s`,
		device.ID,
		SafeDeref(device.Name),
		device.DeviceNumber,
		eventType,
		eventTime.Format(time.RFC3339),
		rdiAlarmEventParamsJSON(eventInfo.Params))
}

func buildRDIAlarmHistoryDraft(device *model.Device, eventInfo *model.EventInfo, opts rdiAlarmEventPlanOptions) *model.AlarmHistory {
	eventType, status, ok := rdiAlarmHistoryMeta(eventInfo)
	if !ok {
		return nil
	}

	params := rdiAlarmEventParamsJSON(eventInfo.Params)
	deviceIDs, _ := json.Marshal([]string{device.ID})
	name, description, content := rdiAlarmHistoryText(device, eventInfo, eventType, params)
	remark := rdiAlarmHistoryRemark(device, eventInfo)

	return &model.AlarmHistory{
		ID:                opts.HistoryID,
		AlarmConfigID:     rdiDirectAlarmConfigID(eventInfo.Method),
		GroupID:           "rdi-direct",
		SceneAutomationID: "rdi-direct",
		Name:              name,
		Description:       &description,
		Content:           &content,
		AlarmStatus:       status,
		TenantID:          device.TenantID,
		Remark:            &remark,
		CreateAt:          opts.EventTime.UTC(),
		AlarmDeviceList:   string(deviceIDs),
	}
}

func rdiAlarmHistoryText(device *model.Device, eventInfo *model.EventInfo, eventType string, params string) (string, string, string) {
	name := fmt.Sprintf("RDI %s", eventType)
	description := fmt.Sprintf("RDI device %s reported %s", SafeDeref(device.Name), eventInfo.Method)
	content := fmt.Sprintf("device_id=%s pid=%s event=%s params=%s", device.ID, device.DeviceNumber, eventInfo.Method, params)
	return name, description, content
}

// rdiAlarmHistoryRemarkMaxBytes 对齐 alarm_history.remark 的 varchar(255)。
// PostgreSQL 对超长值直接报错而不是截断，而告警历史是在邮件之前写入的，
// 因此一条参数过大的事件会连带让整个告警邮件发不出去。
const rdiAlarmHistoryRemarkMaxBytes = 255

func rdiAlarmHistoryRemark(device *model.Device, eventInfo *model.EventInfo) string {
	remarkBytes, _ := json.Marshal(map[string]interface{}{
		"source":     "rdi_event",
		"event_type": eventInfo.Method,
		"pid":        device.DeviceNumber,
		"params":     eventInfo.Params,
	})
	if len(remarkBytes) <= rdiAlarmHistoryRemarkMaxBytes {
		return string(remarkBytes)
	}

	// 超长时丢掉体积最大的 params，保留 event_type：告警类型筛选依赖
	// remark 上的 `"event_type":"..."` LIKE 匹配，不能因为截断而失配。
	// 不能按字节裁剪字符串，否则会写入非法 JSON。
	trimmedBytes, err := json.Marshal(map[string]interface{}{
		"source":         "rdi_event",
		"event_type":     eventInfo.Method,
		"pid":            device.DeviceNumber,
		"params_omitted": true,
	})
	if err != nil || len(trimmedBytes) > rdiAlarmHistoryRemarkMaxBytes {
		// 极端情况下连精简后的对象都放不下，退回最小可筛选载荷。
		minimalBytes, minimalErr := json.Marshal(map[string]interface{}{
			"source":     "rdi_event",
			"event_type": eventInfo.Method,
		})
		if minimalErr != nil || len(minimalBytes) > rdiAlarmHistoryRemarkMaxBytes {
			return ""
		}
		return string(minimalBytes)
	}
	return string(trimmedBytes)
}

func rdiAlarmEventParamsJSON(params map[string]interface{}) string {
	if params == nil {
		return "{}"
	}
	bytes, err := json.Marshal(params)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func rdiAlarmHistoryMeta(eventInfo *model.EventInfo) (string, string, bool) {
	if eventInfo == nil {
		return "", "", false
	}
	switch strings.TrimSpace(eventInfo.Method) {
	case "temperature_alarm":
		return "Temperature Alarm", rdiAlarmStatusFromParams(eventInfo.Params, "H"), true
	case "switch_alarm":
		return "Switch Alarm", rdiAlarmStatusFromParams(eventInfo.Params, "M"), true
	case "warranty_alarm":
		return "Warranty Alarm", rdiAlarmStatusFromParams(eventInfo.Params, "L"), true
	case "sw3_short_press":
		return "SW3 Short Press (Unbind)", "N", true
	case "sw3_long_press":
		return "SW3 Long Press (Factory Reset)", "N", true
	case "sw2_long_press":
		return "SW2 Long Press (WiFi Provisioning)", "N", true
	default:
		return "", "", false
	}
}

func rdiAlarmStatusFromParams(params map[string]interface{}, fallback string) string {
	for _, key := range []string{"alarm_level", "level", "severity", "alarm_status"} {
		if raw, ok := params[key]; ok {
			switch strings.ToUpper(strings.TrimSpace(fmt.Sprint(raw))) {
			case "H", "HIGH", "CRITICAL", "CRIT":
				return "H"
			case "M", "MEDIUM", "MID", "WARNING", "WARN":
				return "M"
			case "L", "LOW", "INFO":
				return "L"
			case "N", "NORMAL", "OK", "RECOVERED", "RECOVERY":
				return "N"
			}
		}
	}
	return fallback
}

func rdiDirectAlarmConfigID(method string) string {
	switch method {
	case "temperature_alarm":
		return "rdi-direct-temperature-alarm"
	case "switch_alarm":
		return "rdi-direct-switch-alarm"
	case "warranty_alarm":
		return "rdi-direct-warranty-alarm"
	case "sw3_short_press":
		return "rdi-direct-sw3-short-press"
	case "sw3_long_press":
		return "rdi-direct-sw3-long-press"
	case "sw2_long_press":
		return "rdi-direct-sw2-long-press"
	default:
		return "rdi-direct-alarm"
	}
}
