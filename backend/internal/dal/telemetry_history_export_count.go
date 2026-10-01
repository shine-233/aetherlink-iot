// 文件用途：遥测历史导出前的行数上限校验（只数到 limit+1 行即停）。
//
// 旧路径复用 GetHistoryTelemetrDataByPage：导出请求通常不带 page/page_size，于是先
// COUNT(*) 全区间、再把全区间所有行整宽读进内存，仅为拿 total 与上限比较。
// PG 17.5 实测单设备单 key 200 万行：旧校验约 1.0s 且载入 200 万个结构体；
// 本函数 30ms，内存 O(1)。
package dal

import (
	"context"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// CountHistoryTelemetrDataUpTo 返回 min(区间内行数, limit+1)。
// 调用方用"结果 > limit"判断是否超限；返回值不是精确总数，不能用于分页 total。
// tenant-scope: caller-enforced（与 GetHistoryTelemetrDataByPage 一致，设备访问权在 service 层校验）。
func CountHistoryTelemetrDataUpTo(p *model.GetTelemetryHistoryDataByPageReq, limit int64) (int64, error) {
	if usesTelemetryQueryClient() {
		// 遥测查询服务没有计数接口，沿用旧行为（取回后计数）。
		total, _, err := GetHistoryTelemetrDataByPage(p)
		return total, err
	}
	if limit < 0 {
		limit = 0
	}
	capped := global.DB.WithContext(context.Background()).
		Model(&model.TelemetryData{}).
		Select("1").
		Where("device_id = ? AND key = ? AND ts BETWEEN ? AND ?", p.DeviceID, p.Key, p.StartTime, p.EndTime).
		Limit(int(limit + 1))
	var count int64
	err := global.DB.WithContext(context.Background()).Table("(?) AS capped_rows", capped).Count(&count).Error
	return count, err
}
