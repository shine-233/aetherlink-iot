package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

func CreateReportSchedule(s *model.ReportSchedule) error {
	return global.DB.Create(s).Error
}

func UpdateReportSchedule(s *model.ReportSchedule) error {
	return global.DB.Save(s).Error
}

func DeleteReportSchedule(id, tenantID string) error {
	return global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&model.ReportSchedule{}).Error
}

// GetReportScheduleInTenant 按租户定位单条记录；未命中时返回 gorm.ErrRecordNotFound。
func GetReportScheduleInTenant(id, tenantID string) (*model.ReportSchedule, error) {
	var s model.ReportSchedule
	err := global.DB.
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&s).Error
	return &s, err
}

func ListReportSchedules(tenantID string, limit int) ([]*model.ReportSchedule, error) {
	var list []*model.ReportSchedule
	err := global.DB.
		Where("tenant_id = ?", tenantID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// ListEnabledReportSchedules 返回全部启用中的任务，供 cron 调度扫描（跨租户，因调度在后台运行）。
// tenant-scope: caller-enforced——本函数只做到期任务枚举；租户边界在执行路径强制
// （ExecuteSchedule 按任务行自带 tenant_id 过滤遥测与收件人，见 service/report_schedule.go）。
func ListEnabledReportSchedules() ([]*model.ReportSchedule, error) {
	var list []*model.ReportSchedule
	err := global.DB.
		Where("enabled = ?", true).
		Find(&list).Error
	return list, err
}

// UpdateReportScheduleRunResult 落库最近一次执行时间与状态，供去重与观测。
func UpdateReportScheduleRunResult(id string, lastRunAt interface{}, status string) error {
	return global.DB.Model(&model.ReportSchedule{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"last_run_at": lastRunAt, "last_status": status}).Error
}

// GetTelemetryDataForReport 租户作用域内按设备/测点/时间窗查询遥测，供报表 CSV 生成复用。
// 与 on-demand 导出同源（telemetry_datas 表），但显式加 tenant_id 过滤保证租户隔离。
func GetTelemetryDataForReport(tenantID, deviceID, key string, startMs, endMs int64, limit int) ([]*model.TelemetryData, error) {
	var rows []*model.TelemetryData
	err := global.DB.
		Where("tenant_id = ? AND device_id = ? AND key = ? AND ts >= ? AND ts <= ?", tenantID, deviceID, key, startMs, endMs).
		Order("ts DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
