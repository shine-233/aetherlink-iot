// 文件用途：TB-15 时序数据保留策略——TimescaleDB 活库时把 data_policy 的设备数据
//
//	retention_days 落成 telemetry_datas 的原生 retention policy（add_retention_policy）。
//
// 核心逻辑：口径修正（压缩≠保留）：57.sql 的压缩策略只省存储、不删数据；过期删除由本文件
//
//	在启动时装配的 retention policy 执行。装配幂等：先查 timescaledb_information.jobs
//	的 policy_retention 守卫，目标值一致则跳过；漂移则 remove+re-add 收敛到当前配置。
//
// 关键注意事项：telemetry_datas.ts 是 UnixMilli bigint（整数时间列）——drop_after 必须是整数
//
//	且按时间列单位（毫秒）表达，并要求先 set_integer_now_func（官方文档口径，2026-09-25
//	核对 TimescaleDB api.md）。失败不阻断启动：普通 PG 部署本就无此策略，warn 后下次
//	启动自动重试；多副本同时启动因守卫查询收敛到同一目标值而幂等。
//
// 重构建议：alarm_info hypertable 的保留策略待 data_policy 增加告警数据类型后同法补挂；
//
//	档案/租户粒度 TTL 属 data_policy 行级扩展，另立批次（ROADMAP TB-15 residual）。
package initialize

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// retentionPolicy 保留策略装配的目标 hypertable 与配套对象。
const (
	// retentionHypertable 设备数据 hypertable（57.sql 已转换，ts 为 UnixMilli bigint）。
	retentionHypertable = "telemetry_datas"
	// integerNowFuncName 整数时间列的 now() 替身：retention job 判定 chunk 年龄必需。
	integerNowFuncName = "aetherlink_ts_now_ms"
	// msPerDay 一天的毫秒数（UnixMilli 时间列的单位换算）。
	msPerDay int64 = 24 * 3600 * 1000
	// maxRetentionDays 与 UpdateDataPolicyReq 的 validate lte=3650 对齐的保留天数上界。
	maxRetentionDays int32 = 3650
)

// dataPolicyRow initialize 启动期对 data_policy 的最小读取投影。
// tenant-scope: system-table —— data_policy 为全局系统表（无租户列），仅 SYS_ADMIN 可写。
type dataPolicyRow struct {
	DataType     string `gorm:"column:data_type"`
	RetentionDay int32  `gorm:"column:retention_days"`
	Enabled      string `gorm:"column:enabled"`
}

// retentionDropAfterMs 纯函数：把"设备数据保留天数"换算成整数时间列上的 drop_after 毫秒值。
// 天数必须落在 [1, maxRetentionDays]——下界防止误配 0 造成全表立即清空，上界对齐
// UpdateDataPolicyReq 的校验口径，避免 API 侧合法、装配侧拒绝的口径分叉。
func retentionDropAfterMs(retentionDays int32) (int64, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("设备数据保留天数必须为正整数，实际为 %d", retentionDays)
	}
	if retentionDays > maxRetentionDays {
		return 0, fmt.Errorf("设备数据保留天数 %d 超过上界 %d（与 data_policy 更新接口口径一致）",
			retentionDays, maxRetentionDays)
	}
	return int64(retentionDays) * msPerDay, nil
}

// selectDeviceDataRetentionDays 纯函数：从 data_policy 行集中选出设备数据（data_type=1）
// 的启用保留天数。多行命中时取首行（种子数据每类型一行；策略行级扩展后此处需重审）。
// 返回 false 表示没有可用的启用策略（未配置/停用/天数非法），装配应跳过而不是猜默认值。
func selectDeviceDataRetentionDays(rows []dataPolicyRow) (int32, bool) {
	for _, row := range rows {
		if row.DataType != "1" || row.Enabled != "1" || row.RetentionDay <= 0 {
			continue
		}
		return row.RetentionDay, true
	}
	return 0, false
}

// integerNowFuncDDL 整数时间列的 now() 替身 DDL。
// STABLE + RETURN bigint 是 set_integer_now_func 的硬性要求；用事务时间 now()
// 换算毫秒，同一事务内取值一致（与官方文档示例同构）。
func integerNowFuncDDL() string {
	return "CREATE OR REPLACE FUNCTION " + integerNowFuncName + "() RETURNS bigint " +
		"LANGUAGE sql STABLE PARALLEL SAFE AS $aetherlink_retention$" +
		" SELECT (extract(epoch FROM now()) * 1000)::bigint $aetherlink_retention$;"
}

// buildSetIntegerNowFuncSQL 生成把 integer_now_func 注册到目标 hypertable 的调用。
// replaceIfExists=true 直接覆盖旧注册；旧版本扩展不认识该参数时报错由调用方按
// "already" 关键字识别"已注册"语义兜底。
func buildSetIntegerNowFuncSQL(replaceIfExists bool) string {
	return fmt.Sprintf("SELECT set_integer_now_func('%s', '%s', %t)",
		retentionHypertable, integerNowFuncName, replaceIfExists)
}

// buildAddRetentionPolicySQL 生成注册 retention policy 的调用：
// drop_after 以占位符传毫秒值，if_not_exists 提供注册级幂等（双保险，主守卫是 jobs 查询）。
func buildAddRetentionPolicySQL() (string, []interface{}) {
	return "SELECT add_retention_policy('" + retentionHypertable +
		"', drop_after => ?, if_not_exists => TRUE)", nil
}

