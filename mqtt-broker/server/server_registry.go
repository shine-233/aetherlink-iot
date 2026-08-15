package server

import "fmt"

var (
	plugins              = make(map[string]NewPlugin)
	topicAliasMgrFactory = make(map[string]NewTopicAliasManager)
	persistenceFactories = make(map[string]NewPersistence)
)

func RegisterPersistenceFactory(name string, new NewPersistence) {
	if _, ok := persistenceFactories[name]; ok {
		panic("duplicated persistence factory: " + name)
	}
	persistenceFactories[name] = new
}

func RegisterTopicAliasMgrFactory(name string, new NewTopicAliasManager) {
	if _, ok := topicAliasMgrFactory[name]; ok {
		panic("duplicated topic alias manager factory: " + name)
	}
	topicAliasMgrFactory[name] = new
}

// RegisterPlugin 按配置中的 plugin_order 名称注册插件工厂。
// 使用注意：重复别名会返回错误，避免在 init 阶段 panic，让 broker 启动失败原因更明确。
func RegisterPlugin(name string, new NewPlugin) error {
	if _, ok := plugins[name]; ok {
		return fmt.Errorf("duplicated plugin: %s", name)
	}
	plugins[name] = new
	return nil
}
