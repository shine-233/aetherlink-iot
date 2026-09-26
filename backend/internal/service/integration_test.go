// 文件用途：Integration 服务层单测（TB-45）——claims fail-closed、转换器绑定校验与租户隔离。
// 核心逻辑：sqlite 内存库 + 真实 dal 栈，验证创建/更新/查询/删除/列表五条路径：
//
//	未登录拒绝、绑定他租户或不存在转换器拒绝（fail-closed）、config 非法 JSON 拒绝、
//	绑定 ID 空白归一（空串解绑）、默认值（enabled=true、config="{}"）与跨租户不可见。
//
// 关键注意事项：绑定校验复用 dal.GetDataConverterByID 的租户过滤，故需同库种下
//
//	data_converters 行；name 搜索走 PG ILIKE 方言不入 sqlite 用例。
//
// 重构建议：SNMP/插件连接器专属配置校验落地时在本文件补对应校验矩阵。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupIntegrationServiceTestDB 建立仅含 integrations 与 data_converters 的 sqlite 内存库。
func setupIntegrationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open integration service sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Integration{}, &model.DataConverter{}); err != nil {
		t.Fatalf("migrate integration service tables: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

var (
	integrationTenantA = "tenant-a"
	integrationTenantB = "tenant-b"
	integrationClaimsA = &utils.UserClaims{ID: "user-a", TenantID: integrationTenantA, Authority: "TENANT_ADMIN"}
	integrationClaimsB = &utils.UserClaims{ID: "user-b", TenantID: integrationTenantB, Authority: "TENANT_ADMIN"}
)

// seedIntegrationConverter 在指定租户种一条上行转换器（绑定校验的依赖行）。
func seedIntegrationConverter(t *testing.T, id, tenant string) {
	t.Helper()
	now := time.Now().UTC()
	conv := &model.DataConverter{
		ID:            id,
		Name:          "conv-" + id,
		Type:          "UPLINK",
		ConverterMode: "JSON_PATH",
		TenantID:      tenant,
		Configuration: `{"telemetry":{"k":"v"}}`,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	if err := global.DB.Create(conv).Error; err != nil {
		t.Fatalf("seed converter %s: %v", id, err)
	}
}

// wantCode 断言 err 为 errcode.Error 且码值等于 want。
func wantCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望出错（code=%d），实际成功", want)
	}
	var ec *errcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("错误应为 errcode.Error，实际 %T: %v", err, err)
	}
	if ec.Code != want {
		t.Fatalf("错误码 = %d, want %d（msg=%s）", ec.Code, want, ec.Error())
	}
}

func TestIntegrationServiceClaimsFailClosed(t *testing.T) {
	setupIntegrationServiceTestDB(t)
	ctx := context.Background()

	_, err := GroupApp.Integration.CreateIntegration(ctx,
		&model.CreateIntegrationReq{Name: "x", ConnectorType: model.IntegrationConnectorOpcua}, nil)
	wantCode(t, err, errcode.CodeNoPermission)

	_, err = GroupApp.Integration.UpdateIntegration(ctx, &model.UpdateIntegrationReq{ID: "int-1"}, nil)
	wantCode(t, err, errcode.CodeNoPermission)

	_, err = GroupApp.Integration.GetIntegrationByID(ctx, "int-1", nil)
	wantCode(t, err, errcode.CodeNoPermission)

	err = GroupApp.Integration.DeleteIntegration(ctx, "int-1", nil)
	wantCode(t, err, errcode.CodeNoPermission)

	_, err = GroupApp.Integration.ListIntegrations(ctx, &model.GetIntegrationListReq{}, nil)
	wantCode(t, err, errcode.CodeNoPermission)
}

