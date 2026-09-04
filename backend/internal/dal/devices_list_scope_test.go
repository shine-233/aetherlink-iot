// 文件用途: ROADMAP C2 —— GetDeviceListByPageForScopes 的层级作用域真实结果集断言。
// 核心逻辑: seed 三个租户设备后,用 scopes=[self,祖先] 调用 scoped 变体,断言只返回作用域内 active 设备,
//   且跨作用域(其它支线租户/空作用域)不泄漏——防止"C2 级联只在 service 层声明、DAL 仍等值过滤"的假覆盖。
package dal

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestGetDeviceListByPageForScopesFiltersAcrossTenantScope(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()
	seed := func(id, tenant string) {
		if err := db.Create(&model.Device{
			ID:           id,
			Voucher:      `{"username":"` + id + `"}`,
			TenantID:     tenant,
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: id,
		}).Error; err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
		if err := db.Create(&model.TelemetryCurrentData{DeviceID: id, Key: "temperature", T: now.Add(-time.Minute)}).Error; err != nil {
			t.Fatalf("seed telemetry %s: %v", id, err)
		}
	}
	seed("scope-t1-a", "tenant-1")
	seed("scope-t1-b", "tenant-1")
	seed("scope-t2-a", "tenant-2")
	seed("scope-t3-a", "tenant-3")

	req := &model.GetDeviceListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 20}}
	ids := func(list []model.GetDeviceListByPageRsp) map[string]bool {
		m := map[string]bool{}
		for _, item := range list {
			m[item.ID] = true
		}
		return m
	}

	// 单作用域: 与旧 tenantID 行为等价。
	count, list, err := GetDeviceListByPageForScopes(req, []string{"tenant-1"})
	if err != nil {
		t.Fatalf("single scope err: %v", err)
	}
	got := ids(list)
	if count != 2 || !got["scope-t1-a"] || !got["scope-t1-b"] || got["scope-t2-a"] || got["scope-t3-a"] {
		t.Fatalf("single scope must only include tenant-1, got count=%d ids=%#v", count, got)
	}

	// 级联作用域 [self, 祖先]: 子租户(tenant-3, parent=tenant-1 链)可读 self+祖先数据。
	count, list, err = GetDeviceListByPageForScopes(req, []string{"tenant-3", "tenant-1"})
	if err != nil {
		t.Fatalf("scoped query err: %v", err)
	}
	got = ids(list)
	if count != 3 || !got["scope-t1-a"] || !got["scope-t3-a"] || got["scope-t2-a"] {
		t.Fatalf("scope {tenant-3, tenant-1} must include both but not tenant-2, got count=%d ids=%#v", count, got)
	}

	// 空作用域 + 非 AllTenants: 维持空结果守卫(不因跳过过滤而全量泄漏)。
	count, list, err = GetDeviceListByPageForScopes(req, nil)
	if err != nil {
		t.Fatalf("nil scope err: %v", err)
	}
	if count != 0 || len(list) != 0 {
		t.Fatalf("nil scope must return empty when AllTenants=false, got count=%d", count)
	}
}
