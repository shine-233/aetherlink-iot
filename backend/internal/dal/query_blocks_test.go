package dal

// 文件用途：query_blocks.go 共享查询构件的回归测试。
// 核心逻辑：用内存 sqlite + assets 表做行为级断言（不测生成的 SQL 文本），
//   覆盖四类 helper 各自最容易回归的那条性质。
// 关键注意事项：
//   - 复用 asset_test.go 的 setupAssetDB / seedAsset，避免重复造测试装置。
//   - fail-closed 是本包的安全前提：空作用域必须返回空结果而不是扫全表，断言要卡死这条。

import (
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/stretchr/testify/require"
)

func TestNormalizePageParamsClamps(t *testing.T) {
	page, size := normalizePageParams(0, 0, 10, maxListLimit)
	require.Equal(t, 1, page)
	require.Equal(t, 10, size)

	page, size = normalizePageParams(-3, -1, 10, maxListLimit)
	require.Equal(t, 1, page)
	require.Equal(t, 10, size)

	// 单页大小必须被硬上限夹紧，超过上限的请求不能把整表读进来。
	page, size = normalizePageParams(2, maxListLimit*10, 10, maxListLimit)
	require.Equal(t, 2, page)
	require.Equal(t, maxListLimit, size)

	// 上限为 0 表示不夹紧（调用方自行保证有界）。
	_, size = normalizePageParams(1, 5000, 10, 0)
	require.Equal(t, 5000, size)
}

func TestScopeTenantColumnEmptyScopeIsFailClosed(t *testing.T) {
	setupAssetDB(t)
	seedAsset(t, "a-scope", "t1", "", "可见资产")

	// 空作用域：既不能扫全表，也不能把一个错误抛给调用方，必须返回空集合。
	list, total, err := ListAssetsByPageWithGroupScope(nil, "", "", 1, 10, nil)
	require.NoError(t, err)
	require.Empty(t, list)
	require.Equal(t, int64(0), total)

	// 单租户与多租户两种形态都要命中。
	list, total, err = ListAssetsByPageWithGroupScope([]string{"t1"}, "", "", 1, 10, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)

	list, _, err = ListAssetsByPageWithGroupScope([]string{"t1", "t2"}, "", "", 1, 10, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestWhereKeywordContainsEscapesWildcards(t *testing.T) {
	setupAssetDB(t)
	seedAsset(t, "a-percent", "t1", "", "pump%")
	seedAsset(t, "a-plain", "t1", "", "pumpX")

	// 用户输入 "%" 是字面量而不是通配符：未转义时 "%" 会命中两条。
	list, total, err := ListAssetsByPageWithGroupScope([]string{"t1"}, "", "%", 1, 10, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	require.Equal(t, "a-percent", list[0].ID)

	// 下划线同理。
	seedAsset(t, "a-under", "t1", "", "pump_")
	list, _, err = ListAssetsByPageWithGroupScope([]string{"t1"}, "", "_", 1, 10, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "a-under", list[0].ID)

	// 关键词前后空白不应影响命中。
	list, _, err = ListAssetsByPageWithGroupScope([]string{"t1"}, "", "  pumpX  ", 1, 10, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "a-plain", list[0].ID)
}

func TestCountAndFindPageDoesNotPolluteStatement(t *testing.T) {
	setupAssetDB(t)
	for _, id := range []string{"a-1", "a-2", "a-3"} {
		seedAsset(t, id, "t1", "", "asset "+id)
	}

	page1, total, err := ListAssetsByPageWithGroupScope([]string{"t1"}, "", "", 1, 2, nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, page1, 2)

	// 第二次调用若复用了第一次的 Statement（旧实现的典型 bug），
	// 会把 Limit/Offset 叠加，导致第二页为空或报错。
	page2, total2, err := ListAssetsByPageWithGroupScope([]string{"t1"}, "", "", 2, 2, nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), total2)
	require.Len(t, page2, 1)

	seen := make(map[string]struct{}, 3)
	for _, a := range page1 {
		seen[a.ID] = struct{}{}
	}
	for _, a := range page2 {
		_, dup := seen[a.ID]
		require.False(t, dup, "页之间出现重复行: %s", a.ID)
		seen[a.ID] = struct{}{}
	}
	require.Len(t, seen, 3)
}

func TestCountAndFindPageRespectsOrder(t *testing.T) {
	setupAssetDB(t)
	seedAsset(t, "a-old", "t1", "", "old")
	seedAsset(t, "a-new", "t1", "", "new")
	earlier := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, global.DB.Model(&model.Asset{}).Where("id = ?", "a-old").Update("created_at", earlier).Error)

	list, _, err := ListAssetsByPageWithGroupScope([]string{"t1"}, "", "", 1, 10, nil)
	require.NoError(t, err)
	require.Len(t, list, 2)
	// created_at DESC：被改早的那条应排在后。
	require.Equal(t, "a-new", list[0].ID)
	require.Equal(t, "a-old", list[1].ID)
}
