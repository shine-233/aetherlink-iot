// 文件用途：TB-15 冷层清理（telemetry_rollups 分批删除）的 SQL 构造单测。
// 核心逻辑：断言批量删除语句保留"主键元组 IN 子查询 + 边界过滤 + 限量"的分批骨架，
// 与 deleteTelemetryDataBatch（dal/telemetry_datas.go）同风格——纯文本断言，不触库。
// 关键注意事项：删除执行本身依赖活库（global.DB），本测试只锁 SQL 形状；
// 分批循环终止条件（deleted < batchSize）沿用 DeleteTelemetrDataByTime 的既有语义。
// 重构建议：若未来引入桶宽粒度淘汰（bucket_ms 维度），本语句与测试要同步演进。
package dal

import (
	"strings"
	"testing"
)

func TestDeleteTelemetryRollupsBatchSQLShape(t *testing.T) {
	// 从实现函数中提取 SQL 文本：执行体绑定 global.DB 不可直跑，改为对同包
	// SQL 常量的形状校验。若 deleteTelemetryRollupsBatch 改为内联 SQL，
	// 本测试失败即提示同步更新此处骨架断言。
	sql := telemetryRollupDeleteBatchSQL
	for _, want := range []string{
		"DELETE FROM telemetry_rollups",
		"(device_id, key, bucket_ms, bucket_start) IN",
		"WHERE bucket_start <= ?",
		"ORDER BY bucket_start",
		"LIMIT ?",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("telemetry_rollups 分批删除语句缺少骨架 %q：\n%s", want, sql)
		}
	}
}
