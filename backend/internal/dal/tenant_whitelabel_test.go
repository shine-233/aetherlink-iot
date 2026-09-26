// 文件用途：白标 DAL 层（TB-47）单元测试。
// 核心逻辑：sqlite 内存库驱动 tenant_translations 的 UPSERT 覆盖/按键删除/语言过滤
//
//	与 tenant_custom_css 的单行 UPSERT/缺失读取，并覆盖租户隔离 fail-closed
//	负向用例（跨租户列表不可见、跨租户删除零命中且不误伤）。
//
// 关键注意事项：全部用例走 sqlite 内存库并 AutoMigrate 仅相关表（复合唯一索引由
//
//	模型 uniqueIndex 标签生成，与 134.sql 的 UNIQUE(tenant_id,lang,key) 对应）；
//	UPSERT 冲突目标依赖该索引，改动模型标签必须同步本文件与迁移。
//
// 重构建议：若 UPSERT 分片提交落地，在本文件补大批量（>500）行为断言。
package dal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	dalWhitelabelTenantA = "whitelabel-tenant-a"
	dalWhitelabelTenantB = "whitelabel-tenant-b"
)

// setupWhitelabelDALTestDB 建立仅含白标两张表的 sqlite 内存库并注入 global.DB。
func setupWhitelabelDALTestDB(t *testing.T) {
	t.Helper()
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:whitelabel_dal_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TenantTranslation{}, &model.TenantCustomCSS{}))
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
}

// whitelabelRow 构造一条翻译覆盖行（id 由这里生成，模拟 service 层行为）。
func whitelabelRow(t *testing.T, tenantID, lang, key, value string) *model.TenantTranslation {
	t.Helper()
	return &model.TenantTranslation{
		ID:       uuid.NewString(),
		TenantID: tenantID,
		Lang:     lang,
		Key:      key,
		Value:    value,
	}
}

// TestUpsertTenantTranslations_InsertAndOverwrite 首插为新增，同键重写覆盖 value（行数不变）。
func TestUpsertTenantTranslations_InsertAndOverwrite(t *testing.T) {
	setupWhitelabelDALTestDB(t)
	ctx := context.Background()

	row := whitelabelRow(t, dalWhitelabelTenantA, "zh-cn", "page.customer.title", "客户")
	require.NoError(t, UpsertTenantTranslations(ctx, []*model.TenantTranslation{row}))

	// 同 (tenant,lang,key) 再写：覆盖 value，不产生第二行。
	updated := whitelabelRow(t, dalWhitelabelTenantA, "zh-cn", "page.customer.title", "客户中心")
	require.NoError(t, UpsertTenantTranslations(ctx, []*model.TenantTranslation{updated}))

	list, err := ListTenantTranslations(ctx, dalWhitelabelTenantA, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "客户中心", list[0].Value)

	// 同租户不同语言同键共存（UNIQUE 是三元组不是二元组）。
	en := whitelabelRow(t, dalWhitelabelTenantA, "en-us", "page.customer.title", "Customer")
	require.NoError(t, UpsertTenantTranslations(ctx, []*model.TenantTranslation{en}))
	list, err = ListTenantTranslations(ctx, dalWhitelabelTenantA, "")
	require.NoError(t, err)
	require.Len(t, list, 2)
}

