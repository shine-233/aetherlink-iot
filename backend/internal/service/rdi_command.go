// rdi_command.go owns RDI command encoding and request helpers.
//
// It converts frontend/backend command inputs into protocol-specific payloads
// for RDI devices. Treat payload format changes as device protocol contract
// changes.
package service

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"aetherlink-iot/backend/pkg/errcode"
)

func allowedRDICommand(identifier string) bool {
	return isAllowedValue(identifier,
		"set_dry_contact",
		"set_alarm_config",
		"set_field_setting",
		"test_dry_contact",
		"ota_upgrade",
		"unbind_device",
		"factory_reset",
	)
}

func validateRDICommand(identifier string, params map[string]interface{}) error {
	if params == nil {
		params = map[string]interface{}{}
	}
	switch identifier {
	case "set_dry_contact":
		if err := requireEnumParam(params, "level", "high", "low"); err != nil {
			return err
		}
		return requireNumberRangeParam(params, "delay_seconds", rdiDryContactDelayMin, rdiDryContactDelayMax, true)
	case "set_alarm_config":
		return validateAlarmConfigCommand(params)
	case "set_field_setting":
		return validateFieldSettingCommand(params)
	case "test_dry_contact":
		if err := requireEnumParam(params, "level", "high", "low"); err != nil {
			return err
		}
		return requireNumberRangeParam(params, "duration_seconds", 1, rdiDryContactDelayMax, true)
	case "ota_upgrade":
		return validateOtaUpgradeCommand(params)
	case "unbind_device", "factory_reset":
		if len(params) > 0 {
			return errcode.NewWithMessage(errcode.CodeParamError, identifier+" does not accept params")
		}
	}
	return nil
}

func validateOtaUpgradeCommand(params map[string]interface{}) error {
	allowedKeys := map[string]struct{}{
		"firmware_url": {},
		"version":      {},
		"size":         {},
		"md5":          {},
	}
	for key := range params {
		if _, ok := allowedKeys[key]; !ok {
			return errcode.NewWithMessage(errcode.CodeParamError, "unsupported ota_upgrade parameter: "+key)
		}
	}
	for _, key := range []string{"firmware_url", "version", "md5"} {
		if err := requireStringParam(params, key); err != nil {
			return err
		}
	}
	firmwareURL := strings.TrimSpace(params["firmware_url"].(string))
	parsedURL, err := url.ParseRequestURI(firmwareURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" || !isAllowedValue(parsedURL.Scheme, "http", "https") {
		return errcode.NewWithMessage(errcode.CodeParamError, "firmware_url must be an http or https URL")
	}
	md5 := strings.TrimSpace(params["md5"].(string))
	if len(md5) != 32 {
		return errcode.NewWithMessage(errcode.CodeParamError, "md5 must be a 32-character hexadecimal string")
	}
	if _, err := hex.DecodeString(md5); err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "md5 must be a 32-character hexadecimal string")
	}
	return requireNumberRangeParam(params, "size", 1, 1<<40, true)
}

func validateAlarmConfigCommand(params map[string]interface{}) error {
	for _, validate := range []func(map[string]interface{}) error{
		validateAlarmConfigSupportedParams,
		validateAlarmConfigCollectionInterval,
		validateAlarmConfigTemperatureSensors,
		validateAlarmConfigBooleanFlags,
		validateAlarmConfigSwitchAlarmModes,
		validateAlarmConfigAlarmDurations,
		validateAlarmConfigDryContact,
		validateAlarmConfigEmailLists,
	} {
		if err := validate(params); err != nil {
			return err
		}
	}
	return nil
}

func validateAlarmConfigSupportedParams(params map[string]interface{}) error {
	for key := range params {
		if _, ok := rdiAlarmConfigAllowedParams[key]; !ok {
			return errcode.NewWithMessage(errcode.CodeParamError, "unsupported set_alarm_config parameter: "+key)
		}
	}
	return nil
}

func validateAlarmConfigCollectionInterval(params map[string]interface{}) error {
	return requireNumberRangeParam(params, "data_collection_interval", rdiCollectionIntervalMin, rdiCollectionIntervalMax, false)
}

