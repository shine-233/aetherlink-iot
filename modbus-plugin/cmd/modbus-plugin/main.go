// 文件用途：Modbus TCP 插件进程入口（ROADMAP B1）。
// 核心逻辑：加载点表配置 → 建立每设备 MQTT 上报通道与命令订阅 → 启动轮询循环 → 暴露 /healthz。
// 关键注意事项：本进程是独立部署的协议适配器，可运行在网关侧或与平台同栈；
//   配置中的设备凭证属于敏感信息，需以挂载文件/密钥管理方式下发，不要提交真实凭证。
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/config"
	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/modbusclient"
	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/platform"
	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/poller"
	"github.com/shine-233/aetherlink-iot/modbus-plugin/internal/reporter"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	configPath := os.Getenv("MODBUS_PLUGIN_CONFIG")
	if configPath == "" {
		configPath = "config.json"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 平台点表拉取：use_platform_profile 的设备以 x-api-key 拉取 target+registers，
	// 覆盖本地回退配置；失败保留本地值继续运行（前端界面保存的点表即此来源）。
	if cfg.Platform.BaseURL != "" && cfg.Platform.APIKey != "" {
		timeout := time.Duration(cfg.Platform.TimeoutMilli) * time.Millisecond
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		pc := platform.NewClient(cfg.Platform.BaseURL, cfg.Platform.APIKey, timeout)
		for i := range cfg.Devices {
			device := &cfg.Devices[i]
			if !device.UsePlatformProfile {
				continue
			}
			changed, err := pc.FetchProfile(ctx, device)
			if err != nil {
				logger.WithError(err).WithField("device", device.DeviceNumber).Warn("platform profile fetch failed; using local fallback")
				continue
			}
			logger.WithFields(logrus.Fields{"device": device.DeviceNumber, "changed": changed}).Info("platform profile loaded")
		}
	}

	var wg sync.WaitGroup
	for i := range cfg.Devices {
		device := cfg.Devices[i]
		rep, err := reporter.NewDeviceReporter(cfg.MQTT, device, logger)
		if err != nil {
			logger.WithError(err).WithField("device", device.DeviceNumber).Fatalf("reporter init failed")
		}
		client := modbusclient.NewClient(device.Target)
		if err := rep.SubscribeCommands(func(key string, value float64) error {
			register, ok := device.FindWritable(key)
			if !ok {
				return nil
			}
			return client.WritePoint(context.Background(), register, value)
		}); err != nil {
			logger.WithError(err).WithField("device", device.DeviceNumber).Warn("command subscribe failed; writes unavailable")
		}
		p := poller.New(device, rep, logger)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer rep.Close()
			p.Run(ctx, cfg.PollInterval())
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "devices": len(cfg.Devices)})
	})
	srv := &http.Server{Addr: cfg.HealthAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Warn("health server stopped")
		}
	}()

	logger.WithField("devices", len(cfg.Devices)).Info("modbus-plugin started")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		logger.Warn("shutdown timeout; some pollers still running")
	}
}
