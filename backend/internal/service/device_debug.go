// 文件用途：维护设备调试开关、调试 TTL 和 broker 日志读取。
// 核心逻辑：校验设备访问权限，写入 Redis 调试状态，并把 broker 日志转换成安全 API 响应。
// 关键注意事项：调试日志可能包含敏感或畸形 payload，必须限制回显、分页和过期策略。
// 重构建议：将 Redis key、日志归一化和权限检查拆分，补齐敏感字段、坏日志和缓存失败测试。
// device_debug.go stores per-device debug state and broker log slices.
//
// Purpose: enable, read, and page device debug sessions backed by Redis keys that the broker can also understand.
// Core logic: enforces device access, normalizes debug TTL/limits, writes config state, and converts Redis log entries into safe API responses.
// Important notes: debug logs may contain malformed or sensitive broker payloads, so invalid entries are sanitized and raw payload echoing must stay blocked.
// Refactor suggestion: move Redis key and log-normalization rules into a small package shared with broker tests.
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/redis/go-redis/v9"
)

type DeviceDebug struct{}

const (
	// Keep the tp:devdebug:* Redis keys stable for the broker debug plugin.
	// The plugin writes captured packets to the logs list directly, while this
	// service owns the HTTP-facing config/status contract.
	devDebugCfgKeyPrefix  = "tp:devdebug:cfg:"
	devDebugLogsKeyPrefix = "tp:devdebug:logs:"

	defaultDebugDurationSeconds = int64(30 * 60)
	defaultDebugMaxItems        = 1000
	defaultDebugPayloadMaxBytes = 4096
	defaultDebugLogsLimit       = int64(100)
	maxDebugLogsLimit           = int64(500)
	debugTTLExtendSeconds       = int64(10 * 60)
)

func devDebugCfgKey(deviceID string) string {
	return devDebugCfgKeyPrefix + strings.TrimSpace(deviceID)
}
func devDebugLogsKey(deviceID string) string {
	return devDebugLogsKeyPrefix + strings.TrimSpace(deviceID)
}

func (s *DeviceDebug) assertDeviceReadAccess(ctx context.Context, deviceID string, claims *utils.UserClaims) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	_, err := ensureTelemetryDeviceReadAccess(deviceID, claims)
	return err
}

func (s *DeviceDebug) assertDeviceWriteAccess(ctx context.Context, deviceID string, claims *utils.UserClaims) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	_, err := ensureTelemetryDeviceWriteAccess(deviceID, claims)
	return err
}

type DeviceDebugStatus struct {
	Enabled          bool                    `json:"enabled"`
	ExpireAt         int64                   `json:"expire_at"`
	RemainingSeconds int64                   `json:"remaining_seconds"`
	Config           model.DeviceDebugConfig `json:"config"`
}

func (s *DeviceDebug) prepareReadableDeviceDebug(ctx context.Context, deviceID string, claims *utils.UserClaims) (string, error) {
	if err := ensureDeviceDebugRedis(); err != nil {
		return "", err
	}
	if err := s.assertDeviceReadAccess(ctx, deviceID, claims); err != nil {
		return "", err
	}
	return strings.TrimSpace(deviceID), nil
}

func (s *DeviceDebug) prepareWritableDeviceDebug(ctx context.Context, deviceID string, claims *utils.UserClaims) (string, error) {
	if err := ensureDeviceDebugRedis(); err != nil {
		return "", err
	}
	if err := s.assertDeviceWriteAccess(ctx, deviceID, claims); err != nil {
		return "", err
	}
	return strings.TrimSpace(deviceID), nil
}

func (s *DeviceDebug) SetDeviceDebug(ctx context.Context, deviceID string, req *model.SetDeviceDebugReq, claims *utils.UserClaims) (DeviceDebugStatus, error) {
	var resp DeviceDebugStatus
	normalizedDeviceID, err := s.prepareWritableDeviceDebug(ctx, deviceID, claims)
	if err != nil {
		return resp, err
	}

	now := time.Now().Unix()

	cfg, ttlSeconds, enabled := normalizeDeviceDebugConfig(req, now)
	if !enabled {
		clearDeviceDebugConfig(ctx, normalizedDeviceID)
		return getDeviceDebugStatusPrepared(ctx, normalizedDeviceID, now)
	}

	if err := storeDeviceDebugConfig(ctx, normalizedDeviceID, cfg, ttlSeconds); err != nil {
		return resp, deviceDebugCacheError(err)
	}

	return getDeviceDebugStatusPrepared(ctx, normalizedDeviceID, now)
}

