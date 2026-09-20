// 文件用途：批次命令作业事件的去重查询（ROADMAP P0.3 进度消费幂等）。
// 核心逻辑：按 (job, tenant, event_type, message) 精确计数，供服务层判断进度事件是否已消费过。
// 关键注意事项：message 由服务层生成确定性去重令牌，因此同一事件重复上报必然命中同一条记录。
package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// HasCommandJobEventMessage 判断指定作业是否已有相同类型与消息的事件。
func HasCommandJobEventMessage(jobID, tenantID, eventType, message string) (bool, error) {
	var count int64
	err := global.DB.Model(&model.CommandJobEvent{}).
		Where("command_job_id = ? AND tenant_id = ? AND event_type = ? AND message = ?",
			jobID, tenantID, eventType, message).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
