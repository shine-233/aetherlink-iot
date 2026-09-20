// 文件用途：计算字段关联实体与遥测读取解析器（对齐 ThingsBoard 4.3 LTS 关联实体聚合）。
// 核心逻辑：从 entity_relations 图谱与网关层级拓扑动态发现关联设备，并从 telemetry_current_datas 读取最新数值。
package calcfield

import (
	"strconv"
	"strings"

	"aetherlink-iot/backend/pkg/global"
)

// ResolveRelatedTargets 根据租户、基准设备、方向与关系类型解析关联目标设备列表。
func ResolveRelatedTargets(tenantID, deviceID, direction, relationType string) ([]string, error) {
	if global.DB == nil {
		return nil, nil
	}
	tenantID = strings.TrimSpace(tenantID)
	deviceID = strings.TrimSpace(deviceID)
	if tenantID == "" || deviceID == "" {
		return nil, nil
	}

	normDir := strings.ToUpper(strings.TrimSpace(direction))
	targetSet := make(map[string]struct{})

	// 1. 查询通用实体关系表 entity_relations (P1.1)
	// 默认 (TO / DOWN / 空) 表示当前设备作为上游/汇总器，聚合其下级关联设备 (Contains/Manages/etc.)
	if normDir == "TO" || normDir == "DOWN" || normDir == "" {
		q := global.DB.Table("entity_relations").
			Select("to_id").
			Where("tenant_id = ? AND LOWER(from_type) = 'device' AND from_id = ? AND LOWER(to_type) = 'device'", tenantID, deviceID)
		if relationType != "" {
			q = q.Where("relation_type = ?", relationType)
		}
		var toIDs []string
		if err := q.Pluck("to_id", &toIDs).Error; err == nil {
			for _, id := range toIDs {
				if id != "" && id != deviceID {
					targetSet[id] = struct{}{}
				}
			}
		}
	}
	// 方向为 FROM / UP 时，查询指向当前设备的来源实体
	if normDir == "FROM" || normDir == "UP" {
		q := global.DB.Table("entity_relations").
			Select("from_id").
			Where("tenant_id = ? AND LOWER(to_type) = 'device' AND to_id = ? AND LOWER(from_type) = 'device'", tenantID, deviceID)
		if relationType != "" {
			q = q.Where("relation_type = ?", relationType)
		}
		var fromIDs []string
		if err := q.Pluck("from_id", &fromIDs).Error; err == nil {
			for _, id := range fromIDs {
				if id != "" && id != deviceID {
					targetSet[id] = struct{}{}
				}
			}
		}
	}

	// 2. 查询网关/子设备层级拓扑 (devices.parent_id)
	if normDir == "TO" || normDir == "DOWN" || normDir == "" {
		var subDeviceIDs []string
		if err := global.DB.Table("devices").
			Select("id").
			Where("tenant_id = ? AND parent_id = ?", tenantID, deviceID).
			Pluck("id", &subDeviceIDs).Error; err == nil {
			for _, id := range subDeviceIDs {
				if id != "" && id != deviceID {
					targetSet[id] = struct{}{}
				}
			}
		}
	} else if normDir == "FROM" || normDir == "UP" {
		var parentID string
		if err := global.DB.Table("devices").
			Select("parent_id").
			Where("tenant_id = ? AND id = ?", tenantID, deviceID).
			Pluck("parent_id", &parentID).Error; err == nil && parentID != "" {
			targetSet[parentID] = struct{}{}
		}
	}

	targets := make([]string, 0, len(targetSet))
	for id := range targetSet {
		targets = append(targets, id)
	}
	return targets, nil
}

// DefaultReadRelatedLatest 从 telemetry_current_datas 读取指定设备的最新数值遥测。
func DefaultReadRelatedLatest(deviceID, sourceKey string) (float64, bool) {
	if global.DB == nil {
		return 0, false
	}
	deviceID = strings.TrimSpace(deviceID)
	sourceKey = strings.TrimSpace(sourceKey)
	if deviceID == "" || sourceKey == "" {
		return 0, false
	}

	var row struct {
		NumberV *float64 `gorm:"column:number_v"`
		BoolV   *bool    `gorm:"column:bool_v"`
		StringV *string  `gorm:"column:string_v"`
	}
	err := global.DB.Table("telemetry_current_datas").
		Select("number_v, bool_v, string_v").
		Where("device_id = ? AND key = ?", deviceID, sourceKey).
		Order("ts DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return 0, false
	}

	if row.NumberV != nil {
		return *row.NumberV, true
	}
	if row.BoolV != nil {
		if *row.BoolV {
			return 1.0, true
		}
		return 0.0, true
	}
	if row.StringV != nil {
		if val, parseErr := strconv.ParseFloat(strings.TrimSpace(*row.StringV), 64); parseErr == nil {
			return val, true
		}
	}
	return 0, false
}
