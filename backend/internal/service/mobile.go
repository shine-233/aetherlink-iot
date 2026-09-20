// 文件用途：移动端能力聚合与弱网幂等命令（ROADMAP P1.4）。
// 核心逻辑：把命令/影子/告警/OTA/看板/推送聚合为 MobileApi，并强制命令走幂等键。
//
// 关键注意事项（本层刻意做成"接线驱动"，而不是先写一堆桩再假装能用）：
//  1. **能力矩阵由实际接线决定**。某个依赖没注入，Capabilities 就报 false，
//     调用该能力直接返回错误。反过来（先报 true 再说）会让移动端展示一堆
//     点了就报错的功能，是最典型的假成功。
//  2. 弱网重试不重复命令：命令必须带幂等键。命中已完成的键时**直接返回原结果，
//     不再下发**——这才是幂等的含义。若改成"命中就报错"，弱网下用户会以为
//     命令没发出去而反复点击，反而制造更多重复。
//  3. 下发失败必须释放幂等键，否则重试会被判为"已处理"而永远发不出去，
//     故障在弱网下表现为"点了没反应"，无从排查。
//  4. 有歧义的结果（如超时）按失败处理并释放，同时在收据里标注 ambiguous，
//     不谎称成功也不谎称失败。
package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/google/uuid"
)

var (
	ErrMobileNotWired        = errors.New("mobile capability is not wired")
	ErrMobileIdempotencyKey  = errors.New("idempotency key is required for mobile commands")
	ErrMobileCommandConflict = errors.New("idempotency key was reused with different parameters")
	ErrMobileBadPlatform     = errors.New("push platform is not allowed")
)

// 移动端依赖契约。未注入即代表该能力未接线。
//
// 这些契约一律收 **完整的 *utils.UserClaims**，而不是拆成 (tenantID, userID)：
// 归属过滤由 claims.Authority 决定（TENANT_USER 只能看自己名下的设备，
// 管理员看全租户），光有 userID 拼不出这个判断。用 (tenantID, userID) 调这些能力，
// 要么把归属过滤整个丢掉，要么拿一个"猜出来的 authority"去查——两种都是越权入口。
type (
	// MobileDeviceLister 设备列表。
	MobileDeviceLister interface {
		List(ctx context.Context, claims *utils.UserClaims, search string, page, pageSize int) ([]MobileDeviceSummary, int64, error)
	}
	// MobileAlarmLister 告警列表。
	MobileAlarmLister interface {
		List(ctx context.Context, claims *utils.UserClaims, page, pageSize int) (MobileAlarmList, error)
	}
	// MobileAlarmAcker 告警确认。
	MobileAlarmAcker interface {
		Acknowledge(ctx context.Context, claims *utils.UserClaims, alarmID string) error
	}
	// MobileShadowStore 设备影子读写。
	MobileShadowStore interface {
		Get(ctx context.Context, claims *utils.UserClaims, deviceID string) (string, error)
		Update(ctx context.Context, claims *utils.UserClaims, deviceID, patch string) error
	}
	// MobileOTAStatusReader OTA 状态读取。
	MobileOTAStatusReader interface {
		Status(ctx context.Context, claims *utils.UserClaims, deviceID string) (string, error)
	}
	// MobileDashboardReader 可查看的看板。
	MobileDashboardReader interface {
		List(ctx context.Context, claims *utils.UserClaims) ([]MobileDashboardSummary, error)
	}
	// MobileCommandSender 真正的命令下发。
	MobileCommandSender interface {
		Send(ctx context.Context, exec ControlExecution) error
	}
)

