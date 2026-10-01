// 文件用途：自动化触发工作池的应用层生命周期托管。
// 核心逻辑：Start 以配置启动 service 层按设备分片的有界工作池；Stop 拒绝新触发并在超时内排空已入队任务。
// 关键注意事项：必须注册在 MQTT/上行服务之前——ServiceManager 反序停机，
// 这样上行先停止产生触发，工作池随后排空，不会在停机窗口里丢掉已入队的触发。
package app

import (
	"context"
	"time"

	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const automationPoolStopTimeout = 20 * time.Second

// AutomationPoolService 托管自动化工作池的启停。
type AutomationPoolService struct{}

func (AutomationPoolService) Name() string { return "自动化触发工作池" }

func (AutomationPoolService) Start() error {
	cfg := service.AutomationPoolConfig{
		Workers:   viper.GetInt("automation.worker_pool.workers"),
		QueueSize: viper.GetInt("automation.worker_pool.queue_size"),
	}
	service.StartAutomationPool(cfg)
	return nil
}

func (AutomationPoolService) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), automationPoolStopTimeout)
	defer cancel()
	err := service.StopAutomationPool(ctx)
	stats := service.AutomationPoolSnapshot()
	logrus.WithFields(logrus.Fields{
		"submitted": stats.Submitted,
		"completed": stats.Completed,
		"dropped":   stats.Dropped,
		"rejected":  stats.Rejected,
		"panics":    stats.Panics,
	}).Info("automation pool stopped")
	return err
}

// WithAutomationPool 注册自动化触发工作池。须排在 WithMQTTService 之前。
func WithAutomationPool() Option {
	return func(application *Application) error {
		application.RegisterService(AutomationPoolService{})
		return nil
	}
}
