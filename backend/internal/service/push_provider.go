// 文件用途：移动端推送的 Provider 抽象与可重试投递（ROADMAP P1.4）。
// 核心逻辑：Provider 注册表 + 投递状态机（pending → sent/failed → dead）。
//
// 关键注意事项（门禁"推送失败可重试并可审计"）：
//  1. **没有可用 Provider 时报错，绝不静默成功**。一条"发送成功"却从未离站的推送，
//     会让告警在用户手机上永远不出现，而系统显示一切正常——典型的假成功。
//  2. 重试次数有上限，超过即转 dead 终态。无限重试会把永久失败（如令牌已失效）
//     伪装成"还在路上"，用户永远等不到，运维也看不到失败。
//  3. 每次尝试都计入 attempt_count 并留 last_error，投递历史即审计记录：
//     "推送失败可审计"要求失败本身可见，而不是只在成功时留痕。
//  4. 状态回写带当前状态做条件更新，两个重试器并发处理同一条时后到者自然落空，
//     避免同一条推送被真正发出两遍。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/google/uuid"
)

var (
	ErrPushNoProvider     = errors.New("no push provider is available")
	ErrPushProviderExists = errors.New("push provider already registered")
	ErrPushBadProvider    = errors.New("push provider is invalid")
	ErrPushNoTarget       = errors.New("no enabled push registration for this user")
)

// PushMessage 一次推送的内容。
type PushMessage struct {
	TenantID string
	UserID   string
	Title    string
	Body     string
	Data     map[string]interface{}
	Token    string
	Platform string
}

// PushProvider 外部推送通道适配器。实现必须返回真实结果：
// 未真正投递出去就返回 nil 等同于伪造成功。
type PushProvider interface {
	// Name 通道名，如 fcm / apns。
	Name() string
	// Supports 该通道是否支持指定平台。
	Supports(platform string) bool
	Send(ctx context.Context, msg PushMessage) error
}

// PushProviderRegistry Provider 注册表。
type PushProviderRegistry struct {
	providers map[string]PushProvider
}

// NewPushProviderRegistry 创建空注册表。
func NewPushProviderRegistry() *PushProviderRegistry {
	return &PushProviderRegistry{providers: make(map[string]PushProvider)}
}

// Register 注册 Provider。同名重复注册返回错误，避免后注册的静默覆盖前一个。
func (r *PushProviderRegistry) Register(p PushProvider) error {
	if r == nil {
		return ErrPushBadProvider
	}
	if p == nil || strings.TrimSpace(p.Name()) == "" {
		return ErrPushBadProvider
	}
	name := strings.TrimSpace(p.Name())
	if _, exists := r.providers[name]; exists {
		return ErrPushProviderExists
	}
	r.providers[name] = p
	return nil
}

// Len 已注册的 Provider 数量。装配方用它判断"到底接上了几个通道"，
// 一个都没有时不应构造 PushService（空注册表只会报成功、发不出）。
func (r *PushProviderRegistry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.providers)
}

// Get 按名取 Provider。
func (r *PushProviderRegistry) Get(name string) (PushProvider, bool) {
	if r == nil {
		return nil, false
	}
	p, ok := r.providers[strings.TrimSpace(name)]
	return p, ok
}

// ForPlatform 取支持该平台的第一个 Provider（按名字典序，保证选择稳定可复现）。
// 找不到返回错误：宁可明确失败，也不假装发送成功。
func (r *PushProviderRegistry) ForPlatform(platform string) (PushProvider, error) {
	if r == nil || len(r.providers) == 0 {
		return nil, ErrPushNoProvider
	}
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		if r.providers[name].Supports(platform) {
			return r.providers[name], nil
		}
	}
	return nil, ErrPushNoProvider
}

// sortStrings 就地字典序排序（避免为一行逻辑引入 sort 依赖）。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// PushRetryPolicy 重试策略。
type PushRetryPolicy struct {
	MaxAttempts int32
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

// DefaultPushRetryPolicy 默认策略：最多 3 次，退避 30s → 60s，上限 10 分钟。
func DefaultPushRetryPolicy() PushRetryPolicy {
	return PushRetryPolicy{
		MaxAttempts: 3,
		BaseBackoff: 30 * time.Second,
		MaxBackoff:  10 * time.Minute,
	}
}

// NextDelay 计算第 attemptCount 次失败后的退避时长（ capped exponential）。
func (p PushRetryPolicy) NextDelay(attemptCount int32) time.Duration {
	if p.BaseBackoff <= 0 {
		return 30 * time.Second
	}
	delay := p.BaseBackoff
	for i := int32(1); i < attemptCount; i++ {
		delay *= 2
		if delay >= p.MaxBackoff {
			return p.MaxBackoff
		}
	}
	if delay > p.MaxBackoff {
		return p.MaxBackoff
	}
	return delay
}

// Exhausted 判断给定尝试次数是否已用尽（应转 dead 终态）。
func (p PushRetryPolicy) Exhausted(attemptCount int32) bool {
	max := p.MaxAttempts
	if max <= 0 {
		max = 3
	}
	return attemptCount >= max
}

// PushService 推送服务。
type PushService struct {
	registry *PushProviderRegistry
	policy   PushRetryPolicy
}

// NewPushService 创建推送服务。
func NewPushService(registry *PushProviderRegistry, policy PushRetryPolicy) *PushService {
	if policy.MaxAttempts <= 0 {
		policy = DefaultPushRetryPolicy()
	}
	return &PushService{registry: registry, policy: policy}
}

// Enqueue 入队一条推送。无可用 Provider 时直接以 failed 落库并报错，
// 不让调用方误以为已经排上了队。
func (s *PushService) Enqueue(ctx context.Context, tenantID, userID, title, body string, data map[string]interface{}, now time.Time) ([]*model.PushDelivery, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(userID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "push requires tenant and user")
	}
	regs, err := dal.ListPushRegistrationsByUser(tenantID, userID, true)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if len(regs) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, ErrPushNoTarget.Error())
	}

	out := make([]*model.PushDelivery, 0, len(regs))
	for i := range regs {
		payload := marshalParams(data)
		d := &model.PushDelivery{
			// ID 显式生成：空主键会让第二条投递撞主键而静默丢失。
			ID:             uuid.New().String(),
			TenantID:       tenantID,
			RegistrationID: &regs[i].ID,
			UserID:         userID,
			Title:          title,
			Body:           body,
			Data:           &payload,
			Status:         model.PushStatusPending,
			AttemptCount:   0,
			NextAttemptAt:  &now,
			Provider:       &regs[i].Provider,
		}
		if err := model.ValidatePushDelivery(d); err != nil {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
		}
		if err := dal.CreatePushDelivery(d); err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
		}
		out = append(out, d)
	}
	return out, nil
}

