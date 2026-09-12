// File purpose: HTTP layer for lightweight telemetry analytics (ROADMAP P2.2).
// Core logic: bind the query, resolve tenant from claims, delegate to the analysis service.
// Key notes:
//   - tenant comes from claims only; device access is re-checked per device by the service.
//   - Undefined percentages are returned as null + a reason string, never as 0.
//   - Export is opt-in via `format`; without it the endpoint is a pure read.

package api

import (
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type TelemetryAnalysisApi struct{}

// AnalyzeTelemetry 轻量分析：多设备聚合 + 可选同比/环比 + 可选导出。
// @Router /api/v1/telemetry/analysis [post]
func (*TelemetryAnalysisApi) AnalyzeTelemetry(c *gin.Context) {
	var req struct {
		DeviceIDs      []string `json:"device_ids" validate:"required,min=1,max=50,dive,max=36"`
		Key            string   `json:"key" validate:"required,max=255"`
		StartTime      int64    `json:"start_time" validate:"required"`
		EndTime        int64    `json:"end_time" validate:"required"`
		Aggregate      string   `json:"aggregate" validate:"omitempty,oneof=avg sum min max count last"`
		Compare        string   `json:"compare" validate:"omitempty,oneof=none previous_period same_period_last"`
		CompareOffsets int      `json:"compare_offsets" validate:"omitempty,min=1,max=12"`
		Format         string   `json:"format" validate:"omitempty,oneof=csv xlsx"`
	}
	if !BindAndValidate(c, &req) {
		return
	}

	claims := c.MustGet("claims").(*utils.UserClaims)
	result, err := service.RunTelemetryAnalysis(c, model.TelemetryAnalysisQuery{
		DeviceIDs:      req.DeviceIDs,
		Key:            strings.TrimSpace(req.Key),
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		Aggregate:      req.Aggregate,
		Compare:        req.Compare,
		CompareOffsets: req.CompareOffsets,
		Format:         req.Format,
	}, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", result)
}

// ExportTelemetryAnalysis 导出分析结果（csv / xlsx），列序与 GET 结果一致。
// @Router /api/v1/telemetry/analysis/export [post]
func (*TelemetryAnalysisApi) ExportTelemetryAnalysis(c *gin.Context) {
	var req struct {
		DeviceIDs []string `json:"device_ids" validate:"required,min=1,max=50,dive,max=36"`
		Key       string   `json:"key" validate:"required,max=255"`
		StartTime int64    `json:"start_time" validate:"required"`
		EndTime   int64    `json:"end_time" validate:"required"`
		Aggregate string   `json:"aggregate" validate:"omitempty,oneof=avg sum min max count last"`
		Compare   string   `json:"compare" validate:"omitempty,oneof=none previous_period same_period_last"`
		Format    string   `json:"format" validate:"omitempty,oneof=csv xlsx"`
	}
	if !BindAndValidate(c, &req) {
		return
	}
	format := req.Format
	if format == "" {
		format = model.TelemetryAnalysisFormatXLSX
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	result, err := service.RunTelemetryAnalysis(c, model.TelemetryAnalysisQuery{
		DeviceIDs: req.DeviceIDs,
		Key:       strings.TrimSpace(req.Key),
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Aggregate: req.Aggregate,
		Compare:   req.Compare,
		Format:    format,
	}, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"file_path": result.ExportPath, "format": result.Format})
}
