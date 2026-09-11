package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"sort"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
)

const (
	reportMaxRows  = 100000
	reportMaxBytes = 25 << 20
)

var errReportByteLimit = errors.New("report byte limit exceeded")

type reportLimitedBuffer struct {
	bytes.Buffer
	limit int
}

func (buffer *reportLimitedBuffer) Write(value []byte) (int, error) {
	if len(value) > buffer.limit-buffer.Len() {
		return 0, errReportByteLimit
	}
	return buffer.Buffer.Write(value)
}

type ReportTelemetryReader interface {
	Read(context.Context, string, string, string, int64, int64, int) ([]*model.TelemetryData, error)
}

type reportTelemetryDAL struct{}

func (reportTelemetryDAL) Read(ctx context.Context, tenantID, deviceID, key string, start, end int64, limit int) ([]*model.TelemetryData, error) {
	return dal.GetTelemetryDataForReportContext(ctx, tenantID, deviceID, key, start, end, limit)
}

type ReportRunProcessor struct {
	Telemetry          ReportTelemetryReader
	SMTP               ReportSMTPAdapter
	EnvelopeFrom       func(context.Context) (string, error)
	RetryGeneration    func(context.Context, string, string, string, string) error
	FailGeneration     func(context.Context, string, string, string) error
	CompleteGeneration func(context.Context, string, string, string, []string, string, string, []byte, int64) error
}

func NewReportRunProcessor(smtp ReportSMTPAdapter) *ReportRunProcessor {
	if smtp == nil {
		smtp = NewConfiguredReportSMTPAdapter()
	}
	return &ReportRunProcessor{
		Telemetry:          reportTelemetryDAL{},
		SMTP:               smtp,
		EnvelopeFrom:       loadReportEnvelopeFrom,
		RetryGeneration:    dal.RetryClaimedReportGeneration,
		FailGeneration:     dal.FailClaimedReportGeneration,
		CompleteGeneration: dal.CompleteReportGeneration,
	}
}

func (processor *ReportRunProcessor) GenerateClaim(ctx context.Context, claim dal.ReportGenerationClaim) error {
	if claim.Run == nil {
		return fmt.Errorf("report generation claim is incomplete")
	}
	retry := processor.retryGeneration()
	fail := processor.failGeneration()
	complete := processor.completeGeneration()
	payload, rows, code, err := processor.generate(ctx, claim.Run)
	if err != nil {
		var settleErr error
		if isRetryableReportGenerationError(code) {
			settleErr = retry(ctx, claim.Run.ID, claim.Token, code, err.Error())
		} else {
			settleErr = fail(ctx, claim.Run.ID, claim.Token, code)
		}
		if settleErr != nil {
			return settleErr
		}
		return err
	}
	envelope, envelopeCode, err := processor.generationEnvelope(ctx, claim.Run, payload)
	if err != nil {
		var settleErr error
		if envelopeCode == "envelope_config_unavailable" {
			settleErr = retry(ctx, claim.Run.ID, claim.Token, envelopeCode, err.Error())
		} else {
			settleErr = fail(ctx, claim.Run.ID, claim.Token, envelopeCode)
		}
		if settleErr != nil {
			return settleErr
		}
		return err
	}
	return complete(ctx, claim.Run.ID, claim.Token, envelope.From, envelope.To,
		envelope.MessageID, envelope.Subject, payload, rows)
}

func (processor *ReportRunProcessor) retryGeneration() func(context.Context, string, string, string, string) error {
	if processor != nil && processor.RetryGeneration != nil {
		return processor.RetryGeneration
	}
	return dal.RetryClaimedReportGeneration
}

func (processor *ReportRunProcessor) failGeneration() func(context.Context, string, string, string) error {
	if processor != nil && processor.FailGeneration != nil {
		return processor.FailGeneration
	}
	return dal.FailClaimedReportGeneration
}

func (processor *ReportRunProcessor) completeGeneration() func(context.Context, string, string, string, []string, string, string, []byte, int64) error {
	if processor != nil && processor.CompleteGeneration != nil {
		return processor.CompleteGeneration
	}
	return dal.CompleteReportGeneration
}

func isRetryableReportGenerationError(code string) bool {
	return code == "telemetry_query_failed"
}

