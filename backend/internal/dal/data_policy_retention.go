// 文件用途：TB-22 表级保留期注册表的 DAL 原语——读取注册表行并按行分批删除过期数据。
//
// 核心逻辑：
//  1. GetRetentionRegistryRows 读取全部注册表行（表很小，全量读即可；缺表时返回空，
//     让未执行 141.sql 的存量库也能正常启动）。
//  2. DeleteExpiredRowsByRegistry 对单行执行一次分批 DELETE：以 ctid 子查询限定批大小，
//     不假设被清理表一定有名为 id 的主键（event_datas / *_set_logs 等结构各异），
//     也避免单条大 DELETE 的长事务与 WAL 尖峰。
//
// 关键注意事项（注入面）：
//   - table_name / time_column 来自数据库行，会被拼进 SQL 文本，属于标识符拼装。
//     必须同时通过：① 严格小写标识符正则；② to_regclass 存在性校验。
//     两条缺一不可——正则挡住引号/分号/注释，to_regclass 挡住"名字合法但表不存在"
//     与大小写陷阱。绝不拼接任何来自 HTTP 请求的取值。
//   - 表名统一以 public. 限定，避免 search_path 被改写后误删同名表。
//   - time_kind 决定边界换算：timestamptz 用 time.Time，unix_ms 用 UnixMilli 整数；
//     两者语义都是"严格早于 cutoff 的行已过期"。
//   - resolved_only 只用于死信类表：未解决的 pending/retrying/dead 是可重放资产，
//     按时间删除会让重放失去输入。
package dal

