package dal

// 文件用途：OTA 升级任务的运维支持包（support bundle）读取。
// 核心逻辑：一次调用产出三份材料——状态分布、失败原因分组、失败样本明细；
//   失败样本带 LIMIT 上限，避免一个失败上万台的任务把整表拉进内存。
// 关键注意事项：
//   - 三条 SQL 共用同一段 baseJoin + whereClause，失败分组/样本只是在其后再 AND status = failed，
//     改动过滤条件时必须三处一起改，否则三份材料口径不一致。
//   - Raw + map 扫描拿到的 count 在不同驱动下可能是 int64/float64/[]byte/string，
//     必须过 int64FromSQLValue 归一化，不能直接类型断言。

import (
	"strconv"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
)

func GetOTAUpgradeTaskSupportBundleRows(taskID string, tenantID string, includeAllTenants bool, failedSampleLimit int) (*OTAUpgradeTaskSupportBundleRows, error) {
	if failedSampleLimit <= 0 {
		failedSampleLimit = 50
	}

	whereClause := "WHERE d.ota_upgrade_task_id = ?"
	params := []interface{}{taskID}
	if !includeAllTenants {
		whereClause += " AND p.tenant_id = ?"
		params = append(params, tenantID)
	}

	baseJoin := ` FROM ota_upgrade_task_details d
		JOIN ota_upgrade_tasks t ON t.id = d.ota_upgrade_task_id
		JOIN ota_upgrade_packages p ON p.id = t.ota_upgrade_package_id `

	result := &OTAUpgradeTaskSupportBundleRows{
		FailedRows:    make([]map[string]interface{}, 0),
		Statistics:    make([]map[string]interface{}, 0),
		FailureGroups: make([]model.OTAUpgradeTaskFailureGroup, 0),
	}

	statsSQL := `SELECT d.status AS status, COUNT(*) AS count` + baseJoin + whereClause + ` GROUP BY d.status ORDER BY d.status ASC`
	if err := global.DB.Raw(statsSQL, params...).Scan(&result.Statistics).Error; err != nil {
		return nil, err
	}
	result.TotalRows, result.FailedCount = otaSupportBundleStatusTotals(result.Statistics)
	if result.FailedCount == 0 {
		return result, nil
	}

	failureGroupParams := append([]interface{}{}, params...)
	failureGroupParams = append(failureGroupParams, model.OtaUpgradeTaskDetailStatusFailed)
	failureGroupSQL := `SELECT COALESCE(d.status_description, '') AS reason, COUNT(*) AS count` +
		baseJoin + whereClause + ` AND d.status = ? GROUP BY COALESCE(d.status_description, '') ORDER BY count DESC, reason ASC`
	if err := global.DB.Raw(failureGroupSQL, failureGroupParams...).Scan(&result.FailureGroups).Error; err != nil {
		return nil, err
	}

	failedRowsParams := append([]interface{}{}, params...)
	failedRowsParams = append(failedRowsParams, model.OtaUpgradeTaskDetailStatusFailed, failedSampleLimit)
	failedRowsSQL := `SELECT d.id,
			d.ota_upgrade_task_id,
			d.device_id,
			dev.device_number,
			dev.name,
			dev.current_version,
			p.version,
			d.steps,
			d.updated_at,
			d.status,
			d.status_description` +
		baseJoin + ` JOIN devices dev ON dev.id = d.device_id ` +
		whereClause + ` AND d.status = ? ORDER BY d.updated_at DESC, d.id ASC LIMIT ?`
	if err := global.DB.Raw(failedRowsSQL, failedRowsParams...).Scan(&result.FailedRows).Error; err != nil {
		return nil, err
	}

	return result, nil
}

func otaSupportBundleStatusTotals(statistics []map[string]interface{}) (int64, int64) {
	var total int64
	var failed int64
	for _, item := range statistics {
		count := int64FromSQLValue(item["count"])
		total += count
		if int16FromSQLValue(item["status"]) == model.OtaUpgradeTaskDetailStatusFailed {
			failed += count
		}
	}
	return total, failed
}

func int16FromSQLValue(value interface{}) int16 {
	value64 := int64FromSQLValue(value)
	parsed, err := strconv.ParseInt(strconv.FormatInt(value64, 10), 10, 16)
	if err != nil {
		return 0
	}
	return int16(parsed)
}

func int64FromSQLValue(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		if v > uint64(^uint(0)>>1) {
			return int64(^uint(0) >> 1)
		}
		return int64(v)
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	case []byte:
		parsed, _ := strconv.ParseInt(string(v), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(v, 10, 64)
		return parsed
	default:
		return 0
	}
}
