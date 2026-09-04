package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata"

	"aetherlink-iot/backend/internal/app"

	"github.com/sirupsen/logrus"
)

// @title           AetherLink IoT API
// @version         1.0
// @description     AetherLink IoT API.
// @schemes         http
// @host            localhost:9999
// @BasePath
// @securityDefinitions.apikey  ApiKeyAuth
// @in                          header
// @name                        x-token
// @securityDefinitions.apikey  PluginApiKeyAuth
// @in                          header
// @name                        X-API-Key
func main() {
	configPath := flag.String("config", "", "config file path")
	flag.Parse()

	configOption := app.WithProductionConfig()
	if *configPath != "" {
		configOption = app.WithConfigFile(*configPath)
	}

	application, err := app.NewApplication(
		configOption,
		app.WithOptionalRsaDecrypt("./configs/rsa_key/private_key.pem"),
		app.WithLogger(),
		app.WithDatabase(),
		app.WithRedis(),

		app.WithStorageService(),
		app.WithFlowService(),
		app.WithHeartbeatMonitor(),
		app.WithDiagnostics(),
		app.WithMQTTService(),
		app.WithDownlinkService(),
		app.WithGRPCService(),
		app.WithHTTPService(),
		app.WithCronService(),
		app.WithMQTTSessionRevocationOutboxWorker(),
		app.WithTelemetryDeadLetterWorker(),
		app.WithTelemetry(),
		app.WithCoAPGateway(), // C6：CoAP/LwM2M 协议网关（protocols.coap.enabled=true 时启动）
		app.WithCollectors(), // C6：SNMP/OPC UA 轮询采集器（collectors.*.enabled=true 时启动）
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "application initialization failed: %v\n", err)
		os.Exit(1)
	}

	if err := application.Start(); err != nil {
		logrus.Fatalf("failed to start services: %v", err)
	}
	defer application.Shutdown()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logrus.Infof("received signal %v; starting graceful shutdown", sig)
		application.Shutdown()
	}()

	application.Wait()
}
