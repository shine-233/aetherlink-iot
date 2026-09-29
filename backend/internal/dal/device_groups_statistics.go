package dal

// 文件用途：设备分组的统计聚合（在线/离线/告警台数）。
// 核心逻辑：单分组版先展开子孙分组再展开设备；批量版用一条递归 CTE 把 N 个分组
//   展开成 (root_id, group_id) 后按 root_id 聚合，把 3N 次往返压成 1 次。
// 关键注意事项：
//   - 两个版本的过滤条件必须逐条对齐：tenant_id / owner 作用域 / activate_flag='active'
//     / 告警取 latest_device_alarms 的 H·M·L。任一处漂移都会让列表页与详情页数字不一致。
//   - owner 作用域用 (? = '' OR d.owner_user_id = ?) 表达"空=不过滤"，不能改成等值比较。

import (
	"strings"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

type deviceGroupStatisticsRow struct {
	DeviceTotal  int64 `json:"device_total"`
	OnlineTotal  int64 `json:"online_total"`
	OfflineTotal int64 `json:"offline_total"`
	AlarmTotal   int64 `json:"alarm_total"`
}

func GetDeviceGroupStatistics(groupID string, tenantID string, ownerUserID *string) (*model.DeviceGroupStatistics, error) {
	groupIDs, err := GetGroupChildrenIds(groupID)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return &model.DeviceGroupStatistics{}, nil
	}

	deviceIDs, err := GetDeviceIdsByGroupIds(groupIDs)
	if err != nil {
		return nil, err
	}
	if len(deviceIDs) == 0 {
		return &model.DeviceGroupStatistics{}, nil
	}

	var row deviceGroupStatisticsRow
	placeholders := strings.TrimRight(strings.Repeat("?,", len(deviceIDs)), ",")
	sql := `
		SELECT
			COUNT(DISTINCT d.id) AS device_total,
			COALESCE(SUM(CASE WHEN d.is_online = 1 THEN 1 ELSE 0 END), 0) AS online_total,
			COALESCE(SUM(CASE WHEN d.is_online = 1 THEN 0 ELSE 1 END), 0) AS offline_total,
			COALESCE(SUM(CASE WHEN lda.alarm_status IN ('H', 'M', 'L') THEN 1 ELSE 0 END), 0) AS alarm_total
		FROM devices d
		LEFT JOIN latest_device_alarms lda ON lda.device_id = d.id AND lda.tenant_id = d.tenant_id
		WHERE d.tenant_id = ?
		  AND (? = '' OR d.owner_user_id = ?)
		  AND d.activate_flag = 'active'
		  AND d.id IN (` + placeholders + `)
	`
	normalizedOwnerUserID := normalizeOwnerUserID(ownerUserID)
	args := make([]interface{}, 0, len(deviceIDs)+3)
	args = append(args, tenantID)
	args = append(args, normalizedOwnerUserID, normalizedOwnerUserID)
	for _, deviceID := range deviceIDs {
		args = append(args, deviceID)
	}

	err = global.DB.Raw(sql, args...).Scan(&row).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	return &model.DeviceGroupStatistics{
		DeviceTotal:  row.DeviceTotal,
		OnlineTotal:  row.OnlineTotal,
		OfflineTotal: row.OfflineTotal,
		AlarmTotal:   row.AlarmTotal,
	}, nil
}

// deviceGroupStatisticsBatchRow 批量统计的单行结果，带 group_id 以便回填。
type deviceGroupStatisticsBatchRow struct {
	GroupID      string `json:"group_id"`
	DeviceTotal  int64  `json:"device_total"`
	OnlineTotal  int64  `json:"online_total"`
	OfflineTotal int64  `json:"offline_total"`
	AlarmTotal   int64  `json:"alarm_total"`
}

// GetDeviceGroupStatisticsBatch 一次性计算多个分组的统计（含各自子孙分组的设备）。
//
// 与单分组版 GetDeviceGroupStatistics 的**过滤条件逐条对齐**（tenant_id /
// owner 作用域 / activate_flag='active' / 告警取 latest_device_alarms 的 H·M·L），
// 区别只在把 N 次往返压成 1 次：递归 CTE 先把每个目标分组展开成
// (root_id, group_id) 对，再并到 r_group_device 与 devices 上按 root_id 聚合。
//
// 为什么必须批量：分组树动辄几十个节点，逐节点调用单分组版会产生 3N 次查询。
//
// tenant-scope: caller-enforced（service 层负责只传入可见分组）。
func GetDeviceGroupStatisticsBatch(groupIDs []string, tenantID string, ownerUserID *string) (map[string]*model.DeviceGroupStatistics, error) {
	result := make(map[string]*model.DeviceGroupStatistics, len(groupIDs))
	if len(groupIDs) == 0 {
		return result, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(groupIDs)), ",")
	sql := `
		WITH RECURSIVE group_tree AS (
			SELECT g.id AS root_id, g.id AS group_id
			FROM groups g
			WHERE g.id IN (` + placeholders + `)
			UNION ALL
			SELECT gt.root_id, child.id
			FROM groups child
			INNER JOIN group_tree gt ON child.parent_id = gt.group_id
		),
		device_scope AS (
			SELECT DISTINCT gt.root_id, rgd.device_id
			FROM group_tree gt
			INNER JOIN r_group_device rgd ON rgd.group_id = gt.group_id
		)
		SELECT
			ds.root_id AS group_id,
			COUNT(DISTINCT d.id) AS device_total,
			COALESCE(SUM(CASE WHEN d.is_online = 1 THEN 1 ELSE 0 END), 0) AS online_total,
			COALESCE(SUM(CASE WHEN d.is_online = 1 THEN 0 ELSE 1 END), 0) AS offline_total,
			COALESCE(SUM(CASE WHEN lda.alarm_status IN ('H', 'M', 'L') THEN 1 ELSE 0 END), 0) AS alarm_total
		FROM device_scope ds
		INNER JOIN devices d ON d.id = ds.device_id
		LEFT JOIN latest_device_alarms lda ON lda.device_id = d.id AND lda.tenant_id = d.tenant_id
		WHERE d.tenant_id = ?
		  AND (? = '' OR d.owner_user_id = ?)
		  AND d.activate_flag = 'active'
		GROUP BY ds.root_id
	`

	normalizedOwnerUserID := normalizeOwnerUserID(ownerUserID)
	args := make([]interface{}, 0, len(groupIDs)+3)
	for _, groupID := range groupIDs {
		args = append(args, groupID)
	}
	args = append(args, tenantID)
	args = append(args, normalizedOwnerUserID, normalizedOwnerUserID)

	var rows []deviceGroupStatisticsBatchRow
	if err := global.DB.Raw(sql, args...).Scan(&rows).Error; err != nil {
		logrus.Error(err)
		return nil, err
	}

	for i := range rows {
		row := rows[i]
		result[row.GroupID] = &model.DeviceGroupStatistics{
			DeviceTotal:  row.DeviceTotal,
			OnlineTotal:  row.OnlineTotal,
			OfflineTotal: row.OfflineTotal,
			AlarmTotal:   row.AlarmTotal,
		}
	}
	// 没有任何设备（或全部被过滤）的分组不会出现在结果集里，补零值，
	// 免得调用方把"没查到"和"零台设备"混为一谈。
	for _, groupID := range groupIDs {
		if _, ok := result[groupID]; !ok {
			result[groupID] = &model.DeviceGroupStatistics{}
		}
	}

	return result, nil
}
