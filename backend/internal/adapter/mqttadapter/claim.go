// File purpose: TB-12 设备端自主认领（MQTT 话题通道 v1/devices/me/claim 和 devices/claim）。
// Core logic: 接收已认证设备发出的自主认领载荷，提取 secretKey 与有效期并完成注册。
package mqttadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
)

// DeviceClaimMQTTPayload TB-12 设备端自主认领载荷定义（对标 ThingsBoard v1/devices/me/claim 规范）。
type DeviceClaimMQTTPayload struct {
	DeviceID    string `json:"device_id"`
	DeviceId    string `json:"deviceId"`
	SecretKey   string `json:"secretKey"`
	ClaimKey    string `json:"claimKey"`
	DurationMs  int64  `json:"durationMs"`
	TTLSeconds  int64  `json:"ttlSeconds"`
	TTLSeconds2 int64  `json:"ttl_seconds"`
}

// HandleDeviceClaimMessage 处理设备自主认领上报消息。
func (a *Adapter) HandleDeviceClaimMessage(payload []byte, topic string) error {
	var targetDeviceID string
	var rawValues []byte

	// 1. 尝试解包标准 broker 封装信封 {"device_id": "...", "values": ...}
	if envelope, err := a.verifyPayload(payload); err == nil {
		targetDeviceID = envelope.DeviceId
		rawValues = envelope.Values
	} else {
		// 备选路径：直接 JSON 载荷（测试/stub 环境）
		rawValues = payload
	}

	var req DeviceClaimMQTTPayload
	if err := json.Unmarshal(rawValues, &req); err != nil {
		a.logger.WithFields(logrus.Fields{
			"topic": topic,
			"error": err,
		}).Error("Invalid device claim JSON payload")
		return fmt.Errorf("invalid device claim JSON: %w", err)
	}

	// 优先以信封认证的 device_id 为准；若无信封则取 payload 内声明的 device_id/deviceId
	if targetDeviceID == "" {
		if req.DeviceID != "" {
			targetDeviceID = req.DeviceID
		} else {
			targetDeviceID = req.DeviceId
		}
	}

	if targetDeviceID == "" {
		a.logger.WithField("topic", topic).Error("device_id missing in claim message")
		return errors.New("device_id missing in claim message")
	}

	// 提取 secretKey（兼容 ThingsBoard secretKey 与 AetherLink claimKey）
	secretKey := strings.TrimSpace(req.SecretKey)
	if secretKey == "" {
		secretKey = strings.TrimSpace(req.ClaimKey)
	}
	if secretKey == "" {
		a.logger.WithFields(logrus.Fields{
			"topic":     topic,
			"device_id": targetDeviceID,
		}).Error("secretKey missing in claim message")
		return errors.New("secretKey or claimKey is required")
	}

	// 计算 TTL（秒）
	var ttlSeconds int64
	if req.DurationMs > 0 {
		ttlSeconds = req.DurationMs / 1000
		if ttlSeconds == 0 {
			ttlSeconds = 1
		}
	} else if req.TTLSeconds > 0 {
		ttlSeconds = req.TTLSeconds
	} else if req.TTLSeconds2 > 0 {
		ttlSeconds = req.TTLSeconds2
	}

	claimSvc := &service.DeviceClaim{}
	resp, err := claimSvc.RegisterDeviceClaimFromDevice(context.Background(), targetDeviceID, secretKey, ttlSeconds)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"topic":     topic,
			"device_id": targetDeviceID,
			"error":     err,
		}).Error("Failed to register device claim from device")
		return err
	}

	a.logger.WithFields(logrus.Fields{
		"topic":         topic,
		"device_id":     targetDeviceID,
		"device_number": resp.DeviceNumber,
		"token_id":      resp.TokenID,
		"expires_at":    resp.ExpiresAt,
	}).Info("Device claim registered successfully via MQTT")

	return nil
}
