package service

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	utils "aetherlink-iot/backend/pkg/utils"
)

const otaSupportBundleFailedSampleLimit = 50

func (o *OTA) CreateOTAUpgradeTask(req *model.CreateOTAUpgradeTaskReq, claims *utils.UserClaims) error {
	pkg, err := ensureOTAPackageAccess(req.OTAUpgradePackageId, claims)
	if err != nil {
		return err
	}
	deviceIDs, err := resolveOTAUpgradeTaskDeviceIDs(req, pkg, claims)
	if err != nil {
		return err
	}
	req.DeviceIdList = deviceIDs
	if err := ensureOTADeviceWriteAccess(req.DeviceIdList, claims); err != nil {
		return err
	}
	if err := applyOTAUpgradeTaskAudit(req, claims, deviceIDs); err != nil {
		return err
	}

	tasks, err := dal.CreateOTAUpgradeTaskWithDetail(req)
	if err == nil {
		go pushOTAUpgradeTaskDetails(o, tasks)
	}
	return err
}

func (*OTA) PreviewOTAUpgradeTask(req *model.PreviewOTAUpgradeTaskReq, claims *utils.UserClaims) (*model.PreviewOTAUpgradeTaskRsp, error) {
	pkg, err := ensureOTAPackageAccess(req.OTAUpgradePackageId, claims)
	if err != nil {
		return nil, err
	}
	maxDevices := resolveOTAUpgradeTaskMaxDevices(req.MaxDevices)
	selection, err := resolveOTAUpgradeTaskFilteredDeviceSelection(
		req.DeviceFilter,
		pkg,
		req.ExcludeDeviceIdList,
		claims,
		maxDevices,
		20,
		false,
	)
	if err != nil {
		return nil, err
	}

	overLimit := selection.selectedCount > maxDevices
	permissionsChecked := false
	if !overLimit && len(selection.selectedIDs) > 0 {
		if err := ensureOTADeviceWriteAccess(selection.selectedIDs, claims); err != nil {
			return nil, err
		}
		permissionsChecked = true
	}

	return &model.PreviewOTAUpgradeTaskRsp{
		TotalMatched:       selection.totalMatched,
		SelectedCount:      selection.selectedCount,
		ExcludedCount:      selection.excludedCount,
		MaxDevices:         maxDevices,
		OverLimit:          overLimit,
		PermissionsChecked: permissionsChecked,
		PreviewDevices:     selection.previewDevices,
	}, nil
}

func (*OTA) DeleteOTAUpgradeTask(id string, claims *utils.UserClaims) error {
	if _, err := ensureOTATaskAccess(id, claims); err != nil {
		return err
	}
	return dal.DeleteOTAUpgradeTask(id)
}

func (*OTA) GetOTAUpgradeTaskListByPage(req *model.GetOTAUpgradeTaskListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if _, err := ensureOTAPackageAccess(req.OTAUpgradePackageId, claims); err != nil {
		return nil, err
	}
	ownerUserID, err := otaTaskOwnerUserIDForClaims(claims)
	if err != nil {
		return nil, err
	}

	total, list, err := dal.GetOtaUpgradeTaskListByPage(req, ownerUserID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}, nil
}

func (*OTA) GetOTAUpgradeTaskDetailListByPage(req *model.GetOTAUpgradeTaskDetailReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if _, err := ensureOTATaskAccess(req.OtaUpgradeTaskId, claims); err != nil {
		return nil, err
	}

	total, list, statistics, err := dal.GetOtaUpgradeTaskDetailListByPage(req)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total":      total,
		"statistics": statistics,
		"list":       list,
	}, nil
}

func (*OTA) GetOTAUpgradeTaskSupportBundle(taskID string, claims *utils.UserClaims) (*model.OTAUpgradeTaskSupportBundle, error) {
	task, err := ensureOTATaskAccess(taskID, claims)
	if err != nil {
		return nil, err
	}

	rows, err := dal.GetOTAUpgradeTaskSupportBundleRows(
		task.ID,
		claims.TenantID,
		claims.Authority == constant.SYS_ADMIN,
		otaSupportBundleFailedSampleLimit,
	)
	if err != nil {
		return nil, err
	}

	failureGroups := normaliseOTAFailureGroups(rows.FailureGroups)
	failedCount := intFromInt64(rows.FailedCount)
	return buildOTAUpgradeTaskSupportBundle(task, rows.FailedRows, rows.Statistics, failureGroups, intFromInt64(rows.TotalRows), failedCount), nil
}

