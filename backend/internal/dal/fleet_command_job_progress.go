// 文件用途：批次作业明细行的进度回写（ROADMAP P0.3，迁移 87）。
// 核心逻辑：把设备上报的进度按"发生时间"条件更新到明细行，并返回可判定的结果分类。
// 关键注意事项：
//  1. 租户隔离：更新与诊断查询都必须带 tenant_id。缺了 tenant_id，一条跨租户的进度
//     上报就会改写别的租户的明细行，而调用方完全无感。
//  2. 终态明细行（failed/canceled）不接受进度：给已经死掉的行写进度等于伪造进展。
//  3. 只有"发生时间不早于当前 progress_at"的上报才被接受。这**不等于**"百分比只增不减"：
//     设备真的回退（例如 OTA 失败重来）会被如实写入，被挡住的只是"到达顺序造成的回退"，
//     即网络重排让旧进度覆盖新进度所显示的假回退。
//  4. 先更新、后诊断：命中时只花一条 SQL；未命中才回查一次原因，让服务层能给出准确语义
//     （设备不属于该批次 / 行已终态 / 进度已过期），而不是笼统地"成功"或"失败"。
package dal

import (
	"errors"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// 进度回写结果分类。服务层据此决定错误码与是否落审计事件，故每个取值都要有明确语义。
const (
	// CommandJobProgressApplied 明细行已被更新。
	CommandJobProgressApplied = "applied"
	// CommandJobProgressNoRow 该设备在此批次下没有明细行（设备不属于这个批次）。
	CommandJobProgressNoRow = "no_row"
	// CommandJobProgressTerminal 明细行处于不接受进度的终态。
	CommandJobProgressTerminal = "terminal"
	// CommandJobProgressStale 上报的发生时间早于行内已有进度，属于乱序到达的旧进度。
	CommandJobProgressStale = "stale"
	// CommandJobProgressConflict 行存在且条件看似满足但更新未命中，通常是并发改写。
	CommandJobProgressConflict = "conflict"
)

// commandJobDetailProgressTerminalStatuses 不接受进度回写的明细行状态。
// 注意与 command_jobs 的终态区分：明细行只有 failed/canceled 是不可再推进的终态；
// blocked/ready 尚未下发，收到进度属于异常数据，但不是终态，暂按正常行处理
// （是否要把"未下发却上报进度"升级为独立的告警，属于监控范畴，不在本迁移内）。
var commandJobDetailProgressTerminalStatuses = []string{"failed", "canceled"}

// FleetCommandJobProgressWriteback 一次明细行进度回写的入参。
type FleetCommandJobProgressWriteback struct {
	JobID    string
	TenantID string
	DeviceID string
	// Percent 0-100，由服务层校验；此处不重复校验，数据库 CHECK 是最后一道闸。
	Percent int
	Status  string
	Error   string
	// At 进度发生时间。零值会让"progress_at <= At"永远为假而无法写入，调用方必须先归一化。
	At time.Time
}

// UpdateFleetCommandJobDetailProgress 把一条设备进度上报写回明细行。
// 返回 CommandJobProgress* 之一；只有返回 error 时才表示数据库本身出错。
func UpdateFleetCommandJobDetailProgress(w FleetCommandJobProgressWriteback) (string, error) {
	updates := map[string]interface{}{
		"progress_percent": w.Percent,
		"progress_at":      w.At,
	}
	// 上报未携带状态/错误时写 NULL：明细行必须是"最近一次被接受的上报"的忠实镜像，
	// 不能让上一次的失败状态残留在一份全新的成功进度上。
	if w.Status == "" {
		updates["progress_status"] = nil
	} else {
		updates["progress_status"] = w.Status
	}
	if w.Error == "" {
		updates["progress_error"] = nil
	} else {
		updates["progress_error"] = w.Error
	}

	result := global.DB.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ? AND tenant_id = ? AND device_id = ?", w.JobID, w.TenantID, w.DeviceID).
		Where("status NOT IN ?", commandJobDetailProgressTerminalStatuses).
		Where("progress_at IS NULL OR progress_at <= ?", w.At).
		Updates(updates)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected > 0 {
		return CommandJobProgressApplied, nil
	}
	return classifyFleetCommandJobProgressMiss(w)
}

// classifyFleetCommandJobProgressMiss 判定更新未命中的原因。
// 顺序要紧：先判终态再判过期，否则一条"已终态且带旧进度"的行会被误报成过期，
// 服务层就会把"行已结束"这个硬错误当成可忽略的重传放过。
func classifyFleetCommandJobProgressMiss(w FleetCommandJobProgressWriteback) (string, error) {
	var detail model.CommandJobDetail
	err := global.DB.Select("id, status, progress_at").
		Where("command_job_id = ? AND tenant_id = ? AND device_id = ?", w.JobID, w.TenantID, w.DeviceID).
		Order("created_at ASC, id ASC").
		First(&detail).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CommandJobProgressNoRow, nil
	}
	if err != nil {
		return "", err
	}
	for _, terminal := range commandJobDetailProgressTerminalStatuses {
		if detail.Status == terminal {
			return CommandJobProgressTerminal, nil
		}
	}
	if detail.ProgressAt != nil && detail.ProgressAt.After(w.At) {
		return CommandJobProgressStale, nil
	}
	return CommandJobProgressConflict, nil
}
