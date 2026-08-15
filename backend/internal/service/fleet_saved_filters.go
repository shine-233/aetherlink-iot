package service

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

type FleetSavedFilter struct{}

const maxFleetSavedFilters = 50

// fleetSavedFilterListLimit 覆盖本人配额加上同租户共享进来的筛选器，因此比
// 单用户配额宽松；本人的保存上限仍然是 maxFleetSavedFilters。
const fleetSavedFilterListLimit = maxFleetSavedFilters * 4

const fleetSavedFilterNotOwnedMessage = "saved filter is shared read-only by another member"

var fleetSavedFilterAllowedKeys = map[string]struct{}{
	"access_way":           {},
	"batch_number":         {},
	"current_version":      {},
	"description":          {},
	"device_config_id":     {},
	"device_number":        {},
	"device_template_id":   {},
	"device_type":          {},
	"firmware_version":     {},
	"group_id":             {},
	"is_enabled":           {},
	"is_online":            {},
	"label":                {},
	"last_reported_after":  {},
	"last_reported_before": {},
	"lifecycle_status":     {},
	"name":                 {},
	"never_reported":       {},
	"pid_number":           {},
	"product_id":           {},
	"search":               {},
	"service_access_id":    {},
	"service_identifier":   {},
	"shared_status":        {},
	"warn_status":          {},
}

func (*FleetSavedFilter) Create(req *model.FleetSavedFilterReq, claims *utils.UserClaims) (*model.FleetSavedFilterRsp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "claims are required")
	}
	normalized, err := normalizeFleetSavedFilterReq(req)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	owned, err := dal.CountFleetSavedFiltersOwnedByUser(claims.TenantID, claims.ID)
	if err != nil {
		return nil, err
	}
	if err := checkFleetSavedFilterQuota(owned); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	filter := &model.FleetSavedFilter{
		ID:           uuid.New(),
		TenantID:     claims.TenantID,
		UserID:       claims.ID,
		Name:         strings.TrimSpace(req.Name),
		DeviceFilter: string(raw),
		PreviewTotal: req.PreviewTotal,
		Shared:       fleetSavedFilterSharedFlag(req.Shared, false),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := dal.CreateFleetSavedFilter(filter); err != nil {
		return nil, fleetSavedFilterWriteError(err)
	}
	return fleetSavedFilterRsp(filter, claims.ID), nil
}

func (*FleetSavedFilter) Update(req *model.FleetSavedFilterReq, claims *utils.UserClaims) (*model.FleetSavedFilterRsp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "claims are required")
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "saved filter request is required")
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "saved filter id is required")
	}
	filter, err := dal.GetFleetSavedFilterInTenant(req.ID, claims.TenantID)
	if err != nil {
		return nil, err
	}
	if err := authorizeFleetSavedFilterWrite(filter, claims); err != nil {
		return nil, err
	}
	normalized, err := normalizeFleetSavedFilterReq(req)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	filter.Name = strings.TrimSpace(req.Name)
	filter.DeviceFilter = string(raw)
	filter.PreviewTotal = req.PreviewTotal
	filter.Shared = fleetSavedFilterSharedFlag(req.Shared, filter.Shared)
	filter.UpdatedAt = time.Now().UTC()
	if err := dal.UpdateFleetSavedFilter(filter); err != nil {
		return nil, fleetSavedFilterWriteError(err)
	}
	return fleetSavedFilterRsp(filter, claims.ID), nil
}

func (*FleetSavedFilter) Delete(id string, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "claims are required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "saved filter id is required")
	}
	filter, err := dal.GetFleetSavedFilterInTenant(id, claims.TenantID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 幂等删除：租户内不存在这条记录时保持原有的成功语义。
			return nil
		}
		return err
	}
	if err := authorizeFleetSavedFilterWrite(filter, claims); err != nil {
		return err
	}
	return dal.DeleteFleetSavedFilter(id, claims.TenantID, claims.ID)
}

func (*FleetSavedFilter) List(claims *utils.UserClaims) (*model.FleetSavedFilterListRsp, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "claims are required")
	}
	filters, err := dal.ListFleetSavedFiltersVisibleToUser(claims.TenantID, claims.ID, fleetSavedFilterListLimit)
	if err != nil {
		return nil, err
	}
	return &model.FleetSavedFilterListRsp{
		List: buildFleetSavedFilterList(filters, claims.ID),
	}, nil
}

// buildFleetSavedFilterList 把可见记录整理成响应列表：先本人拥有的，再别人
// 共享的，同组内按更新时间倒序，让前端不必自己区分可编辑与只读。
func buildFleetSavedFilterList(filters []*model.FleetSavedFilter, userID string) []model.FleetSavedFilterRsp {
	list := make([]model.FleetSavedFilterRsp, 0, len(filters))
	for _, filter := range filters {
		if filter == nil {
			continue
		}
		if !fleetSavedFilterVisibleToUser(filter, userID) {
			continue
		}
		list = append(list, *fleetSavedFilterRsp(filter, userID))
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Owned != list[j].Owned {
			return list[i].Owned
		}
		left, right := list[i].UpdatedAt, list[j].UpdatedAt
		if left == nil || right == nil {
			return right == nil && left != nil
		}
		return left.After(*right)
	})
	return list
}

