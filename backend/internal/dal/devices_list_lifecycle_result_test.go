// 文件用途: 对 REQ-05b lifecycle_status 筛选做真实结果集断言(非仅翻译函数形状)。
// 核心逻辑: seed active/inactive 设备后真跑 GetDeviceListByPage,断言每个 lifecycle 值返回的确切设备集合。
// 关键注意事项: 这是防"假覆盖"的关键——deviceListLifecycleCondition 的形状测试无法证明真实过滤结果,必须对 DB 结果断言。
// 重构建议: 新增"已安装/传输完成"状态字段后,同步补该状态的结果集用例。
package dal

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
)

func TestGetDeviceListByPageLifecycleStatusFiltersRealResultSet(t *testing.T) {
	db := setupDeviceDALTestDB(t)
	now := time.Now().UTC()

	devices := []model.Device{
		{
			ID:           "t1-active-a",
			Voucher:      `{"username":"t1-active-a"}`,
			TenantID:     "tenant-1",
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "t1-active-a",
		},
		{
			ID:           "t1-active-b",
			Voucher:      `{"username":"t1-active-b"}`,
			TenantID:     "tenant-1",
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "t1-active-b",
		},
		{
			ID:           "t1-inactive",
			Voucher:      `{"username":"t1-inactive"}`,
			TenantID:     "tenant-1",
			IsEnabled:    "enabled",
			ActivateFlag: "inactive",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "t1-inactive",
		},
		{
			// 跨租户设备,任何 lifecycle 查询都不应泄漏它。
			ID:           "t2-active",
			Voucher:      `{"username":"t2-active"}`,
			TenantID:     "tenant-2",
			IsEnabled:    "enabled",
			ActivateFlag: "active",
			CreatedAt:    &now,
			UpdateAt:     &now,
			DeviceNumber: "t2-active",
		},
	}
	for _, device := range devices {
		device := device
		if err := db.Create(&device).Error; err != nil {
			t.Fatalf("create device %s: %v", device.ID, err)
		}
	}
	for _, current := range []model.TelemetryCurrentData{
		{DeviceID: "t1-active-a", Key: "temperature", T: now.Add(-2 * time.Minute)},
		{DeviceID: "t1-inactive", Key: "temperature", T: now.Add(-time.Minute)},
		{DeviceID: "t2-active", Key: "temperature", T: now.Add(-time.Minute)},
	} {
		current := current
		if err := db.Create(&current).Error; err != nil {
			t.Fatalf("create telemetry current row for %s: %v", current.DeviceID, err)
		}
	}

	ids := func(list []model.GetDeviceListByPageRsp) map[string]bool {
		m := map[string]bool{}
		for _, item := range list {
			m[item.ID] = true
		}
		return m
	}
	strPtr := func(s string) *string { return &s }

	// 缺省(nil): 保持历史 active-only 行为,只返回 tenant-1 的两台 active,不含 inactive/跨租户。
	count, list, err := GetDeviceListByPage(&model.GetDeviceListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 20},
	}, "tenant-1")
	if err != nil {
		t.Fatalf("default lifecycle query error: %v", err)
	}
	got := ids(list)
	if count != 2 || !got["t1-active-a"] || !got["t1-active-b"] {
		t.Fatalf("default should return only tenant-1 active devices, got count=%d ids=%#v", count, got)
	}
	if got["t1-inactive"] || got["t2-active"] {
		t.Fatalf("default leaked inactive or cross-tenant device: %#v", got)
	}

	// activated: 与缺省等价,只 active。
	count, list, err = GetDeviceListByPage(&model.GetDeviceListByPageReq{
		LifecycleStatus: strPtr("activated"),
		PageReq:         model.PageReq{Page: 1, PageSize: 20},
	}, "tenant-1")
	if err != nil {
		t.Fatalf("activated lifecycle query error: %v", err)
	}
	got = ids(list)
	if count != 2 || !got["t1-active-a"] || !got["t1-active-b"] || got["t1-inactive"] || got["t2-active"] {
		t.Fatalf("activated should return only tenant-1 active devices, got count=%d ids=%#v", count, got)
	}

	// inactive: 只返回 tenant-1 未激活那台(客户"已安装"默认映射)。
	count, list, err = GetDeviceListByPage(&model.GetDeviceListByPageReq{
		LifecycleStatus: strPtr("inactive"),
		PageReq:         model.PageReq{Page: 1, PageSize: 20},
	}, "tenant-1")
	if err != nil {
		t.Fatalf("inactive lifecycle query error: %v", err)
	}
	got = ids(list)
	if count != 1 || !got["t1-inactive"] {
		t.Fatalf("inactive should return only the inactive device, got count=%d ids=%#v", count, got)
	}
	if got["t1-active-a"] || got["t1-active-b"] || got["t2-active"] {
		t.Fatalf("inactive leaked active or cross-tenant device: %#v", got)
	}

	// all: 返回 tenant-1 全部三台(active+inactive),但仍不跨租户。
	count, list, err = GetDeviceListByPage(&model.GetDeviceListByPageReq{
		LifecycleStatus: strPtr("all"),
		PageReq:         model.PageReq{Page: 1, PageSize: 20},
	}, "tenant-1")
	if err != nil {
		t.Fatalf("all lifecycle query error: %v", err)
	}
	got = ids(list)
	if count != 3 || !got["t1-active-a"] || !got["t1-active-b"] || !got["t1-inactive"] {
		t.Fatalf("all should return all three tenant-1 devices, got count=%d ids=%#v", count, got)
	}
	if got["t2-active"] {
		t.Fatalf("all leaked cross-tenant device: %#v", got)
	}

	// transmitted: telemetry-backed devices across lifecycle flags, still scoped to tenant-1.
	count, list, err = GetDeviceListByPage(&model.GetDeviceListByPageReq{
		LifecycleStatus: strPtr("transmitted"),
		PageReq:         model.PageReq{Page: 1, PageSize: 20},
	}, "tenant-1")
	if err != nil {
		t.Fatalf("transmitted lifecycle query error: %v", err)
	}
	got = ids(list)
	if count != 2 || !got["t1-active-a"] || !got["t1-inactive"] {
		t.Fatalf("transmitted should return only tenant-1 devices with current telemetry, got count=%d ids=%#v", count, got)
	}
	if got["t1-active-b"] || got["t2-active"] {
		t.Fatalf("transmitted leaked a device without telemetry or a cross-tenant device: %#v", got)
	}
}
