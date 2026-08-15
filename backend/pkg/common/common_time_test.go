// 文件用途：验证 common 包的字符串、JSON、错误、随机标识、角色判断和时间计算工具。
// 核心逻辑：用表驱动和固定时间输入检查公共帮助函数的格式、边界和错误分支。
// 关键注意事项：随机函数只验证格式和范围，不验证具体随机性；时间测试依赖本地时区构造。
// 重构建议：后续可把时间调度测试拆到独立文件，并为更多场景表达式增加边界样例。
package common

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	constant "aetherlink-iot/backend/pkg/constant"
)

func TestCommonStringJsonAndErrorHelpers(t *testing.T) {
	if !CheckEmpty(constant.EMPTY) {
		t.Fatal("CheckEmpty should return true for constant.EMPTY")
	}
	if CheckEmpty("not-empty") {
		t.Fatal("CheckEmpty should return false for non-empty value")
	}

	raw, err := JsonToString(map[string]any{"device_id": "dev-1", "value": 12})
	if err != nil {
		t.Fatalf("JsonToString returned error: %v", err)
	}
	if !strings.Contains(raw, `"device_id":"dev-1"`) || !strings.Contains(raw, `"value":12`) {
		t.Fatalf("JsonToString = %s, want JSON object fields", raw)
	}
	if _, err := JsonToString(map[string]any{"bad": func() {}}); err == nil {
		t.Fatal("JsonToString expected error for unsupported value")
	}

	baseErr := errors.New("database failed")
	wrapped := GetErrors(baseErr, "load device")
	if !errors.Is(wrapped, baseErr) || !strings.Contains(wrapped.Error(), "load device") {
		t.Fatalf("GetErrors = %v, want wrapped base error with message", wrapped)
	}

	ptr := StringSpt("tenant-1")
	if ptr == nil || *ptr != "tenant-1" {
		t.Fatalf("StringSpt returned %#v", ptr)
	}
	if IsStringEmpty(ptr) {
		t.Fatal("IsStringEmpty should be false for non-empty pointer")
	}
	if !IsStringEmpty(nil) || !IsStringEmpty(StringSpt("")) {
		t.Fatal("IsStringEmpty should be true for nil and empty string")
	}
}

func TestGetResponsePayloadForSuccessAndFailure(t *testing.T) {
	success := GetResponsePayload("property.post", nil)
	var successPayload map[string]any
	if err := json.Unmarshal(success, &successPayload); err != nil {
		t.Fatalf("success payload JSON error: %v", err)
	}
	if successPayload["result"].(float64) != 0 || successPayload["message"] != "success" || successPayload["method"] != "property.post" {
		t.Fatalf("success payload = %#v", successPayload)
	}
	if successPayload["ts"].(float64) <= 0 {
		t.Fatalf("success ts = %#v, want positive", successPayload["ts"])
	}

	failure := GetResponsePayload("", errors.New("bad command"))
	var failurePayload map[string]any
	if err := json.Unmarshal(failure, &failurePayload); err != nil {
		t.Fatalf("failure payload JSON error: %v", err)
	}
	if failurePayload["result"].(float64) != 1 || failurePayload["errcode"] != "000" || failurePayload["message"] != "bad command" {
		t.Fatalf("failure payload = %#v", failurePayload)
	}
	if _, ok := failurePayload["method"]; ok {
		t.Fatalf("failure payload should not include empty method: %#v", failurePayload)
	}
}

