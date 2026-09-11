package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
	"github.com/robfig/cron"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const (
	maxReportIdempotencyKeyBytes = 128
	defaultReportMaxAttempts     = 3
	maximumReportMaxAttempts     = 20
)

// ReportScheduleService owns report schedule and durable run orchestration.
type ReportScheduleService struct{}

func (ReportScheduleService) CreateReportSchedule(ctx context.Context, req *model.CreateReportScheduleRequest, claims *utils.UserClaims) (*model.ReportSchedule, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	location, schedule, err := parseReportSchedule(req.CronExpr, req.Timezone)
	if err != nil {
		return nil, reportAPIError(errcode.CodeParamError, err.Error())
	}
	lookback := req.LookbackHours
	if lookback == 0 {
		lookback = 24
	}
	format := req.Format
	if format == "" {
		format = "csv"
	}
	recipients, err := normalizeReportRecipients(req.Recipients)
	if err != nil {
		return nil, reportAPIError(errcode.CodeParamError, err.Error())
	}
	entity := &model.ReportSchedule{
		ID: uuid.NewString(), TenantID: claims.TenantID, Name: strings.TrimSpace(req.Name),
		CronExpr: strings.TrimSpace(req.CronExpr), Timezone: location.String(), Recipients: strings.Join(recipients, ","),
		DeviceIDs: cloneSortedStrings(req.DeviceIDs), Keys: cloneSortedStrings(req.Keys), LookbackHours: lookback,
		Format: format, Enabled: req.EnabledOrDefault(), Revision: 1, LastStatus: "pending",
	}
	if err := dal.CreateReportScheduleContext(ctx, entity, func(_ *model.ReportSchedule, databaseNow time.Time) (time.Time, error) {
		return schedule.Next(databaseNow.In(location)).UTC(), nil
	}); err != nil {
		return nil, reportStorageError(err)
	}
	return entity, nil
}

func (ReportScheduleService) UpdateReportSchedule(ctx context.Context, id string, req *model.UpdateReportScheduleRequest, claims *utils.UserClaims) (*model.ReportSchedule, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	var next dal.ReportNextOccurrence
	if req.CronExpr != nil && req.Timezone != nil {
		location, schedule, err := parseReportSchedule(strings.TrimSpace(*req.CronExpr), strings.TrimSpace(*req.Timezone))
		if err != nil {
			return nil, reportAPIError(errcode.CodeParamError, err.Error())
		}
		next = func(_ *model.ReportSchedule, databaseNow time.Time) (time.Time, error) {
			return schedule.Next(databaseNow.In(location)).UTC(), nil
		}
	} else {
		next = NextReportScheduleOccurrence
	}
	changes := dal.ReportScheduleChanges{
		Name: req.Name, CronExpr: req.CronExpr, Timezone: req.Timezone, Recipients: req.Recipients,
		DeviceIDs: req.DeviceIDs, Keys: req.Keys, LookbackHours: req.LookbackHours, Format: req.Format, Enabled: req.Enabled,
	}
	if req.Recipients != nil {
		recipients, normalizeErr := normalizeReportRecipients(*req.Recipients)
		if normalizeErr != nil {
			return nil, reportAPIError(errcode.CodeParamError, normalizeErr.Error())
		}
		normalized := strings.Join(recipients, ",")
		changes.Recipients = &normalized
	}
	if req.DeviceIDs != nil {
		value := cloneSortedStrings(*req.DeviceIDs)
		changes.DeviceIDs = &value
	}
	if req.Keys != nil {
		value := cloneSortedStrings(*req.Keys)
		changes.Keys = &value
	}
	updated, err := dal.UpdateReportScheduleContext(ctx, id, claims.TenantID, req.Revision, changes, next)
	if err != nil {
		return nil, reportStorageError(err)
	}
	return updated, nil
}

func (ReportScheduleService) DeleteReportSchedule(ctx context.Context, id string, revision int64, claims *utils.UserClaims) error {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	if err := dal.SoftDeleteReportScheduleContext(ctx, id, claims.TenantID, revision); err != nil {
		return reportStorageError(err)
	}
	return nil
}

func (ReportScheduleService) ListReportSchedules(ctx context.Context, req model.ReportScheduleListRequest, claims *utils.UserClaims) (*model.ReportScheduleListResponse, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	result, err := dal.ListReportSchedulesContext(ctx, claims.TenantID, req)
	if err != nil {
		return nil, reportStorageError(err)
	}
	return result, nil
}

func (ReportScheduleService) GetReportSchedule(ctx context.Context, id string, claims *utils.UserClaims) (*model.ReportSchedule, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	result, err := dal.GetReportScheduleInTenantContext(ctx, id, claims.TenantID)
	if err != nil {
		return nil, reportStorageError(err)
	}
	return result, nil
}

func (ReportScheduleService) SubmitManualRun(ctx context.Context, id, key string, claims *utils.UserClaims) (*model.ReportRunActionResponse, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	if err := validateReportIdempotencyKey(key); err != nil {
		return nil, err
	}
	fingerprint := []byte(fmt.Sprintf("manual\x00%s\x00%s", claims.TenantID, id))
	submission, err := dal.SubmitManualReportRun(ctx, dal.ManualReportRunInput{
		ID: uuid.NewString(), TenantID: claims.TenantID, ScheduleID: id, IdempotencyKey: key,
		RequestFingerprint: fingerprint, MaxAttempts: configuredReportMaxAttempts(),
	})
	if err != nil {
		return nil, reportStorageError(err)
	}
	return reportRunAction(submission, id), nil
}

