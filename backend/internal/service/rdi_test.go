// rdi_test.go locks down RDI helper behavior against requirement fixtures.
//
// Purpose: validate RDI PID normalization, configuration boundaries, thing model definitions, command rules, sharing helpers, and alarm/email mapping.
// Core logic: compares service helpers against explicit requirement tables and boundary inputs without depending on a live RDI device.
// Important notes: these tests are the closest executable contract for the RDI document shape, so requirement changes should update fixtures and negative cases together.
// Refactor suggestion: split command, config, share, and alarm sections into narrower files if RDI coverage keeps growing.
package service

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func rdiTestRequireError(t *testing.T, err error, context string, wantCode int, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s error = nil, want code %d", context, wantCode)
	}
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("%s error type = %T, want *errcode.Error (%v)", context, err, err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("%s code = %d, want %d", context, appErr.Code, wantCode)
	}
	if wantMessage != "" {
		if !appErr.UseCustomMsg || appErr.CustomMsg != wantMessage {
			t.Fatalf("%s message = %q (custom=%t), want %q", context, appErr.CustomMsg, appErr.UseCustomMsg, wantMessage)
		}
	}
}

func TestNormalizeRDIPID(t *testing.T) {
	got, err := NormalizeRDIPID(" 0000000001a2 ")
	if err != nil {
		t.Fatalf("NormalizeRDIPID returned error: %v", err)
	}
	if got != "0000000001A2" {
		t.Fatalf("NormalizeRDIPID = %q, want %q", got, "0000000001A2")
	}

	if _, err := NormalizeRDIPID("too-short"); err == nil {
		t.Fatal("NormalizeRDIPID accepted an invalid PID length")
	}
	if _, err := NormalizeRDIPID("0000000001@2"); err == nil {
		t.Fatal("NormalizeRDIPID accepted non-alphanumeric PID")
	}
}

func TestAllowedRDIHistoryKey(t *testing.T) {
	for _, key := range []string{
		"temperature_1",
		"temperature_2",
		"switch_1",
		"switch_2",
		"dry_contact_output",
		"electricity_consumption",
	} {
		if !allowedRDIHistoryKey(key) {
			t.Fatalf("allowedRDIHistoryKey rejected %q", key)
		}
	}

	for _, key := range []string{"", "pid_number", "firmware_version", "temperature_3"} {
		if allowedRDIHistoryKey(key) {
			t.Fatalf("allowedRDIHistoryKey accepted %q", key)
		}
	}
}

func TestValidateRDIHistoryTimeRange(t *testing.T) {
	baseSeconds := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Unix()
	baseMillis := baseSeconds * 1000
	maxSeconds := int64(rdiHistoryMaxRange / time.Second)
	maxMillis := int64(rdiHistoryMaxRange / time.Millisecond)

	tests := []struct {
		name        string
		startTime   int64
		endTime     int64
		wantMessage string
	}{
		{
			name:      "allows exactly 30 days in seconds",
			startTime: baseSeconds,
			endTime:   baseSeconds + maxSeconds,
		},
		{
			name:      "allows exactly 30 days in milliseconds",
			startTime: baseMillis,
			endTime:   baseMillis + maxMillis,
		},
		{
			name:        "rejects more than 30 days in seconds",
			startTime:   baseSeconds,
			endTime:     baseSeconds + maxSeconds + 1,
			wantMessage: "RDI history time range must not exceed 30 days",
		},
		{
			name:        "rejects more than 30 days in milliseconds",
			startTime:   baseMillis,
			endTime:     baseMillis + maxMillis + 1,
			wantMessage: "RDI history time range must not exceed 30 days",
		},
		{
			name:        "rejects reversed range",
			startTime:   baseSeconds + 1,
			endTime:     baseSeconds,
			wantMessage: "end_time must be greater than or equal to start_time",
		},
		{
			name:        "rejects zero time",
			startTime:   0,
			endTime:     baseSeconds,
			wantMessage: "start_time and end_time must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRDIHistoryTimeRange(tt.startTime, tt.endTime)
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("validateRDIHistoryTimeRange returned error: %v", err)
				}
				return
			}
			rdiTestRequireError(t, err, tt.name, errcode.CodeParamError, tt.wantMessage)
		})
	}
}

func TestValidateRDIConfig(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*model.RDIConfig)
		wantMessage string
	}{
		{
			name: "default config",
		},
		{
			name: "minimum and maximum valid boundaries",
			mutate: func(cfg *model.RDIConfig) {
				cfg.DataCollectionInterval = rdiCollectionIntervalMax
				cfg.Sensor1Lower = rdiTemperatureLowerBound
				cfg.Sensor1Upper = rdiTemperatureUpperBound
				cfg.Sensor1Duration = rdiAlarmDurationMaxSeconds
				cfg.Sensor2Duration = rdiAlarmDurationMaxSeconds
				cfg.Switch1AlarmDuration = rdiAlarmDurationMaxSeconds
				cfg.Switch2AlarmDuration = rdiAlarmDurationMaxSeconds
				cfg.DryContactAlarmDelay = rdiDryContactDelayMax
				cfg.DryContactNormalDelay = rdiDryContactDelayMax
			},
		},
		{
			name: "collection interval below minimum",
			mutate: func(cfg *model.RDIConfig) {
				cfg.DataCollectionInterval = rdiCollectionIntervalMin - 1
			},
			wantMessage: "data_collection_interval must be between 45 and 60 seconds",
		},
		{
			name: "collection interval above maximum",
			mutate: func(cfg *model.RDIConfig) {
				cfg.DataCollectionInterval = rdiCollectionIntervalMax + 1
			},
			wantMessage: "data_collection_interval must be between 45 and 60 seconds",
		},
		{
			name: "temperature below supported range",
			mutate: func(cfg *model.RDIConfig) {
				cfg.Sensor1Lower = rdiTemperatureLowerBound - 1
			},
			wantMessage: "sensor_1 limits must be between -40 and 125 C",
		},
		{
			name: "lower greater than upper",
			mutate: func(cfg *model.RDIConfig) {
				cfg.Sensor1Lower = 90
				cfg.Sensor1Upper = 80
			},
			wantMessage: "sensor_1 lower limit must be less than upper limit",
		},
		{
			name: "negative alarm duration",
			mutate: func(cfg *model.RDIConfig) {
				cfg.Sensor1Duration = -1
			},
			wantMessage: "sensor_1_duration must be between 0 and 86400 seconds",
		},
		{
			name: "alarm duration above maximum",
			mutate: func(cfg *model.RDIConfig) {
				cfg.Switch2AlarmDuration = rdiAlarmDurationMaxSeconds + 1
			},
			wantMessage: "switch_2_alarm_duration must be between 0 and 86400 seconds",
		},
		{
			name: "unsupported switch alarm mode",
			mutate: func(cfg *model.RDIConfig) {
				cfg.Switch1AlarmMode = "on"
			},
			wantMessage: "switch_1_alarm_mode must be powered_on, powered_off, or disabled",
		},
		{
			name: "unsupported dry contact level",
			mutate: func(cfg *model.RDIConfig) {
				cfg.DryContactAlarmLevel = "closed"
			},
			wantMessage: "dry_contact_alarm_level must be high or low",
		},
		{
			name: "dry contact delay below minimum",
			mutate: func(cfg *model.RDIConfig) {
				cfg.DryContactAlarmDelay = rdiDryContactDelayMin - 1
			},
			wantMessage: "dry contact delays must be between 0 and 86400 seconds",
		},
		{
			name: "dry contact delay above maximum",
			mutate: func(cfg *model.RDIConfig) {
				cfg.DryContactNormalDelay = rdiDryContactDelayMax + 1
			},
			wantMessage: "dry contact delays must be between 0 and 86400 seconds",
		},
		{
			name: "invalid alarm email",
			mutate: func(cfg *model.RDIConfig) {
				cfg.SensorAlarmEmails = "ops@example.com,not-an-email"
			},
			wantMessage: "sensor_alarm_emails contains an invalid email address",
		},
		{
			name: "invalid sensor 2 alarm email keeps channel field name",
			mutate: func(cfg *model.RDIConfig) {
				cfg.Sensor2AlarmEmails = "sensor2@example.com,not-an-email"
			},
			wantMessage: "sensor_2_alarm_emails contains an invalid email address",
		},
		{
			name: "invalid switch 1 alarm email keeps channel field name",
			mutate: func(cfg *model.RDIConfig) {
				cfg.Switch1AlarmEmails = "switch1@example.com,not-an-email"
			},
			wantMessage: "switch_1_alarm_emails contains an invalid email address",
		},
		{
			name: "invalid switch 2 alarm email keeps channel field name",
			mutate: func(cfg *model.RDIConfig) {
				cfg.Switch2AlarmEmails = "switch2@example.com,not-an-email"
			},
			wantMessage: "switch_2_alarm_emails contains an invalid email address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultRDIConfig()
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}
			err := validateRDIConfig(cfg)
			if tt.wantMessage != "" {
				rdiTestRequireError(t, err, "validateRDIConfig "+tt.name, errcode.CodeParamError, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("validateRDIConfig returned error: %v", err)
			}
		})
	}
}

