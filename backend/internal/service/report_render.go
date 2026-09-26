// 报表渲染器（TB-49）：把规范化的遥测行渲染为 HTML 表格或简单 PDF 表格。
// 核心逻辑：HTML 走 html/template 内嵌模板（自动 HTML 转义）；PDF 用 go-pdf/fpdf 核心字体逐行写单元格。
// 关键注意事项：PDF 核心字体仅支持 Latin-1，超集字符统一降级为 '?'；两种渲染输出都受 reportMaxBytes 上限约束。
// 重构建议：后续如需富样式/图表嵌入，将渲染器改为按 format 注册的渲染器表并注入 Unicode 字体资源。

package service

import (
	"errors"
	"html/template"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

const (
	reportFallbackTitle = "Report"
	reportPDFUnit       = "mm"
	reportPDFMargin     = 12.0
	reportPDFRowHeight  = 6.0
)

// reportPDFColumnWidths 按 A4 可打印宽度（210-2*12=186mm）分配四列。
var reportPDFColumnWidths = [4]float64{42, 45, 35, 64}

// reportHTMLTemplateSource 内嵌 HTML 表格模板；表头与 CSV 表头口径一致，
// 数据行统一为 <tr><td>…</td></tr> 形态，html/template 负责全部上下文转义。
const reportHTMLTemplateSource = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>{{.Title}}</title></head>
<body>
<h1>{{.Title}}</h1>
<p>{{.WindowStart}} &ndash; {{.WindowEnd}} UTC</p>
<table>
<thead><tr><th>timestamp</th><th>device_id</th><th>key</th><th>value</th></tr></thead>
<tbody>
{{- range .Rows}}
<tr><td>{{.Timestamp}}</td><td>{{.DeviceID}}</td><td>{{.Key}}</td><td>{{.Value}}</td></tr>
{{- end}}
</tbody>
</table>
</body>
</html>
`

// reportHTMLData is the template data root. Rows carry pre-formatted strings so
// the template itself stays presentation-only.
type reportHTMLData struct {
	Title       string
	WindowStart string
	WindowEnd   string
	Rows        []reportTableRow
}

var reportHTMLTemplate = template.Must(template.New("report_html").Parse(reportHTMLTemplateSource))

// renderReportHTML renders the whole artifact in one pass through the bounded
// buffer, so a template blow-up maps to byte_limit_exceeded instead of OOM.
func renderReportHTML(rows []reportTableRow, title string, windowStart, windowEnd time.Time) ([]byte, error) {
	data := reportHTMLData{
		Title:       reportArtifactTitle(title),
		WindowStart: windowStart.UTC().Format(time.RFC3339),
		WindowEnd:   windowEnd.UTC().Format(time.RFC3339),
		Rows:        rows,
	}
	buffer := &reportLimitedBuffer{limit: reportMaxBytes}
	if err := reportHTMLTemplate.Execute(buffer, data); err != nil {
		return nil, err
	}
	return append([]byte(nil), buffer.Bytes()...), nil
}

// renderReportPDF writes a minimal bordered table row by row. Core fonts only
// cover Latin-1, so every cell passes through sanitizeReportPDFText first; the
// final document is streamed through the bounded buffer to keep the byte cap.
func renderReportPDF(rows []reportTableRow, title string, windowStart, windowEnd time.Time) ([]byte, error) {
	pdf := fpdf.New("P", reportPDFUnit, "A4", "")
	// 关闭流压缩：简单表格的体积上限由 reportMaxBytes 封顶，未压缩换来
	// 确定性的产物字节（内容流可直接断言、diff 可读）。
	pdf.SetCompression(false)
	pdf.SetMargins(reportPDFMargin, reportPDFMargin, reportPDFMargin)
	pdf.SetAutoPageBreak(true, reportPDFMargin)
	pdf.SetAuthor("AetherLink", true)
	pdf.SetTitle(reportArtifactTitle(title), true)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(reportPDFRowWidth(), reportPDFRowHeight, sanitizeReportPDFText(reportArtifactTitle(title)), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(reportPDFRowWidth(), reportPDFRowHeight,
		sanitizeReportPDFText(windowStart.UTC().Format(time.RFC3339)+" - "+windowEnd.UTC().Format(time.RFC3339)+" UTC"),
		"", 1, "L", false, 0, "")
	pdf.Ln(2)

	writeReportPDFRow(pdf, []string{"timestamp", "device_id", "key", "value"}, "B")
	for _, row := range rows {
		writeReportPDFRow(pdf, []string{row.Timestamp, row.DeviceID, row.Key, row.Value}, "")
	}

	buffer := &reportLimitedBuffer{limit: reportMaxBytes}
	if err := pdf.Output(buffer); err != nil {
		return nil, err
	}
	return append([]byte(nil), buffer.Bytes()...), nil
}

func reportPDFRowWidth() float64 {
	var total float64
	for _, width := range reportPDFColumnWidths {
		total += width
	}
	return total
}

// writeReportPDFRow emits one bordered row; the last cell closes the line so
// each call lands on its own table row and the auto page break stays row-aligned.
func writeReportPDFRow(pdf *fpdf.Fpdf, cells []string, style string) {
	pdf.SetFont("Helvetica", style, 9)
	for index, cell := range cells {
		line := 0
		if index == len(cells)-1 {
			line = 1
		}
		pdf.CellFormat(reportPDFColumnWidths[index], reportPDFRowHeight, sanitizeReportPDFText(cell), "1", line, "L", false, 0, "")
	}
}

// sanitizeReportPDFText collapses control characters and line breaks into
// spaces and degrades every rune outside Latin-1 to '?', because the PDF core
// fonts cannot represent them and raw UTF-8 bytes would corrupt the table.
func sanitizeReportPDFText(value string) string {
	if value == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for _, symbol := range value {
		switch {
		case symbol == '\n' || symbol == '\r' || symbol == '\t':
			builder.WriteByte(' ')
		case symbol >= 0x20 && symbol <= 0xFF:
			builder.WriteRune(symbol)
		default:
			builder.WriteByte('?')
		}
	}
	return builder.String()
}

func reportArtifactTitle(name string) string {
	if strings.TrimSpace(name) == "" {
		return reportFallbackTitle
	}
	return name
}

// reportRenderErrorCode maps renderer failures onto the generation error codes
// already understood by the retry classifier: the byte cap stays retryable-free
// and deterministic, everything else is a plain render failure.
func reportRenderErrorCode(err error) string {
	if errors.Is(err, errReportByteLimit) {
		return "byte_limit_exceeded"
	}
	return "report_render_failed"
}