func (ReportScheduleService) SubmitRetry(ctx context.Context, id, runID, key string, claims *utils.UserClaims) (*model.ReportRunActionResponse, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	if err := validateReportIdempotencyKey(key); err != nil {
		return nil, err
	}
	fingerprint := []byte(fmt.Sprintf("retry\x00%s\x00%s\x00%s", claims.TenantID, id, runID))
	submission, err := dal.SubmitRetryReportRun(ctx, dal.RetryReportRunInput{
		ID: uuid.NewString(), TenantID: claims.TenantID, ScheduleID: id, ParentRunID: runID,
		IdempotencyKey: key, RequestFingerprint: fingerprint, MaxAttempts: configuredReportMaxAttempts(),
	})
	if err != nil {
		return nil, reportStorageError(err)
	}
	return reportRunAction(submission, id), nil
}

func (ReportScheduleService) ListRuns(ctx context.Context, id string, req model.ReportRunListRequest, claims *utils.UserClaims) (*model.ReportRunListResponse, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	result, err := dal.ListReportRunsContext(ctx, claims.TenantID, id, req)
	if err != nil {
		return nil, reportStorageError(err)
	}
	return result, nil
}

func (ReportScheduleService) GetRun(ctx context.Context, id, runID string, claims *utils.UserClaims) (*model.ReportRunResponse, error) {
	if claims == nil || strings.TrimSpace(claims.TenantID) == "" {
		return nil, reportAPIError(errcode.CodeNoPermission, "report schedule access denied")
	}
	result, err := dal.GetReportRunContext(ctx, claims.TenantID, id, runID)
	if err != nil {
		return nil, reportStorageError(err)
	}
	return result, nil
}

func reportRunAction(submission *dal.ReportRunSubmission, scheduleID string) *model.ReportRunActionResponse {
	// Report the real projected status: an idempotent replay can return a run that
	// has already reached a terminal state, and claiming "queued" would tell the
	// caller that work is still pending when it is not.
	status := submission.OverallStatus
	if strings.TrimSpace(status) == "" {
		status = model.ReportProjectedStatusQueued
	}
	return &model.ReportRunActionResponse{
		RunID: submission.Run.ID, ScheduleID: scheduleID, Status: status,
		StatusURL:        fmt.Sprintf("/api/v1/report/schedules/%s/runs/%s", scheduleID, submission.Run.ID),
		IdempotentReplay: submission.IdempotentReplay, DuplicateDeliveryRisk: submission.DuplicateDeliveryRisk,
	}
}

func parseReportSchedule(expr, timezone string) (*time.Location, cron.Schedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 && len(fields) != 6 {
		return nil, nil, fmt.Errorf("cron_expr must contain 5 or 6 fields")
	}
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return nil, nil, fmt.Errorf("timezone must be a valid IANA timezone")
	}
	var parsed cron.Schedule
	if len(fields) == 5 {
		parsed, err = cron.ParseStandard(strings.Join(fields, " "))
	} else {
		parsed, err = cron.Parse(strings.Join(fields, " "))
	}
	if err != nil {
		return nil, nil, fmt.Errorf("cron_expr is invalid")
	}
	return location, parsed, nil
}

// NextReportScheduleOccurrence is the DAL/worker callback for timezone-aware schedules.
func NextReportScheduleOccurrence(schedule *model.ReportSchedule, after time.Time) (time.Time, error) {
	location, parsed, err := parseReportSchedule(schedule.CronExpr, schedule.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.Next(after.In(location)).UTC(), nil
}

func normalizeReportRecipients(raw string) ([]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' ' })
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		address, err := mail.ParseAddress(strings.TrimSpace(part))
		if err != nil || address.Address != strings.TrimSpace(part) {
			return nil, fmt.Errorf("recipients contains an invalid email address")
		}
		value := strings.ToLower(address.Address)
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("recipients is required")
	}
	sort.Strings(out)
	return out, nil
}

func cloneSortedStrings(input []string) []string {
	out := append([]string(nil), input...)
	for index := range out {
		out[index] = strings.TrimSpace(out[index])
	}
	sort.Strings(out)
	return out
}

func validateReportIdempotencyKey(value string) error {
	if len(value) < 1 || len(value) > maxReportIdempotencyKeyBytes || strings.TrimSpace(value) != value {
		return reportAPIError(errcode.CodeParamError, "Idempotency-Key must contain 1..128 bytes without surrounding whitespace")
	}
	return nil
}

func configuredReportMaxAttempts() int {
	value := viper.GetInt("reports.worker.max_attempts")
	if value < 1 {
		return defaultReportMaxAttempts
	}
	if value > maximumReportMaxAttempts {
		return maximumReportMaxAttempts
	}
	return value
}

func reportStorageError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return reportAPIError(errcode.CodeNotFound, "report resource not found")
	case errors.Is(err, dal.ErrReportScheduleRevisionConflict):
		return reportAPIError(errcode.CodeOpDenied, "report schedule revision conflict")
	case errors.Is(err, dal.ErrReportScheduleHasActiveWork):
		return reportAPIError(errcode.CodeOpDenied, "report schedule has active work")
	case errors.Is(err, dal.ErrReportRunNotRetryable):
		return reportAPIError(errcode.CodeOpDenied, "report run is not retryable")
	case errors.Is(err, dal.ErrReportIdempotencyConflict):
		return reportAPIError(errcode.CodeOpDenied, "Idempotency-Key was already used for another request")
	default:
		return reportAPIError(errcode.CodeSystemError, "report operation failed")
	}
}

func reportAPIError(code int, message string) error { return errcode.NewWithMessage(code, message) }

func reportMessageID(runID string) string {
	digest := sha256.Sum256([]byte(runID))
	return fmt.Sprintf("<report-%s@aetherlink.local>", hex.EncodeToString(digest[:16]))
}
