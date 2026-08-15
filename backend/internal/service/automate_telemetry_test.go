// 文件用途：验证自动化场景触发、装饰和动作执行的服务行为。
// 核心逻辑：用表驱动用例覆盖遥测条件、事件参数、执行失败和装饰结果生成。
// 关键注意事项：自动化触发顺序直接影响用户场景，测试需同时保护匹配语义、限流语义和失败隔离。
// 重构建议：按条件解析、动作副作用和装饰输出拆分夹具，增加事务失败与外部执行失败边界。
// automate_telemetry_test.go protects automation telemetry matching behavior.
//
// Purpose: exercise telemetry condition helpers, event-parameter matching, scene filtering, and failed-execution handling for automation triggers.
// Core logic: uses focused unit tests around type conversion, comparison operators, nested JSON event fields, and ExecuteRun side effects.
// Important notes: trigger semantics are user-visible automation behavior, so broadening operators or payload parsing needs positive and negative cases here.
// Refactor suggestion: split this large suite by helper domain if automation telemetry adds more trigger families.
package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// --- containString ---

func TestAutomateTelemetryContainString_Found(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.containString([]string{"temp", "humidity", "pressure"}, "humidity"))
}

func TestAutomateTelemetryContainString_NotFound(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.containString([]string{"temp", "humidity"}, "voltage"))
}

func TestAutomateTelemetryContainString_EmptySlice(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.containString([]string{}, "temp"))
}

func TestAutomateTelemetryContainString_NilSlice(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.containString(nil, "temp"))
}

// --- automateConditionCheckWithTime ---

func TestAutomateTelemetryConditionCheckWithTime(t *testing.T) {
	originalNow := automateNow
	t.Cleanup(func() {
		automateNow = originalNow
	})

	fixedMonday := time.Date(2026, 6, 22, 10, 30, 0, 0, time.UTC)
	a := &Automate{}

	tests := []struct {
		name         string
		triggerValue string
		want         bool
	}{
		{
			name:         "matches current weekday and time window",
			triggerValue: "1|10:00:00+00:00|11:00:00+00:00",
			want:         true,
		},
		{
			name:         "start and end are inclusive",
			triggerValue: "1|10:30:00+00:00|10:30:00+00:00",
			want:         true,
		},
		{
			name:         "rejects non matching weekday",
			triggerValue: "2|10:00:00+00:00|11:00:00+00:00",
			want:         false,
		},
		{
			name:         "rejects start after current time",
			triggerValue: "1|10:31:00+00:00|11:00:00+00:00",
			want:         false,
		},
		{
			name:         "rejects end before current time",
			triggerValue: "1|10:00:00+00:00|10:29:59+00:00",
			want:         false,
		},
		{
			name:         "supports overnight window before midnight",
			triggerValue: "1|23:00:00+00:00|02:00:00+00:00",
			want:         true,
		},
		{
			name:         "rejects malformed start time",
			triggerValue: "1|bad|11:00:00+00:00",
			want:         false,
		},
		{
			name:         "rejects missing end time",
			triggerValue: "1|10:00:00+00:00",
			want:         false,
		},
		{
			name:         "rejects empty trigger value",
			triggerValue: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentTime := fixedMonday
			if tt.name == "supports overnight window before midnight" {
				currentTime = time.Date(2026, 6, 22, 23, 30, 0, 0, time.UTC)
			}
			automateNow = func() time.Time { return currentTime }
			got := a.automateConditionCheckWithTime(model.DeviceTriggerCondition{TriggerValue: tt.triggerValue})
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- float64Equal ---

func TestAutomateTelemetryFloat64Equal_SameValue(t *testing.T) {
	assert.True(t, float64Equal(1.0, 1.0))
}

func TestAutomateTelemetryFloat64Equal_CloseValue(t *testing.T) {
	assert.True(t, float64Equal(1.0, 1.0+1e-10))
}

func TestAutomateTelemetryFloat64Equal_DifferentValue(t *testing.T) {
	assert.False(t, float64Equal(1.0, 2.0))
}

func TestAutomateTelemetryFloat64Equal_ZeroValues(t *testing.T) {
	assert.True(t, float64Equal(0.0, 0.0))
}

func TestAutomateTelemetryFloat64Equal_NegativeValues(t *testing.T) {
	assert.True(t, float64Equal(-1.5, -1.5))
}

// --- toFloat64 ---

func TestAutomateTelemetryToFloat64_Float64(t *testing.T) {
	v, ok := toFloat64(3.14)
	assert.True(t, ok)
	assert.Equal(t, 3.14, v)
}

func TestAutomateTelemetryToFloat64_Float32(t *testing.T) {
	v, ok := toFloat64(float32(3.14))
	assert.True(t, ok)
	assert.InDelta(t, 3.14, v, 0.01)
}

func TestAutomateTelemetryToFloat64_Int(t *testing.T) {
	v, ok := toFloat64(42)
	assert.True(t, ok)
	assert.Equal(t, 42.0, v)
}

func TestAutomateTelemetryToFloat64_Int64(t *testing.T) {
	v, ok := toFloat64(int64(42))
	assert.True(t, ok)
	assert.Equal(t, 42.0, v)
}

func TestAutomateTelemetryToFloat64_Int32(t *testing.T) {
	v, ok := toFloat64(int32(42))
	assert.True(t, ok)
	assert.Equal(t, 42.0, v)
}

func TestAutomateTelemetryToFloat64_JsonNumber(t *testing.T) {
	v, ok := toFloat64(json.Number("3.14"))
	assert.True(t, ok)
	assert.Equal(t, 3.14, v)
}

func TestAutomateTelemetryToFloat64_StringValid(t *testing.T) {
	v, ok := toFloat64("3.14")
	assert.True(t, ok)
	assert.Equal(t, 3.14, v)
}

func TestAutomateTelemetryToFloat64_StringInvalid(t *testing.T) {
	_, ok := toFloat64("not-a-number")
	assert.False(t, ok)
}

func TestAutomateTelemetryToFloat64_UnsupportedType(t *testing.T) {
	_, ok := toFloat64([]int{1, 2, 3})
	assert.False(t, ok)
}

func TestAutomateTelemetryToFloat64_Bool(t *testing.T) {
	_, ok := toFloat64(true)
	assert.False(t, ok)
}

// --- toBool ---

func TestAutomateTelemetryToBool_BoolTrue(t *testing.T) {
	assert.True(t, toBool(true))
}

func TestAutomateTelemetryToBool_BoolFalse(t *testing.T) {
	assert.False(t, toBool(false))
}

func TestAutomateTelemetryToBool_StringTrue(t *testing.T) {
	assert.True(t, toBool("true"))
}

func TestAutomateTelemetryToBool_StringFalse(t *testing.T) {
	assert.False(t, toBool("false"))
}

func TestAutomateTelemetryToBool_String1(t *testing.T) {
	assert.True(t, toBool("1"))
}

func TestAutomateTelemetryToBool_String0(t *testing.T) {
	assert.False(t, toBool("0"))
}

func TestAutomateTelemetryToBool_InvalidString(t *testing.T) {
	assert.False(t, toBool("not-a-bool"))
}

func TestAutomateTelemetryToBool_Int(t *testing.T) {
	assert.False(t, toBool(1))
}

// --- automateConditionCheckByOperatorWithFloat ---

func TestAutomateTelemetryConditionCheckWithFloat_EQ(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("=", "10.5", 10.5))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("=", "10.5", 10.6))
}

func TestAutomateTelemetryConditionCheckWithFloat_NEQ(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("!=", "10.5", 10.6))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("!=", "10.5", 10.5))
}

