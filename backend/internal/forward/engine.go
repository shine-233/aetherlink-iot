// 文件用途：数据转发引擎——订阅上行总线，按启用规则把设备数据投递到第三方 HTTP/MQTT。
// 核心逻辑：SubscribeAcceptedMessages 消费 *DeviceMessage；按 Type 匹配规则
// （telemetry/property(attribute)/event/status(device_online)），可选模板过滤经设备行懒查缓存；
// 投递 HTTP POST（10s 超时）或 MQTT（复用 simulationpublish 每次拨号，Phase 2 池化挂账）。
// 关键注意事项：单 worker + 带缓冲队列，队列满即丢弃并计数（不阻塞上行主链路）；
// 规则缓存 30s TTL；脚本转换 Phase 1 直通（沙箱接入点见 backend-hardening-plan.md）。

package forward

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/uplink"
	global "aetherlink-iot/backend/pkg/global"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

const (
	forwardEngineQueueSize    = 512
	forwardRuleCacheTTL       = 30 * time.Second
	forwardHTTPClientTimeout  = 10 * time.Second
	forwardMQTTPublishTimeout = 5 * time.Second
)

// ForwardDispatchPublisher MQTT 投递 seam：便于单测注入假发布器。
var ForwardDispatchPublisher = func(broker, topic, username, password, payload string) error {
	opts := mqtt.NewClientOptions().AddBroker(broker)
	if username != "" {
		opts.SetUsername(username)
	}
	if password != "" {
		opts.SetPassword(password)
	}
	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(forwardMQTTPublishTimeout) || token.Error() != nil {
		return fmt.Errorf("mqtt connect failed: %v", token.Error())
	}
	pub := client.Publish(topic, 0, false, []byte(payload))
	ok := pub.WaitTimeout(forwardMQTTPublishTimeout)
	client.Disconnect(200)
	if !ok || pub.Error() != nil {
		return fmt.Errorf("mqtt publish failed: %v", pub.Error())
	}
	return nil
}

type forwardTask struct {
	msg   *uplink.DeviceMessage
	rules []*model.ForwardRule
}

// ForwardRuleEngine 数据转发引擎。
type ForwardEngine struct {
	bus        *uplink.Bus
	logger     *logrus.Logger
	queue      chan forwardTask
	httpClient *http.Client

	mu         sync.Mutex
	cacheBySrc map[string][]*model.ForwardRule
	cacheAt    map[string]time.Time

	templateMu    sync.Mutex
	templateCache map[string]cachedTemplateID
	droppedCount  uint64
	stopCh        chan struct{}
	stoppedCh     chan struct{}
	started       bool
}

type cachedTemplateID struct {
	id       string
	cachedAt time.Time
	exists   bool
}

// NewForwardEngine 构造引擎（不启动；Start 订阅总线）。
func NewForwardEngine(bus *uplink.Bus, logger *logrus.Logger) *ForwardEngine {
	return &ForwardEngine{
		bus:           bus,
		logger:        logger,
		queue:         make(chan forwardTask, forwardEngineQueueSize),
		httpClient:    &http.Client{Timeout: forwardHTTPClientTimeout},
		cacheBySrc:    make(map[string][]*model.ForwardRule),
		cacheAt:       make(map[string]time.Time),
		templateCache: make(map[string]cachedTemplateID),
		stopCh:        make(chan struct{}),
		stoppedCh:     make(chan struct{}),
	}
}

func forwardSourceTypesFor(msgType string) string {
	switch msgType {
	case "telemetry":
		return "telemetry"
	case "attribute":
		return "property"
	case "event":
		return "event"
	case "status", "device_online", "device_offline":
		return "status"
	}
	return ""
}

// Start 订阅总线并启动投递 worker。重复调用幂等。
func (e *ForwardEngine) Start() error {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return nil
	}
	e.started = true
	e.mu.Unlock()

	subscription, err := e.bus.SubscribeAcceptedMessages(forwardEngineQueueSize)
	if err != nil {
		e.started = false
		return err
	}
	go func() {
		defer close(e.stoppedCh)
		for {
			select {
			case <-e.stopCh:
				subscription.Close()
				return
			case msg, ok := <-subscription.Messages:
				if !ok || msg == nil {
					continue
				}
				source := forwardSourceTypesFor(msg.Type)
				if source == "" {
					continue
				}
				rules := e.matchingRules(source, msg)
				if len(rules) == 0 {
					continue
				}
				select {
				case e.queue <- forwardTask{msg: msg, rules: rules}:
				default:
					e.mu.Lock()
					e.droppedCount++
					e.mu.Unlock()
					e.logger.Warn("forward engine queue full; task dropped")
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-e.stopCh:
				return
			case task := <-e.queue:
				payload := string(task.msg.Payload)
				for _, rule := range task.rules {
					e.dispatch(rule, task.msg, payload)
				}
			}
		}
	}()
	e.logger.Info("forward rule engine started")
	return nil
}

