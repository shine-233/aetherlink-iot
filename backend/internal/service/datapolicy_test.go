// 文件用途：TB-15R（138.sql）数据策略行级 TTL 的单测——策略优先级解析与清理覆盖语义。
// 核心逻辑：
//   - 纯函数部分锁定策略行作用域判定（档案级/租户级/全局）；
//   - 端到端部分在内存 sqlite 夹具上跑 CleanSystemDataByCron，验证全局清理不越权
//     删除被行级策略覆盖设备的数据、冷层 rollups 同口径、行级操作日志行被跳过。
//
// 关键注意事项：夹具需接管 global.DB（作用域删除走原生 SQL）与 query 默认库
// （策略读写走 gorm gen），参照 board_carousel_test.go 的接管与恢复模式。
// 重构建议：若解析口径变化（如新增档案组粒度），先改本文件再改实现。
package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// policyFixtureRow 构造一条策略行；tenantID/deviceConfigID 传空串表示 NULL（全局/租户级）。
func policyFixtureRow(id, dataType, tenantID, deviceConfigID string, retention int32, enabled string) *model.DataPolicy {
	row := &model.DataPolicy{
		ID:           id,
		DataType:     dataType,
		RetentionDay: retention,
		Enabled:      enabled,
	}
	if tenantID != "" {
		row.TenantID = &tenantID
	}
	if deviceConfigID != "" {
		row.DeviceConfigID = &deviceConfigID
	}
	return row
}

func TestScopeOfDataPolicy(t *testing.T) {
	require.Equal(t, dataPolicyScopeGlobal, scopeOfDataPolicy(nil))
	require.Equal(t, dataPolicyScopeGlobal, scopeOfDataPolicy(policyFixtureRow("g", "1", "", "", 30, "1")))
	require.Equal(t, dataPolicyScopeTenant, scopeOfDataPolicy(policyFixtureRow("t", "1", "tenant-1", "", 30, "1")))
	require.Equal(t, dataPolicyScopeProfile, scopeOfDataPolicy(policyFixtureRow("p", "1", "tenant-1", "profile-1", 30, "1")))
	// 空串租户等价 NULL（创建入口已归一，防御性确认解析口径）。
	require.Equal(t, dataPolicyScopeGlobal, scopeOfDataPolicy(policyFixtureRow("e", "1", "", "profile-x", 30, "1")))
}

func TestIsDuplicateDataPolicyErr(t *testing.T) {
	require.False(t, isDuplicateDataPolicyErr(nil))
	require.True(t, isDuplicateDataPolicyErr(fmt.Errorf("pq: duplicate key value violates unique constraint \"uq_data_policy_row_level\"")))
	require.True(t, isDuplicateDataPolicyErr(fmt.Errorf("UNIQUE constraint failed: data_policy.tenant_id")))
	require.False(t, isDuplicateDataPolicyErr(fmt.Errorf("connection refused")))
}