func TestAutomateTelemetryConditionCheckWithFloat_GT(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithFloat(">", "10", 11))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat(">", "10", 9))
}

func TestAutomateTelemetryConditionCheckWithFloat_LT(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("<", "10", 9))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("<", "10", 11))
}

func TestAutomateTelemetryConditionCheckWithFloat_GTE(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithFloat(">=", "10", 10))
	assert.True(t, a.automateConditionCheckByOperatorWithFloat(">=", "10", 11))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat(">=", "10", 9))
}

func TestAutomateTelemetryConditionCheckWithFloat_LTE(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("<=", "10", 10))
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("<=", "10", 9))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("<=", "10", 11))
}

func TestAutomateTelemetryConditionCheckWithFloat_BETWEEN(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("between", "10-20", 15))
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("between", "10-20", 10))
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("between", "10-20", 20))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("between", "10-20", 9))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("between", "10-20", 21))
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("between", "-10--1", -5))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("between", "-10--1", 0))
}

func TestAutomateTelemetryConditionCheckWithFloat_BETWEEN_InvalidFormat(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("between", "10", 15))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("between", "a-b", 15))
}

func TestAutomateTelemetryConditionCheckWithFloat_IN(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("in", "10,20,30", 20))
	assert.True(t, a.automateConditionCheckByOperatorWithFloat("in", "10, 20, 30", 20))
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("in", "10,20,30", 15))
}

func TestAutomateTelemetryConditionCheckWithFloat_InvalidCondValue(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("=", "not-a-number", 10))
}

func TestAutomateTelemetryConditionCheckWithFloat_InvalidCondValueForComparisons(t *testing.T) {
	a := &Automate{}

	for _, operator := range []string{"=", "!=", ">", "<", ">=", "<="} {
		t.Run(operator, func(t *testing.T) {
			assert.False(t, a.automateConditionCheckByOperatorWithFloat(operator, "not-a-number", 10))
		})
	}
}

func TestAutomateTelemetryConditionCheckWithFloat_UnknownOperator(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.automateConditionCheckByOperatorWithFloat("???", "10", 10))
}

// --- automateConditionCheckByOperatorWithString ---

func TestAutomateTelemetryConditionCheckWithString_EQ(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString("=", "hello", "hello"))
	assert.True(t, a.automateConditionCheckByOperatorWithString("=", "Hello", "HELLO")) // case insensitive
	assert.False(t, a.automateConditionCheckByOperatorWithString("=", "hello", "world"))
}

func TestAutomateTelemetryConditionCheckWithString_NEQ(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString("!=", "hello", "world"))
	assert.False(t, a.automateConditionCheckByOperatorWithString("!=", "hello", "hello"))
	assert.False(t, a.automateConditionCheckByOperatorWithString("!=", "Hello", "HELLO"))
}

func TestAutomateTelemetryConditionCheckWithString_GT_Numeric(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString(">", "10", "20"))
	assert.False(t, a.automateConditionCheckByOperatorWithString(">", "10", "5"))
}

func TestAutomateTelemetryConditionCheckWithString_GT_Lexicographic(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString(">", "a", "b"))
	assert.False(t, a.automateConditionCheckByOperatorWithString(">", "10", "9a"))
	assert.False(t, a.automateConditionCheckByOperatorWithString(">", "10a", "9"))
}

func TestAutomateTelemetryConditionCheckWithString_LT_Numeric(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString("<", "10", "5"))
	assert.False(t, a.automateConditionCheckByOperatorWithString("<", "10", "20"))
}

func TestAutomateTelemetryConditionCheckWithString_GTE(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString(">=", "10", "10"))
	assert.True(t, a.automateConditionCheckByOperatorWithString(">=", "10", "20"))
}

func TestAutomateTelemetryConditionCheckWithString_LTE(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString("<=", "10", "10"))
	assert.True(t, a.automateConditionCheckByOperatorWithString("<=", "10", "5"))
}

func TestAutomateTelemetryConditionCheckWithString_BETWEEN_Numeric(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString("between", "10-20", "15"))
	assert.True(t, a.automateConditionCheckByOperatorWithString("between", "10-20", "10"))
	assert.True(t, a.automateConditionCheckByOperatorWithString("between", "10-20", "20"))
	assert.False(t, a.automateConditionCheckByOperatorWithString("between", "10-20", "9"))
	assert.False(t, a.automateConditionCheckByOperatorWithString("between", "10-20", "25"))
	assert.True(t, a.automateConditionCheckByOperatorWithString("between", "-10--1", "-5"))
	assert.False(t, a.automateConditionCheckByOperatorWithString("between", "-10--1", "0"))
}

func TestAutomateTelemetryConditionCheckWithString_BETWEEN_Lexicographic(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString("between", "a-z", "a"))
	assert.True(t, a.automateConditionCheckByOperatorWithString("between", "a-z", "m"))
	assert.True(t, a.automateConditionCheckByOperatorWithString("between", "a-z", "z"))
}

