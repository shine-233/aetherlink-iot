// 文件用途：真实 PostgreSQL 集群上的迁移链实测（deep-refactor-backlog-2026-09-28 轨道1 第4项）。
//
// 核心逻辑：环境变量 AETHERLINK_MIG_TEST_DSN 指向一个可建库的 PG 实例（key=value 形式，
// 例如 host=127.0.0.1 port=55441 user=postgres dbname=postgres sslmode=disable）。
// 测试在其实例上 DROP/CREATE 一个全新的 aetherlink_mig_live 库，用生产入口
// initialize.CheckVersion 从版本 0 把 1.sql..142.sql 全链跑通，然后逐项断言本批迁移
// 的落库结果（141 保留期注册表、142 alarm_history_devices 拆表/回填/触发器/索引卫生），
// 并对 141/142.sql 各自重放一次验证幂等守卫，再用 dal 原语实测保留期批次删除。
//
// 关键注意事项：
//   - 未设置 AETHERLINK_MIG_TEST_DSN 时整测 Skip：常规单测与 CI 不需要真实 PG，
//     dal/service 的单测夹具继续用 sqlite。
//   - CheckVersion 以相对路径读取 sql/<n>.sql，故测试把进程 CWD 切到 backend/ 根
//     （按 runtime.Caller 定位，测试结束还原）。
//   - 本测试只连接调用方显式提供的实例，不探测、不修改任何环境中的既有数据库
//     （除自建的 aetherlink_mig_live 一次性库）。
package retention

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	initialize "aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const liveTestDBName = "aetherlink_mig_live"

func openMaintenanceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("AETHERLINK_MIG_TEST_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("AETHERLINK_MIG_TEST_DSN not set; live PostgreSQL migration chain test skipped")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err, "连接 AETHERLINK_MIG_TEST_DSN 实例失败")
	return db
}

func chdirBackendRoot(t *testing.T) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "无法定位测试文件路径")
	// 本文件位于 backend/internal/retention/，上溯三级即 backend/。
	backendRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(backendRoot), "切换到 backend 根目录失败")
	t.Cleanup(func() { _ = os.Chdir(origWD) })
}

func recreateLiveDB(t *testing.T, maintenance *gorm.DB) *gorm.DB {
	t.Helper()
	// PG13+ 支持 DROP DATABASE ... WITH (FORCE)：强断残留连接，保证可重跑。
	require.NoError(t, maintenance.Exec(
		fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, liveTestDBName)).Error)
	require.NoError(t, maintenance.Exec(
		fmt.Sprintf(`CREATE DATABASE %s`, liveTestDBName)).Error)

	dsn := os.Getenv("AETHERLINK_MIG_TEST_DSN")
	liveDSN := appendDSNKey(dsn, "dbname", liveTestDBName)
	db, err := gorm.Open(postgres.Open(liveDSN), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err, "连接新建的 %s 失败", liveTestDBName)
	return db
}

// appendDSNKey 以 key=value 覆写语义替换 DSN 里的 dbname（没有则追加）。
// 值为纯字母数字下划线时无需引用，直接拼装。
func appendDSNKey(dsn, key, value string) string {
	parts := strings.Fields(dsn)
	replaced := false
	for i, p := range parts {
		if strings.HasPrefix(p, key+"=") {
			parts[i] = key + "=" + value
			replaced = true
		}
	}
	if !replaced {
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, " ")
}

func scalarString(t *testing.T, db *gorm.DB, sql string, args ...interface{}) string {
	t.Helper()
	var out string
	require.NoError(t, db.Raw(sql, args...).Scan(&out).Error)
	return out
}

func scalarInt64(t *testing.T, db *gorm.DB, sql string, args ...interface{}) int64 {
	t.Helper()
	var out int64
	require.NoError(t, db.Raw(sql, args...).Scan(&out).Error)
	return out
}

