// 文件用途：规则链 CRUD 与执行入口回归测试（ROADMAP B2）。
// 核心逻辑：sqlite 内存库验证租户守卫、创建/更新/删除回环、缓存失效后可见性。
// 关键注意事项：空租户 claims 必须 fail-closed；跨租户读写不可见。
package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/require"
)

const simpleChainGraph = `{"nodes":[
	{"id":"t","type":"trigger.telemetry"},
	{"id":"m","type":"transform.mapping","config":{"fields":{"temperature":"temp"}}}
],"edges":[{"from":"t","to":"m"}]}`

func modbusChainClaims(tenant string) *utils.UserClaims {
	return &utils.UserClaims{ID: "user-1", TenantID: tenant, Authority: constant.TENANT_ADMIN}
}

func TestRuleChainCrudRoundtripAndTenantIsolation(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.RuleChain{}))
	svc := &RuleChain{}

	body := []byte(`{"name":"high-temp-hook","graph":` + simpleChainGraph + `}`)
	created, err := svc.CreateChain(body, modbusChainClaims("tenant-1"))
	require.NoError(t, err)
	require.Equal(t, "tenant-1", created.TenantID)
	require.NotEmpty(t, created.ID)

	// 同租户可见
	got, err := svc.GetChain(created.ID, modbusChainClaims("tenant-1"))
	require.NoError(t, err)
	require.Equal(t, "high-temp-hook", got.Name)

	// 异租户不可见
	_, err = svc.GetChain(created.ID, modbusChainClaims("tenant-2"))
	require.Error(t, err)

	// 空租户 claims fail-closed
	_, err = svc.CreateChain(body, &utils.UserClaims{ID: "user-x", TenantID: "", Authority: constant.SYS_ADMIN})
	require.Error(t, err)

	// 列表
	listResp, err := svc.ListChains("", 1, 20, modbusChainClaims("tenant-1"))
	require.NoError(t, err)
	require.Equal(t, int64(1), listResp["total"])

	// 更新启用状态
	updateBody := []byte(`{"id":"` + created.ID + `","name":"high-temp-hook-v2","enabled":false,"graph":` + simpleChainGraph + `}`)
	updated, err := svc.UpdateChain(updateBody, modbusChainClaims("tenant-1"))
	require.NoError(t, err)
	require.False(t, updated.Enabled)

	// 删除
	require.NoError(t, svc.DeleteChain(created.ID, modbusChainClaims("tenant-1")))
	listResp2, err := svc.ListChains("", 1, 20, modbusChainClaims("tenant-1"))
	require.NoError(t, err)
	require.Equal(t, int64(0), listResp2["total"])
}

func TestRuleChainCreateRejectsInvalidGraph(t *testing.T) {
	db := setupDeviceServiceTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.RuleChain{}))
	svc := &RuleChain{}

	cyclic := `{"nodes":[
		{"id":"t","type":"trigger.telemetry"},
		{"id":"n","type":"transform.mapping","config":{"fields":{"a":"b"}}}
	],"edges":[{"from":"t","to":"n"},{"from":"n","to":"t"}]}`
	_, err := svc.CreateChain([]byte(`{"name":"cyclic","graph":`+cyclic+`}`), modbusChainClaims("tenant-1"))
	require.Error(t, err)
}