func TestAutomateTelemetryConditionCheckWithString_IN(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperatorWithString("in", "a,b,c", "b"))
	assert.True(t, a.automateConditionCheckByOperatorWithString("in", "a, b, c", "b"))
	assert.False(t, a.automateConditionCheckByOperatorWithString("in", "a,b,c", "d"))
}

func TestAutomateTelemetryConditionCheckWithString_UnknownOperator(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.automateConditionCheckByOperatorWithString("???", "hello", "hello"))
}

// --- automateConditionCheckByOperator ---

func TestAutomateTelemetryConditionCheckByOperator_StringValue(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperator("=", "hello", "hello"))
}

func TestAutomateTelemetryConditionCheckByOperator_FloatValue(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperator("=", "10.5", 10.5))
}

func TestAutomateTelemetryConditionCheckByOperator_BoolValue(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.automateConditionCheckByOperator("=", "true", true))
}

func TestAutomateTelemetryConditionCheckByOperator_UnsupportedType(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.automateConditionCheckByOperator("=", "10", []int{1}))
}

// --- eventValuesEqual ---

func TestAutomateTelemetryEventValuesEqual_NumberEqual(t *testing.T) {
	assert.True(t, eventValuesEqual(10, 10))
	assert.True(t, eventValuesEqual(10.0, float64(10)))
	assert.True(t, eventValuesEqual(10, "10"))
	assert.True(t, eventValuesEqual("10.0", 10))
}

func TestAutomateTelemetryEventValuesEqual_NumberNotEqual(t *testing.T) {
	assert.False(t, eventValuesEqual(10, 20))
	assert.False(t, eventValuesEqual(10, "10.5"))
}

func TestAutomateTelemetryEventValuesEqual_BoolEqual(t *testing.T) {
	assert.True(t, eventValuesEqual(true, true))
	assert.True(t, eventValuesEqual(true, "true"))
	assert.True(t, eventValuesEqual("false", false))
	assert.True(t, eventValuesEqual("1", true))
	assert.True(t, eventValuesEqual("0", false))
	assert.False(t, eventValuesEqual(true, false))
	assert.False(t, eventValuesEqual(true, "false"))
	assert.False(t, eventValuesEqual("true", false))
	assert.False(t, eventValuesEqual("not-a-bool", false))
}

func TestAutomateTelemetryEventValuesEqual_StringEqual(t *testing.T) {
	assert.True(t, eventValuesEqual("hello", "hello"))
	assert.False(t, eventValuesEqual("hello", "world"))
}

func TestAutomateTelemetryEventValuesEqual_NilEqual(t *testing.T) {
	assert.True(t, eventValuesEqual(nil, nil))
	assert.False(t, eventValuesEqual(nil, "value"))
	assert.False(t, eventValuesEqual("value", nil))
}

// --- compareEventNumber ---

func TestAutomateTelemetryCompareEventNumber_GT(t *testing.T) {
	assert.True(t, compareEventNumber(">", 10, 5))
	assert.False(t, compareEventNumber(">", 5, 10))
}

func TestAutomateTelemetryCompareEventNumber_LT(t *testing.T) {
	assert.True(t, compareEventNumber("<", 5, 10))
	assert.False(t, compareEventNumber("<", 10, 5))
}

func TestAutomateTelemetryCompareEventNumber_GTE(t *testing.T) {
	assert.True(t, compareEventNumber(">=", 10, 10))
	assert.True(t, compareEventNumber(">=", 10, 5))
	assert.False(t, compareEventNumber(">=", 5, 10))
}

func TestAutomateTelemetryCompareEventNumber_LTE(t *testing.T) {
	assert.True(t, compareEventNumber("<=", 10, 10))
	assert.True(t, compareEventNumber("<=", 5, 10))
	assert.False(t, compareEventNumber("<=", 10, 5))
}

func TestAutomateTelemetryCompareEventNumber_InvalidActual(t *testing.T) {
	assert.False(t, compareEventNumber(">", "not-a-number", 5))
}

func TestAutomateTelemetryCompareEventNumber_InvalidExpected(t *testing.T) {
	assert.False(t, compareEventNumber(">", 10, "not-a-number"))
}

func TestAutomateTelemetryCompareEventNumber_UnknownOperator(t *testing.T) {
	assert.False(t, compareEventNumber("???", 10, 5))
}

// --- compareEventBetween ---

func TestAutomateTelemetryCompareEventBetween_Array(t *testing.T) {
	assert.True(t, compareEventBetween(15, []interface{}{10, 20}))
	assert.True(t, compareEventBetween(10, []interface{}{10, 20}))
	assert.False(t, compareEventBetween(25, []interface{}{10, 20}))
}

func TestAutomateTelemetryCompareEventBetween_String(t *testing.T) {
	assert.True(t, compareEventBetween(15, "10-20"))
	assert.False(t, compareEventBetween(25, "10-20"))
}

func TestAutomateTelemetryCompareEventBetween_NegativeStringRange(t *testing.T) {
	assert.True(t, compareEventBetween(-5, "-10--1"))
	assert.True(t, compareEventBetween(-10, "-10--1"))
	assert.True(t, compareEventBetween(-1, "-10--1"))
	assert.False(t, compareEventBetween(0, "-10--1"))
}

func TestAutomateTelemetryCompareEventBetween_InvalidArray(t *testing.T) {
	assert.False(t, compareEventBetween(15, []interface{}{10}))
}

func TestAutomateTelemetryCompareEventBetween_InvalidString(t *testing.T) {
	assert.False(t, compareEventBetween(15, "10"))
}

func TestAutomateTelemetryCompareEventBetween_InvalidType(t *testing.T) {
	assert.False(t, compareEventBetween(15, 42))
}

func TestAutomateTelemetryCompareEventBetween_InvalidActual(t *testing.T) {
	assert.False(t, compareEventBetween("not-a-number", "10-20"))
}

// --- compareEventIn ---

func TestAutomateTelemetryCompareEventIn_Array(t *testing.T) {
	assert.True(t, compareEventIn("b", []interface{}{"a", "b", "c"}))
	assert.False(t, compareEventIn("d", []interface{}{"a", "b", "c"}))
}

