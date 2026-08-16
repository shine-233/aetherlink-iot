# 验证策略

本文档说明当前工作树用于公开发布时的验证策略，包括推荐的命令顺序、证据要求，以及“历史归档”和“当前发布证据”的边界。

AetherLink IoT 采用多层验证，因为不同层只能证明不同类型的事实：

- Frontend Vitest：组件状态、API 参数、store、路由辅助逻辑和 UI 边界情况。
- Backend Go tests：服务规则、权限检查、租户边界、DAL 辅助函数和 API/router 合约。
- GMQTT tests：MQTT 认证/ACL、topic 映射、发布订阅行为和 broker plugin hook。
- API automation：HTTP 路由、状态码、响应体、权限、持久化副作用和失败路径。
- Playwright E2E：浏览器路由、用户操作、表单、可见结果和跨页面流程。
- 静态覆盖率/oracle 合约：目录对齐、可追踪性、弱断言门槛和伪覆盖防护。

## 当前证据边界

仓库中可能保留清理前的归档证据。这些材料可以帮助理解测试组合和历史验证范围，但不能直接作为后续清理、重命名或发布整理后的当前 release 结论。

发布证据必须基于当前工作树重新生成，并同时归档命令上下文、退出码和报告。

本项目当前的证据分类是硬边界：

| 分类 | 规则 |
| --- | --- |
| `real-rdi` | 必须同时有真实 PID、真实激活结果、真实设备 voucher、在线/遥测/命令 ACK 证据；没有这些条件不能使用该标签。 |
| `synthetic-rdi` | 只表示隔离 synthetic fixture 的 RDI 软件合同和部分 share/link 流程；不能晋升为 `real-rdi`。 |
| `simulation` | 只覆盖普通非 RDI 设备链路；不能覆盖 RDI 专属行为。 |
| `generic-emulator` | 只覆盖真实 broker/backend 上由 emulator 生成的在线状态和 ACK；不能覆盖真实固件响应。 |
| `historical` | 旧归档只能说明旧批次，不得进入当前 aggregate。 |
| `partial-current` | 当前批次的部分证据；不得写成 release-ready 或 production sign-off。 |
| `pending` / `blocked` | 依赖真实设备、外部服务或目标环境条件，未满足时必须保留阻塞原因。 |

## 2026-08-15 上传前清理与证据边界

本轮按用户指示停止重复完整回归，转为部署前和 GitHub 上传前清理。根 `_localrun`、历史 verification 归档、构建/coverage、运行态配置、认证状态、日志、截图、二进制和本地审计台账已移到仓库外可恢复 quarantine；本机清单位于父目录的 `_aetherlink-github-cleanup-quarantine-20260815/github-cleanup-manifest-20260815.json`，不是公开源码的一部分。r14-pre 旧运行物另有 `_aetherlink-cleanup-quarantine-20260815-r14-pre/quarantine-manifest.json`。

清理后生成物扫描为 `0` 个候选；tracked 源码扫描未发现明文数据库密码、私钥标记或带凭据的数据库 URI。`node_modules` 仅保留在本机供依赖重建，依靠 lockfile 安装，不属于 source package。quarantine 均为 `permanentDelete=false`，不能把“已移出仓库”写成“已永久删除”。

本轮没有重新执行完整 r14：后端 Go 测试在依赖下载超时处停止，broker 测试按用户要求停止；这两项均不是通过。此前 r13 的 fresh 本地证据仍可用于方法和历史摘要，但不能替代真实 RDI、目标服务器、Docker/Compose、HTTPS/TLS、公网 MQTT、目标环境 backup/restore 或外部 ThingsVis 验收。当前状态为 `real_rdi_status=not-tested`、`target_deployment_status=pending`、`production_signoff=not-ready`、`github_upload=executed`。

