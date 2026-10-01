// 文件用途：部署健康检查的对外入口与报告缓存编排（报告结构、单飞缓存、检查项并发调度）。
// 核心逻辑：RunDeploymentHealthCheck 以 5 秒 TTL 缓存整份报告，进行中的请求单飞等待，
// 探针执行见 deployment_health_probes.go，指引聚合见 deployment_health_guidance.go，
// 能力位映射见 deployment_capabilities_runtime.go。
// 关键注意事项：SetMQTTHealthProbe 注册探针即清缓存；被拒绝的检查也必须出现在报告里；
// serverModeEnabled 的运行时门禁兜底 doctor，防止 loopback-only 部署被宣传为就绪。
package service

import (
	"os"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"github.com/spf13/viper"
)

type DeploymentHealthCheck struct {
	OK         bool   `json:"ok"`
	LatencyMS  int64  `json:"latency_ms"`
	Required   bool   `json:"required"`
	Detail     string `json:"detail,omitempty"`
	NextAction string `json:"next_action,omitempty"`
	Error      string `json:"error,omitempty"`
}

type DeploymentHealthGuidance struct {
	Key        string `json:"key"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	NextAction string `json:"next_action,omitempty"`
}

type DeploymentHealthReport struct {
	Service      string                           `json:"service"`
	Status       string                           `json:"status"`
	Version      string                           `json:"version"`
	Timestamp    string                           `json:"timestamp"`
	Checks       map[string]DeploymentHealthCheck `json:"checks"`
	Guidance     []DeploymentHealthGuidance       `json:"guidance"`
	Capabilities []DeploymentCapability           `json:"capabilities"`
}

const deploymentHealthCacheTTL = 5 * time.Second

var deploymentHealthCache struct {
	sync.Mutex
	report    DeploymentHealthReport
	expiresAt time.Time
	loading   bool
	waitCh    chan struct{}
}

func RunDeploymentHealthCheck() DeploymentHealthReport {
	now := time.Now()
	deploymentHealthCache.Lock()
	if !deploymentHealthCache.expiresAt.IsZero() && now.Before(deploymentHealthCache.expiresAt) {
		report := cloneDeploymentHealthReport(deploymentHealthCache.report)
		deploymentHealthCache.Unlock()
		return report
	}
	if deploymentHealthCache.loading {
		waitCh := deploymentHealthCache.waitCh
		deploymentHealthCache.Unlock()
		<-waitCh

		deploymentHealthCache.Lock()
		report := cloneDeploymentHealthReport(deploymentHealthCache.report)
		deploymentHealthCache.Unlock()
		if report.Service != "" {
			return report
		}
		return runDeploymentHealthCheckUncached()
	}

	waitCh := make(chan struct{})
	deploymentHealthCache.loading = true
	deploymentHealthCache.waitCh = waitCh
	deploymentHealthCache.Unlock()

	report := runDeploymentHealthCheckUncached()

	deploymentHealthCache.Lock()
	deploymentHealthCache.report = cloneDeploymentHealthReport(report)
	deploymentHealthCache.expiresAt = time.Now().Add(deploymentHealthCacheTTL)
	deploymentHealthCache.loading = false
	close(waitCh)
	deploymentHealthCache.waitCh = nil
	deploymentHealthCache.Unlock()

	return report
}

func clearDeploymentHealthCache() {
	deploymentHealthCache.Lock()
	defer deploymentHealthCache.Unlock()
	deploymentHealthCache.report = DeploymentHealthReport{}
	deploymentHealthCache.expiresAt = time.Time{}
}

func cloneDeploymentHealthReport(report DeploymentHealthReport) DeploymentHealthReport {
	if report.Checks != nil {
		checks := make(map[string]DeploymentHealthCheck, len(report.Checks))
		for key, value := range report.Checks {
			checks[key] = value
		}
		report.Checks = checks
	}
	report.Guidance = append([]DeploymentHealthGuidance(nil), report.Guidance...)
	report.Capabilities = append([]DeploymentCapability(nil), report.Capabilities...)
	return report
}

func deploymentHealthStatus(checks map[string]DeploymentHealthCheck) string {
	for _, check := range checks {
		if check.Required && !check.OK {
			return "down"
		}
	}
	return "ok"
}

func serverModeEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AETHERLINK_SERVER_MODE"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func serverAddressHealthCheck(guidance DeploymentHealthGuidance) DeploymentHealthCheck {
	ok := guidance.Status == "ok"
	check := DeploymentHealthCheck{
		OK:         ok,
		Required:   true,
		Detail:     guidance.Message,
		NextAction: guidance.NextAction,
	}
	if !ok {
		check.Error = guidance.Message
	}
	return check
}

func addServerAddressHealthChecks(checks map[string]DeploymentHealthCheck, publicURL, mqttAddress string) {
	checks["public_url"] = serverAddressHealthCheck(publicURLGuidance(publicURL))
	checks["mqtt_access_address"] = serverAddressHealthCheck(mqttAccessGuidance(mqttAddress))
}

func runDeploymentHealthCheckUncached() DeploymentHealthReport {
	checkers := map[string]func() DeploymentHealthCheck{
		"database":      checkDatabase,
		"redis":         checkRedis,
		"mqtt":          checkMQTT,
		"file_storage":  checkFileStorage,
		"db_migrations": checkDBMigrations,
	}

	if global.STATUS_REDIS != nil && global.STATUS_REDIS != global.REDIS {
		checkers["status_redis"] = checkStatusRedis
	}
	checks := runDeploymentHealthChecks(checkers)
	if serverModeEnabled() {
		// Doctor catches this before normal startup, but keep the runtime gate too:
		// direct `docker compose up` must not advertise a loopback-only deployment
		// as ready for remote users or devices.
		addServerAddressHealthChecks(checks, configuredPublicURL(), viper.GetString("mqtt.access_address"))
	}

	status := deploymentHealthStatus(checks)

	return DeploymentHealthReport{
		Service:      "aetherlink-iot-backend",
		Status:       status,
		Version:      global.SYSTEM_VERSION,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Checks:       checks,
		Guidance:     buildDeploymentGuidance(),
		Capabilities: buildRuntimeDeploymentCapabilities(collectRuntimeDeploymentCapabilityState(), checks),
	}
}

func runDeploymentHealthChecks(checkers map[string]func() DeploymentHealthCheck) map[string]DeploymentHealthCheck {
	checks := make(map[string]DeploymentHealthCheck, len(checkers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for key, checker := range checkers {
		key, checker := key, checker
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := checker()
			mu.Lock()
			checks[key] = result
			mu.Unlock()
		}()
	}

	wg.Wait()
	return checks
}
