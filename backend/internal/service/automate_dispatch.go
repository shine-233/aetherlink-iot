// 文件用途：自动化触发的统一异步入口与工作池生命周期。
// 核心逻辑：遥测/属性/事件/状态四类上行都经 Dispatch 进入同一个按设备分片的有界工作池，
// 取代各上行自行 go func + 进程级互斥锁的做法。
// 关键注意事项：Dispatch 永不阻塞调用方；返回的错误只表示“入队失败”（队列满/已停止），
// 执行期错误由池内记录日志。
package service

import (
	"context"
	"errors"
	"sync"

	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// automationPoolConfigFromViper 读取可选配置 automation.worker_pool.{workers,queue_size}；
// 缺省（0）时回落到内置默认值。
func automationPoolConfigFromViper() AutomationPoolConfig {
	return AutomationPoolConfig{
		Workers:   viper.GetInt("automation.worker_pool.workers"),
		QueueSize: viper.GetInt("automation.worker_pool.queue_size"),
	}
}

var (
	automationPoolMu sync.Mutex
	automationPool   *shardedPool
	// automationPoolExecute 可注入：测试用它替换真实执行，观察分片/保序/并发行为。
	automationPoolExecute = func(a *Automate, device *model.Device, fromExt AutomateFromExt) error {
		return a.Execute(device, fromExt)
	}
)

// StartAutomationPool 以给定参数启动（或在已停止后重新启动）自动化工作池。
// 已在运行时不做任何事并返回 false。未显式启动时，首次 Dispatch 以默认参数惰性启动。
func StartAutomationPool(cfg AutomationPoolConfig) bool {
	automationPoolMu.Lock()
	defer automationPoolMu.Unlock()
	if automationPool != nil && !automationPool.isStopped() {
		return false
	}
	automationPool = newShardedPool(cfg)
	return true
}

// StopAutomationPool 停止接收新触发并排空已入队任务，受 ctx 截止时间约束。可重复调用。
func StopAutomationPool(ctx context.Context) error {
	automationPoolMu.Lock()
	pool := automationPool
	automationPoolMu.Unlock()
	if pool == nil {
		return nil
	}
	return pool.stop(ctx)
}

// AutomationPoolSnapshot 返回当前工作池计数；未启动时为零值。
func AutomationPoolSnapshot() AutomationPoolStats {
	automationPoolMu.Lock()
	pool := automationPool
	automationPoolMu.Unlock()
	if pool == nil {
		return AutomationPoolStats{}
	}
	return pool.stats()
}

func currentAutomationPool() *shardedPool {
	automationPoolMu.Lock()
	defer automationPoolMu.Unlock()
	if automationPool == nil {
		automationPool = newShardedPool(automationPoolConfigFromViper())
	}
	return automationPool
}

// Dispatch 把一次设备触发投递到按 deviceID 分片的工作池：同设备保序，跨设备并发。
// 返回 ErrAutomationQueueFull / ErrAutomationPoolStopped 表示本次触发未入队。
func (a *Automate) Dispatch(device *model.Device, fromExt AutomateFromExt) error {
	if device == nil {
		return errors.New("device info is required")
	}
	return currentAutomationPool().submit(device.ID, func() {
		if err := automationPoolExecute(a, device, fromExt); err != nil {
			logrus.WithFields(logrus.Fields{
				"device_id":          device.ID,
				"trigger_param_type": fromExt.TriggerParamType,
				"error":              err,
			}).Error("automation execute failed")
		}
	})
}
