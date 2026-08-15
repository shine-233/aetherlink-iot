package service

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

const deviceTwinDesiredPayloadMax = 9999

type DeviceTwin struct{}

type twinExpectedRow struct {
	key              string
	label            string
	source           string
	desired          interface{}
	status           string
	comparable       bool
	desiredUpdatedAt *time.Time
	desiredExpiresAt *time.Time
	desiredRevision  *string
}

type twinReportedEntry struct {
	value      interface{}
	reportedAt *time.Time
}

func (*DeviceTwin) GetDeviceTwin(deviceID string, claims *utils.UserClaims) (*model.DeviceTwinState, error) {
	deviceInfo, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	if err != nil {
		return nil, err
	}

	telemetryData, err := GroupApp.TelemetryData.GetCurrentTelemetrData(deviceID, claims)
	if err != nil {
		return nil, err
	}

	attributeData, err := GroupApp.AttributeData.GetAttributeDataList(deviceID, claims)
	if err != nil {
		return nil, err
	}

	expectedItems, err := dal.ExpectedDataDal{}.ListPendingByDeviceID(context.Background(), deviceID, deviceInfo.TenantID)
	if err != nil {
		return nil, err
	}

	desiredRows := normalizeTwinExpectedRows(expectedItems)
	telemetryMap := twinTelemetryValueMap(telemetryData)
	attributeMap := twinAttributeValueMap(attributeData)
	rows := buildDeviceTwinRows(desiredRows, telemetryMap, attributeMap)

	matchedCount := 0
	unavailableCount := 0
	deltaCount := 0
	staleDesiredCount := countStaleTwinDesired(expectedItems, time.Now())
	for _, row := range rows {
		if row.Matched {
			matchedCount++
		}
		if row.Comparable && row.Reported == nil {
			unavailableCount++
		}
		if row.Comparable && !row.Matched {
			deltaCount++
		}
	}

	reportedCount := len(twinReportedKeySet(telemetryMap, attributeMap))
	summary := buildDeviceTwinSummary(
		len(rows),
		reportedCount,
		matchedCount,
		deltaCount,
		unavailableCount,
		staleDesiredCount,
	)
	return &model.DeviceTwinState{
		Rows:    rows,
		Summary: summary,
	}, nil
}

func countStaleTwinDesired(items []*model.ExpectedData, now time.Time) int {
	count := 0
	for _, item := range items {
		if item == nil || item.ExpiryTime == nil {
			continue
		}
		if item.ExpiryTime.Before(now) {
			count++
		}
	}
	return count
}

func buildDeviceTwinSummary(
	desiredCount int,
	reportedCount int,
	matchedCount int,
	deltaCount int,
	unavailableCount int,
	staleDesiredCount int,
) model.DeviceTwinSummary {
	summary := model.DeviceTwinSummary{
		DesiredCount:      desiredCount,
		ReportedCount:     reportedCount,
		MatchedCount:      matchedCount,
		DeltaCount:        deltaCount,
		UnavailableCount:  unavailableCount,
		StaleDesiredCount: staleDesiredCount,
		EvidenceBoundary:  "platform_visible_evidence_only",
	}

	switch {
	case desiredCount == 0:
		summary.ConvergenceStatus = "no_desired"
		summary.NextAction = "create_desired_state"
	case staleDesiredCount > 0:
		summary.ConvergenceStatus = "expired_desired"
		summary.NextAction = "review_expired_desired_state"
	case deltaCount > 0:
		summary.ConvergenceStatus = "needs_review"
		summary.NextAction = "compare_delta_before_device_action"
	case unavailableCount > 0:
		summary.ConvergenceStatus = "waiting_reported"
		summary.NextAction = "wait_for_reported_state"
	default:
		summary.ConvergenceStatus = "ready"
		summary.NextAction = "safe_to_continue_after_review"
	}

	return summary
}