func TestIntegrationServiceCreateValidations(t *testing.T) {
	setupIntegrationServiceTestDB(t)
	ctx := context.Background()
	seedIntegrationConverter(t, "conv-a", integrationTenantA)
	seedIntegrationConverter(t, "conv-b", integrationTenantB)

	badConfig := "not-json"
	_, err := GroupApp.Integration.CreateIntegration(ctx, &model.CreateIntegrationReq{
		Name: "bad-config", ConnectorType: model.IntegrationConnectorOpcua, Config: &badConfig,
	}, integrationClaimsA)
	wantCode(t, err, errcode.CodeParamError)

	_, err = GroupApp.Integration.CreateIntegration(ctx, &model.CreateIntegrationReq{
		Name: "cross-tenant", ConnectorType: model.IntegrationConnectorOpcua,
		ConverterUplinkID: strPtrOf("conv-b"),
	}, integrationClaimsA)
	wantCode(t, err, errcode.CodeParamError)

	_, err = GroupApp.Integration.CreateIntegration(ctx, &model.CreateIntegrationReq{
		Name: "missing-conv", ConnectorType: model.IntegrationConnectorSnmp,
		ConverterDownlinkID: strPtrOf("conv-missing"),
	}, integrationClaimsA)
	wantCode(t, err, errcode.CodeParamError)

	// 正常路径：空白绑定 ID 归一、空串解绑、默认 enabled=true / config="{}"。
	created, err := GroupApp.Integration.CreateIntegration(ctx, &model.CreateIntegrationReq{
		Name:                "edge-opc",
		ConnectorType:       model.IntegrationConnectorOpcua,
		ConverterUplinkID:   strPtrOf("  conv-a  "),
		ConverterDownlinkID: strPtrOf("   "),
		Config:              strPtrOf(`{"device_ids":["dev-1"]}`),
	}, integrationClaimsA)
	if err != nil {
		t.Fatalf("正常创建不应报错: %v", err)
	}
	if created.ConverterUplinkID == nil || *created.ConverterUplinkID != "conv-a" {
		t.Fatalf("绑定 ID 空白归一失败: %+v", created.ConverterUplinkID)
	}
	if created.ConverterDownlinkID != nil {
		t.Fatalf("空串绑定应归一为未绑定(nil): %+v", created.ConverterDownlinkID)
	}
	if !created.Enabled {
		t.Fatal("未传 enabled 应默认 true")
	}
	if created.Config != `{"device_ids":["dev-1"]}` {
		t.Fatalf("config 未按原样落库: %s", created.Config)
	}

	// 空落库形态：config 缺省为 "{}"，bindings 为 NULL。
	minimal, err := GroupApp.Integration.CreateIntegration(ctx, &model.CreateIntegrationReq{
		Name: "minimal", ConnectorType: model.IntegrationConnectorPlugin,
	}, integrationClaimsA)
	if err != nil {
		t.Fatalf("最小入参创建不应报错: %v", err)
	}
	if minimal.Config != "{}" {
		t.Fatalf("config 缺省应为 {}，实际 %s", minimal.Config)
	}
	var raw model.Integration
	if err := global.DB.Where("id = ?", minimal.ID).First(&raw).Error; err != nil {
		t.Fatalf("回读落库行报错: %v", err)
	}
	if raw.ConverterUplinkID != nil || raw.ConverterDownlinkID != nil {
		t.Fatalf("未绑定列应落 NULL: up=%v down=%v", raw.ConverterUplinkID, raw.ConverterDownlinkID)
	}
}

