// 文件用途：维护设备心跳写入、续期和在线状态服务。
// 核心逻辑：解析心跳 payload，更新 Redis TTL 与设备在线状态，并供监控器读取。
// 关键注意事项：心跳入口是 broker 与业务状态桥梁，错误输入不能污染在线状态或吞掉存储错误。
// 重构建议：拆分心跳解析、存储和状态发布接口，补齐时钟、Redis 失败、并发和超时测试。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
)

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Heartbeat     int `json:"heartbeat"`      // 心跳间隔(秒)
	OnlineTimeout int `json:"online_timeout"` // 在线超时(秒)
}

// HeartbeatService 心跳服务
type HeartbeatService struct {
	redis       *redis.Client
	logger      *logrus.Logger
	configCache heartbeatConfigCache
}

const heartbeatConfigCacheTTL = 30 * time.Second

type heartbeatConfigCacheEntry struct {
	config    *HeartbeatConfig
	expiresAt time.Time
}

type heartbeatConfigCache struct {
	mu    sync.RWMutex
	items map[string]heartbeatConfigCacheEntry
}

// NewHeartbeatService 创建心跳服务实例
func NewHeartbeatService(redis *redis.Client, logger *logrus.Logger) *HeartbeatService {
	return &HeartbeatService{
		redis:  redis,
		logger: logger,
		configCache: heartbeatConfigCache{
			items: make(map[string]heartbeatConfigCacheEntry),
		},
	}
}

// GetConfig 获取设备的心跳配置
func (s *HeartbeatService) GetConfig(device *model.Device) (*HeartbeatConfig, error) {
	// 没有配置ID,返回nil表示无配置
	if device.DeviceConfigID == nil {
		return nil, nil
	}
	deviceConfigID := *device.DeviceConfigID
	if deviceConfigID == "" {
		return nil, nil
	}
	if config, ok := s.getCachedConfig(deviceConfigID); ok {
		return config, nil
	}

	// 从数据库获取设备配置
	deviceConfig, err := dal.GetDeviceConfigByID(deviceConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device config: %w", err)
	}

	// other_config 为空
	if deviceConfig.OtherConfig == nil {
		s.setCachedConfig(deviceConfigID, nil)
		return nil, nil
	}

	// 解析 other_config JSON
	var config HeartbeatConfig
	if err := json.Unmarshal([]byte(*deviceConfig.OtherConfig), &config); err != nil {
		return nil, fmt.Errorf("failed to parse other_config: %w", err)
	}

	// 如果两个配置都为0,返回nil表示无配置
	if config.Heartbeat == 0 && config.OnlineTimeout == 0 {
		s.setCachedConfig(deviceConfigID, nil)
		return nil, nil
	}

	s.setCachedConfig(deviceConfigID, &config)
	return &config, nil
}

func (s *HeartbeatService) getCachedConfig(deviceConfigID string) (*HeartbeatConfig, bool) {
	now := time.Now()
	s.configCache.mu.RLock()
	entry, ok := s.configCache.items[deviceConfigID]
	s.configCache.mu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		return nil, false
	}
	if entry.config == nil {
		return nil, true
	}
	config := *entry.config
	return &config, true
}

func (s *HeartbeatService) setCachedConfig(deviceConfigID string, config *HeartbeatConfig) {
	var cached *HeartbeatConfig
	if config != nil {
		copied := *config
		cached = &copied
	}

	s.configCache.mu.Lock()
	s.configCache.items[deviceConfigID] = heartbeatConfigCacheEntry{
		config:    cached,
		expiresAt: time.Now().Add(heartbeatConfigCacheTTL),
	}
	s.configCache.mu.Unlock()
}

// SetHeartbeat 设置心跳 key
func (s *HeartbeatService) SetHeartbeat(deviceID string, interval int) error {
	if interval <= 0 {
		return fmt.Errorf("invalid heartbeat interval: %d", interval)
	}

	key := fmt.Sprintf("device:%s:heartbeat", deviceID)
	ctx := context.Background()

	if err := s.redis.Set(ctx, key, 1, time.Duration(interval)*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to set heartbeat key: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"interval":  interval,
		"key":       key,
	}).Debug("Heartbeat key set")

	return nil
}

// SetTimeout 设置超时 key
func (s *HeartbeatService) SetTimeout(deviceID string, timeout int) error {
	if timeout <= 0 {
		return fmt.Errorf("invalid timeout: %d", timeout)
	}

	key := fmt.Sprintf("device:%s:timeout", deviceID)
	ctx := context.Background()

	if err := s.redis.Set(ctx, key, 1, time.Duration(timeout)*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to set timeout key: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"timeout":   timeout,
		"key":       key,
	}).Debug("Timeout key set")

	return nil
}

// RefreshHeartbeat 刷新心跳(根据配置自动选择heartbeat或timeout)
func (s *HeartbeatService) RefreshHeartbeat(device *model.Device, config *HeartbeatConfig) error {
	if config == nil {
		return nil
	}

	// 优先级: heartbeat > online_timeout
	if config.Heartbeat > 0 {
		return s.SetHeartbeat(device.ID, config.Heartbeat)
	} else if config.OnlineTimeout > 0 {
		return s.SetTimeout(device.ID, config.OnlineTimeout)
	}

	return nil
}

// DeleteHeartbeatKey 删除心跳key(用于设备删除等场景)
func (s *HeartbeatService) DeleteHeartbeatKey(deviceID string) error {
	ctx := context.Background()

	// 删除两种可能的key
	heartbeatKey := fmt.Sprintf("device:%s:heartbeat", deviceID)
	timeoutKey := fmt.Sprintf("device:%s:timeout", deviceID)

	if err := s.redis.Del(ctx, heartbeatKey, timeoutKey).Err(); err != nil {
		return fmt.Errorf("failed to delete heartbeat keys: %w", err)
	}

	s.logger.WithField("device_id", deviceID).Debug("Heartbeat keys deleted")
	return nil
}

// DeleteTimeoutKey 删除超时key
func (s *HeartbeatService) DeleteTimeoutKey(deviceID string) error {
	ctx := context.Background()
	timeoutKey := fmt.Sprintf("device:%s:timeout", deviceID)
	if err := s.redis.Del(ctx, timeoutKey).Err(); err != nil {
		return fmt.Errorf("failed to delete timeout key: %w", err)
	}
	return nil
}
