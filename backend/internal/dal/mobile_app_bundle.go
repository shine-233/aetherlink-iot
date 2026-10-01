// 文件用途：提供移动应用中心（mobile_app_bundles，ROADMAP TB-23）持久化存储操作。
// 核心逻辑：应用包登记的创建（唯一约束直通）、租户隔离的详情/分页列表、
//
//	draft 限定字段更新、状态流转的 compare-and-swap 更新（WHERE status = 期望值）
//	与行删除；状态机的合法矩阵判定在 service 层，本文件只保证落库不被并发击穿。
//
// 关键注意事项：全部查询/更新/删除函数显式携带 tenant_id 条件（tenant-scope:
//
//	caller-enforced，由 service 层注入 claims 推导出的租户）；状态流转必须走
//	UpdateAppBundleStatus 的 CAS 语义（RowsAffected==0 即期望状态已漂移，按拒绝处理），
//	不要改成"读-改-写"两段式；重复版本由 UNIQUE(tenant_id,platform,version) 兜底，
//	错误映射（isDuplicateAppBundleError）同时兼容 PG 与 sqlite 测试库的报错文案。
//
// 重构建议：uniapp 对接阶段若需要"按平台取最新已发布包"的读面，加只读查询；
//
//	若引入软删除/回收站，删除语义需与本文件的硬删口径一起重审。
package dal

import (
	"context"
	"errors"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// errAppBundleDBNotReady 全局 DB 未初始化时的统一哨兵错误（fail-closed，不猜空结果）。
var errAppBundleDBNotReady = errors.New("database is not initialized")

// CreateAppBundle 插入一条应用包登记。
// 同租户同平台同版本冲突时不做 OnConflict 吞并——重复版本必须显式报错
// （服务层将唯一约束错误映射为业务码），静默吞并会让上传方误以为登记成功。
func CreateAppBundle(record *model.MobileAppBundle) error {
	if global.DB == nil {
		return errAppBundleDBNotReady
	}
	return global.DB.Create(record).Error
}

// GetAppBundleForScope 按 id + 租户查询应用包（租户隔离 fail-closed）。
func GetAppBundleForScope(ctx context.Context, id, tenantID string) (*model.MobileAppBundle, error) {
	if global.DB == nil {
		return nil, errAppBundleDBNotReady
	}
	var record model.MobileAppBundle
	err := global.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetAppBundleByPlatformVersion 按租户 + 平台 + 版本查询应用包（重复版本预检用）。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func GetAppBundleByPlatformVersion(ctx context.Context, tenantID, platform, version string) (*model.MobileAppBundle, error) {
	if global.DB == nil {
		return nil, errAppBundleDBNotReady
	}
	var record model.MobileAppBundle
	err := global.DB.WithContext(ctx).
		Where("tenant_id = ? AND platform = ? AND version = ?", tenantID, platform, version).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ListAppBundlesForScope 分页查询租户应用包列表；platform/status 精确过滤。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func ListAppBundlesForScope(ctx context.Context, req *model.GetAppBundleListReq, tenantID string) (int64, []*model.MobileAppBundle, error) {
	if global.DB == nil {
		return 0, nil, errAppBundleDBNotReady
	}
	var list []*model.MobileAppBundle

	db := global.DB.WithContext(ctx).Model(&model.MobileAppBundle{}).Where("tenant_id = ?", tenantID)
	if req.Platform != nil && strings.TrimSpace(*req.Platform) != "" {
		db = db.Where("platform = ?", strings.TrimSpace(*req.Platform))
	}
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		db = db.Where("status = ?", strings.TrimSpace(*req.Status))
	}

	count, err := countAndFindLegacyPage(db, "created_at DESC, id DESC", req.Page, req.PageSize, &list)
	return count, list, err
}

// UpdateAppBundleReleaseNotes 更新应用包的发布说明；仅 draft 状态可改（WHERE 强制）。
// 未命中（不存在 / 非本租户 / 状态非 draft）返回 gorm.ErrRecordNotFound。
func UpdateAppBundleReleaseNotes(ctx context.Context, id, tenantID, releaseNotes string) error {
	if global.DB == nil {
		return errAppBundleDBNotReady
	}
	result := global.DB.WithContext(ctx).Model(&model.MobileAppBundle{}).
		Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, model.AppBundleStatusDraft).
		Update("release_notes", releaseNotes)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateAppBundleStatus 状态流转的 compare-and-swap：仅当当前 status 等于 fromStatus
// 时才推进到 toStatus，并在 draft→published 流转落 published_at（其他流转不触碰该列）。
// 返回 gorm.ErrRecordNotFound 表示记录不存在 / 非本租户 / 期望状态已漂移（并发流转），
// 由服务层统一映射为"非法流转"业务码。
func UpdateAppBundleStatus(ctx context.Context, id, tenantID, fromStatus, toStatus string, publishedAt *time.Time) error {
	if global.DB == nil {
		return errAppBundleDBNotReady
	}
	updates := map[string]interface{}{
		"status":     toStatus,
		"updated_at": time.Now().UTC(),
	}
	if publishedAt != nil {
		updates["published_at"] = *publishedAt
	}
	result := global.DB.WithContext(ctx).Model(&model.MobileAppBundle{}).
		Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, fromStatus).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteAppBundleForScope 按 id + 租户删除登记行（不校验状态，删除资格由服务层判定）。
// 未命中返回 gorm.ErrRecordNotFound。
func DeleteAppBundleForScope(ctx context.Context, id, tenantID string) error {
	if global.DB == nil {
		return errAppBundleDBNotReady
	}
	result := global.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.MobileAppBundle{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// IsDuplicateAppBundleError 判断是否为 UNIQUE(tenant_id, platform, version) 冲突。
// PostgreSQL 报 "duplicate key value violates unique constraint"，glebarez/sqlite
// 测试库报 "UNIQUE constraint failed"——两种文案都覆盖，避免测试库假绿。
func IsDuplicateAppBundleError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