func TestRDIThingModelDefinitionMatchesRequirementTable(t *testing.T) {
	thingModel := RDIThingModelDefinition()

	requireItems := func(kind string, items []model.RDIThingModelItem, expected []string) map[string]model.RDIThingModelItem {
		t.Helper()
		got := make(map[string]model.RDIThingModelItem, len(items))
		for _, item := range items {
			got[item.Identifier] = item
		}
		for _, identifier := range expected {
			if _, ok := got[identifier]; !ok {
				t.Fatalf("%s thing model missing %q", kind, identifier)
			}
		}
		return got
	}

	properties := requireItems("property", thingModel.Properties, []string{
		"pid_number",
		"firmware_version",
		"wifi_rssi",
		"ethernet_connected",
		"connection_type",
		"data_collection_interval",
		"alarm_sensor_1_enabled",
		"alarm_sensor_2_enabled",
		"sensor_1_upper",
		"sensor_1_lower",
		"sensor_2_upper",
		"sensor_2_lower",
		"sensor_1_duration",
		"sensor_2_duration",
		"switch_1_alarm_mode",
		"switch_2_alarm_mode",
		"switch_1_alarm_duration",
		"switch_2_alarm_duration",
		"dry_contact_alarm_level",
		"dry_contact_normal_level",
		"dry_contact_alarm_delay",
		"dry_contact_normal_delay",
		"notification_enabled",
		"notification_temperature_alarm",
		"notification_switch_alarm",
		"notification_warranty_alarm",
		"sensor_alarm_emails",
		"switch_alarm_emails",
		"warranty_alarm_emails",
	})
	requireItems("telemetry", thingModel.Telemetry, []string{
		"temperature_1",
		"temperature_2",
		"switch_1",
		"switch_2",
		"dry_contact_output",
		"electricity_consumption",
	})
	requireItems("event", thingModel.Events, []string{
		"temperature_alarm",
		"switch_alarm",
		"warranty_alarm",
		"switch_change",
		"device_online",
		"device_offline",
		"sw3_short_press",
		"sw3_long_press",
		"ota_progress",
	})

	services := make(map[string]model.RDIServiceModelItem, len(thingModel.Services))
	for _, service := range thingModel.Services {
		services[service.Identifier] = service
	}
	for _, identifier := range []string{
		"set_dry_contact",
		"set_alarm_config",
		"set_field_setting",
		"test_dry_contact",
		"ota_upgrade",
		"unbind_device",
		"factory_reset",
	} {
		if _, ok := services[identifier]; !ok {
			t.Fatalf("service thing model missing %q", identifier)
		}
	}
	alarmConfigInputs := make(map[string]struct{}, len(services["set_alarm_config"].Inputs))
	for _, input := range services["set_alarm_config"].Inputs {
		alarmConfigInputs[input] = struct{}{}
	}
	for _, input := range []string{
		"data_collection_interval",
		"alarm_sensor_1_enabled",
		"alarm_sensor_2_enabled",
		"sensor_1_upper",
		"sensor_1_lower",
		"sensor_2_upper",
		"sensor_2_lower",
		"sensor_1_duration",
		"sensor_2_duration",
		"switch_1_alarm_mode",
		"switch_2_alarm_mode",
		"switch_1_alarm_duration",
		"switch_2_alarm_duration",
		"dry_contact_alarm_level",
		"dry_contact_normal_level",
		"dry_contact_alarm_delay",
		"dry_contact_normal_delay",
		"notification_enabled",
		"notification_temperature_alarm",
		"notification_switch_alarm",
		"notification_warranty_alarm",
		"sensor_alarm_emails",
		"switch_alarm_emails",
		"warranty_alarm_emails",
	} {
		if _, ok := alarmConfigInputs[input]; !ok {
			t.Fatalf("set_alarm_config inputs missing %q", input)
		}
	}

	for _, identifier := range []string{
		"data_collection_interval",
		"sensor_1_duration",
		"sensor_2_duration",
		"switch_1_alarm_duration",
		"switch_2_alarm_duration",
		"dry_contact_alarm_delay",
		"dry_contact_normal_delay",
	} {
		if identifier == "data_collection_interval" {
			if properties[identifier].Range != "45~60" {
				t.Fatalf("%s Range = %q, want 45~60", identifier, properties[identifier].Range)
			}
			continue
		}
		if properties[identifier].Range != "0~86400" {
			t.Fatalf("%s Range = %q, want 0~86400", identifier, properties[identifier].Range)
		}
	}
}