// Stop 停止引擎（有界等待消费协程退出）。
func (e *ForwardEngine) Stop() error {
	e.mu.Lock()
	if !e.started {
		e.mu.Unlock()
		return nil
	}
	e.started = false
	close(e.stopCh)
	e.mu.Unlock()
	select {
	case <-e.stoppedCh:
	case <-time.After(3 * time.Second):
		e.logger.Warn("forward engine stop wait timeout")
	}
	return nil
}

// matchingRules 取缓存规则并按可选模板过滤。
func (e *ForwardEngine) matchingRules(source string, msg *uplink.DeviceMessage) []*model.ForwardRule {
	now := time.Now()
	e.mu.Lock()
	rules, hasCache := e.cacheBySrc[source]
	at, hasAt := e.cacheAt[source]
	fresh := !hasCache || !hasAt || now.Sub(at) >= forwardRuleCacheTTL
	e.mu.Unlock()

	if fresh {
		rows, err := dal.ListEnabledForwardRules(source)
		if err != nil {
			e.logger.WithError(err).Warn("load enabled forward rules failed; using stale cache if any")
		} else {
			e.mu.Lock()
			e.cacheBySrc[source] = rows
			e.cacheAt[source] = now
			e.mu.Unlock()
			rules = rows
		}
	}

	out := make([]*model.ForwardRule, 0, len(rules))
	for _, rule := range rules {
		if rule.DeviceTemplateID == nil || *rule.DeviceTemplateID == "" {
			out = append(out, rule)
			continue
		}
		if templateID, ok := e.deviceTemplateID(msg.DeviceID); ok && templateID == *rule.DeviceTemplateID {
			out = append(out, rule)
		}
	}
	return out
}

// deviceTemplateID 懒查设备模板归属，带 60s 缓存；查询失败按不存在处理。
func (e *ForwardEngine) deviceTemplateID(deviceID string) (string, bool) {
	if global.DB == nil {
		return "", false
	}
	e.templateMu.Lock()
	if hit, ok := e.templateCache[deviceID]; ok && time.Since(hit.cachedAt) < 60*time.Second {
		e.templateMu.Unlock()
		return hit.id, hit.exists
	}
	e.templateMu.Unlock()

	templateID, exists := "", false
	var row struct{ DeviceTemplateID *string }
	if err := global.DB.Table("devices").
		Select("device_template_id").
		Where("id = ?", deviceID).
		Scan(&row).Error; err == nil && row.DeviceTemplateID != nil && *row.DeviceTemplateID != "" {
		templateID, exists = *row.DeviceTemplateID, true
	}

	e.templateMu.Lock()
	e.templateCache[deviceID] = cachedTemplateID{id: templateID, cachedAt: time.Now(), exists: exists}
	e.templateMu.Unlock()
	return templateID, exists
}

// dispatch 单规则投递。
func (e *ForwardEngine) dispatch(rule *model.ForwardRule, msg *uplink.DeviceMessage, payload string) {
	defer func() {
		if r := recover(); r != nil {
			e.logger.WithField("rule_id", rule.ID).Error("forward dispatch panic recovered")
		}
	}()
	switch rule.TargetType {
	case "http":
		method := "POST"
		if rule.HttpMethod != nil && strings.TrimSpace(*rule.HttpMethod) != "" {
			method = strings.ToUpper(*rule.HttpMethod)
		}
		req, err := http.NewRequest(method, deref(rule.HttpURL), bytes.NewBufferString(payload))
		if err != nil {
			e.logger.WithError(err).WithField("rule_id", rule.ID).Warn("build forward http request failed")
			return
		}
		req.Header.Set("Content-Type", "application/json")
		applyForwardHTTPHeaders(req.Header, rule.HttpHeaders)
		resp, err := e.httpClient.Do(req)
		if err != nil {
			e.logger.WithError(err).WithField("rule_id", rule.ID).Warn("forward http delivery failed")
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			e.logger.WithFields(logrus.Fields{"rule_id": rule.ID, "status": resp.StatusCode}).
				Warn("forward http delivery got non-success status")
		}
	case "mqtt":
		broker := deref(rule.MqttBroker)
		topic := deref(rule.MqttTopic)
		user := deref(rule.MqttUsername)
		pass := deref(rule.MqttPassword)
		if err := ForwardDispatchPublisher(broker, topic, user, pass, payload); err != nil {
			e.logger.WithError(err).WithField("rule_id", rule.ID).Warn("forward mqtt delivery failed")
		}
	}
}

func applyForwardHTTPHeaders(header http.Header, rawHeaders *string) {
	if rawHeaders == nil || strings.TrimSpace(*rawHeaders) == "" {
		return
	}
	pairs := map[string]string{}
	if err := json.Unmarshal([]byte(*rawHeaders), &pairs); err != nil {
		return
	}
	for key, value := range pairs {
		header.Set(key, value)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// ForwardEngineServiceWrapper 包装为可注册 Service。
type ForwardEngineServiceWrapper struct {
	Engine *ForwardEngine
}

func (*ForwardEngineServiceWrapper) Name() string { return "数据转发引擎" }
func (w *ForwardEngineServiceWrapper) Start() error {
	if w.Engine == nil {
		return nil
	}
	return w.Engine.Start()
}
func (w *ForwardEngineServiceWrapper) Stop() error {
	return w.Engine.Stop()
}
