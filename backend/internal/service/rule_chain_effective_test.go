// 文件用途：TB-18 档案级默认规则链解析的单测——纯函数合并语义与 sqlite 库上执行/解析/绑定/删除守卫。
// 核心逻辑：resolveEffectiveRuleChainGraphs 验证档案链优先、租户链兜底、ChainID 去重、顺序稳定；
//
//	GetEffectiveRuleChainsForDevice / ResolveEffectiveRuleChainsForDevice 在内存库验证
//	档案绑定、停用链跳过、跨租户 fail-closed、共享只读拒绝；UpdateDeviceConfig 验证
//	default_rule_chain_id 绑定/解绑/跨租户拒绝；DeleteChain 验证档案引用删除守卫。
//
// 关键注意事项：用例前后必须清 rule chain 双缓存（租户级 + 档案级），防止用例间串扰；
//
//	走 UpdateDeviceConfig 成功路径需要 miniredis（DelDeviceConfigCache 不接受 nil REDIS）。
// 重构建议：若档案解析继续扩面（队列、告警收敛），按同结构补解析矩阵用例而非堆进本文件。
package service

import (
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"
)

// tb18ChainIDOf 按顺序抽取解析结果的 ChainID 列表，便于做顺序敏感断言。
func tb18ChainIDs(graphs []*RuleChainGraph) []string {
	ids := make([]string, 0, len(graphs))
	for _, g := range graphs {
		if g == nil {
			continue
		}
		ids = append(ids, g.ChainID)
	}
	return ids
}

func tb18Graph(id string) *RuleChainGraph {
	return &RuleChainGraph{ChainID: id}
}

// resetTB18RuleChainCaches 清空租户级与档案级链缓存（用例前后各调一次，防串扰）。
func resetTB18RuleChainCaches(t *testing.T, tenants ...string) {
	t.Helper()
	clean := func() {
		for _, tenant := range tenants {
			invalidateRuleChainCache(tenant)
		}
		ruleChainProfileCacheMu.Lock()
		ruleChainProfileCacheByID = map[string]ruleChainProfileCacheEntry{}
		ruleChainProfileCacheMu.Unlock()
	}
	clean()
	t.Cleanup(clean)
}

func TestResolveEffectiveRuleChainGraphsProfileFirstTenantFallbackDedup(t *testing.T) {
	// 档案链优先、租户链按原顺序兜底。
	got := resolveEffectiveRuleChainGraphs(
		[]*RuleChainGraph{tb18Graph("p1")},
		[]*RuleChainGraph{tb18Graph("t1"), tb18Graph("t2")},
	)
	require.Equal(t, []string{"p1", "t1", "t2"}, tb18ChainIDs(got))

	// 去重：档案链同时出现在租户启用链里时只保留档案位次。
	got = resolveEffectiveRuleChainGraphs(
		[]*RuleChainGraph{tb18Graph("p1")},
		[]*RuleChainGraph{tb18Graph("p1"), tb18Graph("t1")},
	)
	require.Equal(t, []string{"p1", "t1"}, tb18ChainIDs(got))

	// 顺序稳定：只交换租户输入顺序，输出随之交换，不引入额外排序。
	got = resolveEffectiveRuleChainGraphs(
		[]*RuleChainGraph{tb18Graph("p1")},
		[]*RuleChainGraph{tb18Graph("t2"), tb18Graph("t1")},
	)
	require.Equal(t, []string{"p1", "t2", "t1"}, tb18ChainIDs(got))

	// 空档案组退化为租户链；nil 图被跳过；空 ChainID 不参与去重（调用方须显式设置 ChainID）。
	got = resolveEffectiveRuleChainGraphs(nil, []*RuleChainGraph{tb18Graph("t1")})
	require.Equal(t, []string{"t1"}, tb18ChainIDs(got))

	got = resolveEffectiveRuleChainGraphs(
		[]*RuleChainGraph{nil, tb18Graph("p1")},
		[]*RuleChainGraph{tb18Graph(""), tb18Graph("")},
	)
	require.Equal(t, []string{"p1", "", ""}, tb18ChainIDs(got))

	// 同租户去重：租户组自身重复的 ChainID 也只保留一次。
	got = resolveEffectiveRuleChainGraphs(nil, []*RuleChainGraph{tb18Graph("t1"), tb18Graph("t1")})
	require.Equal(t, []string{"t1"}, tb18ChainIDs(got))
}

