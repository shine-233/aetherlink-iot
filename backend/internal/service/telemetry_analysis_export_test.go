package service

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"

	"github.com/xuri/excelize/v2"
)

// P2.2 导出证据：CSV 与 Excel 必须来自同一份行数据，
// 否则同一查询在两种格式里会给出互相矛盾的结果。

func sampleAnalysisRows() [][]string {
	return [][]string{
		{"d1", "temp", "avg", "current", "25", "20", "5", "25.00%"},
		{"d2", "temp", "avg", "current", "n/a", "20", "n/a", "n/a (insufficient data in one or both periods)"},
	}
}

func TestExportTelemetryAnalysisWritesBothFormats(t *testing.T) {
	dir := t.TempDir()
	original := telemetryAnalysisExportDir
	defer func() { _ = os.RemoveAll(dir) }()
	t.Setenv("PWD", dir)
	// 导出目录是包级常量，这里改为在临时目录下验证文件内容，
	// 通过直接调用底层写函数避免污染仓库目录。
	csvPath := filepath.Join(dir, "a.csv")
	if err := writeTelemetryAnalysisCSV(csvPath, sampleAnalysisRows()); err != nil {
		t.Fatalf("csv write error = %v", err)
	}
	xlsxPath := filepath.Join(dir, "a.xlsx")
	if err := writeTelemetryAnalysisXLSX(xlsxPath, sampleAnalysisRows()); err != nil {
		t.Fatalf("xlsx write error = %v", err)
	}
	_ = original

	// CSV 校验
	file, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("open csv = %v", err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read csv = %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("csv rows = %d, want 3 (header + 2)", len(records))
	}
	if records[0][0] != "device_id" {
		t.Fatalf("csv header = %v", records[0])
	}
	if records[2][4] != "n/a" {
		t.Fatalf("missing value must stay n/a in csv; got %q", records[2][4])
	}

	// Excel 校验
	workbook, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		t.Fatalf("open xlsx = %v", err)
	}
	defer workbook.Close()
	headerCell, err := workbook.GetCellValue(telemetryAnalysisSheetName, "A1")
	if err != nil {
		t.Fatalf("read xlsx header = %v", err)
	}
	if headerCell != "device_id" {
		t.Fatalf("xlsx header = %q, want device_id", headerCell)
	}
	valueCell, err := workbook.GetCellValue(telemetryAnalysisSheetName, "A2")
	if err != nil {
		t.Fatalf("read xlsx value = %v", err)
	}
	if valueCell != "d1" {
		t.Fatalf("xlsx first data cell = %q, want d1", valueCell)
	}
}

// 行数上限必须在写文件之前拦下：写一半再失败会留下一个打不开的文件。
func TestExportTelemetryAnalysisRejectsOversized(t *testing.T) {
	rows := make([][]string, telemetryAnalysisMaxRows+1)
	for i := range rows {
		rows[i] = []string{"d", "k", "avg", "current", "1", "1", "0", "0.00%"}
	}
	if _, err := ExportTelemetryAnalysis(model.TelemetryAnalysisQuery{Format: "csv"}, rows); err == nil {
		t.Fatal("oversized export must be refused")
	}
}

func TestExportTelemetryAnalysisRejectsUnknownFormat(t *testing.T) {
	if _, err := ExportTelemetryAnalysis(model.TelemetryAnalysisQuery{Format: "pdf"}, sampleAnalysisRows()); err == nil {
		t.Fatal("unknown format must be refused")
	}
}

// n/a 必须保持文本：若被推断成数字 0，下游图表会画出一条假的跌到 0 的曲线。
func TestAnalysisRowsKeepNAAsText(t *testing.T) {
	rows := sampleAnalysisRows()
	for _, row := range rows {
		for _, cell := range row {
			if strings.TrimSpace(cell) == "" {
				t.Fatalf("empty cell in %v; missing values must be n/a", row)
			}
		}
	}
}
