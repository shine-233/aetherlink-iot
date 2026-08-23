// 文件用途：维护遥测模拟数据生成和调试写入服务。
// 核心逻辑：按设备模型生成或接收模拟遥测值，用于调试页面和开发验证。
// 关键注意事项：模拟数据不能绕过租户权限或污染真实生产设备状态，默认开关和范围需明确。
// 重构建议：拆分模拟生成器和写入接口，补齐权限、开关、随机边界和副作用隔离测试。
// telemetry_simulation.go sends simulated telemetry through broker paths.
//
// Purpose: build simulation initialization data, validate MQTT publish requests, and send synthetic telemetry for device diagnostics.
// Core logic: resolves device/config access, parses broker addresses and voucher credentials, applies request overrides, and logs safe publish metadata.
// Important notes: simulation payloads can contain credentials or arbitrary telemetry bodies, so logs must stay redacted and nil/invalid requests must fail before DAL or broker work.
// Refactor suggestion: extract the MQTT client command construction behind an interface for deterministic tests.
package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	model "aetherlink-iot/backend/internal/model"
	config "aetherlink-iot/backend/mqtt"
	simulationpublish "aetherlink-iot/backend/mqtt/simulation_publish"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// 获取模拟设备发送遥测数据的回显数据
func (*TelemetryData) ServeEchoData(req *model.ServeEchoDataReq, clientIP string, claims *utils.UserClaims) (interface{}, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "请求不能为空")
	}
	// 获取设备信息
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(req.DeviceId, claims)
	if err != nil {
		return nil, err
	}
	voucher := deviceInfo.Voucher
	// 校验voucher是否json
	if !IsJSON(voucher) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "设备凭证不是 JSON 格式")
	}
	var voucherMap map[string]interface{}
	err = json.Unmarshal([]byte(voucher), &voucherMap)
	if err != nil {
		return nil, err
	}
	var username, password, host, port, payload, clientID string
	username, password, err = simulationVoucherCredentials(voucherMap)
	if err != nil {
		return nil, err
	}

	accessAddress := viper.GetString("mqtt.broker")
	host, port, err = parseMQTTAccessAddress(accessAddress)
	if err != nil {
		return nil, err
	}

	// Docker Compose 内 localhost 指向后端容器自身而非 broker。
	// 当 AETHERLINK_MQTT_INNER_BROKER 存在时用它替换回环地址。
	if innerBroker := os.Getenv("AETHERLINK_MQTT_INNER_BROKER"); innerBroker != "" {
		switch host {
		case "localhost", "127.0.0.1", "::1":
			host = innerBroker
		}
	}

	// 如果配置的 host 为占位符 "{MQTT_HOST}"，则使用传入的 clientIP（如果有），否则使用配置的 host
	if host == "{MQTT_HOST}" && clientIP != "" {
		host = clientIP
	}
	topic := config.MqttConfig.Telemetry.SubscribeTopic
	clientID = "mqtt_" + uuid.New()[0:12] // 代表随机生成
	payload = `{"temperature":25.5,"humidity":60,"rssi":-52,"online":true,"alarm_count":0}`
	// 拼接命令
	command := utils.BuildMosquittoPubCommand(host, port, username, password, topic, payload, clientID)
	return command, nil
}

// 模拟设备发送遥测数据
func (*TelemetryData) TelemetryPub(mosquittoCommand string, claims *utils.UserClaims) (interface{}, error) {
	if claims == nil || claims.Authority != constant.SYS_ADMIN {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "只有系统管理员可以发布原始遥测命令")
	}

	// 解析mosquitto_pub命令
	params, err := utils.ParseMosquittoPubCommand(mosquittoCommand)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 发送mqtt消息
	if params == nil {
		logrus.Debug("publishing telemetry simulation command without parameters")
	} else {
		logrus.WithField("payload_size", len(params.Payload)).Debug("publishing telemetry simulation command")
	}
	err = simulationpublish.PublishMessage(params.Host, params.Port, params.Topic, params.Payload, params.Username, params.Password, params.ClientId)
	if err != nil {
		return nil, errcode.WithVars(500007, map[string]interface{}{
			"error_message": err.Error(),
		})
	}
	return nil, nil
}

