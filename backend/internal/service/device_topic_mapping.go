// 文件用途：维护设备配置到自定义 MQTT topic 的映射服务。
// 核心逻辑：解析路由 ID、校验设备配置归属、防止重复映射，并在写入后失效 Redis topic 缓存。
// 关键注意事项：topic 映射错误会把遥测路由到错误设备，缓存失效和所有权校验是服务契约。
// 重构建议：抽出映射仓储和缓存失效接口，补齐跨租户、重复创建、事务和缓存失败测试。
// device_topic_mapping.go maps device configs to custom MQTT topics.
//
// Purpose: create, list, update, and delete tenant-scoped topic mappings used by protocol integrations and broker routing.
// Core logic: validates claims, parses numeric route IDs, checks device-config ownership, prevents duplicate mappings, and invalidates Redis topic-map cache entries after writes.
// Important notes: stale or cross-tenant mappings can route telemetry to the wrong device, so cache invalidation and ownership checks are part of the service contract.
// Refactor suggestion: wrap cache invalidation and duplicate detection behind a small mapping repository seam.
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type DeviceTopicMapping struct{}

type normalizedCreateDeviceTopicMappingInput struct {
	Name           string
	Direction      string
	SourceTopic    string
	TargetTopic    string
	Priority       int32
	Enabled        bool
	DataIdentifier *string
}

