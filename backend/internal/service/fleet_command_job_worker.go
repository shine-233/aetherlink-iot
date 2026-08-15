package service

import (
	"context"
	"fmt"
	"sync"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type fleetCommandJobProcessor func(context.Context, string, string, *utils.UserClaims) error

type fleetCommandJobDispatcher interface {
	Enqueue(jobID, operatorID string, claims *utils.UserClaims, process fleetCommandJobProcessor) bool
}

type inProcessFleetCommandJobDispatcher struct {
	mu     sync.Mutex
	active map[string]struct{}
}

var defaultFleetCommandJobDispatcher fleetCommandJobDispatcher = newInProcessFleetCommandJobDispatcher()

func newInProcessFleetCommandJobDispatcher() *inProcessFleetCommandJobDispatcher {
	return &inProcessFleetCommandJobDispatcher{active: map[string]struct{}{}}
}

func (d *inProcessFleetCommandJobDispatcher) Enqueue(
	jobID, operatorID string,
	claims *utils.UserClaims,
	process fleetCommandJobProcessor,
) bool {
	if jobID == "" || claims == nil || process == nil {
		return false
	}

	d.mu.Lock()
	if _, exists := d.active[jobID]; exists {
		d.mu.Unlock()
		logrus.WithField("job_id", jobID).Debug("command job worker already active")
		return false
	}
	d.active[jobID] = struct{}{}
	d.mu.Unlock()

	claimsCopy := *claims
	go func() {
		defer func() {
			d.mu.Lock()
			delete(d.active, jobID)
			d.mu.Unlock()

			if recovered := recover(); recovered != nil {
				logrus.WithField("job_id", jobID).WithField("panic", recovered).Error("command job worker panic")
				recordFleetCommandJobEvent(jobID, claimsCopy.TenantID, nil, nil, commandJobEventWorkerFailed, fmt.Sprintf("command job worker panic: %v", recovered))
			}
		}()
		if err := process(context.Background(), jobID, operatorID, &claimsCopy); err != nil {
			logrus.WithError(err).WithField("job_id", jobID).Warn("command job worker failed")
			recordFleetCommandJobEvent(jobID, claimsCopy.TenantID, nil, nil, commandJobEventWorkerFailed, err.Error())
		}
	}()
	return true
}

func (c *CommandData) dispatchFleetCommandJob(jobID, operatorID string, claims *utils.UserClaims) bool {
	return defaultFleetCommandJobDispatcher.Enqueue(jobID, operatorID, claims, c.processFleetCommandJob)
}
