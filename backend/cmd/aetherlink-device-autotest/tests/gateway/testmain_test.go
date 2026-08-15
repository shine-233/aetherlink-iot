//go:build external_integration

// 文件用途：为网关设备外部集成测试提供统一入口和环境门禁。
// 核心逻辑：只有 AUTOTEST_EXTERNAL=1 时才加载网关配置并探测数据库，否则测试用例会主动跳过。
// 关键注意事项：网关测试依赖真实 API、MQTT broker、PostgreSQL 和多层设备配置，不能作为默认单元测试运行。
// 重构建议：可把直连和网关 TestMain 的重复环境检查抽成共享测试辅助包。

/*
Purpose: 为网关设备外部集成测试提供统一入口和环境门禁。
Core logic: 只有 AUTOTEST_EXTERNAL=1 时才加载网关配置并探测数据库，否则测试用例会通过 helper 主动跳过。
Important notes: 网关测试依赖真实 API、MQTT broker、PostgreSQL 和多层设备配置，不能作为默认单元测试运行。
Refactor suggestion: 可把直连和网关 TestMain 的重复环境检查抽成共享测试辅助包。
*/
package tests

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"

	"aetherlink-iot/aetherlink-device-autotest/internal/config"
	"aetherlink-iot/aetherlink-device-autotest/internal/platform"
)

var externalIntegrationEnabled bool

func TestMain(m *testing.M) {
	externalIntegrationEnabled = os.Getenv("AUTOTEST_EXTERNAL") == "1"
	if !externalIntegrationEnabled {
		os.Exit(m.Run())
	}

	logger := zap.NewNop()
	cfg, err := config.Load("../../config-gateway-community.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gateway integration test config failed: %v\n", err)
		os.Exit(1)
	}
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gateway integration test database unavailable: %v\n", err)
		os.Exit(1)
	}
	_ = dbClient.Close()

	os.Exit(m.Run())
}

func requireExternalIntegration(t *testing.T) {
	t.Helper()
	if !externalIntegrationEnabled {
		t.Skip("gateway integration tests require AUTOTEST_EXTERNAL=1 and a reachable AetherLink IoT API, MQTT broker, and database")
	}
}
