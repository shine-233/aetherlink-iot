// 文件用途：覆盖 model mapping 相关模型映射和转换行为，防止模型字段在重构中丢失或错映射。
// 核心逻辑：通过构造模型样例并断言响应字段、JSON 结构或辅助转换结果，锁定当前模型契约。
// 关键注意事项：测试只验证模型层的轻量转换，不替代服务层权限、事务或数据库集成测试。
// 重构建议：新增模型转换分支时同步补充表驱动用例，并把重复构造数据抽到局部 helper。

package model

import (
	"encoding/json"
	"testing"
)

func strPtr(value string) *string {
	return &value
}

func int16Ptr(value int16) *int16 {
	return &value
}

func TestSysUIElementToRspPreservesMenuRoutePermissionFields(t *testing.T) {
	elementType := int16(4)
	order := int16(12)
	element := &SysUIElement{
		ID:           "menu-device",
		ParentID:     "root",
		ElementCode:  "device_manage",
		ElementType:  elementType,
		Order_:       &order,
		Param1:       strPtr("icon-device"),
		Param2:       strPtr("cache"),
		Param3:       strPtr("pin"),
		Authority:    "TENANT_ADMIN",
		Description:  strPtr("Device menu"),
		Remark:       strPtr("visible in tenant nav"),
		Multilingual: strPtr("route.device.manage"),
		RoutePath:    strPtr("/device/manage"),
	}

	got := element.ToRsp()
	if got.ID != element.ID ||
		got.ParentID != element.ParentID ||
		got.ElementCode != element.ElementCode ||
		got.Authority != element.Authority {
		t.Fatalf("ToRsp core fields mismatch: %#v", got)
	}
	if got.ElementType == nil || *got.ElementType != elementType {
		t.Fatalf("ToRsp ElementType = %#v, want %d", got.ElementType, elementType)
	}
	if got.Orders == nil || *got.Orders != order {
		t.Fatalf("ToRsp Orders = %#v, want %d", got.Orders, order)
	}
	if got.Param1 != element.Param1 || got.Param2 != element.Param2 || got.Param3 != element.Param3 {
		t.Fatalf("ToRsp params should preserve configured pointers: %#v", got)
	}
	if got.Description != element.Description ||
		got.Remark != element.Remark ||
		got.Multilingual != element.Multilingual ||
		got.RoutePath != element.RoutePath {
		t.Fatalf("ToRsp descriptive fields mismatch: %#v", got)
	}
	if got.Children == nil || len(got.Children) != 0 {
		t.Fatalf("ToRsp Children = %#v, want initialized empty slice", got.Children)
	}
}

func TestSysUIElementToRsp1KeepsOnlyTreeSelectorFields(t *testing.T) {
	element := &SysUIElement{
		ID:           "button-save",
		ParentID:     "menu-device",
		ElementCode:  "save",
		ElementType:  3,
		Order_:       int16Ptr(9),
		Authority:    "TENANT_ADMIN",
		Description:  strPtr("Save button"),
		Remark:       strPtr("not exposed"),
		Multilingual: strPtr("not exposed"),
		RoutePath:    strPtr("/not-exposed"),
	}

	got := element.ToRsp1()
	if got.ID != "button-save" || got.ParentID != "menu-device" || got.ElementCode != "save" {
		t.Fatalf("ToRsp1 tree fields mismatch: %#v", got)
	}
	if got.ElementType == nil || *got.ElementType != 3 {
		t.Fatalf("ToRsp1 ElementType = %#v, want 3", got.ElementType)
	}
	if got.Description != element.Description {
		t.Fatalf("ToRsp1 Description = %#v, want original pointer", got.Description)
	}
	if got.Children == nil || len(got.Children) != 0 {
		t.Fatalf("ToRsp1 Children = %#v, want initialized empty slice", got.Children)
	}
}

func TestTenantDashboardMenuToRspPreservesDashboardNavigationContract(t *testing.T) {
	tests := []struct {
		name string
		menu TenantDashboardMenu
	}{
		{
			name: "enabled menu",
			menu: TenantDashboardMenu{
				DashboardID:   "dashboard-1",
				DashboardName: "Operations",
				MenuName:      "Main dashboard",
				ParentCode:    "home",
				Sort:          7,
				Enabled:       true,
			},
		},
		{
			name: "disabled menu",
			menu: TenantDashboardMenu{
				DashboardID:   "dashboard-2",
				DashboardName: "Hidden operations",
				MenuName:      "Disabled dashboard",
				ParentCode:    "visualization",
				Sort:          99,
				Enabled:       false,
			},
		},
		{
			name: "custom parent menu",
			menu: TenantDashboardMenu{
				DashboardID:   "dashboard-3",
				DashboardName: "Energy",
				MenuName:      "Energy operations",
				ParentCode:    "energy-overview",
				Sort:          3,
				Enabled:       true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.menu.ToRsp()
			if got.DashboardID != tc.menu.DashboardID ||
				got.DashboardName != tc.menu.DashboardName ||
				got.MenuName != tc.menu.MenuName ||
				got.Sort != tc.menu.Sort ||
				got.Enabled != tc.menu.Enabled {
				t.Fatalf("TenantDashboardMenu.ToRsp mismatch: %#v, want source menu %#v", got, tc.menu)
			}
			if got.ParentCode != tc.menu.ParentCode {
				t.Fatalf("TenantDashboardMenu.ToRsp ParentCode = %q, want %q", got.ParentCode, tc.menu.ParentCode)
			}
		})
	}
}

func TestJsonRawMessage2StrCompactsObjectAndRejectsInvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{ "name": "sensor", "threshold": 12 }`)

	got, err := JsonRawMessage2Str(&raw)
	if err != nil {
		t.Fatalf("JsonRawMessage2Str returned error: %v", err)
	}
	if got != `{"name":"sensor","threshold":12}` && got != `{"threshold":12,"name":"sensor"}` {
		t.Fatalf("JsonRawMessage2Str = %q, want compact object JSON", got)
	}

	invalid := json.RawMessage(`{"name":`)
	if _, err := JsonRawMessage2Str(&invalid); err == nil {
		t.Fatal("JsonRawMessage2Str expected error for invalid JSON")
	}
}