公开源码已推送到 `https://github.com/shine-233/aetherlink-iot`。当前公开基线已由 GitHub Actions 的 Source CI、Minimum quality gate 和 CodeQL（Go、JavaScript/TypeScript）成功检查；这些是源码与离线门禁证据，不等价于完整 r14、本地目标环境、真实 API/E2E 或真实设备验收。
- 上传前补充清理：7 个 untracked 的一次性历史/生成文件已可恢复移动到 `../_aetherlink-github-cleanup-quarantine-20260815-r2/`，清单回读为 7/7 源路径不存在、SHA-256 一致；首次清单的字节数捕获错误已在同一清单中注明并校正。此次没有重新执行测试、编译或服务启动。
- 未完成的 `_aetherlink-validation-20260815-r16` 也已在不重跑测试的前提下整体移到 `../_aetherlink-github-cleanup-quarantine-20260815-r3/`；2,074 个文件、78,505,657 bytes 的逐文件 SHA-256 校验通过，清单记录 `allMovedVerified=true`、`permanentDelete=false`。该批次不进入当前验证结论。

## 覆盖率能说明什么

覆盖率是度量系统，不是单独的正确性保证。

- Source coverage：说明代码被执行过。
- Endpoint coverage：说明 HTTP 路由被请求过。
- Page coverage：说明浏览器路由或流程被访问过。
- Business coverage：要求对产品行为、状态、权限、错误和可见结果进行精确断言。

当前 r11c 隔离 local-core aggregate 中，API 为 `64/64` modules、`372/372` endpoints；浏览器为 `20/20` modules、`0 failed`；页面/route 为 `56/56`。visualization 模块仍有 `3` 个显式 `runtime-external` partial-skip，分别是 ThingsVis project/dashboard 服务不可达、negative-menu 同一外部服务不可达和 mirror ID 未配置。`business evidence kind 28/28` 只表示 runner 的 evidence bucket 分类通过，严格 `businessClosureEvidence.business` 为 `27/27`；不表示全部产品业务或外部集成已经闭环。Frontend 本轮 typecheck、data-architecture typecheck、packages typecheck 和 coverage 通过；完整 production build 发生 Windows `0xC0000005` 访问冲突，低内存诊断 build 单独通过，详见本文件末尾的 r11c 记录。上述数字仍要按证据分类阅读：六个 device route 的命中不自动产生真实 RDI 页面业务证据，synthetic share/link 不自动产生真实设备验收，endpoint/page coverage 也不等于业务状态闭环。

本轮权威 API/E2E aggregate 使用串行 runner（报告中的 `parallel: false`）。并行执行 data-history 时出现的记录竞争属于 runner/fixture 隔离问题：并行 worker 必须使用独立的 fixture 命名空间、清理边界和 report/output 目录，不能把竞争结果解释成产品业务失败，也不能手工拼接不同 runner 的报告。

2026-08-11 的定向补证据属于 historical：`e2e/02_device.spec.js` 在显式 synthetic PID 下 `8/8` passed；当时曾有 generic emulator 的 `6/6` command-jobs 证据，但那一轮未能可信重跑 `e2e/20_command_jobs.spec.js`。r8 已重新完成 generic command-jobs `6/6`，但仍只是 generic/non-RDI 软件证据；synthetic 与 emulator 都不改变 `real-rdi=pending`。

六个必须单独补齐真实 RDI 页面业务证据的路由是：

- `/device/grouping`
- `/device/grouping-details`
- `/device/service-access`
- `/device/share`
- `/device/shared-with-me`
- `/device/thingsmodel`

当前允许的最强表述是：

> 当源码目录、真实映射测试、用例级业务断言、新鲜归档报告、负向控制和代表性 mutation 证据一致时，对当前已盘点的 P0/P1 能力具备高置信覆盖。

不要把这句话简化为“所有可能的业务逻辑都已保证覆盖”。

## 发布验证顺序

在对清理后的工作树作出公开发布结论之前，需要重新运行并归档以下流程。可先在 `automation_tests/` 运行 `npm run preflight:release` 作为离线静态/契约门禁；它不启动服务，也不等于 release readiness。漏洞数据库、SBOM 生成和托管依赖审查仍为 `not-run`，真实 frontend/backend/broker build/tests、`preflight:api-e2e` 与 E2E 仍不可省略。

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

MQTT automation harness 的默认端口已修复并统一：`standard` profile 使用 `127.0.0.1:1883`，`localdev-status` profile 使用 `127.0.0.1:1885`，显式 `AUTOMATION_MQTT_PORT` 优先；旧的 `1884` fallback 已移除。该修复只消除本地 harness 的默认端口错配，不构成公网 MQTT 或真实设备链路证据。

