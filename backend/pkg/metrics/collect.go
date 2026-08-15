// 文件用途：构造实例遥测信息、持久化实例 ID 并发送遥测周期事件。
// 核心逻辑：读取或生成本地实例 ID，刷新运行时字段，再委托遥测周期逻辑组装并上报事件。
// 关键注意事项：遥测必须保持配置显式启用；实例 ID 文件路径会影响跨重启的 distinct_id 稳定性。
// 重构建议：后续可把文件存储和网络发送抽象出来，方便离线部署和隐私审计。
package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/global"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	EventInstanceRegistered = "instance_registered"
	EventInstanceHeartbeat  = "instance_heartbeat"
	EventInstanceUpgraded   = "instance_upgraded"

	TelemetryEdition       = "community"
	TelemetrySource        = "aetherlink-iot-backend"
	TelemetrySchemaVersion = "2026-04-10"
)

type InstanceInfo struct {
	InstanceID  string `json:"instance_id"`
	DeviceCount int64  `json:"device_count"`
	UserCount   int64  `json:"user_count"`
	Version     string `json:"version"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Timestamp   int64  `json:"timestamp"`
}

func NewInstance() *InstanceInfo {
	return &InstanceInfo{
		Version: global.VERSION,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}

// GetPersistentInstanceID 获取或生成持久化的实例ID
func GetPersistentInstanceID() string {
	idPath := getInstanceIDPath()

	// 1. 尝试从文件读取
	if data, err := os.ReadFile(idPath); err == nil {
		instanceID := strings.TrimSpace(string(data))
		if instanceID != "" {
			return instanceID
		}
	}

	// 2. 如果没有，生成一个
	instanceID := uuid.New()

	// 保存到文件
	_ = os.MkdirAll(filepath.Dir(idPath), 0755)
	if err := os.WriteFile(idPath, []byte(instanceID), 0644); err != nil {
		logrus.Errorf("Failed to save instance_id to file: %v", err)
	}

	return instanceID
}

// RefreshRuntimeInfo updates the instance fields that are produced at runtime.
func (ins *InstanceInfo) RefreshRuntimeInfo() {
	ins.Timestamp = time.Now().Unix()
	ins.InstanceID = GetPersistentInstanceID()
}

// SendToPostHog 发送数据到 PostHog
func (ins *InstanceInfo) SendToPostHog() {
	if err := ReportTelemetryCycle(ins, "telemetry_cycle"); err != nil {
		logrus.Debugf("Failed to send telemetry to PostHog: %v", err)
	}
}

func capturePayload(payload map[string]interface{}) error {
	apiKey := viper.GetString("telemetry.posthog_key")
	if apiKey == "" {
		return fmt.Errorf("posthog api key is not configured")
	}

	host := viper.GetString("telemetry.posthog_host")
	if host == "" {
		host = "https://us.i.posthog.com"
	}

	payload["api_key"] = apiKey

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/capture/", host)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("posthog returned non-OK status: %d", resp.StatusCode)
	}

	return nil
}

func getInstanceIDPath() string {
	if configured := strings.TrimSpace(viper.GetString("telemetry.instance_id_file")); configured != "" {
		return configured
	}
	return filepath.Join("configs", ".instance_id")
}
