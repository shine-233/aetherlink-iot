// Command set log and selected-device command job HTTP handlers.
package api

import (
	"strconv"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type CommandSetLogApi struct{}

// ServeSetLogsDataListByPage queries paged command set logs.
// @Router   /api/v1/command/datas/set/logs [get]
func (CommandSetLogApi) ServeSetLogsDataListByPage(c *gin.Context) {
	var req model.GetCommandSetLogsListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	date, err := service.GroupApp.CommandData.GetCommandSetLogsDataListByPage(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", date)
}

// CommandPutMessage manually publishes a command message.
// @Router   /api/v1/command/datas/pub [post]
func (CommandSetLogApi) CommandPutMessage(c *gin.Context) {
	var req model.PutMessageForCommand
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.CommandPutMessageWithTracking(c, userClaims.ID, &req, strconv.Itoa(constant.Manual), userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// InvokeDirectMethod publishes one online-device command and waits up to 30
// seconds for the correlated device response stored in command_set_logs.
// Publish acceptance and device execution remain distinct response fields.
// @Router   /api/v1/command/datas/direct-method [post]
func (CommandSetLogApi) InvokeDirectMethod(c *gin.Context) {
	var req model.DirectMethodCommandReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.InvokeDirectMethod(c.Request.Context(), userClaims.ID, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// PreviewFleetCommandJob validates selected devices or previews a device_filter scope without publishing commands.
// @Summary Preview fleet command job scope
// @Description Validates selected devices or previews a device_filter scope without publishing commands, returning the eligible devices and any blocking reasons.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param request body model.FleetCommandJobReq true "Fleet command job preview payload"
// @Success 200 {object} model.FleetCommandJobPreviewResult "Preview result with eligible device list"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/preview [post]
func (CommandSetLogApi) PreviewFleetCommandJob(c *gin.Context) {
	var req model.FleetCommandJobReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.PreviewFleetCommandJob(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// SubmitFleetCommandJob queues selected-device or capped device_filter command jobs.
// It persists job/detail rows, schedules in-process dispatch, and lets the
// recovery scanner resume runnable rows from the database if a worker drops.
// External distributed queues and full device-execution callbacks are still separate Jobs engine work.
// @Summary Submit a fleet command job
// @Description Persists a command job and per-device detail rows, schedules dispatch, and returns the job summary. Use include_rows=false to omit per-device rows for large jobs.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param request body model.FleetCommandJobReq true "Fleet command job submit payload"
// @Param include_rows query bool false "Include per-device rows in the response (default true)"
// @Success 200 {object} model.FleetCommandJobSubmitResult "Submitted job with optional rows"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/submit [post]
func (CommandSetLogApi) SubmitFleetCommandJob(c *gin.Context) {
	var req model.FleetCommandJobReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	includeRows := c.DefaultQuery("include_rows", "true") != "false"
	data, err := service.GroupApp.CommandData.SubmitFleetCommandJob(c.Request.Context(), userClaims.ID, &req, userClaims, includeRows)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ListFleetCommandJobs returns recent persisted command jobs for the current tenant.
// @Summary List fleet command jobs
// @Description Returns recent persisted command jobs for the current tenant with pagination and attention filters.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param page query int false "Page number (1-based)"
// @Param page_size query int false "Page size"
// @Param status query string false "Filter by job status"
// @Param attention_filter query string false "Filter by attention state"
// @Param search query string false "Free-text search across job metadata"
// @Success 200 {object} model.FleetCommandJobListResult "Paginated command job list"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs [get]
func (CommandSetLogApi) ListFleetCommandJobs(c *gin.Context) {
	req := model.FleetCommandJobListReq{}
	if rawPage := c.Query("page"); rawPage != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err != nil {
			c.Error(errcode.WithData(errcode.CodeParamError, "page must be an integer"))
			return
		}
		req.Page = parsed
	}
	if rawPageSize := c.Query("page_size"); rawPageSize != "" {
		parsed, err := strconv.Atoi(rawPageSize)
		if err != nil {
			c.Error(errcode.WithData(errcode.CodeParamError, "page_size must be an integer"))
			return
		}
		req.PageSize = parsed
	}
	req.Status = c.Query("status")
	req.AttentionFilter = c.Query("attention_filter")
	req.Search = c.Query("search")

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.ListFleetCommandJobs(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetFleetCommandJob returns persisted command job progress.
// Per-device rows are opt-in with include_rows=true; large Jobs should use the paged rows API.
// @Summary Get fleet command job progress
// @Description Returns persisted command job progress. Per-device rows are opt-in with include_rows=true; large Jobs should use the paged rows API.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID"
// @Param include_rows query bool false "Include per-device rows in the response (default false)"
// @Success 200 {object} model.FleetCommandJobSubmitResult "Command job progress payload"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id} [get]
func (CommandSetLogApi) GetFleetCommandJob(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	includeRows := c.DefaultQuery("include_rows", "false") == "true"
	var data *model.FleetCommandJobSubmitResult
	var err error
	if includeRows {
		data, err = service.GroupApp.CommandData.GetFleetCommandJob(jobID, userClaims)
	} else {
		data, err = service.GroupApp.CommandData.GetFleetCommandJobSummary(jobID, userClaims)
	}
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetFleetCommandJobRows returns paged per-device command job rows.
// @Summary Get fleet command job rows
// @Description Returns paged per-device command job rows for the specified job. Use this endpoint for large jobs where full row payloads would be excessive.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID"
// @Param page query int false "Page number (1-based)"
// @Param page_size query int false "Page size"
// @Param status_filter query string false "Filter rows by status"
// @Param search query string false "Free-text search across row metadata"
// @Success 200 {object} model.FleetCommandJobRowsResult "Paginated per-device rows"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id}/rows [get]
func (CommandSetLogApi) GetFleetCommandJobRows(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	req := model.FleetCommandJobRowsReq{}
	if rawPage := c.Query("page"); rawPage != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err != nil {
			c.Error(errcode.WithData(errcode.CodeParamError, "page must be an integer"))
			return
		}
		req.Page = parsed
	}
	if rawPageSize := c.Query("page_size"); rawPageSize != "" {
		parsed, err := strconv.Atoi(rawPageSize)
		if err != nil {
			c.Error(errcode.WithData(errcode.CodeParamError, "page_size must be an integer"))
			return
		}
		req.PageSize = parsed
	}
	req.StatusFilter = c.Query("status_filter")
	req.Search = c.Query("search")

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.GetFleetCommandJobRows(jobID, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetFleetCommandJobSupportBundle returns a copyable troubleshooting package for support handoff.
// @Summary Get fleet command job support bundle
// @Description Returns a copyable troubleshooting package for support handoff, including per-device status, retryable devices, and next actions.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID"
// @Success 200 {object} model.FleetCommandJobSupportBundle "Support bundle payload"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id}/support-bundle [get]
func (CommandSetLogApi) GetFleetCommandJobSupportBundle(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.GetFleetCommandJobSupportBundle(jobID, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CancelFleetCommandJob cancels not-yet-submitted details in a persisted command job.
// @Summary Cancel a fleet command job
// @Description Cancels not-yet-submitted details in a persisted command job. Use include_rows=false to omit per-device rows for large jobs.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID"
// @Param include_rows query bool false "Include per-device rows in the response (default true)"
// @Success 200 {object} model.FleetCommandJobSubmitResult "Cancelled job summary with optional rows"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id}/cancel [post]
func (CommandSetLogApi) CancelFleetCommandJob(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	includeRows := c.DefaultQuery("include_rows", "true") != "false"
	data, err := service.GroupApp.CommandData.CancelFleetCommandJob(jobID, userClaims, includeRows)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// RetryFleetCommandJob retries retryable failed details in a persisted command job.
// @Summary Retry a fleet command job
// @Description Retries retryable failed details in a persisted command job. Use include_rows=false to omit per-device rows for large jobs.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID"
// @Param include_rows query bool false "Include per-device rows in the response (default true)"
// @Success 200 {object} model.FleetCommandJobSubmitResult "Retried job summary with optional rows"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id}/retry [post]
func (CommandSetLogApi) RetryFleetCommandJob(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	includeRows := c.DefaultQuery("include_rows", "true") != "false"
	data, err := service.GroupApp.CommandData.RetryFleetCommandJob(c.Request.Context(), jobID, userClaims.ID, userClaims, includeRows)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// GetCommandDeliveryDiagnostics returns read-only command delivery diagnostics.
// @Router   /api/v1/command/datas/delivery/diagnostics/{device_id} [get]
func (CommandSetLogApi) GetCommandDeliveryDiagnostics(c *gin.Context) {
	deviceID := devicePathID(c)
	if deviceID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "device_id is required"))
		return
	}

	var limit int
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			c.Error(errcode.WithData(errcode.CodeParamError, "limit must be an integer"))
			return
		}
		limit = parsed
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.GetCommandDeliveryDiagnostics(c.Request.Context(), service.CommandDeliveryDiagnosticsReq{
		DeviceID: deviceID,
		Limit:    limit,
	}, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleCommandList queries command metadata for a device.
// @Router   /api/v1/command/datas/{id} [get]
func (CommandSetLogApi) HandleCommandList(c *gin.Context) {
	id := c.Param("id")

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.GetCommonList(c, id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}
