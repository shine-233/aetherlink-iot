// 文件用途：初始化 Redis 客户端并提供若干设备、脚本相关的缓存辅助函数。
// 核心逻辑：加载主 Redis 与状态 Redis 配置、校验连接可用性，并封装 JSON 结构的缓存读写与删除。
// 关键注意事项：这里既负责基础设施连接，也承载部分业务缓存助手，维护时需区分“连接初始化”和“缓存语义”两类影响面。
// 重构建议：后续可将业务缓存 helper 拆出到独立包，避免一个文件同时承担客户端装配与缓存访问职责。

package initialize

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// redisNilLogger 实现 go-redis v9 的 internal.Logging 接口，空实现。
// 用于禁用 go-redis 内部日志输出，防止 Redis 不可用时错误日志风暴撑爆磁盘。
type redisNilLogger struct{}

func (n *redisNilLogger) Printf(ctx context.Context, format string, v ...interface{}) {}

// init 在包加载时全局禁用 go-redis 内部日志。
func init() {
	redis.SetLogger(&redisNilLogger{})
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// RedisInit 初始化主 Redis 与状态 Redis，并启动依赖其存在的 SSE/WS 管理器。
func RedisInit() (*redis.Client, error) {
	conf, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("加载redis配置失败: %w", err)
	}

	statusConf, err := loadStatusConfig()
	if err != nil {
		return nil, fmt.Errorf("加载redis配置失败: %w", err)
	}

	client := connectRedis(conf)
	statusClient := connectRedis(statusConf)

	if err := checkRedisClient(client); err != nil {
		return nil, fmt.Errorf("连接redis失败: %w", err)
	}
	if err := checkRedisClient(statusClient); err != nil {
		return nil, fmt.Errorf("连接redis失败: %w", err)
	}
	global.REDIS = client
	global.STATUS_REDIS = statusClient
	// 启动SSE
	go global.InitSSEManager()
	// 启动WebSocket管理器
	go global.InitWSManager()
	// Casbin 集群策略同步 watcher（casbin.watcher.enabled 门控；见 ROADMAP C7+）。
	if err := attachCasbinWatcher(); err != nil {
		return nil, fmt.Errorf("挂载 casbin watcher 失败: %w", err)
	}
	return client, nil
}

// connectRedis 根据配置创建 Redis 客户端实例，不在此处触发网络校验。
func connectRedis(conf *RedisConfig) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:       conf.Addr,
		Password:   conf.Password,
		DB:         conf.DB,
		MaxRetries: 1, // 限制重试次数，避免放大错误量
	})
	return redisClient
}

// checkRedisClient 通过 Ping 校验 Redis 客户端是否已成功连通目标实例。
func checkRedisClient(redisClient *redis.Client) error {
	// 通过 cient.Ping() 来检查是否成功连接到了 redis 服务器
	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		return err
	} else {
		log.Println("连接redis成完成...")
		return nil
	}
}

// loadConfig 加载主 Redis 配置，并补齐默认地址。
func loadConfig() (*RedisConfig, error) {
	redisConfig := &RedisConfig{
		Addr:     viper.GetString("db.redis.addr"),
		Password: viper.GetString("db.redis.password"),
		DB:       viper.GetInt("db.redis.db"),
	}

	if redisConfig.Addr == "" {
		redisConfig.Addr = "localhost:6379"
	}
	return redisConfig, nil
}

// loadStatusConfig 加载状态 Redis 配置；若未配置独立 DB，则回退到约定默认值。
func loadStatusConfig() (*RedisConfig, error) {
	db := viper.GetInt("db.redis.db1")
	if db == 0 {
		db = 10 // 默认使用第11个DB
	}
	redisConfig := &RedisConfig{
		Addr:     viper.GetString("db.redis.addr"),
		Password: viper.GetString("db.redis.password"),
		DB:       db,
	}

	if redisConfig.Addr == "" {
		redisConfig.Addr = "localhost:6379"
	}
	return redisConfig, nil
}

// SetRedisForJsondata 将结构化值序列化为 JSON 并写入 Redis。
func SetRedisForJsondata(key string, value interface{}, expiration time.Duration) error {
	if global.REDIS == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return global.REDIS.Set(context.Background(), key, jsonData, expiration).Err()
}