func TestAutomateTelemetryCompareEventIn_String(t *testing.T) {
	assert.True(t, compareEventIn("b", "a,b,c"))
	assert.True(t, compareEventIn("b", "a, b , c"))
	assert.False(t, compareEventIn("d", "a,b,c"))
}

func TestAutomateTelemetryCompareEventIn_InvalidType(t *testing.T) {
	assert.False(t, compareEventIn("b", 42))
}

// --- getValueByDotPath ---

func TestAutomateTelemetryGetValueByDotPath_SimpleKey(t *testing.T) {
	data := map[string]interface{}{"name": "test"}
	val, ok := getValueByDotPath(data, "name")
	assert.True(t, ok)
	assert.Equal(t, "test", val)
}

func TestAutomateTelemetryGetValueByDotPath_NestedKey(t *testing.T) {
	data := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": "deep_value",
		},
	}
	val, ok := getValueByDotPath(data, "level1.level2")
	assert.True(t, ok)
	assert.Equal(t, "deep_value", val)
}

func TestAutomateTelemetryGetValueByDotPath_TrimsPathParts(t *testing.T) {
	data := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": "deep_value",
		},
	}
	val, ok := getValueByDotPath(data, " level1 . level2 ")
	assert.True(t, ok)
	assert.Equal(t, "deep_value", val)
}

func TestAutomateTelemetryGetValueByDotPath_KeyNotFound(t *testing.T) {
	data := map[string]interface{}{"name": "test"}
	_, ok := getValueByDotPath(data, "missing")
	assert.False(t, ok)
}

func TestAutomateTelemetryGetValueByDotPath_NestedKeyNotFound(t *testing.T) {
	data := map[string]interface{}{
		"level1": map[string]interface{}{"level2": "value"},
	}
	_, ok := getValueByDotPath(data, "level1.missing")
	assert.False(t, ok)
}

func TestAutomateTelemetryGetValueByDotPath_EmptyPath(t *testing.T) {
	data := map[string]interface{}{"name": "test"}
	_, ok := getValueByDotPath(data, "")
	assert.False(t, ok)
}

func TestAutomateTelemetryGetValueByDotPath_NonMapIntermediate(t *testing.T) {
	data := map[string]interface{}{
		"level1": "not_a_map",
	}
	_, ok := getValueByDotPath(data, "level1.level2")
	assert.False(t, ok)
}

// --- parseEventActualValue ---

func TestAutomateTelemetryParseEventActualValue_String(t *testing.T) {
	result, err := parseEventActualValue(`{"key":"value"}`)
	assert.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestAutomateTelemetryParseEventActualValue_Bytes(t *testing.T) {
	result, err := parseEventActualValue([]byte(`{"key":"value"}`))
	assert.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestAutomateTelemetryParseEventActualValue_Map(t *testing.T) {
	input := map[string]interface{}{"key": "value"}
	result, err := parseEventActualValue(input)
	assert.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestAutomateTelemetryParseEventActualValue_InvalidString(t *testing.T) {
	_, err := parseEventActualValue("not-json")
	assert.Error(t, err)
}

func TestAutomateTelemetryParseEventActualValue_NullObjectRejected(t *testing.T) {
	_, err := parseEventActualValue("null")
	assert.Error(t, err)
}

func TestAutomateTelemetryParseEventActualValue_OtherType(t *testing.T) {
	_, err := parseEventActualValue(42)
	// int will be marshaled to "42" which is not a map, so error expected
	assert.Error(t, err)
}

// --- AutomateFilter ---

func TestAutomateTelemetryAutomateFilter_TelemetryMatch(t *testing.T) {
	a := &Automate{}
	triggerParamType := model.TRIGGER_PARAM_TYPE_TEL
	triggerParam := "temperature"
	info := initialize.AutomateExecteParams{
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{
				GroupsCondition: initialize.DTConditions{
					{
						TriggerParamType: &triggerParamType,
						TriggerParam:     &triggerParam,
					},
				},
			},
		},
	}
	fromExt := AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_TEL,
		TriggerParam:     []string{"temperature"},
	}
	result := a.AutomateFilter(info, fromExt)
	assert.Len(t, result.AutomateExecteSceeInfos, 1)
}

func TestAutomateTelemetryAutomateFilter_TelemetryNoMatch(t *testing.T) {
	a := &Automate{}
	triggerParamType := model.TRIGGER_PARAM_TYPE_TEL
	triggerParam := "humidity"
	info := initialize.AutomateExecteParams{
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{
				GroupsCondition: initialize.DTConditions{
					{
						TriggerParamType: &triggerParamType,
						TriggerParam:     &triggerParam,
					},
				},
			},
		},
	}
	fromExt := AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_TEL,
		TriggerParam:     []string{"temperature"},
	}
	result := a.AutomateFilter(info, fromExt)
	assert.Len(t, result.AutomateExecteSceeInfos, 0)
}

func TestAutomateTelemetryAutomateFilter_StatusMatch(t *testing.T) {
	a := &Automate{}
	triggerParamType := model.TRIGGER_PARAM_TYPE_STATUS
	info := initialize.AutomateExecteParams{
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{
				GroupsCondition: initialize.DTConditions{
					{
						TriggerParamType: &triggerParamType,
						TriggerParam:     StringPtr("ON-LINE"),
					},
				},
			},
		},
	}
	fromExt := AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_STATUS,
	}
	result := a.AutomateFilter(info, fromExt)
	assert.Len(t, result.AutomateExecteSceeInfos, 1)
}

