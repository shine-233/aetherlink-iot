// 文件用途：gmqttd 进程信号状态机（SIGHUP 热加载 / SIGINT、SIGTERM 优雅停止）。
// 核心逻辑：热加载失败只记录并继续监听，绝不退出循环——否则 signal.Notify 仍在
// 捕获 SIGTERM 却无人读取，进程只能被 SIGKILL 杀死且插件 Unload 全部跳过。
// 停止路径使用有界 deadline 调 Stop，完成后解除信号捕获并退出循环。
// 关键注意事项：信号通道与配置加载均可注入，测试不依赖真实进程信号。

package command

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt/config"
)

const (
	// shutdownTimeoutEnv 覆盖优雅停止的总时限（time.ParseDuration 格式，如 "20s"）。
	// 需小于容器编排的宽限期（compose stop_grace_period / k8s
	// terminationGracePeriodSeconds），否则 SIGKILL 会先于插件 Unload 到达。
	shutdownTimeoutEnv = "GMQTT_SHUTDOWN_TIMEOUT"
	// defaultShutdownTimeout 与部署默认的 30s/35s 宽限期留出余量：
	// 20s 客户端排空 + 插件 Unload（server 侧每个插件另有独立上限）。
	defaultShutdownTimeout = 20 * time.Second
)

// lifecycleServer 是信号循环实际依赖的 broker 能力子集，便于测试替身。
type lifecycleServer interface {
	ApplyConfig(config.Config)
	Stop(ctx context.Context) error
}

// shutdownTimeoutFromEnv 解析停止时限；空值取默认，非法或非正值取默认并返回告警文本。
func shutdownTimeoutFromEnv(raw string) (time.Duration, string) {
	if raw == "" {
		return defaultShutdownTimeout, ""
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultShutdownTimeout, fmt.Sprintf(
			"invalid %s=%q, falling back to %s", shutdownTimeoutEnv, raw, defaultShutdownTimeout)
	}
	return d, ""
}

// loadReloadConfig 与启动路径保持同一套解析 + 环境覆盖 + 校验，
// 防止 SIGHUP 把持久化配置回退到 YAML 静态值。
func loadReloadConfig() (config.Config, error) {
	c, err := config.ParseConfig(ConfigFile)
	if err != nil {
		return config.Config{}, err
	}
	applyPersistenceEnvOverrides(&c)
	if err := validatePersistenceConfig(&c); err != nil {
		return config.Config{}, err
	}
	return c, nil
}

type signalLoop struct {
	srv             lifecycleServer
	logger          *zap.Logger
	reloadCh        <-chan os.Signal
	stopCh          <-chan os.Signal
	loadConfig      func() (config.Config, error)
	shutdownTimeout time.Duration
	// releaseSignals 在开始停止时调用，恢复信号默认处置（可为 nil）。
	releaseSignals func()
}

// run 阻塞直到收到停止信号并完成 Stop，返回 Stop 的错误。
func (l signalLoop) run() error {
	log := l.logger
	if log == nil {
		log = zap.NewNop()
	}
	timeout := l.shutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	for {
		select {
		case <-l.reloadCh:
			c, err := l.loadConfig()
			if err != nil {
				// 保留旧配置继续服务；循环不能退出，否则 SIGTERM 被吞掉。
				log.Error("reload error, keeping previous config", zap.Error(err))
				continue
			}
			l.srv.ApplyConfig(c)
			log.Info("gmqtt reloaded")
		case sig := <-l.stopCh:
			if l.releaseSignals != nil {
				l.releaseSignals()
			}
			log.Info("stop signal received, shutting down",
				zap.String("signal", fmt.Sprint(sig)), zap.Duration("timeout", timeout))
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.srv.Stop(ctx)
			cancel()
			if err != nil {
				log.Warn("gmqtt stop finished with error", zap.Error(err))
			}
			return err
		}
	}
}
