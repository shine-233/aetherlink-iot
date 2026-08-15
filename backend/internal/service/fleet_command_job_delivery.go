package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

const commandJobSummaryRefreshInterval = 25
const commandJobDispatchLeaseSeconds = 120

func (c *CommandData) processFleetCommandJob(ctx context.Context, jobID, operatorID string, claims *utils.UserClaims) error {
	dispatchedRows := 0
	hasUnflushedSummary := false
	var lastLoadedJob *model.CommandJob
	for {
		job, err := loadFleetCommandJobWithFreshTimeout(jobID, claims.TenantID)
		if err != nil {
			if hasUnflushedSummary && lastLoadedJob != nil {
				_ = refreshCommandJobSummary(lastLoadedJob)
			}
			return err
		}
		lastLoadedJob = job
		if job.Status != commandJobStatusRunning {
			return nil
		}

		claim, gateWaitLimit, err := claimNextFleetCommandJobRow(job, claims.TenantID)
		if err != nil {
			if hasUnflushedSummary {
				_ = refreshCommandJobSummary(job)
			}
			return err
		}
		if claim == nil {
			return fmt.Errorf("命令任务下发领取返回空结果")
		}
		if claim.State == dal.CommandJobDispatchDeferred {
			job.NextDispatchAt = claim.NextDispatchAt
			wait, shouldWait := commandJobDispatchGateWaitDuration(claim.RetryAt, time.Now().UTC(), gateWaitLimit)
			if !shouldWait {
				if hasUnflushedSummary {
					_ = refreshCommandJobSummary(job)
				}
				return nil
			}
			if wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-timer.C:
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					return ctx.Err()
				}
			}
			continue
		}
		detail := claim.Detail
		if claim.State == dal.CommandJobDispatchEmpty || detail == nil {
			job.NextDispatchAt = nil
			if err := refreshCommandJobSummary(job); err != nil {
				return err
			}
			if commandJobStatusIsTerminal(job.Status) {
				recordFleetCommandJobEvent(job.ID, job.TenantID, nil, nil, commandJobEventCompleted, fmt.Sprintf("命令任务已进入状态 %s", job.Status))
			}
			return nil
		}
		job.NextDispatchAt = claim.NextDispatchAt

		persisted, err := c.dispatchClaimedFleetCommandJobRow(ctx, operatorID, job, detail, claims)
		if err != nil && !persisted {
			if hasUnflushedSummary {
				_ = refreshCommandJobSummary(job)
			}
			return err
		}
		dispatchedRows++
		hasUnflushedSummary = true
		if shouldRefreshCommandJobSummaryAfterDispatch(dispatchedRows) {
			if err := refreshCommandJobSummary(job); err != nil {
				return err
			}
			hasUnflushedSummary = false
		}
	}
}

func claimNextFleetCommandJobRow(job *model.CommandJob, tenantID string) (*dal.CommandJobDispatchClaim, time.Duration, error) {
	leaseToken := uuid.New()
	messageID := uuid.New()[:8]
	policy, gateWaitLimit := currentFleetCommandJobDispatchPolicy()
	claim, err := dal.ClaimNextReadyCommandJobDispatch(
		job.ID,
		tenantID,
		commandJobDetailStatusDispatching,
		leaseToken,
		messageID,
		policy,
		commandJobDispatchLeaseSeconds*time.Second,
	)
	return claim, gateWaitLimit, err
}

func commandJobDispatchGateWaitDuration(retryAt *time.Time, now time.Time, maximum time.Duration) (time.Duration, bool) {
	if retryAt == nil || maximum <= 0 {
		return 0, false
	}
	wait := retryAt.Sub(now)
	if wait <= 0 {
		return 0, true
	}
	if wait > maximum {
		return wait, false
	}
	return wait, true
}

func commandJobStatusIsTerminal(status string) bool {
	switch status {
	case commandJobStatusCompleted, commandJobStatusPartiallyFailed, commandJobStatusFailed, commandJobStatusCanceled:
		return true
	default:
		return false
	}
}

