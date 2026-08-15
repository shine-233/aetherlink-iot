# 审查报告目录（已退役）

> [!IMPORTANT]
> 2026-07-28：本目录原有的 7 份审查/计划/追踪报告已**永久删除**（未进 git，不可恢复）。
> 原需求追踪矩阵、缺口计划等已退役，**不再阅读、不再引用、不再维护，也不再作为现行入口**。
> 本 README 是 `.gitignore` 白名单里唯一保留的文件（`audit_reports/*` 被整体排除）。

## 现在去哪里找

| 你想找 | 去这里 |
|---|---|
| 客户需求 / 进度 / 待客户确认 | [`../references/客户需求主清单-进度总表.md`](../references/客户需求主清单-进度总表.md)（唯一权威入口） |
| 文档定位索引 | [`../references/文档地图.md`](../references/文档地图.md) |
| 历史快照 / 兼容短状态 | [`current-baseline.md`](../references/current-baseline.md)、[`live-short-status.md`](../references/live-short-status.md)、[`customer-feature-short-status.md`](../references/customer-feature-short-status.md)；仅供回溯或兼容，不作为当前需求/进度依据 |
| 迁移编号 / 原 ledger 结论 | 主清单「全局真相」小节；已删除的 ledger 不再作为入口 |
| 运行 / API / E2E 证据 | [`../verification/`](../verification/)；只承载验证产物，不是需求/进度权威来源 |
| 验证门槛 / 公开边界 | [`../VALIDATION.md`](../VALIDATION.md) / [`../PUBLICATION.md`](../PUBLICATION.md) |

## 目录定位（保留说明）

- 本目录已退役，仅保留本 README 作为删除说明与旧入口迁移指引；不再承载现行审查矩阵、需求状态或运行证据。
- 已删除的 7 份报告不再提供入口；当前运行证据统一归入 `verification/`。
- 新增客户需求或进度只更新主清单；新增运行证据归入 `verification/`；入口变化同步登记到文档地图。

## 维护规则

- 不要把历史快照或兼容短状态里的结论直接写成「发布就绪」。没有 fresh runtime/API/E2E 证据时，一律按静态证据口径表述，相关运行产物归入 `verification/`。
- 迁移编号连续递增，改动前先更新主清单的迁移小节。
