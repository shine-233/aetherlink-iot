// 文件用途：实体版本控制服务层（ROADMAP C7），提供实体快照、版本历史与按版本恢复。
// 核心逻辑：entity_type 经白名单映射为固定表名后，按 (id, tenant_id) 读取整行并序列化为
// 快照；恢复时反序列化为字段 map，剔除不可变列后在同一租户作用域内回写。
// 关键注意事项：
//   1) 表名只来自 entityTypeTables 白名单常量，用户输入永不参与 SQL 拼接，杜绝注入与越表访问；
//   2) 所有读写强制 WHERE id = ? AND tenant_id = ?，claims 缺失或租户为空一律拒绝；
//   3) 恢复时剔除 id/tenant_id/created_at 等不可变列，防止跨租户迁移或主键篡改；
//   4) DryRun 只回显将写入的字段，不落库，便于恢复前确认。
// 重构建议：若后续支持差异对比或自动快照（实体变更触发），在此扩展，但不要放开表名白名单。
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

// entityTypeTables 实体类型到物理表名的白名单映射。
// 仅登记已确认存在的表；新增类型必须在此显式登记，且目标表须含 id 与 tenant_id 列。
var entityTypeTables = map[string]string{
	"board":            model.TableNameBoard,
	"rule_chain":       model.TableNameRuleChain,
	"device_config":    "device_configs",
	"calculated_field": model.TableNameCalculatedField,
}

// entityVersionImmutableColumns 恢复时必须剔除的不可变列：
// id 为主键，tenant_id 为隔离边界，created_at 为审计时间，均不允许被快照覆盖。
var entityVersionImmutableColumns = map[string]struct{}{
	"id":         {},
	"tenant_id":  {},
	"created_at": {},
}

// EntityVersionService 实体版本控制业务入口。
type EntityVersionService struct{}

// entityVersionScope 提取租户作用域；claims 缺失或租户为空一律拒绝。
func entityVersionScope(claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage entity versions")
	}
	tenantID := strings.TrimSpace(claims.TenantID)
	if tenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required to manage entity versions")
	}
	return tenantID, nil
}

// resolveEntityTable 将实体类型解析为白名单表名；未知类型报参数错误并回显可选值。
func resolveEntityTable(entityType string) (string, error) {
	table, ok := entityTypeTables[strings.TrimSpace(entityType)]
	if !ok {
		return "", errcode.NewWithMessage(
			errcode.CodeParamError,
			"unsupported entity_type, allowed values: board, rule_chain, device_config, calculated_field",
		)
	}
	return table, nil
}

// readEntityRow 按 id + 租户读取实体整行为 map；不存在返回 gorm.ErrRecordNotFound。
func readEntityRow(tenantID, table, entityID string) (map[string]interface{}, error) {
	var row map[string]interface{}
	err := global.DB.WithContext(context.Background()).
		Table(table).
		Where("id = ? AND tenant_id = ?", entityID, tenantID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return row, nil
}

// CreateEntityVersion 读取实体当前状态并落一条新快照；版本号在同一实体内递增。
func (*EntityVersionService) CreateEntityVersion(req *model.EntityVersionCreateReq, claims *utils.UserClaims) (*model.EntityVersion, error) {
	tenantID, err := entityVersionScope(claims)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "request body is required")
	}

	table, err := resolveEntityTable(req.EntityType)
	if err != nil {
		return nil, err
	}
	entityID := strings.TrimSpace(req.EntityID)
	if entityID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "entity_id is required")
	}

	row, err := readEntityRow(tenantID, table, entityID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "entity version target not found")
		}
		return nil, err
	}

	raw, err := json.Marshal(row)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "failed to serialize entity snapshot")
	}

	nextNumber, err := dal.GetMaxEntityVersionNumber(tenantID, strings.TrimSpace(req.EntityType), entityID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	version := &model.EntityVersion{
		ID:            uuid.New(),
		TenantID:      tenantID,
		EntityType:    strings.TrimSpace(req.EntityType),
		EntityID:      entityID,
		VersionNumber: nextNumber + 1,
		Snapshot:      string(raw),
		Remark:        req.Remark,
		CreatedAt:     now,
	}
	if claims != nil && strings.TrimSpace(claims.ID) != "" {
		userID := strings.TrimSpace(claims.ID)
		version.CreatedBy = &userID
	}

	if err := dal.CreateEntityVersion(version); err != nil {
		return nil, err
	}
	return version, nil
}

// ListEntityVersions 分页返回某实体的版本历史（按版本号倒序）。
func (*EntityVersionService) ListEntityVersions(req *model.EntityVersionListReq, claims *utils.UserClaims) (*model.EntityVersionListRsp, error) {
	tenantID, err := entityVersionScope(claims)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "entity_type and entity_id are required")
	}
	if _, err := resolveEntityTable(req.EntityType); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.EntityID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "entity_id is required")
	}

	list, total, err := dal.GetEntityVersionsForScope(
		tenantID,
		strings.TrimSpace(req.EntityType),
		strings.TrimSpace(req.EntityID),
		req.Page, req.PageSize,
	)
	if err != nil {
		return nil, err
	}
	return &model.EntityVersionListRsp{Total: total, List: list}, nil
}

// GetEntityVersion 按 id + 租户查询单个版本详情。
func (*EntityVersionService) GetEntityVersion(versionID string, claims *utils.UserClaims) (*model.EntityVersion, error) {
	tenantID, err := entityVersionScope(claims)
	if err != nil {
		return nil, err
	}
	version, err := dal.GetEntityVersionForScope(strings.TrimSpace(versionID), tenantID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "entity version not found")
		}
		return nil, err
	}
	return version, nil
}

// RestoreEntityVersion 将某版本的快照回写到实体；DryRun 为真时只返回将写入的字段。
// 返回 (将写入/已写入的字段 map, 是否 dry run, error)。
func (*EntityVersionService) RestoreEntityVersion(versionID string, req *model.EntityVersionRestoreReq, claims *utils.UserClaims) (map[string]interface{}, bool, error) {
	tenantID, err := entityVersionScope(claims)
	if err != nil {
		return nil, false, err
	}

	version, err := dal.GetEntityVersionForScope(strings.TrimSpace(versionID), tenantID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, errcode.NewWithMessage(errcode.CodeNotFound, "entity version not found")
		}
		return nil, false, err
	}

	table, err := resolveEntityTable(version.EntityType)
	if err != nil {
		return nil, false, err
	}

	// 恢复前确认目标实体仍存在且仍属于当前租户，避免把快照写进被删除或已迁移的实体。
	if _, err := readEntityRow(tenantID, table, version.EntityID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, errcode.NewWithMessage(errcode.CodeNotFound, "entity version target not found")
		}
		return nil, false, err
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(version.Snapshot), &payload); err != nil {
		return nil, false, errcode.NewWithMessage(errcode.CodeParamError, "failed to parse entity snapshot")
	}
	for column := range entityVersionImmutableColumns {
		delete(payload, column)
	}
	if len(payload) == 0 {
		return nil, false, errcode.NewWithMessage(errcode.CodeParamError, "entity snapshot contains no restorable fields")
	}

	dryRun := req != nil && req.DryRun != nil && *req.DryRun
	if dryRun {
		return payload, true, nil
	}

	result := global.DB.WithContext(context.Background()).
		Table(table).
		Where("id = ? AND tenant_id = ?", version.EntityID, tenantID).
		Updates(payload)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, errcode.NewWithMessage(errcode.CodeNotFound, "entity version target not found")
	}
	return payload, false, nil
}