func buildOTAUpgradeTaskSupportBundle(
	task *model.OtaUpgradeTask,
	failedRows []map[string]interface{},
	statistics interface{},
	failureGroups []model.OTAUpgradeTaskFailureGroup,
	totalRows int,
	failedCount int,
) *model.OTAUpgradeTaskSupportBundle {
	failedDevices := make([]model.OTAUpgradeTaskSupportDevice, 0)

	for _, row := range failedRows {
		if int16FromMapValue(row["status"]) != model.OtaUpgradeTaskDetailStatusFailed {
			continue
		}
		reason := strings.TrimSpace(stringFromMapValue(row["status_description"]))
		if reason == "" {
			reason = "No failure reason returned"
		}
		failedDevices = append(failedDevices, model.OTAUpgradeTaskSupportDevice{
			DetailID:       stringFromMapValue(row["id"]),
			DeviceID:       stringFromMapValue(row["device_id"]),
			DeviceNumber:   stringFromMapValue(row["device_number"]),
			Name:           stringFromMapValue(row["name"]),
			CurrentVersion: stringFromMapValue(row["current_version"]),
			TargetVersion:  stringFromMapValue(row["version"]),
			Progress:       row["steps"],
			UpdatedAt:      row["updated_at"],
			FailureReason:  reason,
			ReadyCheckURL:  otaReadyCheckURLFromSupportRow(task.ID, row),
		})
	}

	return &model.OTAUpgradeTaskSupportBundle{
		TaskID:        task.ID,
		TaskName:      task.Name,
		PackageID:     task.OtaUpgradePackageID,
		TargetMode:    task.TargetMode,
		TargetFilter:  task.TargetFilter,
		PreviewTotal:  task.PreviewTotal,
		SelectedCount: task.SelectedCount,
		CreatedAt:     task.CreatedAt,
		GeneratedAt:   time.Now().UTC(),
		Statistics:    statistics,
		TotalRows:     totalRows,
		FailedCount:   failedCount,
		FailedDevices: limitOTASupportDevices(failedDevices, otaSupportBundleFailedSampleLimit),
		FailureGroups: failureGroups,
		NextActions:   otaSupportBundleNextActions(failedCount, totalRows),
		EvidenceBoundary: []string{
			"This bundle is generated from persisted OTA task-detail rows for the requested task.",
			"Failure reasons come from status_description values reported by the platform/device pipeline; they are triage evidence, not proven root cause.",
			"Ready Check links are device diagnostics entrypoints and do not prove a device is physically upgraded.",
			"Retry or cancel device-level OTA rows only after confirming package compatibility, connectivity, and device state.",
		},
		ShareHint: "Share this OTA support bundle with support or field engineers when a rollout has failed devices.",
	}
}

func normaliseOTAFailureGroups(groups []model.OTAUpgradeTaskFailureGroup) []model.OTAUpgradeTaskFailureGroup {
	counts := map[string]int{}
	for _, group := range groups {
		reason := strings.TrimSpace(group.Reason)
		if reason == "" {
			reason = "No failure reason returned"
		}
		counts[reason] += group.Count
	}

	result := make([]model.OTAUpgradeTaskFailureGroup, 0, len(counts))
	for reason, count := range counts {
		result = append(result, model.OTAUpgradeTaskFailureGroup{Reason: reason, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count == result[j].Count {
			return result[i].Reason < result[j].Reason
		}
		return result[i].Count > result[j].Count
	})
	return result
}

func intFromInt64(value int64) int {
	maxInt := int64(^uint(0) >> 1)
	if value > maxInt {
		return int(maxInt)
	}
	if value < 0 {
		return 0
	}
	return int(value)
}

func limitOTASupportDevices(devices []model.OTAUpgradeTaskSupportDevice, limit int) []model.OTAUpgradeTaskSupportDevice {
	if limit <= 0 || len(devices) <= limit {
		return devices
	}
	return devices[:limit]
}

func otaReadyCheckURLFromSupportRow(taskID string, row map[string]interface{}) string {
	deviceID := stringFromMapValue(row["device_id"])
	detailID := stringFromMapValue(row["id"])
	if deviceID == "" || detailID == "" {
		return ""
	}
	return fmt.Sprintf(
		"/device/details?d_id=%s&tab=ready-check&source=ota&ota_task_id=%s&ota_detail_id=%s",
		deviceID,
		taskID,
		detailID,
	)
}

func otaSupportBundleNextActions(failedCount int, totalRows int) []string {
	actions := []string{"Refresh the OTA task detail before making retry or cancel decisions."}
	if failedCount > 0 {
		actions = append(actions, "Group failed devices by reason and inspect Ready Check evidence for representative devices.")
		actions = append(actions, "Confirm package compatibility, current version, connectivity, and broker/API reachability before retrying.")
	}
	if totalRows == 0 {
		actions = append(actions, "No task detail rows were returned; confirm the task id and permissions before treating this bundle as complete.")
	}
	return actions
}

func stringFromMapValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func int16FromMapValue(value interface{}) int16 {
	parse := func(raw string) int16 {
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 16)
		if err != nil {
			return 0
		}
		return int16(parsed)
	}

	switch typed := value.(type) {
	case int16:
		return typed
	case int:
		return parse(strconv.FormatInt(int64(typed), 10))
	case int32:
		return parse(strconv.FormatInt(int64(typed), 10))
	case int64:
		return parse(strconv.FormatInt(typed, 10))
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed {
			return 0
		}
		return parse(strconv.FormatFloat(typed, 'f', 0, 64))
	case string:
		return parse(typed)
	default:
		return 0
	}
}