func TestValidateRDICommand(t *testing.T) {
	tests := []struct {
		name        string
		identifier  string
		params      map[string]interface{}
		wantMessage string
	}{
		{
			name:       "set dry contact",
			identifier: "set_dry_contact",
			params:     map[string]interface{}{"level": "high", "delay_seconds": 10},
		},
		{
			name:        "set dry contact requires delay",
			identifier:  "set_dry_contact",
			params:      map[string]interface{}{"level": "high"},
			wantMessage: "delay_seconds is required",
		},
		{
			name:       "test dry contact duration",
			identifier: "test_dry_contact",
			params:     map[string]interface{}{"level": "low", "duration_seconds": 5},
		},
		{
			name:       "set alarm config accepts full alarm fields",
			identifier: "set_alarm_config",
			params: map[string]interface{}{
				"data_collection_interval":       60,
				"alarm_sensor_1_enabled":         true,
				"alarm_sensor_2_enabled":         false,
				"sensor_1_lower":                 -10,
				"sensor_1_upper":                 80,
				"sensor_2_lower":                 -10,
				"sensor_2_upper":                 80,
				"sensor_1_duration":              30,
				"sensor_2_duration":              30,
				"switch_1_alarm_mode":            "powered_on",
				"switch_2_alarm_mode":            "powered_off",
				"switch_1_alarm_duration":        30,
				"switch_2_alarm_duration":        30,
				"dry_contact_alarm_level":        "high",
				"dry_contact_normal_level":       "low",
				"dry_contact_alarm_delay":        0,
				"dry_contact_normal_delay":       0,
				"notification_enabled":           true,
				"notification_temperature_alarm": true,
				"notification_switch_alarm":      true,
				"notification_warranty_alarm":    false,
				"sensor_alarm_emails":            "sensor@example.com",
				"switch_alarm_emails":            "switch@example.com",
				"warranty_alarm_emails":          "warranty@example.com",
			},
		},
		{
			name:        "set alarm config rejects collection interval below minimum",
			identifier:  "set_alarm_config",
			params:      map[string]interface{}{"data_collection_interval": 44},
			wantMessage: "data_collection_interval must be between 45 and 60",
		},
		{
			name:        "set alarm config rejects collection interval above maximum",
			identifier:  "set_alarm_config",
			params:      map[string]interface{}{"data_collection_interval": 61},
			wantMessage: "data_collection_interval must be between 45 and 60",
		},
		{
			name:        "set alarm config rejects unknown field",
			identifier:  "set_alarm_config",
			params:      map[string]interface{}{"unexpected": true},
			wantMessage: "unsupported set_alarm_config parameter: unexpected",
		},
		{
			name:        "set alarm config rejects invalid boolean",
			identifier:  "set_alarm_config",
			params:      map[string]interface{}{"notification_enabled": "true"},
			wantMessage: "notification_enabled must be a boolean",
		},
		{
			name:        "set alarm config rejects invalid dry contact delay",
			identifier:  "set_alarm_config",
			params:      map[string]interface{}{"dry_contact_alarm_delay": rdiDryContactDelayMax + 1},
			wantMessage: "dry_contact_alarm_delay must be between 0 and 86400",
		},
		{
			name:        "set alarm config rejects invalid email",
			identifier:  "set_alarm_config",
			params:      map[string]interface{}{"sensor_alarm_emails": "bad-email"},
			wantMessage: "sensor_alarm_emails contains an invalid email address",
		},
		{
			name:       "ota upgrade",
			identifier: "ota_upgrade",
			params: map[string]interface{}{
				"firmware_url": "https://example.com/fw.bin",
				"version":      "1.0.3",
				"size":         1024,
				"md5":          "0123456789abcdef0123456789abcdef",
			},
		},
		{
			name:       "ota upgrade rejects unknown field",
			identifier: "ota_upgrade",
			params: map[string]interface{}{
				"firmware_url": "https://example.com/fw.bin",
				"version":      "1.0.3",
				"size":         1024,
				"md5":          "0123456789abcdef0123456789abcdef",
				"unexpected":   true,
			},
			wantMessage: "unsupported ota_upgrade parameter: unexpected",
		},
		{
			name:       "ota upgrade rejects invalid firmware URL",
			identifier: "ota_upgrade",
			params: map[string]interface{}{
				"firmware_url": "ftp://example.com/fw.bin",
				"version":      "1.0.3",
				"size":         1024,
				"md5":          "0123456789abcdef0123456789abcdef",
			},
			wantMessage: "firmware_url must be an http or https URL",
		},
		{
			name:       "ota upgrade rejects invalid md5",
			identifier: "ota_upgrade",
			params: map[string]interface{}{
				"firmware_url": "https://example.com/fw.bin",
				"version":      "1.0.3",
				"size":         1024,
				"md5":          "not-md5",
			},
			wantMessage: "md5 must be a 32-character hexadecimal string",
		},
		{
			name:        "factory reset rejects params",
			identifier:  "factory_reset",
			params:      map[string]interface{}{"unexpected": true},
			wantMessage: "factory_reset does not accept params",
		},
		{
			name:       "field setting accepts n and sw keys",
			identifier: "set_field_setting",
			params: map[string]interface{}{
				"n00": []interface{}{"temperature_1"},
				"sw1": map[string]interface{}{"label": "load 1"},
			},
		},
		{
			name:        "field setting requires known keys",
			identifier:  "set_field_setting",
			params:      map[string]interface{}{"unknown": true},
			wantMessage: "unsupported set_field_setting parameter: unknown",
		},
		{
			name:       "field setting rejects unknown keys mixed with valid keys",
			identifier: "set_field_setting",
			params: map[string]interface{}{
				"n00":     []interface{}{"temperature_1"},
				"unknown": true,
			},
			wantMessage: "unsupported set_field_setting parameter: unknown",
		},
		{
			name:        "field setting rejects n scalar",
			identifier:  "set_field_setting",
			params:      map[string]interface{}{"n00": "temperature_1"},
			wantMessage: "n00 must be an array",
		},
		{
			name:        "field setting rejects sw array",
			identifier:  "set_field_setting",
			params:      map[string]interface{}{"sw1": []interface{}{"bad"}},
			wantMessage: "sw1 must be an object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRDICommand(tt.identifier, tt.params)
			if tt.wantMessage != "" {
				rdiTestRequireError(t, err, "validateRDICommand "+tt.name, errcode.CodeParamError, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("validateRDICommand returned error: %v", err)
			}
		})
	}
}

