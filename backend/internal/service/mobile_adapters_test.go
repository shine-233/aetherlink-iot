package service

import (
	"context"
	"testing"

	utils "aetherlink-iot/backend/pkg/utils"
)

// TestMobileAdaptersRefuseWithoutClaims 三个适配器在没有凭证时一律拒绝。
// 归属过滤由 claims.Authority 决定，没有凭证就无从判断可见范围——
// 此时"返回空列表"会被移动端渲染成"你还没有设备/告警"。
func TestMobileAdaptersRefuseWithoutClaims(t *testing.T) {
	ctx := context.Background()
	lister := NewMobileDeviceLister()
	alarmLister := NewMobileAlarmLister()
	acker := NewMobileAlarmAcker()
	shadow := NewMobileShadowStore()

	if _, _, err := lister.List(ctx, nil, "", 1, 10); err == nil {
		t.Fatal("device list must refuse without claims")
	}
	if _, err := alarmLister.List(ctx, nil, 1, 10); err == nil {
		t.Fatal("alarm list must refuse without claims")
	}
	if err := acker.Acknowledge(ctx, nil, "a1"); err == nil {
		t.Fatal("alarm ack must refuse without claims")
	}
	if _, err := shadow.Get(ctx, nil, "d1"); err == nil {
		t.Fatal("shadow read must refuse without claims")
	}
	if err := shadow.Update(ctx, nil, "d1", `{}`); err == nil {
		t.Fatal("shadow update must refuse without claims")
	}
}

// TestMobileAdaptersRejectIncompleteIdentifiers 缺设备/告警 ID 直接拒绝，
// 不让空 ID 变成一个"查不到"的查询。
func TestMobileAdaptersRejectIncompleteIdentifiers(t *testing.T) {
	ctx := context.Background()
	claims := &utils.UserClaims{ID: "u1", TenantID: "t1", Authority: "TENANT_USER"}

	if err := NewMobileAlarmAcker().Acknowledge(ctx, claims, "  "); err == nil {
		t.Fatal("alarm ack must reject an empty alarm id")
	}
	if _, err := NewMobileShadowStore().Get(ctx, claims, " "); err == nil {
		t.Fatal("shadow read must reject an empty device id")
	}
	if err := NewMobileShadowStore().Update(ctx, claims, "", `{}`); err == nil {
		t.Fatal("shadow update must reject an empty device id")
	}
}

// TestMobileShadowUpdateRejectsInvalidPayload 影子载荷必须是合法 JSON。
// 静默存一份设备侧永远解析不了的字节，比直接报错更糟：故障会推迟到设备上线才暴露。
func TestMobileShadowUpdateRejectsInvalidPayload(t *testing.T) {
	ctx := context.Background()
	claims := &utils.UserClaims{ID: "u1", TenantID: "t1", Authority: "TENANT_ADMIN"}
	shadow := NewMobileShadowStore()

	for name, payload := range map[string]string{
		"empty":    "   ",
		"not json": `{"method":`,
		"garbage":  `set_temp(30)`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := shadow.Update(ctx, claims, "d1", payload); err == nil {
				t.Fatalf("payload %q must be rejected", payload)
			}
		})
	}
}

// TestClampMobilePage 分页必须夹紧。page_size 直接下推会让 0 变成"不限量"，
// 一次拉全表是移动端最典型的拖垮方式。
func TestClampMobilePage(t *testing.T) {
	cases := []struct {
		name             string
		page, pageSize   int
		wantPage, wantPS int
	}{
		{name: "zero falls back to defaults", page: 0, pageSize: 0, wantPage: 1, wantPS: mobileDefaultPageSize},
		{name: "negative falls back", page: -3, pageSize: -1, wantPage: 1, wantPS: mobileDefaultPageSize},
		{name: "oversized page size is capped", page: 2, pageSize: 100000, wantPage: 2, wantPS: mobileMaxPageSize},
		{name: "normal values pass through", page: 3, pageSize: 25, wantPage: 3, wantPS: 25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, pageSize := clampMobilePage(tc.page, tc.pageSize)
			if page != tc.wantPage || pageSize != tc.wantPS {
				t.Fatalf("clampMobilePage(%d,%d) = (%d,%d), want (%d,%d)",
					tc.page, tc.pageSize, page, pageSize, tc.wantPage, tc.wantPS)
			}
		})
	}
}

// TestMobileAlarmListFromMapPreservesTotal 告警列表的 total 必须能接住既有实现
// 返回的几种整型形态；接不住时静默变 0，移动端会一直显示"没有更多"。
func TestMobileAlarmListFromMapPreservesTotal(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]interface{}
		want int64
	}{
		{name: "int64", in: map[string]interface{}{"total": int64(7)}, want: 7},
		{name: "int", in: map[string]interface{}{"total": 5}, want: 5},
		{name: "float64", in: map[string]interface{}{"total": float64(3)}, want: 3},
		{name: "missing", in: map[string]interface{}{}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mobileAlarmListFromMap(tc.in).Total; got != tc.want {
				t.Fatalf("total = %d, want %d", got, tc.want)
			}
		})
	}
}
