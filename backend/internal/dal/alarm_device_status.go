// 本文件负责「设备告警状态」判定与「告警名称」缓存。
//
// 从 alarm.go 按聚合拆出（wave5 be-dal-repositories 轨道）。这两块都是跨配置/信息/历史
// 的横切读取：设备是否有生效告警、告警 id 到名称的缓存读写。
//
// 关键约束：名称缓存失效必须显式调用 DeleteAlarmNameCache，不要依赖过期时间兜底。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

func GetDeviceAlarmStatus(req *model.GetDeviceAlarmStatusReq, tenantID string) (bool, error) {
	latest := query.LatestDeviceAlarm
	result, err := latest.Where(
		latest.TenantID.Eq(tenantID),
		latest.DeviceID.Eq(req.DeviceId),
	).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if result.AlarmStatus == nil {
		return false, nil
	}
	switch strings.ToUpper(strings.TrimSpace(*result.AlarmStatus)) {
	case "H", "M", "L":
		return true, nil
	default:
		return false, nil
	}
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetAlarmNameWithCache(alarmId string) string {
	redis := global.REDIS
	cacheKey := fmt.Sprintf("GetAlarmNameWithCache:alarmId:%s", alarmId)
	var result string
	err := redis.Get(context.Background(), cacheKey).Scan(&result)
	if err == nil && result != "" {
		return result
	}
	alarmConfig, err := query.AlarmConfig.Where(query.AlarmConfig.ID.Eq(alarmId)).Select(query.AlarmConfig.Name).First()
	if err != nil {
		return ""
	}
	redis.Set(context.Background(), cacheKey, alarmConfig.Name, time.Hour)
	return alarmConfig.Name
}

// DeleteAlarmNameCache 删除告警名称缓存。
func DeleteAlarmNameCache(alarmId string) error {
	redis := global.REDIS
	cacheKey := fmt.Sprintf("GetAlarmNameWithCache:alarmId:%s", alarmId)
	return redis.Del(context.Background(), cacheKey).Err()
}

// newAlarmConfigListScopedDB 构造告警配置列表的 raw 语句根与过滤条件，
// 条件语义与收敛前的 applyAlarmConfigListFilters 逐条对齐。
// 空租户守卫（ROADMAP A1）：租户为空且未显式声明全租户视角时拒绝查询，
// 防止条件过滤被静默跳过后退化为跨租户全表扫描（与 board/users 收敛模式一致）。