func TestRDIShareTokenHelpers(t *testing.T) {
	token, err := generateRDIShareToken()
	if err != nil {
		t.Fatalf("generateRDIShareToken returned error: %v", err)
	}
	if token == "" {
		t.Fatal("generateRDIShareToken returned an empty token")
	}
	if len(token) != 43 {
		t.Fatalf("generateRDIShareToken length = %d, want 43 raw-url chars", len(token))
	}
	for _, ch := range token {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			continue
		}
		t.Fatalf("generateRDIShareToken returned non URL-safe raw character %q in %q", ch, token)
	}

	hash := hashRDIShareToken(token)
	if hash == "" || hash == token {
		t.Fatalf("hashRDIShareToken returned an invalid hash: %q", hash)
	}
	if len(hash) != 64 {
		t.Fatalf("hashRDIShareToken length = %d, want 64 hex chars", len(hash))
	}

	now := time.Now().UTC().Unix()
	records := pruneRDIShareTokens([]model.RDIShareTokenRecord{
		{TokenHash: "expired", ExpiresAt: now - 1},
		{TokenHash: "expires-now", ExpiresAt: now},
		{TokenHash: hash, ExpiresAt: now + 60},
		{TokenHash: "", ExpiresAt: now + 60},
	}, now)
	if len(records) != 1 || records[0].TokenHash != hash {
		t.Fatalf("pruneRDIShareTokens = %#v, want only active hash", records)
	}

	if got := normalizeRDIShareExpiresIn(0); got != rdiShareTokenDefaultTTL {
		t.Fatalf("normalizeRDIShareExpiresIn(0) = %d, want default TTL", got)
	}
	if got := normalizeRDIShareExpiresIn(-10); got != rdiShareTokenDefaultTTL {
		t.Fatalf("normalizeRDIShareExpiresIn(-10) = %d, want default TTL", got)
	}
	if got := normalizeRDIShareExpiresIn(rdiShareTokenMaxTTL + 1); got != rdiShareTokenMaxTTL {
		t.Fatalf("normalizeRDIShareExpiresIn(max+1) = %d, want max TTL", got)
	}
	if got := normalizeRDIShareExpiresIn(3600); got != 3600 {
		t.Fatalf("normalizeRDIShareExpiresIn(3600) = %d, want 3600", got)
	}
}

func TestRDIShareRecipientHelpers(t *testing.T) {
	info := map[string]interface{}{
		rdiShareRecipientsKey: []model.RDIShareRecipientRecord{
			{UserID: "user-a", Email: "a@example.com", TenantID: "tenant-a", AcceptedAt: 100},
		},
	}
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal additional info: %v", err)
	}
	additional := string(raw)
	device := &model.Device{ID: "device-a", TenantID: "owner", AdditionalInfo: &additional}
	recipient, ok := rdiShareRecipientForUser(device, &utils.UserClaims{ID: "user-a"})
	if !ok {
		t.Fatal("rdiShareRecipientForUser did not find accepted user")
	}
	if recipient.Email != "a@example.com" || recipient.AcceptedAt != 100 {
		t.Fatalf("unexpected recipient: %#v", recipient)
	}
	if _, ok := rdiShareRecipientForUser(device, &utils.UserClaims{ID: "user-b"}); ok {
		t.Fatal("rdiShareRecipientForUser matched a non-recipient user")
	}
}

func TestRDIShareAdditionalInfoDecoders(t *testing.T) {
	tokens := newRDIShareState(map[string]interface{}{
		rdiShareTokensKey: []map[string]interface{}{
			{
				"token_hash": "hash-a",
				"expires_at": float64(123),
			},
		},
	}).Tokens()
	if len(tokens) != 1 || tokens[0].TokenHash != "hash-a" || tokens[0].ExpiresAt != 123 {
		t.Fatalf("newRDIShareState(...).Tokens() decoded %#v", tokens)
	}

	recipients := newRDIShareState(map[string]interface{}{
		rdiShareRecipientsKey: []map[string]interface{}{
			{
				"user_id":     "user-a",
				"email":       "a@example.com",
				"tenant_id":   "tenant-a",
				"token_hash":  "hash-a",
				"accepted_at": float64(456),
			},
		},
	}).Recipients()
	if len(recipients) != 1 || recipients[0].UserID != "user-a" || recipients[0].AcceptedAt != 456 {
		t.Fatalf("newRDIShareState(...).Recipients() decoded %#v", recipients)
	}

	if got := newRDIShareState(map[string]interface{}{rdiShareTokensKey: make(chan int)}).Tokens(); got != nil {
		t.Fatalf("newRDIShareState(...).Tokens() should return nil for invalid payload, got %#v", got)
	}
	if got := newRDIShareState(map[string]interface{}{rdiShareRecipientsKey: make(chan int)}).Recipients(); got != nil {
		t.Fatalf("newRDIShareState(...).Recipients() should return nil for invalid payload, got %#v", got)
	}
}

func TestRDIDeviceConfigResponseHidesShareMetadata(t *testing.T) {
	additional := `{"connection_type":"wifi","public_note":"visible","rdi_share_tokens":[{"token_hash":"secret-token-hash"}],"rdi_share_recipients":[{"user_id":"user-a","token_hash":"secret-token-hash"}]}`
	device := &model.Device{
		ID:             "device-a",
		DeviceNumber:   "0000000001A2",
		AdditionalInfo: &additional,
	}

	resp := rdiDeviceConfigResponse(device, rdiDeviceConfigResponseOptions{
		IncludeAdditionalInfo: true,
		ExposeAlarmEmails:     true,
	})
	if resp.AdditionalInfo["public_note"] != "visible" {
		t.Fatalf("public additional info was not preserved: %#v", resp.AdditionalInfo)
	}
	if _, ok := resp.AdditionalInfo[rdiShareTokensKey]; ok {
		t.Fatalf("share token metadata leaked in device config response: %#v", resp.AdditionalInfo)
	}
	if _, ok := resp.AdditionalInfo[rdiShareRecipientsKey]; ok {
		t.Fatalf("share recipient metadata leaked in device config response: %#v", resp.AdditionalInfo)
	}
}

