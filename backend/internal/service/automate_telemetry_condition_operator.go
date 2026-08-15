package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

func (a *Automate) automateConditionCheckByOperator(operator string, condValue string, actualValue interface{}) bool {
	switch value := actualValue.(type) {
	case string:
		return a.automateConditionCheckByOperatorWithString(operator, condValue, value)
	case float64:
		return a.automateConditionCheckByOperatorWithFloat(operator, condValue, value)
	case bool:
		return a.automateConditionCheckByOperatorWithString(operator, condValue, fmt.Sprintf("%t", value))
	}
	return false
}

func float64Equal(a, b float64) bool {
	const threshold = 1e-9
	return math.Abs(a-b) < threshold
}

func parseLoggedConditionFloat(condValue string) (float64, bool) {
	condValueFloat, err := strconv.ParseFloat(strings.TrimSpace(condValue), 64)
	if err != nil {
		logrus.Error(err)
		return 0, false
	}
	return condValueFloat, true
}

func parseConditionBetweenRange(condValue string) (float64, float64, bool) {
	return parseEventBetweenRange(condValue)
}

func parseConditionInValues(condValue string) []string {
	rawValues := strings.Split(condValue, ",")
	values := make([]string, 0, len(rawValues))
	for _, value := range rawValues {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func compareStringValues(operator string, condValue string, actualValue string) bool {
	return compareOrderedStringValues(operator, condValue, actualValue)
}

func compareOrderedStringValues(operator string, condValue string, actualValue string) bool {
	actualNumber, actualIsNumber := strconv.ParseFloat(strings.TrimSpace(actualValue), 64)
	conditionNumber, conditionIsNumber := strconv.ParseFloat(strings.TrimSpace(condValue), 64)
	if actualIsNumber == nil && conditionIsNumber == nil {
		return compareParsedNumbers(operator, conditionNumber, actualNumber)
	}
	if actualIsNumber == nil || conditionIsNumber == nil {
		return false
	}
	return compareLexicalStrings(operator, condValue, actualValue)
}

func compareParsedNumbers(operator string, conditionNumber, actualNumber float64) bool {
	switch operator {
	case model.CONDITION_TRIGGER_OPERATOR_GT:
		return actualNumber > conditionNumber
	case model.CONDITION_TRIGGER_OPERATOR_LT:
		return actualNumber < conditionNumber
	case model.CONDITION_TRIGGER_OPERATOR_GTE:
		return actualNumber >= conditionNumber
	case model.CONDITION_TRIGGER_OPERATOR_LTE:
		return actualNumber <= conditionNumber
	}
	return false
}

func compareLexicalStrings(operator string, condValue string, actualValue string) bool {
	switch operator {
	case model.CONDITION_TRIGGER_OPERATOR_GT:
		return strings.Compare(actualValue, condValue) > 0
	case model.CONDITION_TRIGGER_OPERATOR_LT:
		return strings.Compare(actualValue, condValue) < 0
	case model.CONDITION_TRIGGER_OPERATOR_GTE:
		return strings.Compare(actualValue, condValue) >= 0
	case model.CONDITION_TRIGGER_OPERATOR_LTE:
		return strings.Compare(actualValue, condValue) <= 0
	}
	return false
}

func compareFloatValues(operator string, condValueFloat, actualValue float64) bool {
	switch operator {
	case model.CONDITION_TRIGGER_OPERATOR_EQ:
		return float64Equal(condValueFloat, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_NEQ:
		return !float64Equal(condValueFloat, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_GT:
		return actualValue > condValueFloat
	case model.CONDITION_TRIGGER_OPERATOR_LT:
		return actualValue < condValueFloat
	case model.CONDITION_TRIGGER_OPERATOR_GTE:
		return actualValue >= condValueFloat
	case model.CONDITION_TRIGGER_OPERATOR_LTE:
		return actualValue <= condValueFloat
	default:
		return false
	}
}

func (*Automate) automateConditionCheckByOperatorWithFloat(operator string, condValue string, actualValue float64) bool {
	switch operator {
	case model.CONDITION_TRIGGER_OPERATOR_EQ,
		model.CONDITION_TRIGGER_OPERATOR_NEQ,
		model.CONDITION_TRIGGER_OPERATOR_GT,
		model.CONDITION_TRIGGER_OPERATOR_LT,
		model.CONDITION_TRIGGER_OPERATOR_GTE,
		model.CONDITION_TRIGGER_OPERATOR_LTE:
		return compareFloatScalarCondition(operator, condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_BETWEEN:
		return compareFloatBetweenCondition(condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_IN:
		return compareFloatInCondition(condValue, actualValue)
	}
	return false
}

func compareFloatScalarCondition(operator string, condValue string, actualValue float64) bool {
	condValueFloat, ok := parseLoggedConditionFloat(condValue)
	if !ok {
		return false
	}
	if operator == model.CONDITION_TRIGGER_OPERATOR_NEQ {
		logrus.Tracef("condValueFloat=%f actualValue=%f equal=%t", condValueFloat, actualValue, float64Equal(condValueFloat, actualValue))
	}
	return compareFloatValues(operator, condValueFloat, actualValue)
}

func compareFloatBetweenCondition(condValue string, actualValue float64) bool {
	minValue, maxValue, ok := parseConditionBetweenRange(condValue)
	if !ok {
		return false
	}
	return actualValue >= minValue && actualValue <= maxValue
}

func compareFloatInCondition(condValue string, actualValue float64) bool {
	for _, v := range parseConditionInValues(condValue) {
		vFloat, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return false
		}
		if float64Equal(vFloat, actualValue) {
			return true
		}
	}
	return false
}

func stringValueWithinConditionRange(condValue, actualValue string) bool {
	actualValueFloat64, err := strconv.ParseFloat(actualValue, 64)
	if err == nil {
		minValue, maxValue, ok := parseConditionBetweenRange(condValue)
		return ok && actualValueFloat64 >= minValue && actualValueFloat64 <= maxValue
	}
	valParts := strings.Split(condValue, "-")
	if len(valParts) != 2 {
		return false
	}
	minValue := strings.TrimSpace(valParts[0])
	maxValue := strings.TrimSpace(valParts[1])
	return actualValue >= minValue && actualValue <= maxValue
}

func (*Automate) automateConditionCheckByOperatorWithString(operator string, condValue string, actualValue string) bool {
	logrus.Tracef("string condition compare operator=%s condValue=%s actualValue=%s result=%d", operator, condValue, actualValue, strings.Compare(actualValue, condValue))
	switch operator {
	case model.CONDITION_TRIGGER_OPERATOR_EQ:
		return compareStringEquality(condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_NEQ:
		return !compareStringEquality(condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_GT:
		return compareStringValues(operator, condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_LT:
		return compareStringValues(operator, condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_GTE:
		return compareStringValues(operator, condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_LTE:
		return compareStringValues(operator, condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_BETWEEN:
		return stringValueWithinConditionRange(condValue, actualValue)
	case model.CONDITION_TRIGGER_OPERATOR_IN:
		return compareStringInCondition(condValue, actualValue)
	}
	return false
}

func compareStringEquality(condValue string, actualValue string) bool {
	return strings.EqualFold(strings.ToUpper(actualValue), strings.ToUpper(condValue))
}

func compareStringInCondition(condValue string, actualValue string) bool {
	for _, v := range parseConditionInValues(condValue) {
		if v == actualValue {
			return true
		}
	}
	return false
}
