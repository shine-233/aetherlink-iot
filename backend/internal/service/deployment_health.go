package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"github.com/spf13/viper"
	"gorm.io/gorm"
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

var mqttHealthProbe struct {
	sync.RWMutex
	fn func() bool
}

const deploymentHealthCacheTTL = 5 * time.Second

var deploymentHealthCache struct {
	sync.Mutex
	report    DeploymentHealthReport
	expiresAt time.Time
	loading   bool
	waitCh    chan struct{}
}

func SetMQTTHealthProbe(fn func() bool) {
	mqttHealthProbe.Lock()
	defer mqttHealthProbe.Unlock()
	mqttHealthProbe.fn = fn
	clearDeploymentHealthCache()
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

func mqttHealthProbeInstalled() bool {
	mqttHealthProbe.RLock()
	defer mqttHealthProbe.RUnlock()
	return mqttHealthProbe.fn != nil
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

func checkDatabase() DeploymentHealthCheck {
	if global.DB == nil {
		return requiredHealthError("数据库连接还没有初始化", "启动 Postgres 和后端服务后，重新执行部署检查。")
	}

	sqlDB, err := global.DB.DB()
	if err != nil {
		return requiredHealthError(err.Error(), "检查后端数据库配置和 Postgres 日志。")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	started := time.Now()
	err = sqlDB.PingContext(ctx)
	return healthResult(started, err)
}

func checkRedis() DeploymentHealthCheck {
	if global.REDIS == nil {
		return requiredHealthError("Redis 客户端还没有初始化", "启动 Redis 和后端服务后，重新执行部署检查。")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	started := time.Now()
	_, err := global.REDIS.Ping(ctx).Result()
	return healthResult(started, err)
}

func checkStatusRedis() DeploymentHealthCheck {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	started := time.Now()
	_, err := global.STATUS_REDIS.Ping(ctx).Result()
	return healthResult(started, err)
}

func checkMQTT() DeploymentHealthCheck {
	started := time.Now()
	if !deploymentConfigEnabled("mqtt.enabled") {
		return DeploymentHealthCheck{
			OK:        true,
			LatencyMS: time.Since(started).Milliseconds(),
			Required:  false,
			Detail:    "MQTT disabled by configuration",
		}
	}

	mqttHealthProbe.RLock()
	fn := mqttHealthProbe.fn
	mqttHealthProbe.RUnlock()

	if fn == nil {
		return requiredHealthError("MQTT 健康探针还没有初始化", "启动 MQTT 适配客户端，并确认后端 MQTT 配置。")
	}

	if !fn() {
		return DeploymentHealthCheck{
			OK:         false,
			LatencyMS:  time.Since(started).Milliseconds(),
			Required:   true,
			Error:      "MQTT 适配客户端尚未连接",
			NextAction: "检查 mqtt-broker 日志，以及后端 mqtt.broker / mqtt.access_address 配置。",
		}
	}

	return healthResult(started, nil)
}

func checkFileStorage() DeploymentHealthCheck {
	started := time.Now()
	baseDir := "./files"
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return healthResultWithAction(started, err, "确认后端文件卷已挂载，并且后端容器有写入权限。")
	}

	file, err := os.CreateTemp(baseDir, ".deployment-health-*")
	if err != nil {
		return healthResultWithAction(started, err, "确认后端文件卷可写；OTA 包和导出文件需要使用这个路径。")
	}

	tempName := file.Name()
	_, writeErr := file.WriteString("ok")
	closeErr := file.Close()
	removeErr := os.Remove(tempName)

	if writeErr != nil {
		return healthResultWithAction(started, writeErr, "确认后端文件卷可写；OTA 包和导出文件需要使用这个路径。")
	}
	if closeErr != nil {
		return healthResultWithAction(started, closeErr, "检查后端文件卷的文件系统错误。")
	}
	if removeErr != nil {
		return healthResultWithAction(started, removeErr, "后端可以写入文件，但无法清理临时文件；请检查卷权限。")
	}

	result := healthResult(started, nil)
	if absDir, err := filepath.Abs(baseDir); err == nil {
		result.Detail = fmt.Sprintf("可写路径：%s", absDir)
	}
	return result
}

func checkDBMigrations() DeploymentHealthCheck {
	started := time.Now()
	if global.DB == nil {
		return healthResultWithAction(started, errors.New("数据库连接还没有初始化"), "确认 Postgres 可访问后执行数据库迁移。")
	}

	var row struct {
		VersionNumber int32  `gorm:"column:version_number"`
		Version       string `gorm:"column:version"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := global.DB.WithContext(ctx).
		Raw("SELECT version_number, version FROM sys_version ORDER BY version_number DESC LIMIT 1").
		Scan(&row).Error
	if err != nil {
		return healthResultWithAction(started, err, "执行 deploy/postgres 迁移，并检查 backend/sql 文件是否已挂载到 Postgres 初始化目录。")
	}
	if row.VersionNumber == 0 && row.Version == "" {
		return healthResultWithAction(started, gorm.ErrRecordNotFound, "sys_version 表为空；请重新执行迁移后再确认本次部署。")
	}
	if row.VersionNumber != int32(global.VERSION_NUMBER) {
		return healthResultWithAction(
			started,
			fmt.Errorf("database migration version %d does not match expected version %d", row.VersionNumber, global.VERSION_NUMBER),
			fmt.Sprintf("执行 backend/sql 迁移到版本 %d 后再确认本次部署。", global.VERSION_NUMBER),
		)
	}

	result := healthResult(started, nil)
	result.Detail = fmt.Sprintf("最新迁移版本：%d %s", row.VersionNumber, row.Version)
	return result
}

func healthResult(started time.Time, err error) DeploymentHealthCheck {
	result := DeploymentHealthCheck{
		OK:        err == nil,
		LatencyMS: time.Since(started).Milliseconds(),
		Required:  true,
	}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func healthResultWithAction(started time.Time, err error, nextAction string) DeploymentHealthCheck {
	result := healthResult(started, err)
	result.NextAction = nextAction
	return result
}

func requiredHealthError(message string, nextAction string) DeploymentHealthCheck {
	return DeploymentHealthCheck{
		OK:         false,
		Required:   true,
		Error:      message,
		NextAction: nextAction,
	}
}

func buildDeploymentGuidance() []DeploymentHealthGuidance {
	builders := []func() DeploymentHealthGuidance{
		func() DeploymentHealthGuidance { return publicURLGuidance(configuredPublicURL()) },
		func() DeploymentHealthGuidance { return mqttAccessGuidance(viper.GetString("mqtt.access_address")) },
		firstStartGuidance,
		firstDeviceGuidance,
	}

	guidance := make([]DeploymentHealthGuidance, len(builders))
	var wg sync.WaitGroup
	for index, builder := range builders {
		index, builder := index, builder
		wg.Add(1)
		go func() {
			defer wg.Done()
			guidance[index] = builder()
		}()
	}
	wg.Wait()
	return guidance
}

func configuredPublicURL() string {
	if value := strings.TrimSpace(global.OtaAddress); value != "" {
		return value
	}
	return viper.GetString("ota.download_address")
}

func publicURLGuidance(rawURL string) DeploymentHealthGuidance {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return DeploymentHealthGuidance{
			Key:        "public_url",
			Status:     "action",
			Message:    "还没有配置公网访问地址，OTA 链接和复制给设备接入的地址可能不正确。",
			NextAction: "设置 AETHERLINK_PUBLIC_URL 和 GOTP_OTA_DOWNLOAD_ADDRESS，然后重启后端。",
		}
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return DeploymentHealthGuidance{
			Key:        "public_url",
			Status:     "action",
			Message:    fmt.Sprintf("公网访问地址 %q 不是可用的 http(s) URL。", value),
			NextAction: "请填写完整浏览器地址，例如 http://192.168.1.10:8080。",
		}
	}

	if isLocalHost(parsed.Hostname()) {
		return DeploymentHealthGuidance{
			Key:        "public_url",
			Status:     "review",
			Message:    fmt.Sprintf("公网访问地址是 %s；本机测试可用，但远程用户不能打开 localhost。", value),
			NextAction: "服务器部署时，请把 AETHERLINK_PUBLIC_URL 设置为服务器 IP 或域名。",
		}
	}
	if isPlaceholderHost(parsed.Hostname()) {
		return DeploymentHealthGuidance{
			Key:        "public_url",
			Status:     "action",
			Message:    fmt.Sprintf("公网访问地址 %q 仍是占位地址。", value),
			NextAction: "服务器部署时，请将 AETHERLINK_PUBLIC_URL 设置为用户可以访问的服务器 IP 或域名。",
		}
	}

	return DeploymentHealthGuidance{
		Key:     "public_url",
		Status:  "ok",
		Message: fmt.Sprintf("公网访问地址已配置为 %s。", value),
	}
}

func mqttAccessGuidance(rawAddress string) DeploymentHealthGuidance {
	value := strings.TrimSpace(rawAddress)
	host, port, err := net.SplitHostPort(value)
	if value == "" || err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return DeploymentHealthGuidance{
			Key:        "mqtt_access_address",
			Status:     "action",
			Message:    fmt.Sprintf("MQTT 接入地址 %q 不是 host:port 格式。", value),
			NextAction: "设置 AETHERLINK_MQTT_ACCESS_ADDRESS 和 GOTP_MQTT_ACCESS_ADDRESS，例如 192.168.1.10:1883。",
		}
	}
	if !isValidTCPPort(port) {
		return DeploymentHealthGuidance{
			Key:        "mqtt_access_address",
			Status:     "action",
			Message:    fmt.Sprintf("MQTT 接入地址 %q 的端口无效。", value),
			NextAction: "请使用 1 到 65535 之间的 MQTT 端口，例如 192.168.1.10:1883。",
		}
	}

	if isLocalHost(host) {
		return DeploymentHealthGuidance{
			Key:        "mqtt_access_address",
			Status:     "review",
			Message:    fmt.Sprintf("MQTT 接入地址是 %s；本机测试可用，但这台机器外的真实设备不能使用 localhost。", value),
			NextAction: "真实设备接入时，请使用服务器 IP 或域名以及已暴露的 MQTT 端口。",
		}
	}
	if isPlaceholderHost(host) {
		return DeploymentHealthGuidance{
			Key:        "mqtt_access_address",
			Status:     "action",
			Message:    fmt.Sprintf("MQTT 接入地址 %q 仍是占位地址。", value),
			NextAction: "真实设备接入时，请替换为设备可以访问的服务器 IP 或域名。",
		}
	}

	return DeploymentHealthGuidance{
		Key:     "mqtt_access_address",
		Status:  "ok",
		Message: fmt.Sprintf("MQTT 接入地址已配置为 %s。", value),
	}
}

func firstStartGuidance() DeploymentHealthGuidance {
	count, err := countTableRows("users")
	if err != nil {
		return DeploymentHealthGuidance{
			Key:        "first_start_admin",
			Status:     "action",
			Message:    fmt.Sprintf("无法读取用户表：%s。", err.Error()),
			NextAction: "先执行数据库迁移并确认数据库连通，再创建第一个管理员。",
		}
	}
	if count == 0 {
		return DeploymentHealthGuidance{
			Key:        "first_start_admin",
			Status:     "action",
			Message:    "还没有管理员或用户账号。",
			NextAction: "打开网页，完成首次启动的管理员初始化。",
		}
	}
	return DeploymentHealthGuidance{
		Key:     "first_start_admin",
		Status:  "ok",
		Message: fmt.Sprintf("已有 %d 个用户账号。", count),
	}
}

func firstDeviceGuidance() DeploymentHealthGuidance {
	configCount, deviceCount, configErr, deviceErr := countFirstDeviceRows()
	if configErr != nil || deviceErr != nil {
		return DeploymentHealthGuidance{
			Key:        "first_device_progress",
			Status:     "action",
			Message:    fmt.Sprintf("无法读取首次设备相关表：device_configs=%v devices=%v。", configErr, deviceErr),
			NextAction: "先执行数据库迁移，然后在首次接入页面创建产品和设备。",
		}
	}
	if configCount == 0 || deviceCount == 0 {
		return DeploymentHealthGuidance{
			Key:        "first_device_progress",
			Status:     "action",
			Message:    fmt.Sprintf("首次设备接入还未完成：产品/配置=%d，设备=%d。", configCount, deviceCount),
			NextAction: "打开首页首次接入流程，创建第一个产品/设备，并发送一条测试遥测。",
		}
	}
	return DeploymentHealthGuidance{
		Key:     "first_device_progress",
		Status:  "ok",
		Message: fmt.Sprintf("首次设备接入前置数据已存在：产品/配置=%d，设备=%d。", configCount, deviceCount),
	}
}

func countFirstDeviceRows() (configCount int64, deviceCount int64, configErr error, deviceErr error) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		configCount, configErr = countTableRows("device_configs")
	}()
	go func() {
		defer wg.Done()
		deviceCount, deviceErr = countTableRows("devices")
	}()
	wg.Wait()
	return configCount, deviceCount, configErr, deviceErr
}

func countTableRows(table string) (int64, error) {
	if global.DB == nil {
		return 0, errors.New("数据库连接还没有初始化")
	}
	var count int64
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := global.DB.WithContext(ctx).Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count).Error
	return count, err
}

func isLocalHost(host string) bool {
	normalized := strings.TrimSuffix(strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]"), ".")
	return normalized == "localhost" || normalized == "0.0.0.0" || normalized == "::" ||
		normalized == "::1" || strings.HasPrefix(normalized, "127.")
}

func isPlaceholderHost(host string) bool {
	normalized := strings.TrimSuffix(strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]"), ".")
	switch normalized {
	case "", "example.com", "example.net", "example.org", "your-ip", "your_ip", "your-domain", "your_domain", "change-me", "change_me", "placeholder", "todo":
		return true
	default:
		return false
	}
}

func isValidTCPPort(rawPort string) bool {
	port, err := strconv.Atoi(strings.TrimSpace(rawPort))
	return err == nil && port >= 1 && port <= 65535
}