func validateAlarmConfigTemperatureSensors(params map[string]interface{}) error {
	for _, pair := range rdiTemperatureSensorLimitPairs {
		lower, lowerOK, err := optionalNumberParam(params, pair.Lower)
		if err != nil {
			return err
		}
		upper, upperOK, err := optionalNumberParam(params, pair.Upper)
		if err != nil {
			return err
		}
		if lowerOK || upperOK {
			if !lowerOK || !upperOK {
				return errcode.NewWithMessage(errcode.CodeParamError, pair.Lower+" and "+pair.Upper+" must be provided together")
			}
			if err := validateTemperatureRange(pair.Prefix, lower, upper); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAlarmConfigBooleanFlags(params map[string]interface{}) error {
	for _, key := range rdiAlarmConfigBooleanFlagKeys {
		if err := optionalBoolParam(params, key); err != nil {
			return err
		}
	}
	return nil
}

func validateAlarmConfigSwitchAlarmModes(params map[string]interface{}) error {
	for _, key := range rdiSwitchAlarmModeKeys {
		if _, ok := params[key]; ok {
			if err := requireEnumParam(params, key, "powered_on", "powered_off", "disabled"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAlarmConfigAlarmDurations(params map[string]interface{}) error {
	for _, key := range rdiAlarmDurationKeys {
		if err := requireNumberRangeParam(params, key, rdiAlarmDurationMinSeconds, rdiAlarmDurationMaxSeconds, false); err != nil {
			return err
		}
	}
	return nil
}

func validateAlarmConfigDryContact(params map[string]interface{}) error {
	for _, key := range rdiDryContactLevelKeys {
		if _, ok := params[key]; ok {
			if err := requireEnumParam(params, key, "high", "low"); err != nil {
				return err
			}
		}
	}
	for _, key := range rdiDryContactDelayKeys {
		if err := requireNumberRangeParam(params, key, rdiDryContactDelayMin, rdiDryContactDelayMax, false); err != nil {
			return err
		}
	}
	return nil
}

func validateAlarmConfigEmailLists(params map[string]interface{}) error {
	for _, key := range rdiAlarmConfigEmailKeys {
		if raw, ok := params[key]; ok {
			value, ok := raw.(string)
			if !ok {
				return errcode.NewWithMessage(errcode.CodeParamError, key+" must be a string")
			}
			if err := validateEmailList(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFieldSettingCommand(params map[string]interface{}) error {
	if params == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "params is required")
	}
	if err := validateFieldSettingSupportedParams(params); err != nil {
		return err
	}
	hasN, err := validateFieldSettingNumberSlots(params)
	if err != nil {
		return err
	}
	hasSW, err := validateFieldSettingSwitchSlots(params)
	if err != nil {
		return err
	}
	if !hasN && !hasSW {
		return errcode.NewWithMessage(errcode.CodeParamError, "set_field_setting requires at least one n00-n07 or sw1-sw4 parameter")
	}
	return nil
}

func validateFieldSettingSupportedParams(params map[string]interface{}) error {
	for key := range params {
		if !isAllowedFieldSettingKey(key) {
			return errcode.NewWithMessage(errcode.CodeParamError, "unsupported set_field_setting parameter: "+key)
		}
	}
	return nil
}

func validateFieldSettingNumberSlots(params map[string]interface{}) (bool, error) {
	hasN := false
	for i := 0; i <= 7; i++ {
		key := fmt.Sprintf("n%02d", i)
		if raw, ok := params[key]; ok {
			if !isArrayParam(raw) {
				return false, errcode.NewWithMessage(errcode.CodeParamError, key+" must be an array")
			}
			hasN = true
		}
	}
	return hasN, nil
}

func validateFieldSettingSwitchSlots(params map[string]interface{}) (bool, error) {
	hasSW := false
	for i := 1; i <= 4; i++ {
		key := fmt.Sprintf("sw%d", i)
		if raw, ok := params[key]; ok {
			if !isObjectParam(raw) {
				return false, errcode.NewWithMessage(errcode.CodeParamError, key+" must be an object")
			}
			hasSW = true
		}
	}
	return hasSW, nil
}

func isAllowedFieldSettingKey(key string) bool {
	for i := 0; i <= 7; i++ {
		if key == fmt.Sprintf("n%02d", i) {
			return true
		}
	}
	for i := 1; i <= 4; i++ {
		if key == fmt.Sprintf("sw%d", i) {
			return true
		}
	}
	return false
}

func isArrayParam(value interface{}) bool {
	if value == nil {
		return false
	}
	kind := reflect.TypeOf(value).Kind()
	return kind == reflect.Slice || kind == reflect.Array
}

func isObjectParam(value interface{}) bool {
	if value == nil {
		return false
	}
	kind := reflect.TypeOf(value).Kind()
	return kind == reflect.Map || kind == reflect.Struct
}

func requireEnumParam(params map[string]interface{}, key string, allowed ...string) error {
	raw, ok := params[key]
	if !ok {
		return errcode.NewWithMessage(errcode.CodeParamError, key+" is required")
	}
	value, ok := raw.(string)
	if !ok || !isAllowedValue(value, allowed...) {
		return errcode.NewWithMessage(errcode.CodeParamError, key+" has an unsupported value")
	}
	return nil
}

func requireStringParam(params map[string]interface{}, key string) error {
	raw, ok := params[key]
	if !ok {
		return errcode.NewWithMessage(errcode.CodeParamError, key+" is required")
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, key+" must be a non-empty string")
	}
	return nil
}

func requireNumberRangeParam(params map[string]interface{}, key string, min float64, max float64, required bool) error {
	value, ok, err := optionalNumberParam(params, key)
	if err != nil {
		return err
	}
	if !ok {
		if required {
			return errcode.NewWithMessage(errcode.CodeParamError, key+" is required")
		}
		return nil
	}
	if value < min || value > max {
		return errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("%s must be between %g and %g", key, min, max))
	}
	return nil
}

func optionalNumberParam(params map[string]interface{}, key string) (float64, bool, error) {
	raw, ok := params[key]
	if !ok {
		return 0, false, nil
	}
	switch v := raw.(type) {
	case int:
		return float64(v), true, nil
	case int8:
		return float64(v), true, nil
	case int16:
		return float64(v), true, nil
	case int32:
		return float64(v), true, nil
	case int64:
		return float64(v), true, nil
	case uint:
		return float64(v), true, nil
	case uint8:
		return float64(v), true, nil
	case uint16:
		return float64(v), true, nil
	case uint32:
		return float64(v), true, nil
	case uint64:
		return float64(v), true, nil
	case float32:
		return float64(v), true, nil
	case float64:
		return v, true, nil
	case json.Number:
		value, err := v.Float64()
		if err != nil {
			return 0, true, errcode.NewWithMessage(errcode.CodeParamError, key+" must be a number")
		}
		return value, true, nil
	default:
		return 0, true, errcode.NewWithMessage(errcode.CodeParamError, key+" must be a number")
	}
}

func optionalBoolParam(params map[string]interface{}, key string) error {
	raw, ok := params[key]
	if !ok {
		return nil
	}
	if _, ok := raw.(bool); !ok {
		return errcode.NewWithMessage(errcode.CodeParamError, key+" must be a boolean")
	}
	return nil
}

func isAllowedValue(value string, allowed ...string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
