// 文件用途：注册并管理 AetherLink broker 插件的运行时生命周期。
// 核心逻辑：初始化数据库、Redis、内部 MQTT 客户端和设备会话撤销 monitor。
// 关键注意事项：Load 成功启动的后台资源必须在 Unload 中对应关闭。

package aetherlink

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/DrmagicE/gmqtt/config"
	"github.com/DrmagicE/gmqtt/server"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var _ server.Plugin = (*AetherLinkPlugin)(nil)

// Name is the canonical GMQTT runtime name for the AetherLink plugin.
const Name = "aetherlink"

var (
	runtimeInitOnce                      sync.Once
	runtimeInitErr                       error
	runtimeMQTTSessionRevocationBrokerID string
	// Log is initialized during Load and shared by the runtime plugin hooks.
	Log *zap.Logger
)

func init() {
	if err := server.RegisterPlugin(Name, New); err != nil {
		panic(err)
	}
	config.RegisterDefaultPluginConfig(Name, &DefaultConfig)
}

func readRuntimeConfig() error {
	viper.SetConfigName(Name)
	if err := viper.ReadInConfig(); err == nil {
		log.Printf("aetherlink-gmqtt: loaded %s.yml configuration", Name)
		return nil
	}
	return fmt.Errorf("aetherlink-gmqtt: failed to read configuration file: expected %s.yml", Name)
}

// configureRuntimeEnvironment makes every runtime value that is documented as
// GMQTT_* configurable explicit to Viper.  AutomaticEnv alone is easy to
// misread when a key is already present in a config file; an explicit binding
// keeps isolated databases, broker credentials, and deployment overrides from
// silently falling back to stale file values.
func configureRuntimeEnvironment() error {
	viper.SetEnvPrefix("GMQTT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// An explicitly empty value is meaningful for local Redis and for
	// deployments that intentionally disable an optional password.  Without
	// this, Viper silently falls back to a stale non-empty config-file value.
	viper.AllowEmptyEnv(true)
	viper.AutomaticEnv()

	for _, key := range []string{
		"db.psql.psqladdr",
		"db.psql.psqlport",
		"db.psql.psqluser",
		"db.psql.psqlpass",
		"db.psql.psqldb",
		"db.psql.sslmode",
		"db.redis.conn",
		"db.redis.db_num",
		"db.redis.password",
		"mqtt.broker",
		"mqtt.password",
		"mqtt.plugin_password",
		"mqtt_session_revocations.broker_id",
		"payload_schema.enabled",
		"payload_schema.cache_ttl",
		"auth_ratelimit.max_failures_per_minute",
	} {
		if err := viper.BindEnv(key); err != nil {
			return fmt.Errorf("aetherlink-gmqtt: bind runtime environment key %s: %w", key, err)
		}
	}
	return nil
}

// runtimeInit loads aetherlink.yml and initializes database,
// Redis, and the internal MQTT client used for device status and forwarding.
func runtimeInit() error {
	log.Println("aetherlink-gmqtt: initializing config...")
	if err := configureRuntimeEnvironment(); err != nil {
		return err
	}
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	if err := readRuntimeConfig(); err != nil {
		return err
	}
	brokerID, err := normalizeMQTTSessionRevocationBrokerID(
		viper.GetString(mqttSessionRevocationBrokerIDConfigKey),
	)
	if err != nil {
		return fmt.Errorf("aetherlink-gmqtt: invalid mqtt session revocation broker identity: %w", err)
	}
	runtimeMQTTSessionRevocationBrokerID = brokerID

	if err := Init(); err != nil { // init database & redis
		return fmt.Errorf("aetherlink-gmqtt: init database/redis failed: %w", err)
	}
	go DefaultMqttClient.MqttInit()
	return nil
}

// New creates the AetherLink IoT plugin instance.
func New(config config.Config) (server.Plugin, error) {
	return &AetherLinkPlugin{}, nil
}

// AetherLinkPlugin is the AetherLink IoT MQTT plugin for auth,
// routing, status reporting, and device-session revocation.
type AetherLinkPlugin struct {
	mu                            sync.Mutex
	sessionRevocationMonitor      *mqttSessionRevocationMonitor
	voucherCacheInvalidationWatch *voucherCacheInvalidationMonitor
}

// Load initializes logging and runtime dependencies once.
func (t *AetherLinkPlugin) Load(service server.Server) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sessionRevocationMonitor != nil {
		return nil
	}
	Log = server.LoggerWithField(zap.String("plugin", Name))
	runtimeInitOnce.Do(func() {
		runtimeInitErr = runtimeInit()
	})
	if runtimeInitErr != nil {
		SetPayloadSchemaResolver(nil)
		return runtimeInitErr
	}
	configurePayloadSchemaResolver()
	monitor := newMQTTSessionRevocationMonitor(
		newMQTTDeviceSessionRevoker(service.ClientService()),
		subscribeRedisMQTTSessionRevocations,
		runtimeMQTTSessionRevocationBrokerID,
		publishRedisMQTTSessionRevocationAck,
	)
	if err := monitor.Start(); err != nil {
		return fmt.Errorf("aetherlink-gmqtt: start mqtt session revocation monitor: %w", err)
	}
	t.sessionRevocationMonitor = monitor

	// 凭证缓存失效通道是残窗收口的卫生机制：订阅建立失败只降级告警，
	// 不阻断 broker 启动（残留映射仍受缓存 TTL 兜底）。
	invalidationWatch := newVoucherCacheInvalidationMonitor(subscribeRedisVoucherCacheInvalidations)
	if err := invalidationWatch.Start(); err != nil {
		if Log != nil {
			Log.Warn("start voucher cache invalidation monitor failed; rotation residual window falls back to cache TTL",
				zap.Error(err),
			)
		}
	} else {
		t.voucherCacheInvalidationWatch = invalidationWatch
	}
	return nil
}

// Unload releases the plugin-owned internal MQTT client and its worker.
func (t *AetherLinkPlugin) Unload() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	SetPayloadSchemaResolver(nil)
	var firstErr error
	if t.sessionRevocationMonitor != nil {
		firstErr = t.sessionRevocationMonitor.Close()
		t.sessionRevocationMonitor = nil
	}
	if t.voucherCacheInvalidationWatch != nil {
		if err := t.voucherCacheInvalidationWatch.Close(); firstErr == nil {
			firstErr = err
		}
		t.voucherCacheInvalidationWatch = nil
	}
	if err := DefaultMqttClient.Close(); firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// Name returns the canonical runtime plugin name.
func (t *AetherLinkPlugin) Name() string { return Name }
