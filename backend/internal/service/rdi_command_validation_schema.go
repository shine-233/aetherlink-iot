package service

type rdiTemperatureSensorLimitPair struct {
	Lower  string
	Upper  string
	Prefix string
}

var rdiAlarmConfigAllowedParams = map[string]struct{}{
	"data_collection_interval":       {},
	"alarm_sensor_1_enabled":         {},
	"alarm_sensor_2_enabled":         {},
	"sensor_1_upper":                 {},
	"sensor_1_lower":                 {},
	"sensor_2_upper":                 {},
	"sensor_2_lower":                 {},
	"sensor_1_duration":              {},
	"sensor_2_duration":              {},
	"switch_1_alarm_mode":            {},
	"switch_2_alarm_mode":            {},
	"switch_1_alarm_duration":        {},
	"switch_2_alarm_duration":        {},
	"dry_contact_alarm_level":        {},
	"dry_contact_normal_level":       {},
	"dry_contact_alarm_delay":        {},
	"dry_contact_normal_delay":       {},
	"notification_enabled":           {},
	"notification_temperature_alarm": {},
	"notification_switch_alarm":      {},
	"notification_warranty_alarm":    {},
	"sensor_alarm_emails":            {},
	"switch_alarm_emails":            {},
	"warranty_alarm_emails":          {},
	"sensor_1_alarm_emails":          {},
	"sensor_2_alarm_emails":          {},
	"switch_1_alarm_emails":          {},
	"switch_2_alarm_emails":          {},
}

var rdiTemperatureSensorLimitPairs = []rdiTemperatureSensorLimitPair{
	{Lower: "sensor_1_lower", Upper: "sensor_1_upper", Prefix: "sensor_1"},
	{Lower: "sensor_2_lower", Upper: "sensor_2_upper", Prefix: "sensor_2"},
}

var rdiAlarmConfigBooleanFlagKeys = []string{
	"alarm_sensor_1_enabled",
	"alarm_sensor_2_enabled",
	"notification_enabled",
	"notification_temperature_alarm",
	"notification_switch_alarm",
	"notification_warranty_alarm",
}

var rdiSwitchAlarmModeKeys = []string{
	"switch_1_alarm_mode",
	"switch_2_alarm_mode",
}

var rdiAlarmDurationKeys = []string{
	"sensor_1_duration",
	"sensor_2_duration",
	"switch_1_alarm_duration",
	"switch_2_alarm_duration",
}

var rdiDryContactLevelKeys = []string{
	"dry_contact_alarm_level",
	"dry_contact_normal_level",
}

var rdiDryContactDelayKeys = []string{
	"dry_contact_alarm_delay",
	"dry_contact_normal_delay",
}

var rdiAlarmConfigEmailKeys = []string{
	"sensor_alarm_emails",
	"switch_alarm_emails",
	"warranty_alarm_emails",
	"sensor_1_alarm_emails",
	"sensor_2_alarm_emails",
	"switch_1_alarm_emails",
	"switch_2_alarm_emails",
}
