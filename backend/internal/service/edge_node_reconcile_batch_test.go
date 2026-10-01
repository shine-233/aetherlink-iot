// 文件用途：锁定 ReconcileEdgeNode 下发循环的批量取资源语义——
// 一次 reconcile 对看板/规则链各只打一条 DAL 查询（不随计划项数线性增长），
// 且每条计划项仍然独立成败（一项冲突/出错不影响其余项继续下发），不包进同一个事务。
package service

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// countingQueryPlugin 统计对指定表发出的 SELECT 次数，用于断言批量查询确实只发一次。
type countingQueryPlugin struct {
	counts map[string]*int64
}

func (p *countingQueryPlugin) Name() string { return "counting_query_plugin" }

func (p *countingQueryPlugin) Initialize(db *gorm.DB) error {
	return db.Callback().Query().Before("gorm:query").Register("counting_query_plugin:before", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table == "" {
			return
		}
		if counter, ok := p.counts[tx.Statement.Table]; ok {
			atomic.AddInt64(counter, 1)
		}
	})
}

func setupEdgeReconcileBatchTestDB(t *testing.T) (*gorm.DB, *countingQueryPlugin) {
	t.Helper()
	oldDB := global.DB
	dbName := "edge_reconcile_batch_" + strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Device{}, &model.EdgeNode{}, &model.EdgeSyncTask{},
		&model.Board{}, &model.RuleChain{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	plugin := &countingQueryPlugin{counts: map[string]*int64{
		"boards":          new(int64),
		"rule_chains":     new(int64),
		"edge_sync_tasks": new(int64),
	}}
	if err := db.Use(plugin); err != nil {
		t.Fatalf("install plugin: %v", err)
	}
	global.DB = db
	query.SetDefault(db)
	// 版本闸门需要最低版本配置，否则所有计划项都会被 skip。
	oldMin := viper.Get(edgeMinCompatibleVersionKey)
	viper.Set(edgeMinCompatibleVersionKey, "1.0.0")
	t.Cleanup(func() {
		viper.Set(edgeMinCompatibleVersionKey, oldMin)
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db, plugin
}

// TestReconcileEdgeNodeBatchesResourceLookupsAcrossPlanItems 核心不变式：
// 下发循环对看板/规则链各只打一条批量 DAL 查询，查询数不随计划项数线性增长——
// 3 个看板 + 2 个规则链也只应各发 1 条 SELECT，而不是 5 条逐项查询。
func TestReconcileEdgeNodeBatchesResourceLookupsAcrossPlanItems(t *testing.T) {
	db, plugin := setupEdgeReconcileBatchTestDB(t)
	const tenantID = "tenant-reconcile"

	seedDevice := func(id string) {
		require.NoError(t, db.Create(&model.Device{ID: id, DeviceNumber: "num-" + id, TenantID: tenantID, Voucher: "{}"}).Error)
	}
	seedDevice("gw-1")

	now := time.Now()
	require.NoError(t, db.Create(&model.EdgeNode{
		ID: "node-1", TenantID: tenantID, Version: "9.9.9", Status: model.EdgeNodeStatusActive, LastSeenAt: &now,
	}).Error)

	boardCfg := "{}"
	for _, id := range []string{"board-1", "board-2", "board-3"} {
		require.NoError(t, db.Create(&model.Board{ID: id, TenantID: tenantID, Name: "b-" + id, Config: &boardCfg}).Error)
	}
	for _, id := range []string{"chain-1", "chain-2"} {
		require.NoError(t, db.Create(&model.RuleChain{ID: id, TenantID: tenantID, Name: "c-" + id, Graph: []byte("{}")}).Error)
	}

	claims := &utils.UserClaims{TenantID: tenantID}
	req := model.EdgeNodeReconcileReq{
		GatewayDeviceID: "gw-1",
		Resources: []model.EdgeNodeReconcileResource{
			{ResourceType: model.EdgeResourceDashboard, ResourceID: "board-1"},
			{ResourceType: model.EdgeResourceDashboard, ResourceID: "board-2"},
			{ResourceType: model.EdgeResourceDashboard, ResourceID: "board-3"},
			{ResourceType: model.EdgeResourceRuleChain, ResourceID: "chain-1"},
			{ResourceType: model.EdgeResourceRuleChain, ResourceID: "chain-2"},
		},
	}

	seedEdgeSyncHistory(t, db, tenantID, "gw-1", req.Resources)
	rsp, err := (&EdgeNodeService{}).ReconcileEdgeNode("node-1", req, claims)
	require.NoError(t, err)
	require.False(t, rsp.Blocked, "all resources are nil-revision (never reported); must sync, not block")
	require.Equal(t, 5, rsp.Synced)
	require.Len(t, rsp.Dispatch, 5)

	for _, d := range rsp.Dispatch {
		require.Empty(t, d.Error, "resource %s/%s should not fail: %s", d.ResourceType, d.ResourceID, d.Error)
		require.NotEmpty(t, d.TaskID)
	}

	require.Equal(t, int64(1), atomic.LoadInt64(plugin.counts["boards"]),
		"3 dashboard plan items must cost exactly 1 batched boards query, not 3")
	require.Equal(t, int64(1), atomic.LoadInt64(plugin.counts["rule_chains"]),
		"2 rule_chain plan items must cost exactly 1 batched rule_chains query, not 2")

	var persisted int64
	require.NoError(t, db.Model(&model.EdgeSyncTask{}).Where("id NOT LIKE ?", "seed-%").Count(&persisted).Error)
	require.Equal(t, int64(5), persisted)
}

// TestReconcileEdgeNodeBatchContinuesPastPerItemErrors 核心不变式：批量预取不改变现有契约——
// 某个资源已被删除（预取批次里缺席，退回单项查询仍查不到）必须如实记到该项的 Error 里，
// 不中断其余资源的下发，且不包进同一个事务（已下发的任务已经落库，不随后续失败回滚）。
func TestReconcileEdgeNodeBatchContinuesPastPerItemErrors(t *testing.T) {
	db, _ := setupEdgeReconcileBatchTestDB(t)
	const tenantID = "tenant-reconcile-2"

	require.NoError(t, db.Create(&model.Device{ID: "gw-1", DeviceNumber: "num-gw-1", TenantID: tenantID, Voucher: "{}"}).Error)
	now := time.Now()
	require.NoError(t, db.Create(&model.EdgeNode{
		ID: "node-1", TenantID: tenantID, Version: "9.9.9", Status: model.EdgeNodeStatusActive, LastSeenAt: &now,
	}).Error)

	boardCfg := "{}"
	require.NoError(t, db.Create(&model.Board{ID: "board-ok", TenantID: tenantID, Name: "ok", Config: &boardCfg}).Error)
	// board-missing 不落库：模拟资源已被删除，预取批次与单项兜底查询都应查不到。

	claims := &utils.UserClaims{TenantID: tenantID}
	req := model.EdgeNodeReconcileReq{
		GatewayDeviceID: "gw-1",
		Resources: []model.EdgeNodeReconcileResource{
			{ResourceType: model.EdgeResourceDashboard, ResourceID: "board-missing"},
			{ResourceType: model.EdgeResourceDashboard, ResourceID: "board-ok"},
		},
	}

	seedEdgeSyncHistory(t, db, tenantID, "gw-1", req.Resources)
	rsp, err := (&EdgeNodeService{}).ReconcileEdgeNode("node-1", req, claims)
	require.NoError(t, err)
	require.False(t, rsp.Blocked)
	require.Len(t, rsp.Dispatch, 2)

	require.Equal(t, "board-missing", rsp.Dispatch[0].ResourceID)
	require.NotEmpty(t, rsp.Dispatch[0].Error, "missing dashboard must surface as a per-item error")
	require.Empty(t, rsp.Dispatch[0].TaskID)

	require.Equal(t, "board-ok", rsp.Dispatch[1].ResourceID)
	require.Empty(t, rsp.Dispatch[1].Error, "sibling item must still succeed despite the earlier item's error")
	require.NotEmpty(t, rsp.Dispatch[1].TaskID)
	require.Equal(t, 1, rsp.Synced)

	var persisted int64
	require.NoError(t, db.Model(&model.EdgeSyncTask{}).Where("id NOT LIKE ?", "seed-%").Count(&persisted).Error)
	require.Equal(t, int64(1), persisted, "the successful item's task must be persisted despite the sibling's failure")
}

// TestReconcileEdgeNodeEdgeSyncTaskScansPerItem 下发循环每条计划项对 edge_sync_tasks 只扫一次
// （history 未触顶时 pending 从 history 内存过滤），外加 reconcile 开头那一次修订号扫描。
func TestReconcileEdgeNodeEdgeSyncTaskScansPerItem(t *testing.T) {
	db, plugin := setupEdgeReconcileBatchTestDB(t)
	const tenantID = "tenant-reconcile-3"
	require.NoError(t, db.Create(&model.Device{ID: "gw-1", DeviceNumber: "num-gw-1", TenantID: tenantID, Voucher: "{}"}).Error)
	now := time.Now()
	require.NoError(t, db.Create(&model.EdgeNode{
		ID: "node-1", TenantID: tenantID, Version: "9.9.9", Status: model.EdgeNodeStatusActive, LastSeenAt: &now,
	}).Error)
	boardCfg := "{}"
	resources := make([]model.EdgeNodeReconcileResource, 0, 3)
	for _, id := range []string{"board-1", "board-2", "board-3"} {
		require.NoError(t, db.Create(&model.Board{ID: id, TenantID: tenantID, Name: id, Config: &boardCfg}).Error)
		resources = append(resources, model.EdgeNodeReconcileResource{ResourceType: model.EdgeResourceDashboard, ResourceID: id})
	}

	seedEdgeSyncHistory(t, db, tenantID, "gw-1", resources)
	rsp, err := (&EdgeNodeService{}).ReconcileEdgeNode("node-1", model.EdgeNodeReconcileReq{
		GatewayDeviceID: "gw-1", Resources: resources,
	}, &utils.UserClaims{TenantID: tenantID})
	require.NoError(t, err)
	require.Equal(t, 3, rsp.Synced)
	require.Equal(t, int64(1+3), atomic.LoadInt64(plugin.counts["edge_sync_tasks"]),
		"1 reconcile-level revision scan + 1 merged history/pending scan per item (was 2 per item)")
}

// TestReconcileEdgeNodeFallsBackWhenPrefetchFails 预取查询失败不应让整个 reconcile 失败：
// 退回逐项查询，查询失败的项如实记 Error，其余项照常下发（与批量化之前的契约一致）。
func TestReconcileEdgeNodeFallsBackWhenPrefetchFails(t *testing.T) {
	db, _ := setupEdgeReconcileBatchTestDB(t)
	const tenantID = "tenant-reconcile-4"
	require.NoError(t, db.Create(&model.Device{ID: "gw-1", DeviceNumber: "num-gw-1", TenantID: tenantID, Voucher: "{}"}).Error)
	now := time.Now()
	require.NoError(t, db.Create(&model.EdgeNode{
		ID: "node-1", TenantID: tenantID, Version: "9.9.9", Status: model.EdgeNodeStatusActive, LastSeenAt: &now,
	}).Error)
	boardCfg := "{}"
	require.NoError(t, db.Create(&model.Board{ID: "board-ok", TenantID: tenantID, Name: "ok", Config: &boardCfg}).Error)
	// 规则链表不存在：批量预取（第二条查询）必然失败。
	require.NoError(t, db.Migrator().DropTable(&model.RuleChain{}))
	seedEdgeSyncHistory(t, db, tenantID, "gw-1", []model.EdgeNodeReconcileResource{
		{ResourceType: model.EdgeResourceRuleChain, ResourceID: "chain-1"},
		{ResourceType: model.EdgeResourceDashboard, ResourceID: "board-ok"},
	})

	rsp, err := (&EdgeNodeService{}).ReconcileEdgeNode("node-1", model.EdgeNodeReconcileReq{
		GatewayDeviceID: "gw-1",
		Resources: []model.EdgeNodeReconcileResource{
			{ResourceType: model.EdgeResourceRuleChain, ResourceID: "chain-1"},
			{ResourceType: model.EdgeResourceDashboard, ResourceID: "board-ok"},
		},
	}, &utils.UserClaims{TenantID: tenantID})
	require.NoError(t, err, "prefetch failure must degrade to per-item lookups, not fail the reconcile")
	require.Len(t, rsp.Dispatch, 2)
	require.NotEmpty(t, rsp.Dispatch[0].Error)
	require.Empty(t, rsp.Dispatch[1].Error)
	require.NotEmpty(t, rsp.Dispatch[1].TaskID)
	require.Equal(t, 1, rsp.Synced)
}

// TestLoadEdgeSyncHistoryAndPendingMatchesSeparateQueries 合并扫描与原先两条独立查询结果一致：
// 未触顶时 pending 由 history 内存过滤；触顶时退回独立 pending 查询，冲突闸门覆盖范围不缩水。
func TestLoadEdgeSyncHistoryAndPendingMatchesSeparateQueries(t *testing.T) {
	for _, total := range []int{5, edgeSyncRevisionScanLimit + 30} {
		t.Run(fmt.Sprintf("total_%d", total), func(t *testing.T) {
			db, _ := setupEdgeReconcileBatchTestDB(t)
			const tenantID = "tenant-scan"
			base := time.Now().Add(-time.Hour)
			for i := 0; i < total; i++ {
				status := "synced"
				if i%3 == 0 || i < 40 { // 老的在途任务足够多，触顶场景下会被 history 截断
					status = edgeSyncStatusPending
				}
				ts := base.Add(time.Duration(i) * time.Second)
				require.NoError(t, db.Create(&model.EdgeSyncTask{
					ID: fmt.Sprintf("task-%04d", i), TenantID: tenantID, GatewayDeviceID: "gw-1", GatewayDeviceNumber: "n",
					ResourceType: model.EdgeResourceDashboard, ResourceID: fmt.Sprintf("r-%d", i%7), Payload: "{}",
					Status: status, CreatedAt: ts, UpdatedAt: ts,
				}).Error)
			}
			history, pending, err := loadEdgeSyncHistoryAndPending(tenantID, model.EdgeResourceDashboard, "gw-1")
			require.NoError(t, err)

			wantHistory, err := dal.ListEdgeSyncTasks(tenantID, model.EdgeResourceDashboard, "gw-1", "", edgeSyncRevisionScanLimit)
			require.NoError(t, err)
			wantPending, err := dal.ListEdgeSyncTasks(tenantID, model.EdgeResourceDashboard, "gw-1", edgeSyncStatusPending, edgeSyncConflictScanLimit)
			require.NoError(t, err)
			require.Equal(t, taskIDs(wantHistory), taskIDs(history))
			require.Equal(t, taskIDs(wantPending), taskIDs(pending))
		})
	}
}

func taskIDs(tasks []*model.EdgeSyncTask) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.ID)
	}
	return out
}

// seedEdgeSyncHistory 为每个资源落一条已同步的历史任务（revision=1），使云端修订号合法；
// 否则云端修订号缺失会被规划为 needs_attention 并阻断下发。
func seedEdgeSyncHistory(t *testing.T, db *gorm.DB, tenantID, gatewayID string, resources []model.EdgeNodeReconcileResource) {
	t.Helper()
	ts := time.Now().Add(-time.Minute)
	for i, res := range resources {
		require.NoError(t, db.Create(&model.EdgeSyncTask{
			ID: fmt.Sprintf("seed-%d-%s", i, res.ResourceID), TenantID: tenantID, GatewayDeviceID: gatewayID,
			GatewayDeviceNumber: "num-" + gatewayID, ResourceType: res.ResourceType, ResourceID: res.ResourceID,
			Payload: `{"revision":1}`, Status: "synced", CreatedAt: ts, UpdatedAt: ts,
		}).Error)
	}
}
