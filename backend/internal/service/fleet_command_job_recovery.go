package service

import (
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

const defaultFleetCommandJobRecoveryLimit = 100

func (c *CommandData) RecoverTimedOutFleetCommandJobs() error {
	jobs, err := dal.ListTimedOutRunningCommandJobs(time.Now().UTC(), defaultFleetCommandJobRecoveryLimit)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := expireCommandJobIfTimedOut(job); err != nil {
			logrus.WithError(err).WithField("job_id", job.ID).Warn("recover timed-out command job failed")
			continue
		}
	}
	return nil
}

func (c *CommandData) ResumeRunnableFleetCommandJobs() error {
	jobs, err := dal.ListRunnableCommandJobs(
		time.Now().UTC(),
		[]string{commandJobDetailStatusReady, commandJobDetailStatusDispatching},
		defaultFleetCommandJobRecoveryLimit,
	)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		now := time.Now().UTC()
		if job.Status == commandJobStatusScheduled {
			activated, err := dal.ActivateScheduledCommandJob(job.ID, job.TenantID, now)
			if err != nil {
				logrus.WithError(err).WithField("job_id", job.ID).Warn("activate scheduled command job failed")
				continue
			}
			if !activated {
				continue
			}
			job.Status = commandJobStatusRunning
			job.UpdatedAt = now
			recordFleetCommandJobEvent(
				job.ID,
				job.TenantID,
				nil,
				nil,
				commandJobEventStarted,
				"scheduled command job reached scheduled_at and entered the durable dispatch queue",
			)
		}
		interrupted, err := dal.FailInterruptedCommandJobDetails(
			job.ID,
			job.TenantID,
			commandJobMaxDispatchAttempts,
			commandJobNextRetryAfter(1, now),
			now,
		)
		if err != nil {
			logrus.WithError(err).WithField("job_id", job.ID).Warn("mark interrupted command job rows failed")
			continue
		}
		if interrupted > 0 {
			recordFleetCommandJobEvent(
				job.ID,
				job.TenantID,
				nil,
				nil,
				commandJobEventResumed,
				fmt.Sprintf("%d interrupted dispatching row(s) marked retryable after backend restart", interrupted),
			)
			if err := refreshCommandJobSummary(job); err != nil {
				logrus.WithError(err).WithField("job_id", job.ID).Warn("refresh resumed command job summary failed")
				continue
			}
			if job.Status != commandJobStatusRunning {
				continue
			}
		}
		hasReadyRows, err := commandJobHasReadyRows(job.ID, job.TenantID)
		if err != nil {
			return err
		}
		if !hasReadyRows {
			continue
		}
		dispatched := c.dispatchFleetCommandJob(job.ID, job.OperatorID, &utils.UserClaims{
			ID:       job.OperatorID,
			TenantID: job.TenantID,
		})
		if dispatched {
			recordFleetCommandJobEvent(job.ID, job.TenantID, nil, nil, commandJobEventQueued, "running command job resumed by durable recovery scan")
		}
	}
	return nil
}

func commandJobHasReadyRows(jobID, tenantID string) (bool, error) {
	metrics, err := dal.GetCommandJobSummaryMetrics(jobID, tenantID, commandJobMaxDispatchAttempts, time.Now().UTC())
	if err != nil {
		return false, err
	}
	return metrics.StatusCounts[commandJobDetailStatusReady] > 0, nil
}
