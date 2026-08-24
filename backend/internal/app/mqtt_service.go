// 文件用途：承载应用启动编排中 mqtt service 相关的服务或配置逻辑。
// 核心逻辑：把配置加载、服务注册、启动、停止和依赖注入封装为应用层可组合入口，主要围绕 type MQTTService、var globalMQTTAdapter、func GetGlobalMQTTAdapter、func NewMQTTService 等声明展开。
// 关键注意事项：应用生命周期影响全局资源，修改需保持幂等、关闭顺序和错误回滚语义。
// 重构建议：后续可继续收敛服务接口边界，让启动编排与具体基础设施实现解耦。

package app

import (
	"fmt"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/internal/adapter/mqttadapter"
	"aetherlink-iot/backend/internal/mqttdebug"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/mqtt"
	"aetherlink-iot/backend/pkg/utils"

	mqtt_client "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// MQTTService 实现MQTT相关服务
type MQTTService struct {
	app         *Application
	initialized bool
	mqttAdapter *mqttadapter.Adapter
	mqttDebug   *mqttdebug.Manager
}

// 全局 Adapter 实例（供其他模块调用）
var globalMQTTAdapter *mqttadapter.Adapter

// GetGlobalMQTTAdapter 获取全局 MQTT Adapter 实例
func GetGlobalMQTTAdapter() *mqttadapter.Adapter {
	return globalMQTTAdapter
}

// NewMQTTService 创建MQTT服务实例
func NewMQTTService() *MQTTService {
	return &MQTTService{
		initialized: false,
	}
}

// mqttServiceEnabled 只有显式配置 mqtt.enabled=true 才启动外部 broker 连接。
// 未配置时保持本地核心服务可启动，并将 MQTT 标记为可选能力。
func mqttServiceEnabled() bool {
	return viper.IsSet("mqtt.enabled") && viper.GetBool("mqtt.enabled")
}

// Name 返回服务名称
func (s *MQTTService) Name() string {
	return "MQTT服务"
}

// Start 启动MQTT服务
func (s *MQTTService) Start() error {
	if !mqttServiceEnabled() {
		service.SetDeviceMQTTDebugRuntime(nil)
		logrus.Info("MQTT service disabled by config")
		return nil
	}

	logrus.Info("正在启动MQTT服务...")

	// 初始化MQTT配置（只加载配置，不创建客户端）
	if err := mqtt.MqttInit(); err != nil {
		return err
	}

	// 初始化限流器
	initialize.NewAutomateLimiter()

	// 注意: 设备状态监控已由 Flow 层的 HeartbeatMonitor 和 StatusUplink 接管
	// 不再使用 device.InitDeviceStatus()

	// MQTT subscriptions and publishes are owned by the adapter-managed flow.
	// Keep this service as the lifecycle boundary for that adapter.

	// ✨ 创建 MQTT Adapter 并订阅所有 Topic
	if err := s.initMQTTAdapter(); err != nil {
		logrus.WithError(err).Error("Failed to initialize MQTT Adapter")
		return err
	}

	s.initialized = true
	logrus.Info("MQTT服务启动完成")
	return nil
}

// initMQTTAdapter 初始化 MQTT Adapter（创建独立的 MQTT 客户端）
func (s *MQTTService) initMQTTAdapter() error {
	initialized := false
	defer func() {
		if !initialized {
			s.cleanupMQTTAdapterRuntime()
		}
	}()

	// 1. 获取 Flow Bus
	bus := s.app.GetUplinkBus()
	if bus == nil {
		return fmt.Errorf("uplink bus not initialized, cannot create MQTT Adapter")
	}

	// 2. 创建 Adapter 专用的 MQTT 客户端（不依赖 mqtt/publish/）
	// 拨号地址统一经助手归一化：回环配置回退 env，并把 localhost 固定为 127.0.0.1 防 ::1 偏好；debug 会话客户端复用同一地址。
	broker := utils.ResolveMQTTBrokerDialAddress(viper.GetString("mqtt.broker"))
	username := viper.GetString("mqtt.user")
	password := viper.GetString("mqtt.pass")
	clientID := viper.GetString("mqtt.client_id")

	// 3. 先创建临时 Adapter（用于订阅回调）
	var tempAdapter *mqttadapter.Adapter

	mqttConfig := mqttadapter.MQTTConfig{

		Broker:   broker,
		Username: username,
		Password: password,
		ClientID: clientID,

		// ✨ 设置连接成功回调：重连后自动重新订阅所有 Topic
		OnConnectCallback: func(client mqtt_client.Client) {
			if tempAdapter == nil {
				return // 首次连接时 adapter 还未创建，跳过
			}

			logrus.Info("Re-subscribing all topics after reconnection...")

			// 重新订阅响应 Topic
			if err := tempAdapter.SubscribeResponseTopics(client); err != nil {
				logrus.WithError(err).Error("Failed to re-subscribe response topics")
			}

			// 重新订阅设备上行 Topic
			if err := tempAdapter.SubscribeDeviceTopics(client); err != nil {
				logrus.WithError(err).Error("Failed to re-subscribe device topics")
			}

			// 重新订阅网关上行 Topic
			if err := tempAdapter.SubscribeGatewayTopics(client); err != nil {
				logrus.WithError(err).Error("Failed to re-subscribe gateway topics")
			}

			logrus.Info("All topics re-subscribed successfully after reconnection")
		},
	}

	mqttClient, err := mqttadapter.CreateMQTTClient(mqttConfig, s.app.Logger)
	if err != nil {
		return fmt.Errorf("failed to create MQTT client for Adapter: %w", err)
	}
	service.SetMQTTHealthProbe(func() bool {
		return mqttClient.IsConnected()
	})

	// 4. 创建 MQTT Adapter
	s.mqttAdapter = mqttadapter.NewAdapter(bus, mqttClient, s.app.Logger)
	tempAdapter = s.mqttAdapter       // 赋值给临时变量，供回调使用
	globalMQTTAdapter = s.mqttAdapter // 设置全局实例
	logrus.Info("MQTT Adapter created with independent client")

	// 5. 首次订阅所有 Topic（重连后会通过 OnConnectCallback 自动重新订阅）
	if err := s.mqttAdapter.SubscribeResponseTopics(mqttClient); err != nil {
		return fmt.Errorf("failed to subscribe response topics: %w", err)
	}

	if err := s.mqttAdapter.SubscribeDeviceTopics(mqttClient); err != nil {
		return fmt.Errorf("failed to subscribe device topics: %w", err)
	}

	if err := s.mqttAdapter.SubscribeGatewayTopics(mqttClient); err != nil {
		return fmt.Errorf("failed to subscribe gateway topics: %w", err)
	}

	// Debug sessions use isolated MQTT clients. Reusing mqttClient here would
	// let a manual Subscribe replace a production route handler in Paho.
	s.mqttDebug = mqttdebug.NewManager(mqttdebug.Config{
		Broker:       broker,
		Username:     username,
		Password:     password,
		UplinkSource: NewBusUplinkSource(bus),
	}, s.app.Logger)
	service.SetDeviceMQTTDebugRuntime(s.mqttDebug)

	logrus.Info("MQTT Adapter initialized successfully - all subscriptions active")
	logrus.Info("📌 Automatic re-subscription on reconnect is enabled")
	logrus.Info("MQTT adapter-managed flow is active")
	initialized = true
	return nil
}

// cleanupMQTTAdapterRuntime releases both fully initialized and partially
// initialized MQTT resources. Start failures are not registered for the
// ServiceManager rollback pass, so initMQTTAdapter must own this cleanup.
func (s *MQTTService) cleanupMQTTAdapterRuntime() {
	service.SetDeviceMQTTDebugRuntime(nil)
	if s.mqttDebug != nil {
		s.mqttDebug.Stop()
		s.mqttDebug = nil
	}
	if s.mqttAdapter != nil {
		mqttadapter.DisconnectMQTTClient(s.mqttAdapter.GetMQTTClient(), s.app.Logger)
		s.mqttAdapter = nil
	}
	service.SetMQTTHealthProbe(nil)
	globalMQTTAdapter = nil
}

// Stop 停止MQTT服务
func (s *MQTTService) Stop() error {
	if !s.initialized {
		return nil
	}

	logrus.Info("正在停止MQTT服务...")
	s.cleanupMQTTAdapterRuntime()
	s.initialized = false

	logrus.Info("MQTT服务已停止")
	return nil
}

// WithMQTTService 将MQTT服务添加到应用
func WithMQTTService() Option {
	return func(app *Application) error {
		service := NewMQTTService()
		service.app = app // ✨ 设置 Application 引用
		app.RegisterService(service)
		app.mqttService = service // ✨ 保存服务引用
		return nil
	}
}
