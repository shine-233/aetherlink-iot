package dal

// 文件用途：设备在线/离线趋势（按时段的小时粒度序列）。
// 核心逻辑：整条趋势在一条 SQL 里算完——generate_series 铺小时轴，
//   再用"区间前已在线基数 + 每小时状态跃迁增量"滚动累加，最终与设备总量取 LEAST/GREATEST 夹紧。
// 关键注意事项：
//   - 依赖 PG 专有语法（generate_series / FILTER / 窗口帧），SQLite 测试库跑不了，勿在单测里直调。
//   - 只统计 activate_flag <> 'inactive' 且 created_at <= 区间末的设备，与列表口径保持一致。
//   - ownerFilter 为空串表示不过滤归属（管理员视角），用 ($4 = '' OR ...) 表达，不能改成等值。

import (
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// GetDeviceTrend returns hourly online and offline device counts for the tenant.
// tenantID identifies the tenant scope.
// startTime defaults to the previous 48 hours when omitted.
// endTime defaults to now when omitted.
func GetDeviceTrend(tenantID string, ownerUserID *string, startTime, endTime *int64) ([]model.DeviceTrendPoint, error) {
	now := time.Now()
	if endTime == nil {
		t := now.Unix()
		endTime = &t
	}
	if startTime == nil {
		t := now.Add(-48 * time.Hour).Unix()
		startTime = &t
	}

	startTimeUTC := time.Unix(*startTime, 0).UTC()
	endTimeUTC := time.Unix(*endTime, 0).UTC()

	var results []model.DeviceTrendPoint

	sql := `
WITH
-- 1. Build the hourly time series.
hour_series AS (
    SELECT generate_series AS hour_ts
    FROM generate_series($2::timestamptz, $3::timestamptz, '1 hour') AS generate_series
),
-- 2. Count devices created before the requested end time.
device_total AS (
    SELECT COUNT(*)::bigint AS total_cnt
    FROM devices
    WHERE tenant_id = $1
      AND ($4 = '' OR owner_user_id = $4)
      AND activate_flag <> 'inactive'
      AND created_at <= $3
),
-- 3. Count devices already online before the requested range.
--    Per-device LATERAL probe on idx_device_status_history_tenant_device_id_desc
--    (144.sql): walks id DESC and stops at the first row before $2, so cost is
--    bounded by device count + in-range rows, not by total history depth.
--    Same semantics as the old "MAX(id) per device, then join back" form.
before_online AS (
    SELECT COUNT(*)::bigint AS cnt
    FROM devices d
    CROSS JOIN LATERAL (
        SELECT dsh.status
        FROM device_status_history dsh
        WHERE dsh.tenant_id = $1
          AND dsh.device_id = d.id
          AND dsh.change_time < $2
        ORDER BY dsh.id DESC
        LIMIT 1
    ) latest
    WHERE d.tenant_id = $1
      AND d.activate_flag <> 'inactive'
      AND ($4 = '' OR d.owner_user_id = $4)
      AND latest.status = 1
),
-- 4. Estimate devices that never emitted status changes.
never_reported AS (
    SELECT COUNT(*)::bigint AS cnt
    FROM devices d
    WHERE d.tenant_id = $1
      AND ($4 = '' OR d.owner_user_id = $4)
      AND d.activate_flag <> 'inactive'
      AND d.created_at <= $3
      AND d.is_online = 1
      AND NOT EXISTS (
          SELECT 1
          FROM device_status_history dsh
          WHERE dsh.tenant_id = $1
            AND dsh.device_id = d.id
      )
),
-- 5. Load latest status change per device per hour inside the requested range.
--    DISTINCT ON keeps the MAX(id) row of each (device, hour) in one pass; the
--    old GROUP BY + join-back hashed the whole history table to re-fetch status.
all_changes AS (
    SELECT DISTINCT ON (dsh.device_id, date_trunc('hour', dsh.change_time))
        dsh.device_id,
        dsh.status,
        date_trunc('hour', dsh.change_time) AS hour_ts
    FROM device_status_history dsh
    WHERE dsh.tenant_id = $1
      AND dsh.change_time >= $2
      AND dsh.change_time <= $3
      AND dsh.device_id IN (
          SELECT id FROM devices
          WHERE tenant_id = $1
            AND activate_flag <> 'inactive'
            AND ($4 = '' OR owner_user_id = $4)
      )
    ORDER BY dsh.device_id, date_trunc('hour', dsh.change_time), dsh.id DESC
),
-- 6. Compare each hourly status point with the previous point.
device_prev AS (
    SELECT
        device_id,
        hour_ts,
        status,
        LAG(status) OVER (
            PARTITION BY device_id ORDER BY hour_ts
        ) AS prev_status
    FROM all_changes
),
-- 7. Aggregate per-hour online and offline deltas.
hourly_delta AS (
    SELECT hour_ts,
        COUNT(*) FILTER (
            WHERE status = 1 AND (prev_status IS NULL OR prev_status != 1)
        )::bigint AS online_delta,
        COUNT(*) FILTER (
            WHERE status = 0 AND (prev_status IS NULL OR prev_status != 0)
        )::bigint AS offline_delta
    FROM device_prev
    GROUP BY hour_ts
),
-- 8. Merge the base online count with per-hour deltas.
merged AS (
    SELECT
        s.hour_ts,
        (SELECT cnt FROM before_online) + (SELECT cnt FROM never_reported) AS init_online,
        COALESCE(h.online_delta,  0)::bigint AS od,
        COALESCE(h.offline_delta, 0)::bigint AS fd
    FROM hour_series s
    LEFT JOIN hourly_delta h ON h.hour_ts = s.hour_ts
),
-- 9. Roll forward the online count hour by hour.
--    cur_online = GREATEST(0, prev_online + od - fd)
with_online AS (
    SELECT
        hour_ts,
        GREATEST(
            init_online + SUM(od - fd) OVER (
                ORDER BY hour_ts
                ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
            ), 0
        )::bigint AS cur_online
    FROM merged
)
-- 10. Return hourly total, online, and offline counts.
SELECT
    h.hour_ts                                        AS timestamp,
    t.total_cnt                                      AS device_total,
    LEAST(w.cur_online, t.total_cnt)::bigint         AS device_online,
    GREATEST(t.total_cnt - LEAST(w.cur_online, t.total_cnt), 0)::bigint AS device_offline
FROM with_online w
JOIN merged h ON h.hour_ts = w.hour_ts
CROSS JOIN device_total t
ORDER BY h.hour_ts ASC;
`
	ownerFilter := ""
	if ownerUserID != nil {
		ownerFilter = strings.TrimSpace(*ownerUserID)
	}
	err := global.DB.Raw(sql, tenantID, startTimeUTC, endTimeUTC, ownerFilter).Scan(&results).Error
	if err != nil {
		logrus.Error("GetDeviceTrend query failed")
		return nil, err
	}

	return results, nil
}

// Device trend queries intentionally keep SQL comments close to the CTEs above.
