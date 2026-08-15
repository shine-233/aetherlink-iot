package publish

import (
	"errors"
	"fmt"
	"strings"
	"time"

	config "aetherlink-iot/backend/mqtt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

var mqttClient mqtt.Client

var (
	ErrPublisherUnavailable = errors.New("mqtt publisher is unavailable")
	ErrPublishTimeout       = errors.New("mqtt publish timed out with unknown delivery outcome")
)

const defaultOTAPublishTimeout = 5 * time.Second

// CreateMqttClient builds and connects the shared MQTT publisher client.
func CreateMqttClient() {
	opts := buildPublishClientOptions(config.NewShortClientID("aetherlink-go-pub-"))
	configurePublisherClientCallbacks(opts)

	mqttClient = mqtt.NewClient(opts)
	for {
		if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
			logrus.Error("MQTT Broker 1 connection failed:", token.Error())
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}
}

// configurePublisherClientCallbacks leaves reconnect ownership with Paho.
// AutoReconnect already serializes the reconnect attempt after a lost
// connection; manually calling Disconnect and Connect from OnConnectionLost
// races that state machine and can make a live publisher use a closed socket.
func configurePublisherClientCallbacks(opts *mqtt.ClientOptions) {
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		logrus.Info("mqtt connect success")
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		logrus.Warn("mqtt connect lost: ", err)
	})
}

func buildPublishClientOptions(clientID string) *mqtt.ClientOptions {
	return config.NewPahoClientOptions(config.PahoClientOptionsConfig{
		Broker:               config.MqttConfig.Broker,
		Username:             config.MqttConfig.User,
		Password:             config.MqttConfig.Pass,
		ClientID:             clientID,
		CleanSession:         true,
		ResumeSubs:           true,
		AutoReconnect:        true,
		ConnectRetryInterval: 5 * time.Second,
		MaxReconnectInterval: 20 * time.Second,
		OrderMatters:         false,
	})
}

func otaAddressTopic(deviceNumber string) string {
	return config.MqttConfig.OTA.PublishTopic + deviceNumber
}

// PublishOtaAddress sends an OTA package message to a direct device.
func PublishOtaAddress(deviceNumber string, payload []byte) error {
	return PublishOtaAddressWithTimeout(deviceNumber, payload, defaultOTAPublishTimeout)
}

// PublishOtaAddressWithTimeout waits only for the bounded broker publish
// acknowledgement. A timeout is deliberately reported as an unknown outcome:
// the shared client may still complete the publish after this method returns.
func PublishOtaAddressWithTimeout(deviceNumber string, payload []byte, timeout time.Duration) error {
	deviceNumber = strings.TrimSpace(deviceNumber)
	if deviceNumber == "" {
		return fmt.Errorf("%w: ota device number is empty", ErrPublisherUnavailable)
	}
	if timeout <= 0 {
		timeout = defaultOTAPublishTimeout
	}
	client := mqttClient
	if client == nil || !client.IsConnectionOpen() {
		return fmt.Errorf("%w: shared client is not connected", ErrPublisherUnavailable)
	}
	topic := otaAddressTopic(deviceNumber)
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("%w: ota publish topic is empty", ErrPublisherUnavailable)
	}
	qos := byte(config.MqttConfig.OTA.QoS)
	token := client.Publish(topic, qos, false, payload)
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("%w after %s", ErrPublishTimeout, timeout)
	}
	if err := token.Error(); err != nil {
		logrus.WithError(err).WithField("topic", topic).Error("MQTT OTA publish failed")
		return err
	}
	return nil
}
