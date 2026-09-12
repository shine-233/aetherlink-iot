// 文件用途: 读取批次作业明细行用于报告导出（P0.3）。
// 核心逻辑: 以租户 + 作业 ID 为条件读取明细行，含迁移 87 新增的进度列。
// 关键注意事项: 必须带 tenant_id 过滤；进度列用独立行结构读取，因为
//   model.CommandJobDetail 是生成产物，按约定不手改（同 P0.2/P0.4 的做法）。

package dal

import (
	"context"
	"time"

	"aetherlink-iot/backend/pkg/global"
)

// CommandJobReportRowLimit 单次导出读取的明细行上限。
// 导出面向运维排障，必须大于列表页的 inline 上限，但仍要有界——
// 无上限的全表读取会把一次导出变成一次数据库压力事件。
const CommandJobReportRowLimit = 50000

type CommandJobReportRow struct {
	DeviceID     string  `gorm:"column:device_id"`
	DeviceNumber string  `gorm:"column:device_number"`
	Name         string  `gorm:"column:name"`
	Online       bool    `gorm:"column:online"`
	Eligible     bool    `gorm:"column:eligible"`
	Status       string  `gorm:"column:status"`
	// 进度三列均为指针：NULL 表示"从未上报进度"，与"上报了 0%"是两种事实，
	// 导出时不得把 NULL 写成 0（那等于给没进展的设备凭空记一笔"已开始"）。
	ProgressPercent *int       `gorm:"column:progress_percent"`
	ProgressStatus  *string    `gorm:"column:progress_status"`
	ProgressError   *string    `gorm:"column:progress_error"`
	ProgressAt      *time.Time `gorm:"column:progress_at"`
	ResponseStatus  *string    `gorm:"column:response_status"`
	ResponseError   *string    `gorm:"column:response_error"`
	ResponseAt      *time.Time `gorm:"column:response_at"`
	Reason          *string    `gorm:"column:reason"`
}

func clampCommandJobReportLimit(limit int) int {
	if limit <= 0 || limit > CommandJobReportRowLimit {
		return CommandJobReportRowLimit
	}
	return limit
}

// ListFleetCommandJobReportRows 按租户 + 作业 ID 读取导出用明细行。
// 顺序固定（created_at, id），保证同一作业多次导出的行序一致、可比对。
func ListFleetCommandJobReportRows(ctx context.Context, jobID, tenantID string, limit int) ([]CommandJobReportRow, error) {
	var rows []CommandJobReportRow
	err := global.DB.WithContext(ctx).
		Table("command_job_details").
		Select(`device_id, device_number, name, online, eligible, status,
			progress_percent, progress_status, progress_error, progress_at,
			response_status, response_error, response_at, reason`).
		Where("command_job_id = ?", jobID).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC, id ASC").
		Limit(clampCommandJobReportLimit(limit)).
		Find(&rows).Error
	return rows, err
}
