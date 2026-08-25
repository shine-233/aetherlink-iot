// 文件用途：计算字段引擎——订阅上行总线的遥测消息，按设备模板的启用字段派生新遥测并写回存储链路。
// 核心逻辑：SubscribeAcceptedMessages 扇出消费（不抢占主流量）；payload 只保留数值/布尔参与运算；
// govaluate 求值结果仅接受数值/布尔/字符串；产物构造成 DeviceMessage{Type:"telemetry"} 交由
// StorageEnqueuer seam 写回正常存储链路。
// 关键注意事项：
// 1. 防环：Metadata["calcfield_generated"]==true 的消息直接跳过；本引擎产物一律打上该标记。
// 2. 缓存：设备→模板归属懒查缓存 60s；模板启用字段清单缓存 30s（启停开关最迟 30s 生效）。
// 3. 写回队列满时丢弃并计数（DroppedCount），绝不反压设备上行主链路。
// 重构建议：若后续需要多规则共享中间变量或窗口聚合，把 compiledRule 求值改成带上下文的批次接口。
package calcfield

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/uplink"

	"github.com/casbin/govaluate"
	"github.com/sirupsen/logrus"
)

// MetadataGeneratedFlag 标记"该消息由计算字段引擎产出"。再次进入引擎的消息直接跳过，防止自激循环。
const MetadataGeneratedFlag = "calcfield_generated"

const (
	deviceTemplateCacheTTL = 60 * time.Second
	fieldRulesCacheTTL     = 30 * time.Second
	engineObserverBuffer   = 1024
)

// StorageEnqueuer 是计算字段产物的写回 seam。生产环境在 app 层用 GetStorageInput()
// 返回值适配到 storage.DurableMessageInput；测试用内存实现。
type StorageEnqueuer interface {
	// EnqueueDerivedTelemetry 把派生遥测写入正常存储链路；false 表示被拒绝或写回队列已满。
	EnqueueDerivedTelemetry(ctx context.Context, msg *uplink.DeviceMessage) bool
}

// FieldRule 是一条参与求值的启用字段。
type FieldRule struct {
	ID         string
	OutputKey  string
	Expression string
}

// TemplateSource 提供设备→模板归属与模板启用字段两类查询，便于单测注入桩实现。
type TemplateSource interface {
	ResolveTemplateID(ctx context.Context, deviceID string) (string, error)
	ListEnabledFields(ctx context.Context, templateID string) ([]FieldRule, error)
}

// dalTemplateSource 默认实现：查询走 internal/dal 的 raw 链。
type dalTemplateSource struct{}

// ResolveTemplateID 解析设备归属模板 id；未绑定配置/模板返回空串。
func (dalTemplateSource) ResolveTemplateID(_ context.Context, deviceID string) (string, error) {
	return dal.GetDeviceTemplateIDByDeviceID(deviceID)
}

// ListEnabledFields 返回模板下全部启用计算字段。
func (dalTemplateSource) ListEnabledFields(_ context.Context, templateID string) ([]FieldRule, error) {
	fields, err := dal.ListEnabledCalculatedFieldsByTemplate(templateID)
	if err != nil {
		return nil, err
	}
	rules := make([]FieldRule, 0, len(fields))
	for _, field := range fields {
		rules = append(rules, FieldRule{ID: field.ID, OutputKey: field.OutputKey, Expression: field.Expression})
	}
	return rules, nil
}

// compiledRule 是编译后的字段规则；表达式解析失败的字段在装载时跳过并记录日志。
type compiledRule struct {
	id        string
	outputKey string
	expr      *govaluate.EvaluableExpression
	variables []string
}

type cachedTemplate struct {
	templateID string
	expiresAt  time.Time
}

type cachedRules struct {
	rules     []compiledRule
	expiresAt time.Time
}

// Engine 计算字段引擎。
type Engine struct {
	bus            *uplink.Bus
	storage        StorageEnqueuer
	logger         *logrus.Logger
	templateSource TemplateSource

	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once

	dropped atomic.Uint64

	cacheMu       sync.RWMutex
	templateCache map[string]cachedTemplate
	rulesCache    map[string]cachedRules
}