func TestAutomateTelemetryAutomateFilterPreservesMatchingSceneAndActions(t *testing.T) {
	a := &Automate{}

	targetDevice := "device-telemetry-target"
	targetEvent := "scene-event-target"
	targetAttr := "attribute-target"
	type testcase struct {
		name             string
		fromType         string
		fromParams       []string
		matchType        string
		matchParam       string
		nonMatchType     string
		nonMatchParam    string
		wantSceneID      string
		wantActionTarget *string
	}

	tests := []testcase{
		{
			name:             "telemetry source accepts TELEMETRY alias and preserves action",
			fromType:         model.TRIGGER_PARAM_TYPE_TEL,
			fromParams:       []string{"temperature", "humidity"},
			matchType:        model.TRIGGER_PARAM_TYPE_TELEMETRY,
			matchParam:       "temperature",
			nonMatchType:     model.TRIGGER_PARAM_TYPE_TEL,
			nonMatchParam:    "pressure",
			wantSceneID:      "scene-telemetry-alias",
			wantActionTarget: &targetDevice,
		},
		{
			name:             "event source accepts EVENT alias and preserves action",
			fromType:         model.TRIGGER_PARAM_TYPE_EVT,
			fromParams:       []string{"overheat"},
			matchType:        model.TRIGGER_PARAM_TYPE_EVENT,
			matchParam:       "overheat",
			nonMatchType:     model.TRIGGER_PARAM_TYPE_EVT,
			nonMatchParam:    "door-open",
			wantSceneID:      "scene-event-alias",
			wantActionTarget: &targetEvent,
		},
		{
			name:             "attribute source accepts ATTRIBUTES alias and preserves action",
			fromType:         model.TRIGGER_PARAM_TYPE_ATTR,
			fromParams:       []string{"firmware"},
			matchType:        model.TRIGGER_PARAM_TYPE_ATTRIBUTES,
			matchParam:       "firmware",
			nonMatchType:     model.TRIGGER_PARAM_TYPE_ATTR,
			nonMatchParam:    "model",
			wantSceneID:      "scene-attributes-alias",
			wantActionTarget: &targetAttr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchType := tt.matchType
			matchParam := tt.matchParam
			nonMatchType := tt.nonMatchType
			nonMatchParam := tt.nonMatchParam

			info := initialize.AutomateExecteParams{
				AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
					{
						SceneAutomationId: "wrong-scene",
						GroupsCondition: initialize.DTConditions{
							{
								TriggerParamType: &nonMatchType,
								TriggerParam:     &nonMatchParam,
							},
						},
						Actions: []model.ActionInfo{
							{
								ID:           "wrong-action",
								ActionType:   model.AUTOMATE_ACTION_TYPE_ONE,
								ActionTarget: StringPtr("wrong-target"),
							},
						},
					},
					{
						SceneAutomationId: tt.wantSceneID,
						GroupsCondition: initialize.DTConditions{
							{
								TriggerParamType: &matchType,
								TriggerParam:     &matchParam,
							},
						},
						Actions: []model.ActionInfo{
							{
								ID:           "expected-action",
								ActionType:   model.AUTOMATE_ACTION_TYPE_ONE,
								ActionTarget: tt.wantActionTarget,
							},
						},
					},
				},
			}

			result := a.AutomateFilter(info, AutomateFromExt{
				TriggerParamType: tt.fromType,
				TriggerParam:     tt.fromParams,
			})

			if assert.Len(t, result.AutomateExecteSceeInfos, 1) {
				gotScene := result.AutomateExecteSceeInfos[0]
				assert.Equal(t, tt.wantSceneID, gotScene.SceneAutomationId)
				if assert.Len(t, gotScene.Actions, 1) {
					assert.Equal(t, "expected-action", gotScene.Actions[0].ID)
					assert.Equal(t, tt.wantActionTarget, gotScene.Actions[0].ActionTarget)
				}
			}
		})
	}
}

func TestAutomateTelemetryAutomateFilter_NilTriggerParams(t *testing.T) {
	a := &Automate{}
	info := initialize.AutomateExecteParams{
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{
				GroupsCondition: initialize.DTConditions{
					{
						TriggerParamType: nil,
						TriggerParam:     nil,
					},
				},
			},
		},
	}
	fromExt := AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_TEL,
		TriggerParam:     []string{"temperature"},
	}
	result := a.AutomateFilter(info, fromExt)
	assert.Len(t, result.AutomateExecteSceeInfos, 0)
}

func TestAutomateTelemetryAutomateFilter_EmptyScenes(t *testing.T) {
	a := &Automate{}
	info := initialize.AutomateExecteParams{
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{},
	}
	fromExt := AutomateFromExt{
		TriggerParamType: model.TRIGGER_PARAM_TYPE_TEL,
		TriggerParam:     []string{"temperature"},
	}
	result := a.AutomateFilter(info, fromExt)
	assert.Len(t, result.AutomateExecteSceeInfos, 0)
}

func TestAutomateTelemetryExecuteRun_AttemptsDuplicateFailedSceneOnlyOnce(t *testing.T) {
	originalLimiterAllow := executeRunLimiterAllow
	originalCheckSceneAutomationHasClose := executeRunCheckSceneAutomationHasClose
	originalConditionCheck := executeRunConditionCheck
	originalSceneAutomateExecute := executeRunSceneAutomateExecute
	originalActionAfterDecoration := executeRunActionAfterDecoration
	t.Cleanup(func() {
		executeRunLimiterAllow = originalLimiterAllow
		executeRunCheckSceneAutomationHasClose = originalCheckSceneAutomationHasClose
		executeRunConditionCheck = originalConditionCheck
		executeRunSceneAutomateExecute = originalSceneAutomateExecute
		executeRunActionAfterDecoration = originalActionAfterDecoration
	})

	executeRunLimiterAllow = func(a *Automate, id string) bool {
		return true
	}
	executeRunCheckSceneAutomationHasClose = func(a *Automate, sceneAutomationId string) bool {
		return false
	}
	executeRunConditionCheck = func(a *Automate, conditions initialize.DTConditions, deviceId string) bool {
		return true
	}

	callCount := 0
	executeRunSceneAutomateExecute = func(a *Automate, sceneAutomationId string, deviceIds []string, actions []model.ActionInfo) error {
		callCount++
		return errors.New("scene execution failed")
	}
	executeRunActionAfterDecoration = func(a *Automate, actions []model.ActionInfo, deviceId string, err error) {}

	a := &Automate{}
	info := initialize.AutomateExecteParams{
		DeviceId: "device-1",
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{SceneAutomationId: "scene-1"},
			{SceneAutomationId: "scene-1"},
		},
	}

	err := a.ExecuteRun(info)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scene scene-1 execution failed")
	assert.Equal(t, 1, callCount)
	assert.True(t, a.attemptedSceneIDs["scene-1"])
	assert.False(t, a.executedSceneIDs["scene-1"])
}

