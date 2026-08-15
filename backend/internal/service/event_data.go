// 文件用途：维护设备事件数据查询和事件模型响应服务。
// 核心逻辑：读取事件模型与事件数据，按设备权限和请求条件组装前端事件视图。
// 关键注意事项：事件数据可用于告警和审计，跨租户读取、空设备和坏条件必须 fail-closed。
// 重构建议：抽出事件查询仓储，补齐权限、分页、时间范围和事件 payload 兼容测试。
package service

import (
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

type EventData struct{}

func (*EventData) GetEventDatasListByPage(req *model.GetEventDatasListByPageReq, claims *utils.UserClaims) (interface{}, error) {
	if _, err := ensureTelemetryDeviceReadAccess(req.DeviceId, claims); err != nil {
		return nil, err
	}

	count, data, err := dal.GetEventDatasListByPage(req)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	dataMap := make(map[string]interface{})
	dataMap["count"] = count
	dataMap["list"] = data

	return dataMap, nil
}