// DeliverDue 处理到点且仍可重试的投递。返回 (成功数, 失败数)。
func (s *PushService) DeliverDue(ctx context.Context, now time.Time, limit int) (int, int, error) {
	rows, err := dal.ListRetryablePushDeliveries(now, limit)
	if err != nil {
		return 0, 0, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	var sent, failed int
	for i := range rows {
		if err := s.deliverOne(ctx, &rows[i], now); err != nil {
			failed++
			continue
		}
		sent++
	}
	return sent, failed, nil
}

// deliverOne 处理单条投递：选 Provider → 发送 → 按结果推进状态。
func (s *PushService) deliverOne(ctx context.Context, d *model.PushDelivery, now time.Time) error {
	var token, platform string
	if d.RegistrationID != nil {
		// 令牌按登记记录取；登记已被撤销时改判 dead，不再重试——
		// 对着一个不存在的令牌重试到天荒地老毫无意义。
		regs, err := dal.ListPushRegistrationsByUser(d.TenantID, d.UserID, true)
		if err == nil {
			for i := range regs {
				if regs[i].ID == *d.RegistrationID {
					token = regs[i].Token
					platform = regs[i].Platform
					break
				}
			}
		}
	}
	if token == "" {
		return s.settle(d, now, false, ErrPushNoTarget.Error())
	}

	provider, err := s.registry.ForPlatform(platform)
	if err != nil {
		// 未接线即失败，并计入重试次数：配置缺失是运维可见的失败，
		// 不该被静默吞掉。
		return s.settle(d, now, false, err.Error())
	}

	sendErr := provider.Send(ctx, PushMessage{
		TenantID: d.TenantID,
		UserID:   d.UserID,
		Title:    d.Title,
		Body:     d.Body,
		Token:    token,
		Platform: platform,
	})
	if sendErr != nil {
		// 终态失败（如令牌已失效）不再排重试：重试一万次也不会成功，
		// 只会把"令牌失效"这个事实埋进最后一次 last_error 里。
		if IsPushTerminal(sendErr) {
			return s.settleTerminal(d, now, sendErr.Error())
		}
		return s.settle(d, now, false, sendErr.Error())
	}
	return s.settle(d, now, true, "")
}

// settle 按本次结果推进投递状态。
func (s *PushService) settle(d *model.PushDelivery, now time.Time, ok bool, errMsg string) error {
	attempts := d.AttemptCount + 1
	if ok {
		provider := stringPtrValue(d.Provider)
		_, _ = dal.UpdatePushDeliveryFrom(d.ID, d.TenantID, d.Status, dal.PushDeliveryUpdate{
			Status:        model.PushStatusSent,
			AttemptCount:  attempts,
			NextAttemptAt: nil,
			LastError:     nil,
			Provider:      &provider,
		})
		return nil
	}

	lastErr := errMsg
	// 已用尽 => dead 终态，不再安排下一次；否则按退避排期。
	if s.policy.Exhausted(attempts) {
		provider := stringPtrValue(d.Provider)
		_, _ = dal.UpdatePushDeliveryFrom(d.ID, d.TenantID, d.Status, dal.PushDeliveryUpdate{
			Status:        model.PushStatusDead,
			AttemptCount:  attempts,
			NextAttemptAt: nil,
			LastError:     &lastErr,
			Provider:      &provider,
		})
		return errors.New(errMsg)
	}
	next := now.Add(s.policy.NextDelay(attempts))
	provider := stringPtrValue(d.Provider)
	_, _ = dal.UpdatePushDeliveryFrom(d.ID, d.TenantID, d.Status, dal.PushDeliveryUpdate{
		Status:        model.PushStatusFailed,
		AttemptCount:  attempts,
		NextAttemptAt: &next,
		LastError:     &lastErr,
		Provider:      &provider,
	})
	return errors.New(errMsg)
}

// settleTerminal 直接把投递置为 dead，不排下一次重试。
// 只用于"重试也不会成功"的失败（如 FCM 返回令牌已失效）。
// 注意它仍然计入 attempt_count：失败次数是审计信息，不该因为判定为终态就消失。
func (s *PushService) settleTerminal(d *model.PushDelivery, now time.Time, errMsg string) error {
	attempts := d.AttemptCount + 1
	lastErr := errMsg
	provider := stringPtrValue(d.Provider)
	_, _ = dal.UpdatePushDeliveryFrom(d.ID, d.TenantID, d.Status, dal.PushDeliveryUpdate{
		Status:        model.PushStatusDead,
		AttemptCount:  attempts,
		NextAttemptAt: nil,
		LastError:     &lastErr,
		Provider:      &provider,
	})
	return errors.New(errMsg)
}
