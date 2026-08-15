package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
)

func RecordFleetCommandJobDeviceResponse(deviceID, messageID, status, payload, responseError string, responseAt time.Time) error {
	detail, affected, err := dal.UpdateCommandJobDetailResponseByMessageID(deviceID, messageID, status, payload, responseError, responseAt)
	if errors.Is(err, dal.ErrAmbiguousCommandJobDetailResponse) {
		if recordErr := recordAmbiguousFleetCommandJobDeviceResponse(deviceID, messageID, status, responseError); recordErr != nil {
			return recordErr
		}
		return err
	}
	if err != nil || affected == 0 || detail == nil {
		return err
	}

	job, err := dal.GetCommandJobByID(detail.CommandJobID, detail.TenantID)
	if err != nil {
		return err
	}
	if applyFleetCommandJobDeviceAckState(detail, status, responseError, responseAt, job.Status == commandJobStatusCanceled) {
		if err := dal.UpdateCommandJobDetail(detail); err != nil {
			return err
		}
	}

	if err := refreshCommandJobSummary(job); err != nil {
		return err
	}
	recordFleetCommandJobEvent(
		detail.CommandJobID,
		detail.TenantID,
		&detail.ID,
		&detail.DeviceID,
		commandJobDeviceAckEventType(status),
		commandJobDeviceAckEventMessage(messageID, status, responseError),
	)
	return nil
}

func recordAmbiguousFleetCommandJobDeviceResponse(deviceID, messageID, status, responseError string) error {
	candidates, err := dal.FindCommandJobDetailResponseCandidates(deviceID, messageID, 20)
	if err != nil {
		return err
	}
	recordedJobs := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.CommandJobID == "" || candidate.TenantID == "" {
			continue
		}
		if _, exists := recordedJobs[candidate.CommandJobID]; exists {
			continue
		}
		recordedJobs[candidate.CommandJobID] = struct{}{}
		recordFleetCommandJobEvent(
			candidate.CommandJobID,
			candidate.TenantID,
			nil,
			&deviceID,
			commandJobEventDeviceAckAmbiguous,
			commandJobDeviceAckAmbiguousEventMessage(messageID, status, responseError),
		)
	}
	return nil
}

func applyFleetCommandJobDeviceAckState(detail *model.CommandJobDetail, status, responseError string, responseAt time.Time, jobCanceled ...bool) bool {
	if detail == nil {
		return false
	}
	isCanceledJob := len(jobCanceled) > 0 && jobCanceled[0]
	switch status {
	case strconv.Itoa(constant.ResponseSStatusFailed):
		detail.Status = commandJobDetailStatusFailed
		detail.CanRetry = !isCanceledJob && detail.DispatchAttempts < commandJobMaxDispatchAttempts
		if detail.CanRetry {
			retryAfter := commandJobNextRetryAfter(detail.DispatchAttempts, responseAt)
			detail.NextRetryAfter = &retryAfter
		} else {
			detail.NextRetryAfter = nil
		}
		if isCanceledJob {
			detail.Reason = StringPtr(commandJobCanceledDeviceAckFailureReason(responseError))
			detail.Advice = StringPtr("This canceled job received a late device failure acknowledgement; keep the evidence for support and create a fresh preview if the operation still needs to run.")
		} else {
			detail.Reason = StringPtr(commandJobDeviceAckFailureReason(responseError))
			detail.Advice = StringPtr("Device returned a failure response; review the payload and device state before retrying this row.")
		}
		if detail.CompletedAt == nil {
			detail.CompletedAt = &responseAt
		}
		return true
	case strconv.Itoa(constant.ResponseStatusOk):
		detail.Status = commandJobDetailStatusSubmitted
		detail.CanRetry = false
		detail.NextRetryAfter = nil
		detail.Reason = nil
		detail.Advice = nil
		if detail.CompletedAt == nil {
			detail.CompletedAt = &responseAt
		}
		return true
	default:
		return false
	}
}

func commandJobDeviceAckFailureReason(responseError string) string {
	if responseError == "" {
		return "device returned a failure response for this command message"
	}
	return "device returned a failure response: " + responseError
}

func commandJobCanceledDeviceAckFailureReason(responseError string) string {
	if responseError == "" {
		return "canceled job received a late device failure response for this command message"
	}
	return "canceled job received a late device failure response: " + responseError
}

func commandJobDeviceAckEventType(status string) string {
	if status == strconv.Itoa(constant.ResponseSStatusFailed) {
		return commandJobEventDeviceAckFailed
	}
	return commandJobEventDeviceAckSuccess
}

func commandJobDeviceAckEventMessage(messageID, status, responseError string) string {
	message := fmt.Sprintf("device response recorded for message %s with status %s", messageID, status)
	if responseError != "" {
		return message + ": " + responseError
	}
	return message
}

func commandJobDeviceAckAmbiguousEventMessage(messageID, status, responseError string) string {
	message := fmt.Sprintf("device response for message %s with status %s was not applied because multiple command job detail candidates matched this device and message", messageID, status)
	if responseError != "" {
		return message + ": " + responseError
	}
	return message
}
