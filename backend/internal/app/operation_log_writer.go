// 文件用途：承载应用启动编排中「操作日志异步写入」相关的配置与服务接线。
// 核心逻辑：把 viper 配置映射成 middleware.OperationLogWriterConfig 并启动后台写入器；
//           停止由 Application.Shutdown 显式调用，必须在关闭数据库之前完成。
// 关键注意事项：**默认关闭**——开启后审计条目在硬崩溃（kill -9 / 掉电）时会丢失队列中
//           尚未落库的部分，这是异步审计的固有代价，应由部署方显式选择。
//           与 141.sql「客户数据默认关闭」、timescale_mode「默认 auto 保持原行为」同一惯例。

package app

import (
	"time"

	"aetherlink-iot/backend/internal/middleware"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// operationLogShutdownTimeout 关停时等待队列 drain 的上限。
const operationLogShutdownTimeout = 5 * time.Second

// WithOperationLogWriter 按配置启动操作日志异步批量写入器（默认关闭）。
func WithOperationLogWriter() Option {
	return func(a *Application) error {
		config := middleware.DefaultOperationLogWriterConfig()

		if viper.IsSet("operation_log.async_enabled") {
			config.Enabled = viper.GetBool("operation_log.async_enabled")
		}
		if v := viper.GetInt("operation_log.async_queue_size"); v > 0 {
			config.QueueSize = v
		}
		if v := viper.GetInt("operation_log.async_batch_size"); v > 0 {
			config.BatchSize = v
		}
		// 与 diagnostics 同款：支持 "1s" 字符串与整数毫秒两种写法。
		if viper.IsSet("operation_log.async_flush_interval") {
			if s := viper.GetString("operation_log.async_flush_interval"); s != "" {
				if d, err := time.ParseDuration(s); err == nil {
					config.FlushInterval = d
				}
			} else if ms := viper.GetInt("operation_log.async_flush_interval"); ms > 0 {
				config.FlushInterval = time.Duration(ms) * time.Millisecond
			}
		}

		if !config.Enabled {
			logrus.Info("operation log async writer disabled (operation_log.async_enabled=false), keeping synchronous insert")
			return nil
		}

		logrus.Infof(
			"operation log async writer config: queue=%d batch=%d interval=%v",
			config.QueueSize, config.BatchSize, config.FlushInterval,
		)
		return middleware.StartOperationLogWriter(config)
	}
}

// stopOperationLogWriter 由 Application.Shutdown 调用：先 drain 再停，
// 必须早于数据库连接关闭，否则剩余条目会全部写失败。
func stopOperationLogWriter() {
	middleware.StopOperationLogWriter(operationLogShutdownTimeout)
}
