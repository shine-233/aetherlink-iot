// 文件用途：提供白标（TB-47）翻译覆盖与自定义 CSS 的持久化存储操作。
// 核心逻辑：tenant_translations 的批量 UPSERT / 按键删除 / 租户内列表，以及
//
//	tenant_custom_css 的单行读取与 UPSERT；全部查询显式携带 tenant_id 条件。
//
// 关键注意事项：租户隔离 fail-closed——tenant-scope: caller-enforced，
//
//	租户条件由 service 层注入 claims.TenantID（或 SYS_ADMIN 的全局空串）落地为
//	WHERE；删除/列表永不接受空 items 或缺租户上下文的调用。SQL 中 "key" 列名
//	必须带双引号（PG 与 sqlite 双方言安全）。
//
// 重构建议：批量 UPSERT 目前单语句多值插入（单事务）；若单批条数上限放大，
//
//	考虑分片提交避免单语句参数上限。
package dal

import (
	"context"
	"errors"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// errWhitelabelDBNotReady 数据库句柄未初始化（进程启动早期或测试环境未注入）。
var errWhitelabelDBNotReady = errors.New("database is not initialized")

// UpsertTenantTranslations 批量写入翻译覆盖（UNIQUE(tenant_id,lang,key) 冲突时覆盖 value）。
// rows 必须已由 service 层填好 tenant_id / id；空切片幂等返回。
func UpsertTenantTranslations(ctx context.Context, rows []*model.TenantTranslation) error {
	if global.DB == nil {
		return errWhitelabelDBNotReady
	}
	if len(rows) == 0 {
		return nil
	}
	return global.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "lang"},
				{Name: "key"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).
		Create(&rows).Error
}

// DeleteTenantTranslations 按租户删除指定 (lang,key) 翻译覆盖，返回实际删除行数。
// 按语言分组后逐组删除（"key" 带引号保证 PG/sqlite 双方言可用）；
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func DeleteTenantTranslations(ctx context.Context, tenantID string, keys []model.TenantTranslationKeyItem) (int64, error) {
	if global.DB == nil {
		return 0, errWhitelabelDBNotReady
	}
	if len(keys) == 0 {
		return 0, nil // 空删除集幂等返回；tenantID 为空串是合法的全局作用域
	}
	byLang := make(map[string][]string, 4)
	for _, item := range keys {
		byLang[item.Lang] = append(byLang[item.Lang], item.Key)
	}
	var total int64
	for lang, keyList := range byLang {
		res := global.DB.WithContext(ctx).
			Where("tenant_id = ? AND lang = ? AND \"key\" IN ?", tenantID, lang, keyList).
			Delete(&model.TenantTranslation{})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	return total, nil
}

// ListTenantTranslations 列出租户（或全局）翻译覆盖；lang 非空时追加语言过滤。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为 WHERE。
func ListTenantTranslations(ctx context.Context, tenantID, lang string) ([]*model.TenantTranslation, error) {
	if global.DB == nil {
		return nil, errWhitelabelDBNotReady
	}
	db := global.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if lang != "" {
		db = db.Where("lang = ?", lang)
	}
	var list []*model.TenantTranslation
	err := db.Order("lang ASC, \"key\" ASC").Find(&list).Error
	return list, err
}

// GetTenantCustomCSS 读取租户自定义 CSS；无行返回 (nil, nil)（未配置语义，不视为错误）。
// tenant-scope: caller-enforced —— 租户条件由调用方传入 tenantID 落地为主键定位。
func GetTenantCustomCSS(ctx context.Context, tenantID string) (*model.TenantCustomCSS, error) {
	if global.DB == nil {
		return nil, errWhitelabelDBNotReady
	}
	var row model.TenantCustomCSS
	err := global.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// UpsertTenantCustomCSS 写入租户自定义 CSS（tenant_id 主键冲突时覆盖 css）；
// css 为空串即清除样式（保留行，简化幂等语义）。
func UpsertTenantCustomCSS(ctx context.Context, tenantID, css string) error {
	if global.DB == nil {
		return errWhitelabelDBNotReady
	}
	row := model.TenantCustomCSS{TenantID: tenantID, CSS: css}
	return global.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"css", "updated_at"}),
		}).
		Create(&row).Error
}
