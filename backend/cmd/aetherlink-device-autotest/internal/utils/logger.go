// 文件用途：提供设备自动测试工具的全局日志初始化入口。
// 核心逻辑：按 log level 选择 zap development 或 production 配置，并暴露 Sync 刷新日志缓冲。
// 关键注意事项：当前全局 Logger 与显式传入 logger 的方式并存，审查时需避免产生双日志源。
// 重构建议：可统一为依赖注入 logger，或仅保留 CLI 层全局初始化以减少共享状态。

package utils

import (
	"go.uber.org/zap"
)

var Logger *zap.Logger

// InitLogger 初始化日志
func InitLogger(level string) error {
	var config zap.Config

	if level == "debug" {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}

	var err error
	Logger, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

// Sync 刷新日志缓冲
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
