// 报表渲染与附件投递单测（TB-49）：覆盖 HTML 表头/行数、PDF 字节头、附件文件名与投递断言。
// 核心逻辑：generate 按快照格式分发渲染器；信封构建器把产物装进附件并通过 gomail 落成 MIME。
// 关键注意事项：所有断言不依赖数据库；SMTP 侧用接口桩捕获信封/报文，验证附件而非正文内联。
// 重构建议：如渲染器后续扩展（富样式/图表），按格式补充对应的字节头与内容断言用例。

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"gopkg.in/gomail.v2"
)

func htmlReportFixture() *model.ReportScheduleRun {
	start := time.Unix(1700000000, 0).UTC() // 2023-11-14T22:13:20Z
	return &model.ReportScheduleRun{
		ID: "run-1", TenantID: "tenant-1", WindowStartAt: start, WindowEndAt: start.Add(time.Hour),
		ConfigSnapshot: model.ReportRunConfigSnapshot{
			ScheduleName: "Daily Ops", Recipients: "ops@example.test",
			DeviceIDs: []string{"device-a"}, Keys: []string{"temperature"}, Format: model.ReportFormatHTML,
		},
	}
}

func reportRowsReader(rows int) ReportTelemetryReader {
	return reportTelemetryReaderFunc(func(_ context.Context, _, _, _ string, start, _ int64, _ int) ([]*model.TelemetryData, error) {
		out := make([]*model.TelemetryData, rows)
		for index := range out {
			value := float64(index)
			out[index] = &model.TelemetryData{T: start + int64(index)*1000, NumberV: &value}
		}
		return out, nil
	})
}

// HTML 产物必须带四列表头并逐行落表，且模板转义必须生效（值里的标签不得逃逸）。
func TestReportGeneratorRendersHTMLWithHeaderAndRowCount(t *testing.T) {
	processor := &ReportRunProcessor{Telemetry: reportRowsReader(3)}
	payload, rows, code, err := processor.generate(context.Background(), htmlReportFixture())
	if err != nil || code != "" || rows != 3 {
		t.Fatalf("generate = rows %d code %q error %v", rows, code, err)
	}
	html := string(payload)
	for _, header := range []string{"<th>timestamp</th>", "<th>device_id</th>", "<th>key</th>", "<th>value</th>"} {
		if !strings.Contains(html, header) {
			t.Fatalf("HTML artifact misses header %q: %s", header, html)
		}
	}
	if got := strings.Count(html, "<tr><td>"); got != 3 {
		t.Fatalf("HTML data row count = %d, want 3", got)
	}
	if !strings.Contains(html, "Daily Ops") {
		t.Fatalf("HTML artifact misses schedule title: %s", html)
	}
	if !strings.Contains(html, "2023-11-14T22:13:20Z") || !strings.Contains(html, "2023-11-14T23:13:20Z") {
		t.Fatalf("HTML artifact misses snapshot window: %s", html)
	}
}

// 值中的 HTML 元字符必须被 html/template 转义，报表才不会把遥测值当标记渲染。
func TestReportHTMLRenderingEscapesTelemetryValues(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	markup := `<b>&"</b>`
	processor := &ReportRunProcessor{Telemetry: reportTelemetryReaderFunc(func(context.Context, string, string, string, int64, int64, int) ([]*model.TelemetryData, error) {
		return []*model.TelemetryData{{T: start.UnixMilli() + 500, StringV: &markup}}, nil
	})}
	run := htmlReportFixture()
	run.WindowStartAt, run.WindowEndAt = start, start.Add(time.Hour)
	payload, _, _, err := processor.generate(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), markup) || !strings.Contains(string(payload), "&lt;b&gt;&amp;") {
		t.Fatalf("HTML artifact did not escape telemetry value: %s", string(payload))
	}
}

// PDF 产物必须以 %PDF- 字节头开始，且未压缩流里能找到规范化后的行文本。
func TestReportGeneratorRendersPDFWithHeaderBytes(t *testing.T) {
	processor := &ReportRunProcessor{Telemetry: reportRowsReader(2)}
	run := htmlReportFixture()
	run.ConfigSnapshot.Format = model.ReportFormatPDF
	payload, rows, code, err := processor.generate(context.Background(), run)
	if err != nil || code != "" || rows != 2 {
		t.Fatalf("generate = rows %d code %q error %v", rows, code, err)
	}
	if !bytes.HasPrefix(payload, []byte("%PDF-")) {
		t.Fatalf("PDF artifact starts with %q, want %%PDF- header", payload[:min(5, len(payload))])
	}
	if !strings.Contains(string(payload), "device-a") {
		t.Fatalf("PDF artifact misses normalized row text: %.120s", payload)
	}
}

