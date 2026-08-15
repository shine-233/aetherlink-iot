package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

const (
	eventParamMatchModeField = "field"
	eventOperatorExists      = "exists"
)

var eventBetweenRangePattern = regexp.MustCompile(`^\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+))\s*-\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+))\s*$`)

type eventParamMatchConfig struct {
	MatchMode  string                `json:"match_mode"`
	Conditions []eventParamCondition `json:"conditions"`
}

type eventParamCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type normalizedEventParamCondition struct {
	field    string
	operator string
	value    interface{}
}

func (a *Automate) automateEventParamConditionCheck(triggerValue string, actualValue interface{}) (bool, string, bool) {
	config, handled := parseEventParamMatchConfig(triggerValue)
	if !handled {
		return false, "", false
	}
	return a.evaluateEventParamMatchConfig(config, actualValue)
}

func parseEventParamMatchConfig(triggerValue string) (eventParamMatchConfig, bool) {
	var config eventParamMatchConfig
	if err := json.Unmarshal([]byte(triggerValue), &config); err != nil {
		return eventParamMatchConfig{}, false
	}
	if config.MatchMode != eventParamMatchModeField {
		return eventParamMatchConfig{}, false
	}
	return config, true
}

func (a *Automate) evaluateEventParamMatchConfig(config eventParamMatchConfig, actualValue interface{}) (bool, string, bool) {
	if len(config.Conditions) == 0 {
		return false, "event param match config has no conditions", true
	}

	actualMap, err := parseEventActualValue(actualValue)
	if err != nil {
		logrus.WithError(err).Debug("failed to parse event param match actual value")
		return false, "event actual value is not a valid JSON object", true
	}

	return a.evaluateEventParamConditions(actualMap, config.Conditions)
}

func (a *Automate) evaluateEventParamConditions(actualMap map[string]interface{}, conditions []eventParamCondition) (bool, string, bool) {
	for _, condition := range conditions {
		// Field conditions are evaluated in declaration order and short-circuit on
		// the first mismatch so the caller gets the first actionable reason.
		ok, detail := a.matchEventParamCondition(actualMap, condition)
		if !ok {
			return false, detail, true
		}
	}

	return true, "event param conditions matched", true
}

func parseEventActualValue(actualValue interface{}) (map[string]interface{}, error) {
	ensureObject := func(data map[string]interface{}, err error) (map[string]interface{}, error) {
		if err != nil {
			return nil, err
		}
		if data == nil {
			return nil, errors.New("event actual value must be a JSON object")
		}
		return data, nil
	}

	switch value := actualValue.(type) {
	case string:
		var data map[string]interface{}
		return ensureObject(data, json.Unmarshal([]byte(value), &data))
	case []byte:
		var data map[string]interface{}
		return ensureObject(data, json.Unmarshal(value, &data))
	case map[string]interface{}:
		return ensureObject(value, nil)
	default:
		// Fall back to a JSON round-trip so weakly typed cache or message payloads
		// are normalized into one object-shaped path.
		bytes, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var data map[string]interface{}
		return ensureObject(data, json.Unmarshal(bytes, &data))
	}
}

func (a *Automate) matchEventParamCondition(data map[string]interface{}, condition eventParamCondition) (bool, string) {
	normalized, errDetail := normalizeEventParamCondition(condition)
	if errDetail != "" {
		return false, errDetail
	}

	actualValue, exists := getValueByDotPath(data, normalized.field)
	if normalized.operator == eventOperatorExists {
		return matchEventParamExistsCondition(normalized.field, normalized.value, exists)
	}
	if !exists {
		return false, fmt.Sprintf("event field [%s] not found", normalized.field)
	}

	return a.matchResolvedEventParamCondition(normalized, actualValue)
}

func normalizeEventParamCondition(condition eventParamCondition) (normalizedEventParamCondition, string) {
	field := strings.TrimSpace(condition.Field)
	if field == "" {
		return normalizedEventParamCondition{}, "event condition field is required"
	}

	operator := strings.TrimSpace(condition.Operator)
	if operator == "" {
		operator = model.CONDITION_TRIGGER_OPERATOR_EQ
	}

	return normalizedEventParamCondition{
		field:    field,
		operator: operator,
		value:    condition.Value,
	}, ""
}

