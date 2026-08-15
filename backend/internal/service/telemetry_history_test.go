package service

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestNormalizeTelemetryHistoryPageTimeRangeConvertsUnixSeconds(t *testing.T) {
	req := &model.GetTelemetryHistoryDataByPageReq{
		StartTime: 1_756_000_000,
		EndTime:   1_756_003_600,
	}

	normalizeTelemetryHistoryPageTimeRange(req)

	if req.StartTime != 1_756_000_000_000 || req.EndTime != 1_756_003_600_000 {
		t.Fatalf("normalized range = (%d, %d), want millisecond bounds", req.StartTime, req.EndTime)
	}
}

func TestNormalizeTelemetryHistoryPageTimeRangeKeepsMilliseconds(t *testing.T) {
	req := &model.GetTelemetryHistoryDataByPageReq{
		StartTime: 1_756_000_000_000,
		EndTime:   1_756_003_600_000,
	}

	normalizeTelemetryHistoryPageTimeRange(req)

	if req.StartTime != 1_756_000_000_000 || req.EndTime != 1_756_003_600_000 {
		t.Fatalf("millisecond range changed to (%d, %d)", req.StartTime, req.EndTime)
	}
}