func TestAutomateTelemetryExecuteRun_DoesNotConsumeLimiterWhenConditionFails(t *testing.T) {
	originalLimiterAllow := executeRunLimiterAllow
	originalCheckSceneAutomationHasClose := executeRunCheckSceneAutomationHasClose
	originalConditionCheck := executeRunConditionCheck
	originalSceneAutomateExecute := executeRunSceneAutomateExecute
	t.Cleanup(func() {
		executeRunLimiterAllow = originalLimiterAllow
		executeRunCheckSceneAutomationHasClose = originalCheckSceneAutomationHasClose
		executeRunConditionCheck = originalConditionCheck
		executeRunSceneAutomateExecute = originalSceneAutomateExecute
	})

	limiterCalls := 0
	executeRunLimiterAllow = func(a *Automate, id string) bool {
		limiterCalls++
		return true
	}
	executeRunCheckSceneAutomationHasClose = func(a *Automate, sceneAutomationId string) bool {
		return false
	}
	executeRunConditionCheck = func(a *Automate, conditions initialize.DTConditions, deviceId string) bool {
		return false
	}
	executeRunSceneAutomateExecute = func(a *Automate, sceneAutomationId string, deviceIds []string, actions []model.ActionInfo) error {
		t.Fatal("scene should not execute when condition fails")
		return nil
	}

	err := (&Automate{}).ExecuteRun(initialize.AutomateExecteParams{
		DeviceId: "device-1",
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{SceneAutomationId: "scene-1"},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, limiterCalls)
}

func TestAutomateTelemetryExecuteRun_ChecksClosedBeforeLimiter(t *testing.T) {
	originalLimiterAllow := executeRunLimiterAllow
	originalCheckSceneAutomationHasClose := executeRunCheckSceneAutomationHasClose
	originalConditionCheck := executeRunConditionCheck
	t.Cleanup(func() {
		executeRunLimiterAllow = originalLimiterAllow
		executeRunCheckSceneAutomationHasClose = originalCheckSceneAutomationHasClose
		executeRunConditionCheck = originalConditionCheck
	})

	limiterCalls := 0
	conditionCalls := 0
	executeRunLimiterAllow = func(a *Automate, id string) bool {
		limiterCalls++
		return true
	}
	executeRunCheckSceneAutomationHasClose = func(a *Automate, sceneAutomationId string) bool {
		return true
	}
	executeRunConditionCheck = func(a *Automate, conditions initialize.DTConditions, deviceId string) bool {
		conditionCalls++
		return true
	}

	err := (&Automate{}).ExecuteRun(initialize.AutomateExecteParams{
		DeviceId: "device-1",
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{SceneAutomationId: "scene-1"},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, limiterCalls)
	assert.Equal(t, 0, conditionCalls)
}

func TestAutomateTelemetryExecuteRun_ConsumesLimiterOnlyForReadyScene(t *testing.T) {
	originalLimiterAllow := executeRunLimiterAllow
	originalCheckSceneAutomationHasClose := executeRunCheckSceneAutomationHasClose
	originalConditionCheck := executeRunConditionCheck
	originalSceneAutomateExecute := executeRunSceneAutomateExecute
	originalActionAfterDecoration := executeRunActionAfterDecoration
	t.Cleanup(func() {
		executeRunLimiterAllow = originalLimiterAllow
		executeRunCheckSceneAutomationHasClose = originalCheckSceneAutomationHasClose
		executeRunConditionCheck = originalConditionCheck
		executeRunSceneAutomateExecute = originalSceneAutomateExecute
		executeRunActionAfterDecoration = originalActionAfterDecoration
	})

	limiterCalls := 0
	executeRunLimiterAllow = func(a *Automate, id string) bool {
		limiterCalls++
		assert.Equal(t, "scene-1:device-1", id)
		return true
	}
	executeRunCheckSceneAutomationHasClose = func(a *Automate, sceneAutomationId string) bool {
		return false
	}
	executeRunConditionCheck = func(a *Automate, conditions initialize.DTConditions, deviceId string) bool {
		return true
	}
	executeRunSceneAutomateExecute = func(a *Automate, sceneAutomationId string, deviceIds []string, actions []model.ActionInfo) error {
		return nil
	}
	executeRunActionAfterDecoration = func(a *Automate, actions []model.ActionInfo, deviceId string, err error) {}

	a := &Automate{}
	err := a.ExecuteRun(initialize.AutomateExecteParams{
		DeviceId: "device-1",
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{SceneAutomationId: "scene-1"},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, limiterCalls)
	assert.True(t, a.attemptedSceneIDs["scene-1"])
	assert.True(t, a.executedSceneIDs["scene-1"])
}

func TestAutomateTelemetryExecuteRun_SkipsPreviouslyAttemptedSceneBeforeGuards(t *testing.T) {
	originalLimiterAllow := executeRunLimiterAllow
	originalCheckSceneAutomationHasClose := executeRunCheckSceneAutomationHasClose
	originalConditionCheck := executeRunConditionCheck
	t.Cleanup(func() {
		executeRunLimiterAllow = originalLimiterAllow
		executeRunCheckSceneAutomationHasClose = originalCheckSceneAutomationHasClose
		executeRunConditionCheck = originalConditionCheck
	})

	guardCalls := 0
	executeRunLimiterAllow = func(a *Automate, id string) bool {
		guardCalls++
		return true
	}
	executeRunCheckSceneAutomationHasClose = func(a *Automate, sceneAutomationId string) bool {
		guardCalls++
		return false
	}
	executeRunConditionCheck = func(a *Automate, conditions initialize.DTConditions, deviceId string) bool {
		guardCalls++
		return true
	}

	a := &Automate{}
	a.markSceneAttempted("scene-1")

	err := a.ExecuteRun(initialize.AutomateExecteParams{
		DeviceId: "device-1",
		AutomateExecteSceeInfos: []initialize.AutomateExecteSceneInfo{
			{SceneAutomationId: "scene-1"},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, 0, guardCalls)
}

func TestAutomateTelemetryErrorRecoverLogsPanic(t *testing.T) {
	logger := logrus.StandardLogger()
	originalOut := logger.Out
	originalFormatter := logger.Formatter
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
	t.Cleanup(func() {
		logrus.SetOutput(originalOut)
		logrus.SetFormatter(originalFormatter)
	})

	func() {
		defer (&Automate{}).ErrorRecover()
		panic("recover-me")
	}()

	assert.Contains(t, buf.String(), "automation panic recovered")
	assert.Contains(t, buf.String(), "recover-me")
}

// --- automateEventParamConditionCheck ---

func TestAutomateTelemetryEventParamConditionCheck_InvalidJSON(t *testing.T) {
	a := &Automate{}
	ok, _, handled := a.automateEventParamConditionCheck("not-json", map[string]interface{}{})
	assert.False(t, ok)
	assert.False(t, handled)
}

func TestAutomateTelemetryEventParamConditionCheck_NonFieldMode(t *testing.T) {
	a := &Automate{}
	ok, _, handled := a.automateEventParamConditionCheck(`{"match_mode":"regex"}`, map[string]interface{}{})
	assert.False(t, ok)
	assert.False(t, handled)
}

func TestAutomateTelemetryEventParamConditionCheck_FieldModeNoConditions(t *testing.T) {
	a := &Automate{}
	ok, detail, handled := a.automateEventParamConditionCheck(`{"match_mode":"field","conditions":[]}`, map[string]interface{}{})
	assert.False(t, ok)
	assert.True(t, handled)
	assert.Equal(t, "event param match config has no conditions", detail)
}

func TestAutomateTelemetryEventParamConditionCheck_FieldModeWithConditions(t *testing.T) {
	a := &Automate{}
	triggerValue := `{"match_mode":"field","conditions":[{"field":"status","operator":"=","value":"ok"}]}`
	actualValue := map[string]interface{}{"status": "ok"}
	ok, detail, handled := a.automateEventParamConditionCheck(triggerValue, actualValue)
	assert.True(t, ok)
	assert.True(t, handled)
	assert.Equal(t, "event param conditions matched", detail)
}

func TestAutomateTelemetryEventParamConditionCheck_FieldModeConditionNotMet(t *testing.T) {
	a := &Automate{}
	triggerValue := `{"match_mode":"field","conditions":[{"field":"status","operator":"=","value":"ok"}]}`
	actualValue := map[string]interface{}{"status": "error"}
	ok, detail, handled := a.automateEventParamConditionCheck(triggerValue, actualValue)
	assert.False(t, ok)
	assert.True(t, handled)
	assert.Equal(t, "event field [status]: error = ok did not match", detail)
}

func TestAutomateTelemetryEventParamConditionCheck_BetweenNegativeStringRange(t *testing.T) {
	a := &Automate{}
	triggerValue := `{"match_mode":"field","conditions":[{"field":"temperature","operator":"between","value":"-10--1"}]}`
	actualValue := map[string]interface{}{"temperature": -5}
	ok, detail, handled := a.automateEventParamConditionCheck(triggerValue, actualValue)
	assert.True(t, ok)
	assert.True(t, handled)
	assert.NotEmpty(t, detail)
}

func TestAutomateTelemetryEventParamConditionCheck_InvalidActualValue(t *testing.T) {
	a := &Automate{}
	triggerValue := `{"match_mode":"field","conditions":[{"field":"status","operator":"=","value":"ok"}]}`
	ok, _, handled := a.automateEventParamConditionCheck(triggerValue, 42)
	assert.False(t, ok)
	assert.True(t, handled)
}

// --- matchEventParamCondition ---

func TestAutomateTelemetryMatchEventParamCondition_EmptyField(t *testing.T) {
	a := &Automate{}
	ok, detail := a.matchEventParamCondition(map[string]interface{}{}, eventParamCondition{Field: "", Operator: "=", Value: "test"})
	assert.False(t, ok)
	assert.Equal(t, "event condition field is required", detail)
}

func TestAutomateTelemetryMatchEventParamCondition_ExistsOperator(t *testing.T) {
	a := &Automate{}
	data := map[string]interface{}{
		"status": "ok",
		"nested": map[string]interface{}{
			"state": "ready",
		},
	}

	tests := []struct {
		name       string
		condition  eventParamCondition
		wantOK     bool
		wantDetail string
	}{
		{
			name:       "nil value defaults to expected true when field exists",
			condition:  eventParamCondition{Field: "status", Operator: "exists"},
			wantOK:     true,
			wantDetail: "event field [status] exists = true",
		},
		{
			name:       "explicit true matches existing field",
			condition:  eventParamCondition{Field: "status", Operator: "exists", Value: true},
			wantOK:     true,
			wantDetail: "event field [status] exists = true",
		},
		{
			name:       "explicit true rejects missing field",
			condition:  eventParamCondition{Field: "missing", Operator: "exists", Value: true},
			wantOK:     false,
			wantDetail: "event field [missing] exists = true, actual = false",
		},
		{
			name:       "explicit false matches missing field",
			condition:  eventParamCondition{Field: "missing", Operator: "exists", Value: false},
			wantOK:     true,
			wantDetail: "event field [missing] exists = false",
		},
		{
			name:       "explicit false rejects existing field",
			condition:  eventParamCondition{Field: "status", Operator: "exists", Value: false},
			wantOK:     false,
			wantDetail: "event field [status] exists = false, actual = true",
		},
		{
			name:       "nested path exists",
			condition:  eventParamCondition{Field: " nested . state ", Operator: "exists", Value: true},
			wantOK:     true,
			wantDetail: "event field [nested . state] exists = true",
		},
		{
			name:       "nested path missing with false expectation",
			condition:  eventParamCondition{Field: "nested.missing", Operator: "exists", Value: false},
			wantOK:     true,
			wantDetail: "event field [nested.missing] exists = false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, detail := a.matchEventParamCondition(data, tt.condition)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantDetail, detail)
		})
	}
}

func TestAutomateTelemetryMatchEventParamCondition_FieldMissing(t *testing.T) {
	a := &Automate{}
	data := map[string]interface{}{}
	ok, detail := a.matchEventParamCondition(data, eventParamCondition{Field: "status", Operator: "=", Value: "ok"})
	assert.False(t, ok)
	assert.Equal(t, "event field [status] not found", detail)
}

// --- matchEventParamValue ---

func TestAutomateTelemetryMatchEventParamValue_EQ(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.matchEventParamValue("=", "ok", "ok"))
	assert.False(t, a.matchEventParamValue("=", "ok", "error"))
}

func TestAutomateTelemetryMatchEventParamValue_NEQ(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.matchEventParamValue("!=", "ok", "error"))
	assert.False(t, a.matchEventParamValue("!=", "ok", "ok"))
}

func TestAutomateTelemetryMatchEventParamValue_GT(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.matchEventParamValue(">", 5, 10))
	assert.False(t, a.matchEventParamValue(">", 10, 5))
}

func TestAutomateTelemetryMatchEventParamValue_BETWEEN(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.matchEventParamValue("between", []interface{}{10, 20}, 15))
	assert.True(t, a.matchEventParamValue("between", []interface{}{10, 20}, 10))
	assert.True(t, a.matchEventParamValue("between", []interface{}{10, 20}, 20))
	assert.False(t, a.matchEventParamValue("between", []interface{}{10, 20}, 25))
}