API、E2E 和 synthetic-rdi 运行必须使用独立的 report/output 目录，并记录 effective backend、broker、database、evidence kind 和 cleanup 结果；不得把 focused、full 和 historical 报告手工拼接。r11c、r8 及更早批次的运行目录已移到仓库外 quarantine，仅保留数字摘要和方法说明；它们只作 historical，不能作为当前清理后的 fresh evidence。

建议把运行拆成四条 lane：普通 `simulation` telemetry、`generic-emulator` command、隔离 `synthetic-rdi` contract，以及条件满足后才执行的 `real-rdi`。synthetic fixture 运行结束必须再次执行 `--status` 并得到 `absent`；不能用普通 simulation 或 generic emulator 填充 real-rdi lane。

对于 preview/E2E 证据，preview 端口上的 `/api/v1/*` 必须真实代理到后端 API 并返回 JSON。仅启动前端 preview、让 API 路径返回 HTML，不算有效的 API/E2E 发布证据。

`npm run preflight:api-e2e` 必须先通过，上述 API/E2E 命令才能计入发布证据。preflight 是配置门禁加有限连通性门禁：它会请求 preview 首页、preview 代理下的 deployment health JSON 和 backend health JSON，但不会启动服务、登录、执行浏览器流程或证明业务正确。运行前仍应确认 `frontend/dist` 已生成，并继续执行登录页渲染及完整 API/E2E。preflight 会检查：

- 集中维护的六账号发布列表。
- 是否残留公开的 `CHANGE_ME_*` 占位符。
- `FRONTEND_URL` 或 `frontendURL` 与 `PREVIEW_URL` 是否都指向 `PREVIEW_PORT`（默认 `9725`；2026-08-14 r8 实际使用 `9725`；`19725` 仅可作为未来并行隔离运行的可选端口示例）。
- `API_TARGET` 是否已设置。
- `API_BASE_URL` 与 `API_TARGET` 是否使用同一个后端 origin。
- `PLAYWRIGHT_USE_PREVIEW_PROXY` 是否为 `1`。
- `PLAYWRIGHT_REUSE_EXISTING_SERVER` 是否为 `0`。

preflight 成功不能标记 `real-rdi`。real-rdi gate 还必须有真实 PID、真实激活、真实设备上线、真实遥测回读、真实命令发送和设备响应；当前缺失时状态必须是 `pending`，而不是“测试稍后补一条就等价通过”。

发布账号是本地或测试后端中的 AetherLink IoT 应用账号，不是 GitHub 凭据。真实密码应保存在环境变量、CI secrets 或被忽略的本地文件中。

数据库凭据在当前证据中只记录“用户提供的凭据已验证成功”；不得在验证文档、日志、报告、备份清单或部署包中记录数据库密码明文。

进行本地验证时，先启动后端，在 `automation_tests/` 下运行 `npm run prepare:local-accounts`，然后执行 `. .\.local\automation-env.ps1`，再运行 preflight。这样做的目的，是预先创建或隔离测试账号，让认证状态和密钥留在源码控制之外，并在占位符残留时快速失败。

对于本轮已经落地但尚未做统一运行期验证的热点，后续验证记录里应至少显式标记：

- ThingsVis：`ThingsVisAppFrame.vue` 已拆出 `thingsvisAppFrameLifecycle.ts` 与 `thingsvisFrameTransportBridge.ts`，后续一旦进入验证轮，应重点覆盖 `tv:ready` 初始化、可信消息来源校验、viewer/editor 分流和 `tv:platform-data` 回推。
- broker：`client_options.go` 已独立承载协商后的 `ClientOptions`；后续验证轮应重点覆盖连接协商、插件可见 `Client` 契约读取和协议选项透传。
- 自动化遥测：`automate_telemetry.go` 已把设备条件求值与动作分发拆到旁路文件；后续验证轮应重点覆盖条件命中、动作分发表现和错误汇总语义没有回归。
- `edit-premise`：当前属于文档化/收尾状态；后续验证轮应重点覆盖前提回显、触发参数选择、事件参数条件与时间条件编辑路径，确认 helper 拆分没有破坏页面行为。

## 兼容名称

当前仓库的公开文档、默认 broker 入口、现行 ThingsVis 运行时常量和生成的 telemetry client wrapper，均已使用 AetherLink IoT 命名。

