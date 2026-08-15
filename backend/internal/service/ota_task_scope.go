package service

import (
	"encoding/json"
	"fmt"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

func resolveOTAUpgradeTaskDeviceIDs(req *model.CreateOTAUpgradeTaskReq, pkg *model.OtaUpgradePackage, claims *utils.UserClaims) ([]string, error) {
	hasExplicitDevices := len(req.DeviceIdList) > 0
	hasFilter := req.DeviceFilter != nil

	if hasExplicitDevices && hasFilter {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id_list and device_filter cannot be used together")
	}
	if !hasExplicitDevices && !hasFilter {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_id_list or device_filter is required")
	}
	if hasExplicitDevices {
		return uniqueNonEmptyStrings(req.DeviceIdList), nil
	}
	return resolveOTAUpgradeTaskFilteredDeviceIDs(req, pkg, claims)
}

func applyOTAUpgradeTaskAudit(req *model.CreateOTAUpgradeTaskReq, claims *utils.UserClaims, deviceIDs []string) error {
	targetMode := "explicit"
	if req.DeviceFilter != nil {
		targetMode = "filter"
		filterSnapshot, err := json.Marshal(req.DeviceFilter)
		if err != nil {
			return errcode.NewWithMessage(errcode.CodeParamError, "device_filter cannot be recorded")
		}
		snapshot := string(filterSnapshot)
		req.TargetFilter = &snapshot
	}

	selectedCount := len(deviceIDs)
	req.TargetMode = targetMode
	req.SelectedCount = &selectedCount
	if req.ExpectedTotal != nil {
		req.PreviewTotal = req.ExpectedTotal
	} else {
		previewTotal := int64(selectedCount)
		req.PreviewTotal = &previewTotal
	}
	if claims != nil {
		req.CreatedBy = &claims.ID
		req.CreatedByAuthority = &claims.Authority
	}
	return nil
}

func resolveOTAUpgradeTaskFilteredDeviceIDs(req *model.CreateOTAUpgradeTaskReq, pkg *model.OtaUpgradePackage, claims *utils.UserClaims) ([]string, error) {
	maxDevices := resolveOTAUpgradeTaskMaxDevices(req.MaxDevices)
	selection, err := resolveOTAUpgradeTaskFilteredDeviceSelection(
		req.DeviceFilter,
		pkg,
		req.ExcludeDeviceIdList,
		claims,
		maxDevices,
		0,
		true,
	)
	if err != nil {
		return nil, err
	}
	if req.ExpectedTotal != nil && *req.ExpectedTotal != selection.totalMatched {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_filter result changed, refresh the device list before creating ota task")
	}

	if selection.selectedCount > maxDevices {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, fmt.Sprintf("device_filter matched %d devices, max_devices is %d", selection.selectedCount, maxDevices))
	}
	if selection.selectedCount == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_filter matched no devices")
	}
	return selection.selectedIDs, nil
}

type otaFilteredDeviceSelection struct {
	totalMatched   int64
	selectedCount  int
	excludedCount  int
	selectedIDs    []string
	previewDevices []model.GetDeviceListByPageRsp
}

func resolveOTAUpgradeTaskFilteredDeviceSelection(
	filter *model.OTAUpgradeTaskDeviceFilter,
	pkg *model.OtaUpgradePackage,
	excludeDeviceIDs []string,
	claims *utils.UserClaims,
	maxDevices int,
	previewLimit int,
	requireAllSelectedIDs bool,
) (*otaFilteredDeviceSelection, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to create ota task")
	}
	tenantID := claims.TenantID
	if claims.Authority == constant.SYS_ADMIN && pkg.TenantID != nil && strings.TrimSpace(*pkg.TenantID) != "" {
		tenantID = *pkg.TenantID
	}

	deviceListReq, err := buildOTAFilteredDeviceListReq(filter, pkg)
	if err != nil {
		return nil, err
	}
	applyDeviceListOwnerFilterForClaims(deviceListReq, claims)
	total, err := dal.CountDeviceListByFilter(deviceListReq, tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	excludedIDs := uniqueNonEmptyStrings(excludeDeviceIDs)
	excludedCount, err := dal.CountDeviceListFilteredIDs(deviceListReq, tenantID, excludedIDs)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	selectedCount64 := total - excludedCount
	if selectedCount64 < 0 {
		selectedCount64 = 0
	}
	selection := &otaFilteredDeviceSelection{
		totalMatched:  total,
		selectedCount: safeIntFromInt64(selectedCount64),
		excludedCount: safeIntFromInt64(excludedCount),
		selectedIDs:   []string{},
	}
	if selectedCount64 == 0 {
		return selection, nil
	}

	needsAllIDs := requireAllSelectedIDs || selectedCount64 <= int64(maxDevices)
	if needsAllIDs || previewLimit > 0 {
		scanLimit := otaFilteredIDScanLimit(selectedCount64, excludedCount, previewLimit, needsAllIDs)
		scannedIDs, err := dal.ListDeviceIDsByFilter(deviceListReq, tenantID, scanLimit)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
		selection.selectedIDs = filterOTADeviceIDs(scannedIDs, excludedIDs)
	}

	if previewLimit > 0 && len(selection.selectedIDs) > 0 {
		previewIDs := limitStrings(selection.selectedIDs, previewLimit)
		selection.previewDevices, err = dal.GetDeviceListRowsByFilterAndIDs(deviceListReq, tenantID, previewIDs)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"sql_error": err.Error(),
			})
		}
	}

	return selection, nil
}

