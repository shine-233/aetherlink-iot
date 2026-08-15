// 文件用途：device_twin_drift_query.go 中不依赖 DB 的纯辅助函数单元测试。
// 覆盖:only_drift 过滤(保留 severity>0 条目但不改分类计数)、设备名映射、
// max_devices 归一化(默认/上限/合法值)、列表请求构建(筛选字段透传)。
// 不测 GetDeviceTwinDriftIndex 的 DAL 枚举路径——那需真实 PG,属运行时验证。
package service

import (
	"testing"

	model "aetherlink-iot/backend/internal/model"
)

func TestFilterDeviceTwinDriftOnlyDrift_KeepsSeverityButPreservesCounts(t *testing.T) {
	index := model.DeviceTwinDriftIndex{
		Entries: []model.DeviceTwinDriftEntry{
			{DeviceID: "a", Severity: 40},
			{DeviceID: "b", Severity: 0},
			{DeviceID: "c", Severity: 10},
			{DeviceID: "d", Severity: 0},
		},
		TotalDevices:     4,
		DriftDevices:     1,
		ReadyDevices:     2,
		NoDesiredDevices: 1,
	}

	filtered := filterDeviceTwinDriftOnlyDrift(index)

	if len(filtered.Entries) != 2 {
		t.Fatalf("entries = %d, want 2 (only severity>0)", len(filtered.Entries))
	}
	for _, e := range filtered.Entries {
		if e.Severity <= 0 {
			t.Fatalf("entry %q severity %d should have been filtered out", e.DeviceID, e.Severity)
		}
	}
	// 分类计数是"整个查询范围"的规模,过滤不得改动它们。
	if filtered.TotalDevices != 4 || filtered.ReadyDevices != 2 || filtered.NoDesiredDevices != 1 {
		t.Fatalf("category counts must be preserved, got %+v", filtered)
	}
}

func TestFilterDeviceTwinDriftOnlyDrift_EmptyEntries(t *testing.T) {
	filtered := filterDeviceTwinDriftOnlyDrift(model.DeviceTwinDriftIndex{TotalDevices: 3, ReadyDevices: 3})
	if len(filtered.Entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(filtered.Entries))
	}
	if filtered.TotalDevices != 3 {
		t.Fatalf("TotalDevices = %d, want 3", filtered.TotalDevices)
	}
}

func TestDeviceTwinDriftNameByID_MapsAndSkipsBlank(t *testing.T) {
	rows := []model.GetDeviceListByPageRsp{
		{ID: "dev-1", Name: "boiler"},
		{ID: "  ", Name: "ignored-blank-id"},
		{ID: "dev-2", Name: "chiller"},
	}
	nameByID := deviceTwinDriftNameByID(rows)

	if len(nameByID) != 2 {
		t.Fatalf("map size = %d, want 2 (blank id skipped)", len(nameByID))
	}
	if nameByID["dev-1"] != "boiler" || nameByID["dev-2"] != "chiller" {
		t.Fatalf("unexpected name mapping: %+v", nameByID)
	}
}

func TestNormalizeDeviceTwinDriftIndexMaxDevices(t *testing.T) {
	cases := []struct {
		name string
		req  *model.DeviceTwinDriftIndexReq
		want int
	}{
		{"nil req -> default", nil, defaultDeviceTwinDriftIndexMaxDevices},
		{"zero -> default", &model.DeviceTwinDriftIndexReq{MaxDevices: 0}, defaultDeviceTwinDriftIndexMaxDevices},
		{"negative -> default", &model.DeviceTwinDriftIndexReq{MaxDevices: -5}, defaultDeviceTwinDriftIndexMaxDevices},
		{"in range kept", &model.DeviceTwinDriftIndexReq{MaxDevices: 250}, 250},
		{"over cap clamped", &model.DeviceTwinDriftIndexReq{MaxDevices: 9999}, maxDeviceTwinDriftIndexMaxDevices},
		{"at cap kept", &model.DeviceTwinDriftIndexReq{MaxDevices: maxDeviceTwinDriftIndexMaxDevices}, maxDeviceTwinDriftIndexMaxDevices},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeDeviceTwinDriftIndexMaxDevices(c.req); got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestBuildDeviceTwinDriftDeviceListReq_PassesFiltersAndPaging(t *testing.T) {
	group := "grp-1"
	cfg := "cfg-1"
	product := "prod-1"
	search := "boiler"
	online := 1
	req := &model.DeviceTwinDriftIndexReq{
		GroupId:        &group,
		DeviceConfigId: &cfg,
		ProductID:      &product,
		Search:         &search,
		IsOnline:       &online,
	}

	listReq := buildDeviceTwinDriftDeviceListReq(req, 123)

	if listReq.Page != 1 || listReq.PageSize != 123 {
		t.Fatalf("paging = page %d size %d, want page 1 size 123", listReq.Page, listReq.PageSize)
	}
	if listReq.GroupId != &group || listReq.DeviceConfigId != &cfg || listReq.ProductID != &product {
		t.Fatalf("filter pointers not passed through")
	}
	if listReq.Search != &search || listReq.IsOnline != &online {
		t.Fatalf("search/online not passed through")
	}
}

func TestBuildDeviceTwinDriftDeviceListReq_NilReqStillPages(t *testing.T) {
	listReq := buildDeviceTwinDriftDeviceListReq(nil, 50)
	if listReq.Page != 1 || listReq.PageSize != 50 {
		t.Fatalf("nil req should still set paging, got page %d size %d", listReq.Page, listReq.PageSize)
	}
}
