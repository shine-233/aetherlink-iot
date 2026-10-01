// 文件用途：部署健康报告的「下一步指引」聚合：公网地址 / MQTT 接入地址 / 首次启动 / 首台设备。
// 核心逻辑：buildDeploymentGuidance 并发收集各条指引；地址类指引解析 URL/host:port 并按
// 本机回环、占位主机名分级提示；首次启动与首台设备指引通过表白名单的 countTableRows 读行数。
// 关键注意事项：指引的 Key/Status/Message 文案是前端与部署文档依赖的契约，改动需同步
// deployment_capability_test.go；countTableRows 的表白名单防注入约束不可放宽。
package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"github.com/spf13/viper"
)

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

// deploymentHealthRowCountTables 是 countTableRows 允许统计行数的表白名单。
// 表名会拼接进 Raw SQL，必须收敛为硬编码集合，防止注入面扩大。
var deploymentHealthRowCountTables = map[string]struct{}{
	"users":          {},
	"device_configs": {},
	"devices":        {},
}

func countTableRows(table string) (int64, error) {
	if _, allowed := deploymentHealthRowCountTables[table]; !allowed {
		return 0, fmt.Errorf("deployment health 不允许统计表 %q 的行数：表名不在白名单内", table)
	}
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
