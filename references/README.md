# references 参考文档目录

本目录保存 AetherLink IoT 当前仍在使用的短参考文档。这里的文件应回答"现在仓库处于什么状态、下一步该做什么、哪些证据不能过度解读"。

2026-07-28 已做归档整理：逐轮进度快照、分支 ledger、历史计划与一次性清理清单已移到
已于 2026-07-28 永久删除，结论已吸收进主清单。

## 当前入口

- [`客户需求主清单-进度总表.md`](./客户需求主清单-进度总表.md)：**唯一权威入口**，REQ-01 ~ REQ-58 全部状态、待客户确认清单、本轮可做/需环境边界。
- [`文档地图.md`](./文档地图.md)：全仓文档定位索引，说明每份留下的文档扮演什么角色、哪些已归档。

## 仍然有效的专项参考

这些文件描述工程规则、目录约定或专项分类，不承担"整体进度"叙述，主清单不覆盖它们。

- `dependency-capability-matrix.md`：运行、测试、生成和外部可选能力的依赖矩阵；记录本地替代、阻断状态和删包规则。
- `developer-guide.md`：开发入口、轻量检查、首台设备主线和兼容字段。
- `api-guide.md`：API 改动入口、首台设备 API 关注点和覆盖口径。
- `plugin-guide.md`：前端插件管理与 Broker 插件边界。
- `plugin-runtime-surface.md`：Broker 插件运行时表面、外部合同和证据边界。
- `mcp-integration-guide.md`：MCP 集成建议和当前未实现边界（design-only）。
- `mcp-tool-contract.md`：未来 MCP 工具合同草案（design-only，不计为交付或 runtime coverage）。
- `coverage-criteria.md`：覆盖分层与弱断言判定标准。
- `repository-file-inventory-summary.md` / `repository-file-inventory.csv`：由 `automation_tests/scripts/generate_repository_inventory.js` 生成的本地全仓文件级台账；默认被 `.gitignore` 排除并保存在仓库外，发布前不纳入 source package。需要审计时可在本地重新生成；它不替代业务运行验证。
- `source-file-inventory.md`：内部历史源码盘点快照，因包含过期审阅范围而不随公开 source package 发布；它不属于当前运行或测试输入。
- `source-quality-review.md`：公开发布的源码质量审阅清单，由覆盖合同读取并保留当前静态证据边界。

## 历史短状态（保留但不再维护）

以下三份仍留在本目录，因为主清单的证据边界小节引用它们的时间线；结论一律以主清单为准。

- `current-baseline.md`、`live-short-status.md`、`coverage-short-status.md`、`customer-feature-short-status.md`

## 维护规则

- 当前参考文档必须使用中文说明，命令和路径可保留英文。
- 状态变化时优先更新主清单；短状态文档只补时间线，不单独宣布"完成"。
- 没有 fresh runtime/API/E2E 证据时，不得把静态清理结论写成"已发布就绪"。
- 新增文档同步登记到 `文档地图.md`。
