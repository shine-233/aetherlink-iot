// 文件用途：TB-15R（138.sql）作用域删除原语的单测——锁定行级 TTL 的覆盖语义。
// 核心逻辑：内存 sqlite（与 telemetry_retention_test.go 同模式）夹具 data_policy /
// devices / telemetry_datas / telemetry_rollups，验证：
//   - 档案级删除只触及精确 (租户, 档案) 设备；
//   - 租户级删除排除同租户被启用档案级策略覆盖的设备；
//   - 全局回落删除排除被任何启用行级策略覆盖的设备；停用策略不产生覆盖；
//   - 冷层 telemetry_rollups 与热层同口径作用域删除。
//
// 关键注意事项：排除子查询直接引用 data_policy / devices 表，两表必须参与迁移；
// SQL 需保持 PG / sqlite 双兼容。
// 重构建议：作用域粒度扩展（如操作日志行级）时在此补对应用例。
package dal

import (
	"fmt"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRowLevelScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { global.DB = oldDB })
	global.DB = db
	require.NoError(t, db.AutoMigrate(&model.DataPolicy{}, &model.Device{}, &model.TelemetryData{}, &model.TelemetryRollup{}))
	return db
}

func seedRowLevelDevice(t *testing.T, db *gorm.DB, id, tenantID, deviceConfigID string) {
	t.Helper()
	device := &model.Device{
		ID:           id,
		TenantID:     tenantID,
		Voucher:      id,
		DeviceNumber: id,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
	}
	if deviceConfigID != "" {
		device.DeviceConfigID = &deviceConfigID
	}
	require.NoError(t, db.Create(device).Error)
}

func seedRowLevelPolicy(t *testing.T, db *gorm.DB, id, dataType, tenantID, deviceConfigID string, retention int32, enabled string) {
	t.Helper()
	policy := &model.DataPolicy{
		ID:           id,
		DataType:     dataType,
		RetentionDay: retention,
		Enabled:      enabled,
	}
	if tenantID != "" {
		policy.TenantID = &tenantID
	}
	if deviceConfigID != "" {
		policy.DeviceConfigID = &deviceConfigID
	}
	require.NoError(t, db.Create(policy).Error)
}

func seedTelemetryRow(t *testing.T, db *gorm.DB, deviceID, key string, ts int64) {
	t.Helper()
	require.NoError(t, db.Create(&model.TelemetryData{DeviceID: deviceID, Key: key, T: ts}).Error)
}

// remainingTelemetryTs 按 (设备, ts) 收敛剩余热层行，便于精确断言删了哪几行。
func remainingTelemetryTs(t *testing.T, db *gorm.DB) map[string][]int64 {
	t.Helper()
	type row struct {
		DeviceID string `gorm:"column:device_id"`
		T        int64  `gorm:"column:ts"`
	}
	rows := make([]row, 0, 16)
	require.NoError(t, db.Model(&model.TelemetryData{}).
		Select("device_id, ts").Order("device_id, ts").Scan(&rows).Error)
	remaining := make(map[string][]int64, 8)
	for _, r := range rows {
		remaining[r.DeviceID] = append(remaining[r.DeviceID], r.T)
	}
	return remaining
}

func remainingRollupBucketStarts(t *testing.T, db *gorm.DB) map[string][]int64 {
	t.Helper()
	type row struct {
		DeviceID    string `gorm:"column:device_id"`
		BucketStart int64  `gorm:"column:bucket_start"`
	}
	rows := make([]row, 0, 16)
	require.NoError(t, db.Model(&model.TelemetryRollup{}).
		Select("device_id, bucket_start").Order("device_id, bucket_start").Scan(&rows).Error)
	remaining := make(map[string][]int64, 8)
	for _, r := range rows {
		remaining[r.DeviceID] = append(remaining[r.DeviceID], r.BucketStart)
	}
	return remaining
}

func TestTelemetryDeviceScopeFilterBuildsExpectedFragments(t *testing.T) {
	frag, args := telemetryDeviceScopeFilter("td", telemetryCleanupScope{TenantID: "t1", DeviceConfigID: "p1"})
	require.Contains(t, frag, "d.device_config_id = ?")
	require.Equal(t, []interface{}{"t1", "p1"}, args)

	frag, args = telemetryDeviceScopeFilter("td", telemetryCleanupScope{TenantID: "t1"})
	require.Contains(t, frag, "NOT IN")
	require.Contains(t, frag, "data_policy p")
	require.Len(t, args, 3)

	frag, args = telemetryDeviceScopeFilter("tr", telemetryCleanupScope{})
	require.Contains(t, frag, "JOIN data_policy p")
	require.Contains(t, frag, "p.device_config_id IS NULL")
	require.Empty(t, args)
}

