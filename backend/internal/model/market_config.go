// 文件用途：提供 market config 相关模型补充类型、常量或转换 helper，支撑 backend/internal/model 内的共享数据契约。
// 核心逻辑：围绕模型层的通用结构、枚举和轻量转换函数组织代码，供 API、DAL 与 service 层调用。
// 关键注意事项：模型文件应保持无副作用和轻业务逻辑，复杂校验、权限判断或事务编排应留在 service/DAL 层。
// 重构建议：随着模型职责增多，可按领域拆分文件并为关键转换补充单元测试，避免通用文件继续膨胀。

package model

// MarketConfig holds configuration for the Horizon Market Service integration
type MarketConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
}