// tb18SeedChain 写入一条规则链行（sqlite，AutoMigrate 生成表）。
// 注意：模型 Enabled 带 default:true 标签，struct Create 的 false 会被 gorm 零值省略成
// 库默认 true——与生产一致，停用态必须走 map 更新显式落 false。
func tb18SeedChain(t *testing.T, db *gorm.DB, id, name, tenantID string, enabled bool) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, db.Create(&model.RuleChain{
		ID:        id,
		TenantID:  tenantID,
		Name:      name,
		Enabled:   true,
		Graph:     []byte(simpleChainGraph),
		CreatedAt: &now,
		UpdatedAt: &now,
	}).Error)
	if !enabled {
		require.NoError(t, db.Model(&model.RuleChain{}).Where("id = ?", id).Update("enabled", false).Error)
	}
}

// tb18SeedConfig 写入一条设备档案行，可指定默认规则链。
func tb18SeedConfig(t *testing.T, db *gorm.DB, id, tenantID string, defaultChainID *string) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, db.Create(&model.DeviceConfig{
		ID:                 id,
		Name:               id,
		DeviceType:         "1",
		TenantID:           tenantID,
		DefaultRuleChainID: defaultChainID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}).Error)
}

func TestGetEffectiveRuleChainsForDeviceResolution(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.RuleChain{}))
	resetTB18RuleChainCaches(t, "tenant-1", "tenant-2")

	tb18SeedChain(t, db, "chain-a", "tenant-a-chain", "tenant-1", true)
	tb18SeedChain(t, db, "chain-b", "tenant-b-chain", "tenant-1", true)
	tb18SeedChain(t, db, "chain-off", "disabled-chain", "tenant-1", false)
	tb18SeedChain(t, db, "chain-t2", "tenant2-chain", "tenant-2", true)

	boundB := "chain-b"
	boundOff := "chain-off"
	boundMissing := "chain-missing"
	tb18SeedConfig(t, db, "cfg-b", "tenant-1", &boundB)
	tb18SeedConfig(t, db, "cfg-none", "tenant-1", nil)
	tb18SeedConfig(t, db, "cfg-off", "tenant-1", &boundOff)
	tb18SeedConfig(t, db, "cfg-missing", "tenant-1", &boundMissing)

	deviceWith := func(id, tenantID, configID string) model.Device {
		cfg := configID
		return model.Device{ID: id, TenantID: tenantID, DeviceConfigID: &cfg}
	}

	// 档案绑定链优先，租户链兜底且去重（chain-b 同时是租户启用链，只出现在档案位次）。
	got := GetEffectiveRuleChainsForDevice(deviceWith("d1", "tenant-1", "cfg-b"))
	require.Equal(t, []string{"chain-b", "chain-a"}, tb18ChainIDs(got))

	// 未绑档案 / 档案未绑链：纯租户级链，顺序与启用链一致。
	got = GetEffectiveRuleChainsForDevice(model.Device{ID: "d2", TenantID: "tenant-1"})
	require.Equal(t, []string{"chain-a", "chain-b"}, tb18ChainIDs(got))
	got = GetEffectiveRuleChainsForDevice(deviceWith("d3", "tenant-1", "cfg-none"))
	require.Equal(t, []string{"chain-a", "chain-b"}, tb18ChainIDs(got))

	// 绑定链已停用 / 已不存在：跳过档案链，回落租户级。
	got = GetEffectiveRuleChainsForDevice(deviceWith("d4", "tenant-1", "cfg-off"))
	require.Equal(t, []string{"chain-a", "chain-b"}, tb18ChainIDs(got))
	got = GetEffectiveRuleChainsForDevice(deviceWith("d5", "tenant-1", "cfg-missing"))
	require.Equal(t, []string{"chain-a", "chain-b"}, tb18ChainIDs(got))

	// 跨租户档案 fail-closed：tenant-2 的设备挂 tenant-1 的档案，档案链不生效。
	got = GetEffectiveRuleChainsForDevice(deviceWith("d6", "tenant-2", "cfg-b"))
	require.Equal(t, []string{"chain-t2"}, tb18ChainIDs(got))

	// 空租户 fail-closed。
	require.Empty(t, GetEffectiveRuleChainsForDevice(model.Device{ID: "d7", TenantID: ""}))
}

