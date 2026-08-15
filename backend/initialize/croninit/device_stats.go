// 文件用途：注册并执行租户设备状态统计定时任务，为看板类场景沉淀周期快照。
// 核心逻辑：按小时遍历租户，获取设备总量与在线量，并把 JSON 快照写入 Redis 列表。
// 关键注意事项：该任务会周期性触发数据库/Redis 访问，修改频率、键格式或保留时长前需评估兼容性。
// 重构建议：后续可将采集、序列化和存储拆分，提升可测性并降低任务函数体积。

package croninit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
)

// DeviceStatsData 表示单次统计写入 Redis 的设备状态快照。
type DeviceStatsData struct {
	DeviceTotal int64     `json:"device_total"`
	DeviceOn    int64     `json:"device_on"`
	Timestamp   time.Time `json:"timestamp"`
}

const (
	// Redis 键模式: device_stats:{tenant_id}:{date}
	deviceStatsKeyPattern = "device_stats:%s:%s"
	// 数据保留时间（48 小时）
	dataRetentionPeriod = 48 * time.Hour
)

// InitDeviceStatsCron 向调度器注册整点执行的设备统计任务。
func InitDeviceStatsCron(c *cron.Cron) {
	// 每小时整点执行
	c.AddFunc("0 0 * * * *", func() {
		collectDeviceStats()
	})
}

// collectDeviceStats 遍历租户并写入当日设备统计快照。
func collectDeviceStats() {
	ctx := context.Background()
	logrus.Info("开始执行设备状态统计任务")

	// 获取所有租户ID列表
	userList, err := dal.UserVo{}.GetTenantAdminList()
	if err != nil {
		logrus.Errorf("获取租户列表失败: %v", err)
		return
	}

	currentTime := time.Now()
	dateStr := currentTime.Format("2006-01-02")

	for _, user := range userList {
		if user.TenantID == nil || *user.TenantID == "" {
			continue
		}
		claims := &utils.UserClaims{
			ID:        user.ID,
			Authority: constant.TENANT_ADMIN,
			TenantID:  *user.TenantID,
		}

		// 获取该租户的设备统计数据
		deviceStats, err := service.GroupApp.Board.GetDeviceByTenantID(ctx, claims)
		if err != nil {
			logrus.Errorf("获取租户 %s 的设备统计数据失败: %v", *user.TenantID, err)
			continue
		}

		// 构建统计数据
		statsData := DeviceStatsData{
			DeviceTotal: deviceStats.DeviceTotal,
			DeviceOn:    deviceStats.DeviceOn,
			Timestamp:   currentTime,
		}

		// 序列化数据
		statsJSON, err := json.Marshal(statsData)
		if err != nil {
			logrus.Errorf("序列化统计数据失败: %v", err)
			continue
		}

		// 构建Redis键
		key := fmt.Sprintf(deviceStatsKeyPattern, *user.TenantID, dateStr)

		// 将数据存储到Redis List中
		err = global.REDIS.RPush(ctx, key, string(statsJSON)).Err()
		if err != nil {
			logrus.Errorf("存储统计数据到Redis失败: %v", err)
			continue
		}

		// 设置过期时间
		err = global.REDIS.Expire(ctx, key, dataRetentionPeriod).Err()
		if err != nil {
			logrus.Errorf("设置Redis key过期时间失败: %v", err)
		}
	}

	logrus.Info("设备状态统计任务执行完成")
}
