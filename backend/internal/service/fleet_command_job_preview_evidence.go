package service

import (
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
)

type fleetCommandTelemetryEvidence map[string][]*model.TelemetryCurrentData

func loadFleetCommandTelemetryEvidence(deviceIDs []string) (fleetCommandTelemetryEvidence, error) {
	telemetry, err := dal.GetCurrentTelemetryDataEvolutionByDeviceIDs(deviceIDs)
	if err != nil {
		return nil, err
	}
	return fleetCommandTelemetryEvidence(telemetry), nil
}

func attachFleetCommandTelemetryEvidence(deviceID string, row *model.FleetCommandJobPreviewRow) {
	telemetry, err := dal.GetCurrentTelemetryDataEvolution(deviceID)
	if err != nil {
		row.Readiness = append(row.Readiness, "current telemetry unavailable")
		return
	}
	attachFleetCommandTelemetryEvidenceRows(telemetry, row)
}

func attachFleetCommandTelemetryEvidenceFromSnapshot(deviceID string, row *model.FleetCommandJobPreviewRow, snapshot fleetCommandTelemetryEvidence) {
	if snapshot == nil {
		attachFleetCommandTelemetryEvidence(deviceID, row)
		return
	}
	telemetry, ok := snapshot[deviceID]
	if !ok {
		row.Readiness = append(row.Readiness, "no current telemetry yet")
		return
	}
	attachFleetCommandTelemetryEvidenceRows(telemetry, row)
}

func attachFleetCommandTelemetryEvidenceRows(telemetry []*model.TelemetryCurrentData, row *model.FleetCommandJobPreviewRow) {
	row.TelemetryCurrentCount = len(telemetry)
	if len(telemetry) == 0 || telemetry[0] == nil {
		row.Readiness = append(row.Readiness, "no current telemetry yet")
		return
	}

	latest := telemetry[0]
	row.LatestTelemetryKey = latest.Key
	row.LatestTelemetryAt = &latest.T
	row.Readiness = append(row.Readiness, "current telemetry received")
}