// GetRedisForJsondata 从 Redis 读取 JSON 并反序列化到目标对象。
func GetRedisForJsondata(key string, dest interface{}) error {
	if global.REDIS == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	val, err := global.REDIS.Get(context.Background(), key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// GetDeviceCacheById 优先命中 Redis 设备缓存，未命中时回源 DAL 并写回缓存。
func GetDeviceCacheById(deviceId string) (*model.Device, error) {
	var device model.Device
	err := GetRedisForJsondata(deviceId, &device)
	if err == nil {
		return &device, nil
	}
	// 从数据库中获取设备信息
	deviceFromDB, err := getDeviceCacheByIdFromDAL(deviceId)
	if err != nil {
		return nil, err
	}
	// 将设备信息存入redis。
	// P2 修复（2026-08-25）：兜底 TTL 替代永久缓存——写路径主动失效仍是主机制，
	// 兜底过期确保任何遗漏失效的写路径最终自愈，不再产生永久脏读。
	err = SetRedisForJsondata(deviceId, deviceFromDB, constant.CacheFallbackTTL)
	if err != nil {
		return nil, err
	}
	return deviceFromDB, nil
}

// GetScriptByDeviceAndScriptType 读取设备对应脚本缓存，未命中时查库并回填。
func GetScriptByDeviceAndScriptType(device *model.Device, scriptType string) (*model.DataScript, error) {
	var script *model.DataScript
	script = &model.DataScript{}
	if device.DeviceConfigID == nil {
		return nil, fmt.Errorf("设备配置id为空")
	}
	key := *device.DeviceConfigID + "_" + scriptType + "_script"
	err := GetRedisForJsondata(key, script)
	if err != nil {
		logrus.Debug("get redis cache entry failed")
		script, err = dal.GetDataScriptByDeviceConfigIdAndScriptType(device.DeviceConfigID, scriptType)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error": err.Error(),
			})
		}
		if script == nil {
			return nil, nil
		}
		err = SetRedisForJsondata(key, script, constant.CacheFallbackTTL)
		if err != nil {
			logrus.Debug("set redis cache entry failed")
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error": err.Error(),
			})
		}
		logrus.Debug("set redis cache entry succeeded")
	}
	return script, nil
}

// DelDeviceCache 删除设备主缓存。
func DelDeviceCache(deviceId string) error {
	err := global.REDIS.Del(context.Background(), deviceId).Err()
	if err != nil {
		logrus.Warn("delete Redis device cache failed")
	}
	return err
}

const delDeviceCacheBatchSize = 500

// DelDeviceCaches deletes device main cache keys in chunks for bulk operations.
func DelDeviceCaches(deviceIDs []string) error {
	keys := normalizeDeviceCacheKeys(deviceIDs)
	if len(keys) == 0 {
		return nil
	}
	if global.REDIS == nil {
		return fmt.Errorf("redis client is not initialized")
	}

	var firstErr error
	for start := 0; start < len(keys); start += delDeviceCacheBatchSize {
		end := start + delDeviceCacheBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		if err := global.REDIS.Del(context.Background(), keys[start:end]...).Err(); err != nil {
			logrus.Warn("delete Redis device cache batch failed")
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func normalizeDeviceCacheKeys(deviceIDs []string) []string {
	keys := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			continue
		}
		if _, ok := seen[deviceID]; ok {
			continue
		}
		seen[deviceID] = struct{}{}
		keys = append(keys, deviceID)
	}
	return keys
}

// DelDeviceConfigCache 删除设备配置缓存。
func DelDeviceConfigCache(deviceConfigId string) error {
	err := global.REDIS.Del(context.Background(), deviceConfigId+"_config").Err()
	if err != nil {
		logrus.Warn("delete Redis device config cache failed")
	}
	return err
}

// DelDeviceDataScriptCache 批量删除某设备配置下所有脚本类型缓存。
func DelDeviceDataScriptCache(deviceConfigID string) error {
	scriptType := []string{"A", "B", "C", "D", "E", "F"}
	var key []string
	for _, scriptType := range scriptType {
		key = append(key, deviceConfigID+"_"+scriptType+"_script")
	}

	err := global.REDIS.Del(context.Background(), key...).Err()
	if err != nil {
		logrus.Warn("delete Redis cache entry failed")
	}
	return err
}

// getDeviceCacheByIdFromDAL 从 DAL 回源设备信息，并兜底处理初始化缺失时的 panic。
func getDeviceCacheByIdFromDAL(deviceId string) (device *model.Device, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("device cache DAL is not initialized: %v", recovered)
		}
	}()

	return dal.GetDeviceCacheById(deviceId)
}