import (
	"fmt"
	"regexp"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// retentionIdentifierPattern 允许拼进 SQL 的标识符形态：小写字母开头，仅含小写字母、
// 数字与下划线。不含引号、点号、分号、空白与注释符——任何试图逃逸标识符位置的输入
// （如 `foo; DROP` 或 `foo" --`）都在此被拒绝，PG 侧不再有机会解析它。
var retentionIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// retentionRegistrySelectSQL 注册表行读取。列顺序与 model.DataRetentionRegistry 对齐。
const retentionRegistrySelectSQL = `SELECT id, table_name, time_column, time_kind, retention_days,
       category, enabled, batch_size, resolved_only, last_cleanup_time, last_cleanup_data_time,
       remark, created_at, updated_at
FROM public.data_retention_registry
ORDER BY table_name`

// GetRetentionRegistryRows 读取全部注册表行。
// 注册表尚未建立（141.sql 未执行）时返回空切片与 nil 错误——清理是可选能力，
// 不应因为缺表让 cron 报错或让启动失败。
// tenant-scope: system-table —— 注册表是平台管理面配置，无租户列。
func GetRetentionRegistryRows() ([]*model.DataRetentionRegistry, error) {
	var exists bool
	if err := global.DB.Raw(`SELECT to_regclass('public.data_retention_registry') IS NOT NULL`).Scan(&exists).Error; err != nil {
		// 探测失败一律按"注册表不可用"处理：清理是可选能力，不能因为一次探测报错
		// 让 cron 失败或让启动中断。sqlite（后端单测夹具）没有 to_regclass，
		// 走的就是这条分支——注册表清理在单测里整段跳过，语义不受影响。
		logrus.Warnf("[retention-registry] probe data_retention_registry failed, skip registry cleanup: %v", err)
		return nil, nil
	}
	if !exists {
		return nil, nil
	}
	rows := make([]*model.DataRetentionRegistry, 0)
	if err := global.DB.Raw(retentionRegistrySelectSQL).Scan(&rows).Error; err != nil {
		logrus.Error(err)
		return nil, err
	}
	return rows, nil
}

// RetentionCutoff 单行注册表一次清理的时间边界：两个字段只有一个有语义，由 TimeKind 决定。
type RetentionCutoff struct {
	TimestampTZ time.Time // time_kind='timestamptz' 时的边界
	UnixMs      int64     // time_kind='unix_ms' 时的边界
}

// retentionQualifiedTable 标识符白名单 + 存在性校验后的限定表名（public.xxx）。
// 返回错误时调用方必须跳过该行，不得继续拼 SQL。
// 第二返回值表示目标关系是否为分区表（TimescaleDB hypertable 也在此列）——
// 分区表上不能使用 ctid 分批，见 DeleteExpiredRowsByRegistry 的说明。
func retentionQualifiedTable(row *model.DataRetentionRegistry) (string, bool, error) {
	if row == nil {
		return "", false, fmt.Errorf("nil retention registry row")
	}
	if !retentionIdentifierPattern.MatchString(row.TableName) {
		return "", false, fmt.Errorf("retention registry table name rejected by identifier whitelist: %q", row.TableName)
	}
	if !retentionIdentifierPattern.MatchString(row.TimeColumn) {
		return "", false, fmt.Errorf("retention registry time column rejected by identifier whitelist: %q", row.TimeColumn)
	}
	qualified := "public." + row.TableName
	// 二次校验：名字形态合法但实际不是已知关系（缺表/被误改/大小写陷阱）时拒绝。
	var exists bool
	if err := global.DB.Raw("SELECT to_regclass(?) IS NOT NULL", qualified).Scan(&exists).Error; err != nil {
		return "", false, err
	}
	if !exists {
		return "", false, fmt.Errorf("retention registry table does not exist: %s", qualified)
	}
	// 分区判定：alarm_info 在生产上是 TimescaleDB hypertable（57.sql 转换），
	// hypertable 以 PG 原生分区实现，ctid 只在单个 chunk 内唯一。
	var relkind string
	if err := global.DB.Raw(
		"SELECT relkind::text FROM pg_class WHERE oid = to_regclass(?)", qualified,
	).Scan(&relkind).Error; err != nil {
		return "", false, err
	}
	return qualified, relkind == "p", nil
}

// DeleteExpiredRowsByRegistry 对单行注册表执行一次分批 DELETE，返回本次删除行数。
// 删除条件：<时间列> < cutoff [AND status = 'resolved']，批大小由行的 batch_size 决定。
// 用 ctid 而非主键圈定批次：被清理表结构各异，ctid 是 PG 保证存在的物理定位符，
// 且不要求调用方知道主键列名。
//
// 分区表（含 TimescaleDB hypertable）例外：ctid 只在单个 chunk/分区内唯一，
// 跨分区复用同一个 ctid 值会让 `ctid IN (...)` 命中别的 chunk 上的行——
// 对该类关系退化为不带 LIMIT 的单条 DELETE（行数上界由保留期边界天然约束：
// 稳态下每轮只删一天的量，首轮追平历史时由运维自行在低峰触发）。
func DeleteExpiredRowsByRegistry(row *model.DataRetentionRegistry, cutoff RetentionCutoff) (int64, error) {
	qualified, partitioned, err := retentionQualifiedTable(row)
	if err != nil {
		return 0, err
	}

	var (
		cond string
		arg  interface{}
	)
	switch row.TimeKind {
	case model.RetentionTimeKindUnixMs:
		cond = fmt.Sprintf("%s < ?", row.TimeColumn)
		arg = cutoff.UnixMs
	case model.RetentionTimeKindTimestampTZ:
		cond = fmt.Sprintf("%s < ?", row.TimeColumn)
		arg = cutoff.TimestampTZ
	default:
		return 0, fmt.Errorf("unknown retention time_kind %q for table %s", row.TimeKind, row.TableName)
	}
	// 死信类只回收已解决行：未解决行是可重放资产，按时间删会切断重放输入。
	if row.ResolvedOnly {
		cond += " AND status = 'resolved'"
	}

	var sql string
	var args []interface{}
	if partitioned {
		sql = fmt.Sprintf(`DELETE FROM %s WHERE %s`, qualified, cond)
		args = []interface{}{arg}
	} else {
		// ctid 子查询限定批大小，使 DELETE 的单语句影响行数有上界。
		batchSize := row.BatchSize
		if batchSize <= 0 {
			batchSize = 10000
		}
		sql = fmt.Sprintf(
			`DELETE FROM %s WHERE ctid IN (SELECT ctid FROM %s WHERE %s LIMIT ?)`,
			qualified, qualified, cond)
		args = []interface{}{arg, batchSize}
	}
	result := global.DB.Exec(sql, args...)
	if result.Error != nil {
		logrus.Error(result.Error)
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// UpdateRetentionRegistryCleanupTime 回写单行注册表的清理水位。
// last_cleanup_data_time 记录"已清理到的时间边界"，供运维核对进度；
// 只在成功清理后调用，失败轮次不推进水位（下轮从头重试同一区间）。
func UpdateRetentionRegistryCleanupTime(id string, now time.Time, dataTime time.Time) error {
	return global.DB.Exec(
		`UPDATE public.data_retention_registry
		 SET last_cleanup_time = ?, last_cleanup_data_time = ?, updated_at = now()
		 WHERE id = ?`, now, dataTime, id).Error
}