// MobileDeviceSummary 移动端设备摘要。
type MobileDeviceSummary struct {
	DeviceID     string     `json:"device_id"`
	Name         string     `json:"name"`
	DeviceNumber string     `json:"device_number"`
	Online       bool       `json:"online"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	// WarnStatus 设备级告警标记："Y" 有告警，"N" 正常。
	// 它是**布尔标记的投影**，刻意不叫 alarm_count：设备列表查询不 join 告警表，
	// 造一个 0/1 的"条数"会让移动端把它当成告警数量展示——那是编出来的数字。
	WarnStatus string `json:"warn_status"`
}

// MobileAlarmList 移动端告警列表。
type MobileAlarmList struct {
	Total int64 `json:"total"`
	// List 透传既有告警查询的投影（ah.* + alarm_config_name / alarm_level）。
	// 不为移动端另造一个结构体：那份投影是动态 map，重新定义字段会静默丢字段，
	// 或者把写错名字的字段变成空值——两种都属于"看起来没报错其实没数据"。
	List []map[string]interface{} `json:"list"`
}

// MobileDashboardSummary 移动端看板摘要。
type MobileDashboardSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	HomeFlag string `json:"home_flag"`
}

// MobileCapabilityMatrix 移动端可用能力。全部由接线情况推导。
type MobileCapabilityMatrix struct {
	Telemetry    bool `json:"telemetry"`
	Commands     bool `json:"commands"`
	Alarms       bool `json:"alarms"`
	Shadow       bool `json:"shadow"`
	OTA          bool `json:"ota"`
	Dashboards   bool `json:"dashboards"`
	Push         bool `json:"push"`
	OfflineCache bool `json:"offline_cache"`
}

// CommandReceipt 命令执行收据（幂等键对应的结果）。
type CommandReceipt struct {
	Key        string    `json:"key"`
	DeviceID   string    `json:"device_id"`
	Command    string    `json:"command"`
	Accepted   bool      `json:"accepted"`
	Ambiguous  bool      `json:"ambiguous"`
	Error      string    `json:"error,omitempty"`
	FinishedAt time.Time `json:"finished_at"`
}

// CommandIdempotencyStore 幂等键存储。
type CommandIdempotencyStore interface {
	// Claim 认领键。claimed=false 且 receipt 非 nil 表示此前已完成，调用方应直接复用结果。
	Claim(ctx context.Context, tenantID, userID, key, fingerprint string, now time.Time) (claimed bool, receipt *CommandReceipt, err error)
	// Complete 写入终态收据。
	Complete(ctx context.Context, tenantID, userID, key string, receipt CommandReceipt) error
	// Release 释放认领（下发失败时调用，允许重试）。
	Release(ctx context.Context, tenantID, userID, key string) error
}

// InMemoryCommandIdempotencyStore 进程内幂等存储。
// 局限（明确声明）：仅进程内有效，重启即失。弱网重试窗口通常只有秒到分钟级，
// 进程内存储足以覆盖；跨实例/跨重启的强幂等需换成数据库或 Redis 实现同一接口。
type InMemoryCommandIdempotencyStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	claims  map[string]string         // key -> fingerprint（进行中）
	results map[string]CommandReceipt // key -> 已完成收据
	// fingerprints 记录**已完成**键的参数指纹。
	// 必须保留：若完成后就丢掉指纹，那么"同一键换一条命令"会命中旧收据并直接返回，
	// 用户以为第二条命令生效了，其实返回的是第一条的结果——这是最难排查的一类假成功。
	fingerprints map[string]string    // key -> fingerprint（已完成）
	claimed      map[string]time.Time // key -> 认领时刻
}

// NewInMemoryCommandIdempotencyStore 创建进程内幂等存储。ttl<=0 时默认 10 分钟。
func NewInMemoryCommandIdempotencyStore(ttl time.Duration) *InMemoryCommandIdempotencyStore {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &InMemoryCommandIdempotencyStore{
		ttl:          ttl,
		claims:       make(map[string]string),
		results:      make(map[string]CommandReceipt),
		fingerprints: make(map[string]string),
		claimed:      make(map[string]time.Time),
	}
}

// sweep 清理过期条目。没有它，长时间运行的进程会无界堆积键。
func (s *InMemoryCommandIdempotencyStore) sweep(now time.Time) {
	for id, at := range s.claimed {
		if now.Sub(at) > s.ttl {
			delete(s.claimed, id)
			delete(s.claims, id)
		}
	}
	for id, r := range s.results {
		if now.Sub(r.FinishedAt) > s.ttl {
			delete(s.results, id)
			delete(s.fingerprints, id)
		}
	}
}

func idemKey(tenantID, userID, key string) string {
	return strings.TrimSpace(tenantID) + "|" + strings.TrimSpace(userID) + "|" + strings.TrimSpace(key)
}

// Claim 认领幂等键。
func (s *InMemoryCommandIdempotencyStore) Claim(ctx context.Context, tenantID, userID, key, fingerprint string, now time.Time) (bool, *CommandReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep(now)
	id := idemKey(tenantID, userID, key)

	// 已完成的键：先比对指纹。指纹不同说明这不是重试，而是另一条命令借用了同一个键，
	// 必须拒绝——直接返回旧收据会让用户以为新命令生效了。
	if r, ok := s.results[id]; ok {
		if fp, seen := s.fingerprints[id]; seen && fp != fingerprint {
			return false, nil, ErrMobileCommandConflict
		}
		cp := r
		return false, &cp, nil
	}
	if fp, ok := s.claims[id]; ok {
		if fp != fingerprint {
			return false, nil, ErrMobileCommandConflict
		}
		return false, nil, nil
	}
	s.claims[id] = fingerprint
	s.claimed[id] = now
	return true, nil, nil
}

// Complete 写入终态收据。
func (s *InMemoryCommandIdempotencyStore) Complete(ctx context.Context, tenantID, userID, key string, receipt CommandReceipt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := idemKey(tenantID, userID, key)
	// 把指纹从"进行中"迁到"已完成"，供后续重试比对。
	if fp, ok := s.claims[id]; ok {
		s.fingerprints[id] = fp
	}
	s.results[id] = receipt
	delete(s.claims, id)
	delete(s.claimed, id)
	return nil
}

// Release 释放认领。
func (s *InMemoryCommandIdempotencyStore) Release(ctx context.Context, tenantID, userID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := idemKey(tenantID, userID, key)
	delete(s.claims, id)
	delete(s.claimed, id)
	return nil
}

// ---------------------------------------------------------------------------
// MobileService
// ---------------------------------------------------------------------------

// MobileService 移动端聚合服务。所有能力均为可选注入，未注入即不可用。
type MobileService struct {
	devices     MobileDeviceLister
	alarmLister MobileAlarmLister
	alarms      MobileAlarmAcker
	shadow      MobileShadowStore
	ota         MobileOTAStatusReader
	dashboards  MobileDashboardReader
	commands    MobileCommandSender
	idem        CommandIdempotencyStore
	push        *PushService
}

// MobileServiceDeps 依赖注入集合。
type MobileServiceDeps struct {
	Devices     MobileDeviceLister
	AlarmLister MobileAlarmLister
	Alarms      MobileAlarmAcker
	Shadow      MobileShadowStore
	OTA         MobileOTAStatusReader
	Dashboards  MobileDashboardReader
	Commands    MobileCommandSender
	Idempotency CommandIdempotencyStore
	Push        *PushService
}

// NewMobileService 创建移动端服务。幂等存储为 nil 时自动降级为进程内实现，
// 但**命令能力仍要求 commands 已接线**，否则 Capabilities.Commands 为 false。
func NewMobileService(deps MobileServiceDeps) *MobileService {
	if deps.Idempotency == nil {
		deps.Idempotency = NewInMemoryCommandIdempotencyStore(0)
	}
	return &MobileService{
		devices:     deps.Devices,
		alarmLister: deps.AlarmLister,
		alarms:      deps.Alarms,
		shadow:      deps.Shadow,
		ota:         deps.OTA,
		dashboards:  deps.Dashboards,
		commands:    deps.Commands,
		idem:        deps.Idempotency,
		push:        deps.Push,
	}
}

// Capabilities 返回实际可用的能力。未接线的能力一律 false。
// OfflineCache 恒为 false：离线缓存是客户端能力，服务端无从得知客户端是否已缓存，
// 报 true 等于替客户端撒谎。
func (s *MobileService) Capabilities(ctx context.Context) MobileCapabilityMatrix {
	return MobileCapabilityMatrix{
		Telemetry: s.devices != nil,
		Commands:  s.commands != nil && s.idem != nil,
		// 告警能力要求"能看"和"能确认"都接线：只有确认没有列表，移动端拿不到
		// 告警 ID，确认入口根本点不到；只有列表没有确认，就是个看板。
		Alarms:       s.alarmLister != nil && s.alarms != nil,
		Shadow:       s.shadow != nil,
		OTA:          s.ota != nil,
		Dashboards:   s.dashboards != nil,
		Push:         s.push != nil,
		OfflineCache: false,
	}
}

// ListDevices 列出设备。未接线返回错误而非空列表——空列表会被移动端
// 渲染成"你还没有设备"，把未接线伪装成真的没有数据。
func (s *MobileService) ListDevices(ctx context.Context, claims *utils.UserClaims, search string, page, pageSize int) ([]MobileDeviceSummary, int64, error) {
	if s.devices == nil {
		return nil, 0, errcode.NewWithMessage(errcode.CodeOpDenied, ErrMobileNotWired.Error())
	}
	if claims == nil {
		return nil, 0, errcode.NewWithMessage(errcode.CodeNoPermission, "mobile device list requires authenticated claims")
	}
	return s.devices.List(ctx, claims, search, page, pageSize)
}

// ListAlarms 列出告警。
func (s *MobileService) ListAlarms(ctx context.Context, claims *utils.UserClaims, page, pageSize int) (MobileAlarmList, error) {
	if s.alarmLister == nil {
		return MobileAlarmList{}, errcode.NewWithMessage(errcode.CodeOpDenied, ErrMobileNotWired.Error())
	}
	if claims == nil {
		return MobileAlarmList{}, errcode.NewWithMessage(errcode.CodeNoPermission, "mobile alarm list requires authenticated claims")
	}
	return s.alarmLister.List(ctx, claims, page, pageSize)
}

// AcknowledgeAlarm 确认告警。
func (s *MobileService) AcknowledgeAlarm(ctx context.Context, claims *utils.UserClaims, alarmID string) error {
	if s.alarms == nil {
		return errcode.NewWithMessage(errcode.CodeOpDenied, ErrMobileNotWired.Error())
	}
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "mobile alarm acknowledgement requires authenticated claims")
	}
	return s.alarms.Acknowledge(ctx, claims, alarmID)
}

// GetShadow 读取影子。
func (s *MobileService) GetShadow(ctx context.Context, claims *utils.UserClaims, deviceID string) (string, error) {
	if s.shadow == nil {
		return "", errcode.NewWithMessage(errcode.CodeOpDenied, ErrMobileNotWired.Error())
	}
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "mobile shadow read requires authenticated claims")
	}
	return s.shadow.Get(ctx, claims, deviceID)
}

// UpdateShadow 更新影子。
func (s *MobileService) UpdateShadow(ctx context.Context, claims *utils.UserClaims, deviceID, patch string) error {
	if s.shadow == nil {
		return errcode.NewWithMessage(errcode.CodeOpDenied, ErrMobileNotWired.Error())
	}
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "mobile shadow update requires authenticated claims")
	}
	return s.shadow.Update(ctx, claims, deviceID, patch)
}

// OTAStatus 读取 OTA 状态。
func (s *MobileService) OTAStatus(ctx context.Context, claims *utils.UserClaims, deviceID string) (string, error) {
	if s.ota == nil {
		return "", errcode.NewWithMessage(errcode.CodeOpDenied, ErrMobileNotWired.Error())
	}
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "mobile OTA status requires authenticated claims")
	}
	return s.ota.Status(ctx, claims, deviceID)
}

// ListDashboards 列出可查看看板。
func (s *MobileService) ListDashboards(ctx context.Context, claims *utils.UserClaims) ([]MobileDashboardSummary, error) {
	if s.dashboards == nil {
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied, ErrMobileNotWired.Error())
	}
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "mobile dashboard list requires authenticated claims")
	}
	return s.dashboards.List(ctx, claims)
}

// SendCommand 幂等下发命令。
// 命中已完成幂等键时直接返回原收据，不再下发（见文件头注意事项 2）。
func (s *MobileService) SendCommand(ctx context.Context, tenantID, userID, idempotencyKey string, exec ControlExecution) (*CommandReceipt, error) {
	if s.commands == nil || s.idem == nil {
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied, ErrMobileNotWired.Error())
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, ErrMobileIdempotencyKey.Error())
	}
	// 指纹涵盖命令的全部实质参数：换了参数就不是同一次重试。
	fingerprint := strings.Join([]string{exec.DeviceID, exec.WidgetType, exec.Command, exec.Params}, "|")
	now := time.Now()

	claimed, existing, err := s.idem.Claim(ctx, tenantID, userID, idempotencyKey, fingerprint, now)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	if existing != nil {
		cp := *existing
		return &cp, nil
	}
	if !claimed {
		// 已有人在处理同一键（并发重复提交）：明确返回冲突，
		// 而不是当成自己的成功——那会让两个调用方都以为自己发成功了。
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied, "command with this idempotency key is still in flight")
	}

	sendErr := s.commands.Send(ctx, exec)
	receipt := CommandReceipt{
		Key:        idempotencyKey,
		DeviceID:   exec.DeviceID,
		Command:    exec.Command,
		Accepted:   sendErr == nil,
		FinishedAt: time.Now(),
	}
	if sendErr != nil {
		receipt.Error = sendErr.Error()
		// 失败即释放，保证弱网下可以重试（注意事项 3）。
		_ = s.idem.Release(ctx, tenantID, userID, idempotencyKey)
		return &receipt, errcode.NewWithMessage(errcode.CodeOpDenied, sendErr.Error())
	}
	_ = s.idem.Complete(ctx, tenantID, userID, idempotencyKey, receipt)
	return &receipt, nil
}

// SubscribePush 登记推送令牌。平台非法直接拒绝。
func (s *MobileService) SubscribePush(ctx context.Context, tenantID, userID, platform, token, provider string) (*model.PushDeviceRegistration, error) {
	if !model.IsAllowedPushPlatform(platform) {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, ErrMobileBadPlatform.Error())
	}
	reg := &model.PushDeviceRegistration{
		ID:       uuid.New().String(),
		TenantID: tenantID,
		UserID:   userID,
		Platform: platform,
		Token:    strings.TrimSpace(token),
		Provider: provider,
		Enabled:  true,
	}
	if err := model.ValidatePushRegistration(reg); err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	if err := dal.UpsertPushRegistration(reg); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	return reg, nil
}

// UnsubscribePush 撤销一条令牌登记。
func (s *MobileService) UnsubscribePush(ctx context.Context, tenantID, registrationID string) error {
	// id 列是 uuid：非 uuid 直接进 SQL 会让 PG 抛 22P02（无效的类型 uuid 输入语法），
	// 而该错误会经 CodeDBError 把驱动原文带回客户端。这里先判定，统一按"未找到"返回。
	if _, err := uuid.Parse(registrationID); err != nil {
		return errcode.NewWithMessage(errcode.CodeNotFound, "push registration not found")
	}
	affected, err := dal.DeletePushRegistrationInTenant(registrationID, tenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if affected == 0 {
		return errcode.NewWithMessage(errcode.CodeNotFound, "push registration not found")
	}
	return nil
}
