// 文件用途：移动端推送的持久化模型与校验（ROADMAP P1.4）。
// 核心逻辑：推送令牌登记 + 投递记录（可重试、可审计）。
// 关键注意事项：
//  1. status 必须能区分"还能重试的 failed"与"已放弃的 dead"。二者混为一谈时，
//     重试器会无限捞起早已放弃的投递，把永久失败伪装成"还在路上"。
//  2. attempt_count 从 0 起且允许为 0：0 表示"还没试过"，与"试了 0 次成功"不同。
//     真正表达"试过但没成功"靠 status=failed，不靠计数。
//  3. 令牌去重在数据库层由唯一约束保证（tenant_id, user_id, platform, token），
//     重复登记走幂等更新。否则同一次推送会被放大成 N 条，用户被重复打扰。
package model

import (
	"errors"
	"strings"
	"time"
)

// 表名常量。
const (
	TableNamePushDeviceRegistration = "push_device_registrations"
	TableNamePushDelivery           = "push_deliveries"
)

// 推送平台。
const (
	PushPlatformIOS     = "ios"
	PushPlatformAndroid = "android"
	PushPlatformH5      = "h5"
)

// 投递状态。
const (
	PushStatusPending = "pending"
	PushStatusSent    = "sent"
	PushStatusFailed  = "failed"
	PushStatusDead    = "dead"
)

// 大小上限。
const (
	PushMaxTokenLength = 512
	PushMaxTitleLength = 255
	PushMaxBodySize    = 4096
	PushMaxErrorSize   = 2048
)

var (
	ErrPushNilRegistration = errors.New("push registration is nil")
	ErrPushNilDelivery     = errors.New("push delivery is nil")
	ErrPushMissingTenant   = errors.New("push entity requires tenant id")
	ErrPushMissingUser     = errors.New("push entity requires user id")
	ErrPushMissingToken    = errors.New("push registration requires a device token")
	ErrPushTokenTooLong    = errors.New("push device token is too long")
	ErrPushBadPlatform     = errors.New("push platform is not allowed")
	ErrPushBadStatus       = errors.New("push delivery status is not allowed")
	ErrPushMissingTitle    = errors.New("push delivery requires a title")
	ErrPushBodyTooLarge    = errors.New("push delivery body is too large")
	ErrPushNegativeAttempt = errors.New("push delivery attempt count cannot be negative")
	ErrPushDeadWithRetry   = errors.New("terminal push delivery must not carry a retry time")
)

// PushDeviceRegistration 移动端推送令牌登记。
type PushDeviceRegistration struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	TenantID string `gorm:"column:tenant_id;not null;uniqueIndex:idx_push_reg_unique,priority:1" json:"tenant_id"`
	UserID   string `gorm:"column:user_id;not null;uniqueIndex:idx_push_reg_unique,priority:2" json:"user_id"`
	Platform string `gorm:"column:platform;not null;uniqueIndex:idx_push_reg_unique,priority:3" json:"platform"`
	// 令牌去重由复合唯一索引保证：重复登记走 upsert 而不是堆积重复行，
	// 否则一次推送会被放大成 N 条。
	Token string `gorm:"column:token;not null;uniqueIndex:idx_push_reg_unique,priority:4" json:"token"`
	Provider  string    `gorm:"column:provider;not null" json:"provider"`
	Enabled   bool      `gorm:"column:enabled;not null" json:"enabled"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName PushDeviceRegistration's table name
func (*PushDeviceRegistration) TableName() string { return TableNamePushDeviceRegistration }

// PushDelivery 推送投递与重试审计。
type PushDelivery struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID       string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	RegistrationID *string    `gorm:"column:registration_id" json:"registration_id,omitempty"`
	UserID         string     `gorm:"column:user_id;not null" json:"user_id"`
	Title          string     `gorm:"column:title;not null" json:"title"`
	Body           string     `gorm:"column:body;not null" json:"body"`
	Data           *string    `gorm:"column:data;type:jsonb;not null" json:"data"`
	Status         string     `gorm:"column:status;not null" json:"status"`
	AttemptCount   int32      `gorm:"column:attempt_count;not null" json:"attempt_count"`
	NextAttemptAt  *time.Time `gorm:"column:next_attempt_at" json:"next_attempt_at,omitempty"`
	LastError      *string    `gorm:"column:last_error" json:"last_error,omitempty"`
	Provider       *string    `gorm:"column:provider" json:"provider,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName PushDelivery's table name
func (*PushDelivery) TableName() string { return TableNamePushDelivery }

// IsAllowedPushPlatform 判断平台是否合法。
func IsAllowedPushPlatform(platform string) bool {
	switch platform {
	case PushPlatformIOS, PushPlatformAndroid, PushPlatformH5:
		return true
	default:
		return false
	}
}

// IsAllowedPushStatus 判断投递状态是否合法。
func IsAllowedPushStatus(status string) bool {
	switch status {
	case PushStatusPending, PushStatusSent, PushStatusFailed, PushStatusDead:
		return true
	default:
		return false
	}
}

// IsPushTerminalStatus dead 为唯一终态，不再重试。
func IsPushTerminalStatus(status string) bool {
	return status == PushStatusDead || status == PushStatusSent
}

// IsPushRetryable 判断该状态是否还允许再次投递。
// sent 与 dead 都不可重试——重发已成功的推送是骚扰，重试已放弃的是自欺欺人。
func IsPushRetryable(status string) bool {
	return status == PushStatusPending || status == PushStatusFailed
}

// ValidatePushRegistration 校验令牌登记。
func ValidatePushRegistration(r *PushDeviceRegistration) error {
	if r == nil {
		return ErrPushNilRegistration
	}
	if strings.TrimSpace(r.TenantID) == "" {
		return ErrPushMissingTenant
	}
	if strings.TrimSpace(r.UserID) == "" {
		return ErrPushMissingUser
	}
	if strings.TrimSpace(r.Token) == "" {
		return ErrPushMissingToken
	}
	if len(r.Token) > PushMaxTokenLength {
		return ErrPushTokenTooLong
	}
	if !IsAllowedPushPlatform(r.Platform) {
		return ErrPushBadPlatform
	}
	if strings.TrimSpace(r.Provider) == "" {
		return errors.New("push registration requires a provider name")
	}
	return nil
}

// ValidatePushDelivery 校验投递记录。
func ValidatePushDelivery(d *PushDelivery) error {
	if d == nil {
		return ErrPushNilDelivery
	}
	if strings.TrimSpace(d.TenantID) == "" {
		return ErrPushMissingTenant
	}
	if strings.TrimSpace(d.UserID) == "" {
		return ErrPushMissingUser
	}
	if strings.TrimSpace(d.Title) == "" {
		return ErrPushMissingTitle
	}
	if len(d.Title) > PushMaxTitleLength {
		return errors.New("push delivery title is too long")
	}
	if len(d.Body) > PushMaxBodySize {
		return ErrPushBodyTooLarge
	}
	if !IsAllowedPushStatus(d.Status) {
		return ErrPushBadStatus
	}
	if d.AttemptCount < 0 {
		return ErrPushNegativeAttempt
	}
	if d.LastError != nil && len(*d.LastError) > PushMaxErrorSize {
		return errors.New("push delivery error message is too large")
	}
	// 终态不得携带重试时间：带着 next_attempt_at 的 dead 会被重试器重新捞起，
	// 于是"已放弃"实际上又活了过来，状态机自相矛盾。
	if !IsPushRetryable(d.Status) && d.NextAttemptAt != nil {
		return ErrPushDeadWithRetry
	}
	return nil
}
