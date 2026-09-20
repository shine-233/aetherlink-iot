// Command set log and selected-device command job HTTP handlers.
package api

import (
	"strconv"
	"time"

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
// @Param page query int false "Page number (1-based, default 1)"
// @Param page_size query int false "Page size (default 10, max 50)"
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

// GetFleetCommandJobReport 导出批次作业明细行报告（P0.3）。
// format=csv 时返回 CSV 文本，format=json 时返回结构化行。
// @Summary Export fleet command job report
// @Description Exports per-device detail rows for a command job as CSV or JSON. NULL progress columns export as empty, never as 0.
// @Tags CommandJobs
// @Accept json
// @Produce text/csv
// @Param job_id path string true "Command job ID"
// @Param format query string false "csv or json" default(csv)
// @Param limit query int false "Row limit"
// @Success 200 {object} service.FleetCommandJobReport
// @Router /api/v1/command/datas/jobs/{job_id}/report [get]
func (CommandSetLogApi) GetFleetCommandJobReport(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	var limit int
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			c.Error(errcode.WithData(errcode.CodeParamError, "limit must be a non-negative integer"))
			return
		}
		limit = parsed
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.CommandData.GetFleetCommandJobReport(
		jobID, c.Query("format"), limit, userClaims,
	)
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

// PauseFleetCommandJob 暂停批次派发（ROADMAP P0.3）。
//
// 端点存在的理由：状态机、暂停清 next_dispatch_at、恢复置 now 并触发派发这三件事
// 此前只在服务层存在（fleet_command_job_state_machine.go），全库除单测外没有任何
// 调用方——运维无法真正暂停一个批次。这里补上 HTTP 入口使能力可达。
// 非法转移（含"终态批次不可复活"）由服务层返回 CodeOpDenied 并带双向状态，此处不吞错。
// @Summary Pause a fleet command job
// @Description Moves a scheduled/running job to paused and clears next_dispatch_at so the worker stops dispatching. Idempotent when already paused and emits no second event.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID"
// @Param include_rows query bool false "Include per-device rows in the response (default true)"
// @Success 200 {object} model.FleetCommandJobSubmitResult "Paused job summary with optional rows"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id}/pause [post]
func (CommandSetLogApi) PauseFleetCommandJob(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	includeRows := c.DefaultQuery("include_rows", "true") != "false"
	data, err := service.GroupApp.CommandData.PauseFleetCommandJob(jobID, userClaims, includeRows)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ResumeFleetCommandJob 恢复被暂停的批次（ROADMAP P0.3）。
// 只有 paused 可恢复；终态批次必须先重试或新建，服务层拒绝"复活"并返回 CodeOpDenied。
// 注意与 worker 故障自愈（commandJobEventResumed）是两件事，不可混用。
// @Summary Resume a paused fleet command job
// @Description Moves a paused job back to running, sets next_dispatch_at to now and triggers one dispatch immediately.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID"
// @Param include_rows query bool false "Include per-device rows in the response (default true)"
// @Success 200 {object} model.FleetCommandJobSubmitResult "Resumed job summary with optional rows"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id}/resume [post]
func (CommandSetLogApi) ResumeFleetCommandJob(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	includeRows := c.DefaultQuery("include_rows", "true") != "false"
	data, err := service.GroupApp.CommandData.ResumeFleetCommandJob(c.Request.Context(), jobID, userClaims.ID, userClaims, includeRows)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// RollbackFleetCommandJob 基于已结束批次创建一个回滚批次（ROADMAP P0.3）。
// 回滚不改写原批次：原批次历史保持只读，新批次与原批次双向留下 rollback 审计事件。
// 只允许对 completed/partially_failed/failed 执行，其余状态服务层返回 CodeOpDenied。
// @Summary Roll back a finished fleet command job
// @Description Creates a NEW job from the submitted payload and leaves the original job read-only, recording rollback audit events on both sides.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID to roll back"
// @Param request body model.FleetCommandJobReq true "Payload for the rollback job"
// @Param include_rows query bool false "Include per-device rows in the response (default true)"
// @Success 200 {object} model.FleetCommandJobSubmitResult "Newly created rollback job with optional rows"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id}/rollback [post]
func (CommandSetLogApi) RollbackFleetCommandJob(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	var req model.FleetCommandJobReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	includeRows := c.DefaultQuery("include_rows", "true") != "false"
	data, err := service.GroupApp.CommandData.RollbackFleetCommandJob(c.Request.Context(), jobID, userClaims.ID, &req, userClaims, includeRows)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// fleetCommandJobProgressReq 批次进度上报载荷（ROADMAP P0.3）。
// 刻意不收 tenant_id 与 job_id：tenant 一律取 claims，job 一律取路径参数。
// 允许调用方指定租户等于把归属判定交出去，允许指定 job 则会让进度串到别的批次上。
type fleetCommandJobProgressReq struct {
	DeviceID string     `json:"device_id" binding:"required"`
	Percent  int        `json:"percent" binding:"min=0,max=100"`
	Status   string     `json:"status"`
	Error    string     `json:"error"`
	At       *time.Time `json:"at"`
}

// ConsumeFleetCommandJobProgress 消费一条设备/网关上报的批次进度（ROADMAP P0.3）。
//
// 语义如实说明：服务层对"重复上报被幂等忽略"与"乱序旧进度被丢弃"同样返回 nil，
// 因此本端点返回成功只表示**已接受**，不表示这一次上报真的推进了进度。
// 想确认推进要看明细行的 progress_percent/progress_at（迁移 87 新增列），
// 不能靠本端点的响应次数当作进度——那正是 P0.3 去重令牌要防的"用事件数量伪造推进速度"。
//
// 注：设备侧 OTA 进度目前仍走另一条链路（uplink/event.go → RecordOTAProgress →
// ota_upgrade_task_details）。两条进度模型并存，本端点服务的是 command_job_details
// 这一侧（协议 stub / 网关 / 服务端集成）。是否统一属于独立决策，见 ROADMAP P0.3。
// @Summary Report progress for a device in a fleet command job
// @Description Consumes one device progress report and writes it back to the job detail row. Duplicate and out-of-order reports are accepted but ignored.
// @Tags CommandJobs
// @Accept json
// @Produce json
// @Param job_id path string true "Fleet command job ID"
// @Param request body fleetCommandJobProgressReq true "Progress report payload"
// @Success 200 {object} map[string]interface{} "Accepted acknowledgement"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router   /api/v1/command/datas/jobs/{job_id}/progress [post]
func (CommandSetLogApi) ConsumeFleetCommandJobProgress(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.Error(errcode.WithData(errcode.CodeParamError, "job_id is required"))
		return
	}

	var req fleetCommandJobProgressReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	event := service.FleetCommandJobProgressEvent{
		JobID:    jobID,
		TenantID: userClaims.TenantID,
		DeviceID: req.DeviceID,
		Percent:  req.Percent,
		Status:   req.Status,
		Error:    req.Error,
	}
	// At 为零值时服务层会回退到"现在"（progress_at 的比较以它为准，零值会让
	// "progress_at <= At" 恒为假，进度将永久写不进明细行），故这里允许省略。
	if req.At != nil {
		event.At = *req.At
	}

	if err := service.GroupApp.CommandData.ConsumeFleetCommandJobProgress(c.Request.Context(), event); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"job_id": jobID, "device_id": req.DeviceID, "accepted": true})
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
