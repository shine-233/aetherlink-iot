//go:build external_integration

// 文件用途：为直连设备外部集成测试提供统一入口和环境门禁。
// 核心逻辑：只有 AUTOTEST_EXTERNAL=1 时才加载直连配置并探测数据库，否则测试用例会主动跳过。
// 关键注意事项：这些测试依赖真实 API、MQTT broker 和 PostgreSQL，不应被当作普通 go test 默认套件。
// 重构建议：可把环境探测结果结构化输出，并允许通过环境变量覆盖配置路径。

/*
Purpose: 为直连设备外部集成测试提供统一入口和环境门禁。
Core logic: 只有 AUTOTEST_EXTERNAL=1 时才加载直连配置并探测数据库，否则测试用例会通过 helper 主动跳过。
Important notes: 这些测试依赖真实 API、MQTT broker 和 PostgreSQL，不应被当作普通 go test 默认套件。
Refactor suggestion: 可把环境探测结果结构化输出，并允许通过环境变量覆盖配置路径。
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
	cfg, err := config.Load("../../config-community.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "direct integration test config failed: %v\n", err)
		os.Exit(1)
	}
	dbClient, err := platform.NewDBClient(&cfg.Database, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "direct integration test database unavailable: %v\n", err)
		os.Exit(1)
	}
	_ = dbClient.Close()

	os.Exit(m.Run())
}

func requireExternalIntegration(t *testing.T) {
	t.Helper()
	if !externalIntegrationEnabled {
		t.Skip("direct integration tests require AUTOTEST_EXTERNAL=1 and a reachable AetherLink IoT API, MQTT broker, and database")
	}
}