func validateEventParamMatchConfigShape(config eventParamMatchConfig) string {
	if config.MatchMode != eventParamMatchModeField {
		return "event trigger_value match_mode must be field"
	}
	if len(config.Conditions) == 0 {
		return "event trigger_value must contain at least one condition"
	}
	for _, condition := range config.Conditions {
		if detail := validateEventParamConditionShape(condition); detail != "" {
			return detail
		}
	}
	return ""
}

func validateEventParamConditionShape(condition eventParamCondition) string {
	normalized, detail := normalizeEventParamCondition(condition)
	if detail != "" {
		if detail == "event condition field is required" {
			return "event trigger_value condition field is required"
		}
		return detail
	}

	switch normalized.operator {
	case model.CONDITION_TRIGGER_OPERATOR_EQ, model.CONDITION_TRIGGER_OPERATOR_NEQ:
		if isBlankEventParamValidationValue(normalized.value) {
			return "event trigger_value condition value is required"
		}
	case model.CONDITION_TRIGGER_OPERATOR_GT, model.CONDITION_TRIGGER_OPERATOR_LT,
		model.CONDITION_TRIGGER_OPERATOR_GTE, model.CONDITION_TRIGGER_OPERATOR_LTE:
		if _, ok := toFloat64(normalized.value); !ok {
			return "event trigger_value numeric operator requires a number"
		}
	case model.CONDITION_TRIGGER_OPERATOR_BETWEEN:
		if !isValidEventBetweenValidationValue(normalized.value) {
			return "event trigger_value between operator requires two ordered numeric values"
		}
	case model.CONDITION_TRIGGER_OPERATOR_IN:
		if !isValidEventInValidationValue(normalized.value) {
			return "event trigger_value in operator requires a non-empty list"
		}
	case eventOperatorExists:
		if _, ok := normalized.value.(bool); !ok {
			return "event trigger_value exists operator requires a boolean value"
		}
	default:
		return fmt.Sprintf("event trigger_value operator [%s] is not supported", normalized.operator)
	}

	return ""
}

func isBlankEventParamValidationValue(value interface{}) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func isValidEventBetweenValidationValue(value interface{}) bool {
	switch typed := value.(type) {
	case []interface{}:
		if len(typed) != 2 {
			return false
		}
		minValue, minOK := toFloat64(typed[0])
		maxValue, maxOK := toFloat64(typed[1])
		return minOK && maxOK && minValue <= maxValue
	case string:
		_, _, ok := parseEventBetweenRange(typed)
		return ok
	default:
		return false
	}
}

