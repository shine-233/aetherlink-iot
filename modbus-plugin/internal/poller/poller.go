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
func (p *DevicePoller) CollectOnce(ctx context.Context) {
	values := make(map[string]any, len(p.cfg.Registers))
	for i := range p.cfg.Registers {
		if err := ctx.Err(); err != nil {
			return
		}
		r := &p.cfg.Registers[i]
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
	if len(values) == 0 {
		return
	}
	if err := p.reporter.PublishTelemetry(p.cfg.DeviceNumber, values); err != nil {
		p.logger.WithError(err).WithField("device", p.cfg.DeviceNumber).Warn("telemetry publish failed")
	}
}