`COMPATIBILITY.md` 不再用于保留旧名称清单，而是用于说明未来如何处理以下三类外部合约变更：

- broker plugin loading/config surfaces。
- ThingsVis embed/SSO identifiers and host keys。
- generated gRPC symbols under the telemetry contract。

验证规则是：如果这些外部合约类别再次变化，应将其视为 breaking migration，并在同一轮工作中重新运行聚焦的 broker/frontend/backend/API/E2E 验证；只有在归档证据之后，才能声明发布就绪。

ThingsVis 仍是可选的可视化/看板集成。只要它仍保留在仓库中，其 embed/SSO 路径就仍需要真实覆盖和新鲜的 preview-proxy 证据。

当前 full E2E 的 ThingsVis 3 个用例因 `127.0.0.1:8000` 不可达和 `THINGSVIS_MIRRORED_DASHBOARD_ID` 未配置而显式 skip；command-jobs 的 6 个真实下发/ACK 用例因 seeded device offline 且未启动 generic emulator 而显式 skip。它们是 `blocked`/`pending` 的外部运行条件，不得被 `20/20 modules passed` 掩盖。

当前仍为 `pending` 的目标环境门：Docker/Compose target、HTTPS/TLS 与反向代理、公网 MQTT、真实 RDI、ThingsVis 外部集成，以及目标环境 backup/restore。隔离本地 PostgreSQL 的备份恢复演练只能证明该隔离库的可读/可恢复，不能替代目标环境的跨组件 backup/restore 演练。

## 2026-08-12 synthetic-rdi / protocol-emulator fresh 验证

本节记录本轮新生成的隔离软件证据，不改变 `real-rdi`、ThingsVis 或生产部署门禁。

- 当前 fresh 证据目录原始文件已移到仓库外 quarantine。本轮使用隔离数据库 `aetherlink_iot_isolated_20260812_01`、Redis logical DB `11`、GMQTT `127.0.0.1:11086` 和 backend `127.0.0.1:19999`；旧的 `19998/11086` 运行态在 lane 开始前由本轮明确 PID 回收，fresh lane 结束后 `19999/11086` 已回收。
- fresh fixture 的起始状态为 `inactive/disabled`，由公开 `POST /api/v1/rdi/devices/activate` 返回 `200`，并记录 `activated-this-run`；本轮 PID 为 `SYN260812229`，API 回读和 SQL 回读均为 `active/enabled`。激活和数据库回读原始 JSON 已移到仓库外 quarantine。数据库密码只作为运行时参数提供，未写入文档、日志、manifest 或证据包。
- share/link 软件合同 `34 total / 34 pass / 0 fail / 0 pending`，原始报告已移到仓库外 quarantine。报告包含 owner/recipient tenant 不同、首次/重复接受、`shared-with-me` 回读、共享用户只读限制、写入/再次 share 拒绝和无效 token 等断言。
- 协议 emulator 的 success/failure 两条路径均观察到 `offline -> online -> offline`，均回读 fresh `temperature_1=25.5`，success ACK 为 `status=3/result=0/message=success`，failure ACK 为 `status=4/result=1/message=failed`，最终 SQL `is_online=0`。离线 manifest/session/replay 也通过；Node synthetic contract 为 `8 passed`，Go emulator `go test -p 1 ./cmd/synthetic-rdi-protocol-emulator -count=1` 通过，证据包 `55/55` 文件哈希一致、数据库/MQTT/JWT secret 与 JWT/Bearer 模式命中均为 `0`。
- 本轮结论是 `synthetic-rdi / protocol-emulator = partial-current software-path-passed`，不是 `real-rdi`，也不是 production sign-off。模拟 PID、voucher、硬件身份、固件 MQTT session、遥测、在线状态和 ACK 只证明 `protocol-emulator -> isolated GMQTT -> backend -> API/SQL` 软件路径。
- 仍然 pending 的硬门禁：真实 RDI PID/activation、真实 voucher/硬件身份、真实固件 MQTT session、真实物理遥测/在线状态/ACK、真实设备 RDI share/link 与生产跨租户权限链、ThingsVis/negative-menu、HTTPS/TLS、反向代理、公网 MQTT、目标环境 backup/restore 和 Docker/Compose target runtime。不得用 simulation device、generic emulator 或 synthetic fixture 关闭这些门禁。