func TestRandomIdentifiersAndCodesRespectBusinessFormats(t *testing.T) {
	messageID := GetMessageID()
	if matched := regexp.MustCompile(`^\d{7}$`).MatchString(messageID); !matched {
		t.Fatalf("GetMessageID = %q, want seven digits", messageID)
	}

	randomText, err := GenerateRandomString(24)
	if err != nil {
		t.Fatalf("GenerateRandomString returned error: %v", err)
	}
	if len(randomText) != 24 {
		t.Fatalf("GenerateRandomString length = %d, want 24", len(randomText))
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(randomText) {
		t.Fatalf("GenerateRandomString = %q, want alphanumeric", randomText)
	}

	nineDigits, err := GetRandomNineDigits()
	if err != nil {
		t.Fatalf("GetRandomNineDigits returned error: %v", err)
	}
	if !regexp.MustCompile(`^\d{9}$`).MatchString(nineDigits) {
		t.Fatalf("GetRandomNineDigits = %q, want nine digits", nineDigits)
	}
	parsedNineDigits, err := strconv.Atoi(nineDigits)
	if err != nil || parsedNineDigits < 100000000 || parsedNineDigits > 999999999 {
		t.Fatalf("GetRandomNineDigits = %q, want numeric range", nineDigits)
	}

	code, err := GenerateNumericCode(6)
	if err != nil {
		t.Fatalf("GenerateNumericCode returned error: %v", err)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
		t.Fatalf("GenerateNumericCode = %q, want six digits", code)
	}
	if _, err := GenerateNumericCode(0); err == nil {
		t.Fatal("GenerateNumericCode expected error for zero length")
	}
}

func TestAdminAuthorityCheck(t *testing.T) {
	if !CheckUserIsAdmin(constant.SYS_ADMIN) {
		t.Fatal("CheckUserIsAdmin should accept SYS_ADMIN")
	}
	if CheckUserIsAdmin("TENANT_ADMIN") {
		t.Fatal("CheckUserIsAdmin should reject non SYS_ADMIN authority")
	}
}

func TestDateHelpersAndWeekdayMapping(t *testing.T) {
	today := GetToday()
	if today.Hour() != 0 || today.Minute() != 0 || today.Second() != 0 || today.Nanosecond() != 0 {
		t.Fatalf("GetToday = %s, want local midnight", today)
	}
	if GetMonthStart().Day() != 1 {
		t.Fatalf("GetMonthStart day = %d, want 1", GetMonthStart().Day())
	}
	if GetYearStart().Month() != time.January || GetYearStart().Day() != 1 {
		t.Fatalf("GetYearStart = %s, want Jan 1", GetYearStart())
	}
	if !GetYesterdayBegin().Equal(GetToday().Add(-1)) {
		t.Fatalf("GetYesterdayBegin = %s, want yesterday midnight minus one nanosecond day behavior", GetYesterdayBegin())
	}

	date := time.Date(2026, 6, 27, 9, 8, 7, 0, time.Local)
	if got := DateTimeToString(date, ""); got != "2026-06-27 09:08:07" {
		t.Fatalf("DateTimeToString default = %q", got)
	}
	if got := DateTimeToString(date, "2006/01/02"); got != "2026/06/27" {
		t.Fatalf("DateTimeToString custom = %q", got)
	}

	weekdays := map[time.Weekday]int{
		time.Monday:    1,
		time.Tuesday:   2,
		time.Wednesday: 3,
		time.Thursday:  4,
		time.Friday:    5,
		time.Saturday:  6,
		time.Sunday:    7,
	}
	for weekday, want := range weekdays {
		if got := GetWeekDay(time.Date(2026, 6, 22+int(weekday-time.Monday+7)%7, 0, 0, 0, 0, time.UTC)); got != want {
			t.Fatalf("GetWeekDay(%s) = %d, want %d", weekday, got, want)
		}
	}
}

func TestNextTimeCalculations(t *testing.T) {
	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.Local)
	target := time.Date(0, 1, 1, 11, 30, 0, 0, time.Local)
	if got := GetNextTime(now, []time.Weekday{time.Saturday}, target); !got.Equal(time.Date(2026, 6, 27, 11, 30, 0, 0, time.Local)) {
		t.Fatalf("GetNextTime same day future = %s", got)
	}
	if got := GetNextTime(now, []time.Weekday{time.Saturday}, time.Date(0, 1, 1, 9, 30, 0, 0, time.Local)); !got.Equal(time.Date(2026, 7, 4, 9, 30, 0, 0, time.Local)) {
		t.Fatalf("GetNextTime next week = %s", got)
	}
	if got := GetNextTime(now, nil, target); !got.IsZero() {
		t.Fatalf("GetNextTime without weekdays = %s, want zero", got)
	}

	if got := getMonthNextTime(now, time.Date(0, 1, 15, 8, 0, 0, 0, time.Local)); !got.Equal(time.Date(2026, 7, 15, 8, 0, 0, 0, time.Local)) {
		t.Fatalf("getMonthNextTime next month = %s", got)
	}
	if got := getMonthNextTime(now, time.Date(0, 1, 28, 8, 0, 0, 0, time.Local)); !got.Equal(time.Date(2026, 6, 28, 8, 0, 0, 0, time.Local)) {
		t.Fatalf("getMonthNextTime same month = %s", got)
	}
}

func TestGetSceneExecuteTimeRejectsInvalidSchedulesAndReturnsFutureForCron(t *testing.T) {
	invalid := []struct {
		taskType  string
		condition string
	}{
		{taskType: "HOUR", condition: "99"},
		{taskType: "DAY", condition: "bad"},
		{taskType: "WEEK", condition: "123"},
		{taskType: "MONTH", condition: "bad"},
		{taskType: "CRON", condition: "bad"},
		{taskType: "UNKNOWN", condition: ""},
	}
	for _, tt := range invalid {
		if _, err := GetSceneExecuteTime(tt.taskType, tt.condition); err == nil {
			t.Fatalf("GetSceneExecuteTime(%q,%q) expected error", tt.taskType, tt.condition)
		}
	}

	next, err := GetSceneExecuteTime("CRON", "0 */5 * * * ?")
	if err != nil {
		t.Fatalf("GetSceneExecuteTime cron returned error: %v", err)
	}
	if !next.After(time.Now()) {
		t.Fatalf("GetSceneExecuteTime cron = %s, want future time", next)
	}
}
