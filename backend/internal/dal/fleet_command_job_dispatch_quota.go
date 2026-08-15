package dal

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	commandJobDispatchQuotaScopeGlobal = "global"
	commandJobDispatchQuotaScopeTenant = "tenant"
	commandJobDispatchQuotaGlobalID    = "all"
)

type CommandJobDispatchPolicy struct {
	GlobalMaxConcurrent     int
	TenantMaxConcurrent     int
	GlobalRatePerSecond     float64
	TenantRatePerSecond     float64
	ContentionRetryInterval time.Duration
}

type CommandJobDispatchClaimState string

const (
	CommandJobDispatchClaimed  CommandJobDispatchClaimState = "claimed"
	CommandJobDispatchDeferred CommandJobDispatchClaimState = "deferred"
	CommandJobDispatchEmpty    CommandJobDispatchClaimState = "empty"
)

type CommandJobDispatchClaim struct {
	State             CommandJobDispatchClaimState
	Detail            *model.CommandJobDetail
	RetryAt           *time.Time
	NextDispatchAt    *time.Time
	Reason            string
	GlobalDispatching int64
	TenantDispatching int64
}

func (policy CommandJobDispatchPolicy) validate() error {
	if policy.GlobalMaxConcurrent <= 0 || policy.TenantMaxConcurrent <= 0 {
		return fmt.Errorf("command job dispatch concurrency limits must be positive")
	}
	if policy.GlobalRatePerSecond <= 0 || policy.TenantRatePerSecond <= 0 {
		return fmt.Errorf("command job dispatch rates must be positive")
	}
	if policy.ContentionRetryInterval <= 0 {
		return fmt.Errorf("command job dispatch contention retry interval must be positive")
	}
	return nil
}

