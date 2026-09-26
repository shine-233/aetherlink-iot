// 文件用途：TB-15R 行级数据保留 TTL 的作用域删除原语——按 data_policy 行的
// 租户/档案粒度圈定过期遥测（热层 telemetry_datas、冷层 telemetry_rollups）的删除范围。
// 核心逻辑：一条原始行 → 一个作用域 → 分批 DELETE，删除语句内联设备范围过滤子查询：
//   - 档案级 (tenant_id, device_config_id 均非空)：精确删除该租户该档案设备的数据；
//   - 租户级 (仅 tenant_id 非空)：删除该租户全部设备的数据，但排除同租户内被启用
//     档案级策略覆盖的设备（档案精确优先，租户级只兜"档案没有单独策略"的设备）；
//   - 全局 (tenant_id 为空)：删除全部设备的数据，但排除被任何启用行级策略
//     （租户级/档案级）覆盖的设备（行级优先、全局回落）。
//
// 关键注意事项：排除条件只认 data_type='1' 且 enabled='1' 且 retention_days>0 的
// 行级策略——停用或非法策略不产生覆盖，其设备回落到更宽作用域正常清理。
// SQL 保持 PG / sqlite 双兼容（后端单测用内存 sqlite 锁定覆盖语义）。
// 重构建议：若行级粒度扩展到操作日志等新数据类型，在此抽公共作用域描述而非复制分支。
package dal

