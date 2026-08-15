// 文件用途：维护操作日志查询和管理后台审计记录服务。
// 核心逻辑：按用户、租户、时间和操作类型读取日志，并整理分页响应。
// 关键注意事项：操作日志是审计证据，过滤条件、脱敏字段和跨租户隔离不能被破坏。
// 重构建议：抽出日志查询仓储，补齐权限、分页、时间范围和敏感字段脱敏测试。
package service

import (
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

type OperationLogs struct{}

func normalizeOperationLogList(list interface{}) interface{} {
	if rows, ok := list.([]model.GetOperationLogListByPageRsp); ok && rows == nil {
		return make([]model.GetOperationLogListByPageRsp, 0)
	}
	return list
}

func (*OperationLogs) GetListByPage(params *model.GetOperationLogListByPageReq, userClaims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetListByPage(params, userClaims)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	// Keep successful empty pages JSON-stable. GORM may leave a scanned
	// zero-row slice nil even though the DAL starts with an empty slice.
	list = normalizeOperationLogList(list)

	response := map[string]interface{}{
		"total": total,
		"list":  list,
	}

	return response, nil
}