// buildRemoveRetentionPolicySQL 生成注销 retention policy 的调用（值漂移收敛用）。
func buildRemoveRetentionPolicySQL() string {
	return "SELECT remove_retention_policy('" + retentionHypertable + "', if_exists => TRUE)"
}

// buildRetentionJobGuardSQL 生成幂等守卫查询：读现有 policy_retention 作业的 drop_after。
// config->>'drop_after' 对整数时间列存的就是毫秒原文，Go 侧按 int64 比对。
func buildRetentionJobGuardSQL() string {
	return "SELECT config->>'drop_after' AS drop_after FROM timescaledb_information.jobs " +
		"WHERE job_type = 'policy_retention' AND hypertable_name = '" + retentionHypertable + "' LIMIT 1"
}

// currentRetentionDropAfterMs 读当前已注册的 drop_after 毫秒值；nil 表示尚未注册或无法
// 识别（config 缺 drop_after 等版本差异）——后者保守地保持现状，不做 remove+re-add。
func currentRetentionDropAfterMs(db *gorm.DB) (*int64, error) {
	var raw *string
	if err := db.Raw(buildRetentionJobGuardSQL()).Scan(&raw).Error; err != nil {
		return nil, fmt.Errorf("查询 retention policy 守卫失败: %w", err)
	}
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	parsed, parseErr := strconv.ParseInt(strings.TrimSpace(*raw), 10, 64)
	if parseErr != nil {
		logrus.Infof("[TimescaleRetention] 已有 retention 作业但 drop_after=%q 不可解析，保持现状", *raw)
		return nil, nil
	}
	return &parsed, nil
}

// ensureIntegerNowFunc 保证整数时间列的 now() 替身已创建并注册。
// 注册阶段对旧版本扩展的兼容：报错含 "already" 视为已注册（旧版无 replace_if_exists 参数，
// 且重复注册才报 already）；其余错误如实上抛。
func ensureIntegerNowFunc(db *gorm.DB) error {
	if err := db.Exec(integerNowFuncDDL()).Error; err != nil {
		return fmt.Errorf("创建 integer_now_func %s 失败: %w", integerNowFuncName, err)
	}
	if err := db.Exec(buildSetIntegerNowFuncSQL(true)).Error; err != nil {
		lowered := strings.ToLower(err.Error())
		if strings.Contains(lowered, "already") {
			return nil
		}
		return fmt.Errorf("注册 integer_now_func 失败: %w", err)
	}
	return nil
}

// ApplyTimescaleRetentionPolicy 把 data_policy 的设备数据保留天数装配成
// telemetry_datas 的原生 retention policy。失败不阻断启动（调用方 warn 即可）。
func ApplyTimescaleRetentionPolicy(db *gorm.DB) error {
	// 与 57.sql 的执行开关同源：显式 off 表示本部署明确不使用 TimescaleDB，
	// 即使扩展存在也保持普通 PG，装配必须同样跳过。
	mode, err := normalizeTimescaleMode(readTimescaleMode())
	if err != nil {
		return err
	}
	if mode == timescaleModeOff {
		logrus.Info("[TimescaleRetention] TimescaleDB 显式关闭，跳过 retention policy 装配")
		return nil
	}
	installed, err := timescaleExtensionInstalled(db)
	if err != nil {
		return err
	}
	if !installed {
		logrus.Info("[TimescaleRetention] 未安装 timescaledb 扩展，跳过 retention policy 装配")
		return nil
	}

	// PgInit 阶段 gen 单例尚未绑定（query.SetDefault 在 NewApplication 里），
	// 这里用原生 SQL 读取策略行，不触碰 gen query。
	var rows []dataPolicyRow
	if err := db.Raw("SELECT data_type, retention_days, enabled FROM data_policy").Scan(&rows).Error; err != nil {
		return fmt.Errorf("读取 data_policy 失败: %w", err)
	}
	days, ok := selectDeviceDataRetentionDays(rows)
	if !ok {
		logrus.Info("[TimescaleRetention] 无启用的设备数据保留策略（data_type=1），跳过 retention policy 装配")
		return nil
	}
	dropAfter, err := retentionDropAfterMs(days)
	if err != nil {
		return err
	}

	if err := ensureIntegerNowFunc(db); err != nil {
		return err
	}

	current, err := currentRetentionDropAfterMs(db)
	if err != nil {
		return err
	}
	switch {
	case current != nil && *current == dropAfter:
		logrus.Infof("[TimescaleRetention] telemetry_datas retention policy 已是目标值 drop_after=%dms（%d 天），跳过", dropAfter, days)
		return nil
	case current != nil:
		// 值漂移：data_policy 的保留天数变过，remove+re-add 收敛到新值。
		if err := db.Exec(buildRemoveRetentionPolicySQL()).Error; err != nil {
			return fmt.Errorf("注销旧 retention policy 失败: %w", err)
		}
		logrus.Infof("[TimescaleRetention] telemetry_datas retention policy 值漂移 %dms -> %dms，已重注册", *current, dropAfter)
	}

	addSQL, _ := buildAddRetentionPolicySQL()
	if err := db.Exec(addSQL, dropAfter).Error; err != nil {
		return fmt.Errorf("注册 retention policy 失败（telemetry_datas 可能尚未完成 57.sql 的 hypertable 转换）: %w", err)
	}
	logrus.Infof("[TimescaleRetention] telemetry_datas retention policy 就绪: drop_after=%dms（%d 天）", dropAfter, days)
	return nil
}