func (*DeviceTopicMapping) CreateDeviceTopicMapping(req *model.CreateDeviceTopicMappingReq, claims *utils.UserClaims) (model.DeviceTopicMapping, error) {
	var mapping model.DeviceTopicMapping

	ctx := context.Background()

	if _, err := loadOwnedDeviceConfig(ctx, req.DeviceConfigID, claims, "device config not found", "device config not owned by current tenant", func(err error) error {
		logrus.Error(err)
		return wrapTopicMappingDBError(err)
	}); err != nil {
		return mapping, err
	}

	normalized, err := normalizeCreateDeviceTopicMappingInput(req)
	if err != nil {
		return mapping, err
	}

	exists, err := dal.TopicMappingExists(ctx, req.DeviceConfigID, normalized.Direction, normalized.SourceTopic, normalized.TargetTopic)
	if err != nil {
		return mapping, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if exists {
		return mapping, errcode.NewWithMessage(errcode.CodeParamError, "topic mapping already exists")
	}

	mapping.DeviceConfigID = req.DeviceConfigID
	mapping.Name = normalized.Name
	mapping.Direction = normalized.Direction
	mapping.SourceTopic = normalized.SourceTopic
	mapping.TargetTopic = normalized.TargetTopic
	mapping.Priority = normalized.Priority
	mapping.Enabled = normalized.Enabled
	mapping.Description = req.Description
	mapping.DataIdentifier = normalized.DataIdentifier
	now := time.Now().UTC()
	mapping.CreatedAt = now
	mapping.UpdatedAt = now

	if err := dal.CreateDeviceTopicMapping(&mapping); err != nil {
		logrus.Error(err)
		return mapping, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	if err := invalidateTopicMappingCache(ctx, req.DeviceConfigID); err != nil {
		logrus.Error(err)
		return mapping, errcode.WithData(errcode.CodeCacheError, map[string]interface{}{
			"cache_error": err.Error(),
		})
	}

	return mapping, nil
}

func normalizeCreateDeviceTopicMappingInput(req *model.CreateDeviceTopicMappingReq) (normalizedCreateDeviceTopicMappingInput, error) {
	normalized := normalizedCreateDeviceTopicMappingInput{
		Name:           strings.TrimSpace(req.Name),
		Direction:      strings.ToLower(strings.TrimSpace(req.Direction)),
		SourceTopic:    strings.TrimSpace(req.SourceTopic),
		TargetTopic:    strings.TrimSpace(req.TargetTopic),
		Priority:       100,
		Enabled:        true,
		DataIdentifier: normalizeOptionalTopicMappingDataIdentifier(req.DataIdentifier),
	}
	if normalized.Name == "" || normalized.SourceTopic == "" || normalized.TargetTopic == "" {
		return normalized, errcode.NewWithMessage(errcode.CodeParamError, "name/source_topic/target_topic cannot be blank")
	}
	if req.Priority != nil {
		normalized.Priority = *req.Priority
	}
	if req.Enabled != nil {
		normalized.Enabled = *req.Enabled
	}
	return normalized, nil
}

func normalizeOptionalTopicMappingDataIdentifier(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

type listResp struct {
	Total int64                      `json:"total"`
	List  []model.DeviceTopicMapping `json:"list"`
}

func (*DeviceTopicMapping) ListDeviceTopicMappings(req *model.ListDeviceTopicMappingReq, claims *utils.UserClaims) (listResp, error) {
	ctx := context.Background()
	var resp listResp

	if _, err := loadOwnedDeviceConfig(ctx, req.DeviceConfigID, claims, "device config not found", "device config not owned by current tenant", func(err error) error {
		logrus.Error(err)
		return wrapTopicMappingDBError(err)
	}); err != nil {
		return resp, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	items, total, err := dal.ListDeviceTopicMappings(ctx, req)
	if err != nil {
		return resp, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	// convert to non-pointer slice for JSON stability
	resp.Total = total
	resp.List = make([]model.DeviceTopicMapping, 0, len(items))
	for _, it := range items {
		resp.List = append(resp.List, *it)
	}
	return resp, nil
}

func (*DeviceTopicMapping) UpdateDeviceTopicMapping(idStr string, req *model.UpdateDeviceTopicMappingReq, claims *utils.UserClaims) (model.DeviceTopicMapping, error) {
	var result model.DeviceTopicMapping
	ctx := context.Background()

	id, exist, err := loadAccessibleDeviceTopicMapping(ctx, idStr, claims)
	if err != nil {
		return result, err
	}

	updateMap := buildDeviceTopicMappingUpdateMap(req)
	if isDeviceTopicMappingNoopUpdate(updateMap) {
		return *exist, nil
	}

	updated, err := saveDeviceTopicMappingUpdate(ctx, id, exist, updateMap)
	if err != nil {
		return result, err
	}
	return *updated, nil
}

func parseDeviceTopicMappingID(idStr string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || id <= 0 {
		return 0, errcode.NewWithMessage(errcode.CodeParamError, "invalid id")
	}
	return id, nil
}

func loadDeviceTopicMappingByID(ctx context.Context, id int64) (*model.DeviceTopicMapping, error) {
	exist, err := dal.GetDeviceTopicMappingByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "topic mapping not found")
		}
		return nil, wrapTopicMappingDBError(err)
	}
	return exist, nil
}

func validateDeviceTopicMappingAccess(ctx context.Context, exist *model.DeviceTopicMapping, claims *utils.UserClaims) error {
	_, err := loadOwnedDeviceConfig(ctx, exist.DeviceConfigID, claims, "", "no permission", wrapTopicMappingDBError)
	return err
}

// Update and delete should surface the same invalid-id, not-found, and no-permission behavior.
func loadAccessibleDeviceTopicMapping(ctx context.Context, idStr string, claims *utils.UserClaims) (int64, *model.DeviceTopicMapping, error) {
	id, err := parseDeviceTopicMappingID(idStr)
	if err != nil {
		return 0, nil, err
	}

	exist, err := loadDeviceTopicMappingByID(ctx, id)
	if err != nil {
		return 0, nil, err
	}

	if err := validateDeviceTopicMappingAccess(ctx, exist, claims); err != nil {
		return 0, nil, err
	}

	return id, exist, nil
}

func loadOwnedDeviceConfig(
	ctx context.Context,
	deviceConfigID string,
	claims *utils.UserClaims,
	notFoundMessage string,
	noPermissionMessage string,
	wrapDBError func(error) error,
) (*model.DeviceConfig, error) {
	q := query.DeviceConfig
	deviceConfig, err := query.DeviceConfig.WithContext(ctx).
		Select(q.ID, q.TenantID).
		Where(q.ID.Eq(deviceConfigID)).
		First()
	if err != nil {
		if notFoundMessage != "" && errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, notFoundMessage)
		}
		return nil, wrapDBError(err)
	}
	if deviceConfig.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, noPermissionMessage)
	}
	return deviceConfig, nil
}

