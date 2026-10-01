package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"gopkg.in/gomail.v2"
)

type ReportSMTPOutcome string

const (
	ReportSMTPAccepted  ReportSMTPOutcome = "accepted"
	ReportSMTPFailed    ReportSMTPOutcome = "failed"
	ReportSMTPAmbiguous ReportSMTPOutcome = "ambiguous"
)

// reportDeliveryBodyNote 是投递正文的固定说明（TB-49：报表产物一律走附件，不再内联进正文）。
const reportDeliveryBodyNote = "The scheduled AetherLink report is attached to this message."

// reportAttachmentFallbackName 是 schedule 名清洗后为空时的附件名兜底。
const reportAttachmentFallbackName = "report"

type ReportSMTPEnvelope struct {
	From       string
	To         []string
	MessageID  string
	Subject    string
	Body       string
	Attachment ReportSMTPAttachment
}

// ReportSMTPAttachment carries the report artifact out of band: the payload is
// never inlined into the body, whatever the format is.
type ReportSMTPAttachment struct {
	Filename string
	Content  []byte
}

type ReportSMTPResult struct {
	Outcome  ReportSMTPOutcome
	Code     string
	Accepted string
}

// ReportSMTPAdapter is injectable at the report boundary. The production
// implementation resolves configured credentials for each send and never
// copies them into report run/delivery persistence.
type ReportSMTPAdapter interface {
	Send(context.Context, ReportSMTPEnvelope) ReportSMTPResult
}

type configuredReportSMTPAdapter struct {
	load func() (model.EmailConfig, error)
	dial func(model.EmailConfig) (gomail.SendCloser, error)
}

func NewConfiguredReportSMTPAdapter() ReportSMTPAdapter {
	return &configuredReportSMTPAdapter{
		load: loadReportEmailConfig,
		dial: func(config model.EmailConfig) (gomail.SendCloser, error) {
			return newEmailProviderDialer(config).Dial()
		},
	}
}

func loadReportEmailConfig() (model.EmailConfig, error) {
	configuration, err := dal.GetNotificationServicesConfigByType(model.NoticeType_Email)
	if err != nil {
		return model.EmailConfig{}, err
	}
	return resolveEmailProviderConfig(configuration)
}

func loadReportEnvelopeFrom(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	configuration, err := loadReportEmailConfig()
	if err != nil {
		return "", err
	}
	sender := strings.TrimSpace(configuration.FromEmail)
	if sender == "" {
		return "", fmt.Errorf("SMTP envelope sender is not configured")
	}
	return sender, nil
}

func (adapter *configuredReportSMTPAdapter) Send(ctx context.Context, envelope ReportSMTPEnvelope) ReportSMTPResult {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ReportSMTPResult{Outcome: ReportSMTPFailed, Code: "smtp_canceled_before_connect"}
	default:
	}
	configuration, err := adapter.load()
	if err != nil {
		return ReportSMTPResult{Outcome: ReportSMTPFailed, Code: "smtp_not_configured"}
	}
	if strings.TrimSpace(envelope.From) == "" {
		return ReportSMTPResult{Outcome: ReportSMTPFailed, Code: "smtp_invalid_persisted_envelope"}
	}
	connection, err := adapter.dial(configuration)
	if err != nil {
		return ReportSMTPResult{Outcome: ReportSMTPFailed, Code: classifyReportSMTPError(err, false)}
	}
	message := gomail.NewMessage()
	message.SetHeader("From", envelope.From)
	message.SetHeader("To", envelope.To...)
	message.SetHeader("Message-ID", envelope.MessageID)
	message.SetHeader("Subject", envelope.Subject)
	message.SetBody("text/plain; charset=UTF-8", envelope.Body)
	// The artifact travels as a MIME attachment (TB-49). The filename comes from
	// the envelope builders, so an unnamed attachment would be a builder bug and
	// is dropped here instead of rendering a broken MIME part.
	if len(envelope.Attachment.Content) > 0 && strings.TrimSpace(envelope.Attachment.Filename) != "" {
		message.Attach(envelope.Attachment.Filename, gomail.SetCopyFunc(func(writer io.Writer) error {
			_, err := writer.Write(envelope.Attachment.Content)
			return err
		}))
	}

	// Once Send starts, an error may occur after the server accepted the DATA
	// terminator. Without a durable provider receipt it is unsafe to auto-retry.
	sendErr := connection.Send(envelope.From, envelope.To, message)
	closeErr := connection.Close()
	if sendErr != nil {
		return ReportSMTPResult{Outcome: ReportSMTPAmbiguous, Code: classifyReportSMTPError(sendErr, true)}
	}
	if closeErr != nil {
		return ReportSMTPResult{Outcome: ReportSMTPAccepted, Code: "smtp_accepted_close_failed", Accepted: envelope.MessageID}
	}
	return ReportSMTPResult{Outcome: ReportSMTPAccepted, Code: "smtp_accepted", Accepted: envelope.MessageID}
}

