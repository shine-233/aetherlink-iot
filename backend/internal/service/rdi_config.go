// rdi_config.go validates and materializes RDI device configuration.
//
// Purpose: provide default RDI configuration, merge persisted additional_info
// values into typed config, and validate threshold, interval, delay, duration,
// and alarm-recipient fields. Core logic protects frontend/device assumptions
// about allowed numeric ranges and email formats. Important notes: validation
// drift can break device command behavior and RDI device forms, so changes need
// focused boundary tests. Refactor suggestion: keep range validators pure and
// move any future product-specific defaults into named config profiles.
package service

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

func DefaultRDIConfig() model.RDIConfig {
	return model.RDIConfig{
		DataCollectionInterval:       60,
		AlarmSensor1Enabled:          true,
		AlarmSensor2Enabled:          true,
		Sensor1Upper:                 80,
		Sensor1Lower:                 -10,
		Sensor2Upper:                 80,
		Sensor2Lower:                 -10,
		Sensor1Duration:              30,
		Sensor2Duration:              30,
		Switch1AlarmMode:             "disabled",
		Switch2AlarmMode:             "disabled",
		Switch1AlarmDuration:         30,
		Switch2AlarmDuration:         30,
		DryContactAlarmLevel:         "high",
		DryContactNormalLevel:        "low",
		DryContactAlarmDelay:         0,
		DryContactNormalDelay:        0,
		NotificationEnabled:          false,
		NotificationTemperatureAlarm: true,
		NotificationSwitchAlarm:      true,
		NotificationWarrantyAlarm:    true,
		SensorAlarmEmails:            "",
		SwitchAlarmEmails:            "",
		WarrantyAlarmEmails:          "",
		Sensor1AlarmEmails:           "",
		Sensor2AlarmEmails:           "",
		Switch1AlarmEmails:           "",
		Switch2AlarmEmails:           "",
		FieldSetting:                 map[string]interface{}{},
	}
}

func configFromAdditionalInfo(additional map[string]interface{}) model.RDIConfig {
	cfg := DefaultRDIConfig()
	if val, ok := additional[rdiConfigKey]; ok {
		if bytes, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(bytes, &cfg)
		}
	}
	cfg.DataCollectionInterval = normalizeRDICollectionInterval(cfg.DataCollectionInterval)
	if cfg.FieldSetting == nil {
		cfg.FieldSetting = map[string]interface{}{}
	}
	return cfg
}

func normalizeRDICollectionInterval(value int) int {
	if value < rdiCollectionIntervalMin || value > rdiCollectionIntervalMax {
		return DefaultRDIConfig().DataCollectionInterval
	}
	return value
}

func validateRDIConfig(cfg model.RDIConfig) error {
	if cfg.DataCollectionInterval < rdiCollectionIntervalMin || cfg.DataCollectionInterval > rdiCollectionIntervalMax {
		return errcode.NewWithMessage(errcode.CodeParamError, "data_collection_interval must be between 45 and 60 seconds")
	}
	if err := validateTemperatureRange("sensor_1", cfg.Sensor1Lower, cfg.Sensor1Upper); err != nil {
		return err
	}
	if err := validateTemperatureRange("sensor_2", cfg.Sensor2Lower, cfg.Sensor2Upper); err != nil {
		return err
	}
	if err := validateRDIConfigAlarmDurations(cfg); err != nil {
		return err
	}
	if err := validateRDIConfigSwitchAlarmModes(cfg); err != nil {
		return err
	}
	if err := validateRDIConfigDryContact(cfg); err != nil {
		return err
	}
	return validateRDIConfigEmailLists(cfg)
}

func validateRDIConfigAlarmDurations(cfg model.RDIConfig) error {
	for _, item := range []struct {
		name  string
		value int
	}{
		{name: "sensor_1_duration", value: cfg.Sensor1Duration},
		{name: "sensor_2_duration", value: cfg.Sensor2Duration},
		{name: "switch_1_alarm_duration", value: cfg.Switch1AlarmDuration},
		{name: "switch_2_alarm_duration", value: cfg.Switch2AlarmDuration},
	} {
		if item.value < rdiAlarmDurationMinSeconds || item.value > rdiAlarmDurationMaxSeconds {
			return errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("%s must be between %d and %d seconds", item.name, rdiAlarmDurationMinSeconds, rdiAlarmDurationMaxSeconds))
		}
	}
	return nil
}

func validateRDIConfigSwitchAlarmModes(cfg model.RDIConfig) error {
	if !isAllowedValue(cfg.Switch1AlarmMode, "powered_on", "powered_off", "disabled") {
		return errcode.NewWithMessage(errcode.CodeParamError, "switch_1_alarm_mode must be powered_on, powered_off, or disabled")
	}
	if !isAllowedValue(cfg.Switch2AlarmMode, "powered_on", "powered_off", "disabled") {
		return errcode.NewWithMessage(errcode.CodeParamError, "switch_2_alarm_mode must be powered_on, powered_off, or disabled")
	}
	return nil
}

func validateRDIConfigDryContact(cfg model.RDIConfig) error {
	if !isAllowedValue(cfg.DryContactAlarmLevel, "high", "low") {
		return errcode.NewWithMessage(errcode.CodeParamError, "dry_contact_alarm_level must be high or low")
	}
	if !isAllowedValue(cfg.DryContactNormalLevel, "high", "low") {
		return errcode.NewWithMessage(errcode.CodeParamError, "dry_contact_normal_level must be high or low")
	}
	if cfg.DryContactAlarmDelay < rdiDryContactDelayMin || cfg.DryContactAlarmDelay > rdiDryContactDelayMax || cfg.DryContactNormalDelay < rdiDryContactDelayMin || cfg.DryContactNormalDelay > rdiDryContactDelayMax {
		return errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("dry contact delays must be between %d and %d seconds", rdiDryContactDelayMin, rdiDryContactDelayMax))
	}
	return nil
}

func validateRDIConfigEmailLists(cfg model.RDIConfig) error {
	for _, item := range []struct {
		name  string
		value string
	}{
		{name: "sensor_alarm_emails", value: cfg.SensorAlarmEmails},
		{name: "switch_alarm_emails", value: cfg.SwitchAlarmEmails},
		{name: "warranty_alarm_emails", value: cfg.WarrantyAlarmEmails},
		{name: "sensor_1_alarm_emails", value: cfg.Sensor1AlarmEmails},
		{name: "sensor_2_alarm_emails", value: cfg.Sensor2AlarmEmails},
		{name: "switch_1_alarm_emails", value: cfg.Switch1AlarmEmails},
		{name: "switch_2_alarm_emails", value: cfg.Switch2AlarmEmails},
	} {
		if err := validateEmailList(item.name, item.value); err != nil {
			return err
		}
	}
	return nil
}

func validateTemperatureRange(prefix string, lower float64, upper float64) error {
	if lower < rdiTemperatureLowerBound || lower > rdiTemperatureUpperBound || upper < rdiTemperatureLowerBound || upper > rdiTemperatureUpperBound {
		return errcode.NewWithMessage(errcode.CodeParamError, prefix+" limits must be between -40 and 125 C")
	}
	if lower >= upper {
		return errcode.NewWithMessage(errcode.CodeParamError, prefix+" lower limit must be less than upper limit")
	}
	return nil
}

func validateEmailList(fieldName string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, part := range strings.Split(value, ",") {
		email := strings.TrimSpace(part)
		if email == "" {
			continue
		}
		if _, err := mail.ParseAddress(email); err != nil {
			return errcode.NewWithMessage(errcode.CodeParamError, fieldName+" contains an invalid email address")
		}
	}
	return nil
}