func (s *DeviceDebug) GetDeviceDebugStatus(ctx context.Context, deviceID string, claims *utils.UserClaims) (DeviceDebugStatus, error) {
	var resp DeviceDebugStatus
	normalizedDeviceID, err := s.prepareReadableDeviceDebug(ctx, deviceID, claims)
	if err != nil {
		return resp, err
	}

	return getDeviceDebugStatusPrepared(ctx, normalizedDeviceID, time.Now().Unix())
}

type DeviceDebugLogsResp struct {
	Total  int64                       `json:"total"`
	Offset int64                       `json:"offset"`
	Limit  int64                       `json:"limit"`
	List   []model.DeviceDebugLogEntry `json:"list"`
}

func (s *DeviceDebug) GetDeviceDebugLogs(ctx context.Context, deviceID string, req *model.GetDeviceDebugLogsReq, claims *utils.UserClaims) (DeviceDebugLogsResp, error) {
	var resp DeviceDebugLogsResp
	normalizedDeviceID, err := s.prepareReadableDeviceDebug(ctx, deviceID, claims)
	if err != nil {
		return resp, err
	}

	offset, limit := normalizeDeviceDebugLogPage(req)
	return queryDeviceDebugLogs(ctx, normalizedDeviceID, offset, limit)
}

func ensureDeviceDebugRedis() error {
	if global.REDIS == nil {
		return errcode.NewWithMessage(errcode.CodeSystemError, "redis not initialized")
	}
	return nil
}

func clearDeviceDebugConfig(ctx context.Context, deviceID string) {
	_ = global.REDIS.Del(ctx, devDebugCfgKey(deviceID)).Err()
}

