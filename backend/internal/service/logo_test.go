// 文件用途：验证 Logo 配置服务的默认值和租户展示规则。
// 核心逻辑：构造配置记录并断言读取、更新和空配置回退的响应形态。
// 关键注意事项：Logo 配置属于租户可见品牌资产，测试需避免跨租户读写和空路径污染。
// 重构建议：拆出配置仓储接口，补齐权限拒绝、文件路径异常和默认配置回退测试。
package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestLogoListResponsePreservesPublicSystemBrandingContract(t *testing.T) {
	remark := "default branding"
	logos := []*model.Logo{{
		ID:             "logo-1",
		SystemName:     "AetherLink IoT",
		LogoCache:      "/static/logo-cache.png",
		LogoBackground: "/static/logo-background.png",
		LogoLoading:    "/static/logo-loading.png",
		HomeBackground: "/static/home-background.png",
		Remark:         &remark,
	}}

	got := logoListResponse(1, logos)
	if len(got) != 2 {
		t.Fatalf("logo list payload keys = %#v, want exactly total/list", got)
	}
	if got["total"] != int64(1) {
		t.Fatalf("logo list total = %#v, want 1", got["total"])
	}
	if _, ok := got["data"]; ok {
		t.Fatalf("logo list payload should not expose data instead of list: %#v", got)
	}

	gotList, ok := got["list"].([]*model.Logo)
	if !ok {
		t.Fatalf("logo list payload list type = %T, want []*model.Logo", got["list"])
	}
	if len(gotList) != 1 {
		t.Fatalf("logo list length = %d, want 1", len(gotList))
	}

	item := gotList[0]
	if item.ID != "logo-1" ||
		item.SystemName != "AetherLink IoT" ||
		item.LogoCache != "/static/logo-cache.png" ||
		item.LogoBackground != "/static/logo-background.png" ||
		item.LogoLoading != "/static/logo-loading.png" ||
		item.HomeBackground != "/static/home-background.png" ||
		item.Remark == nil ||
		*item.Remark != "default branding" {
		t.Fatalf("logo list item contract changed: %#v", item)
	}
}