// 未知快照格式必须 fail-closed 拒绝，而不是悄悄按 CSV 兜底。
func TestReportGeneratorRejectsUnknownSnapshotFormat(t *testing.T) {
	processor := &ReportRunProcessor{Telemetry: reportRowsReader(1)}
	run := htmlReportFixture()
	run.ConfigSnapshot.Format = "xlsx"
	payload, _, code, err := processor.generate(context.Background(), run)
	if err == nil || code != "invalid_snapshot" || payload != nil {
		t.Fatalf("generate = payload %v code %q error %v", payload, code, err)
	}
}

// 渲染失败在字节上限内映射为 byte_limit_exceeded，其余归为 report_render_failed。
func TestReportRenderErrorCodeClassifiesByteLimit(t *testing.T) {
	if code := reportRenderErrorCode(errReportByteLimit); code != "byte_limit_exceeded" {
		t.Fatalf("byte limit code = %q", code)
	}
	if code := reportRenderErrorCode(errors.New("template exploded")); code != "report_render_failed" {
		t.Fatalf("render code = %q", code)
	}
}

// 附件文件名口径：<清洗后的 schedule 名>-<窗口结束日 UTC>.<格式扩展名>。
func TestReportAttachmentFilenameUsesScheduleNameAndDate(t *testing.T) {
	run := htmlReportFixture()
	if got := reportAttachmentFilename(run); got != "Daily_Ops-2023-11-14.html" {
		t.Fatalf("html attachment filename = %q", got)
	}
	run.ConfigSnapshot.Format = model.ReportFormatPDF
	if got := reportAttachmentFilename(run); got != "Daily_Ops-2023-11-14.pdf" {
		t.Fatalf("pdf attachment filename = %q", got)
	}
	run.ConfigSnapshot.Format = model.ReportFormatCSV
	if got := reportAttachmentFilename(run); got != "Daily_Ops-2023-11-14.csv" {
		t.Fatalf("csv attachment filename = %q", got)
	}
	if got := reportAttachmentFilename(nil); got != "report-0001-01-01.csv" {
		t.Fatalf("nil run attachment filename = %q", got)
	}
}

func TestSanitizeReportFilenameStripsUnsafeCharacters(t *testing.T) {
	if got := sanitizeReportFilename("../../etc passwd"); strings.ContainsAny(got, "/ ") {
		t.Fatalf("filename keeps path separators or spaces: %q", got)
	}
	if got := sanitizeReportFilename("  部门/日报  "); got != "report" {
		t.Fatalf("non-ASCII name = %q, want collapsed to fallback", got)
	}
	if got := sanitizeReportFilename("   "); got != "report" {
		t.Fatalf("blank name = %q, want fallback", got)
	}
	long := strings.Repeat("a", 100)
	if got := sanitizeReportFilename(long); len(got) != 64 {
		t.Fatalf("long name length = %d, want capped at 64", len(got))
	}
}

// 生成信封必须把产物装进附件并配好文件名；正文只能是固定说明，不得再内联产物。
func TestReportDeliveryEnvelopeAttachesArtifactInsteadOfInlining(t *testing.T) {
	run := htmlReportFixture()
	payload := []byte("timestamp,device_id,key,value\n2023,device-a,temperature,21.5\n")
	envelope, err := reportDeliveryEnvelope(run, "sender@example.test", payload)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Attachment.Filename != "Daily_Ops-2023-11-14.html" {
		t.Fatalf("attachment filename = %q", envelope.Attachment.Filename)
	}
	if !bytes.Equal(envelope.Attachment.Content, payload) {
		t.Fatalf("attachment content = %q, want the generated payload", envelope.Attachment.Content)
	}
	if envelope.Body != reportDeliveryBodyNote || strings.Contains(envelope.Body, string(payload)) {
		t.Fatalf("body must be the fixed note, got %q", envelope.Body)
	}
}