func TestResolveEffectiveRuleChainsForDeviceAccessAndContract(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.RuleChain{}))
	resetTB18RuleChainCaches(t, "tenant-1", "tenant-2")

	tb18SeedChain(t, db, "chain-a", "tenant-a-chain", "tenant-1", true)
	tb18SeedChain(t, db, "chain-b", "tenant-b-chain", "tenant-1", true)
	tb18SeedChain(t, db, "chain-off", "disabled-chain", "tenant-1", false)
	boundB := "chain-b"
	boundOff := "chain-off"
	tb18SeedConfig(t, db, "cfg-b", "tenant-1", &boundB)
	tb18SeedConfig(t, db, "cfg-off", "tenant-1", &boundOff)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&model.Device{
		ID: "d1", DeviceNumber: "d1-number", Voucher: "d1-voucher",
		TenantID: "tenant-1", IsEnabled: "enabled", ActivateFlag: "active",
		CreatedAt: &now, UpdateAt: &now, DeviceConfigID: &boundB,
	}).Error)
	cfgB := "cfg-b"
	require.NoError(t, db.Model(&model.Device{}).Where("id = ?", "d1").Update("device_config_id", cfgB).Error)
	require.NoError(t, db.Create(&model.Device{
		ID: "d-off", DeviceNumber: "d-off-number", Voucher: "d-off-voucher",
		TenantID: "tenant-1", IsEnabled: "enabled", ActivateFlag: "active",
		CreatedAt: &now, UpdateAt: &now, DeviceConfigID: &boundOff,
	}).Error)
	cfgOff := "cfg-off"
	require.NoError(t, db.Model(&model.Device{}).Where("id = ?", "d-off").Update("device_config_id", cfgOff).Error)

	svc := &RuleChain{}
	admin := &utils.UserClaims{ID: "admin-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}

	// 契约：档案绑定元数据回显 + 生效链清单（档案链在前、租户链兜底、source 标注）。
	res, err := svc.ResolveEffectiveRuleChainsForDevice("d1", admin)
	require.NoError(t, err)
	require.Equal(t, "d1", res.DeviceID)
	require.NotNil(t, res.DeviceConfigID)
	require.Equal(t, "cfg-b", *res.DeviceConfigID)
	require.NotNil(t, res.DefaultRuleChainID)
	require.Equal(t, "chain-b", *res.DefaultRuleChainID)
	require.Len(t, res.Chains, 2)
	require.Equal(t, "chain-b", res.Chains[0].ID)
	require.Equal(t, "profile", res.Chains[0].Source)
	require.Equal(t, "tenant-b-chain", res.Chains[0].Name)
	require.Equal(t, "chain-a", res.Chains[1].ID)
	require.Equal(t, "tenant", res.Chains[1].Source)

	// 绑定值已停用：default_rule_chain_id 原样回显，但不出现在生效清单。
	res, err = svc.ResolveEffectiveRuleChainsForDevice("d-off", admin)
	require.NoError(t, err)
	require.NotNil(t, res.DefaultRuleChainID)
	require.Equal(t, "chain-off", *res.DefaultRuleChainID)
	require.Len(t, res.Chains, 2)
	for _, ref := range res.Chains {
		require.NotEqual(t, "chain-off", ref.ID)
		require.Equal(t, "tenant", ref.Source)
	}

	// 跨租户 fail-closed；共享只读（TENANT_USER 且非 owner）拒绝；nil claims 拒绝；空 id 参数错误。
	_, err = svc.ResolveEffectiveRuleChainsForDevice("d1", &utils.UserClaims{ID: "admin-2", TenantID: "tenant-2", Authority: constant.TENANT_ADMIN})
	require.Error(t, err)
	_, err = svc.ResolveEffectiveRuleChainsForDevice("d1", &utils.UserClaims{ID: "user-1", TenantID: "tenant-1", Authority: constant.TENANT_USER})
	require.Error(t, err)
	_, err = svc.ResolveEffectiveRuleChainsForDevice("d1", nil)
	require.Error(t, err)
	_, err = svc.ResolveEffectiveRuleChainsForDevice("", admin)
	require.Error(t, err)
}