func buildOTAFilteredDeviceListReq(filter *model.OTAUpgradeTaskDeviceFilter, pkg *model.OtaUpgradePackage) (*model.GetDeviceListByPageReq, error) {
	if filter == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_filter is required")
	}
	if filter.DeviceConfigId != nil && strings.TrimSpace(*filter.DeviceConfigId) != "" && strings.TrimSpace(*filter.DeviceConfigId) != pkg.DeviceConfigID {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_filter.device_config_id must match ota package device_config_id")
	}
	deviceConfigID := pkg.DeviceConfigID
	return &model.GetDeviceListByPageReq{
		// OTA filter tasks must resolve the complete matching fleet, not the
		// current device-table page. The DAL treats zero pagination as unpaged.
		PageReq: model.PageReq{
			Page:     0,
			PageSize: 0,
		},
		DeviceNumber:       filter.DeviceNumber,
		IsEnabled:          filter.IsEnabled,
		ProductID:          filter.ProductID,
		Label:              filter.Label,
		Name:               filter.Name,
		CurrentVersion:     filter.CurrentVersion,
		PIDNumber:          filter.PIDNumber,
		FirmwareVersion:    filter.FirmwareVersion,
		Description:        filter.Description,
		SharedStatus:       filter.SharedStatus,
		GroupId:            filter.GroupId,
		DeviceConfigId:     &deviceConfigID,
		DeviceTemplateID:   filter.DeviceTemplateID,
		IsOnline:           filter.IsOnline,
		WarnStatus:         filter.WarnStatus,
		Search:             filter.Search,
		AccessWay:          filter.AccessWay,
		BatchNumber:        filter.BatchNumber,
		DeviceType:         filter.DeviceType,
		ServiceIdentifier:  filter.ServiceIdentifier,
		ServiceAccessID:    filter.ServiceAccessID,
		LastReportedAfter:  filter.LastReportedAfter,
		LastReportedBefore: filter.LastReportedBefore,
		NeverReported:      filter.NeverReported,
		LifecycleStatus:    filter.LifecycleStatus,
	}, nil
}

func filteredOTADeviceIDs(devices []model.GetDeviceListByPageRsp, excludeIDs []string) []string {
	deviceIDs := make([]string, 0, len(devices))
	for _, device := range devices {
		deviceIDs = append(deviceIDs, device.ID)
	}
	return filterOTADeviceIDs(deviceIDs, excludeIDs)
}

func filterOTADeviceIDs(deviceIDs []string, excludeIDs []string) []string {
	excluded := map[string]struct{}{}
	for _, id := range uniqueNonEmptyStrings(excludeIDs) {
		excluded[id] = struct{}{}
	}
	ids := make([]string, 0, len(deviceIDs))
	seen := map[string]struct{}{}
	for _, deviceID := range deviceIDs {
		id := strings.TrimSpace(deviceID)
		if id == "" {
			continue
		}
		if _, ok := excluded[id]; ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func otaFilteredIDScanLimit(selectedCount int64, excludedCount int64, previewLimit int, needsAllIDs bool) int {
	if selectedCount <= 0 {
		return 0
	}
	if needsAllIDs {
		return safeIntFromInt64(selectedCount + excludedCount)
	}
	if previewLimit <= 0 {
		return 0
	}
	return safeIntFromInt64(int64(previewLimit) + excludedCount)
}

func limitStrings(values []string, limit int) []string {
	if limit <= 0 {
		return []string{}
	}
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func safeIntFromInt64(value int64) int {
	if value <= 0 {
		return 0
	}
	maxInt := int64(^uint(0) >> 1)
	if value > maxInt {
		return int(maxInt)
	}
	return int(value)
}

func uniqueNonEmptyStrings(values []string) []string {
	ids := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func resolveOTAUpgradeTaskMaxDevices(maxDevices *int) int {
	const defaultMaxDevices = 5000
	if maxDevices == nil || *maxDevices <= 0 || *maxDevices > defaultMaxDevices {
		return defaultMaxDevices
	}
	return *maxDevices
}

func previewOTADevices(devices []model.GetDeviceListByPageRsp, selectedDeviceIDs []string, limit int) []model.GetDeviceListByPageRsp {
	if limit <= 0 {
		return []model.GetDeviceListByPageRsp{}
	}
	selected := map[string]struct{}{}
	for _, id := range selectedDeviceIDs {
		selected[id] = struct{}{}
	}
	preview := make([]model.GetDeviceListByPageRsp, 0, limit)
	for _, device := range devices {
		if _, ok := selected[device.ID]; !ok {
			continue
		}
		preview = append(preview, device)
		if len(preview) >= limit {
			break
		}
	}
	return preview
}