// fleetSavedFilterVisibleToUser 是读可见性规则：本人拥有的总是可见，其他人的
// 只有显式共享后才可见。调用方必须先按租户过滤。
func fleetSavedFilterVisibleToUser(filter *model.FleetSavedFilter, userID string) bool {
	if filter == nil {
		return false
	}
	if filter.UserID == userID {
		return true
	}
	return filter.Shared
}

// authorizeFleetSavedFilterWrite 是写权限规则：只有所有者能改删，共享出来的
// 筛选器对其他成员是只读的。
func authorizeFleetSavedFilterWrite(filter *model.FleetSavedFilter, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "claims are required")
	}
	if filter == nil {
		return errcode.NewWithMessage(errcode.CodeNotFound, "saved filter not found")
	}
	if filter.TenantID != claims.TenantID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, fleetSavedFilterNotOwnedMessage)
	}
	if filter.UserID != claims.ID {
		return errcode.NewWithMessage(errcode.CodeNoPermission, fleetSavedFilterNotOwnedMessage)
	}
	return nil
}

// checkFleetSavedFilterQuota 只接收本人拥有的记录数，共享进来的筛选器不占配额。
func checkFleetSavedFilterQuota(ownedCount int64) error {
	if ownedCount >= maxFleetSavedFilters {
		return errcode.NewWithMessage(errcode.CodeParamError, "saved filter limit reached")
	}
	return nil
}

func fleetSavedFilterSharedFlag(requested *bool, current bool) bool {
	if requested == nil {
		return current
	}
	return *requested
}

func normalizeFleetSavedFilterReq(req *model.FleetSavedFilterReq) (map[string]interface{}, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "saved filter request is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "saved filter name is required")
	}
	filter := normalizeFleetSavedFilterParams(req.DeviceFilter)
	if len(filter) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device_filter must contain at least one supported filter field")
	}
	if err := validateFleetSavedFilterLifecycle(filter); err != nil {
		return nil, err
	}
	if err := validateFleetSavedFilterLastReport(filter); err != nil {
		return nil, err
	}
	return filter, nil
}

func normalizeFleetSavedFilterParams(params map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for key, value := range params {
		if _, ok := fleetSavedFilterAllowedKeys[key]; !ok {
			continue
		}
		if key == "never_reported" {
			if typed, ok := value.(bool); ok {
				result[key] = typed
			}
			continue
		}
		if key == "last_reported_after" || key == "last_reported_before" {
			switch typed := value.(type) {
			case float64:
				if typed > 0 && typed == math.Trunc(typed) {
					result[key] = typed
				}
			case int:
				if typed > 0 {
					result[key] = typed
				}
			case int64:
				if typed > 0 {
					result[key] = typed
				}
			}
			continue
		}
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				result[key] = trimmed
			}
		case float64:
			result[key] = typed
		case int:
			result[key] = typed
		case bool:
			result[key] = typed
		}
	}
	return result
}

func validateFleetSavedFilterLifecycle(filter map[string]interface{}) error {
	value, exists := filter["lifecycle_status"]
	if !exists {
		return nil
	}
	status, ok := value.(string)
	if !ok {
		return errcode.NewWithMessage(errcode.CodeParamError, "lifecycle_status is invalid")
	}
	switch status {
	case "activated", "inactive", "transmitted", "all":
		return nil
	default:
		return errcode.NewWithMessage(errcode.CodeParamError, "lifecycle_status is invalid")
	}
}

func validateFleetSavedFilterLastReport(filter map[string]interface{}) error {
	_, hasAfter := filter["last_reported_after"]
	_, hasBefore := filter["last_reported_before"]
	if neverReported, ok := filter["never_reported"].(bool); ok && neverReported && (hasAfter || hasBefore) {
		return errcode.NewWithMessage(
			errcode.CodeParamError,
			"never_reported cannot be combined with a last reported range",
		)
	}
	if hasAfter && hasBefore && fleetSavedFilterTimestamp(filter["last_reported_after"]) >= fleetSavedFilterTimestamp(filter["last_reported_before"]) {
		return errcode.NewWithMessage(errcode.CodeParamError, "last reported range is invalid")
	}
	return nil
}

func fleetSavedFilterTimestamp(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func fleetSavedFilterRsp(filter *model.FleetSavedFilter, viewerUserID string) *model.FleetSavedFilterRsp {
	if filter == nil {
		return nil
	}
	deviceFilter := map[string]interface{}{}
	_ = json.Unmarshal([]byte(filter.DeviceFilter), &deviceFilter)
	createdAt := filter.CreatedAt
	updatedAt := filter.UpdatedAt
	return &model.FleetSavedFilterRsp{
		ID:           filter.ID,
		Name:         filter.Name,
		DeviceFilter: deviceFilter,
		PreviewTotal: filter.PreviewTotal,
		Shared:       filter.Shared,
		Owned:        filter.UserID == viewerUserID,
		OwnerUserID:  filter.UserID,
		CreatedAt:    &createdAt,
		UpdatedAt:    &updatedAt,
	}
}

func fleetSavedFilterWriteError(err error) error {
	if isFleetSavedFilterDuplicateNameError(err) {
		return errcode.NewWithMessage(errcode.CodeParamError, "saved filter name already exists")
	}
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

func isFleetSavedFilterDuplicateNameError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "SQLSTATE 23505") &&
		strings.Contains(message, "fleet_saved_filters_user_name_unique")
}