func isValidEventInValidationValue(value interface{}) bool {
	switch typed := value.(type) {
	case []interface{}:
		if len(typed) == 0 {
			return false
		}
		for _, item := range typed {
			if isBlankEventParamValidationValue(item) {
				return false
			}
		}
		return true
	case string:
		for _, item := range strings.Split(typed, ",") {
			if strings.TrimSpace(item) != "" {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func matchEventParamExistsCondition(field string, expectedValue interface{}, exists bool) (bool, string) {
	expected := true
	if expectedValue != nil {
		expected = toBool(expectedValue)
	}
	if exists == expected {
		return true, fmt.Sprintf("event field [%s] exists = %t", field, expected)
	}
	return false, fmt.Sprintf("event field [%s] exists = %t, actual = %t", field, expected, exists)
}

func (a *Automate) matchResolvedEventParamCondition(condition normalizedEventParamCondition, actualValue interface{}) (bool, string) {
	ok := a.matchEventParamValue(condition.operator, condition.value, actualValue)
	return ok, formatEventParamConditionDetail(condition.field, actualValue, condition.operator, condition.value, ok)
}

func formatEventParamConditionDetail(field string, actualValue interface{}, operator string, expectedValue interface{}, ok bool) string {
	detail := fmt.Sprintf("event field [%s]: %v %s %v", field, actualValue, operator, expectedValue)
	if ok {
		return detail
	}
	return detail + " did not match"
}

func getValueByDotPath(data map[string]interface{}, path string) (interface{}, bool) {
	var current interface{} = data
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		value, exists := currentMap[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func (a *Automate) matchEventParamValue(operator string, expected interface{}, actual interface{}) bool {
	if actualValues, ok := actual.([]interface{}); ok && operator != model.CONDITION_TRIGGER_OPERATOR_BETWEEN {
		if operator == model.CONDITION_TRIGGER_OPERATOR_NEQ {
			if len(actualValues) == 0 {
				return false
			}
			for _, actualItem := range actualValues {
				if a.matchEventParamValue(model.CONDITION_TRIGGER_OPERATOR_EQ, expected, actualItem) {
					return false
				}
			}
			return true
		}
		for _, actualItem := range actualValues {
			if a.matchEventParamValue(operator, expected, actualItem) {
				return true
			}
		}
		return false
	}

	switch operator {
	case model.CONDITION_TRIGGER_OPERATOR_EQ:
		return eventValuesEqual(actual, expected)
	case model.CONDITION_TRIGGER_OPERATOR_NEQ:
		return !eventValuesEqual(actual, expected)
	case model.CONDITION_TRIGGER_OPERATOR_GT, model.CONDITION_TRIGGER_OPERATOR_LT,
		model.CONDITION_TRIGGER_OPERATOR_GTE, model.CONDITION_TRIGGER_OPERATOR_LTE:
		return compareEventNumber(operator, actual, expected)
	case model.CONDITION_TRIGGER_OPERATOR_BETWEEN:
		return compareEventBetween(actual, expected)
	case model.CONDITION_TRIGGER_OPERATOR_IN:
		return compareEventIn(actual, expected)
	default:
		return false
	}
}

func eventValuesEqual(actual interface{}, expected interface{}) bool {
	if actualNumber, ok := toFloat64(actual); ok {
		if expectedNumber, ok := toFloat64(expected); ok {
			return float64Equal(actualNumber, expectedNumber)
		}
	}
	if actualBool, ok := actual.(bool); ok {
		expectedBool, ok := toBoolValue(expected)
		return ok && actualBool == expectedBool
	}
	if expectedBool, ok := expected.(bool); ok {
		actualBool, ok := toBoolValue(actual)
		return ok && actualBool == expectedBool
	}
	if actual == nil || expected == nil {
		return actual == nil && expected == nil
	}
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
}

func compareEventNumber(operator string, actual interface{}, expected interface{}) bool {
	actualNumber, ok := toFloat64(actual)
	if !ok {
		return false
	}
	expectedNumber, ok := toFloat64(expected)
	if !ok {
		return false
	}
	switch operator {
	case model.CONDITION_TRIGGER_OPERATOR_GT:
		return actualNumber > expectedNumber
	case model.CONDITION_TRIGGER_OPERATOR_LT:
		return actualNumber < expectedNumber
	case model.CONDITION_TRIGGER_OPERATOR_GTE:
		return actualNumber >= expectedNumber
	case model.CONDITION_TRIGGER_OPERATOR_LTE:
		return actualNumber <= expectedNumber
	}
	return false
}

func compareEventBetween(actual interface{}, expected interface{}) bool {
	actualNumber, ok := toFloat64(actual)
	if !ok {
		return false
	}

	var minValue, maxValue float64
	switch value := expected.(type) {
	case []interface{}:
		if len(value) != 2 {
			return false
		}
		var minOK, maxOK bool
		minValue, minOK = toFloat64(value[0])
		maxValue, maxOK = toFloat64(value[1])
		if !minOK || !maxOK {
			return false
		}
	case string:
		var parseOK bool
		minValue, maxValue, parseOK = parseEventBetweenRange(value)
		if !parseOK {
			return false
		}
	default:
		return false
	}

	return actualNumber >= minValue && actualNumber <= maxValue
}

func parseEventBetweenRange(value string) (float64, float64, bool) {
	parts := eventBetweenRangePattern.FindStringSubmatch(value)
	if len(parts) != 3 {
		return 0, 0, false
	}
	minValue, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, false
	}
	maxValue, err := strconv.ParseFloat(parts[2], 64)
	if err != nil || minValue > maxValue {
		return 0, 0, false
	}
	return minValue, maxValue, true
}

func compareEventIn(actual interface{}, expected interface{}) bool {
	switch values := expected.(type) {
	case []interface{}:
		for _, value := range values {
			if eventValuesEqual(actual, value) {
				return true
			}
		}
	case string:
		for _, value := range strings.Split(values, ",") {
			if eventValuesEqual(actual, strings.TrimSpace(value)) {
				return true
			}
		}
	}
	return false
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func toBool(value interface{}) bool {
	result, _ := toBoolValue(value)
	return result
}

func toBoolValue(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		return b, err == nil
	default:
		return false, false
	}
}