func ClaimNextReadyCommandJobDispatch(
	jobID, tenantID, dispatchingStatus, leaseToken, messageID string,
	policy CommandJobDispatchPolicy,
	leaseDuration time.Duration,
) (*CommandJobDispatchClaim, error) {
	jobID = strings.TrimSpace(jobID)
	tenantID = strings.TrimSpace(tenantID)
	if jobID == "" || tenantID == "" || dispatchingStatus == "" || leaseToken == "" || messageID == "" {
		return nil, fmt.Errorf("command job dispatch claim identity is incomplete")
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	if leaseDuration <= 0 {
		return nil, fmt.Errorf("command job dispatch lease duration must be positive")
	}

	claim := &CommandJobDispatchClaim{State: CommandJobDispatchEmpty}
	err := global.DB.Transaction(func(tx *gorm.DB) error {
		globalQuota, tenantQuota, err := lockCommandJobDispatchQuotas(tx, tenantID, policy)
		if err != nil {
			return err
		}
		// The authoritative claim clock must be read after both quota locks are
		// held. Under contention, using a timestamp captured before the lock wait
		// can commit an already-expired lease and allow another backend to reclaim
		// the row immediately.
		now, err := commandJobDispatchDatabaseNow(tx)
		if err != nil {
			return err
		}
		leaseUntil := now.Add(leaseDuration)
		if err := updateCommandJobDispatchQuotaMetadata(tx, globalQuota, policy.GlobalMaxConcurrent, policy.GlobalRatePerSecond, now); err != nil {
			return err
		}
		if err := updateCommandJobDispatchQuotaMetadata(tx, tenantQuota, policy.TenantMaxConcurrent, policy.TenantRatePerSecond, now); err != nil {
			return err
		}

		globalDispatching, err := countActiveCommandJobDispatches(tx, "", dispatchingStatus, now)
		if err != nil {
			return err
		}
		tenantDispatching, err := countActiveCommandJobDispatches(tx, tenantID, dispatchingStatus, now)
		if err != nil {
			return err
		}
		claim.GlobalDispatching = globalDispatching
		claim.TenantDispatching = tenantDispatching

		// 并发/限速判定抽到 EvaluateCommandJobDispatchGate（纯函数），这里只负责
		// 把数据库读到的实际状态喂进去并落地结果，保证可测口径与运行口径一致。
		if gate := EvaluateCommandJobDispatchGate(CommandJobDispatchGateInput{
			Policy:               policy,
			Now:                  now,
			GlobalDispatching:    globalDispatching,
			TenantDispatching:    tenantDispatching,
			GlobalNextDispatchAt: globalQuota.NextDispatchAt,
			TenantNextDispatchAt: tenantQuota.NextDispatchAt,
		}); !gate.Allow {
			return deferCommandJobDispatchClaim(tx, claim, jobID, tenantID, gate.RetryAt, gate.Reason)
		}

		var detail model.CommandJobDetail
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("command_job_id = ? AND tenant_id = ? AND status = ? AND eligible = ?", jobID, tenantID, "ready", true).
			Where(
				"EXISTS (SELECT 1 FROM "+model.TableNameCommandJob+" j WHERE j.id = "+model.TableNameCommandJobDetail+".command_job_id AND j.tenant_id = "+model.TableNameCommandJobDetail+".tenant_id AND j.status = ? AND (j.timeout_at IS NULL OR j.timeout_at > ?))",
				"running",
				now,
			).
			Order("created_at ASC, id ASC").
			First(&detail).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return setCommandJobNextDispatchAt(tx, jobID, tenantID, nil)
			}
			return err
		}

		result := tx.Model(&model.CommandJobDetail{}).
			Where("id = ? AND tenant_id = ? AND status = ?", detail.ID, tenantID, "ready").
			Where(
				"EXISTS (SELECT 1 FROM "+model.TableNameCommandJob+" j WHERE j.id = "+model.TableNameCommandJobDetail+".command_job_id AND j.tenant_id = "+model.TableNameCommandJobDetail+".tenant_id AND j.status = ? AND (j.timeout_at IS NULL OR j.timeout_at > ?))",
				"running",
				now,
			).
			Updates(map[string]interface{}{
				"status":                   dispatchingStatus,
				"can_retry":                false,
				"message_id":               messageID,
				"dispatch_attempts":        gorm.Expr("dispatch_attempts + 1"),
				"dispatch_lease_token":     leaseToken,
				"dispatch_lease_until":     leaseUntil,
				"last_dispatch_started_at": now,
				"next_retry_after":         nil,
				"updated_at":               now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		globalNext := nextCommandJobDispatchQuotaSlot(globalQuota.NextDispatchAt, now, policy.GlobalRatePerSecond)
		tenantNext := nextCommandJobDispatchQuotaSlot(tenantQuota.NextDispatchAt, now, policy.TenantRatePerSecond)
		if err := updateCommandJobDispatchQuota(tx, globalQuota, policy.GlobalMaxConcurrent, policy.GlobalRatePerSecond, globalNext, now); err != nil {
			return err
		}
		if err := updateCommandJobDispatchQuota(tx, tenantQuota, policy.TenantMaxConcurrent, policy.TenantRatePerSecond, tenantNext, now); err != nil {
			return err
		}
		jobNext := laterCommandJobDispatchTime(globalNext, tenantNext)
		if err := setCommandJobNextDispatchAt(tx, jobID, tenantID, &jobNext); err != nil {
			return err
		}

		detail.Status = dispatchingStatus
		detail.CanRetry = false
		detail.MessageID = &messageID
		detail.DispatchAttempts++
		detail.DispatchLeaseToken = &leaseToken
		detail.DispatchLeaseUntil = &leaseUntil
		detail.LastDispatchStartedAt = &now
		detail.NextRetryAfter = nil
		detail.UpdatedAt = now
		claim.State = CommandJobDispatchClaimed
		claim.Detail = &detail
		claim.NextDispatchAt = &jobNext
		claim.GlobalDispatching++
		claim.TenantDispatching++
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &CommandJobDispatchClaim{State: CommandJobDispatchEmpty}, nil
	}
	if err != nil {
		return nil, err
	}
	return claim, nil
}

func commandJobDispatchDatabaseNow(tx *gorm.DB) (time.Time, error) {
	var now time.Time
	err := tx.Raw("SELECT clock_timestamp()").Scan(&now).Error
	return now.UTC(), err
}

func lockCommandJobDispatchQuotas(
	tx *gorm.DB,
	tenantID string,
	policy CommandJobDispatchPolicy,
) (*model.CommandJobDispatchQuota, *model.CommandJobDispatchQuota, error) {
	seedNow, err := commandJobDispatchDatabaseNow(tx)
	if err != nil {
		return nil, nil, err
	}
	rows := []*model.CommandJobDispatchQuota{
		{
			ScopeType:      commandJobDispatchQuotaScopeGlobal,
			ScopeID:        commandJobDispatchQuotaGlobalID,
			NextDispatchAt: seedNow,
			MaxConcurrent:  policy.GlobalMaxConcurrent,
			RatePerSecond:  policy.GlobalRatePerSecond,
			UpdatedAt:      seedNow,
		},
		{
			ScopeType:      commandJobDispatchQuotaScopeTenant,
			ScopeID:        tenantID,
			NextDispatchAt: seedNow,
			MaxConcurrent:  policy.TenantMaxConcurrent,
			RatePerSecond:  policy.TenantRatePerSecond,
			UpdatedAt:      seedNow,
		},
	}
	for _, row := range rows {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
			return nil, nil, err
		}
	}

	globalQuota := &model.CommandJobDispatchQuota{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("scope_type = ? AND scope_id = ?", commandJobDispatchQuotaScopeGlobal, commandJobDispatchQuotaGlobalID).
		First(globalQuota).Error; err != nil {
		return nil, nil, err
	}
	tenantQuota := &model.CommandJobDispatchQuota{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("scope_type = ? AND scope_id = ?", commandJobDispatchQuotaScopeTenant, tenantID).
		First(tenantQuota).Error; err != nil {
		return nil, nil, err
	}

	return globalQuota, tenantQuota, nil
}

func countActiveCommandJobDispatches(tx *gorm.DB, tenantID, dispatchingStatus string, now time.Time) (int64, error) {
	var count int64
	query := tx.Model(&model.CommandJobDetail{}).
		Where("status = ? AND eligible = ? AND (dispatch_lease_until IS NULL OR dispatch_lease_until > ?)", dispatchingStatus, true, now)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.Count(&count).Error
	return count, err
}

func deferCommandJobDispatchClaim(
	tx *gorm.DB,
	claim *CommandJobDispatchClaim,
	jobID, tenantID string,
	retryAt time.Time,
	reason string,
) error {
	claim.State = CommandJobDispatchDeferred
	claim.RetryAt = &retryAt
	claim.NextDispatchAt = &retryAt
	claim.Reason = reason
	return setCommandJobNextDispatchAt(tx, jobID, tenantID, &retryAt)
}

func setCommandJobNextDispatchAt(tx *gorm.DB, jobID, tenantID string, next *time.Time) error {
	return tx.Model(&model.CommandJob{}).
		Where("id = ? AND tenant_id = ? AND status = ?", jobID, tenantID, "running").
		Update("next_dispatch_at", next).Error
}

func updateCommandJobDispatchQuotaMetadata(
	tx *gorm.DB,
	quota *model.CommandJobDispatchQuota,
	maxConcurrent int,
	ratePerSecond float64,
	now time.Time,
) error {
	return tx.Model(&model.CommandJobDispatchQuota{}).
		Where("scope_type = ? AND scope_id = ?", quota.ScopeType, quota.ScopeID).
		Updates(map[string]interface{}{
			"max_concurrent":  maxConcurrent,
			"rate_per_second": ratePerSecond,
			"updated_at":      now,
		}).Error
}

func updateCommandJobDispatchQuota(
	tx *gorm.DB,
	quota *model.CommandJobDispatchQuota,
	maxConcurrent int,
	ratePerSecond float64,
	next, now time.Time,
) error {
	return tx.Model(&model.CommandJobDispatchQuota{}).
		Where("scope_type = ? AND scope_id = ?", quota.ScopeType, quota.ScopeID).
		Updates(map[string]interface{}{
			"next_dispatch_at": next,
			"max_concurrent":   maxConcurrent,
			"rate_per_second":  ratePerSecond,
			"updated_at":       now,
		}).Error
}

func nextCommandJobDispatchQuotaSlot(cursor, now time.Time, ratePerSecond float64) time.Time {
	if cursor.Before(now) {
		cursor = now
	}
	return cursor.Add(commandJobDispatchRateInterval(ratePerSecond))
}

func commandJobDispatchRateInterval(ratePerSecond float64) time.Duration {
	if ratePerSecond <= 0 {
		return time.Second
	}
	if ratePerSecond < 0.001 {
		ratePerSecond = 0.001
	}
	nanoseconds := math.Ceil(float64(time.Second) / ratePerSecond)
	if nanoseconds < float64(time.Millisecond) {
		nanoseconds = float64(time.Millisecond)
	}
	return time.Duration(nanoseconds)
}

func laterCommandJobDispatchTime(left, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}
