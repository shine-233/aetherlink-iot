// 文件用途：提供遥测、属性或事件存储模块的 interfaces 能力。
// 核心逻辑：管理存储配置、消息模型、批量写入、去重、指标采集和直写通道，主要围绕 type Logger 等声明展开。
// 关键注意事项：存储链路涉及并发、通道关闭和数据库表结构，修改需保持写入顺序与失败处理可观测。
// 重构建议：后续可将批处理策略、指标和数据库写入进一步解耦，便于压测和替换实现。

package storage

// Logger 日志接口
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
}