func storeDeviceDebugConfig(ctx context.Context, deviceID string, cfg model.DeviceDebugConfig, ttlSeconds int64) error {
	raw, _ := json.Marshal(cfg)
	ttl := time.Duration(ttlSeconds) * time.Second
	pipe := global.REDIS.Pipeline()
	pipe.Set(ctx, devDebugCfgKey(deviceID), raw, ttl)
	// Extend an existing log list with the config TTL so logs remain readable
	// for the full debug window without resurrecting logs when none exist.
	pipe.Expire(ctx, devDebugLogsKey(deviceID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func queryDeviceDebugStatus(ctx context.Context, deviceID string, now int64) (DeviceDebugStatus, error) {
	cfg, found, err := loadDeviceDebugConfig(ctx, deviceID)
	if err != nil {
		return DeviceDebugStatus{}, err
	}
	if !found {
		return deviceDebugStatusFromConfig(defaultDeviceDebugConfig(), now, false), nil
	}
	return deviceDebugStatusFromConfig(cfg, now, true), nil
}

func getDeviceDebugStatusPrepared(ctx context.Context, normalizedDeviceID string, now int64) (DeviceDebugStatus, error) {
	return queryDeviceDebugStatus(ctx, normalizedDeviceID, now)
}

func loadDeviceDebugConfig(ctx context.Context, deviceID string) (model.DeviceDebugConfig, bool, error) {
	val, err := global.REDIS.Get(ctx, devDebugCfgKey(deviceID)).Result()
	if err != nil && err != redis.Nil {
		return model.DeviceDebugConfig{}, false, deviceDebugCacheError(err)
	}
	if err == redis.Nil {
		return model.DeviceDebugConfig{}, false, nil
	}

	cfg, decodeErr := decodeDeviceDebugConfig(val)
	if decodeErr != nil {
		return model.DeviceDebugConfig{}, false, deviceDebugCacheError(decodeErr)
	}
	return cfg, true, nil
}

func queryDeviceDebugLogs(ctx context.Context, deviceID string, offset, limit int64) (DeviceDebugLogsResp, error) {
	total, rows, err := loadDeviceDebugLogRows(ctx, deviceID, offset, limit)
	if err != nil {
		return DeviceDebugLogsResp{}, err
	}

	return DeviceDebugLogsResp{
		Total:  total,
		Offset: offset,
		Limit:  limit,
		List:   normalizeDeviceDebugLogRows(rows),
	}, nil
}

func loadDeviceDebugLogRows(ctx context.Context, deviceID string, offset, limit int64) (int64, []string, error) {
	key := devDebugLogsKey(deviceID)
	start := offset
	stop := offset + limit - 1

	pipe := global.REDIS.Pipeline()
	llen := pipe.LLen(ctx, key)
	lrange := pipe.LRange(ctx, key, start, stop)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return 0, nil, deviceDebugCacheError(err)
	}

	total, _ := llen.Result()
	rows, _ := lrange.Result()
	return total, rows, nil
}

func normalizeDeviceDebugLogPage(req *model.GetDeviceDebugLogsReq) (int64, int64) {
	offset := int64(0)
	limit := defaultDebugLogsLimit
	if req == nil {
		return offset, limit
	}
	if req.Offset > 0 {
		offset = req.Offset
	}
	if req.Limit > 0 {
		limit = req.Limit
	}
	if limit > maxDebugLogsLimit {
		limit = maxDebugLogsLimit
	}
	return offset, limit
}

func decodeDeviceDebugConfig(raw string) (model.DeviceDebugConfig, error) {
	var cfg model.DeviceDebugConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return model.DeviceDebugConfig{}, err
	}
	return cfg, nil
}

func deviceDebugStatusFromConfig(cfg model.DeviceDebugConfig, now int64, found bool) DeviceDebugStatus {
	status := DeviceDebugStatus{
		Config: cfg,
	}
	if !found {
		return status
	}
	status.Enabled = cfg.Enabled
	status.ExpireAt = cfg.ExpireAt
	if cfg.ExpireAt > 0 && now > cfg.ExpireAt {
		status.Enabled = false
	}
	if cfg.ExpireAt > now {
		status.RemainingSeconds = cfg.ExpireAt - now
	}
	return status
}

func deviceDebugCacheError(err error) error {
	return errcode.WithData(errcode.CodeCacheError, map[string]interface{}{"cache_error": err.Error()})
}

func normalizeDeviceDebugConfig(req *model.SetDeviceDebugReq, now int64) (model.DeviceDebugConfig, int64, bool) {
	if req != nil && req.Enabled != nil && !*req.Enabled {
		return model.DeviceDebugConfig{}, 0, false
	}

	cfg := defaultDeviceDebugConfig()
	cfg.Enabled = true

	switch {
	case req != nil && req.ExpireAt != nil && *req.ExpireAt > 0:
		cfg.ExpireAt = *req.ExpireAt
	case req != nil && req.Duration != nil:
		if *req.Duration <= 0 {
			return model.DeviceDebugConfig{}, 0, false
		}
		cfg.ExpireAt = now + *req.Duration
	default:
		cfg.ExpireAt = now + defaultDebugDurationSeconds
	}

	if req != nil && req.MaxItems != nil && *req.MaxItems > 0 {
		cfg.MaxItems = *req.MaxItems
	}
	if req != nil && req.PayloadMaxBytes != nil && *req.PayloadMaxBytes >= 0 {
		cfg.PayloadMaxBytes = *req.PayloadMaxBytes
	}

	ttlSeconds := deviceDebugConfigTTLSeconds(now, cfg.ExpireAt)
	if ttlSeconds <= 0 {
		return model.DeviceDebugConfig{}, 0, false
	}

	return cfg, ttlSeconds, true
}

func deviceDebugConfigTTLSeconds(now int64, expireAt int64) int64 {
	return (expireAt - now) + debugTTLExtendSeconds
}

func defaultDeviceDebugConfig() model.DeviceDebugConfig {
	return model.DeviceDebugConfig{
		Enabled:         false,
		ExpireAt:        0,
		MaxItems:        defaultDebugMaxItems,
		PayloadMaxBytes: defaultDebugPayloadMaxBytes,
	}
}

func normalizeDeviceDebugOutcome(result string) string {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "ok":
		return "ok"
	case "denied", "deny":
		return "deny"
	case "error":
		return "error"
	case "discarded", "drop":
		return "drop"
	default:
		return result
	}
}
