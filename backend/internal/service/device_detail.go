package service

import (
	"encoding/json"
	"fmt"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// GetDeviceByIDV1 returns the API detail view for one device.
func (*Device) GetDeviceByIDV1(id string, claims *utils.UserClaims) (map[string]interface{}, error) {
	device, err := ensureTelemetryDeviceReadAccess(id, claims)
	if err != nil {
		return nil, err
	}
	sharedReadOnly := !hasTelemetryTenantAccess(device, claims, false)
	data, err := loadDeviceDetail(id)
	if err != nil {
		return nil, err
	}

	if err := attachDeviceDetailEnrichment(data, id); err != nil {
		return nil, err
	}
	if sharedReadOnly {
		data = minimizeSharedDeviceDetail(data)
	}

	return data, nil
}

var sharedDeviceDetailFields = map[string]struct{}{
	"id":                  {},
	"name":                {},
	"is_enabled":          {},
	"activate_flag":       {},
	"created_at":          {},
	"update_at":           {},
	"device_number":       {},
	"product_id":          {},
	"parent_id":           {},
	"protocol":            {},
	"label":               {},
	"location":            {},
	"sub_device_addr":     {},
	"current_version":     {},
	"device_config_id":    {},
	"batch_number":        {},
	"activate_at":         {},
	"is_online":           {},
	"access_way":          {},
	"description":         {},
	"last_offline_time":   {},
	"device_config_name":  {},
	"gateway_device_name": {},
	"t":                   {},
	"ts":                  {},
	"warn_status":         {},
	"has_chart_config":    {},
	"device_status":       {},
}

func sharedDeviceConfigSummary(config *model.DeviceConfig) map[string]interface{} {
	if config == nil {
		return nil
	}
	return map[string]interface{}{
		"id":                 config.ID,
		"name":               config.Name,
		"device_template_id": config.DeviceTemplateID,
		"device_type":        config.DeviceType,
		"protocol_type":      config.ProtocolType,
		"device_conn_type":   config.DeviceConnType,
		"description":        config.Description,
		"image_url":          config.ImageURL,
	}
}

// minimizeSharedDeviceDetail is the response seam for accepted recipients.
// It uses an allowlist so new persistence fields do not become share-visible by
// default, and replaces the full DeviceConfig with the display-only subset.
func minimizeSharedDeviceDetail(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(sharedDeviceDetailFields)+2)
	for key := range sharedDeviceDetailFields {
		if value, ok := data[key]; ok {
			result[key] = value
		}
	}
	if config, ok := data["device_config"].(*model.DeviceConfig); ok {
		result["device_config"] = sharedDeviceConfigSummary(config)
	}
	result["shared_read_only"] = true
	return result
}

func loadDeviceDetail(id string) (map[string]interface{}, error) {
	data, err := dal.GetDeviceDetail(id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
			"message":   "get device failed",
		})
	}
	return data, nil
}

type deviceDetailConfigResult struct {
	config         *model.DeviceConfig
	hasChartConfig bool
	err            error
}

func attachDeviceDetailEnrichment(data map[string]interface{}, id string) error {
	alarmStatusCh := make(chan string, 1)
	configCh := make(chan deviceDetailConfigResult, 1)

	go func() {
		alarmStatusCh <- loadDeviceLatestAlarmStatus(id)
	}()

	go func() {
		config, hasChartConfig, err := loadDeviceConfigDetail(data)
		configCh <- deviceDetailConfigResult{
			config:         config,
			hasChartConfig: hasChartConfig,
			err:            err,
		}
	}()

	alarmStatus := <-alarmStatusCh
	configResult := <-configCh
	if configResult.err != nil {
		return configResult.err
	}

	data["warn_status"] = alarmStatus
	if configResult.config != nil {
		data["device_config"] = configResult.config
		data["has_chart_config"] = configResult.hasChartConfig
		data["device_status"] = data["is_online"]
	}
	return nil
}

func loadDeviceLatestAlarmStatus(id string) string {
	alarmStatus, err := dal.GetDeviceLatestAlarmStatus(id)
	if err != nil {
		logrus.Warn("[GetDeviceByIDV1] get device alarm status failed")
		alarmStatus = "N"
	}
	return alarmStatus
}

func loadDeviceConfigDetail(data map[string]interface{}) (*model.DeviceConfig, bool, error) {
	deviceConfigID, ok := deviceDetailConfigID(data)
	if !ok {
		return nil, false, nil
	}

	deviceConfig, err := dal.GetDeviceConfigByID(deviceConfigID)
	if err != nil {
		return nil, false, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
			"message":   "get device config failed",
		})
	}
	return deviceConfig, deviceConfigHasChartConfig(deviceConfig), nil
}

func deviceDetailConfigID(data map[string]interface{}) (string, bool) {
	v, ok := data["device_config_id"]
	if !ok || v == nil || v == "" {
		return "", false
	}
	return fmt.Sprintf("%v", v), true
}

func deviceConfigHasChartConfig(config *model.DeviceConfig) bool {
	if config == nil || config.DeviceTemplateID == nil || strings.TrimSpace(*config.DeviceTemplateID) == "" {
		return false
	}
	template, err := dal.GetDeviceTemplateChartConfigByID(strings.TrimSpace(*config.DeviceTemplateID), config.TenantID)
	if err != nil {
		logrus.Warn("[GetDeviceByIDV1] get thing model chart config failed")
		return false
	}
	return deviceTemplateChartConfigHasContent(template.WebChartConfig) || deviceTemplateChartConfigHasContent(template.AppChartConfig)
}

func deviceTemplateChartConfigHasContent(raw *string) bool {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return false
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(*raw), &payload); err != nil {
		return false
	}
	if nodes, ok := payload["nodes"].([]interface{}); ok && len(nodes) > 0 {
		return true
	}
	if _, ok := payload["canvas"].(map[string]interface{}); ok {
		return true
	}
	_, hasDataSources := payload["dataSources"].([]interface{})
	return hasDataSources
}