// GetSimulationInit 获取模拟表单初始值
func (*TelemetryData) GetSimulationInit(deviceId string, claims *utils.UserClaims) (*model.SimulationInitResp, error) {
	// 获取设备信息
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(deviceId, claims)
	if err != nil {
		return nil, err
	}
	if deviceInfo == nil {
		return nil, errcode.New(204003) // 设备不存在
	}

	// 解析 voucher（设备凭证，JSON 格式）
	voucher := deviceInfo.Voucher
	if voucher == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "设备凭证为空")
	}
	if !IsJSON(voucher) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "设备凭证不是有效 JSON")
	}
	var voucherMap map[string]interface{}
	if err := json.Unmarshal([]byte(voucher), &voucherMap); err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	username, password, err := simulationVoucherCredentials(voucherMap)
	if err != nil {
		return nil, err
	}

	// 获取 MQTT 服务器配置
	accessAddress := viper.GetString("mqtt.broker")
	host, portText, err := parseMQTTAccessAddress(accessAddress)
	if err != nil {
		return nil, err
	}
	if innerBroker := os.Getenv("AETHERLINK_MQTT_INNER_BROKER"); innerBroker != "" {
		switch host {
		case "localhost", "127.0.0.1", "::1":
			host = innerBroker
		}
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "MQTT access_address 端口无效")
	}

	// 生成随机客户端ID
	clientID := "mqtt_" + uuid.New()[0:12]

	// 生成6位随机数字
	randNum := fmt.Sprintf("%06d", rand.Intn(1000000))

	// 将 Topic 中的通配符 + 替换为随机数字
	attrTopic := strings.ReplaceAll(config.MqttConfig.Attributes.SubscribeTopic, "+", randNum)
	eventTopic := strings.ReplaceAll(config.MqttConfig.Events.SubscribeTopic, "+", randNum)

	// 默认数据
	defaultData := `{"temperature":25.5,"humidity":60,"rssi":-52,"online":true,"alarm_count":0}`

	resp := &model.SimulationInitResp{
		Username: username,
		Password: password,
		ClientID: clientID,
		Server:   host,
		Port:     port,
		Topic:    config.MqttConfig.Telemetry.SubscribeTopic,
		TopicOptions: []model.SimulationTopicOption{
			{Label: "遥测", Value: config.MqttConfig.Telemetry.SubscribeTopic},
			{Label: "属性", Value: attrTopic},
			{Label: "事件", Value: eventTopic},
		},
		DefaultData:      defaultData,
		EventDefaultData: `{"method":"report_alarm","params":{"alarm_code":"over_temperature","level":"warning","value":38.5}}`,
	}
	return resp, nil
}

// SimulationSend 发送模拟数据
func (*TelemetryData) SimulationSend(req *model.SimulationSendReq, claims *utils.UserClaims) error {
	if req == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "请求不能为空")
	}
	// 参数校验
	if req.DeviceID == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "device_id 不能为空")
	}
	if req.Data == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "data 不能为空")
	}

	// 校验 data 是否为有效 JSON
	if !IsJSON(req.Data) {
		return errcode.NewWithMessage(errcode.CodeParamError, "data 必须是有效 JSON")
	}

	// 获取设备信息
	deviceInfo, err := ensureTelemetryDeviceWriteAccess(req.DeviceID, claims)
	if err != nil {
		return err
	}
	if deviceInfo == nil {
		return errcode.New(204003)
	}

	// 解析 voucher
	voucher := deviceInfo.Voucher
	if voucher == "" || !IsJSON(voucher) {
		return errcode.NewWithMessage(errcode.CodeParamError, "设备凭证无效")
	}
	var voucherMap map[string]interface{}
	if err := json.Unmarshal([]byte(voucher), &voucherMap); err != nil {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	username, password, err := simulationVoucherCredentials(voucherMap)
	if err != nil {
		return err
	}

	// 确定 MQTT 参数
	accessAddress := viper.GetString("mqtt.broker")
	host, port, err := parseMQTTAccessAddress(accessAddress)
	if err != nil {
		return err
	}
	if innerBroker := os.Getenv("AETHERLINK_MQTT_INNER_BROKER"); innerBroker != "" {
		switch host {
		case "localhost", "127.0.0.1", "::1":
			host = innerBroker
		}
	}
	if err := validateSimulationPublishTarget(viper.GetBool("mqtt.enabled"), config.MqttConfig.Telemetry.SubscribeTopic); err != nil {
		return errcode.WithVars(500007, map[string]interface{}{
			"error_message": err.Error(),
		})
	}

	host, port, topic, err := resolveSimulationSendTarget(req, host, port, config.MqttConfig.Telemetry.SubscribeTopic)
	if err != nil {
		return err
	}

	// 生成客户端ID
	clientID := "mqtt_" + uuid.New()[0:12]

	// 发送 MQTT 消息
	logrus.WithField("payload_size", len(req.Data)).Debug("publishing telemetry simulation data")
	err = simulationpublish.PublishMessage(host, port, topic, req.Data, username, password, clientID)
	if err != nil {
		return errcode.WithVars(500007, map[string]interface{}{
			"error_message": err.Error(),
		})
	}
	return nil
}