func TestRDIAlarmEmailResponseVisibilityByClaims(t *testing.T) {
	ownerID := "owner-user"
	// 设备必须带 tenant_id：告警邮箱披露对空租户设备 fail closed，
	// 只有 SYS_ADMIN 能绕过租户比对。
	device := &model.Device{TenantID: "tenant-a", OwnerUserID: &ownerID}
	tests := []struct {
		name   string
		claims *utils.UserClaims
		want   bool
	}{
		{
			name:   "device owner",
			claims: &utils.UserClaims{ID: ownerID, TenantID: "tenant-a", Authority: constant.TENANT_USER},
			want:   true,
		},
		{
			name:   "tenant admin",
			claims: &utils.UserClaims{ID: "tenant-admin", TenantID: "tenant-a", Authority: constant.TENANT_ADMIN},
			want:   true,
		},
		{
			name:   "system admin",
			claims: &utils.UserClaims{ID: "system-admin", TenantID: "system", Authority: constant.SYS_ADMIN},
			want:   true,
		},
		{
			name:   "ordinary share recipient",
			claims: &utils.UserClaims{ID: "share-recipient", TenantID: "tenant-a", Authority: constant.TENANT_USER},
			want:   false,
		},
		{
			name:   "cross tenant admin",
			claims: &utils.UserClaims{ID: "tenant-admin", TenantID: "tenant-b", Authority: constant.TENANT_ADMIN},
			want:   false,
		},
		{
			name:   "empty tenant device fails closed",
			claims: &utils.UserClaims{ID: ownerID, TenantID: "", Authority: constant.TENANT_USER},
			want:   false,
		},
		{name: "missing claims", claims: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rdiMayExposeAlarmEmails(device, tt.claims); got != tt.want {
				t.Fatalf("rdiMayExposeAlarmEmails() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRDIConfigWithoutAlarmEmailsPreservesEveryOtherField(t *testing.T) {
	full := rdiTestConfigWithAlarmEmails()
	redacted := rdiConfigWithoutAlarmEmails(full)
	rdiTestAssertAlarmEmails(t, redacted, "")
	rdiTestAssertAlarmEmails(t, full, "alarm@example.com")

	restored := redacted
	restored.SensorAlarmEmails = full.SensorAlarmEmails
	restored.SwitchAlarmEmails = full.SwitchAlarmEmails
	restored.WarrantyAlarmEmails = full.WarrantyAlarmEmails
	restored.Sensor1AlarmEmails = full.Sensor1AlarmEmails
	restored.Sensor2AlarmEmails = full.Sensor2AlarmEmails
	restored.Switch1AlarmEmails = full.Switch1AlarmEmails
	restored.Switch2AlarmEmails = full.Switch2AlarmEmails
	if !reflect.DeepEqual(restored, full) {
		t.Fatalf("redaction changed non-email RDI config: got %#v want %#v", restored, full)
	}
}

func TestRDIDeviceConfigResponseRedactsConfigAndAdditionalInfoForShareRead(t *testing.T) {
	storedConfig := map[string]interface{}{
		"data_collection_interval": 45,
		"notification_enabled":     true,
		"sensor_alarm_emails":      "alarm@example.com",
		"switch_alarm_emails":      "alarm@example.com",
		"warranty_alarm_emails":    "alarm@example.com",
		"sensor_1_alarm_emails":    "alarm@example.com",
		"sensor_2_alarm_emails":    "alarm@example.com",
		"switch_1_alarm_emails":    "alarm@example.com",
		"switch_2_alarm_emails":    "alarm@example.com",
		"future_config_flag":       "keep-me",
		"field_setting":            map[string]interface{}{"n00": "visible"},
	}
	additionalBytes, err := json.Marshal(map[string]interface{}{
		rdiConfigKey:          storedConfig,
		"connection_type":     "wifi",
		"public_note":         "keep-me",
		"sensor_alarm_emails": "legacy-alarm@example.com",
	})
	if err != nil {
		t.Fatalf("marshal RDI response fixture: %v", err)
	}
	additional := string(additionalBytes)
	device := &model.Device{ID: "device-a", DeviceNumber: "0000000001A2", AdditionalInfo: &additional}

	sharedResponse := rdiDeviceConfigResponse(device, rdiDeviceConfigResponseOptions{IncludeAdditionalInfo: true})
	rdiTestAssertAlarmEmails(t, sharedResponse.Config, "")
	if sharedResponse.Config.DataCollectionInterval != 45 || !sharedResponse.Config.NotificationEnabled {
		t.Fatalf("shared response changed non-email config: %#v", sharedResponse.Config)
	}
	if sharedResponse.Config.FieldSetting["n00"] != "visible" {
		t.Fatalf("shared response changed field_setting: %#v", sharedResponse.Config.FieldSetting)
	}
	if sharedResponse.AdditionalInfo["public_note"] != "keep-me" {
		t.Fatalf("shared response changed public additional info: %#v", sharedResponse.AdditionalInfo)
	}
	if sharedResponse.AdditionalInfo["sensor_alarm_emails"] != "" {
		t.Fatalf("shared response leaked legacy top-level alarm email: %#v", sharedResponse.AdditionalInfo)
	}
	sharedStoredConfig, ok := sharedResponse.AdditionalInfo[rdiConfigKey].(map[string]interface{})
	if !ok {
		t.Fatalf("shared response rdi_config type = %T, want map", sharedResponse.AdditionalInfo[rdiConfigKey])
	}
	for _, key := range rdiAlarmEmailConfigKeys {
		if sharedStoredConfig[key] != "" {
			t.Fatalf("shared response additional_info.%s = %#v, want empty", key, sharedStoredConfig[key])
		}
	}
	if sharedStoredConfig["future_config_flag"] != "keep-me" {
		t.Fatalf("shared response dropped future config field: %#v", sharedStoredConfig)
	}

	privilegedResponse := rdiDeviceConfigResponse(device, rdiDeviceConfigResponseOptions{
		IncludeAdditionalInfo: true,
		ExposeAlarmEmails:     true,
	})
	rdiTestAssertAlarmEmails(t, privilegedResponse.Config, "alarm@example.com")
	privilegedStoredConfig, ok := privilegedResponse.AdditionalInfo[rdiConfigKey].(map[string]interface{})
	if !ok || privilegedStoredConfig["sensor_alarm_emails"] != "alarm@example.com" {
		t.Fatalf("privileged response lost stored alarm emails: %#v", privilegedResponse.AdditionalInfo[rdiConfigKey])
	}
	if privilegedResponse.AdditionalInfo["sensor_alarm_emails"] != "legacy-alarm@example.com" {
		t.Fatalf("privileged response lost legacy top-level alarm email: %#v", privilegedResponse.AdditionalInfo)
	}
}

func TestRDIAlarmEmailAdditionalInfoRedactionDropsMalformedOpaqueConfig(t *testing.T) {
	additional := map[string]interface{}{
		rdiConfigKey:          `{"sensor_alarm_emails":"opaque-secret@example.com"}`,
		"public_note":         "keep-me",
		"switch_alarm_emails": "legacy-secret@example.com",
	}
	redactRDIAlarmEmailsFromAdditionalInfo(additional)
	if _, exists := additional[rdiConfigKey]; exists {
		t.Fatalf("malformed opaque rdi_config was returned to share reader: %#v", additional)
	}
	if additional["switch_alarm_emails"] != "" || additional["public_note"] != "keep-me" {
		t.Fatalf("malformed config redaction changed the wrong fields: %#v", additional)
	}
}

func rdiTestConfigWithAlarmEmails() model.RDIConfig {
	cfg := DefaultRDIConfig()
	cfg.DataCollectionInterval = 45
	cfg.NotificationEnabled = true
	cfg.SensorAlarmEmails = "alarm@example.com"
	cfg.SwitchAlarmEmails = "alarm@example.com"
	cfg.WarrantyAlarmEmails = "alarm@example.com"
	cfg.Sensor1AlarmEmails = "alarm@example.com"
	cfg.Sensor2AlarmEmails = "alarm@example.com"
	cfg.Switch1AlarmEmails = "alarm@example.com"
	cfg.Switch2AlarmEmails = "alarm@example.com"
	cfg.FieldSetting = map[string]interface{}{"future": "keep-me"}
	return cfg
}

func rdiTestAssertAlarmEmails(t *testing.T, cfg model.RDIConfig, want string) {
	t.Helper()
	values := map[string]string{
		"sensor_alarm_emails":   cfg.SensorAlarmEmails,
		"switch_alarm_emails":   cfg.SwitchAlarmEmails,
		"warranty_alarm_emails": cfg.WarrantyAlarmEmails,
		"sensor_1_alarm_emails": cfg.Sensor1AlarmEmails,
		"sensor_2_alarm_emails": cfg.Sensor2AlarmEmails,
		"switch_1_alarm_emails": cfg.Switch1AlarmEmails,
		"switch_2_alarm_emails": cfg.Switch2AlarmEmails,
	}
	for key, got := range values {
		if got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestRDIDeviceSharedStatus(t *testing.T) {
	tests := []struct {
		name           string
		additionalInfo *string
		want           string
	}{
		{name: "nil additional info", want: "unshared"},
		{name: "empty additional info", additionalInfo: StringPtr("   "), want: "unshared"},
		{name: "invalid json", additionalInfo: StringPtr("{"), want: "unshared"},
		{name: "missing recipients", additionalInfo: StringPtr(`{"rdi_share_tokens":[]}`), want: "unshared"},
		{name: "empty recipients", additionalInfo: StringPtr(`{"rdi_share_recipients":[]}`), want: "unshared"},
		{name: "empty recipient object", additionalInfo: StringPtr(`{"rdi_share_recipients":[{}]}`), want: "unshared"},
		{
			name:           "valid recipient",
			additionalInfo: StringPtr(`{"rdi_share_recipients":[{"user_id":"user-a","email":"a@example.com","tenant_id":"tenant-a","token_hash":"hash","accepted_at":100}]}`),
			want:           "shared",
		},
		{
			name:           "valid recipient with whitespace",
			additionalInfo: StringPtr(`  {"rdi_share_recipients":[{"user_id":"user-b"}]}  `),
			want:           "shared",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rdiDeviceSharedStatus(tt.additionalInfo); got != tt.want {
				t.Fatalf("rdiDeviceSharedStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRDIAlarmEmailTargets(t *testing.T) {
	cfg := DefaultRDIConfig()
	cfg.NotificationEnabled = true
	cfg.SensorAlarmEmails = "Sensor@Example.com, sensor@example.com, other@example.com"
	cfg.SwitchAlarmEmails = "switch@example.com"

	eventType, recipients := rdiAlarmEmailTargets(cfg, "temperature_alarm")
	if eventType != "temperature_alarm" {
		t.Fatalf("eventType = %q, want temperature_alarm", eventType)
	}
	if len(recipients) != 2 || recipients[0] != "sensor@example.com" || recipients[1] != "other@example.com" {
		t.Fatalf("temperature recipients = %#v", recipients)
	}

	eventType, recipients = rdiAlarmEmailTargets(cfg, "switch_alarm")
	if eventType != "switch_alarm" {
		t.Fatalf("eventType = %q, want switch_alarm", eventType)
	}
	if len(recipients) != 1 || recipients[0] != "switch@example.com" {
		t.Fatalf("switch recipients = %#v", recipients)
	}

	cfg.NotificationSwitchAlarm = false
	if _, recipients = rdiAlarmEmailTargets(cfg, "switch_alarm"); len(recipients) != 0 {
		t.Fatalf("disabled switch alarm returned recipients = %#v", recipients)
	}

	cfg.NotificationSwitchAlarm = true
	cfg.NotificationTemperatureAlarm = false
	if _, recipients = rdiAlarmEmailTargets(cfg, "temperature_alarm"); len(recipients) != 0 {
		t.Fatalf("disabled temperature alarm returned recipients = %#v", recipients)
	}

	cfg.NotificationTemperatureAlarm = true
	if _, recipients = rdiAlarmEmailTargets(cfg, "device_online"); len(recipients) != 0 {
		t.Fatalf("unknown RDI event returned recipients = %#v", recipients)
	}

	cfg.NotificationEnabled = false
	if _, recipients = rdiAlarmEmailTargets(cfg, "temperature_alarm"); len(recipients) != 0 {
		t.Fatalf("disabled notification returned recipients = %#v", recipients)
	}
}

func TestRDIConfigFromAdditionalInfoMergesStoredConfigAndKeepsFieldSettingMap(t *testing.T) {
	additional := map[string]interface{}{
		rdiConfigKey: map[string]interface{}{
			"data_collection_interval": 45,
			"sensor_1_upper":           70,
			"field_setting": map[string]interface{}{
				"n00": []interface{}{float64(1), float64(2)},
			},
		},
	}

	cfg := configFromAdditionalInfo(additional)
	if cfg.DataCollectionInterval != 45 {
		t.Fatalf("data collection interval = %d, want 45", cfg.DataCollectionInterval)
	}
	if cfg.Sensor1Upper != 70 {
		t.Fatalf("sensor 1 upper = %v, want 70", cfg.Sensor1Upper)
	}
	if cfg.Sensor1Lower != -10 {
		t.Fatalf("default sensor 1 lower should be preserved, got %v", cfg.Sensor1Lower)
	}
	if cfg.FieldSetting == nil || cfg.FieldSetting["n00"] == nil {
		t.Fatalf("field setting should be decoded and kept non-nil, got %#v", cfg.FieldSetting)
	}
}

func TestRDIConfigFromAdditionalInfoUsesDefaultsForMissingInvalidOrNilFieldSetting(t *testing.T) {
	cfg := configFromAdditionalInfo(map[string]interface{}{})
	if cfg.DataCollectionInterval != 60 || cfg.FieldSetting == nil {
		t.Fatalf("missing config should return defaults with non-nil field setting, got %#v", cfg)
	}

	cfg = configFromAdditionalInfo(map[string]interface{}{
		rdiConfigKey: map[string]interface{}{
			"data_collection_interval": 45,
			"field_setting":            nil,
		},
	})
	if cfg.DataCollectionInterval != 45 {
		t.Fatalf("stored interval = %d, want 45", cfg.DataCollectionInterval)
	}
	if cfg.FieldSetting == nil {
		t.Fatal("nil field_setting in stored config should be normalized to an empty map")
	}
}

func TestRDIConfigFromAdditionalInfoNormalizesStoredCollectionInterval(t *testing.T) {
	cfg := configFromAdditionalInfo(map[string]interface{}{
		rdiConfigKey: map[string]interface{}{
			"data_collection_interval": 120,
		},
	})
	if cfg.DataCollectionInterval != 60 {
		t.Fatalf("out-of-range stored interval = %d, want default 60", cfg.DataCollectionInterval)
	}
}

func TestRDIPhysicalUnbindEventBuildsSoftUnbindUpdate(t *testing.T) {
	rawAdditional := StringPtr(`{"rdi_config":{"data_collection_interval":60},"rdi_share_tokens":[{"token_hash":"secret"}],"rdi_share_recipients":[{"user_id":"u1"}],"public_note":"keep"}`)
	device := &model.Device{ID: "dev-1", TenantID: "tenant-1", AdditionalInfo: rawAdditional}
	now := time.Unix(1700000000, 0).UTC()

	if !isRDIPhysicalUnbindEvent(&model.EventInfo{Method: "sw3_short_press"}) {
		t.Fatal("sw3_short_press should be treated as an RDI physical unbind event")
	}
	if isRDIPhysicalUnbindEvent(&model.EventInfo{Method: "sw3_long_press"}) {
		t.Fatal("sw3_long_press must not trigger the physical unbind path")
	}

	updates := rdiPhysicalUnbindUpdates(device, now)
	if updates["tenant_id"] != "" || updates["activate_flag"] != "inactive" || updates["is_enabled"] != "disabled" || updates["is_online"] != int16(0) {
		t.Fatalf("unexpected physical unbind status updates: %#v", updates)
	}
	if updates["owner_user_id"] != nil {
		t.Fatalf("physical unbind should clear owner_user_id, got %#v", updates["owner_user_id"])
	}
	if updates["update_at"] != now {
		t.Fatalf("update_at = %#v, want %#v", updates["update_at"], now)
	}
	additional := parseAdditionalInfo(StringPtr(updates["additional_info"].(string)))
	if _, ok := additional[rdiShareTokensKey]; ok {
		t.Fatalf("share tokens were not cleared: %#v", additional)
	}
	if _, ok := additional[rdiShareRecipientsKey]; ok {
		t.Fatalf("share recipients were not cleared: %#v", additional)
	}
	if additional["public_note"] != "keep" {
		t.Fatalf("non-share additional info should be preserved, got %#v", additional)
	}
	if _, ok := additional[rdiConfigKey]; !ok {
		t.Fatalf("rdi config should be preserved, got %#v", additional)
	}
}

func TestRDISystemInfoFromAdditionalInfoDecodesKnownAndExtraFields(t *testing.T) {
	additional := map[string]interface{}{
		rdiSystemInfoKey: map[string]interface{}{
			"installation_location":  "greenhouse-1",
			"maintenance_technician": "ops-a",
			"extra_fields": map[string]interface{}{
				"address":                  "Pudong 1",
				"installation_date":        "2026-07-09",
				"installer_company":        "Installer Co",
				"installer_contact":        "Alex",
				"installer_name":           "Alex Name",
				"installer_phone":          "+1 555 0000",
				"installer_email":          "alex@example.com",
				"controller_serial_number": "RDI-SN-001",
				"rack":                     "A1",
			},
		},
	}

	info := systemInfoFromAdditionalInfo(additional)
	if info.InstallationLocation != "greenhouse-1" || info.MaintenanceTechnician != "ops-a" {
		t.Fatalf("unexpected system info: %#v", info)
	}
	if info.Address != "Pudong 1" ||
		info.InstallationDate != "2026-07-09" ||
		info.InstallerCompany != "Installer Co" ||
		info.InstallerContact != "Alex" ||
		info.InstallerName != "Alex Name" ||
		info.InstallerPhone != "+1 555 0000" ||
		info.InstallerEmail != "alex@example.com" ||
		info.ControllerSerialNumber != "RDI-SN-001" {
		t.Fatalf("promoted system info fields were not decoded from extra_fields: %#v", info)
	}
	if info.ExtraFields == nil || info.ExtraFields["rack"] != "A1" {
		t.Fatalf("extra fields should be decoded and kept non-nil, got %#v", info.ExtraFields)
	}

	info = systemInfoFromAdditionalInfo(map[string]interface{}{})
	if info.ExtraFields == nil {
		t.Fatal("missing system info should still return a non-nil extra fields map")
	}
}

func TestNormalizeRDISystemInfoForStoragePromotesAndDeduplicatesExtraFields(t *testing.T) {
	info := normalizeRDISystemInfoForStorage(model.RDISystemInfo{
		ExtraFields: map[string]interface{}{
			"address":                  "Pudong 1",
			"installation_date":        "2026-07-09",
			"installer_company":        "Installer Co",
			"installer_contact":        "Alex",
			"installer_name":           "Alex Name",
			"installer_phone":          "+1 555 0000",
			"installer_email":          "alex@example.com",
			"controller_serial_number": "RDI-SN-001",
			"room":                     "A101",
		},
	})

	if info.Address != "Pudong 1" ||
		info.InstallationDate != "2026-07-09" ||
		info.InstallerCompany != "Installer Co" ||
		info.InstallerContact != "Alex" ||
		info.InstallerName != "Alex Name" ||
		info.InstallerPhone != "+1 555 0000" ||
		info.InstallerEmail != "alex@example.com" ||
		info.ControllerSerialNumber != "RDI-SN-001" {
		t.Fatalf("promoted fields were not copied before storage: %#v", info)
	}

	if info.ExtraFields["room"] != "A101" {
		t.Fatalf("unrelated extra field should be preserved, got %#v", info.ExtraFields)
	}
	for _, key := range promotedRDISystemInfoExtraKeys {
		if _, ok := info.ExtraFields[key]; ok {
			t.Fatalf("promoted key %q should be removed from extra_fields, got %#v", key, info.ExtraFields)
		}
	}
}

func TestRDIAlarmEmailTargetsForParamsPrefersSensorAndSwitchSpecificRecipients(t *testing.T) {
	cfg := DefaultRDIConfig()
	cfg.NotificationEnabled = true
	cfg.SensorAlarmEmails = "fallback-sensor@example.com"
	cfg.SwitchAlarmEmails = "fallback-switch@example.com"
	cfg.Sensor1AlarmEmails = "sensor1@example.com"
	cfg.Sensor2AlarmEmails = "sensor2@example.com"
	cfg.Switch1AlarmEmails = "switch1@example.com"
	cfg.Switch2AlarmEmails = "switch2@example.com"

	eventType, recipients := rdiAlarmEmailTargetsForParams(cfg, "temperature_alarm", map[string]interface{}{"sensor_id": "T2"})
	if eventType != "temperature_alarm" || len(recipients) != 1 || recipients[0] != "sensor2@example.com" {
		t.Fatalf("temperature sensor-specific recipients = %q/%#v", eventType, recipients)
	}

	eventType, recipients = rdiAlarmEmailTargetsForParams(cfg, "switch_alarm", map[string]interface{}{"switch_no": "SW1"})
	if eventType != "switch_alarm" || len(recipients) != 1 || recipients[0] != "switch1@example.com" {
		t.Fatalf("switch-specific recipients = %q/%#v", eventType, recipients)
	}
}

func TestRDIAlarmEmailTargetsForParamsFallsBackWhenSpecificRecipientMissing(t *testing.T) {
	cfg := DefaultRDIConfig()
	cfg.NotificationEnabled = true
	cfg.SensorAlarmEmails = "fallback-sensor@example.com"
	cfg.SwitchAlarmEmails = "fallback-switch@example.com"

	eventType, recipients := rdiAlarmEmailTargetsForParams(cfg, "temperature_alarm", map[string]interface{}{"sensor": "S1"})
	if eventType != "temperature_alarm" || len(recipients) != 1 || recipients[0] != "fallback-sensor@example.com" {
		t.Fatalf("temperature fallback recipients = %q/%#v", eventType, recipients)
	}

	eventType, recipients = rdiAlarmEmailTargetsForParams(cfg, "switch_alarm", map[string]interface{}{"channel": "SW2"})
	if eventType != "switch_alarm" || len(recipients) != 1 || recipients[0] != "fallback-switch@example.com" {
		t.Fatalf("switch fallback recipients = %q/%#v", eventType, recipients)
	}
}

func TestRDIAlarmHistoryMeta(t *testing.T) {
	eventType, status, ok := rdiAlarmHistoryMeta(&model.EventInfo{
		Method: "temperature_alarm",
		Params: map[string]interface{}{"severity": "critical"},
	})
	if !ok || eventType != "Temperature Alarm" || status != "H" {
		t.Fatalf("temperature meta = (%q, %q, %v), want Temperature Alarm/H/true", eventType, status, ok)
	}

	eventType, status, ok = rdiAlarmHistoryMeta(&model.EventInfo{
		Method: "switch_alarm",
		Params: map[string]interface{}{"alarm_level": "low"},
	})
	if !ok || eventType != "Switch Alarm" || status != "L" {
		t.Fatalf("switch meta = (%q, %q, %v), want Switch Alarm/L/true", eventType, status, ok)
	}

	eventType, status, ok = rdiAlarmHistoryMeta(&model.EventInfo{
		Method: "device_online",
		Params: map[string]interface{}{"status": "online"},
	})
	if ok || eventType != "" || status != "" {
		t.Fatalf("info event meta = (%q, %q, %v), want empty/false", eventType, status, ok)
	}
}

func TestRDIAlarmStatusFromParamsCanonicalizesLevels(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		fallback string
		want     string
	}{
		{name: "critical severity", params: map[string]interface{}{"severity": "crit"}, fallback: "L", want: "H"},
		{name: "warning level", params: map[string]interface{}{"level": " warning "}, fallback: "L", want: "M"},
		{name: "low info", params: map[string]interface{}{"alarm_level": "INFO"}, fallback: "H", want: "L"},
		{name: "recovered status", params: map[string]interface{}{"alarm_status": "recovered"}, fallback: "H", want: "N"},
		{name: "unknown keeps fallback", params: map[string]interface{}{"severity": "unknown"}, fallback: "M", want: "M"},
		{name: "missing keeps fallback", params: nil, fallback: "L", want: "L"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rdiAlarmStatusFromParams(tt.params, tt.fallback); got != tt.want {
				t.Fatalf("rdiAlarmStatusFromParams() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRDIDirectAlarmConfigIDMapsKnownRDIEvents(t *testing.T) {
	tests := map[string]string{
		"temperature_alarm": "rdi-direct-temperature-alarm",
		"switch_alarm":      "rdi-direct-switch-alarm",
		"warranty_alarm":    "rdi-direct-warranty-alarm",
		"sw3_short_press":   "rdi-direct-sw3-short-press",
		"sw3_long_press":    "rdi-direct-sw3-long-press",
		"sw2_long_press":    "rdi-direct-sw2-long-press",
		"unknown":           "rdi-direct-alarm",
	}

	for method, want := range tests {
		if got := rdiDirectAlarmConfigID(method); got != want {
			t.Fatalf("rdiDirectAlarmConfigID(%q) = %q, want %q", method, got, want)
		}
	}
}

func TestRDIAlarmSourceIndexRecognizesSensorAndSwitchAliases(t *testing.T) {
	aliases := map[interface{}]int{
		"T1":                 1,
		"temperature-2":      2,
		"NC input 1 level":   1,
		"switch_2_level":     2,
		float64(1):           1,
		"unrecognized-input": 0,
	}

	for raw, want := range aliases {
		if got := rdiAlarmSourceIndexFromValue(raw); got != want {
			t.Fatalf("rdiAlarmSourceIndexFromValue(%#v) = %d, want %d", raw, got, want)
		}
	}

	params := map[string]interface{}{
		"ignored":   "T1",
		"sensor_id": "sensor-2",
		"channel":   "T1",
	}
	if got := rdiAlarmSourceIndex(params, []string{"sensor_id", "channel"}); got != 2 {
		t.Fatalf("rdiAlarmSourceIndex should prefer the first configured key, got %d", got)
	}
	if got := rdiAlarmSourceIndex(nil, []string{"sensor_id"}); got != 0 {
		t.Fatalf("rdiAlarmSourceIndex nil params = %d, want 0", got)
	}
}

func TestParseRDIEmailRecipientsNormalizesDeduplicatesAndDropsInvalidEntries(t *testing.T) {
	got := parseRDIEmailRecipients("Ops <OPS@Example.com>, ops@example.com, bad-email, other@example.com, , Other@Example.com")

	if len(got) != 2 {
		t.Fatalf("parseRDIEmailRecipients length = %d, want 2: %#v", len(got), got)
	}
	if got[0] != "ops@example.com" || got[1] != "other@example.com" {
		t.Fatalf("parseRDIEmailRecipients = %#v, want normalized unique recipients", got)
	}
	if empty := parseRDIEmailRecipients(" , bad-email "); len(empty) != 0 {
		t.Fatalf("parseRDIEmailRecipients invalid-only = %#v, want empty", empty)
	}
}

func TestRDIAdditionalInfoAndAccessHelpersHandleBoundaryInputs(t *testing.T) {
	if got := parseAdditionalInfo(nil); len(got) != 0 {
		t.Fatalf("parseAdditionalInfo(nil) = %#v, want empty map", got)
	}
	if got := parseAdditionalInfo(StringPtr(" { ")); len(got) != 0 {
		t.Fatalf("parseAdditionalInfo(invalid) = %#v, want empty map", got)
	}
	additional := parseAdditionalInfo(StringPtr(`{"connection_type":" wifi ","numeric":12}`))
	if got := readString(additional, "connection_type", "unknown"); got != "wifi" {
		t.Fatalf("readString trimmed value = %q, want wifi", got)
	}
	if got := readString(additional, "numeric", "unknown"); got != "unknown" {
		t.Fatalf("readString non-string = %q, want fallback", got)
	}

	rdiTestRequireError(t, assertRDIDeviceAccess(nil, &utils.UserClaims{TenantID: "tenant-a"}), "nil RDI device access", errcode.CodeParamError, "device_id is required")
	rdiTestRequireError(t, assertRDIDeviceAccess(&model.Device{TenantID: "tenant-a"}, nil), "nil claims RDI device access", errcode.CodeNoPermission, "")
	if err := assertRDIDeviceAccess(&model.Device{TenantID: "tenant-a"}, &utils.UserClaims{TenantID: "tenant-a"}); err != nil {
		t.Fatalf("assertRDIDeviceAccess same tenant returned error: %v", err)
	}
	rdiTestRequireError(t, assertRDIDeviceAccess(&model.Device{TenantID: "tenant-a"}, &utils.UserClaims{TenantID: "tenant-b"}), "cross-tenant RDI device access", errcode.CodeNoPermission, "")
}
