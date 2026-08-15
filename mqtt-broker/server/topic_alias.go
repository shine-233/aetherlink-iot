// 文件用途：定义 MQTT v5 topic alias 管理器工厂和客户端别名检查接口。
// 核心逻辑：由具体实现决定 Publish 是否复用已有 alias 或分配新 alias。
// 使用注意：topic alias 属于 MQTT v5 wire-level 兼容边界，返回值语义不能随意改变。
// 重构建议：后续可把 alias 策略说明和 FIFO 实现关系补充到目录 README。

package server

import (
	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

type NewTopicAliasManager func(config config.Config, maxAlias uint16, clientID string) TopicAliasManager

// TopicAliasManager manage the topic alias for a V5 client.
// see topicalias/fifo for more details.
type TopicAliasManager interface {
	// Check return the alias number and whether the alias exist.
	// For examples:
	// If the Publish alias exist and the manager decides to use the alias, it return the alias number and true.
	// If the Publish alias exist, but the manager decides not to use alias, it return 0 and true.
	// If the Publish alias not exist and the manager decides to assign a new alias, it return the new alias and false.
	// If the Publish alias not exist, but the manager decides not to assign alias, it return the 0 and false.
	Check(publish *packets.Publish) (alias uint16, exist bool)
}
