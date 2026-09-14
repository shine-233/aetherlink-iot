// 文件用途：P3 审计导出——操作日志按时间窗导出 CSV，供合同审计/离线核查。
// 核心逻辑：租户作用域与既有列表查询同口径（当前登录用户租户）；必填时间窗；
// 行数上限防导出变成全表拉取。
//
// 关键注意事项：
//   - **不导出 request_message / response_message 载荷**：审计最小化约定
//     （与 market_import_audit 同源）——请求体可能带凭证/业务敏感数据，
//     导出文件会脱离库的访问控制，成为第二份无保护副本。
//   - 租户作用域刻意沿用 GetListByPage 的保守口径（仅本租户）：
//     该查询注释明确要求"恢复跨租户查询必须补权限测试"，导出是更高敏的出口，
//     不先于列表开放跨租户。
package service

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

const (
	auditExportDir       = "./files/export/"
	auditExportMaxRows   = 100000
	auditExportMaxWindow = 366 * 24 * time.Hour // 单次导出窗口不超过一年
)

// AuditLogExportResult 导出结果（与 telemetry 分析导出同一信封）。
type AuditLogExportResult struct {
	FilePath string `json:"file_path"`
	Format   string `json:"format"`
	Rows     int    `json:"rows"`
}

// ExportAuditLogs 导出当前租户在时间窗内的操作日志 CSV。
func ExportAuditLogs(req model.AuditLogExportReq, claims *utils.UserClaims) (*AuditLogExportResult, error) {
	if claims == nil || claims.TenantID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "tenant context is required")
	}
	if req.StartTime == nil || req.EndTime == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "start_time and end_time are required")
	}
	window := req.EndTime.Sub(*req.StartTime)
	if window <= 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "end_time must be after start_time")
	}
	if window > auditExportMaxWindow {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "export window exceeds one year; narrow the range")
	}
	rows, err := dal.ListOperationLogsForExport(claims.TenantID, *req.StartTime, *req.EndTime, auditExportMaxRows)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if len(rows) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "no audit logs in the requested window")
	}

	if mkErr := os.MkdirAll(auditExportDir, 0o755); mkErr != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{"error": mkErr.Error()})
	}
	fileName := fmt.Sprintf("audit-log-%s-%d.csv", claims.TenantID, time.Now().UnixMilli())
	filePath := filepath.Join(auditExportDir, fileName)
	file, cErr := os.Create(filePath)
	if cErr != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{"error": cErr.Error()})
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	header := []string{"id", "created_at", "user_id", "tenant_id", "ip", "path", "name", "latency_ms", "remark"}
	if wErr := writer.Write(header); wErr != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{"error": wErr.Error()})
	}
	for _, row := range rows {
		record := []string{
			row.ID,
			row.CreatedAt.UTC().Format(time.RFC3339),
			row.UserID,
			row.TenantID,
			row.IP,
			derefAuditString(row.Path),
			derefAuditString(row.Name),
			fmt.Sprintf("%d", derefAuditInt64(row.Latency)),
			derefAuditString(row.Remark),
		}
		if wErr := writer.Write(record); wErr != nil {
			return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{"error": wErr.Error()})
		}
	}
	writer.Flush()
	if fErr := writer.Error(); fErr != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{"error": fErr.Error()})
	}
	return &AuditLogExportResult{FilePath: filePath, Format: "csv", Rows: len(rows)}, nil
}

func derefAuditString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefAuditInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
