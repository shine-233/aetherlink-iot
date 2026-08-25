# references 参考文档目录

本目录保存 AetherLink IoT 当前仍在使用的短参考文档。这里的文件应回答"现在仓库处于什么状态、下一步该做什么、哪些证据不能过度解读"。

2026-07-28 已做归档整理：逐轮进度快照、分支 ledger、历史计划与一次性清理清单已移出本目录；结论已吸收进主清单。2026-08-25 起新增 `archive/` 子目录，集中存放一次性调查笔记（见下文）。

## 当前入口

- [../ROADMAP.md](../ROADMAP.md)：公开能力规划入口（原内部需求台账已移出公开仓库，仅保存在维护者私有环境）。
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
- `source-quality-review.md`：公开发布的源码质量审阅清单，由覆盖合同读取并保留当前静态证据边界。
- `backend-hardening-plan.md` / `frontend-visual-upgrade-plan.md` / `gen-inheritance-audit.md` / `coverage-artifact-index.md`：现役专项计划与证据索引。

## 本地生成、不入库的台账

`repository-file-inventory-summary.md` 与 `repository-file-inventory.csv` 由
`automation_tests/scripts/generate_repository_inventory.js` 生成本地全仓文件级台账，默认被
`.gitignore` 排除并保存在仓库外；需要审计时在本地重新生成。它们不替代业务运行验证。

历史内部源码盘点快照（原 `source-file-inventory.md`）因包含过期审阅范围已于 2026-07 移出公开仓库。

## archive/（一次性调查笔记）

`archive/` 子目录保存已完成使命的一次性调查记录，仅作历史证据保留，不再维护：

- `archive/dependabot-pr-disposition-20260817.md`：Dependabot PR 逐项处置台账（PUBLICATION.md 引用）。
- `archive/github-feature-audit-20260817.md`：GitHub 仓库配置审计矩阵（PUBLICATION.md 引用）。
- `archive/ghcr-visibility-readback-20260817.md`：GHCR 包可见性回读记录。
- `archive/simulated-integration-boundary-20260817.md`：模拟集成边界调查。

## 维护规则

- 当前参考文档必须使用中文说明，命令和路径可保留英文。
- 状态变化时优先更新主清单；短状态文档只补时间线，不单独宣布"完成"。
- 没有 fresh runtime/API/E2E 证据时，不得把静态清理结论写成"已发布就绪"。
- 新增文档同步登记到 `文档地图.md`；一次性调查笔记一律放 `archive/`，不得散落在本目录顶层。