func TestUpdateDeviceConfigDefaultRuleChainBindUnbindFlow(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.RuleChain{}))
	resetTB18RuleChainCaches(t, "tenant-1", "tenant-2")

	// UpdateDeviceConfig 成功路径会调 DelDeviceConfigCache，需要可用的 REDIS。
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	oldRedis := global.REDIS
	global.REDIS = client
	t.Cleanup(func() { global.REDIS = oldRedis })

	tb18SeedChain(t, db, "chain-x", "own-chain", "tenant-1", true)
	tb18SeedChain(t, db, "chain-y", "other-chain", "tenant-2", true)
	tb18SeedConfig(t, db, "cfg-9", "tenant-1", nil)

	svc := &DeviceConfig{}
	claims := &utils.UserClaims{ID: "admin-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}

	// 绑定：透传落库并在详情回显。
	_, err := svc.UpdateDeviceConfig(model.UpdateDeviceConfigReq{
		Id:                 "cfg-9",
		DefaultRuleChainId: StringPtr("chain-x"),
	}, claims)
	require.NoError(t, err)
	var cfg model.DeviceConfig
	require.NoError(t, db.First(&cfg, "id = ?", "cfg-9").Error)
	require.NotNil(t, cfg.DefaultRuleChainID)
	require.Equal(t, "chain-x", *cfg.DefaultRuleChainID)

	// 跨租户绑定拒绝（fail-closed）。
	_, err = svc.UpdateDeviceConfig(model.UpdateDeviceConfigReq{
		Id:                 "cfg-9",
		DefaultRuleChainId: StringPtr("chain-y"),
	}, claims)
	require.Error(t, err)
	require.Contains(t, err.Error(), "default_rule_chain_id is not available for this tenant")

	// 不存在的链拒绝。
	_, err = svc.UpdateDeviceConfig(model.UpdateDeviceConfigReq{
		Id:                 "cfg-9",
		DefaultRuleChainId: StringPtr("chain-404"),
	}, claims)
	require.Error(t, err)

	// 解绑（空字符串约定）：落库为 NULL。
	_, err = svc.UpdateDeviceConfig(model.UpdateDeviceConfigReq{
		Id:                 "cfg-9",
		DefaultRuleChainId: StringPtr(""),
	}, claims)
	require.NoError(t, err)
	require.NoError(t, db.First(&cfg, "id = ?", "cfg-9").Error)
	require.Nil(t, cfg.DefaultRuleChainID)
}

func TestDeleteChainGuardAgainstProfileBinding(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.RuleChain{}))
	resetTB18RuleChainCaches(t, "tenant-1")

	tb18SeedChain(t, db, "chain-x", "bound-chain", "tenant-1", true)
	boundX := "chain-x"
	tb18SeedConfig(t, db, "cfg-bound", "tenant-1", &boundX)

	svc := &RuleChain{}
	claims := &utils.UserClaims{ID: "admin-1", TenantID: "tenant-1", Authority: constant.TENANT_ADMIN}

	// 仍被档案引用：删除被拒绝。
	err := svc.DeleteChain("chain-x", claims)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unbind it first")

	// 解绑后可删除。
	require.NoError(t, db.Model(&model.DeviceConfig{}).Where("id = ?", "cfg-bound").Update("default_rule_chain_id", nil).Error)
	require.NoError(t, svc.DeleteChain("chain-x", claims))
	var count int64
	require.NoError(t, db.Model(&model.RuleChain{}).Where("id = ?", "chain-x").Count(&count).Error)
	require.Equal(t, int64(0), count)
}