// 投递侧（smtp 桩）：从持久化投递重建的信封同样以附件携带产物，
// 文件名从已声明的 run（schedule 名+日期+格式）确定性推导。
func TestDeliverClaimSendsPersistedPayloadAsAttachment(t *testing.T) {
	claim := reportDeliveryClaimFixture()
	start := time.Unix(1700000000, 0).UTC()
	claim.Run.WindowEndAt = start.Add(time.Hour)
	claim.Run.ConfigSnapshot = model.ReportRunConfigSnapshot{ScheduleName: "Daily", Format: model.ReportFormatPDF}
	var sent ReportSMTPEnvelope
	var settlement string
	processor := &ReportRunProcessor{
		SMTP: reportSMTPFunc(func(_ context.Context, envelope ReportSMTPEnvelope) ReportSMTPResult {
			sent = envelope
			return ReportSMTPResult{Outcome: ReportSMTPAccepted}
		}),
		SettleDeliveryAccepted: func(context.Context, string, string) error { settlement = "accepted"; return nil },
	}
	if err := processor.DeliverClaim(context.Background(), claim); err != nil {
		t.Fatalf("DeliverClaim error = %v", err)
	}
	if settlement != "accepted" {
		t.Fatalf("settlement = %q, want accepted", settlement)
	}
	if sent.Attachment.Filename != "Daily-2023-11-14.pdf" {
		t.Fatalf("attachment filename = %q", sent.Attachment.Filename)
	}
	if !bytes.Equal(sent.Attachment.Content, claim.Delivery.Payload) {
		t.Fatal("attachment content must be the persisted payload")
	}
	if sent.Body != reportDeliveryBodyNote {
		t.Fatalf("body = %q, want the fixed note", sent.Body)
	}
}

type reportSMTPFunc func(context.Context, ReportSMTPEnvelope) ReportSMTPResult

func (function reportSMTPFunc) Send(ctx context.Context, envelope ReportSMTPEnvelope) ReportSMTPResult {
	return function(ctx, envelope)
}

// gomail 侧接线断言：适配器真的把附件挂成 MIME part（Content-Disposition: attachment），
// 而不是把产物塞回正文。
func TestConfiguredReportSMTPAdapterAttachesArtifactAsMIMEPart(t *testing.T) {
	var captured bytes.Buffer
	adapter := &configuredReportSMTPAdapter{
		load: func() (model.EmailConfig, error) { return model.EmailConfig{FromEmail: "reports@example.test"}, nil },
		dial: func(model.EmailConfig) (gomail.SendCloser, error) {
			return capturingReportConnection{sink: &captured}, nil
		},
	}
	envelope := ReportSMTPEnvelope{
		From: "reports@example.test", To: []string{"ops@example.test"}, MessageID: "<report@test>",
		Subject: "Report", Body: reportDeliveryBodyNote,
		Attachment: ReportSMTPAttachment{Filename: "Daily_Ops-2023-11-14.pdf", Content: []byte("%PDF-1.4 artifact")},
	}
	result := adapter.Send(context.Background(), envelope)
	if result.Outcome != ReportSMTPAccepted {
		t.Fatalf("send outcome = %v code %q", result.Outcome, result.Code)
	}
	mime := captured.String()
	if !strings.Contains(mime, `filename="Daily_Ops-2023-11-14.pdf"`) || !strings.Contains(mime, "attachment") {
		t.Fatalf("MIME message misses attachment part: %s", mime)
	}
	if !strings.Contains(mime, "JVBERi0") { // base64("%PDF-")
		t.Fatalf("MIME message misses base64 payload: %s", mime)
	}
	if !strings.Contains(mime, reportDeliveryBodyNote) {
		t.Fatalf("MIME body should keep the fixed note: %s", mime)
	}
}

// capturingReportConnection 实现 gomail.SendCloser，把待发送的 MIME 报文
// 物化到调用方提供的缓冲，供 MIME 断言使用。
type capturingReportConnection struct {
	sink *bytes.Buffer
}

func (connection capturingReportConnection) Send(from string, _ []string, message io.WriterTo) error {
	if from == "" {
		return errors.New("capturing connection requires an envelope sender")
	}
	_, err := message.WriteTo(connection.sink)
	return err
}

func (connection capturingReportConnection) Close() error { return nil }
