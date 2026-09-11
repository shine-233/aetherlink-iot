package service

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"gopkg.in/gomail.v2"
)

type reportSMTPConnection struct {
	send  func(string) error
	close func() error
}

func (connection reportSMTPConnection) Send(from string, _ []string, _ io.WriterTo) error {
	if connection.send == nil {
		return nil
	}
	return connection.send(from)
}

func (connection reportSMTPConnection) Close() error {
	if connection.close == nil {
		return nil
	}
	return connection.close()
}

type timeoutReportSMTPError struct{}

func (timeoutReportSMTPError) Error() string   { return "timeout" }
func (timeoutReportSMTPError) Timeout() bool   { return true }
func (timeoutReportSMTPError) Temporary() bool { return true }

func TestConfiguredReportSMTPAdapterClassifiesAcceptanceBoundary(t *testing.T) {
	config := model.EmailConfig{FromEmail: "reports@example.com"}
	envelope := ReportSMTPEnvelope{From: "reports@example.com", To: []string{"recipient@example.com"}, MessageID: "<report@test>", Subject: "Report", Body: "body"}
	tests := []struct {
		name        string
		dial        func(model.EmailConfig) (gomail.SendCloser, error)
		wantOutcome ReportSMTPOutcome
		wantCode    string
	}{
		{name: "definite dial timeout", dial: func(model.EmailConfig) (gomail.SendCloser, error) { return nil, timeoutReportSMTPError{} }, wantOutcome: ReportSMTPFailed, wantCode: "smtp_connect_timeout"},
		{name: "send response unknown", dial: func(model.EmailConfig) (gomail.SendCloser, error) {
			return reportSMTPConnection{send: func(string) error { return errors.New("connection closed after DATA") }}, nil
		}, wantOutcome: ReportSMTPAmbiguous, wantCode: "smtp_acceptance_unknown"},
		{name: "accepted despite close failure", dial: func(model.EmailConfig) (gomail.SendCloser, error) {
			return reportSMTPConnection{close: func() error { return errors.New("QUIT failed") }}, nil
		}, wantOutcome: ReportSMTPAccepted, wantCode: "smtp_accepted_close_failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := &configuredReportSMTPAdapter{load: func() (model.EmailConfig, error) { return config, nil }, dial: test.dial}
			result := adapter.Send(context.Background(), envelope)
			if result.Outcome != test.wantOutcome || result.Code != test.wantCode {
				t.Fatalf("result = %+v, want outcome %q code %q", result, test.wantOutcome, test.wantCode)
			}
		})
	}
}

func TestConfiguredReportSMTPAdapterPreservesPersistedSender(t *testing.T) {
	var sentFrom string
	adapter := &configuredReportSMTPAdapter{
		load: func() (model.EmailConfig, error) { return model.EmailConfig{FromEmail: "configured@example.com"}, nil },
		dial: func(model.EmailConfig) (gomail.SendCloser, error) {
			return reportSMTPConnection{send: func(from string) error { sentFrom = from; return nil }}, nil
		},
	}
	result := adapter.Send(context.Background(), ReportSMTPEnvelope{From: "persisted@example.com", To: []string{"to@example.com"}, MessageID: "<id@test>", Subject: "subject", Body: "body"})
	if result.Outcome != ReportSMTPAccepted || sentFrom != "persisted@example.com" {
		t.Fatalf("result = %+v, sender = %q", result, sentFrom)
	}
}

func TestClassifyReportSMTPPreAcceptanceErrors(t *testing.T) {
	if code := classifyReportSMTPError(timeoutReportSMTPError{}, false); code != "smtp_connect_timeout" {
		t.Fatalf("timeout code = %q", code)
	}
	if code := classifyReportSMTPError(&net.OpError{Op: "dial", Err: errors.New("refused")}, true); code != "smtp_acceptance_unknown" {
		t.Fatalf("post-DATA code = %q", code)
	}
	if code := classifyReportSMTPError(context.DeadlineExceeded, false); code != "smtp_connect_timeout" {
		t.Fatalf("deadline code = %q", code)
	}
}

func TestReportDeliveryEnvelopeAndMessageIDAreDeterministic(t *testing.T) {
	start := time.Unix(1700000000, 0).UTC()
	run := &model.ReportScheduleRun{ID: "run-1", ConfigSnapshot: model.ReportRunConfigSnapshot{ScheduleName: "Daily", Recipients: "B@example.com; a@example.com"}, WindowStartAt: start, WindowEndAt: start.Add(time.Hour)}
	first, err := reportDeliveryEnvelope(run, "reports@example.com", []byte("csv"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := reportDeliveryEnvelope(run, "reports@example.com", []byte("csv"))
	if err != nil {
		t.Fatal(err)
	}
	if first.MessageID != second.MessageID || first.Subject != second.Subject || first.Body != second.Body || len(first.To) != 2 || first.To[0] != "a@example.com" {
		t.Fatalf("envelopes are not deterministic: first=%+v second=%+v", first, second)
	}
}