func (processor *ReportRunProcessor) generate(ctx context.Context, run *model.ReportScheduleRun) ([]byte, int64, string, error) {
	if run == nil || run.ConfigSnapshot.Format != "csv" || !run.WindowStartAt.Before(run.WindowEndAt) {
		return nil, 0, "invalid_snapshot", fmt.Errorf("unsupported report snapshot")
	}
	if processor.Telemetry == nil {
		return nil, 0, "telemetry_query_failed", fmt.Errorf("report telemetry reader is unavailable")
	}
	devices := cloneSortedStrings(run.ConfigSnapshot.DeviceIDs)
	keys := cloneSortedStrings(run.ConfigSnapshot.Keys)
	buffer := &reportLimitedBuffer{limit: reportMaxBytes}
	if _, err := buffer.Write([]byte("\xEF\xBB\xBF")); err != nil {
		return nil, 0, reportWriteErrorCode(err), err
	}
	writer := csv.NewWriter(buffer)
	if err := writer.Write([]string{"timestamp", "device_id", "key", "value"}); err != nil {
		return nil, 0, reportWriteErrorCode(err), err
	}
	var count int64
	for _, deviceID := range devices {
		for _, key := range keys {
			remaining := reportMaxRows - int(count)
			rows, err := processor.Telemetry.Read(ctx, run.TenantID, deviceID, key,
				run.WindowStartAt.UnixMilli(), run.WindowEndAt.UnixMilli(), remaining+1)
			if err != nil {
				return nil, count, "telemetry_query_failed", err
			}
			if len(rows) > remaining {
				return nil, count, "row_limit_exceeded", fmt.Errorf("report row limit exceeded")
			}
			for _, row := range rows {
				if row == nil {
					return nil, count, "telemetry_query_failed", fmt.Errorf("telemetry query returned an invalid row")
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].T != rows[j].T {
					return rows[i].T < rows[j].T
				}
				return historyTelemetryValueToString(rows[i]) < historyTelemetryValueToString(rows[j])
			})
			for _, row := range rows {
				if err := writer.Write([]string{
					time.UnixMilli(row.T).UTC().Format(time.RFC3339Nano), deviceID, key, historyTelemetryValueToString(row),
				}); err != nil {
					return nil, count, reportWriteErrorCode(err), err
				}
				count++
			}
			writer.Flush()
			if err := writer.Error(); err != nil {
				return nil, count, reportWriteErrorCode(err), err
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, count, reportWriteErrorCode(err), err
	}
	return append([]byte(nil), buffer.Bytes()...), count, "", nil
}

func reportWriteErrorCode(err error) string {
	if errors.Is(err, errReportByteLimit) {
		return "byte_limit_exceeded"
	}
	return "csv_write_failed"
}

func (processor *ReportRunProcessor) generationEnvelope(ctx context.Context, run *model.ReportScheduleRun, payload []byte) (ReportSMTPEnvelope, string, error) {
	load := processor.EnvelopeFrom
	if load == nil {
		load = loadReportEnvelopeFrom
	}
	from, err := load(ctx)
	if err != nil {
		return ReportSMTPEnvelope{}, "envelope_config_unavailable", err
	}
	envelope, err := reportDeliveryEnvelope(run, from, payload)
	if err != nil {
		return ReportSMTPEnvelope{}, "invalid_envelope", err
	}
	return envelope, "", nil
}

func (processor *ReportRunProcessor) DeliverClaim(ctx context.Context, claim dal.ReportDeliveryClaim) error {
	if claim.Run == nil || claim.Delivery == nil {
		return fmt.Errorf("report delivery claim is incomplete")
	}
	envelope, err := persistedReportDeliveryEnvelope(claim.Delivery)
	if err != nil {
		return dal.SettleReportDeliveryFailed(ctx, claim.Run.ID, claim.Token, "invalid_envelope")
	}
	if processor.SMTP == nil {
		return dal.RetryClaimedReportDelivery(ctx, claim.Run.ID, claim.Token, "smtp_not_configured")
	}
	result := processor.SMTP.Send(ctx, envelope)
	switch result.Outcome {
	case ReportSMTPAccepted:
		return dal.SettleReportDeliveryAccepted(ctx, claim.Run.ID, claim.Token)
	case ReportSMTPFailed:
		return dal.RetryClaimedReportDelivery(ctx, claim.Run.ID, claim.Token, result.Code)
	default:
		return dal.SettleReportDeliveryAmbiguous(ctx, claim.Run.ID, claim.Token, result.Code)
	}
}

func isReportClaimLost(err error) bool { return errors.Is(err, dal.ErrReportClaimLost) }