func TestDeleteTelemetryDataBatchScopedOverrideSemantics(t *testing.T) {
	db := setupRowLevelScopeTestDB(t)
	const (
		tenantA  = "tenant-a"
		tenantB  = "tenant-b"
		tenantC  = "tenant-c"
		profileA = "profile-a1"
		profileB = "profile-b1"
		oldTS    = int64(50)
		newTS    = int64(500)
	)

	// d1：tenantA+profileA，被档案级行覆盖；d2：tenantA 无档案；
	// d3：tenantB+profileB，被档案级行覆盖；d5：tenantB 无档案；d4：tenantC 无任何行级。
	for _, device := range []struct{ id, tenant, profile string }{
		{"d1", tenantA, profileA},
		{"d2", tenantA, ""},
		{"d3", tenantB, profileB},
		{"d4", tenantC, ""},
		{"d5", tenantB, ""},
	} {
		seedRowLevelDevice(t, db, device.id, device.tenant, device.profile)
		seedTelemetryRow(t, db, device.id, "temperature", oldTS)
		seedTelemetryRow(t, db, device.id, "temperature", newTS)
	}
	seedRowLevelPolicy(t, db, "pol-a-profile", "1", tenantA, profileA, 7, "1")
	seedRowLevelPolicy(t, db, "pol-b-profile", "1", tenantB, profileB, 7, "1")

	// 租户级删除 tenantA：d1 被同租户档案级行覆盖而排除，只删 d2 的过期行。
	deleted, err := deleteTelemetryDataBatchScoped(oldTS, telemetryRetentionDeleteBatchSize, telemetryCleanupScope{TenantID: tenantA})
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	// 档案级删除 (tenantA, profileA)：精确删 d1 的过期行，其余设备不受影响。
	deleted, err = deleteTelemetryDataBatchScoped(oldTS, telemetryRetentionDeleteBatchSize, telemetryCleanupScope{TenantID: tenantA, DeviceConfigID: profileA})
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	// 全局回落删除：d1（tenantA，档案级行覆盖）与 d2（tenantA 无档案，档案级行
	// 不覆盖无档案设备——但 d2 的过期行已被上面租户级删除清掉）均不再出现；
	// d3 被档案级行 (tenantB, profileB) 覆盖而排除；d5 无档案不被该行覆盖 → 全局清；
	// d4 无任何行级 → 全局清。本步共删 d4、d5 两行。
	deleted, err = deleteTelemetryDataBatchScoped(oldTS, telemetryRetentionDeleteBatchSize, telemetryCleanupScope{})
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)

	require.Equal(t, map[string][]int64{
		"d1": {newTS},
		"d2": {newTS},
		// d3 过期行被 (tenantB, profileB) 行级策略覆盖而保留——该行未运行，
		// 全局回落不得越权替删（覆盖语义的核心断言）。
		"d3": {oldTS, newTS},
		"d4": {newTS},
		"d5": {newTS},
	}, remainingTelemetryTs(t, db))
}

func TestDeleteTelemetryDataBatchScopedDisabledPolicyDoesNotExclude(t *testing.T) {
	db := setupRowLevelScopeTestDB(t)
	const (
		tenantB  = "tenant-b"
		profileB = "profile-b1"
		oldTS    = int64(50)
		newTS    = int64(500)
	)
	// 停用的档案级行不产生覆盖：tenantB 设备回落全局清理。
	seedRowLevelDevice(t, db, "d3", tenantB, profileB)
	seedRowLevelDevice(t, db, "d5", tenantB, "")
	seedRowLevelPolicy(t, db, "pol-b-disabled", "1", tenantB, profileB, 7, "2")
	for _, deviceID := range []string{"d3", "d5"} {
		seedTelemetryRow(t, db, deviceID, "temperature", oldTS)
		seedTelemetryRow(t, db, deviceID, "temperature", newTS)
	}

	deleted, err := deleteTelemetryDataBatchScoped(oldTS, telemetryRetentionDeleteBatchSize, telemetryCleanupScope{})
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)

	require.Equal(t, map[string][]int64{
		"d3": {newTS},
		"d5": {newTS},
	}, remainingTelemetryTs(t, db))
}

func TestDeleteTelemetryRollupsBatchScopedFollowsHotLayerScope(t *testing.T) {
	db := setupRowLevelScopeTestDB(t)
	const (
		tenantA  = "tenant-a"
		profileA = "profile-a1"
		tenantC  = "tenant-c"
		oldStart = int64(50)
		newStart = int64(500)
		bucketMs = int64(3600000)
	)
	seedRowLevelDevice(t, db, "d1", tenantA, profileA)
	seedRowLevelDevice(t, db, "d4", tenantC, "")
	seedRowLevelPolicy(t, db, "pol-a-profile", "1", tenantA, profileA, 7, "1")
	for _, deviceID := range []string{"d1", "d4"} {
		require.NoError(t, db.Create(&model.TelemetryRollup{
			DeviceID: deviceID, Key: "temperature", BucketMs: bucketMs,
			BucketStart: oldStart, CountV: 1,
		}).Error)
		require.NoError(t, db.Create(&model.TelemetryRollup{
			DeviceID: deviceID, Key: "temperature", BucketMs: bucketMs,
			BucketStart: newStart, CountV: 1,
		}).Error)
	}

	// 全局回落删除冷层：d1 被 (tenantA, profileA) 行级覆盖而保留过期桶，
	// d4 无行级覆盖 → 过期桶随全局边界删除。
	deleted, err := deleteTelemetryRollupsBatchScoped(oldStart, telemetryRetentionDeleteBatchSize, telemetryCleanupScope{})
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	require.Equal(t, map[string][]int64{
		"d1": {oldStart, newStart},
		"d4": {newStart},
	}, remainingRollupBucketStarts(t, db))
}
