// 文件用途：部署健康报告的能力位（DeploymentCapability）运行时状态采集与映射。
// 核心逻辑：collectRuntimeDeploymentCapabilityState 只读 viper/global 的当前配置，
// buildRuntimeDeploymentCapabilities 把「启用/已配置/健康」三元组映射到能力目录 ID。
// 关键注意事项：可选集成的启用判断一律 fail-closed（viper 未设置视为关闭）；已配置但无
// 运行时探针的外部服务（thingsvis/http-adapter/market/external-telemetry-store）健康位
// 恒为 false，不因拆分文件而放宽。
package service

import (
	"strings"

	"aetherlink-iot/backend/pkg/global"

	"github.com/spf13/viper"
)

type runtimeDeploymentCapabilityState struct {
	postgresConfigured               bool
	redisConfigured                  bool
	mqttEnabled                      bool
	mqttConfigured                   bool
	marketEnabled                    bool
	marketConfigured                 bool
	thingsVisEnabled                 bool
	thingsVisConfigured              bool
	httpAdapterEnabled               bool
	httpAdapterConfigured            bool
	externalTelemetryStoreEnabled    bool
	externalTelemetryStoreConfigured bool
	usageTelemetryEnabled            bool
	usageTelemetryConfigured         bool
}

// deploymentConfigEnabled keeps optional runtime integrations fail-closed.
func deploymentConfigEnabled(key string) bool {
	return viper.IsSet(key) && viper.GetBool(key)
}

// marketConfigEnabled keeps the optional external Market integration fail-closed.
func marketConfigEnabled() bool {
	return viper.IsSet("market.enabled") && viper.GetBool("market.enabled")
}

func optionalIntegrationState(key string) (enabled, configured bool) {
	enabled = viper.GetBool("integrations." + key + ".enabled")
	configured = enabled && viper.GetBool("integrations."+key+".configured")
	return enabled, configured
}

func externalTelemetryStoreState() (enabled, configured bool) {
	switch strings.ToUpper(strings.TrimSpace(viper.GetString("grpc.tptodb_type"))) {
	case "TSDB", "KINGBASE", "POLARDB":
		enabled = true
	}
	configured = enabled && strings.TrimSpace(viper.GetString("grpc.tptodb_server")) != ""
	return enabled, configured
}

func collectRuntimeDeploymentCapabilityState() runtimeDeploymentCapabilityState {
	mqttEnabled := deploymentConfigEnabled("mqtt.enabled")
	marketEnabled := marketConfigEnabled()
	thingsVisEnabled, thingsVisConfigured := optionalIntegrationState("thingsvis")
	httpAdapterEnabled, httpAdapterConfigured := optionalIntegrationState("http_adapter")
	externalTelemetryStoreEnabled, externalTelemetryStoreConfigured := externalTelemetryStoreState()
	return runtimeDeploymentCapabilityState{
		postgresConfigured:               global.DB != nil,
		redisConfigured:                  global.REDIS != nil,
		mqttEnabled:                      mqttEnabled,
		mqttConfigured:                   mqttEnabled && strings.TrimSpace(viper.GetString("mqtt.broker")) != "" && mqttHealthProbeInstalled(),
		marketEnabled:                    marketEnabled,
		marketConfigured:                 marketEnabled && isConfiguredMarketBaseURL(viper.GetString("market.base_url")),
		thingsVisEnabled:                 thingsVisEnabled,
		thingsVisConfigured:              thingsVisConfigured,
		httpAdapterEnabled:               httpAdapterEnabled,
		httpAdapterConfigured:            httpAdapterConfigured,
		externalTelemetryStoreEnabled:    externalTelemetryStoreEnabled,
		externalTelemetryStoreConfigured: externalTelemetryStoreConfigured,
	}
}

func buildRuntimeDeploymentCapabilities(state runtimeDeploymentCapabilityState, checks map[string]DeploymentHealthCheck) []DeploymentCapability {
	checkOK := func(key string) bool {
		check, exists := checks[key]
		return exists && check.OK
	}
	redisHealthy := state.redisConfigured && checkOK("redis")
	if _, exists := checks["status_redis"]; exists {
		redisHealthy = redisHealthy && checkOK("status_redis")
	}

	return BuildDeploymentCapabilities(map[string]DeploymentCapabilityState{
		"postgres": {
			Enabled: true, Configured: state.postgresConfigured,
			Healthy: state.postgresConfigured && checkOK("database") && checkOK("db_migrations"),
		},
		"redis": {
			Enabled: true, Configured: state.redisConfigured, Healthy: redisHealthy,
		},
		"mqtt-broker": {
			Enabled: state.mqttEnabled, Configured: state.mqttConfigured,
			Healthy: state.mqttConfigured && checkOK("mqtt"),
		},
		"native-visualization": {Enabled: true, Configured: true, Healthy: true},
		// Optional image readiness is verified by Compose. Until the backend gains an
		// authenticated runtime probe, configured external services remain explicitly blocked.
		"thingsvis": {
			Enabled: state.thingsVisEnabled, Configured: state.thingsVisConfigured, Healthy: false,
		},
		"http-adapter": {
			Enabled: state.httpAdapterEnabled, Configured: state.httpAdapterConfigured, Healthy: false,
		},
		"market": {
			Enabled: state.marketEnabled, Configured: state.marketConfigured, Healthy: false,
		},
		"smtp":         {Enabled: false, Configured: false, Healthy: false},
		"map-provider": {Enabled: false, Configured: false, Healthy: false},
		// The external telemetry client preserves the legacy gRPC contract. Until
		// an authenticated probe exists, a configured runtime remains blocked.
		"external-telemetry-store": {
			Enabled:    state.externalTelemetryStoreEnabled,
			Configured: state.externalTelemetryStoreConfigured,
			Healthy:    false,
		},
	})
}