// TestListTenantTranslations_TenantIsolationAndLangFilter 租户隔离与语言过滤。
func TestListTenantTranslations_TenantIsolationAndLangFilter(t *testing.T) {
	setupWhitelabelDALTestDB(t)
	ctx := context.Background()

	rows := []*model.TenantTranslation{
		whitelabelRow(t, dalWhitelabelTenantA, "zh-cn", "page.customer.title", "客户中心"),
		whitelabelRow(t, dalWhitelabelTenantA, "en-us", "page.customer.title", "Customer"),
		whitelabelRow(t, dalWhitelabelTenantB, "zh-cn", "page.device.title", "设备"),
	}
	require.NoError(t, UpsertTenantTranslations(ctx, rows))

	// 租户 A 看不到租户 B 的行（fail-closed 隔离）。
	onlyA, err := ListTenantTranslations(ctx, dalWhitelabelTenantA, "")
	require.NoError(t, err)
	require.Len(t, onlyA, 2)
	// 语言过滤只保留对应语言。
	onlyZh, err := ListTenantTranslations(ctx, dalWhitelabelTenantA, "zh-cn")
	require.NoError(t, err)
	require.Len(t, onlyZh, 1)
	require.Equal(t, "zh-cn", onlyZh[0].Lang)
	// 空串作用域（SYS_ADMIN 全局行）不返回任何租户行。
	globalRows, err := ListTenantTranslations(ctx, "", "")
	require.NoError(t, err)
	require.Empty(t, globalRows)
}

// TestDeleteTenantTranslations_CrossTenantZeroHit 跨租户删除零命中且不误伤。
func TestDeleteTenantTranslations_CrossTenantZeroHit(t *testing.T) {
	setupWhitelabelDALTestDB(t)
	ctx := context.Background()

	require.NoError(t, UpsertTenantTranslations(ctx, []*model.TenantTranslation{
		whitelabelRow(t, dalWhitelabelTenantA, "zh-cn", "page.customer.title", "客户中心"),
		whitelabelRow(t, dalWhitelabelTenantA, "en-us", "page.customer.title", "Customer"),
	}))

	// 用租户 B 的作用域删租户 A 的键：0 行命中，租户 A 数据完好。
	deleted, err := DeleteTenantTranslations(ctx, dalWhitelabelTenantB, []model.TenantTranslationKeyItem{
		{Lang: "zh-cn", Key: "page.customer.title"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)
	kept, err := ListTenantTranslations(ctx, dalWhitelabelTenantA, "")
	require.NoError(t, err)
	require.Len(t, kept, 2)

	// 本租户跨语言批量删除：按语言分组逐组命中，计数准确。
	deleted, err = DeleteTenantTranslations(ctx, dalWhitelabelTenantA, []model.TenantTranslationKeyItem{
		{Lang: "zh-cn", Key: "page.customer.title"},
		{Lang: "en-us", Key: "page.customer.title"},
		{Lang: "fr-fr", Key: "never.existed"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), deleted)
	kept, err = ListTenantTranslations(ctx, dalWhitelabelTenantA, "")
	require.NoError(t, err)
	require.Empty(t, kept)
}

// TestTenantCustomCSS_SingleRowUpsertAndGet 自定义 CSS：缺失读取、单行 UPSERT、清除语义。
func TestTenantCustomCSS_SingleRowUpsertAndGet(t *testing.T) {
	setupWhitelabelDALTestDB(t)
	ctx := context.Background()

	// 无行=未配置，不视为错误。
	row, err := GetTenantCustomCSS(ctx, dalWhitelabelTenantA)
	require.NoError(t, err)
	require.Nil(t, row)

	require.NoError(t, UpsertTenantCustomCSS(ctx, dalWhitelabelTenantA, ".app{color:red}"))
	row, err = GetTenantCustomCSS(ctx, dalWhitelabelTenantA)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Equal(t, ".app{color:red}", row.CSS)

	// 再写覆盖而非新增行（tenant_id 主键单行语义）。
	require.NoError(t, UpsertTenantCustomCSS(ctx, dalWhitelabelTenantA, ".app{color:blue}"))
	row, err = GetTenantCustomCSS(ctx, dalWhitelabelTenantA)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Equal(t, ".app{color:blue}", row.CSS)

	// 租户 B 不受影响（主键隔离）。
	rowB, err := GetTenantCustomCSS(ctx, dalWhitelabelTenantB)
	require.NoError(t, err)
	require.Nil(t, rowB)

	// 空串=清除样式（行保留，css 为空）。
	require.NoError(t, UpsertTenantCustomCSS(ctx, dalWhitelabelTenantA, ""))
	row, err = GetTenantCustomCSS(ctx, dalWhitelabelTenantA)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Empty(t, row.CSS)
}