func (c *CommandData) dispatchClaimedFleetCommandJobRow(
	ctx context.Context,
	operatorID string,
	job *model.CommandJob,
	detail *model.CommandJobDetail,
	claims *utils.UserClaims,
) (bool, error) {
	recordFleetCommandJobEvent(job.ID, job.TenantID, &detail.ID, &detail.DeviceID, commandJobEventDispatchStarted, "worker 已领取设备命令行")
	persisted, err := c.submitClaimedFleetCommandJobDetail(ctx, operatorID, job.Identify, job.CommandValue, detail, strconv.Itoa(constant.Manual), claims)
	if err != nil {
		recordFleetCommandJobEvent(job.ID, job.TenantID, &detail.ID, &detail.DeviceID, commandJobEventDispatchFailed, err.Error())
		if !persisted {
			return persisted, err
		}
	}
	if err == nil && detail.Status == commandJobDetailStatusFailed {
		recordFleetCommandJobEvent(job.ID, job.TenantID, &detail.ID, &detail.DeviceID, commandJobEventDispatchFailed, SafeDeref(detail.Reason))
	} else if detail.Status != commandJobDetailStatusFailed {
		recordFleetCommandJobEvent(job.ID, job.TenantID, &detail.ID, &detail.DeviceID, commandJobEventDispatchSubmitted, SafeDeref(detail.MessageID))
	}
	return persisted, err
}

func shouldRefreshCommandJobSummaryAfterDispatch(dispatchedRows int) bool {
	return dispatchedRows > 0 && dispatchedRows%commandJobSummaryRefreshInterval == 0
}

func (c *CommandData) submitFleetCommandJobDetails(ctx context.Context, operatorID, identify string, value *string, details []*model.CommandJobDetail, claims *utils.UserClaims) error {
	manualOperationType := strconv.Itoa(constant.Manual)
	for _, detail := range details {
		if !detail.Eligible {
			continue
		}
		if err := c.submitFleetCommandJobDetail(ctx, operatorID, identify, value, detail, manualOperationType, claims); err != nil {
			return err
		}
	}
	return nil
}

func (c *CommandData) submitFleetCommandJobDetail(ctx context.Context, operatorID, identify string, value *string, detail *model.CommandJobDetail, operationType string, claims *utils.UserClaims) error {
	tracking, err := c.CommandPutMessageWithTracking(ctx, operatorID, &model.PutMessageForCommand{
		DeviceID: detail.DeviceID,
		Identify: identify,
		Value:    value,
	}, operationType, claims)
	now := time.Now().UTC()
	if err != nil {
		applyFleetCommandJobDetailFailure(detail, err, now)
		return dal.UpdateCommandJobDetail(detail)
	}

	applyFleetCommandJobDetailSuccess(detail, tracking, now)
	return dal.UpdateCommandJobDetail(detail)
}

func (c *CommandData) submitClaimedFleetCommandJobDetail(ctx context.Context, operatorID, identify string, value *string, detail *model.CommandJobDetail, operationType string, claims *utils.UserClaims) (bool, error) {
	leaseToken := SafeDeref(detail.DispatchLeaseToken)
	if leaseToken == "" {
		return false, fmt.Errorf("已领取的命令任务行缺少下发租约")
	}
	messageID := SafeDeref(detail.MessageID)
	if messageID == "" {
		return false, fmt.Errorf("已领取的命令任务行缺少预分配 message id")
	}

	tracking, err := c.CommandPutMessageWithTracking(ctx, operatorID, &model.PutMessageForCommand{
		DeviceID: detail.DeviceID,
		Identify: identify,
		Value:    value,
	}, operationType, WithCommandDeliveryMessageID(messageID), claims)
	now := time.Now().UTC()
	if err != nil {
		applyFleetCommandJobDetailFailure(detail, err, now)
	} else {
		applyFleetCommandJobDetailSuccess(detail, tracking, now)
	}

	affected, updateErr := dal.UpdateClaimedCommandJobDetailAfterDispatch(detail, leaseToken)
	if updateErr != nil {
		return false, updateErr
	}
	if affected == 0 {
		return false, fmt.Errorf("命令任务行下发租约已不再属于当前 worker")
	}
	return true, err
}