func TestIntegrationServiceUpdatePaths(t *testing.T) {
	setupIntegrationServiceTestDB(t)
	ctx := context.Background()
	seedIntegrationConverter(t, "conv-a", integrationTenantA)
	seedIntegrationConverter(t, "conv-b", integrationTenantB)

	created, err := GroupApp.Integration.CreateIntegration(ctx, &model.CreateIntegrationReq{
		Name:              "int-1",
		ConnectorType:     model.IntegrationConnectorOpcua,
		ConverterUplinkID: strPtrOf("conv-a"),
	}, integrationClaimsA)
	if err != nil {
		t.Fatalf("准备数据失败: %v", err)
	}

	// 跨租户更新：NotFound fail-closed。
	_, err = GroupApp.Integration.UpdateIntegration(ctx, &model.UpdateIntegrationReq{
		ID: created.ID, Name: strPtrOf("hijacked"),
	}, integrationClaimsB)
	wantCode(t, err, errcode.CodeNotFound)

	badConfig := "{"
	_, err = GroupApp.Integration.UpdateIntegration(ctx, &model.UpdateIntegrationReq{
		ID: created.ID, Config: &badConfig,
	}, integrationClaimsA)
	wantCode(t, err, errcode.CodeParamError)

	_, err = GroupApp.Integration.UpdateIntegration(ctx, &model.UpdateIntegrationReq{
		ID: created.ID, ConverterUplinkID: strPtrOf("conv-b"),
	}, integrationClaimsA)
	wantCode(t, err, errcode.CodeParamError)

	// 正常更新：改名 + 停用 + 空串解绑。
	updated, err := GroupApp.Integration.UpdateIntegration(ctx, &model.UpdateIntegrationReq{
		ID:                created.ID,
		Name:              strPtrOf("int-1-renamed"),
		Enabled:           boolPtrOf(false),
		ConverterUplinkID: strPtrOf(""),
	}, integrationClaimsA)
	if err != nil {
		t.Fatalf("正常更新不应报错: %v", err)
	}
	if updated.Name != "int-1-renamed" || updated.Enabled {
		t.Fatalf("名称/启停未更新: %+v", updated)
	}
	if updated.ConverterUplinkID != nil {
		t.Fatalf("空串解绑失败: %+v", updated.ConverterUplinkID)
	}
}

func TestIntegrationServiceGetDeleteListTenantScoped(t *testing.T) {
	setupIntegrationServiceTestDB(t)
	ctx := context.Background()
	seedIntegrationConverter(t, "conv-a", integrationTenantA)

	intA, err := GroupApp.Integration.CreateIntegration(ctx, &model.CreateIntegrationReq{
		Name: "int-a", ConnectorType: model.IntegrationConnectorOpcua,
	}, integrationClaimsA)
	if err != nil {
		t.Fatalf("seed tenant-a 集成失败: %v", err)
	}
	if _, err := GroupApp.Integration.CreateIntegration(ctx, &model.CreateIntegrationReq{
		Name: "int-b", ConnectorType: model.IntegrationConnectorSnmp,
	}, integrationClaimsB); err != nil {
		t.Fatalf("seed tenant-b 集成失败: %v", err)
	}

	// 跨租户读：NotFound。
	_, err = GroupApp.Integration.GetIntegrationByID(ctx, intA.ID, integrationClaimsB)
	wantCode(t, err, errcode.CodeNotFound)

	// 列表租户隔离。
	own, err := GroupApp.Integration.ListIntegrations(ctx, &model.GetIntegrationListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
	}, integrationClaimsA)
	if err != nil {
		t.Fatalf("租户 A 列表报错: %v", err)
	}
	if own["total"] != int64(1) {
		t.Fatalf("租户 A 列表 total = %v, want 1", own["total"])
	}
	other, err := GroupApp.Integration.ListIntegrations(ctx, &model.GetIntegrationListReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
	}, integrationClaimsB)
	if err != nil {
		t.Fatalf("租户 B 列表报错: %v", err)
	}
	if other["total"] != int64(1) {
		t.Fatalf("租户 B 列表 total = %v, want 1（只见本租户）", other["total"])
	}

	// 跨租户删：NotFound 且数据不动；本租户删：成功且再读 NotFound。
	err = GroupApp.Integration.DeleteIntegration(ctx, intA.ID, integrationClaimsB)
	wantCode(t, err, errcode.CodeNotFound)
	if err := GroupApp.Integration.DeleteIntegration(ctx, intA.ID, integrationClaimsA); err != nil {
		t.Fatalf("本租户删报错: %v", err)
	}
	_, err = GroupApp.Integration.GetIntegrationByID(ctx, intA.ID, integrationClaimsA)
	wantCode(t, err, errcode.CodeNotFound)
}

// strPtrOf / boolPtrOf 测试辅助构造器。
func strPtrOf(s string) *string { return &s }
func boolPtrOf(b bool) *bool    { return &b }
