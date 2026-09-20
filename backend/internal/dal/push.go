// 文件用途：移动端推送数据访问（ROADMAP P1.4）。
// 核心逻辑：令牌登记幂等写入、投递记录创建、可重试投递捞取与状态回写。
// 关键注意事项：
//  1. 令牌登记走数据库唯一约束的 upsert，不做"先查再插"：
//     高并发下先查再插会漏判，进而堆积重复令牌行，把一次推送放大成 N 条。
//  2. 捞取可重试投递必须由 status 与 next_attempt_at 共同限定：
//     只看 status 会捞起还没到下一点的投递，导致重试节奏被击穿（瞬间打满重试次数）。
//  3. 状态回写同样带当前状态做条件更新：两次重试器并发处理同一条时，
//     后到的那次条件不匹配而落空，避免同一条推送被真正发出两遍。
package dal

import (
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm/clause"
)

// UpsertPushRegistration 幂等登记令牌：同 (租户,用户,平台,令牌) 已存在则刷新
// provider/enabled，不新增行。
func UpsertPushRegistration(r *model.PushDeviceRegistration) error {
	return global.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"}, {Name: "user_id"}, {Name: "platform"}, {Name: "token"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"provider", "enabled", "updated_at"}),
	}).Create(r).Error
}

// ListPushRegistrationsByUser 列出用户的有效令牌。
func ListPushRegistrationsByUser(tenantID, userID string, enabledOnly bool) ([]model.PushDeviceRegistration, error) {
	var rows []model.PushDeviceRegistration
	q := global.DB.Where("tenant_id = ? AND user_id = ?", tenantID, userID)
	if enabledOnly {
		q = q.Where("enabled = ?", true)
	}
	err := q.Order("created_at ASC").Find(&rows).Error
	return rows, err
}

// DeletePushRegistrationInTenant 删除（撤销）一条令牌登记，返回受影响行数。
func DeletePushRegistrationInTenant(id, tenantID string) (int64, error) {
	res := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.PushDeviceRegistration{})
	return res.RowsAffected, res.Error
}

// CreatePushDelivery 创建一条投递记录。
func CreatePushDelivery(d *model.PushDelivery) error {
	return global.DB.Create(d).Error
}

// GetPushDeliveryInTenant 租户内读取投递；未命中返回 gorm.ErrRecordNotFound。
func GetPushDeliveryInTenant(id, tenantID string) (*model.PushDelivery, error) {
	var m model.PushDelivery
	if err := global.DB.Where("id = ? AND tenant_id = ?", id, tenantID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListRetryablePushDeliveries 捞取到点且仍可重试的投递。
// tenant-scope: system-internal —— 重试 worker 必须跨租户扫描待发投递，
// 这是系统内部作业而非对外查询接口；每条投递在回写与下发时仍按记录自带的
// tenant_id 逐条限定（见 UpdatePushDeliveryFrom），不存在跨租户数据暴露。
// nextAttemptBefore 为当前时刻：未到点的投递不参与，避免重试节奏被击穿。
func ListRetryablePushDeliveries(now time.Time, limit int) ([]model.PushDelivery, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []model.PushDelivery
	err := global.DB.
		Where("status IN ?", []string{model.PushStatusPending, model.PushStatusFailed}).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	return rows, err
}

// PushDeliveryUpdate 投递状态回写字段。
type PushDeliveryUpdate struct {
	Status        string
	AttemptCount  int32
	NextAttemptAt *time.Time
	LastError     *string
	Provider      *string
}

// UpdatePushDeliveryFrom 按当前状态条件更新投递，返回受影响行数。
// fromStatus 不匹配即落空：并发重试器重复处理同一条时后到者自然失效。
func UpdatePushDeliveryFrom(id, tenantID, fromStatus string, u PushDeliveryUpdate) (int64, error) {
	res := global.DB.Model(&model.PushDelivery{}).
		Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, fromStatus).
		Updates(map[string]interface{}{
			"status":          u.Status,
			"attempt_count":   u.AttemptCount,
			"next_attempt_at": u.NextAttemptAt,
			"last_error":      u.LastError,
			"provider":        u.Provider,
		})
	return res.RowsAffected, res.Error
}

// ListPushDeliveriesByUser 列出用户的投递历史（审计用），按创建时间倒序。
func ListPushDeliveriesByUser(tenantID, userID string, limit int) ([]model.PushDelivery, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []model.PushDelivery
	err := global.DB.Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