## 维护与审查建议

- 每次发布前，都应从当前工作树重新生成验证证据，不要沿用旧归档中的通过结论。
- 如果某一层验证无法运行，应在发布说明或评审记录中明确写出阻塞原因、未覆盖风险和下一步验证入口。
- 更新命令、端口或环境约束时，应同步检查 `PUBLICATION.md`、自动化 preflight 脚本和本地状态文档，避免文档与实际门槛漂移。

## 2026-08-14 r8 当前 fresh override

本节覆盖本文更早的历史数字。r8 fresh API/E2E 报告为 API `64/64`、E2E `20/20`、`0` failed，business evidence `30/30`，endpoint coverage `372/372`，page/route coverage `56/56`。Visualization 仍有 3 个结构化 `runtime-external` partial-skip：ThingsVis `127.0.0.1:8000` 不可达、negative-menu 同一外部服务不可达、`THINGSVIS_MIRRORED_DASHBOARD_ID` 未配置。

本轮 synthetic software lane 的历史证据已移到仓库外 quarantine；它通过了 synthetic activation、share/link focused API、协议 emulator 的 success/failure ACK、遥测、在线/离线回读、SQL readback、offline manifest/session/replay 和脱敏扫描。manifest 仍明确 `synthetic-rdi / protocol-emulator / real_rdi_status=not-tested / production_signoff=not-ready`。这不能替代真实 RDI PID、voucher、硬件身份、真实固件 MQTT session、真实遥测、真实在线状态、真实 ACK 或生产跨租户 share/link。

当前默认数据库 `aetherlink_iot` 只读版本仍为 `45`（`version=0.0.23`、`devices=11`），不是源码 `VERSION_NUMBER=48`；隔离 r8 数据库为 48 不能替代默认目标库。Go 迁移单测本轮因 `proxy.golang.org` 依赖下载失败而未进入测试函数。Docker/Compose、HTTPS/TLS、公网 MQTT、目标环境 backup/restore、真实 RDI 和 ThingsVis 外部集成仍是部署硬门禁。

ThingsVis 不因 Native provider 可运行而清理：Native 是默认核心 provider，ThingsVis 是可选 legacy compatibility provider；`negative-menu` 是 ownership rejection 的测试标签，不是服务。只清理已确认可再生且无引用的运行产物，并使用可恢复 quarantine；保留 ThingsVis 源码、optional compose/nginx、测试合同、当前 fresh 证据和数据库备份/恢复证据。

## 2026-08-14 r11c 当前复测与清理记录

本节覆盖本文早于 r11c 的当前数字；旧 r8/r9c/r10 等章节只作历史复盘，不能覆盖本节。

- API/E2E fresh 证据：`84/84` aggregate、API `64/64`、E2E `20/20`、`0 failed`；endpoint `372/372`，page/route `56/56`。evidence kind 为 `unknown 24/24`、`contract 3/3`、`catalog 1/1`、`preflight 1/1`、`config 1/1`、`boundary 26/26`、`business 28/28`；严格 `businessClosureEvidence.business` 为 `27/27`，不能把 `84/84` 或 `56/56` 写成所有业务闭环。
- Visualization 仍有且只有 3 个 `runtime-external / seedable=false` partial-skip：ThingsVis `127.0.0.1:8000` connection refused、negative-menu 使用同一外部服务不可达、`THINGSVIS_MIRRORED_DASHBOARD_ID` 未配置。Native 通过不改变这三个 ThingsVis 外部门禁。
- synthetic 证据目录已移到仓库外 quarantine；manifest 固定 `fixture_provenance=synthetic-rdi`、`evidence_class=protocol-emulator`、`real_rdi_status=not-tested`、`production_signoff=not-ready`。本轮通过 synthetic activation、share/link focused API、success/failure ACK、`offline -> online -> offline`、`temperature_1=25.5`、API/SQL readback、offline manifest/session/replay 和 secret scan；它只证明 isolated software path，不是 real-RDI。
- 前端 typecheck、data-architecture typecheck、packages typecheck 和 coverage 通过；原始完整 production build 退出 `-1073741819`（Windows `0xC0000005`），低内存诊断 build 退出 `0`，产物和摘要已移到仓库外 quarantine。这仍不等于目标机部署、HTTPS/TLS、反向代理或公网 MQTT 验收。
- 当前默认数据库仍为 `aetherlink_iot: sys_version=45, version=0.0.23, devices=11`；源码迁移 `29.sql`–`48.sql` 连续，隔离数据库 readback 为 `48`，不能替代默认目标库升级。backup/restore 证据仍只覆盖本地隔离 PostgreSQL。
- 本轮只把两个失败 synthetic 中间目录移动到项目外可恢复 quarantine：`_localrun/synthetic-live-20260814-r11c` 和 `_localrun/synthetic-live-20260814-r11c-rerun`；对应清单随项目外历史 quarantine 保留。没有永久删除，没有动 ThingsVis/Native 源码、optional Compose/Nginx、依赖树、数据库或成功证据。

