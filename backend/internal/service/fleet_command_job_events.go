package service

import (
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

func recordFleetCommandJobEvent(jobID, tenantID string, detailID, deviceID *string, eventType, message string) {
	event := &model.CommandJobEvent{
		ID:           uuid.New(),
		CommandJobID: jobID,
		TenantID:     tenantID,
		DetailID:     detailID,
		DeviceID:     deviceID,
		EventType:    eventType,
		Message:      message,
		CreatedAt:    time.Now().UTC(),
	}
	if err := dal.CreateCommandJobEvent(event); err != nil {
		logrus.WithError(err).WithField("job_id", jobID).WithField("event_type", eventType).Warn("record command job event failed")
	}
}

func fleetCommandJobEventsFromPersistence(events []*model.CommandJobEvent) []model.FleetCommandJobEvent {
	result := make([]model.FleetCommandJobEvent, 0, len(events))
	for _, event := range events {
		createdAt := event.CreatedAt
		result = append(result, model.FleetCommandJobEvent{
			ID:        event.ID,
			EventType: event.EventType,
			DetailID:  SafeDeref(event.DetailID),
			DeviceID:  SafeDeref(event.DeviceID),
			Message:   event.Message,
			CreatedAt: &createdAt,
		})
	}
	return result
}
