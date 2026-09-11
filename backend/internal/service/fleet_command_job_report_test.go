package service

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/utils"
)

func reportTime(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return &parsed
}

func reportString(value string) *string { return &value }

func parseReportCSV(t *testing.T, content string) [][]string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(content)).ReadAll()
	if err != nil {
		t.Fatalf("report CSV is not parseable: %v", err)
	}
	return records
}

// NULL 进度导出成 0 是最危险的静默错误：它给"从未上报"的设备凭空记一笔
// "已开始 0%"，让运维以为整批都在推进。这里钉住 NULL 必须导出为空。
func TestFormatFleetCommandJobReportCSVKeepsNullDistinctFromZero(t *testing.T) {
	zero := 0
	fifty := 50
	content := FormatFleetCommandJobReportCSV([]dal.CommandJobReportRow{
		{DeviceID: "device-null", ProgressPercent: nil},
		{DeviceID: "device-zero", ProgressPercent: &zero},
		{DeviceID: "device-half", ProgressPercent: &fifty},
	})
	records := parseReportCSV(t, content)
	if len(records) != 4 {
		t.Fatalf("CSV rows = %d, want 4 (header + 3)", len(records))
	}
	percent := 6
	cases := []struct {
		row     int
		device  string
		want    string
		message string
	}{
		{row: 1, device: "device-null", want: "", message: "NULL progress must export as empty, not 0"},
		{row: 2, device: "device-zero", want: "0", message: "a real 0% must export as 0"},
		{row: 3, device: "device-half", want: "50", message: "50% must export as 50"},
	}
	for _, testCase := range cases {
		if got := records[testCase.row][percent]; got != testCase.want {
			t.Fatalf("%s: got %q, want %q", testCase.message, got, testCase.want)
		}
		if got := records[testCase.row][0]; got != testCase.device {
			t.Fatalf("row %d device = %q, want %q", testCase.row, got, testCase.device)
		}
	}
}

// 空结果也必须给出带表头的空报表：空文件无法区分"没数据"和"导出坏了"。
func TestFormatFleetCommandJobReportCSVAlwaysEmitsHeader(t *testing.T) {
	records := parseReportCSV(t, FormatFleetCommandJobReportCSV(nil))
	if len(records) != 1 {
		t.Fatalf("empty report records = %d, want 1 (header only)", len(records))
	}
	if records[0][0] != "device_id" {
		t.Fatalf("header[0] = %q, want device_id", records[0][0])
	}
}

// CSV 公式注入：以 = + - @ 开头的字段会被 Excel/Sheets 当公式执行。
// 设备名与错误信息是设备侧可控内容，直接导出等于给了注入点。
func TestFormatFleetCommandJobReportCSVNeutralizesFormulaInjection(t *testing.T) {
	content := FormatFleetCommandJobReportCSV([]dal.CommandJobReportRow{
		{DeviceID: "d1", Name: "=cmd|'/c calc'!A1", Reason: reportString("+1+1"), ProgressError: reportString("@SUM(A1)")},
	})
	records := parseReportCSV(t, content)
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	body := records[1]
	// 值不能丢（去掉前置引号即可还原），但绝不能以公式触发符开头。
	for _, index := range []int{2, 8, 13} {
		cell := body[index]
		if cell == "" {
			continue
		}
		if cell[0] == '=' || cell[0] == '+' || cell[0] == '-' || cell[0] == '@' {
			t.Fatalf("column %d exported an executable formula cell: %q", index, cell)
		}
	}
	if !strings.Contains(body[2], "=cmd") {
		t.Fatalf("value should be preserved (recoverable), got %q", body[2])
	}
	if !strings.HasPrefix(body[2], "'") {
		t.Fatalf("formula cell must be prefixed with a quote, got %q", body[2])
	}
}

// device_id 不能加引号：运维靠它做关联，加了会让 vlookup 全线失效。
func TestFormatFleetCommandJobReportCSVLeavesDeviceIDUnquoted(t *testing.T) {
	content := FormatFleetCommandJobReportCSV([]dal.CommandJobReportRow{{DeviceID: "device-42"}})
	records := parseReportCSV(t, content)
	if got := records[1][0]; got != "device-42" {
		t.Fatalf("device_id = %q, want device-42 (must stay joinable)", got)
	}
}

// 负数前缀的消毒不能误伤真正的数值列：progress_percent 是 0-100 的整数，
// 正常不会以 '-' 开头，但即便写成 -5 也应当原样导出（数值列不走消毒）。
func TestFormatFleetCommandJobReportCSVSanitizesOnlyTextColumns(t *testing.T) {
	content := FormatFleetCommandJobReportCSV([]dal.CommandJobReportRow{
		{DeviceID: "d1", Name: "-suspicious", Reason: reportString("-also")},
	})
	records := parseReportCSV(t, content)
	if !strings.HasPrefix(records[1][2], "'") {
		t.Fatalf("text column starting with '-' must be sanitized, got %q", records[1][2])
	}
	if !strings.HasPrefix(records[1][13], "'") {
		t.Fatalf("reason starting with '-' must be sanitized, got %q", records[1][13])
	}
}

// 时间戳统一 UTC + RFC3339，否则同一作业两次导出的时间列无法逐行比对。
func TestFormatFleetCommandJobReportCSVNormalizesTimestamps(t *testing.T) {
	stamp := reportTime("2026-09-11T10:00:00+08:00")
	content := FormatFleetCommandJobReportCSV([]dal.CommandJobReportRow{{DeviceID: "d1", ProgressAt: stamp}})
	records := parseReportCSV(t, content)
	if got := records[1][9]; got != "2026-09-11T02:00:00Z" {
		t.Fatalf("progress_at = %q, want UTC RFC3339 2026-09-11T02:00:00Z", got)
	}
}

func TestGetFleetCommandJobReportRejectsMissingTenantAndBadFormat(t *testing.T) {
	automate := &CommandData{}
	if _, err := automate.GetFleetCommandJobReport("job-1", "csv", 0, nil); err == nil {
		t.Fatal("missing tenant must be rejected; otherwise the export would read across tenants")
	}
	if _, err := automate.GetFleetCommandJobReport("job-1", "csv", 0, &utils.UserClaims{}); err == nil {
		t.Fatal("empty tenant id must be rejected")
	}
	claims := &utils.UserClaims{TenantID: "tenant-hq"}
	if _, err := automate.GetFleetCommandJobReport("", "csv", 0, claims); err == nil {
		t.Fatal("missing job id must be rejected")
	}
	if _, err := automate.GetFleetCommandJobReport("job-1", "xlsx", 0, claims); err == nil {
		t.Fatal("unsupported format must be rejected")
	}
}
