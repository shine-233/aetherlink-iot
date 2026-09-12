// File purpose: P2.2 analytics export (CSV / XLSX).
// Core logic: both formats render from the SAME row builder, so the two files can never
// disagree about the same query result.
// Key notes:
//   - Streaming writer for xlsx (like the pre-register export) so a 200k-row export does not
//     hold the whole workbook in memory.
//   - Row cap is enforced before writing. A half-written file is worse than a refusal.
//   - "n/a" cells stay textual in both formats; coercing them to 0 would draw a fake drop to
//     zero on any chart built from the export.

package service

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/xuri/excelize/v2"
)

const (
	telemetryAnalysisExportDir  = "./files/excel/"
	telemetryAnalysisMaxRows    = 200000
	telemetryAnalysisSheetName  = "Sheet1"
)

// TelemetryAnalysisExportResult 导出结果。
type TelemetryAnalysisExportResult struct {
	FilePath string `json:"file_path"`
	Format   string `json:"format"`
	Rows     int    `json:"rows"`
}

// ExportTelemetryAnalysis 按格式导出分析结果。
func ExportTelemetryAnalysis(q model.TelemetryAnalysisQuery, rows [][]string) (*TelemetryAnalysisExportResult, error) {
	if len(rows) > telemetryAnalysisMaxRows {
		return nil, errcode.NewWithMessage(
			errcode.CodeParamError,
			fmt.Sprintf("telemetry analysis export is limited to %d rows; got %d", telemetryAnalysisMaxRows, len(rows)),
		)
	}
	format := strings.ToLower(strings.TrimSpace(q.Format))
	if format == "" {
		format = model.TelemetryAnalysisFormatXLSX
	}
	if format != model.TelemetryAnalysisFormatXLSX && format != model.TelemetryAnalysisFormatCSV {
		return nil, errcode.NewWithMessage(errcode.CodeParamError,
			fmt.Sprintf("unsupported export format %q; use csv or xlsx", q.Format))
	}
	if err := os.MkdirAll(telemetryAnalysisExportDir, os.ModePerm); err != nil {
		return nil, err
	}
	fileName := telemetryAnalysisExportFileName(format)
	filePath := filepath.Join(telemetryAnalysisExportDir, fileName)

	var err error
	if format == model.TelemetryAnalysisFormatCSV {
		err = writeTelemetryAnalysisCSV(filePath, rows)
	} else {
		err = writeTelemetryAnalysisXLSX(filePath, rows)
	}
	if err != nil {
		return nil, err
	}
	return &TelemetryAnalysisExportResult{FilePath: filePath, Format: format, Rows: len(rows)}, nil
}

func telemetryAnalysisExportFileName(format string) string {
	stamp := time.Now().UTC().Format("20060102150405")
	return fmt.Sprintf("telemetry_analysis_%s.%s", stamp, format)
}

// writeTelemetryAnalysisCSV 写 CSV。表头与 Excel 完全一致。
func writeTelemetryAnalysisCSV(filePath string, rows [][]string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.Write(telemetryAnalysisColumns()); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

// writeTelemetryAnalysisXLSX 用流式写入，避免整份工作簿驻留内存。
// 单元格一律按字符串写入：数值列里混着 "n/a"，若让 excelize 自行推断类型，
// 会把 "n/a" 与数字分成两种类型，筛选与图表都会出错。
func writeTelemetryAnalysisXLSX(filePath string, rows [][]string) error {
	file := excelize.NewFile()
	streamWriter, err := file.NewStreamWriter(telemetryAnalysisSheetName)
	if err != nil {
		return err
	}
	header := make([]interface{}, 0, len(telemetryAnalysisColumns()))
	for _, column := range telemetryAnalysisColumns() {
		header = append(header, column)
	}
	if err := streamWriter.SetRow("A1", header); err != nil {
		return err
	}
	for index, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, index+2)
		if err != nil {
			return err
		}
		values := make([]interface{}, 0, len(row))
		for _, value := range row {
			values = append(values, value)
		}
		if err := streamWriter.SetRow(cell, values); err != nil {
			return err
		}
	}
	if err := streamWriter.Flush(); err != nil {
		return err
	}
	return file.SaveAs(filePath)
}
