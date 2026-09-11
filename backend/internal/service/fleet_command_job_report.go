// 文件用途: P0.3 报告导出——把批次作业明细行导出为可比对的结构化报告。
// 核心逻辑: 服务层做租户与参数校验并调用 DAL 读取，CSV 渲染为纯函数以便无数据库验证。
// 关键注意事项: 进度三列 NULL 必须导出为空串而非 0。NULL 表示"从未上报"，
//   写成 0 等于给没有进展的设备凭空记一笔"已开始 0%"，是伪造进度。

package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// FleetCommandJobReport 导出报告的结果载体。
type FleetCommandJobReport struct {
	JobID     string                    `json:"job_id"`
	Format    string                    `json:"format"`
	Truncated bool                      `json:"truncated"`
	RowCount  int                       `json:"row_count"`
	Rows      []dal.CommandJobReportRow `json:"rows,omitempty"`
	CSV       string                    `json:"csv,omitempty"`
}

// fleetCommandJobReportColumns 导出列顺序。固定顺序保证同一作业多次导出的
// 文件可以按行直接比对（运维是靠 diff 看变化的）。
var fleetCommandJobReportColumns = []string{
	"device_id",
	"device_number",
	"name",
	"online",
	"eligible",
	"status",
	"progress_percent",
	"progress_status",
	"progress_error",
	"progress_at",
	"response_status",
	"response_error",
	"response_at",
	"reason",
}

// GetFleetCommandJobReport 读取一个批次作业的明细行并渲染成报告。
// limit <= 0 表示采用 DAL 的上限；超过上限同样收敛到上限，
// 因此调用方无法通过传大数把一次导出变成全表读取。
func (c *CommandData) GetFleetCommandJobReport(jobID string, format string, limit int, claims *utils.UserClaims) (*FleetCommandJobReport, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	if jobID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "job_id is required")
	}
	if format == "" {
		format = "csv"
	}
	switch format {
	case "csv", "json":
	default:
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "format must be csv or json")
	}
	if limit <= 0 {
		limit = dal.CommandJobReportRowLimit
	}

	rows, err := dal.ListFleetCommandJobReportRows(context.Background(), jobID, claims.TenantID, limit)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	report := &FleetCommandJobReport{
		JobID:     jobID,
		Format:    format,
		RowCount:  len(rows),
		Truncated: len(rows) >= dal.CommandJobReportRowLimit,
	}
	if format == "csv" {
		report.CSV = FormatFleetCommandJobReportCSV(rows)
		return report, nil
	}
	report.Rows = rows
	return report, nil
}

// FormatFleetCommandJobReportCSV 把明细行渲染成 CSV 文本（纯函数，无 IO）。
func FormatFleetCommandJobReportCSV(rows []dal.CommandJobReportRow) string {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)
	// 表头始终写出：空结果也给出可解析的空报表，而不是无法判断是"没数据"还是"坏了"的空文件。
	_ = writer.Write(fleetCommandJobReportColumns)
	for _, row := range rows {
		_ = writer.Write([]string{
			// device_id 保持原样：它由系统分配，且运维要靠它做 vlookup 关联，
			// 前置单引号会让关联全部失效。device_number / name 属用户可控内容，需消毒。
			row.DeviceID,
			sanitizeReportCell(row.DeviceNumber),
			sanitizeReportCell(row.Name),
			strconv.FormatBool(row.Online),
			strconv.FormatBool(row.Eligible),
			row.Status,
			formatReportInt(row.ProgressPercent),
			sanitizeReportCell(formatReportString(row.ProgressStatus)),
			sanitizeReportCell(formatReportString(row.ProgressError)),
			formatReportTime(row.ProgressAt),
			sanitizeReportCell(formatReportString(row.ResponseStatus)),
			sanitizeReportCell(formatReportString(row.ResponseError)),
			formatReportTime(row.ResponseAt),
			sanitizeReportCell(formatReportString(row.Reason)),
		})
	}
	writer.Flush()
	return buffer.String()
}

// sanitizeReportCell 抑制 CSV 公式注入。
// 以 = + - @ 或制表符/回车开头的字段会被 Excel、Sheets 等当成公式执行，
// 设备名与错误信息是设备侧可控内容，直接导出等于给了注入点。
// 做法是前置单引号：表格软件按文本显示，值本身不丢（去掉首字符即可还原）。
func sanitizeReportCell(value string) string {
	if value == "" {
		return ""
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	}
	return value
}

// formatReportInt 保留 NULL 与 0 的区别：NULL 导出为空，0 导出为 "0"。
func formatReportInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func formatReportString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func formatReportTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