func (*DeviceTwin) UpsertDesired(deviceID string, req *model.UpsertDeviceTwinDesiredReq, claims *utils.UserClaims) (*model.ExpectedData, error) {
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(deviceID, claims)
	if err != nil {
		return nil, err
	}
	if len(req.Desired) == 0 {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"desired": "desired is required",
		})
	}

	var desiredValue interface{}
	if err := json.Unmarshal(req.Desired, &desiredValue); err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"desired": err.Error(),
		})
	}
	payload, err := marshalExpectedDataPayloadJSON(map[string]interface{}{
		req.Key: desiredValue,
	})
	if err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"desired": err.Error(),
		})
	}
	if len(payload) > deviceTwinDesiredPayloadMax {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"desired": "desired payload is too large",
		})
	}

	label := req.Key
	expectedData, err := dal.ExpectedDataDal{}.UpsertPendingDesired(context.Background(), &model.ExpectedData{
		ID:         uuid.New(),
		DeviceID:   deviceID,
		SendType:   req.Source,
		Payload:    payload,
		CreatedAt:  time.Now(),
		Status:     expectedDataStatusPending,
		ExpiryTime: req.Expiry,
		Label:      &label,
		TenantID:   deviceInfo.TenantID,
	})
	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return expectedData, nil
}

func normalizeTwinExpectedRows(items []*model.ExpectedData) []twinExpectedRow {
	rows := make([]twinExpectedRow, 0)
	for _, item := range items {
		if item == nil {
			continue
		}

		source := item.SendType
		if source == "" {
			source = "command"
		}
		status := item.Status
		if status == "" {
			status = expectedDataStatusPending
		}
		baseRow := twinExpectedRow{
			source:           source,
			status:           status,
			desiredUpdatedAt: twinTimePointer(item.CreatedAt),
			desiredExpiresAt: twinOptionalTimePointer(item.ExpiryTime),
			desiredRevision:  twinStringPointer(item.ID),
		}
		parsedPayload := parseTwinPayload(item.Payload)

		if (source == "telemetry" || source == "attribute") && parsedPayload != nil {
			if payloadMap, ok := parsedPayload.(map[string]interface{}); ok {
				appendTwinObjectEntries(&rows, payloadMap, baseRow)
				continue
			}
		}

		fallbackKey := twinFallbackExpectedKey(item, len(rows)+1, source)
		baseRow.key = fallbackKey
		baseRow.label = twinExpectedLabel(item, fallbackKey)
		baseRow.desired = parsedPayload
		baseRow.comparable = source != "command"
		rows = append(rows, baseRow)
	}
	return rows
}

func appendTwinObjectEntries(rows *[]twinExpectedRow, payload map[string]interface{}, baseRow twinExpectedRow) {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		row := baseRow
		row.key = key
		row.label = key
		row.desired = payload[key]
		row.comparable = row.source != "command"
		*rows = append(*rows, row)
	}
}

func twinTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func twinOptionalTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	return twinTimePointer(*value)
}

func twinStringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func parseTwinPayload(payload string) interface{} {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return ""
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed
	}

	return trimmed
}

func twinFallbackExpectedKey(item *model.ExpectedData, index int, source string) string {
	if item.Label != nil && *item.Label != "" {
		return *item.Label
	}
	if item.ID != "" {
		return item.ID
	}
	return source + "-" + strconv.Itoa(index)
}

func twinExpectedLabel(item *model.ExpectedData, fallbackKey string) string {
	if item.Label != nil && *item.Label != "" {
		return *item.Label
	}
	return fallbackKey
}

func twinTelemetryValueMap(raw interface{}) map[string]twinReportedEntry {
	result := make(map[string]twinReportedEntry)
	for _, item := range toTwinMapSlice(raw) {
		key, _ := item["key"].(string)
		label, _ := item["label"].(string)
		entry := twinReportedEntry{
			value:      item["value"],
			reportedAt: twinReportedTime(item["ts"]),
		}
		if key != "" {
			result[key] = entry
		}
		if label != "" {
			if _, exists := result[label]; !exists {
				result[label] = entry
			}
		}
	}
	return result
}