func readFileAtBackendRoot(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", rel))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// TestMigrationChainLiveOnPostgreSQL17 是本文件的主测试：全链迁移 + 结构断言 + 幂等重放。
func TestMigrationChainLiveOnPostgreSQL17(t *testing.T) {
	maintenance := openMaintenanceDB(t)
	chdirBackendRoot(t)
	live := recreateLiveDB(t, maintenance)

	// ---- 1. 迁移链从头到尾跑通（生产入口 CheckVersion，版本 0 → 142）----
	require.NoError(t, initialize.CheckVersion(live), "CheckVersion 全链执行失败")

	// sys_version 收敛到程序版本。
	require.Equal(t, fmt.Sprint(global.VERSION_NUMBER),
		scalarString(t, live, `SELECT version_number::text FROM sys_version`),
		"sys_version.version_number 未收敛到 VERSION_NUMBER")
	require.Equal(t, global.VERSION,
		scalarString(t, live, `SELECT version FROM sys_version`),
		"sys_version.version 未收敛到程序版本号")

	// ---- 2. 141.sql 保留期注册表 ----
	require.Equal(t, int64(1), scalarInt64(t, live,
		`SELECT (to_regclass('public.data_retention_registry') IS NOT NULL)::int`),
		"data_retention_registry 表不存在")
	seedRows := scalarInt64(t, live, `SELECT count(*) FROM public.data_retention_registry`)
	require.Equal(t, int64(19), seedRows, "141.sql 应登记 19 张表（18 张客户/审计/死信 + 1 张幂等回执）")
	enabledRows := scalarInt64(t, live,
		`SELECT count(*) FROM public.data_retention_registry WHERE enabled = '1'`)
	require.Equal(t, int64(1), enabledRows, "默认开启的只能有幂等回执一行")
	require.Equal(t, "uplink_storage_receipts",
		scalarString(t, live, `SELECT table_name FROM public.data_retention_registry WHERE enabled = '1'`),
		"默认开启行应为幂等回执表")
	customerDisabled := scalarInt64(t, live,
		`SELECT count(*) FROM public.data_retention_registry WHERE category = 'customer_data' AND enabled = '2'`)
	require.Equal(t, int64(14), customerDisabled, "customer_data 行必须全部默认关闭")

	// ---- 3. 142.sql alarm_history_devices 拆表：结构 + 触发器 + 回填 + 双写投影 ----
	require.Equal(t, int64(1), scalarInt64(t, live,
		`SELECT (to_regclass('public.alarm_history_devices') IS NOT NULL)::int`),
		"alarm_history_devices 表不存在")
	require.Equal(t, int64(1), scalarInt64(t, live,
		`SELECT count(*) FROM pg_trigger WHERE tgrelid = 'public.alarm_history'::regclass AND tgname = 'trg_alarm_history_devices_sync' AND NOT tgisinternal`),
		"同步触发器 trg_alarm_history_devices_sync 不存在")

	// jsonb 列仍是写入事实源：INSERT/UPDATE 后关联表自动投影（触发器即双写过渡的落点）。
	// 列清单对齐 1.sql 的 NOT NULL 约束（alarm_config_id/group_id/scene_automation_id 必填）。
	require.NoError(t, live.Exec(`INSERT INTO public.alarm_history
		(id, alarm_config_id, group_id, scene_automation_id, name, content, alarm_status, tenant_id, create_at, alarm_device_list)
		VALUES ('ah-live-1', 'ac-live-1', '', '', 'n', 'c', 'H', 't-live', now(), '["dev-a","dev-b"]'::jsonb)`).Error)
	require.Equal(t, int64(2), scalarInt64(t, live,
		`SELECT count(*) FROM public.alarm_history_devices WHERE alarm_history_id = 'ah-live-1'`),
		"INSERT 后关联表应投影 2 行")
	require.NoError(t, live.Exec(`UPDATE public.alarm_history
		SET alarm_device_list = '["dev-c"]'::jsonb WHERE id = 'ah-live-1'`).Error)
	require.Equal(t, int64(1), scalarInt64(t, live,
		`SELECT count(*) FROM public.alarm_history_devices WHERE alarm_history_id = 'ah-live-1' AND device_id = 'dev-c'`),
		"UPDATE 后关联表应跟随 jsonb 列同步")

	// 回填语句对存量行生效：清掉投影后重放 142.sql 的回填段应把行补回来。
	require.NoError(t, live.Exec(`DELETE FROM public.alarm_history_devices WHERE alarm_history_id = 'ah-live-1'`).Error)
	require.Equal(t, int64(0), scalarInt64(t, live,
		`SELECT count(*) FROM public.alarm_history_devices WHERE alarm_history_id = 'ah-live-1'`))
	require.NoError(t, live.Exec(`INSERT INTO public.alarm_history_devices (alarm_history_id, device_id, tenant_id)
		SELECT ah.id, btrim(elem.value), ah.tenant_id
		FROM public.alarm_history ah
		CROSS JOIN LATERAL jsonb_array_elements_text(
			COALESCE(CASE WHEN jsonb_typeof(ah.alarm_device_list) = 'array'
				THEN (SELECT jsonb_agg(e) FROM jsonb_array_elements(ah.alarm_device_list) AS e WHERE jsonb_typeof(e) = 'string') END,
				'[]'::jsonb)) AS elem(value)
		WHERE btrim(elem.value) <> ''
		ON CONFLICT (alarm_history_id, device_id) DO NOTHING`).Error)
	require.Equal(t, int64(1), scalarInt64(t, live,
		`SELECT count(*) FROM public.alarm_history_devices WHERE alarm_history_id = 'ah-live-1' AND device_id = 'dev-c'`),
		"回填语句未把存量 jsonb 行投影到关联表")

	// ---- 4. 遥测索引卫生 ----
	require.Equal(t, int64(0), scalarInt64(t, live,
		`SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'telemetry_datas_ts_idx_copy1'`),
		"Navicat 残留索引 telemetry_datas_ts_idx_copy1 应被 142.sql 删除")
	require.Equal(t, int64(1), scalarInt64(t, live,
		`SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'idx_devices_parent_sub_addr'`),
		"142.sql 应补 (parent_id, sub_device_addr) 索引")

	// ---- 5. 幂等重放：141/142.sql 各再执行一次必须无错 ----
	require.NoError(t, live.Exec(readFileAtBackendRoot(t, filepath.Join("sql", "141.sql"))).Error,
		"141.sql 重放失败（幂等守卫不完整）")
	require.NoError(t, live.Exec(readFileAtBackendRoot(t, filepath.Join("sql", "142.sql"))).Error,
		"142.sql 重放失败（幂等守卫不完整）")
	require.Equal(t, seedRows, scalarInt64(t, live, `SELECT count(*) FROM public.data_retention_registry`),
		"重放 141.sql 后注册表行数变化（ON CONFLICT 守卫失效）")
}

// TestRetentionBatchDeleteLiveOnPostgreSQL17 用一次性探针表实测保留期注册表的
// 批次删除原语（dal.GetRetentionRegistryRows / DeleteExpiredRowsByRegistry /
// UpdateRetentionRegistryCleanupTime）：timestamptz 与 unix_ms 两类时间列、
// ctid 分批上界、resolved_only 死信口径、水位回写。
func TestRetentionBatchDeleteLiveOnPostgreSQL17(t *testing.T) {
	maintenance := openMaintenanceDB(t)
	chdirBackendRoot(t)
	live := recreateLiveDB(t, maintenance)
	require.NoError(t, initialize.CheckVersion(live), "CheckVersion 全链执行失败")

	// dal 的保留期原语读 global.DB（生产由 pg_init 装配，pg_init.go: global.DB = db）；
	// 测试进程里由本测试注入并还原。
	origDB := global.DB
	global.DB = live
	t.Cleanup(func() { global.DB = origDB })

	// 注册表原语在真实链上可读。
	rows, err := dal.GetRetentionRegistryRows()
	require.NoError(t, err)
	require.NotEmpty(t, rows, "注册表行为空")

	// ---- 探针表：timestamptz 列，15 行过期 + 5 行新鲜，batch_size=10 ----
	require.NoError(t, live.Exec(`CREATE TABLE public.retention_probe_tz (id serial PRIMARY KEY, created_at timestamptz NOT NULL)`).Error)
	require.NoError(t, live.Exec(`INSERT INTO public.retention_probe_tz (created_at)
		SELECT now() - interval '400 days' FROM generate_series(1, 15)`).Error)
	require.NoError(t, live.Exec(`INSERT INTO public.retention_probe_tz (created_at)
		SELECT now() FROM generate_series(1, 5)`).Error)
	tzRow := &model.DataRetentionRegistry{
		ID: "probe-tz", TableName: "retention_probe_tz", TimeColumn: "created_at",
		TimeKind: model.RetentionTimeKindTimestampTZ, RetentionDays: 365,
		Enabled: model.RetentionEnabled, BatchSize: 10,
	}
	deleted := runRetentionToCompletion(t, live, tzRow)
	require.Equal(t, int64(15), deleted, "timestamptz 口径应恰好删掉 15 行过期数据（分批累计）")
	require.Equal(t, int64(5), scalarInt64(t, live, `SELECT count(*) FROM public.retention_probe_tz`),
		"新鲜行不应被删除")

	// ---- 探针表：unix_ms（bigint）列，覆盖 event_datas.ts 的同构形态 ----
	require.NoError(t, live.Exec(`CREATE TABLE public.retention_probe_ms (id serial PRIMARY KEY, ts int8 NOT NULL)`).Error)
	require.NoError(t, live.Exec(`INSERT INTO public.retention_probe_ms (ts)
		SELECT (extract(epoch from now() - interval '400 days') * 1000)::int8 FROM generate_series(1, 7)`).Error)
	require.NoError(t, live.Exec(`INSERT INTO public.retention_probe_ms (ts)
		SELECT (extract(epoch from now()) * 1000)::int8 FROM generate_series(1, 3)`).Error)
	msRow := &model.DataRetentionRegistry{
		ID: "probe-ms", TableName: "retention_probe_ms", TimeColumn: "ts",
		TimeKind: model.RetentionTimeKindUnixMs, RetentionDays: 365,
		Enabled: model.RetentionEnabled, BatchSize: 10000,
	}
	deleted = runRetentionToCompletion(t, live, msRow)
	require.Equal(t, int64(7), deleted, "unix_ms 口径应恰好删掉 7 行过期数据")

	// ---- resolved_only：未解决行绝不按时间删 ----
	require.NoError(t, live.Exec(`CREATE TABLE public.retention_probe_dl (id serial PRIMARY KEY, created_at timestamptz NOT NULL, status varchar(16) NOT NULL)`).Error)
	require.NoError(t, live.Exec(`INSERT INTO public.retention_probe_dl (created_at, status)
		SELECT now() - interval '400 days', s FROM generate_series(1, 4), (VALUES ('resolved'), ('dead')) AS v(s)`).Error)
	dlRow := &model.DataRetentionRegistry{
		ID: "probe-dl", TableName: "retention_probe_dl", TimeColumn: "created_at",
		TimeKind: model.RetentionTimeKindTimestampTZ, RetentionDays: 365,
		Enabled: model.RetentionEnabled, BatchSize: 10000, ResolvedOnly: true,
	}
	deleted = runRetentionToCompletion(t, live, dlRow)
	require.Equal(t, int64(4), deleted, "resolved_only 口径只应删掉 4 行 resolved")
	require.Equal(t, int64(4), scalarInt64(t, live,
		`SELECT count(*) FROM public.retention_probe_dl WHERE status = 'dead'`),
		"未解决行（可重放资产）不应被删除")

	// ---- 注入面：非法标识符必须被白名单拒绝，绝不拼进 SQL ----
	badRow := &model.DataRetentionRegistry{
		ID: "probe-bad", TableName: `retention_probe_tz; DROP TABLE public.retention_probe_tz`,
		TimeColumn: "created_at", TimeKind: model.RetentionTimeKindTimestampTZ,
		RetentionDays: 365, Enabled: model.RetentionEnabled, BatchSize: 10,
	}
	_, err = dal.DeleteExpiredRowsByRegistry(badRow, dal.RetentionCutoff{TimestampTZ: time.Now()})
	require.Error(t, err, "非法表名必须被标识符白名单拒绝")
	require.Equal(t, int64(5), scalarInt64(t, live, `SELECT count(*) FROM public.retention_probe_tz`),
		"被拒绝的行不应产生任何删除")
}

// runRetentionToCompletion 以 service/datapolicy_retention.go 相同的循环推进单行注册表：
// 反复调用 dal.DeleteExpiredRowsByRegistry 直到不再删满一批，返回累计删除行数并回写水位。
func runRetentionToCompletion(t *testing.T, live *gorm.DB, row *model.DataRetentionRegistry) int64 {
	t.Helper()
	var total int64
	now := time.Now()
	boundary := now.AddDate(0, 0, -int(row.RetentionDays))
	cutoff := dal.RetentionCutoff{TimestampTZ: boundary}
	if row.TimeKind == model.RetentionTimeKindUnixMs {
		cutoff = dal.RetentionCutoff{UnixMs: boundary.UnixMilli()}
	}
	for i := 0; i < 1000; i++ {
		affected, err := dal.DeleteExpiredRowsByRegistry(row, cutoff)
		require.NoError(t, err)
		total += affected
		if affected < int64(row.BatchSize) {
			break
		}
	}
	require.NoError(t, dal.UpdateRetentionRegistryCleanupTime(row.ID, now, boundary))
	require.NoError(t, live.Exec(`DELETE FROM public.data_retention_registry WHERE id = ?`, row.ID).Error,
		"清理探针注册行失败")
	return total
}