import (
	"fmt"

	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// telemetryCleanupScope 删除作用域参数，字段语义与 data_policy 行级列对齐
// （空串 = NULL）。全局回落/租户级的排除语义由 telemetryDeviceScopeFilter 按组合推导。
type telemetryCleanupScope struct {
	TenantID       string
	DeviceConfigID string
}

// telemetryDeviceScopeFilter 返回内层查询中对别名 alias 表（telemetry_datas /
// telemetry_rollups）device_id 列的作用域过滤片段与绑定参数。
// 片段以 AND 开头，拼接在 `WHERE <时间列> <= ?` 之后。
func telemetryDeviceScopeFilter(alias string, scope telemetryCleanupScope) (string, []interface{}) {
	switch {
	case scope.TenantID != "" && scope.DeviceConfigID != "":
		// 档案级：精确 (租户, 档案)。设备未绑档案（device_config_id IS NULL）不被档案级行覆盖。
		return fmt.Sprintf(
			`AND %[1]s.device_id IN (
					SELECT d.id FROM devices d
					WHERE d.tenant_id = ? AND d.device_config_id = ?
				)`, alias), []interface{}{scope.TenantID, scope.DeviceConfigID}
	case scope.TenantID != "":
		// 租户级：租户全部设备，排除同租户被启用档案级策略覆盖的设备。
		return fmt.Sprintf(
			`AND %[1]s.device_id IN (SELECT d.id FROM devices d WHERE d.tenant_id = ?)
				AND %[1]s.device_id NOT IN (
					SELECT d2.id FROM devices d2
					WHERE d2.tenant_id = ? AND d2.device_config_id IN (
						SELECT p.device_config_id FROM data_policy p
						WHERE p.tenant_id = ? AND p.data_type = '1' AND p.enabled = '1'
							AND p.retention_days > 0 AND p.device_config_id IS NOT NULL
					)
				)`, alias), []interface{}{scope.TenantID, scope.TenantID, scope.TenantID}
	default:
		// 全局回落：排除被任何启用行级策略覆盖的设备
		//（租户级行覆盖其全部设备；档案级行只覆盖绑定了该档案的设备）。
		return fmt.Sprintf(
			`AND %[1]s.device_id NOT IN (
					SELECT d.id FROM devices d
					JOIN data_policy p ON p.tenant_id IS NOT NULL
						AND p.data_type = '1' AND p.enabled = '1' AND p.retention_days > 0
						AND d.tenant_id = p.tenant_id
						AND (p.device_config_id IS NULL OR p.device_config_id = d.device_config_id)
				)`, alias), nil
	}
}

// telemetryScopedDeleteBatchSQL 热层作用域分批删除：与 deleteTelemetryDataBatch 同一
// "主键元组 IN 子查询限量提交"骨架，仅内层多拼一段设备范围过滤。
const telemetryScopedDeleteBatchSQL = `
		DELETE FROM telemetry_datas
		WHERE (device_id, key, ts) IN (
			SELECT td.device_id, td.key, td.ts
			FROM telemetry_datas td
			WHERE td.ts <= ? %s
			ORDER BY td.ts
			LIMIT ?
		)`

// telemetryRollupsScopedDeleteBatchSQL 冷层作用域分批删除，口径对齐
// telemetryRollupDeleteBatchSQL（bucket_start <= cutoff 毫秒）。
const telemetryRollupsScopedDeleteBatchSQL = `
		DELETE FROM telemetry_rollups
		WHERE (device_id, key, bucket_ms, bucket_start) IN (
			SELECT tr.device_id, tr.key, tr.bucket_ms, tr.bucket_start
			FROM telemetry_rollups tr
			WHERE tr.bucket_start <= ? %s
			ORDER BY tr.bucket_start
			LIMIT ?
		)`

func deleteTelemetryDataBatchScoped(cutoff int64, batchSize int, scope telemetryCleanupScope) (int64, error) {
	filter, filterArgs := telemetryDeviceScopeFilter("td", scope)
	params := append([]interface{}{cutoff}, filterArgs...)
	params = append(params, batchSize)
	result := global.DB.Exec(fmt.Sprintf(telemetryScopedDeleteBatchSQL, filter), params...)
	return result.RowsAffected, result.Error
}

func deleteTelemetryRollupsBatchScoped(cutoff int64, batchSize int, scope telemetryCleanupScope) (int64, error) {
	filter, filterArgs := telemetryDeviceScopeFilter("tr", scope)
	params := append([]interface{}{cutoff}, filterArgs...)
	params = append(params, batchSize)
	result := global.DB.Exec(fmt.Sprintf(telemetryRollupsScopedDeleteBatchSQL, filter), params...)
	return result.RowsAffected, result.Error
}

// DeleteTelemetrDataByTimeForScope 按行级策略作用域分批删除过期遥测（TB-15R）。
// tenantID 为空 => 全局回落（排除被启用行级策略覆盖的设备）；
// tenantID 非空且 deviceConfigID 为空 => 租户级（排除同租户被档案级覆盖的设备）；
// 两者均非空 => 档案级精确范围。
// tenant-scope: system-job —— 数据保留清理作业按 data_policy 策略行显式圈定
// 租户/档案范围，只删不读，范围完全来自策略行本身。
func DeleteTelemetrDataByTimeForScope(cutoff int64, tenantID, deviceConfigID string) error {
	scope := telemetryCleanupScope{TenantID: tenantID, DeviceConfigID: deviceConfigID}
	for {
		deleted, err := deleteTelemetryDataBatchScoped(cutoff, telemetryRetentionDeleteBatchSize, scope)
		if err != nil {
			logrus.Error(err)
			return err
		}
		if deleted < telemetryRetentionDeleteBatchSize {
			return nil
		}
	}
}

// DeleteTelemetryRollupsByTimeForScope 冷层同口径作用域删除（TB-15R）：
// 原始行按策略作用域删到哪一档，冷层派生行就跟到哪一档，避免"冷层只进不出"。
// tenant-scope: system-job —— 同 DeleteTelemetrDataByTimeForScope。
func DeleteTelemetryRollupsByTimeForScope(cutoff int64, tenantID, deviceConfigID string) error {
	scope := telemetryCleanupScope{TenantID: tenantID, DeviceConfigID: deviceConfigID}
	for {
		deleted, err := deleteTelemetryRollupsBatchScoped(cutoff, telemetryRetentionDeleteBatchSize, scope)
		if err != nil {
			logrus.Error(err)
			return err
		}
		if deleted < telemetryRetentionDeleteBatchSize {
			return nil
		}
	}
}
