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
- **compose lane 首轮全绿归档已存在**：`verification/automation-run-20260824-compose-first-green/archive-manifest.json`（PR #123 提交集，run 32692782323）；`continue-on-error` 已移除。后续合并需维持全绿或注明降级原因。
- 真实环境门禁（目标服务器部署、HTTPS/TLS 终止、公网 MQTT、backup/restore、ThingsVis 外部集成、real-rdi）全部维持 pending/blocked，不允许 simulation 冒充。
- 性能层：`performance/` 仅有结构化框架与目录占位，尚无实测 benchmark 数据。
- 前端类型收敛：全局 `any` 长尾约 **1183 处**（排除测试；集中在 core/data-architecture 约 304 处）。视觉升级 Phase1 路由转场、断点令牌、.stagger 工具类、prefers-reduced-motion 守卫已落地（#125）；骨架屏替换 / ECharts 品牌 theme / motion-v 微交互尚未开始。
- 设备凭证哈希存储：voucher hash phase 1 已落地（commit 6a97089 + vouchercache.go + 双端契约测试）；脚本沙箱 Worker 化仍在计划中（`references/backend-hardening-plan.md`）。
- 性能层：`performance/` 只有脚手架与意图目录，无任何实测 benchmark 数据。
- 前端质量地基：全局 `any` 长尾清理（约 1300 处，集中在 core/data-architecture）、视觉升级 Phase 1–4、移动端关键路径响应式——均为独立车道，未完成。
- 设备凭证哈希存储、脚本沙箱 Worker 化、service_access.voucher 哈希化：设计与计划见 `references/backend-hardening-plan.md`。

## P2/P3 收敛批次（2026-08-24，fix/remaining-p2p3）

本节记录 2026-08-24 P2/P3 批次的完成边界与后续批次目标，均基于当前工作树重新验证。

**后端**
- 运维暴露面门禁（P3）：`backend/router/router_init.go` 的 `/swagger/*any`、`/metrics`、`/metrics-viewer(/echarts.min.js)` 在 `GOTP_ENV=production` 时跳过注册；非生产环境保持原样。router contract 测试仍锁定字面量存在性。
- JWT Redis 键哈希（P3）：新增 `pkg/utils.TokenDigest`（HMAC-SHA256hex，域分离密钥独立于 voucher），`middleware/jwt_auth.go`、`api/telemetry_ws_auth.go`、`service/sys_user_auth.go`（login/logout/refresh/transform）全部改为摘要作键。**部署注意：上线后旧明文 token 键立即失效，全体用户需重新登录（一次性会话失效）；如需平滑迁移需另行设计双读。** `loginEmailTokenKey(<email>)` 的 value 已在 P1 加固批次（2026-08-26）改为存 TokenDigest 摘要且 TTL 与会话超时对齐（此前存明文 token 且无 TTL）。
- DAL 测试盲区收敛（P2）：79 个 DAL 源文件中本轮补齐 3 个核心文件——`device_auth_test.go`（摘要/明文双读与惰性升级）、`alarm_test.go`（告警配置/信息列表租户 scope SQL + JOIN 投影 + trigger_duration 零值显式写）、`users_test.go`（GetUsersByEmail raw 链 + GetUsersByPhoneNumber 双模式）。剩余 DAL 文件的测试补齐为后续批次目标。

**前端**
- 空态覆盖率（P2）：device 主干列表已有 `DeviceManageEmptyState`（device/manage/index.vue #empty 插槽）；automation 已有 `n-empty`（scene-manage/index.vue 与 scene-linkage dataList.vue）；home 无表格视图，本轮将 `HomeFirstDeviceDeploymentHealthSection.vue` 空结果文本升级为 `n-empty` 作示范。其余约 231 个视图文件的空态巡检为后续批次目标。
- i18n 绕过精确计数（P2）：按"非测试 views+components 源码中含 CJK 字符的字符串字面量行"口径统计为 **734 行 / 51 个文件**（排除注释行、`__tests__`、`*.test.ts`；统计脚本口径见批次记录）。最大热点：`rdi/constants/rdi-labels.ts`（150 行，双语常量表，需专项车道）、`home/homeFirstDeviceWorkbench.ts`（157 行，注意该 vue 视图内另有 406 个历史 U+FFFD 乱码字符属数据损坏问题）。示范修复：`device/template/components/step/add-edit-commands.vue`（2 处枚举提示 → `device_template.table_header.pleaseAddEnumItem` 新键，四语言已补）与 `add-edit-attributes.vue`（1 处 `'新增成功'` → `common.addSuccess`）。其余 ~731 行为后续批次目标。
- 无障碍 aria-\*（P3）：已在 3 个高频对话框加 `:aria-label="title"`（management/role、alarm/notification-group、apply/service 的 table-action-modal）作示范；全面 aria 巡检为后续批次目标。

## P1 安全加固批次（2026-08-26，improve/p1p2-hardening）

本节记录 2026-08-26 P1 加固批次的完成边界，均基于当前工作树静态与单测验证。

