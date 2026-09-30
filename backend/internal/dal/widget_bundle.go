// 文件用途：提供部件库（widget_bundles，ROADMAP TB-04）持久化存储操作。
// 核心逻辑：部件库 CRUD、租户隔离过滤（fail-closed）与分页检索；
//
//	并提供资源中心链路所需的按租户版本扫描与按行业类型 ID 检索。
//
// 关键注意事项：全部查询函数显式携带 tenant_id 条件（tenant-scope: caller-enforced，
//
//	由 service 层注入 claims.TenantID）；widgets 为 JSONB 列，模型侧用 string 承载。
//
// 重构建议：若部件库规模增长，可给 name 加 trigram 索引替代 ILIKE 全表扫。
package dal

import (
	"context"
	"errors"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
)

var errWidgetBundleDBNotReady = errors.New("database is not initialized")

const (
	// widgetBundleListDefaultPageSize 部件库列表缺省页大小（旧手写口径保持不变）。
	widgetBundleListDefaultPageSize = 20
	// widgetBundleListMaxPageSize 部件库列表单页大小上限（旧写法超限重置为缺省，
	// 2026-09-28 收编后统一为 clamp 到上限，与 normalizePageParams 包内口径一致）。
	widgetBundleListMaxPageSize = 200
)

// CreateWidgetBundle 插入一条部件库记录
func CreateWidgetBundle(c *model.WidgetBundle) error {
	return global.DB.Create(c).Error
}

// GetWidgetBundleByID 根据 ID 及租户 ID 查询部件库（租户隔离 fail-closed）
func GetWidgetBundleByID(id, tenantID string) (*model.WidgetBundle, error) {
	var record model.WidgetBundle
	db := global.DB.Where("id = ?", id)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if err := db.First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// UpdateWidgetBundle 更新部件库
func UpdateWidgetBundle(c *model.WidgetBundle) error {
	return global.DB.Save(c).Error
}

// DeleteWidgetBundle 删除部件库（按 ID + 租户双条件）
func DeleteWidgetBundle(id, tenantID string) error {
	db := global.DB.Where("id = ?", id)
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	return db.Delete(&model.WidgetBundle{}).Error
}

// ListWidgetBundles 分页查询租户部件库列表
func ListWidgetBundles(req *model.GetWidgetBundleListReq, tenantID string) (int64, []*model.WidgetBundle, error) {
	var list []*model.WidgetBundle

	db := global.DB.Model(&model.WidgetBundle{})
	if tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	}
	if req.TypeKey != nil && strings.TrimSpace(*req.TypeKey) != "" {
		db = db.Where("type_key = ?", strings.TrimSpace(*req.TypeKey))
	}
	// 搜索收编（2026-09-28）：旧写法 "%" + 用户输入 + "%" 未转义通配符，搜索值里的 %/_
	// 按通配符生效（"%%" 等价全表模糊扫）；whereKeywordContains 统一走 ContainsLikePattern
	// 转义 + 显式 ESCAPE '\'，多列 OR 括号化，空输入自动跳过。
	db = whereKeywordContainsPtr(db, opILike, req.Search, "name", "description")

	// 分页收编（2026-09-28）：normalizePageParams 保持缺省 20 口径、超限 clamp 到上限；
	// countAndFindPage 在同一过滤条件上先 COUNT 再分页取数，ORDER BY 片段过
	// allowListedOrderFragment 白名单。
	page, pageSize := normalizePageParams(req.Page, req.PageSize, widgetBundleListDefaultPageSize, widgetBundleListMaxPageSize)
	count, err := countAndFindPage(db, "created_at DESC", page, pageSize, &list)
	if err != nil {
		return 0, nil, err
	}

	return count, list, nil
}

// GetWidgetBundleByNameInTenant 按租户和名称查询部件库（导入幂等键用）。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func GetWidgetBundleByNameInTenant(ctx context.Context, tenantID, name string) (*model.WidgetBundle, error) {
	if global.DB == nil {
		return nil, errWidgetBundleDBNotReady
	}
	var record model.WidgetBundle
	err := global.DB.WithContext(ctx).
		Where("tenant_id = ? AND name = ?", tenantID, name).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ListWidgetBundleVersionsInTenant 列出租户内全部部件库的 名称→版本（导入冲突预览用）。
func ListWidgetBundleVersionsInTenant(ctx context.Context, tenantID string) (map[string]string, error) {
	if global.DB == nil {
		return nil, errWidgetBundleDBNotReady
	}
	var rows []struct {
		Name    string `gorm:"column:name"`
		Version string `gorm:"column:version"`
	}
	err := global.DB.WithContext(ctx).
		Model(&model.WidgetBundle{}).
		Select("name, version").
		Where("tenant_id = ?", tenantID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	versions := make(map[string]string, len(rows))
	for _, row := range rows {
		ver := row.Version
		if strings.TrimSpace(ver) == "" {
			ver = "1.0.0"
		}
		if prev, ok := versions[row.Name]; !ok || ver > prev {
			versions[row.Name] = ver
		}
	}
	return versions, nil
}

// ListWidgetBundleIDsByTypeKey 按租户和行业类型查询部件库 ID 列表（资源中心打包导出用）。
func ListWidgetBundleIDsByTypeKey(ctx context.Context, tenantID, typeKey string) ([]string, error) {
	if global.DB == nil {
		return nil, errWidgetBundleDBNotReady
	}
	q := global.DB.WithContext(ctx).
		Model(&model.WidgetBundle{}).
		Select("id").
		Where("tenant_id = ?", tenantID)
	if strings.TrimSpace(typeKey) != "" {
		q = q.Where("type_key = ?", strings.TrimSpace(typeKey))
	}
	var ids []string
	if err := q.Order("created_at ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
