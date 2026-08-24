# 验证策略

本文档说明公开发布所需的验证分层、命令顺序、证据要求与当前 pending 门禁。历史批次会话记录已归档至 `references/archive/validation-session-log-202608.md`，只作 historical，不构成当前发布证据；发布证据必须基于当前工作树重新生成并归档命令上下文、退出码和报告。

## 分层验证

不同层只能证明不同类型的事实：

- Frontend Vitest：组件状态、API 参数、store、路由辅助逻辑和 UI 边界情况。
- Backend Go tests：服务规则、权限检查、租户边界、DAL 辅助函数和 API/router 合约。
- GMQTT tests：MQTT 认证/ACL、topic 映射、发布订阅行为和 broker plugin hook。
- API automation：HTTP 路由、状态码、响应体、权限、持久化副作用和失败路径。
- Playwright E2E：浏览器路由、用户操作、表单、可见结果和跨页面流程。
- 静态覆盖率/oracle 合约：目录对齐、可追踪性、弱断言门槛和伪覆盖防护。
- Compose stack lane（CI）：全栈 Docker Compose + preflight + API automation + E2E 的自包含运行时信号（nightly + 手动触发，fail-closed）。

## 证据分类硬边界

| 分类 | 规则 |
| --- | --- |
| `real-rdi` | 必须同时有真实 PID、真实激活结果、真实设备凭证、在线/遥测/命令 ACK 证据；没有这些条件不能使用该标签。 |
| `synthetic-rdi` | 只表示隔离 synthetic fixture 的 RDI 软件合同和部分 share/link 流程；不能晋升为 `real-rdi`。 |
| `simulation` | 只覆盖普通非 RDI 设备链路；不能覆盖 RDI 专属行为。 |
| `generic-emulator` | 只覆盖真实 broker/backend 上由 emulator 生成的在线状态和 ACK；不能覆盖真实固件响应。 |
| `historical` | 旧归档只能说明旧批次，不得进入当前 aggregate。 |
| `partial-current` | 当前批次的部分证据；不得写成 release-ready 或 production sign-off。 |
| `pending` / `blocked` | 依赖真实设备、外部服务或目标环境条件，未满足时必须保留阻塞原因。 |

## 覆盖率能说明什么

覆盖率是度量系统，不是单独的正确性保证：

- Source coverage 说明代码被执行过；Endpoint coverage 说明路由被请求过；Page coverage 说明页面被访问过。
- Business coverage 要求对产品行为、状态、权限、错误和可见结果进行精确断言。

六个曾要求单独补齐真实 RDI 页面业务证据的路由（r12 已用真实浏览器操作覆盖一轮，后续批次需保持）：`/device/grouping`、`/device/grouping-details`、`/device/service-access`、`/device/share`、`/device/shared-with-me`、`/device/thingsmodel`。

当前允许的最强表述是：

> 当源码目录、真实映射测试、用例级业务断言、新鲜归档报告、负向控制和代表性 mutation 证据一致时，对当前已盘点的 P0/P1 能力具备高置信覆盖。

不要把这句话简化为"所有可能的业务逻辑都已保证覆盖"。

## 发布验证顺序

在对清理后的工作树作出公开发布结论之前，需要重新运行并归档以下流程。可先在 `automation_tests/` 运行 `npm run preflight:release` 作为离线静态/契约门禁；它不启动服务，也不等于 release readiness。正式 tag release workflow 由仓库内生成器产出 SBOM 并由容器 workflow 生成 image SBOM；发布后仍须下载资产、重算 checksum 并验证 provenance/attestation。

```powershell
cd frontend
pnpm typecheck
pnpm test:coverage
pnpm build

cd ..\backend
go test ./... -coverprofile=coverage/coverage.out -covermode=atomic -timeout 10m
go build ./...

cd ..\mqtt-broker
go test ./...
go build -o build/gmqttd.exe ./cmd/gmqttd

cd ..\automation_tests
$env:FRONTEND_URL='http://127.0.0.1:9725'
$env:PREVIEW_URL='http://127.0.0.1:9725'
$env:PREVIEW_PORT='9725'
$env:API_BASE_URL='http://127.0.0.1:19999/api/v1'
$env:API_TARGET='http://127.0.0.1:19999'
$env:PLAYWRIGHT_USE_PREVIEW_PROXY='1'
$env:PLAYWRIGHT_REUSE_EXISTING_SERVER='0'
npm run preflight:api-e2e
node run_tests.js --include-e2e --workers=1 --archive
```

MQTT automation harness 默认端口已统一：`standard` profile 使用 `127.0.0.1:1883`，`localdev-status` 使用 `127.0.0.1:1885`，显式 `AUTOMATION_MQTT_PORT` 优先。

API、E2E 和 synthetic-rdi 运行必须使用独立的 report/output 目录，并记录 effective backend、broker、database、evidence kind 和 cleanup 结果；不得把 focused、full 和 historical 报告手工拼接。建议拆四条 lane：普通 `simulation` telemetry、`generic-emulator` command、隔离 `synthetic-rdi` contract，以及条件满足后才执行的 `real-rdi`。外部常驻设备模拟器与规格自管模拟器不能同时在线（ACK 抢答冲突），需分 lane。

对于 preview/E2E 证据，preview 端口上的 `/api/v1/*` 必须真实代理到后端 API 并返回 JSON；仅启动前端 preview 不算有效证据。`npm run preflight:api-e2e` 是配置+有限连通性门禁（六账号列表、无 `CHANGE_ME_*` 残留、URL/端口一致性、代理开关），通过它不等于业务正确性证明，更不能标记 `real-rdi`。

本地验证流程：先启动后端 → `npm run prepare:local-accounts` → 加载 `. .\.local\automation-env.ps1` → 再跑 preflight 与 runner。测试账号是本地应用账号，不是 GitHub 凭据；数据库密码不得写入任何文档/日志/报告。

## 外部合约变更

兼容名称与三类外部合约（broker plugin 面 / ThingsVis embed-SSO / telemetry gRPC symbols）的变更规则以 `COMPATIBILITY.md` 为唯一权威；任何一侧变化按 breaking migration 处理并在同一轮重跑聚焦的 broker/frontend/backend/API/E2E 验证。

## 当前 pending 门禁快照（2026-08）

- **compose lane 首轮全绿归档**：`continue-on-error` 已移除、::1 拨号根因已在源码层收敛（统一拨号助手），但尚未产生一轮全绿归档——需 CI dispatch 或有 Docker 的主机执行后归档到 `verification/`。
- 真实环境验收：目标服务器部署、HTTPS/TLS 反向代理、公网 MQTT、backup/restore、ThingsVis 外部集成、real-rdi 全部维持 pending/blocked，不得用 simulation 冒充。
- 性能层：`performance/` 只有脚手架与意图目录，无任何实测 benchmark 数据。
- 前端质量地基：全局 `any` 长尾清理（约 1300 处，集中在 core/data-architecture）、视觉升级 Phase 1–4、移动端关键路径响应式——均为独立车道，未完成。
- 设备凭证哈希存储、脚本沙箱 Worker 化、service_access.voucher 哈希化：设计与计划见 `references/backend-hardening-plan.md`。

## 维护与审查建议

- 每次发布前从当前工作树重新生成验证证据，不要沿用旧归档中的通过结论。
- 某一层无法运行时，在发布说明中明确写出阻塞原因、未覆盖风险和下一步验证入口。
- 更新命令、端口或环境约束时，同步检查 `PUBLICATION.md`、自动化 preflight 脚本和本文件，避免漂移。
- 新的 dated 会话记录写入 `references/archive/` 对应文件，不要叠回本文。