// setupDataPolicyCronTestDB 建隔离内存库并接管 global.DB / query 默认库。
func setupDataPolicyCronTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := "data_policy_cron_" + strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.DataPolicy{}, &model.Device{}, &model.TelemetryData{},
		&model.TelemetryRollup{}, &model.OperationLog{},
	))
	global.DB = db
	query.SetDefault(db)
	t.Cleanup(func() {
		global.DB = oldDB
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
	return db
}

func TestCleanSystemDataByCronRowLevelOverride(t *testing.T) {
	db := setupDataPolicyCronTestDB(t)

	const (
		tenantA  = "tenant-a"
		tenantB  = "tenant-b"
		tenantC  = "tenant-c"
		profileB = "profile-b1"
		keptTS   = int64(9999999999999) // 远未来哨兵值，任何保留天数都删不掉
		bucketMs = int64(3600000)
	)

	// 策略：全局 30 天；tenantA 租户级 90 天（比全局长 → 覆盖保留）；
	// (tenantB, profileB) 档案级 1 天；行级操作日志行（应被跳过）；停用的全局操作日志行。
	policies := []*model.DataPolicy{
		policyFixtureRow("pol-global-1", "1", "", "", 30, "1"),
		policyFixtureRow("pol-tenant-a", "1", tenantA, "", 90, "1"),
		policyFixtureRow("pol-profile-b", "1", tenantB, profileB, 1, "1"),
		policyFixtureRow("pol-oplog-row", "2", tenantA, "", 15, "1"),
		policyFixtureRow("pol-oplog-global-off", "2", "", "", 15, "2"),
	}
	for _, p := range policies {
		require.NoError(t, db.Create(p).Error)
	}

	// 设备与数据：
	//   d-a：tenantA（租户级 90 天覆盖）→ 60 天前的行保留（比 90 天新），冷层桶保留；
	//   d-b：tenantB+profileB（档案级 1 天覆盖）→ 60 天前的行删除；
	//   d-c：tenantB 无档案（无租户级行 → 全局 30 天）→ 删除；
	//   d-d：tenantC（无任何行级 → 全局 30 天）→ 删除。
	//   60 天前的数据比全局 30 天旧——若全局回落错误地波及被行级覆盖的设备，断言即失败。
	sixtyDaysAgo := time.Now().AddDate(0, 0, -60).UnixMilli()
	for _, device := range []struct{ id, tenant, profile string }{
		{"d-a", tenantA, ""},
		{"d-b", tenantB, profileB},
		{"d-c", tenantB, ""},
		{"d-d", tenantC, ""},
	} {
		require.NoError(t, db.Create(&model.Device{
			ID: device.id, TenantID: device.tenant, Voucher: device.id,
			DeviceNumber: device.id, IsEnabled: "enabled", ActivateFlag: "active",
		}).Error)
		require.NoError(t, db.Create(&model.TelemetryData{DeviceID: device.id, Key: "temperature", T: sixtyDaysAgo}).Error)
		require.NoError(t, db.Create(&model.TelemetryData{DeviceID: device.id, Key: "temperature", T: keptTS}).Error)
		require.NoError(t, db.Create(&model.TelemetryRollup{
			DeviceID: device.id, Key: "temperature", BucketMs: bucketMs,
			BucketStart: sixtyDaysAgo, CountV: 1,
		}).Error)
	}
	require.NoError(t, db.Create(&model.OperationLog{
		ID: "op-1", IP: "127.0.0.1", UserID: "u1", CreatedAt: time.Now().AddDate(0, 0, -60), TenantID: tenantA,
	}).Error)

	require.NoError(t, GroupApp.DataPolicy.CleanSystemDataByCron())

	// 热层：只有 d-a 的过期行因租户级 90 天覆盖而保留，其余设备的过期行全删。
	for _, deviceID := range []string{"d-b", "d-c", "d-d"} {
		var oldCount int64
		require.NoError(t, db.Model(&model.TelemetryData{}).
			Where("device_id = ? AND ts <= ?", deviceID, sixtyDaysAgo).Count(&oldCount).Error)
		require.EqualValues(t, 0, oldCount, "device %s expired rows should be cleaned", deviceID)
	}
	var keptOld int64
	require.NoError(t, db.Model(&model.TelemetryData{}).
		Where("device_id = ? AND ts <= ?", "d-a", sixtyDaysAgo).Count(&keptOld).Error)
	require.EqualValues(t, 1, keptOld, "tenant-level policy (90d) should override global (30d) for tenant-a device")

	// 哨兵行全部保留。
	var sentinel int64
	require.NoError(t, db.Model(&model.TelemetryData{}).Where("ts = ?", keptTS).Count(&sentinel).Error)
	require.EqualValues(t, 4, sentinel)

	// 冷层：d-a 过期桶保留，其余设备过期桶随各自生效策略删除。
	var dARollup int64
	require.NoError(t, db.Model(&model.TelemetryRollup{}).
		Where("device_id = ? AND bucket_start <= ?", "d-a", sixtyDaysAgo).Count(&dARollup).Error)
	require.EqualValues(t, 1, dARollup)
	var otherRollups int64
	require.NoError(t, db.Model(&model.TelemetryRollup{}).
		Where("device_id IN ?", []string{"d-b", "d-c", "d-d"}).Count(&otherRollups).Error)
	require.EqualValues(t, 0, otherRollups)

	// 行级操作日志行被跳过：操作日志原样保留，且该策略行的清理时间未被推进。
	var opLogs int64
	require.NoError(t, db.Model(&model.OperationLog{}).Count(&opLogs).Error)
	require.EqualValues(t, 1, opLogs)
	var skippedPolicy model.DataPolicy
	require.NoError(t, db.Where("id = ?", "pol-oplog-row").First(&skippedPolicy).Error)
	require.Nil(t, skippedPolicy.LastCleanupTime)

	// 执行过的策略行都推进了清理时间。
	var executedCount int64
	require.NoError(t, db.Model(&model.DataPolicy{}).
		Where("id IN ? AND last_cleanup_time IS NOT NULL", []string{"pol-global-1", "pol-tenant-a", "pol-profile-b"}).
		Count(&executedCount).Error)
	require.EqualValues(t, 3, executedCount)
}
