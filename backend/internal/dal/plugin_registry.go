package dal

import (
	"context"
	"errors"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// errPluginDBNotInitialized 数据库未就绪统一错误（PHASE-D-D9）。
var errPluginDBNotInitialized = errors.New("db is not initialized")

// isPluginRecordNotFound gorm 未命中判定。
func isPluginRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// PHASE-D-D9 BEGIN 插件注册表 DAL

// GetPluginByName 按唯一名读取。
// tenant-scope: platform-level —— 插件登记为平台级资源（无租户列，SYS_ADMIN 管理面），
// 访问在 service 层按 Authority 守卫，不适用 tenant_id 过滤。
func GetPluginByName(ctx context.Context, name string) (*model.PluginRegistry, error) {
	if global.DB == nil {
		return nil, errPluginDBNotInitialized
	}
	var row model.PluginRegistry
	err := global.DB.WithContext(ctx).
		Table(model.TableNamePluginRegistry).
		Where("name = ?", name).
		Take(&row).Error
	if err != nil {
		if isPluginRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// GetPluginByID 按 ID 读取。
// tenant-scope: platform-level —— 同 GetPluginByName：平台级资源，无租户列可过滤。
func GetPluginByID(ctx context.Context, id string) (*model.PluginRegistry, error) {
	if global.DB == nil {
		return nil, errPluginDBNotInitialized
	}
	var row model.PluginRegistry
	err := global.DB.WithContext(ctx).
		Table(model.TableNamePluginRegistry).
		Where("id = ?", id).
		Take(&row).Error
	if err != nil {
		if isPluginRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// CreatePluginRegistry 新建登记（name 唯一）。
func CreatePluginRegistry(ctx context.Context, row *model.PluginRegistry) error {
	if global.DB == nil {
		return errPluginDBNotInitialized
	}
	return global.DB.WithContext(ctx).Table(model.TableNamePluginRegistry).Create(row).Error
}

// ListPluginRegistries 全量列表（插件数量级小，不分页）。
// tenant-scope: platform-level —— 同 GetPluginByName：平台级资源，无租户列可过滤。
func ListPluginRegistries(ctx context.Context) ([]model.PluginRegistry, error) {
	if global.DB == nil {
		return nil, errPluginDBNotInitialized
	}
	rows := make([]model.PluginRegistry, 0)
	err := global.DB.WithContext(ctx).
		Table(model.TableNamePluginRegistry).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}

// UpdatePluginRegistry 更新版本/描述/状态（token 摘要仅创建时写入，轮换走独立函数）。
func UpdatePluginRegistry(ctx context.Context, row *model.PluginRegistry) error {
	if global.DB == nil {
		return errPluginDBNotInitialized
	}
	return global.DB.WithContext(ctx).Table(model.TableNamePluginRegistry).Save(row).Error
}

// UpdatePluginStatusAndHeartbeat 网关状态回写（online/offline + 心跳 + 可选版本）。
func UpdatePluginStatusAndHeartbeat(ctx context.Context, id, status string, now time.Time, version *string) error {
	if global.DB == nil {
		return errPluginDBNotInitialized
	}
	updates := map[string]interface{}{
		"status":         status,
		"last_heartbeat": now,
		"updated_at":     now,
	}
	if version != nil && *version != "" {
		updates["version"] = *version
	}
	return global.DB.WithContext(ctx).
		Table(model.TableNamePluginRegistry).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdatePluginTokenHash 凭证轮换。
func UpdatePluginTokenHash(ctx context.Context, id, tokenHash string, now time.Time) error {
	if global.DB == nil {
		return errPluginDBNotInitialized
	}
	return global.DB.WithContext(ctx).
		Table(model.TableNamePluginRegistry).
		Where("id = ?", id).
		Updates(map[string]interface{}{"token_hash": tokenHash, "updated_at": now}).Error
}

// DeletePluginRegistry 删除登记。
func DeletePluginRegistry(ctx context.Context, id string) error {
	if global.DB == nil {
		return errPluginDBNotInitialized
	}
	return global.DB.WithContext(ctx).
		Table(model.TableNamePluginRegistry).
		Where("id = ?", id).
		Delete(&model.PluginRegistry{}).Error
}

// PHASE-D-D9 END
