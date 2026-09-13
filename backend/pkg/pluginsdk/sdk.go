// Package pluginsdk 是协议插件 SDK（ROADMAP P1.x → P2.1）的契约面：
// 插件作者实现 ProtocolAdapter 并携带 Manifest 向平台注册，
// 平台侧（PluginRegistryService）用本包做 manifest 校验与兼容性判定。
//
// 设计约束：
//   - 本包是**叶子包**，只依赖标准库——插件作者不必拉起整个服务端即可编译。
//     它与 internal/service 的 widget schema 校验器语义同源（JSON Schema 子集），
//     但刻意不共享代码：SDK 必须能独立分发。
//   - 契约_fail closed_：无法解析或校验不过的 manifest 一律拒绝注册，
//     与市场包验签（P1.6）同一口径。
package pluginsdk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// HostVersion 是本构建的插件 API 契约版本。
// manifest.min_host_version 高于它时拒绝注册——插件要求的能力宿主还不具备。
const HostVersion = "1.0.0"

// HealthStatus 插件健康状态。
type HealthStatus string

const (
	HealthOK      HealthStatus = "ok"
	HealthDegrade HealthStatus = "degraded"
	HealthDown    HealthStatus = "down"
)

// Health 健康探针结果。down/degraded 的 Message 应包含可归因的细节。
type Health struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}

// AdapterConfig 一次设备接入的配置：设备身份 + manifest.config_schema 约束的参数。
type AdapterConfig struct {
	DeviceID string         `json:"device_id"`
	Values   map[string]any `json:"values,omitempty"`
}

// Device Discover 发现的设备。
type Device struct {
	ID       string         `json:"id"`
	Name     string         `json:"name,omitempty"`
	Number   string         `json:"number,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TelemetryPoint 单个遥测点。
type TelemetryPoint struct {
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// Telemetry 一次读取的遥测集合。
type Telemetry struct {
	DeviceID string           `json:"device_id"`
	Points   []TelemetryPoint `json:"points"`
}

// Command 下行的命令。
type Command struct {
	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
	Payload  []byte `json:"payload,omitempty"`
}

// ProtocolAdapter 协议插件契约。
//
// 与路线图草图的两处偏差（刻意为之）：
//   - 所有方法携带 context.Context——适配器要做网络 IO，取消与超时是必须的；
//   - ReadTelemetry/WriteCommand 带设备标识——一个适配器进程通常服务多个设备。
type ProtocolAdapter interface {
	Name() string
	ValidateConfig(ctx context.Context, config AdapterConfig) error
	Connect(ctx context.Context, config AdapterConfig) error
	Discover(ctx context.Context) ([]Device, error)
	ReadTelemetry(ctx context.Context, deviceID string, keys []string) (Telemetry, error)
	WriteCommand(ctx context.Context, command Command) error
	Health(ctx context.Context) (Health, error)
	Close(ctx context.Context) error
}

// ErrHostTooOld manifest 要求的宿主版本高于当前构建。
var ErrHostTooOld = errors.New("pluginsdk: host version is older than manifest requires")

// CheckHostCompatibility 判断 manifest 声明的最低宿主版本能否被当前构建满足。
// minHostVersion 为空视为无要求；版本非点分数字视为不满足（fail closed）。
func CheckHostCompatibility(minHostVersion string) error {
	if minHostVersion == "" {
		return nil
	}
	host, ok := parseVersionSegments(HostVersion)
	if !ok {
		return fmt.Errorf("%w: host version %q is not dotted numeric", ErrHostTooOld, HostVersion)
	}
	minimum, ok := parseVersionSegments(minHostVersion)
	if !ok {
		return fmt.Errorf("%w: manifest min_host_version %q is not dotted numeric", ErrHostTooOld, minHostVersion)
	}
	if compareSegments(host, minimum) < 0 {
		return fmt.Errorf("%w: host %s < required %s", ErrHostTooOld, HostVersion, minHostVersion)
	}
	return nil
}

// parseVersionSegments 点分数字版本 → 各段数值（非数值段视为不可解析）。
func parseVersionSegments(version string) ([]int, bool) {
	raw := strings.Split(strings.TrimSpace(version), ".")
	segments := make([]int, 0, len(raw))
	for _, part := range raw {
		if part == "" {
			return nil, false
		}
		value := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil, false
			}
			value = value*10 + int(r-'0')
		}
		segments = append(segments, value)
	}
	return segments, len(segments) > 0
}

// compareSegments 逐段比较，长度不同按 0 补齐。
func compareSegments(left, right []int) int {
	size := len(left)
	if len(right) > size {
		size = len(right)
	}
	for i := 0; i < size; i++ {
		var l, r int
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	}
	return 0
}
