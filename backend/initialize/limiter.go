// 文件用途：提供启动后可复用的内存限流器，保护自动化触发与设备认证等高频入口。
// 核心逻辑：按 key 懒加载 `rate.Limiter`，把不同设备或请求源的限流状态隔离保存。
// 关键注意事项：限流参数是业务行为的一部分，注释可增强说明，但不可误导为动态配置能力。
// 重构建议：后续可增加清理策略或显式配置注入，避免 map 长期增长并提升测试可控性。

package initialize

import (
	"sync"

	"golang.org/x/time/rate"
)

type AutomateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

var alimit *AutomateLimiter

// NewAutomateLimiter 返回自动化流程共用的限流器单例。
func NewAutomateLimiter() *AutomateLimiter {
	if alimit == nil {
		alimit = &AutomateLimiter{
			limiters: make(map[string]*rate.Limiter),
		}
	}
	return alimit
}

// GetLimiter 按业务键获取限流器；若不存在则以固定阈值创建。
func (rl *AutomateLimiter) GetLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, ok := rl.limiters[key]
	if !ok {
		limiter = rate.NewLimiter(rate.Limit(1.0/3.0), 10) // 每秒处理1个请求，最多允许10个并发请求
		rl.limiters[key] = limiter
	}
	return limiter
}

// Allow 检查当前 key 是否还能立即通过一次自动化处理请求。
func (rl *AutomateLimiter) Allow(key string) bool {
	limiter := rl.GetLimiter(key)
	return limiter.Allow()
}

type DeviceAuthLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

var daLimiter *DeviceAuthLimiter

// NewDeviceAuthLimiter 返回设备认证流程共用的限流器单例。
func NewDeviceAuthLimiter() *DeviceAuthLimiter {
	if daLimiter == nil {
		daLimiter = &DeviceAuthLimiter{
			limiters: make(map[string]*rate.Limiter),
		}
	}
	return daLimiter
}

// GetLimiter 按设备认证键获取限流器；未命中时使用更严格的突发容量创建。
func (dal *DeviceAuthLimiter) GetLimiter(key string) *rate.Limiter {
	dal.mu.Lock()
	defer dal.mu.Unlock()

	limiter, ok := dal.limiters[key]
	if !ok {
		limiter = rate.NewLimiter(rate.Limit(1.0/3.0), 1) // 每3秒1个请求，突发容量为1
		dal.limiters[key] = limiter
	}
	return limiter
}

func (dal *DeviceAuthLimiter) Allow(key string) bool {
	limiter := dal.GetLimiter(key)
	return limiter.Allow()
}