最终状态仍为：`native_core_status=local-core-verified`、`synthetic_software_status=software-path-passed/partial-current`、`thingsvis_optional_status=external-blocked/optional-disabled`、`real_rdi_status=not-tested`、`target_deployment_status=pending`、`production_signoff=not-ready`。

## 2026-08-14 r13 fresh quality/runtime/dev refresh

本节是当前 fresh 本地证据，覆盖前端质量门禁、fresh production build、隔离 API/E2E、真实浏览器登录和 dev server smoke；它仍不替代目标环境或真实 RDI 验收。

- Fresh frontend quality：主 `vue-tsc --noEmit --skipLibCheck --incremental false`、`typecheck:data-architecture`、`typecheck:packages` 均退出 `0`。`typecheck:packages` 必须通过 `pnpm.cmd run typecheck:packages` 调用，直接执行 workspace 脚本会被项目自身拒绝。
- Fresh Vitest coverage（r13 历史批次，原始目录已移到仓库外 quarantine）：405/405 test files、3575/3575 tests、0 failed；832 source files，statements/lines `134813/174404 = 77.30%`，branches `15154/19540 = 77.55%`，functions `4561/6828 = 66.80%`。报告目录和日志目录分离，数字由 `coverage-final.json` 与 `lcov.info` 回读确认。
- Fresh production build（r13 历史批次，原始目录已移到仓库外 quarantine）：直接 `pnpm.cmd exec vite build --outDir <absolute-dir>`，标准 `NODE_OPTIONS=--max-old-space-size=4096`，退出 `0`，没有覆盖 `frontend/dist`。
- Fresh isolated runtime（r13 历史批次，原始目录已移到仓库外 quarantine）：数据库 `aetherlink_iot_predeploy_retest_20260814_r13`，GMQTT `11092`，Backend `19997`，Preview `9725`，并显式把 `-PreviewDistDir` 指向 r13 build。aggregate `84/84`、API `64/64`、E2E `20/20`、`0 failed`、endpoint `372/372`、page/route `56/56`；business bucket `28/28`，严格 `businessClosureEvidence.business=27/27`。
- 唯一 partial-skip 仍是 visualization 的 3 个 `runtime-external / seedable=false`：ThingsVis project/dashboard 服务 `127.0.0.1:8000` 不可达、negative-menu 依赖同一服务、`THINGSVIS_MIRRORED_DASHBOARD_ID` 未配置。它们没有被 Native provider 或 mock 消除。
- 隔离库 readback 为 `sys_version=48`、`version=0.0.23`；新的 local custom-format `pg_dump/pg_restore` drill 退出 `0`，恢复库同样为 `48/0.0.23`，原始结果已移到仓库外 quarantine。这仍只是本机隔离 PostgreSQL 证据；默认 `aetherlink_iot` 目标库仍需单独升级/确认，不能被隔离库替代。
- Local dev smoke：Backend `19996`、Vite dev `9726`；dev proxy health `200`，login render `0` 且无 page error/failed request，真实 login E2E `12/12`；结束后 `19996/9726` 与 r13 runtime 的 `11092/19997/9725` 均无监听。
- Synthetic seed/activation 输出明确 `real_rdi_status=not-tested`、`production_signoff=not-ready`。本轮仍不能宣称真实 RDI PID、voucher、硬件身份、固件 MQTT session、物理遥测、物理在线状态、物理 ACK 或生产跨租户 share/link。

