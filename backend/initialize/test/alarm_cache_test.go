//go:build integration

// 文件用途：验证告警缓存初始化后的写入、按分组查询、按场景查询和删除行为。
// 核心逻辑：加载本地配置并连接 Redis，检查告警缓存对设备、分组和场景自动化的索引。
// 维护提示：本文件带 integration 标签，运行前需确认 conf-localdev.yml 和 Redis 可用。
// 关键注意事项：集成测试依赖 conf-localdev.yml 与 Redis，默认 go test 只能验证包边界不能证明缓存闭环。
// 重构建议：可抽出 Redis client 与配置加载 seam，用 fake 存储覆盖缓存索引规则的单元测试。

package test

import (
	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/pkg/global"
	"testing"

	"github.com/sirupsen/logrus"
)

func init() {
	initialize.ViperInit("../../configs/conf-localdev.yml")
	initialize.LogInIt()
	initialize.RedisInit()
	alarmCache = initialize.NewAlarmCache()

}

var (
	alarmCache          *initialize.AlarmCache
	group_id            = "group_id_1234"
	scene_automation_id = "scene_automation_id1234"
	device_ids          = []string{"device_id123", "device_id456"}
	contents            = []string{"温度大于30", "湿度大于27"}
)

func requireAlarmCache(t *testing.T) {
	t.Helper()
	if alarmCache == nil || global.REDIS == nil {
		t.Skip("redis is not configured for alarm cache integration test")
	}
}

func TestSetDevice(t *testing.T) {
	requireAlarmCache(t)
	logrus.Debug("单元测试开始执行:")
	err := alarmCache.SetDevice(group_id, scene_automation_id, device_ids, contents)
	if err != nil {
		t.Error("设置告警缓存失败", err)
	}

	res1, err := alarmCache.GetByGroupId(group_id)
	if err != nil {
		t.Error("查询告警缓存失败1", err)
	}
	t.Logf("res:%#v", res1)
	res2, err := alarmCache.GetBySceneAutomationId(scene_automation_id)
	if err != nil {
		t.Error("查询告警缓存失败2", err)
	}
	t.Logf("res:%#v", res2)

}

func TestDeleteBygroupId(t *testing.T) {
	requireAlarmCache(t)
	t.Log("测试删除缓存...")

	err := alarmCache.DeleteBygroupId(group_id)
	if err != nil {
		t.Error("设置告警缓存失败", err)
	}
}