**后端（已验证：`go build ./...` + `go test ./...` 全绿）**
- LIKE 通配转义（P2）：新增 `dal.EscapeLikePattern/ContainsLikePattern`，接入 users/alarm/device_config/device_selector/device_template/device_query_reads/devices_list_read_model*/board/device_groups 全部用户输入拼接点；`like_escape_test.go` 钉死转义顺序契约。
- 刷新令牌吊销（F1/LOW）：`RefreshToken` 在新会话写入成功后吊销旧 token 摘要；api 层经 `middleware.SelectJWTAuthToken` 同源提取。
- `<email>_token` 键改摘要存储 + 会话对齐 TTL（F2/LOW）：不再落地明文 JWT。
- 登录防爆破 IP 维度（F3/LOW）：账号+IP 双维度；配置 `classified-protect.ip-login-max-fail-times/ip-login-fail-window-seconds`（默认 20 次/600 秒启用）。
- Casbin 路由覆盖审计（P1）：启动期 fail-fast 校验"CasbinRBAC 之后注册的路由必须登记进资源表"；`casbin.route-audit-mode: fail-fast|warn|off`。**存量库升级注意见 COMPATIBILITY.md。**
- 裸查 DAL 收敛（F4）：`GetDeviceByID→GetDeviceByIDUnscoped`、`GetDevicesByIDs→GetDevicesByIDsUnscoped`（编译器级警示 + 调用面收敛）；`DeleteDeviceConfig/DeleteDeviceGroup` 改为 `*ForTenant(id, tenantID)` 双条件删除，含异租户拒绝回归测试。
- 既有失败修复：`deployment_health_migrations_test.go` 由 CGO sqlite 切换 glebarez 纯 Go 驱动（Windows/CGO_ENABLED=0 可跑）。

**前端（已验证：prettier/eslint 通过；全量 typecheck/test/build 见批次提交 CI）**
- 依赖分类修正（B1）：`motion-v` devDependencies→dependencies（运行时 import）；`@types/three` 归位 devDependencies。vendor-three/vendor-motion 手动分包此前已在分支内完成（B2 无需改动）。
- 插件表单空态（B5）：`apply/plugin/components/form.vue` 在插件未返回 schema 时渲染 `NEmpty(common.noData)`。其余两个审计命中项复核为误报（table-action-modal 已有 submitLoading；change-information 已有状态处理），未做无意义 diff。

**部署（C1）**
- doctor 新增 `server-mqtt-plaintext-tls` 检查（sh+ps1 对齐）：server 模式下公网 MQTT 默认明文 → 默认强警告，`AETHERLINK_STRICT_TLS=1` 升级为阻断错误；`.env.example` 已补变量说明。

**明确 pending / 后续批次**
- IP 锁定、Casbin 审计、刷新吊销均为静态+单测证据；真实 Redis/浏览器 E2E 运行验证 pending。
- A5 http_client 死代码 `Post/Delete` 已删除（零调用方核实）；`PostJson` 返回 *http.Response 的契约保持，由现有调用方正确 Close。
<<<<<<< HEAD
- **JWT HttpOnly cookie 经复核（2026-08-26 第二车道）实际已达成**：`middleware/auth_cookie.go` 的 `ensureAuthCookieDefaults` 将 `auth.cookie.enabled` 默认置 true，登录/刷新响应均追加 HttpOnly SameSite=Lax cookie；剩余议题仅为生产 HTTPS 下开启 `auth.cookie.secure`（部署侧动作）。
- **零测试目录收敛进展**：`internal/middleware/response` 已补 8 用例契约测试（panic 恢复/已写响应跳过/错误包/data 包裹/变量替换/Accept-Language）。其余目录中 `internal/logic`、`api/sseapi` 强依赖 gorm query 单例与 Redis hub，需先抽存储接口再可测；`internal/query` 为 gorm-gen 生成物、`cmd/gen`/`cmd/virtual_sensor` 为工具入口——按现状维持 pending。
- 未纳入本批：巨型文件同包拆分（867 行 attribute_event_ingress.go 等 top10，纯代码移动也会污染 blame 与在途分支合并，需独立 lane 排期）、1014 处硬编码 hex 的 token 化（需先定设计 token 权威值与视觉回归基线）、Device3DPanel 硬编码材质色集中化（随面板接线已隔离在单文件，后续随主题系统统一处理）。

### P2 补充车道（2026-08-26 第二批，improve/p2-lane-tests-visual）

- Device3DPanel 正式接线（此前为无引用死代码）：新增设备详情「3D 预览」tab（registry 懒加载 + vendor-three 分包按需下载），薄包装复用 useTelemetryRealtimeState，温度启发式（精确 temperature → temp 模糊）驱动材质颜色，WebGL 缺失自动降级；i18n 四语言补齐；6 用例单测钉住接线契约。
=======
- 未纳入本批：13 个零测试业务目录补齐、巨型文件拆分、1014 处硬编码 hex 的 token 化、JWT 主通道切换 HttpOnly cookie、Device3DPanel 接线——需独立车道或产品决策。
>>>>>>> origin/main

## 维护与审查建议

- 每次发布前从当前工作树重新生成验证证据，不要沿用旧归档中的通过结论。
- 某一层无法运行时，在发布说明中明确写出阻塞原因、未覆盖风险和下一步验证入口。
- 更新命令、端口或环境约束时，同步检查 `PUBLICATION.md`、自动化 preflight 脚本和本文件，避免漂移。
- 新的 dated 会话记录写入 `references/archive/` 对应文件，不要叠回本文。
