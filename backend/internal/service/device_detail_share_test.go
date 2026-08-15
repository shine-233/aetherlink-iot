package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func deviceDetailShareString(value string) *string {
	return &value
}

func TestMinimizeSharedDeviceDetailUsesDisplayOnlyAllowlist(t *testing.T) {
	data := map[string]interface{}{
		"id":                   "device-1",
		"name":                 "Shared RDI",
		"device_number":        "ABC123456789",
		"device_config_id":     "config-1",
		"device_config_name":   "RDI",
		"is_online":            int16(1),
		"voucher":              `{"username":"device","password":"secret"}`,
		"tenant_id":            "owner-tenant",
		"owner_user_id":        "owner-user",
		"protocol_config":      `{"password":"protocol-secret"}`,
		"additional_info":      `{"alarm_email":"owner@example.com"}`,
		"other_config":         `{"token":"top-level-other-secret"}`,
		"template_secret":      "top-level-template-secret",
		"future_secret_column": "must-not-become-visible",
		"device_config": &model.DeviceConfig{
			ID:               "config-1",
			Name:             "RDI",
			DeviceTemplateID: deviceDetailShareString("template-1"),
			DeviceType:       "1",
			ProtocolType:     deviceDetailShareString("MQTT"),
			VoucherType:      deviceDetailShareString("BASIC"),
			ProtocolConfig:   deviceDetailShareString(`{"password":"config-secret"}`),
			AdditionalInfo:   deviceDetailShareString(`{"private":true}`),
			TenantID:         "owner-tenant",
			OtherConfig:      deviceDetailShareString(`{"token":"other-secret"}`),
			TemplateSecret:   deviceDetailShareString("template-secret"),
		},
	}

	result := minimizeSharedDeviceDetail(data)
	for _, forbidden := range []string{
		"voucher",
		"tenant_id",
		"owner_user_id",
		"protocol_config",
		"additional_info",
		"other_config",
		"template_secret",
		"future_secret_column",
	} {
		if _, ok := result[forbidden]; ok {
			t.Fatalf("shared detail exposed forbidden field %q: %#v", forbidden, result[forbidden])
		}
	}
	if result["shared_read_only"] != true {
		t.Fatalf("shared_read_only = %#v, want true", result["shared_read_only"])
	}
	if result["id"] != "device-1" || result["device_config_id"] != "config-1" {
		t.Fatalf("shared display identity fields changed: %#v", result)
	}

	config, ok := result["device_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("device_config type = %T, want display-only map", result["device_config"])
	}
	for _, forbidden := range []string{
		"voucher_type",
		"protocol_config",
		"additional_info",
		"tenant_id",
		"other_config",
		"template_secret",
	} {
		if _, ok := config[forbidden]; ok {
			t.Fatalf("shared config exposed forbidden field %q: %#v", forbidden, config[forbidden])
		}
	}
	templateID, ok := config["device_template_id"].(*string)
	if !ok || templateID == nil || *templateID != "template-1" {
		t.Fatalf("device_template_id = %#v, want template-1", config["device_template_id"])
	}
}
