// 文件用途：提供设备自动测试工具的命令行入口。
// 核心逻辑：解析配置路径和测试模式，初始化日志，创建设备连接，订阅主题并触发上报流程。
// 关键注意事项：该入口只适合基础冒烟式发布，不替代 tests/direct 与 tests/gateway 的外部集成断言。
// 重构建议：可把各模式执行逻辑移入可测试 runner，并让 CLI 返回明确退出码与结构化结果。

/*
Purpose: 提供设备自动测试工具的命令行入口。
Core logic: 解析配置路径和测试模式，初始化日志，创建设备连接，订阅主题，并按模式触发遥测、属性或事件上报。
Important notes: 该入口只做基础冒烟式发布，不替代 tests/direct 与 tests/gateway 中依赖真实环境的集成断言。
Refactor suggestion: 可把各模式执行逻辑移入可测试的 runner 包，并让 CLI 返回明确退出码与结构化结果。
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
	"aetherlink-iot/aetherlink-device-autotest/internal/device"
	"aetherlink-iot/aetherlink-device-autotest/internal/utils"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	testMode := flag.String("mode", "telemetry", "Test mode: telemetry, attribute, event, all, command-emulator, ota-emulator, telemetry-json, telemetry-raw")
	commandSuccess := flag.Bool("command-success", true, "Command emulator response result (only used with -mode command-emulator)")
	commandReceiptPath := flag.String("command-receipt-path", "", "Path to write command emulator receipts")
	otaProgressPath := flag.String("ota-progress-path", "", "Path to write OTA emulator receipts")
	otaProgressValues := flag.String("ota-progress-values", "", "Comma-separated integer progress values for OTA emulator")
	otaVersion := flag.String("ota-version", "", "Target OTA version reported by OTA emulator")
	otaFailure := flag.Bool("ota-failure", false, "Whether OTA emulator reports terminal failure")
	telemetryPayload := flag.String("telemetry-payload", "", "JSON payload to publish in telemetry-json mode")
	telemetryRawPayload := flag.String("telemetry-raw-payload", "", "Raw payload string to publish in telemetry-raw mode")
	flag.Parse()

	// 初始化日志
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	logger.Info("Starting AetherLink Device Autotest",
		zap.String("mode", *testMode),
		zap.String("device_type", cfg.DeviceType),
		zap.String("device_id", cfg.Device.DeviceID))

	// 创建设备
	dev, err := device.NewDevice(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create device", zap.Error(err))
	}
	if err := dev.Connect(); err != nil {
		logger.Fatal("Failed to connect device", zap.Error(err))
	}
	defer dev.Disconnect()

	logger.Info("Device connected successfully")

	// 入口阶段先把下行主题全部订阅起来，后续不同模式都复用这一份连接状态。
	if *testMode == "command-emulator" {
		direct, ok := dev.(*device.DirectDevice)
		if !ok {
			logger.Fatal("command-emulator requires direct device mode")
		}
		signals := make(chan os.Signal, 1)
		stop := make(chan struct{})
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		go func() {
			<-signals
			close(stop)
		}()
		logger.Info("Command emulator is listening", zap.Bool("success", *commandSuccess), zap.String("receipt_path", *commandReceiptPath))
		if err := direct.RunCommandEmulator(stop, *commandSuccess, *commandReceiptPath); err != nil {
			logger.Fatal("Command emulator stopped with error", zap.Error(err))
		}
		return
	}

	if *testMode == "ota-emulator" {
		direct, ok := dev.(*device.DirectDevice)
		if !ok {
			logger.Fatal("ota-emulator requires direct device mode")
		}
		signals := make(chan os.Signal, 1)
		stop := make(chan struct{})
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		go func() {
			<-signals
			close(stop)
		}()
		logger.Info("OTA emulator is listening",
			zap.String("receipt_path", *otaProgressPath),
			zap.String("progress_values", *otaProgressValues),
			zap.String("version", *otaVersion),
			zap.Bool("failure", *otaFailure))
		if err := direct.RunOTAEmulator(stop, *otaProgressPath, *otaProgressValues, *otaVersion, *otaFailure); err != nil {
			logger.Fatal("OTA emulator stopped with error", zap.Error(err))
		}
		return
	}

	if err := dev.SubscribeAll(); err != nil {
		logger.Fatal("Failed to subscribe topics", zap.Error(err))
	}

	// 这里的 mode 只负责触发一次基础上报，用于确认链路可达，不承担 tests 目录里的完整断言。
	switch *testMode {
	case "telemetry":
		runTelemetryTest(dev, cfg, logger)
	case "telemetry-json":
		direct, ok := dev.(*device.DirectDevice)
		if !ok {
			logger.Fatal("telemetry-json requires direct device mode")
		}
		var data interface{}
		if err := json.Unmarshal([]byte(*telemetryPayload), &data); err != nil {
			logger.Fatal("Failed to parse telemetry-payload as JSON", zap.Error(err))
		}
		if err := direct.PublishTelemetry(data); err != nil {
			logger.Fatal("Failed to publish telemetry", zap.Error(err))
		}
	case "telemetry-raw":
		direct, ok := dev.(*device.DirectDevice)
		if !ok {
			logger.Fatal("telemetry-raw requires direct device mode")
		}
		if err := direct.PublishRaw(direct.Topics().Telemetry(), []byte(*telemetryRawPayload)); err != nil {
			logger.Fatal("Failed to publish raw telemetry", zap.Error(err))
		}
	case "attribute":
		runAttributeTest(dev, cfg, logger)
	case "event":
		runEventTest(dev, cfg, logger)
	case "all":
		runTelemetryTest(dev, cfg, logger)
		time.Sleep(2 * time.Second)
		runAttributeTest(dev, cfg, logger)
		time.Sleep(2 * time.Second)
		runEventTest(dev, cfg, logger)
	default:
		logger.Error("Unknown test mode", zap.String("mode", *testMode))
	}

	logger.Info("Test completed successfully")
}

func runTelemetryTest(dev device.Device, cfg *config.Config, logger *zap.Logger) {
	logger.Info("Running telemetry test...")

	// 内置测试数据偏向冒烟验证；稳定回放应接入显式 fixture loader，而不是依赖未使用的静态 fixture 文件。
	data := utils.BuildTelemetryData()
	if err := dev.PublishTelemetry(data); err != nil {
		logger.Error("Failed to publish telemetry", zap.Error(err))
		return
	}

	logger.Info("Telemetry data published", zap.Any("data", data))
}

func runAttributeTest(dev device.Device, cfg *config.Config, logger *zap.Logger) {
	logger.Info("Running attribute test...")

	// 平台侧属性上报与回执通常通过 message_id 关联，因此这里每次发布前重新生成。
	messageID := utils.GenerateMessageID()
	data := utils.BuildAttributeData()

	if err := dev.PublishAttribute(data, messageID); err != nil {
		logger.Error("Failed to publish attribute", zap.Error(err))
		return
	}

	logger.Info("Attribute data published",
		zap.String("message_id", messageID),
		zap.Any("data", data))
}

func runEventTest(dev device.Device, cfg *config.Config, logger *zap.Logger) {
	logger.Info("Running event test...")

	messageID := utils.GenerateMessageID()
	method := "TestEvent"
	// 事件的最终 MQTT payload 结构由具体设备 builder 决定，CLI 入口只传递语义化输入。
	params := map[string]interface{}{
		"test_key": "test_value",
		"count":    1,
	}

	if err := dev.PublishEvent(method, params, messageID); err != nil {
		logger.Error("Failed to publish event", zap.Error(err))
		return
	}

	logger.Info("Event published",
		zap.String("message_id", messageID),
		zap.String("method", method),
		zap.Any("params", params))
}
