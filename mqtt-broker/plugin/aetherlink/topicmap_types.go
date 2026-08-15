// 文件用途：维护 plugin\aetherlink\topicmap_types.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import "time"

// Direction represents mapping direction: "up" or "down".
type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
)

// DeviceTopicMapping maps a device's source topic to a platform target topic.
// Mirrors the PostgreSQL table `device_topic_mappings`.
type DeviceTopicMapping struct {
	ID             int64      `gorm:"column:id;primaryKey"`
	DeviceConfigID string     `gorm:"column:device_config_id"`
	Name           string     `gorm:"column:name"`
	Direction      string     `gorm:"column:direction"`
	SourceTopic    string     `gorm:"column:source_topic"`
	TargetTopic    string     `gorm:"column:target_topic"`
	DataIdentifier *string    `gorm:"column:data_identifier"`
	Priority       int        `gorm:"column:priority"`
	Enabled        bool       `gorm:"column:enabled"`
	Description    *string    `gorm:"column:description"`
	CreatedAt      *time.Time `gorm:"column:created_at"`
	UpdatedAt      *time.Time `gorm:"column:updated_at"`
}

func (DeviceTopicMapping) TableName() string {
	return "device_topic_mappings"
}
