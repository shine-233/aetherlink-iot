// 文件用途：TB-22 表级保留期注册表的清理执行面（被 CleanSystemDataByCron 调用）。
//
// 核心逻辑：遍历 data_retention_registry 的启用行，按各自 (表名, 时间列, 保留天数,
// 批大小) 分批删除过期数据，删完回写清理水位。
//
// 关键注意事项：
//   - 与 data_policy 两条既有出口（data_type='1' 设备数据、'2' 操作日志）并存且互不干扰：
//     注册表只管"此前完全没有删除出口"的只增不删表；operation_logs 同时在两处登记时
//     删除幂等（都是按时间边界删同一批行），不会互相放大删除范围。
//   - 单行失败不中断整轮：注册表行可能指向被外键 RESTRICT 约束的表（如
//     ota_upgrade_task_details 受 devices 约束），个别表失败只告警，其余表继续清理。
//     清理是"尽量推进"的运维动作，不是必须整体成功的事务。
//   - 每轮每表的批次数有上界（retentionMaxBatchesPerRun），避免大表一次性删到
//     拖垮 cron：本轮没删完的表下一轮继续（水位只在至少删到一行时推进）。
//   - 客户数据默认关闭（迁移种子 enabled='2'），本文件不会替部署方打开它。
package service

import (
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

// retentionMaxBatchesPerRun 单轮内单表最多执行的批次数上界。
// 与批大小共同决定单表单轮删除上限（默认 50 * 10000 = 50 万行），
// 足够消化日常增量又不会让 cron 长时间持锁；积压过多时靠后续轮次追平。
const retentionMaxBatchesPerRun = 50

// cleanRetentionRegistry 执行一轮表级保留期注册表清理。
// 注册表不可用时（141.sql 未执行 / 单测 sqlite 夹具）静默跳过，返回 nil。
func cleanRetentionRegistry(now time.Time) {
	rows, err := dal.GetRetentionRegistryRows()
	if err != nil {
		logrus.Warnf("[CleanSystemDataByCron] load retention registry failed: %v", err)
		return
	}
	for _, row := range rows {
		cleanOneRetentionRegistryRow(row, now)
	}
}

// cleanOneRetentionRegistryRow 处理单行注册表：跳过停用/非法行，分批删除后回写水位。
func cleanOneRetentionRegistryRow(row *model.DataRetentionRegistry, now time.Time) {
	if row == nil {
		return
	}
	if row.Enabled != model.RetentionEnabled {
		return
	}
	if row.RetentionDays <= 0 {
		logrus.Warnf("[CleanSystemDataByCron] skip invalid retention registry day, table=%s, retention_days=%d", row.TableName, row.RetentionDays)
		return
	}
	if row.LastCleanupTime != nil && utils.IsToday(*row.LastCleanupTime) {
		return
	}

	cutoff, dataTime := retentionCutoffOf(row)
	deleted := int64(0)
	for i := 0; i < retentionMaxBatchesPerRun; i++ {
		affected, err := dal.DeleteExpiredRowsByRegistry(row, cutoff)
		if err != nil {
			// 单行失败不中断：见文件头"关键注意事项"。
			logrus.Warnf("[CleanSystemDataByCron] retention registry cleanup failed, table=%s: %v", row.TableName, err)
			break
		}
		deleted += affected
		if affected < int64(row.BatchSize) {
			break // 本批未填满 → 没有更多过期行
		}
	}
	if deleted == 0 {
		return
	}
	if err := dal.UpdateRetentionRegistryCleanupTime(row.ID, now, dataTime); err != nil {
		logrus.Warnf("[CleanSystemDataByCron] update retention registry watermark failed, table=%s: %v", row.TableName, err)
		return
	}
	logrus.Infof("[CleanSystemDataByCron] retention registry cleaned table=%s rows=%d", row.TableName, deleted)
}

// retentionCutoffOf 按 time_kind 换算删除边界：
// timestamptz 用 time.Time，unix_ms 用 UnixMilli 整数；两者语义一致——严格早于边界的行过期。
// 同时返回用于回写水位的 time.Time 形式边界。
func retentionCutoffOf(row *model.DataRetentionRegistry) (dal.RetentionCutoff, time.Time) {
	days := int(row.RetentionDays)
	if row.TimeKind == model.RetentionTimeKindUnixMs {
		ms := utils.MillisecondsTimestampDaysAgo(days)
		return dal.RetentionCutoff{UnixMs: ms}, utils.DaysAgo(days)
	}
	boundary := utils.DaysAgo(days)
	return dal.RetentionCutoff{TimestampTZ: boundary}, boundary
}