func classifyReportSMTPError(err error, dataStarted bool) string {
	if err == nil {
		return ""
	}
	if dataStarted {
		return "smtp_acceptance_unknown"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "smtp_connect_timeout"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return "smtp_connect_timeout"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "smtp_connection_refused"
	}
	return "smtp_pre_accept_failure"
}

func reportDeliveryEnvelope(run *model.ReportScheduleRun, from string, payload []byte) (ReportSMTPEnvelope, error) {
	if run == nil || strings.TrimSpace(from) == "" || len(payload) == 0 || !run.WindowStartAt.Before(run.WindowEndAt) {
		return ReportSMTPEnvelope{}, fmt.Errorf("report delivery envelope is invalid")
	}
	recipients, err := normalizeReportRecipients(run.ConfigSnapshot.Recipients)
	if err != nil {
		return ReportSMTPEnvelope{}, err
	}
	return ReportSMTPEnvelope{
		From: strings.TrimSpace(from), To: recipients, MessageID: reportMessageID(run.ID),
		Subject: fmt.Sprintf("[AetherLink report] %s", run.ConfigSnapshot.ScheduleName), Body: reportDeliveryBodyNote,
		Attachment: ReportSMTPAttachment{
			Filename: reportAttachmentFilename(run),
			Content:  append([]byte(nil), payload...),
		},
	}, nil
}

// persistedReportDeliveryEnvelope rebuilds the immutable message from the
// delivery row. The attachment name is derived again from the claimed run
// (schedule name + window date + format), which is deterministic from data the
// delivery already owns, so no extra persistence is needed.
func persistedReportDeliveryEnvelope(delivery *model.ReportScheduleDelivery, run *model.ReportScheduleRun) (ReportSMTPEnvelope, error) {
	if delivery == nil || strings.TrimSpace(delivery.EnvelopeFrom) == "" || len(delivery.EnvelopeRecipients) == 0 ||
		strings.TrimSpace(delivery.MessageID) == "" || strings.TrimSpace(delivery.Subject) == "" || len(delivery.Payload) == 0 {
		return ReportSMTPEnvelope{}, fmt.Errorf("persisted report delivery envelope is invalid")
	}
	return ReportSMTPEnvelope{
		From: delivery.EnvelopeFrom, To: append([]string(nil), delivery.EnvelopeRecipients...),
		MessageID: delivery.MessageID, Subject: delivery.Subject, Body: reportDeliveryBodyNote,
		Attachment: ReportSMTPAttachment{
			Filename: reportAttachmentFilename(run),
			Content:  append([]byte(nil), delivery.Payload...),
		},
	}, nil
}

// reportAttachmentFilename 组装附件文件名：<清洗后的 schedule 名>-<窗口结束日 UTC>.<扩展名>。
// run 缺失（防御路径）时同样给出合法文件名，绝不让空文件名进入 MIME part。
func reportAttachmentFilename(run *model.ReportScheduleRun) string {
	name, windowEnd, format := reportAttachmentFallbackName, time.Time{}, model.ReportFormatCSV
	if run != nil {
		name = sanitizeReportFilename(run.ConfigSnapshot.ScheduleName)
		windowEnd = run.WindowEndAt
		format = run.ConfigSnapshot.Format
	}
	return fmt.Sprintf("%s-%s%s", name, windowEnd.UTC().Format("2006-01-02"), reportFileExtension(format))
}

// sanitizeReportFilename 保留字母数字、'-' 与 '_'，其余字符（含路径分隔符与
// Unicode）折叠为 '_'，首尾 '_' 去除，空名回退 "report"，上限 64 字符。
func sanitizeReportFilename(name string) string {
	var builder strings.Builder
	for _, symbol := range strings.TrimSpace(name) {
		switch {
		case symbol >= 'a' && symbol <= 'z', symbol >= 'A' && symbol <= 'Z', symbol >= '0' && symbol <= '9',
			symbol == '-', symbol == '_':
			builder.WriteRune(symbol)
		default:
			builder.WriteByte('_')
		}
	}
	cleaned := strings.Trim(builder.String(), "_")
	if cleaned == "" {
		cleaned = reportAttachmentFallbackName
	}
	if len(cleaned) > 64 {
		cleaned = cleaned[:64]
	}
	return cleaned
}

func reportFileExtension(format string) string {
	switch format {
	case model.ReportFormatHTML:
		return ".html"
	case model.ReportFormatPDF:
		return ".pdf"
	default:
		return ".csv"
	}
}
