// 文件用途：维护 plugin\aetherlink\devdebug.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package aetherlink

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"gopkg.in/redis.v5"
)

const (
	devDebugCfgKeyPrefix  = "tp:devdebug:cfg:"
	devDebugLogsKeyPrefix = "tp:devdebug:logs:"
	deviceDebugRedacted   = "[REDACTED]"
)

var devDebugNow = time.Now
var deviceDebugSensitiveFieldPattern = regexp.MustCompile(`(?i)((password|passwd|pwd|token|secret|api[_-]?key|apikey|authorization|voucher|private[_-]?key)\s*[:=]\s*)("[^"]*"|[^&,\s}]+)`)

type DeviceDebugConfig struct {
	Enabled         bool  `json:"enabled"`
	ExpireAt        int64 `json:"expire_at"`
	MaxItems        int   `json:"max_items"`
	PayloadMaxBytes int   `json:"payload_max_bytes"`
}

// DeviceDebugLogEntry is a protocol-agnostic device interaction log entry.
// Protocol-private fields should be placed into Meta.
type DeviceDebugLogEntry struct {
	Ts        string                 `json:"ts"`
	DeviceID  string                 `json:"device_id"`
	Protocol  string                 `json:"protocol,omitempty"` // mqtt|modbus|tcp|...
	Direction string                 `json:"direction"`          // up|down|na
	Action    string                 `json:"action"`             // connect|auth|publish|read|write|...
	Outcome   string                 `json:"outcome,omitempty"`  // ok|deny|error|drop
	Error     string                 `json:"error,omitempty"`    // error detail if any
	Payload   string                 `json:"payload,omitempty"`  // optional, may be truncated
	Meta      map[string]interface{} `json:"meta,omitempty"`     // protocol-private fields
}

func devDebugCfgKey(deviceID string) string {
	return devDebugCfgKeyPrefix + strings.TrimSpace(deviceID)
}

func devDebugLogsKey(deviceID string) string {
	return devDebugLogsKeyPrefix + strings.TrimSpace(deviceID)
}

func GetDeviceDebugConfig(deviceID string) (DeviceDebugConfig, bool, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DeviceDebugConfig{}, false, errors.New("empty device_id")
	}
	if redisCache == nil {
		return DeviceDebugConfig{}, false, errors.New("redis not initialized")
	}
	var cfg DeviceDebugConfig
	if err := GetRedisForJsondata(devDebugCfgKey(deviceID), &cfg); err != nil {
		if err == redis.Nil {
			return DeviceDebugConfig{}, false, nil
		}
		return DeviceDebugConfig{}, false, err
	}
	if !cfg.Enabled {
		return cfg, false, nil
	}
	if cfg.ExpireAt > 0 && devDebugNow().Unix() > cfg.ExpireAt {
		return cfg, false, nil
	}
	if cfg.MaxItems <= 0 {
		cfg.MaxItems = 1000
	}
	if cfg.PayloadMaxBytes < 0 {
		cfg.PayloadMaxBytes = 0
	}
	return cfg, true, nil
}

func loadDeviceDebugConfigForWrite(deviceID string) (string, DeviceDebugConfig, bool, error) {
	normalizedDeviceID := strings.TrimSpace(deviceID)
	cfg, enabled, err := GetDeviceDebugConfig(normalizedDeviceID)
	if err != nil || !enabled {
		return normalizedDeviceID, cfg, false, err
	}
	return normalizedDeviceID, cfg, true, nil
}

// WriteDeviceDebugLog appends a log entry if device debug is enabled.
// It is safe to call frequently; missing/expired config results in a no-op.
func WriteDeviceDebugLog(deviceID string, entry DeviceDebugLogEntry) (bool, error) {
	normalizedDeviceID, cfg, enabled, err := loadDeviceDebugConfigForWrite(deviceID)
	if err != nil || !enabled {
		return false, err
	}
	return writeDeviceDebugLogWithConfig(normalizedDeviceID, cfg, entry)
}

func WriteDeviceDebugLogWithPayloadBytes(deviceID string, entry DeviceDebugLogEntry, payload []byte) (bool, error) {
	normalizedDeviceID, cfg, enabled, err := loadDeviceDebugConfigForWrite(deviceID)
	if err != nil || !enabled {
		return false, err
	}
	entry.Payload = string(payload)
	return writeDeviceDebugLogWithConfig(normalizedDeviceID, cfg, entry)
}

func writeDeviceDebugLogWithConfig(normalizedDeviceID string, cfg DeviceDebugConfig, entry DeviceDebugLogEntry) (bool, error) {
	if redisCache == nil {
		return false, errors.New("redis not initialized")
	}
	entry.DeviceID = normalizedDeviceID
	if entry.Ts == "" {
		entry.Ts = devDebugNow().Format(time.RFC3339Nano)
	}
	entry.Payload = sanitizeDeviceDebugPayload(entry.Payload)

	if cfg.PayloadMaxBytes <= 0 {
		entry.Payload = ""
	} else if len(entry.Payload) > cfg.PayloadMaxBytes {
		entry.Payload = entry.Payload[:cfg.PayloadMaxBytes]
		if entry.Meta == nil {
			entry.Meta = map[string]interface{}{}
		}
		entry.Meta["payload_truncated"] = true
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		return false, err
	}

	logsKey := devDebugLogsKey(normalizedDeviceID)
	pipe := redisCache.Pipeline()
	pipe.LPush(logsKey, raw)
	pipe.LTrim(logsKey, 0, int64(cfg.MaxItems-1))
	if cfg.ExpireAt > 0 {
		ttlSeconds := (cfg.ExpireAt - devDebugNow().Unix()) + 10*60
		if ttlSeconds > 0 {
			pipe.Expire(logsKey, time.Duration(ttlSeconds)*time.Second)
		}
	}
	_, err = pipe.Exec()
	if err == redis.Nil {
		return false, nil
	}
	return err == nil, err
}

func sanitizeDeviceDebugPayload(payload string) string {
	if payload == "" {
		return ""
	}
	if sanitized, ok := sanitizeDeviceDebugJSONPayload(payload); ok {
		return sanitized
	}
	return deviceDebugSensitiveFieldPattern.ReplaceAllString(payload, `${1}`+deviceDebugRedacted)
}

func sanitizeDeviceDebugJSONPayload(payload string) (string, bool) {
	var decoded interface{}
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return "", false
	}
	redactDeviceDebugJSONValue(decoded)
	sanitized, err := json.Marshal(decoded)
	if err != nil {
		return "", false
	}
	return string(sanitized), true
}

func redactDeviceDebugJSONValue(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if isSensitiveDeviceDebugKey(key) {
				typed[key] = deviceDebugRedacted
				continue
			}
			redactDeviceDebugJSONValue(child)
		}
	case []interface{}:
		for _, child := range typed {
			redactDeviceDebugJSONValue(child)
		}
	}
}

func isSensitiveDeviceDebugKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	return strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "passwd") ||
		normalized == "pwd" ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "voucher") ||
		strings.Contains(normalized, "privatekey")
}