func TestAutomateTelemetryMatchEventParamValue_IN(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.matchEventParamValue("in", []interface{}{"a", "b", "c"}, "b"))
	assert.False(t, a.matchEventParamValue("in", []interface{}{"a", "b", "c"}, "d"))
	assert.True(t, a.matchEventParamValue("in", "a,b,c", "b"))
	assert.False(t, a.matchEventParamValue("in", "a,b,c", "d"))
}

func TestAutomateTelemetryMatchEventParamValue_ArrayActual(t *testing.T) {
	a := &Automate{}
	assert.True(t, a.matchEventParamValue("=", "ok", []interface{}{"error", "ok"}))
	assert.False(t, a.matchEventParamValue("=", "ok", []interface{}{"error", "fail"}))
	assert.False(t, a.matchEventParamValue("!=", "ok", []interface{}{"ok", "error"}))
	assert.True(t, a.matchEventParamValue("!=", "ok", []interface{}{"error", "warn"}))
	assert.True(t, a.matchEventParamValue("=", 10, []interface{}{"5", "10"}))
	assert.True(t, a.matchEventParamValue(">", 10, []interface{}{5, 11}))
	assert.True(t, a.matchEventParamValue("in", []interface{}{"a", "b"}, []interface{}{"c", "b"}))
	assert.True(t, a.matchEventParamValue("=", true, []interface{}{"false", "true"}))
	assert.False(t, a.matchEventParamValue("=", true, []interface{}{"false", "not-a-bool"}))
}

