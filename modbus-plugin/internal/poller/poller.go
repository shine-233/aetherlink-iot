// 文件用途：轮询采集器（ROADMAP B1）。
// 核心逻辑：按设备周期读取全部点表，产出键值快照交由 Reporter 上报；错误退避不中断循环。
// 关键注意事项：读值失败仅记录并跳过本轮，不影响后续轮次；ctx 取消即退出。
package poller

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/config"
	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/modbusclient"
)

// Reporter 上报接口（reporter 包实现，测试可替换）。
type Reporter interface {
	PublishTelemetry(deviceNumber string, values map[string]any) error
}

// DevicePoller 单设备轮询器。
type DevicePoller struct {
	cfg      config.DeviceConfig
	client   *modbusclient.Client
	reporter Reporter
	logger   *logrus.Logger
	// batches 是构造时按点表预计算好的批量读分组（地址连续/相邻的点位合并为一次
	// Modbus 事务），采集路径复用它，不在每轮 CollectOnce 里重新分组。
	batches []modbusclient.Batch
}

// New 创建单设备轮询器。
func New(cfg config.DeviceConfig, reporter Reporter, logger *logrus.Logger) *DevicePoller {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &DevicePoller{
		cfg:      cfg,
		client:   modbusclient.NewClient(cfg.Target),
		reporter: reporter,
		logger:   logger,
		batches:  modbusclient.PlanBatches(cfg.Registers, -1),
	}
}

// Run 启动轮询直到 ctx 取消。
func (p *DevicePoller) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.CollectOnce(ctx)
		}
	}
}

// CollectOnce 执行一轮采集与上报。
// 采集路径优先走构造时预计算的批量分组（p.batches）：地址连续或相邻的点位用一次
// Modbus 事务读回，解码后按点位切回各自的值，整轮事务数从 N（点位数）降到分组数。
// 单个批次读失败时该批次内全部点位降级为逐点读做兜底，不影响其他批次。
func (p *DevicePoller) CollectOnce(ctx context.Context) {
	values := make(map[string]any, len(p.cfg.Registers))
	for _, b := range p.batches {
		if err := ctx.Err(); err != nil {
			return
		}
		if errs := p.client.ReadBatches(ctx, []modbusclient.Batch{b}, values); len(errs) > 0 {
			p.logger.WithError(errs[0]).
				WithField("device", p.cfg.DeviceNumber).
				WithField("type", b.Type).
				WithField("start", b.Start).
				WithField("count", b.Count).
				Warn("modbus batch read failed; falling back to per-point read")
			p.fallbackReadPoints(ctx, b.Points, values)
		}
	}
	if len(values) == 0 {
		return
	}
	if err := p.reporter.PublishTelemetry(p.cfg.DeviceNumber, values); err != nil {
		p.logger.WithError(err).WithField("device", p.cfg.DeviceNumber).Warn("telemetry publish failed")
	}
}

// fallbackReadPoints 对一组点位逐点读取（批量事务失败时的兜底路径）。
// 单点失败只记录并跳过，不影响同批次其余点位或后续批次。
func (p *DevicePoller) fallbackReadPoints(ctx context.Context, points []*config.RegisterPoint, values map[string]any) {
	for _, r := range points {
		if err := ctx.Err(); err != nil {
			return
		}
		value, err := p.client.ReadPoint(ctx, r)
		if err != nil {
			p.logger.WithError(err).
				WithField("device", p.cfg.DeviceNumber).
				WithField("key", r.Key).
				Warn("modbus read failed; skip this point")
			continue
		}
		values[r.Key] = value
	}
}
