// 文件用途：api_usage_daily（TB-17 API 日调用计量）的持久化访问层。
// 核心逻辑：GetAPIUsage 读指定租户/日期的累计调用数（无行返回 0）；UpsertAPIUsage 以
// GREATEST 语义幂等落库——多实例并发落库时取大者，防止慢实例的旧计数回拨当日累计。
// 关键注意事项：计量链路 fail-open——本层错误由调用方（internal/quota）吞掉并放行请求，
// 绝不因计量失败阻断业务；usage_date 以 UTC 日期字符串与 internal/quota.UsageDate 对齐。
// 重构建议：若后续新增 transport 维度计量，可复用本 upsert 形态抽成泛型工具。
package dal

import (
	"errors"
	"strings"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

var errAPIUsageDBNotInitialized = errors.New("database not initialized for api usage metering")

// GetAPIUsage 查询租户在指定 UTC 日期的 API 调用累计数；无记录返回 0。
// tenant-scope: caller-enforced —— 调用方必须传 tenant_id，本函数强制按 tenant_id + usage_date 过滤。
func GetAPIUsage(tenantID, date string) (int64, error) {
	if global.DB == nil {
		return 0, errAPIUsageDBNotInitialized
	}
	var row model.ApiUsageDaily
	err := global.DB.Table("api_usage_daily").
		Where("tenant_id = ? AND usage_date = ?", strings.TrimSpace(tenantID), strings.TrimSpace(date)).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return row.APICalls, nil
}

// UpsertAPIUsage 幂等累加落库：不存在则插入，存在则取已有值与本次值的较大者。
// 取 GREATEST 而非直接覆盖：计数器的真值在 Redis（多实例共享），本表是定期落库的
// 快照，慢实例可能拿着偏旧的计数落库——GREATEST 保证当日累计只增不减。
func UpsertAPIUsage(tenantID, date string, calls int64) error {
	if global.DB == nil {
		return errAPIUsageDBNotInitialized
	}
	return global.DB.Exec(`
INSERT INTO api_usage_daily (tenant_id, usage_date, api_calls, created_at, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (tenant_id, usage_date) DO UPDATE
SET api_calls = GREATEST(api_usage_daily.api_calls, EXCLUDED.api_calls),
    updated_at = CURRENT_TIMESTAMP
`, strings.TrimSpace(tenantID), strings.TrimSpace(date), calls).Error
}
