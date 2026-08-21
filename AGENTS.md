# AetherLink IoT 项目规则

## 定位

AetherLink IoT 是面向物联网设备接入、监控和私有部署的平台源码仓库。

## 启动与验证

- 单机部署：先运行根目录 `start-aetherlink.*` 的 doctor，再启动 Docker Compose；详见 `START-HERE.md` 与 `deploy/README.md`。
- 前端：`cd frontend && pnpm typecheck && pnpm test:run && pnpm build`。
- 后端：`cd backend && go test ./... && go build ./...`。
- Broker：`cd mqtt-broker && go test ./... && go build ./...`。
- 自动化：`cd automation_tests && npm run test:list`；真实 API/E2E 前先执行对应 preflight。

## 技术栈与目录

- `frontend/`：Vue 3、TypeScript、Vite、Vitest。
- `backend/`：Go、Gin、GORM、PostgreSQL。
- `mqtt-broker/`：Go、GMQTT。
- `automation_tests/`：Mocha、Playwright 和部署/覆盖契约。
- `references/` 保存现役基线与文档地图；`verification/` 仅保存验证证据；能力规划入口是根目录 `ROADMAP.md`。

## 现役边界

- 默认核心栈仅依赖 PostgreSQL、Redis、仓库内 MQTT Broker、Backend 和 Frontend。
- 本地 native board 是默认可视化 provider；ThingsVis 与 HTTP adapter 仅通过显式 optional profile/配置启用。
- Market、SMTP、地图 provider 属于外部可选能力；未配置时不得阻断核心启动，也不得泄露配置值。
- 外接模块必须保留稳定接口契约；能本地化的核心能力优先使用本地实现，不能本地化的能力返回明确的 optional/external-blocked 状态。
- 数据库迁移当前最高为 `49.sql` / `VERSION_NUMBER=49`；修改迁移前先核对 `backend/sql/` 与目标数据库的 `sys_version`。

## 修改约定

- 当前工作树有大量既有未提交改动；禁止回退、覆盖、批量格式化或顺手重构无关文件。
- 不直接删除生成文件、第三方契约、审计底表或运行残留；先核对 `GENERATED_FILES.md`，删除候选在最终汇报后另行确认。
- 接口、配置或行为变化必须同步注释、目标测试和权威文档。
- 静态契约或局部测试通过不等于发布就绪；未运行的 Docker、API、E2E 或真实运行验证必须标为 pending。
