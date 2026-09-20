// 文件用途：租户数据访问层（DAL）——租户目录、配额统计与层级查询。
package dal

import (
	"errors"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

var errTenantDBNotInitialized = errors.New("db is not initialized")

// CountAllTenants 统计系统内全部租户总数（供商业许可证 max_tenants 配额检查）。
func CountAllTenants() (int64, error) {
	if global.DB == nil {
		return 0, errTenantDBNotInitialized
	}
	var count int64
	err := global.DB.Table(model.TableNameTenant).Count(&count).Error
	return count, err
}

// CreateTenant 插入新租户。
func CreateTenant(t *model.Tenant) error {
	if global.DB == nil {
		return errTenantDBNotInitialized
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}
	return global.DB.Table(model.TableNameTenant).Create(t).Error
}

// GetTenantByID 按 ID 获取租户信息。
// tenant-scope: caller-enforced —— 调用方（服务层）负责可见性裁决：SYS_ADMIN 全局、
// TENANT_ADMIN 仅 self+子孙；自助开通探测路径尚无租户上下文，按 ID 精确读取。
func GetTenantByID(id string) (*model.Tenant, error) {
	if global.DB == nil {
		return nil, errTenantDBNotInitialized
	}
	var t model.Tenant
	err := global.DB.Table(model.TableNameTenant).Where("id = ?", strings.TrimSpace(id)).Take(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// GetTenantByName 按租户名称获取租户信息。
// tenant-scope: caller-enforced —— 跨租户全局重名校验专用（开通/创建前置），刻意不带租户过滤。
func GetTenantByName(name string) (*model.Tenant, error) {
	if global.DB == nil {
		return nil, errTenantDBNotInitialized
	}
	var t model.Tenant
	err := global.DB.Table(model.TableNameTenant).Where("name = ?", strings.TrimSpace(name)).Take(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// ListTenants 分页查询租户列表。
// tenant-scope: caller-enforced —— 可见范围由服务层解析层级后经 tenantIDs 传入，
// DAL 不再叠加租户过滤（SYS_ADMIN 传空=全平台）。
// tenantIDs 为空表示平台级查询全部；非空表示租户作用域（self + 下级子租户）。
func ListTenants(offset, limit int, search string, tenantIDs []string) ([]*model.Tenant, int64, error) {
	if global.DB == nil {
		return nil, 0, errTenantDBNotInitialized
	}
	db := global.DB.Table(model.TableNameTenant)
	if len(tenantIDs) > 0 {
		db = db.Where("id IN ?", tenantIDs)
	}
	search = strings.TrimSpace(search)
	if search != "" {
		db = db.Where("name ILIKE ? OR id ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	var list []*model.Tenant
	err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// UpdateTenant 更新租户基本信息。
func UpdateTenant(id string, updates map[string]interface{}) error {
	if global.DB == nil {
		return errTenantDBNotInitialized
	}
	updates["updated_at"] = time.Now().UTC()
	return global.DB.Table(model.TableNameTenant).Where("id = ?", id).Updates(updates).Error
}

// GetTenantEntityCounts 获取租户名下的设备数与用户数。
func GetTenantEntityCounts(tenantID string) (int64, int64, error) {
	if global.DB == nil {
		return 0, 0, errTenantDBNotInitialized
	}
	var deviceCount int64
	var userCount int64
	if err := global.DB.Table(model.TableNameDevice).Where("tenant_id = ?", tenantID).Count(&deviceCount).Error; err != nil {
		return 0, 0, err
	}
	if err := global.DB.Table(model.TableNameUser).Where("tenant_id = ?", tenantID).Count(&userCount).Error; err != nil {
		return 0, 0, err
	}
	return deviceCount, userCount, nil
}
