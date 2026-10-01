// 文件用途：Integration DAL 层单测（TB-45）——CRUD、租户隔离与列表过滤。
// 核心逻辑：sqlite 内存库构造隔离环境，验证 Get/Delete 的租户过滤 fail-closed
//
//	（跨租户读不到/删不掉）与 List 的租户 + connector_type + enabled 过滤及分页。
//
// 关键注意事项：name 搜索走 PG ILIKE 方言，sqlite 不支持，故 Search 用例不入本文件
//
//	（与 data_converter 等存量栈同口径，方言行为由契约测试覆盖）。
//
// 重构建议：若后续引入跨方言兼容的测试库，可补 Search 过滤用例。
package dal

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupIntegrationDALTestDB 建立仅含 integrations 与 data_converters 的 sqlite 内存库。
func setupIntegrationDALTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open integration sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Integration{}, &model.DataConverter{}); err != nil {
		t.Fatalf("migrate integration: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

// seedIntegrationDALRow 按参数插入一条集成实例（绑定与配置固定，专注作用域断言）。
func seedIntegrationDALRow(t *testing.T, db *gorm.DB, id, tenant, connector string, enabled bool) {
	t.Helper()
	now := time.Now().UTC()
	uplink := "conv-" + tenant
	row := &model.Integration{
		ID:                  id,
		Name:                "int-" + id,
		TenantID:            tenant,
		ConnectorType:       connector,
		ConverterUplinkID:   &uplink,
		ConverterDownlinkID: nil,
		Config:              `{"device_ids":["dev-1"]}`,
		Enabled:             enabled,
		CreatedAt:           &now,
		UpdatedAt:           &now,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("seed integration %s: %v", id, err)
	}
}

func TestIntegrationDALTenantIsolation(t *testing.T) {
	db := setupIntegrationDALTestDB(t)
	seedIntegrationDALRow(t, db, "int-1", "tenant-1", model.IntegrationConnectorOpcua, true)

	// 跨租户读：fail-closed 返回 NotFound。
	if _, err := GetIntegrationByID("int-1", "tenant-2"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("跨租户 GetIntegrationByID error = %v, want record not found", err)
	}
	// 本租户读：命中且绑定字段原样保留。
	got, err := GetIntegrationByID("int-1", "tenant-1")
	if err != nil {
		t.Fatalf("本租户 GetIntegrationByID 报错: %v", err)
	}
	if got.ConnectorType != model.IntegrationConnectorOpcua || got.Config != `{"device_ids":["dev-1"]}` {
		t.Fatalf("字段回读不符: %+v", got)
	}
	if got.ConverterUplinkID == nil || *got.ConverterUplinkID != "conv-tenant-1" {
		t.Fatalf("上行绑定回读不符: %+v", got.ConverterUplinkID)
	}

	// 跨租户删：0 行命中按 NotFound 返回（不泄露存在性差异），数据不得被删。
	if err := DeleteIntegration("int-1", "tenant-2"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("跨租户 DeleteIntegration error = %v, want record not found", err)
	}
	var count int64
	if err := db.Model(&model.Integration{}).Where("id = ?", "int-1").Count(&count).Error; err != nil {
		t.Fatalf("count after cross-tenant delete: %v", err)
	}
	if count != 1 {
		t.Fatalf("跨租户删除后记录仍在，count = %d, want 1", count)
	}

	// 本租户删：成功且二次删报 NotFound。
	if err := DeleteIntegration("int-1", "tenant-1"); err != nil {
		t.Fatalf("本租户 DeleteIntegration 报错: %v", err)
	}
	if err := DeleteIntegration("int-1", "tenant-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("重复删除 error = %v, want record not found", err)
	}
}

func TestIntegrationDALListScopeAndFilters(t *testing.T) {
	db := setupIntegrationDALTestDB(t)
	seedIntegrationDALRow(t, db, "int-a", "tenant-1", model.IntegrationConnectorOpcua, true)
	seedIntegrationDALRow(t, db, "int-b", "tenant-1", model.IntegrationConnectorSnmp, false)
	seedIntegrationDALRow(t, db, "int-c", "tenant-2", model.IntegrationConnectorOpcua, true)

	// 租户隔离：只见本租户两行。
	total, list, err := ListIntegrations(&model.GetIntegrationListReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}, "tenant-1")
	if err != nil {
		t.Fatalf("ListIntegrations 报错: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("租户过滤不符: total=%d len=%d, want 2/2", total, len(list))
	}
	for _, row := range list {
		if row.TenantID != "tenant-1" {
			t.Fatalf("列表混入他租户数据: %+v", row)
		}
	}

	// connector_type 过滤。
	conn := model.IntegrationConnectorOpcua
	total, list, err = ListIntegrations(&model.GetIntegrationListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 20}, ConnectorType: &conn,
	}, "tenant-1")
	if err != nil {
		t.Fatalf("connector_type 过滤报错: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].ID != "int-a" {
		t.Fatalf("connector_type 过滤不符: total=%d list=%+v", total, list)
	}

	// enabled 过滤。
	enabledFlag := false
	total, _, err = ListIntegrations(&model.GetIntegrationListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 20}, Enabled: &enabledFlag,
	}, "tenant-1")
	if err != nil {
		t.Fatalf("enabled 过滤报错: %v", err)
	}
	if total != 1 {
		t.Fatalf("enabled 过滤不符: total=%d, want 1", total)
	}

	// 分页：每页 1 条取第 2 页，总数不变。
	total, page2, err := ListIntegrations(&model.GetIntegrationListReq{PageReq: model.PageReq{Page: 2, PageSize: 1}}, "tenant-1")
	if err != nil {
		t.Fatalf("分页查询报错: %v", err)
	}
	if total != 2 || len(page2) != 1 {
		t.Fatalf("分页不符: total=%d len=%d, want 2/1", total, len(page2))
	}
}

func TestIntegrationDALUpdatePersistsBindings(t *testing.T) {
	db := setupIntegrationDALTestDB(t)
	seedIntegrationDALRow(t, db, "int-1", "tenant-1", model.IntegrationConnectorOpcua, true)

	got, err := GetIntegrationByID("int-1", "tenant-1")
	if err != nil {
		t.Fatalf("准备更新前读取报错: %v", err)
	}
	downlink := "conv-down"
	got.Enabled = false
	got.ConverterDownlinkID = &downlink
	got.Config = `{"device_ids":["dev-2"]}`
	if err := UpdateIntegration(got); err != nil {
		t.Fatalf("UpdateIntegration 报错: %v", err)
	}

	reloaded, err := GetIntegrationByID("int-1", "tenant-1")
	if err != nil {
		t.Fatalf("更新后读取报错: %v", err)
	}
	if reloaded.Enabled {
		t.Fatal("enabled 应已更新为 false")
	}
	if reloaded.ConverterDownlinkID == nil || *reloaded.ConverterDownlinkID != "conv-down" {
		t.Fatalf("下行绑定未落库: %+v", reloaded.ConverterDownlinkID)
	}
	if reloaded.Config != `{"device_ids":["dev-2"]}` {
		t.Fatalf("config 未落库: %s", reloaded.Config)
	}
}
