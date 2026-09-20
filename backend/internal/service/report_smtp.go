package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"

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

type ReportSMTPEnvelope struct {
	From      string
	To        []string
	MessageID string
	Subject   string
	Body      string
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
		Subject: fmt.Sprintf("[AetherLink report] %s", run.ConfigSnapshot.ScheduleName), Body: string(payload),
	}, nil
}

func persistedReportDeliveryEnvelope(delivery *model.ReportScheduleDelivery) (ReportSMTPEnvelope, error) {
	if delivery == nil || strings.TrimSpace(delivery.EnvelopeFrom) == "" || len(delivery.EnvelopeRecipients) == 0 ||
		strings.TrimSpace(delivery.MessageID) == "" || strings.TrimSpace(delivery.Subject) == "" || len(delivery.Payload) == 0 {
		return ReportSMTPEnvelope{}, fmt.Errorf("persisted report delivery envelope is invalid")
	}
	return ReportSMTPEnvelope{
		From: delivery.EnvelopeFrom, To: append([]string(nil), delivery.EnvelopeRecipients...),
		MessageID: delivery.MessageID, Subject: delivery.Subject, Body: string(delivery.Payload),
	}, nil
}
