// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"strings"

	"aetherlink-iot/backend/internal/query"
)

// LatestDeviceAlarmQuery 设备告警查询结构体
type LatestDeviceAlarmQuery struct{}

// CountDevicesByTenantAndStatus 根据租户、活动告警状态和可选 owner 范围统计设备数量。
func (q *LatestDeviceAlarmQuery) CountDevicesByTenantAndStatus(ctx context.Context, tenantID string, ownerUserID *string) (int64, error) {
	return q.CountDevicesByScopeAndStatus(ctx, tenantID, ownerUserID, false)
}

// CountDevicesByScopeAndStatus expands beyond one tenant only when the service
// has already authorized an explicit system-administrator request.
func (q *LatestDeviceAlarmQuery) CountDevicesByScopeAndStatus(ctx context.Context, tenantID string, ownerUserID *string, allTenants bool) (int64, error) {
	lda := query.LatestDeviceAlarm
	device := query.Device

	// Only persisted active severities make a device alarmed. Unknown or legacy
	// values must not be promoted to an active alert by a broad != N predicate.
	// Always join the live device row so deleted/inactive historical streams do
	// not inflate the active-system card above the /device list result.
	builder := lda.WithContext(ctx).
		Join(device, lda.DeviceID.EqCol(device.ID), lda.TenantID.EqCol(device.TenantID)).
		Where(lda.AlarmStatus.In("H", "M", "L"), device.ActivateFlag.Eq("active"))
	if !allTenants {
		builder = builder.Where(lda.TenantID.Eq(tenantID), device.TenantID.Eq(tenantID))
	}
	if ownerUserID != nil && strings.TrimSpace(*ownerUserID) != "" {
		builder = builder.Where(device.OwnerUserID.Eq(strings.TrimSpace(*ownerUserID)))
	}
	var count int64
	err := builder.UnderlyingDB().Distinct("latest_device_alarms.device_id").Count(&count).Error
	return count, err
}