// NewEngine 创建计算字段引擎。调用方需保证 storage 非 nil；logger 为空时回退标准 logger。
func NewEngine(bus *uplink.Bus, storage StorageEnqueuer, logger *logrus.Logger) *Engine {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		bus:            bus,
		storage:        storage,
		logger:         logger,
		templateSource: dalTemplateSource{},
		ctx:            ctx,
		cancel:         cancel,
		done:           make(chan struct{}),
		templateCache:  make(map[string]cachedTemplate),
		rulesCache:     make(map[string]cachedRules),
	}
}

// Start 订阅上行总线扇出流并开始消费。重复调用是幂等的。
func (e *Engine) Start() error {
	var startErr error
	e.startOnce.Do(func() {
		if e.bus == nil || e.storage == nil {
			startErr = fmt.Errorf("calcfield engine requires bus and storage enqueuer")
			return
		}
		subscription, err := e.bus.SubscribeAcceptedMessages(engineObserverBuffer)
		if err != nil {
			startErr = fmt.Errorf("subscribe accepted messages for calcfield engine: %w", err)
			return
		}
		go e.consume(subscription)
		e.logger.Info("Calcfield engine started")
	})
	return startErr
}

// Stop 停止消费并等待消费协程退出。
func (e *Engine) Stop() error {
	e.stopOnce.Do(func() {
		e.cancel()
	})
	<-e.done
	return nil
}

// DroppedCount 返回因写回队列满而被丢弃的派生遥测条数。
func (e *Engine) DroppedCount() uint64 { return e.dropped.Load() }

func (e *Engine) consume(subscription *uplink.AcceptedMessageSubscription) {
	defer close(e.done)
	defer subscription.Close()
	for {
		select {
		case <-e.ctx.Done():
			return
		case msg, ok := <-subscription.Messages:
			if !ok {
				return
			}
			e.processMessage(msg)
		}
	}
}

// processMessage 处理单条上行消息：只关心遥测类型，且跳过自身产出的消息。
func (e *Engine) processMessage(msg *uplink.DeviceMessage) {
	if msg == nil || msg.Type != uplink.MessageTypeTelemetry {
		return
	}
	// 防环：计算字段产出的消息不再参与计算。
	if generated, exists := msg.GetMetadata(MetadataGeneratedFlag); exists {
		if flag, isBool := generated.(bool); isBool && flag {
			return
		}
	}

	payload := decodeFlatPayload(msg.Payload)
	if len(payload) == 0 {
		return
	}

	templateID := e.resolveTemplateIDCached(msg.DeviceID)
	if templateID == "" {
		return
	}

	rules := e.listRulesCached(templateID)
	if len(rules) == 0 {
		return
	}

	timestamp := msg.Timestamp
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}
	for _, rule := range rules {
		value, ok := evaluateRule(rule, payload)
		if !ok {
			continue
		}
		e.enqueueDerived(msg, rule.outputKey, value, timestamp)
	}
}

// decodeFlatPayload 把遥测 JSON 解析为扁平 map，仅保留数值与布尔值参与运算。
func decodeFlatPayload(raw []byte) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded == nil {
		return nil
	}
	flat := make(map[string]interface{}, len(decoded))
	for key, value := range decoded {
		switch typed := value.(type) {
		case float64:
			flat[key] = typed
		case bool:
			flat[key] = typed
		default:
			// 字符串、嵌套对象、数组等不参与表达式运算。
		}
	}
	return flat
}

// evaluateRule 对单条规则求值：缺变量跳过，结果只接受数值/布尔/字符串。
func evaluateRule(rule compiledRule, payload map[string]interface{}) (interface{}, bool) {
	params := make(map[string]interface{}, len(rule.variables))
	for _, variable := range rule.variables {
		value, exists := payload[variable]
		if !exists {
			return nil, false
		}
		params[variable] = value
	}
	result, err := rule.expr.Evaluate(params)
	if err != nil {
		return nil, false
	}
	switch typed := result.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case bool:
		return typed, true
	case string:
		return typed, true
	default:
		return nil, false
	}
}