func twinAttributeValueMap(raw interface{}) map[string]twinReportedEntry {
	result := make(map[string]twinReportedEntry)
	for _, item := range toTwinMapSlice(raw) {
		key, _ := item["key"].(string)
		if key != "" {
			result[key] = twinReportedEntry{
				value:      item["value"],
				reportedAt: twinReportedTime(item["ts"]),
			}
		}
	}
	return result
}

func twinReportedValue(row twinExpectedRow, telemetryMap, attributeMap map[string]twinReportedEntry) (twinReportedEntry, bool) {
	switch row.source {
	case "telemetry":
		if entry, ok := telemetryMap[row.key]; ok {
			return entry, true
		}
		if entry, ok := telemetryMap[row.label]; ok {
			return entry, true
		}
	case "attribute":
		if entry, ok := attributeMap[row.key]; ok {
			return entry, true
		}
	}
	return twinReportedEntry{}, false
}

func twinReportedKeySet(telemetryMap, attributeMap map[string]twinReportedEntry) map[string]struct{} {
	keys := make(map[string]struct{}, len(telemetryMap)+len(attributeMap))
	for key := range telemetryMap {
		keys[key] = struct{}{}
	}
	for key := range attributeMap {
		keys[key] = struct{}{}
	}
	return keys
}

func twinReportedTime(value interface{}) *time.Time {
	switch typed := value.(type) {
	case time.Time:
		return twinTimePointer(typed)
	case *time.Time:
		return twinOptionalTimePointer(typed)
	case int64:
		return twinTimePointer(time.UnixMilli(typed))
	case int:
		return twinTimePointer(time.UnixMilli(int64(typed)))
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(typed))
		if err == nil {
			return twinTimePointer(parsed)
		}
	}
	return nil
}

func twinLastWriteSource(desiredUpdatedAt, reportedAt *time.Time, hasReported bool) *string {
	if !hasReported {
		if desiredUpdatedAt == nil {
			return nil
		}
		return twinStringPointer("desired")
	}
	if reportedAt == nil {
		return nil
	}
	if desiredUpdatedAt == nil {
		return twinStringPointer("reported")
	}
	if desiredUpdatedAt.Equal(*reportedAt) {
		return nil
	}
	if desiredUpdatedAt.After(*reportedAt) {
		return twinStringPointer("desired")
	}
	return twinStringPointer("reported")
}

func buildDeviceTwinRows(
	desiredRows []twinExpectedRow,
	telemetryMap map[string]twinReportedEntry,
	attributeMap map[string]twinReportedEntry,
) []model.DeviceTwinRow {
	rows := make([]model.DeviceTwinRow, 0, len(desiredRows))
	for _, row := range desiredRows {
		reportedEntry, hasReported := twinReportedValue(row, telemetryMap, attributeMap)
		var reported interface{}
		var reportedAt *time.Time
		if hasReported {
			reported = reportedEntry.value
			reportedAt = reportedEntry.reportedAt
		}
		reportedFresh := reportedAt != nil && (row.desiredUpdatedAt == nil || !reportedAt.Before(*row.desiredUpdatedAt))
		matched := row.comparable && reportedFresh && twinComparableString(row.desired) == twinComparableString(reported)
		rows = append(rows, model.DeviceTwinRow{
			Key:              row.key,
			Label:            row.label,
			Source:           row.source,
			Desired:          row.desired,
			Reported:         reported,
			Comparable:       row.comparable,
			ReportedFresh:    reportedFresh,
			Matched:          matched,
			Status:           row.status,
			DesiredUpdatedAt: row.desiredUpdatedAt,
			DesiredExpiresAt: row.desiredExpiresAt,
			ReportedAt:       reportedAt,
			DesiredRevision:  row.desiredRevision,
			LastWriteSource:  twinLastWriteSource(row.desiredUpdatedAt, reportedAt, hasReported),
		})
	}
	return rows
}

func twinComparableString(value interface{}) string {
	if value == nil {
		return "null"
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func toTwinMapSlice(raw interface{}) []map[string]interface{} {
	switch items := raw.(type) {
	case []map[string]interface{}:
		return items
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if record, ok := item.(map[string]interface{}); ok {
				result = append(result, record)
			}
		}
		return result
	default:
		return []map[string]interface{}{}
	}
}