ThingsVis 清理决策：不清理。Native board 是默认本地核心 provider；ThingsVis 是显式可选的 legacy compatibility provider，仍有 provider composition、API wrapper、路由、iframe/SSO、旧 dashboard/menu、optional Compose/Nginx 和测试合同引用。`negative-menu` 是 dashboard-menu ownership rejection 的负向业务场景，不是服务或孤儿模块。类似 optional/external 能力也有当前引用，不能按“本地没有服务”批量删除。后续只可对确认无引用、无活动进程、可再生成的缓存/旧中间物做带 SHA-256 manifest 的可恢复 quarantine。

本轮方法修正：coverage 使用直接 Vitest + 唯一绝对 `--coverage.reportsDirectory`，不使用 `pnpm test:coverage -- --coverage.reportsDirectory=...`；主 typecheck 使用 `--incremental false`；full runtime 必须显式传新 `RunDir`、数据库、broker/backend 端口和 `-PreviewDistDir`；密码只通过当前进程环境注入；测试结束后必须再次核对 PID/端口。所有这些输出仍保留到最终冻结、清理和按方法重跑完成后。

## 2026-08-14 r12 device-page fresh browser refresh (historical; superseded by r13)

本轮在独立 PostgreSQL 数据库 `aetherlink_iot_predeploy_retest_20260814_r12_device_pages`、broker `127.0.0.1:11091`、backend `127.0.0.1:19998` 和 preview `127.0.0.1:9725` 上重新执行完整 runner；报告和归档原始文件已移到仓库外 quarantine。本轮 aggregate `84/84`、API `64/64`、E2E `20/20`、`0 failed`、endpoint `372/372`、page/route `56/56`；严格 `businessClosureEvidence.business` 为 `27/27`。这些数字仍不等于真实 RDI 或目标环境部署通过。

`e2e/02_device.spec.js` 本轮 `8/8` 浏览器用例通过，六个此前要求补 fresh 业务证据的页面均已被真实浏览器操作覆盖：

- `/device/thingsmodel`：创建物模型后填写名称、点击 Search，核对浏览器响应和页面结果，并在 `finally` 删除模板；
- `/device/grouping` 与 `/device/grouping-details`：创建分组、填写筛选条件、点击匹配行进入详情，核对列表/详情/统计 API 与页面状态，最后删除分组；
- `/device/service-access`：核对租户管理员权限拒绝 API、浏览器响应、空状态和 service catalog 入口；该用例不创建状态，因此 cleanup 为 N/A；
- `/device/share`：覆盖有效、无效、空 token，点击 retry，核对 accept/public API 和成功/错误/缺失 token 状态，最后撤销 token 并清理 fixture；
- `/device/shared-with-me`：在 recipient browser context 中实际接受 share、导航到 shared-with-me 页面、核对跨账号列表和重复接受结果，最后关闭 context、清理动态账号、撤销 token 和清理 fixture。

本轮隔离库 readback 为 `sys_version=48`；默认目标库仍只读为 `aetherlink_iot: sys_version=45, version=0.0.23`。本轮结束后 `11091/19998/9725` 均无监听；隔离库中的 `e2e_ui_device_*`、`e2e_device_group_*`、`e2e_thing_model_*` 和动态 share recipient marker 均为 `0`。该 lane 使用显式 `synthetic-rdi` fixture，页面结果只能标记为 `synthetic-rdi / partial-current`，不能标记为 `real-rdi`。

Visualization 的 3 个 `runtime-external / seedable=false` partial-skip 在 r12 仍然保留：ThingsVis `127.0.0.1:8000` 不可达、negative-menu 依赖同一 ThingsVis 服务、`THINGSVIS_MIRRORED_DASHBOARD_ID` 未配置。Native 已通过不能关闭这些门禁。

本轮只做了两项可恢复移动：此前后端不可达、尚未启动浏览器的 focused 尝试，以及无当前引用且已被新鲜质量证据取代的旧 frontend local 日志目录；对应清单随项目外历史 quarantine 保留。没有永久删除、没有删除 ThingsVis/Native 源码、没有删除依赖树、没有删除当前 r12 成功证据，也没有修改默认数据库。