func telemetryPublishLogFields(params *utils.MQTTParams) logrus.Fields {
	fields := logrus.Fields{}
	if params == nil {
		return fields
	}
	fields["host"] = params.Host
	fields["port"] = params.Port
	fields["topic"] = params.Topic
	fields["client_id"] = params.ClientId
	fields["payload_size"] = len(params.Payload)
	return fields
}

func parseMQTTAccessAddress(accessAddress string) (string, string, error) {
	accessAddress = strings.TrimSpace(accessAddress)
	if accessAddress == "" {
		return "", "", errcode.NewWithMessage(errcode.CodeParamError, "未配置 MQTT access_address")
	}
	addressParts := strings.Split(accessAddress, ":")
	if len(addressParts) != 2 || strings.TrimSpace(addressParts[0]) == "" || strings.TrimSpace(addressParts[1]) == "" {
		return "", "", errcode.NewWithMessage(errcode.CodeParamError, "MQTT access_address 必须是 host:port")
	}
	port, err := strconv.Atoi(strings.TrimSpace(addressParts[1]))
	if err != nil || port <= 0 || port > 65535 {
		return "", "", errcode.NewWithMessage(errcode.CodeParamError, "MQTT access_address 端口无效")
	}
	return strings.TrimSpace(addressParts[0]), strconv.Itoa(port), nil
}

func simulationVoucherCredentials(voucherMap map[string]interface{}) (string, string, error) {
	if voucherMap == nil {
		return "", "", errcode.NewWithMessage(errcode.CodeParamError, "设备凭证为空")
	}
	username, ok := voucherMap["username"].(string)
	username = strings.TrimSpace(username)
	if !ok || username == "" {
		return "", "", errcode.NewWithMessage(errcode.CodeParamError, "设备凭证中缺少 username")
	}
	password, _ := voucherMap["password"].(string)
	return username, password, nil
}

func resolveSimulationSendTarget(
	req *model.SimulationSendReq,
	defaultHost, defaultPort, defaultTopic string,
) (string, string, string, error) {
	host := defaultHost
	port := defaultPort
	topic := defaultTopic
	if req == nil {
		return host, port, topic, nil
	}

	if trimmedServer := strings.TrimSpace(req.Server); trimmedServer != "" {
		host = trimmedServer
	}
	if req.Port != nil {
		if *req.Port <= 0 || *req.Port > 65535 {
			return "", "", "", errcode.NewWithMessage(errcode.CodeParamError, "MQTT 端口无效")
		}
		port = strconv.Itoa(*req.Port)
	}
	if trimmedTopic := strings.TrimSpace(req.Topic); trimmedTopic != "" {
		topic = trimmedTopic
	}
	return host, port, topic, nil
}

func validateSimulationPublishTarget(enabled bool, topic string) error {
	if !enabled {
		return fmt.Errorf("mqtt.enabled must be true before publishing simulated telemetry")
	}
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("MQTT telemetry topic is not initialized; enable MQTT service before publishing simulated telemetry")
	}
	return nil
}

func simulationSendLogFields(host, port, topic, clientID, payload string) logrus.Fields {
	return logrus.Fields{
		"host":         host,
		"port":         port,
		"topic":        topic,
		"client_id":    clientID,
		"payload_size": len(payload),
	}
}
