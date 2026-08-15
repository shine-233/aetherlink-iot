package service

// RDI alarm recipient helpers keep notification policy, source/channel aliases,
// and email cleanup out of the device/config workflow entrypoints.

import (
	"fmt"
	"net/mail"
	"strings"

	"aetherlink-iot/backend/internal/model"
)

func rdiAlarmEmailTargets(cfg model.RDIConfig, method string) (string, []string) {
	if !cfg.NotificationEnabled {
		return "", nil
	}

	switch strings.TrimSpace(method) {
	case "temperature_alarm":
		if !cfg.NotificationTemperatureAlarm {
			return "", nil
		}
		return "temperature_alarm", parseRDIEmailRecipients(cfg.SensorAlarmEmails)
	case "switch_alarm":
		if !cfg.NotificationSwitchAlarm {
			return "", nil
		}
		return "switch_alarm", parseRDIEmailRecipients(cfg.SwitchAlarmEmails)
	case "warranty_alarm":
		if !cfg.NotificationWarrantyAlarm {
			return "", nil
		}
		return "warranty_alarm", parseRDIEmailRecipients(cfg.WarrantyAlarmEmails)
	default:
		return "", nil
	}
}

func rdiAlarmEmailTargetsForParams(cfg model.RDIConfig, method string, params map[string]interface{}) (string, []string) {
	eventType, fallbackRecipients := rdiAlarmEmailTargets(cfg, method)
	if eventType == "" {
		return "", nil
	}

	switch eventType {
	case "temperature_alarm":
		if recipients := rdiSensorAlarmRecipients(cfg, params); len(recipients) > 0 {
			return eventType, recipients
		}
	case "switch_alarm":
		if recipients := rdiSwitchAlarmRecipients(cfg, params); len(recipients) > 0 {
			return eventType, recipients
		}
	}

	return eventType, fallbackRecipients
}

func rdiSensorAlarmRecipients(cfg model.RDIConfig, params map[string]interface{}) []string {
	switch rdiAlarmSourceIndex(params, []string{"sensor_id", "sensor", "sensor_no", "source", "source_type", "channel"}) {
	case 1:
		return parseRDIEmailRecipients(cfg.Sensor1AlarmEmails)
	case 2:
		return parseRDIEmailRecipients(cfg.Sensor2AlarmEmails)
	default:
		return nil
	}
}

func rdiSwitchAlarmRecipients(cfg model.RDIConfig, params map[string]interface{}) []string {
	switch rdiAlarmSourceIndex(params, []string{"switch_id", "switch", "switch_no", "source", "source_type", "channel"}) {
	case 1:
		return parseRDIEmailRecipients(cfg.Switch1AlarmEmails)
	case 2:
		return parseRDIEmailRecipients(cfg.Switch2AlarmEmails)
	default:
		return nil
	}
}

func rdiAlarmSourceIndex(params map[string]interface{}, keys []string) int {
	if len(params) == 0 {
		return 0
	}
	for _, key := range keys {
		if raw, ok := params[key]; ok {
			if idx := rdiAlarmSourceIndexFromValue(raw); idx != 0 {
				return idx
			}
		}
	}
	return 0
}

func rdiAlarmSourceIndexFromValue(raw interface{}) int {
	value := strings.ToUpper(strings.TrimSpace(fmt.Sprint(raw)))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "1", "T1", "S1", "SENSOR1", "SENSOR_1", "TEMP1", "TEMP_1", "TEMPERATURE1", "TEMPERATURE_1", "NC1", "NC_1", "NC_INPUT1", "NC_INPUT_1", "NC_INPUT_1_LEVEL", "SW1", "SW_1", "SWITCH1", "SWITCH_1", "SWITCH_1_LEVEL":
		return 1
	case "2", "T2", "S2", "SENSOR2", "SENSOR_2", "TEMP2", "TEMP_2", "TEMPERATURE2", "TEMPERATURE_2", "NC2", "NC_2", "NC_INPUT2", "NC_INPUT_2", "NC_INPUT_2_LEVEL", "SW2", "SW_2", "SWITCH2", "SWITCH_2", "SWITCH_2_LEVEL":
		return 2
	default:
		return 0
	}
}

func parseRDIEmailRecipients(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	recipients := make([]string, 0)
	seen := map[string]struct{}{}
	for _, part := range strings.Split(value, ",") {
		email := strings.TrimSpace(part)
		if email == "" {
			continue
		}
		address, err := mail.ParseAddress(email)
		if err != nil {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(address.Address))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		recipients = append(recipients, normalized)
	}
	return recipients
}