func buildDeviceTopicMappingUpdateMap(req *model.UpdateDeviceTopicMappingReq) map[string]interface{} {
	updateMap := make(map[string]interface{})
	if req.DeviceConfigID != nil {
		updateMap["device_config_id"] = strings.TrimSpace(*req.DeviceConfigID)
	}
	if req.Name != nil {
		updateMap["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Direction != nil {
		updateMap["direction"] = strings.ToLower(strings.TrimSpace(*req.Direction))
	}
	if req.SourceTopic != nil {
		updateMap["source_topic"] = strings.TrimSpace(*req.SourceTopic)
	}
	if req.TargetTopic != nil {
		updateMap["target_topic"] = strings.TrimSpace(*req.TargetTopic)
	}
	if req.Priority != nil {
		updateMap["priority"] = *req.Priority
	}
	if req.Enabled != nil {
		updateMap["enabled"] = *req.Enabled
	}
	if req.Description != nil {
		updateMap["description"] = req.Description
	}
	if req.DataIdentifier != nil {
		dataIdentifier := strings.TrimSpace(*req.DataIdentifier)
		if dataIdentifier != "" {
			updateMap["data_identifier"] = dataIdentifier
		} else {
			// 如果传入空字符串，则设置为 null
			updateMap["data_identifier"] = nil
		}
	}
	updateMap["updated_at"] = time.Now().UTC()

	return updateMap
}

func isDeviceTopicMappingNoopUpdate(updateMap map[string]interface{}) bool {
	return len(updateMap) == 1
}

func saveDeviceTopicMappingUpdate(ctx context.Context, id int64, exist *model.DeviceTopicMapping, updateMap map[string]interface{}) (*model.DeviceTopicMapping, error) {
	if err := dal.UpdateDeviceTopicMappingByID(ctx, id, updateMap); err != nil {
		return nil, wrapTopicMappingDBError(err)
	}

	invalidateUpdatedTopicMappingCaches(ctx, exist, updateMap)

	updated, err := dal.GetDeviceTopicMappingByID(ctx, id)
	if err != nil {
		return nil, wrapTopicMappingDBError(err)
	}
	return updated, nil
}

func invalidateUpdatedTopicMappingCaches(ctx context.Context, exist *model.DeviceTopicMapping, updateMap map[string]interface{}) {
	targetIDs := topicMappingCacheDeviceConfigIDs(exist, updateMap)
	for deviceConfigID := range targetIDs {
		if err := invalidateTopicMappingCache(ctx, deviceConfigID); err != nil {
			logrus.Errorf("invalidate topic mapping cache (%s) failed: %v", deviceConfigID, err)
		}
	}
}

func topicMappingCacheDeviceConfigIDs(exist *model.DeviceTopicMapping, updateMap map[string]interface{}) map[string]struct{} {
	targetIDs := map[string]struct{}{
		exist.DeviceConfigID: {},
	}
	if v, ok := updateMap["device_config_id"]; ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			targetIDs[s] = struct{}{}
		}
	}
	return targetIDs
}

func wrapTopicMappingDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}

func (*DeviceTopicMapping) DeleteDeviceTopicMapping(idStr string, claims *utils.UserClaims) error {
	ctx := context.Background()
	id, exist, err := loadAccessibleDeviceTopicMapping(ctx, idStr, claims)
	if err != nil {
		return err
	}

	if err := dal.DeleteDeviceTopicMappingByID(ctx, id); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if err := invalidateTopicMappingCache(ctx, exist.DeviceConfigID); err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeCacheError, map[string]interface{}{
			"cache_error": err.Error(),
		})
	}
	return nil
}

// 说明：删除设备主题转换缓存
func invalidateTopicMappingCache(ctx context.Context, deviceConfigID string) error {
	if global.REDIS == nil {
		return fmt.Errorf("redis client not initialized")
	}
	keys := []string{
		fmt.Sprintf("tp:topicmap:up:%s", deviceConfigID),
		fmt.Sprintf("tp:topicmap:down:%s", deviceConfigID),
		fmt.Sprintf("tp:topicmap:downrev:%s", deviceConfigID),
	}
	if err := global.REDIS.Del(ctx, keys...).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	return nil
}