// enqueueDerived 构造派生遥测消息并写回存储 seam；失败计入丢弃计数。
func (e *Engine) enqueueDerived(source *uplink.DeviceMessage, outputKey string, value interface{}, timestamp int64) {
	payload, err := json.Marshal(map[string]interface{}{outputKey: value})
	if err != nil {
		e.dropped.Add(1)
		e.logger.WithFields(logrus.Fields{
			"device_id":  source.DeviceID,
			"output_key": outputKey,
		}).WithError(err).Warn("Failed to marshal calcfield payload")
		return
	}

	metadata := map[string]interface{}{MetadataGeneratedFlag: true}
	if source.TenantID != "" {
		metadata["tenant_id"] = source.TenantID
	}

	derived := &uplink.DeviceMessage{
		Type:      uplink.MessageTypeTelemetry,
		DeviceID:  source.DeviceID,
		TenantID:  source.TenantID,
		Timestamp: timestamp,
		Payload:   payload,
		Metadata:  metadata,
	}
	if !e.storage.EnqueueDerivedTelemetry(e.ctx, derived) {
		e.dropped.Add(1)
		e.logger.WithFields(logrus.Fields{
			"device_id":  source.DeviceID,
			"output_key": outputKey,
		}).Warn("Storage queue full or unavailable, calcfield result dropped")
	}
}

// resolveTemplateIDCached 设备→模板归属懒查，60s 缓存；查询失败不缓存。
func (e *Engine) resolveTemplateIDCached(deviceID string) string {
	now := time.Now()
	e.cacheMu.RLock()
	cached, exists := e.templateCache[deviceID]
	e.cacheMu.RUnlock()
	if exists && now.Before(cached.expiresAt) {
		return cached.templateID
	}

	templateID, err := e.templateSource.ResolveTemplateID(e.ctx, deviceID)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"error":     err,
		}).Debug("Calcfield template lookup failed")
		return ""
	}

	e.cacheMu.Lock()
	e.templateCache[deviceID] = cachedTemplate{templateID: templateID, expiresAt: now.Add(deviceTemplateCacheTTL)}
	e.cacheMu.Unlock()
	return templateID
}

// listRulesCached 模板启用字段清单缓存，30s TTL；每次刷新重新解析表达式。
func (e *Engine) listRulesCached(templateID string) []compiledRule {
	now := time.Now()
	e.cacheMu.RLock()
	cached, exists := e.rulesCache[templateID]
	e.cacheMu.RUnlock()
	if exists && now.Before(cached.expiresAt) {
		return cached.rules
	}

	fields, err := e.templateSource.ListEnabledFields(e.ctx, templateID)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"template_id": templateID,
			"error":       err,
		}).Debug("Calcfield enabled fields lookup failed")
		return nil
	}

	rules := compileFieldRules(fields, e.logger)
	e.cacheMu.Lock()
	e.rulesCache[templateID] = cachedRules{rules: rules, expiresAt: now.Add(fieldRulesCacheTTL)}
	e.cacheMu.Unlock()
	return rules
}

// compileFieldRules 预编译表达式；解析失败的规则跳过（服务层已在保存时拦截）。
func compileFieldRules(fields []FieldRule, logger *logrus.Logger) []compiledRule {
	rules := make([]compiledRule, 0, len(fields))
	for _, field := range fields {
		expr, err := govaluate.NewEvaluableExpression(field.Expression)
		if err != nil {
			if logger != nil {
				logger.WithFields(logrus.Fields{
					"field_id": field.ID,
					"error":    err,
				}).Warn("Calcfield expression failed to compile; skipping rule")
			}
			continue
		}
		rules = append(rules, compiledRule{
			id:        field.ID,
			outputKey: field.OutputKey,
			expr:      expr,
			variables: expr.Vars(),
		})
	}
	return rules
}
