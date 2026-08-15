// 文件用途：维护 plugin\aetherlink\topicmap_repo.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"context"
	"errors"
	"sort"
)

// LoadEnabledMappings loads enabled mappings for a device_config_id and direction, sorted by priority ASC.
func LoadEnabledMappings(ctx context.Context, deviceConfigID string, direction Direction) ([]DeviceTopicMapping, error) {
	if deviceConfigID == "" {
		return nil, errors.New("empty deviceConfigID")
	}
	var rows []DeviceTopicMapping
	tx := db.WithContext(ctx).
		Model(&DeviceTopicMapping{}).
		Where("device_config_id = ? AND direction = ? AND enabled = true", deviceConfigID, string(direction)).
		Find(&rows)
	if tx.Error != nil {
		return nil, tx.Error
	}
	// Ensure ascending priority
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Priority < rows[j].Priority })
	return rows, nil
}
