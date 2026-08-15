// 文件用途：定义 service 包对外部状态发布能力的最小接口。
// 核心逻辑：声明心跳监控需要的离线状态发布方法，避免 service 与 app 层实现循环依赖。
// 关键注意事项：接口变更会影响 broker/app 注入边界，应保持小而稳定并明确错误语义。
// 重构建议：为状态发布增加契约测试或 fake，实现发布失败、幂等和调用顺序的边界验证。
package service

// StatusPublisher 状态发布接口
// 由 HeartbeatMonitor 使用，在 app 层由 Flow Bus 实现
// 接口定义在使用方（service），避免循环依赖
type StatusPublisher interface {
	// PublishStatusOffline 发布设备离线状态
	// deviceID: 设备ID
	// source: 离线来源（如 "heartbeat_expired", "timeout_expired"）
	PublishStatusOffline(deviceID, source string) error
}