func TestAutomateTelemetryMatchEventParamValue_UnknownOperator(t *testing.T) {
	a := &Automate{}
	assert.False(t, a.matchEventParamValue("???", "ok", "ok"))
}

// --- AutomateConditionCheckWithGroupOne ---

func TestAutomateTelemetryConditionCheckWithGroupOne_TimeType(t *testing.T) {
	a := &Automate{}
	cond := model.DeviceTriggerCondition{
		TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_TIME,
		TriggerValue:         "", // empty trigger value returns false
	}
	ok, content := a.AutomateConditionCheckWithGroupOne(cond, "device1")
	assert.False(t, ok)
	assert.Equal(t, "", content)
}

func TestAutomateTelemetryConditionCheckWithGroupOne_DefaultType(t *testing.T) {
	a := &Automate{}
	cond := model.DeviceTriggerCondition{
		TriggerConditionType: "99", // unknown type returns true
	}
	ok, content := a.AutomateConditionCheckWithGroupOne(cond, "device1")
	assert.True(t, ok)
	assert.Equal(t, "", content)
}

func TestAutomateExecuteRejectsNilDevice(t *testing.T) {
	a := &Automate{}
	err := a.Execute(nil, AutomateFromExt{})
	assert.EqualError(t, err, "device info is required")
}

func TestAutomateConditionCheckWithDeviceRejectsMissingTriggerPointers(t *testing.T) {
	a := &Automate{device: &model.Device{Name: StringPtr("device-1")}}
	source := "device-1"

	ok, detail := a.automateConditionCheckWithDevice(model.DeviceTriggerCondition{
		TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE,
		TriggerSource:        &source,
	}, "device-1")

	assert.False(t, ok)
	assert.Equal(t, "", detail)
}

func TestAutomateConditionCheckWithDeviceRejectsNonStringStatusValue(t *testing.T) {
	a := &Automate{
		device: &model.Device{Name: StringPtr("device-1")},
		formExt: AutomateFromExt{
			TriggerValues: map[string]interface{}{"login": 1},
		},
	}
	source := "device-1"
	triggerType := model.TRIGGER_PARAM_TYPE_STATUS
	triggerParam := "ON-LINE"

	ok, detail := a.automateConditionCheckWithDevice(model.DeviceTriggerCondition{
		TriggerConditionType: model.DEVICE_TRIGGER_CONDITION_TYPE_MULTIPLE,
		TriggerSource:        &source,
		TriggerParamType:     &triggerType,
		TriggerParam:         &triggerParam,
	}, "device-1")

	assert.False(t, ok)
	assert.Contains(t, detail, "status value is not a string")
}

func TestAutomateActionExecuteRejectsMissingAndUnsupportedActions(t *testing.T) {
	a := &Automate{}

	result, err := a.AutomateActionExecute("scene-empty-actions", []string{"device-1"}, nil, "tenant-1")
	assert.EqualError(t, err, "automate action list is empty")
	assert.Equal(t, "automate action list is empty", result)

	result, err = a.AutomateActionExecute("scene-unsupported-action", []string{"device-1"}, []model.ActionInfo{
		{
			ID:         "unsupported-action",
			ActionType: "unsupported",
		},
	}, "tenant-1")
	assert.EqualError(t, err, "unsupported automate action")
	assert.Equal(t, "unsupported automate action", result)
}

func TestAppendAutomateActionResult(t *testing.T) {
	assert.Equal(t, "action-1 execute success;", appendAutomateActionResult("", "action-1", nil))
	assert.Equal(t, "existing;action-2 execute failed;", appendAutomateActionResult("existing;", "action-2", errors.New("failed")))
}

func TestActiveSceneExecutePropagatesActionLookupError(t *testing.T) {
	expected := errors.New("action lookup failed")
	original := getActionInfoListBySceneID
	getActionInfoListBySceneID = func(sceneIDs []string) ([]model.ActionInfo, error) {
		assert.Equal(t, []string{"scene-lookup-error"}, sceneIDs)
		return nil, expected
	}
	t.Cleanup(func() {
		getActionInfoListBySceneID = original
	})

	err := (&Automate{}).ActiveSceneExecute("scene-lookup-error", "tenant-1")

	assert.ErrorIs(t, err, expected)
}
