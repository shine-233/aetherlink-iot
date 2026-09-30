// Package retention 收敛数据库模式生命周期（schema lifecycle）相关的横切测试与工具。
//
// 当前职责：
//   - migration_live_test.go：真实 PostgreSQL 集群上的迁移链实测（受
//     AETHERLINK_MIG_TEST_DSN 环境变量门控，未设置时跳过，不影响常规单测）。
//
// 说明：表级保留期注册表（TB-22，data_retention_registry）的运行时实现并未拆成独立包——
// 批次删除按决策文档挂进既有 cron（service.CleanSystemDataByCron），数据访问原语在
// internal/dal/data_policy_retention.go，cron 编排在 internal/service/datapolicy_retention.go。
// 本包只承载不依赖业务运行时的生命周期验证设施；若后续保留期治理长出独立执行面
// （如带外迁移步、分区改造），应优先落在本包。
package retention
