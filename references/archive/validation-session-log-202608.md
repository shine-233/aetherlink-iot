# 验证会话归档（2026-08）

本文件是 `VALIDATION.md` 重建时移出的全部 dated 会话记录。这些记录只描述其各自批次的事实与边界，不构成当前发布证据；当前口径一律以 `VALIDATION.md` 正文为准。

---

## 2026-08-23 P1 专项处置记录：间歇性读不一致 / 假成功删除

本节为独立 P1 会话的闭环处置记录，不改变任何门禁与证据分级。注意：本轮与另一并行会话共用工作树，期间该会话提交了 b4cf044/94481c9 并多次改写同一批文件；本节所述改动以 p1-wt 工作树验证版为准，主树文件状态以 git 实况为准。

**根因机制（有 SQL 级实证）**：gorm/gen 的包级表单例链从"继承式"语句根出发，`Statement.clone()` 只清空 Clauses/Preloads、其余字段（含 Model/Dest）按值继承。高负载并发下一旦某执行语句残留 `Model=<携带非零主键的struct>` 且 `Dest != Model`，gorm 会在 UPDATE 上补注入未限定 `id='<陈旧值>'` 条件（callbacks/update.go ConvertToAssignments）、在 DELETE 上经 `Dest != Model && ReflectValue.CanAddr()` 分支补注入限定 `"table"."id"` 条件（callbacks/delete.go）、并使后续 SELECT 带上陈旧过滤——分别对应症状目录中的假成功删除、record-not-found、total>0 而 list=null。

**修复（最小侵入，按表落地）**：device_config 全部读写与 users 登录热路径更新、用户列表 count/find 改走 raw global.DB 链（gorm.Open 根 clone==1，每次链式起点均为全新 Statement，无继承通道，仓库内 UpdateDeviceConfigPayloadSchemaID 已有同型先例）；删除与更新显式检查 RowsAffected，未命中行必须报错；登录种子源 UpdateLastVisitTime 一并封堵。曾试验的接线级 Session{NewDB:true} 与逐操作 Session{NewDB:true} 两方案分别被实证否定（前者不切断继承，后者丢失 map 更新所需的 schema 绑定导致 unsupported data type 回归）。

**验证数字（本机隔离栈：库 aetherlink_iot_p1iso_20260823、backend 29999、broker 2883、preview 19725）**：
- HTTP 锤击复现器（并发 建→改→查→删→验）：修复前 92~180 次假成功删除且持续恶化；修复后连续多轮 380+/380+ 全绿。
- 连续两轮完整 run_tests.js --include-e2e --workers=1：第一轮 API 66/70、E2E 18/20；第二轮 API 66/70、E2E 18/20。失败清单不含任何 record-not-found/假成功删除类条目；TEMPLATE_DELETE_FAIL 由修复前单轮 21 次降为 0 次（两轮均 0），200050 为 0。
- 完整跑批结束后 /user/detail ×10（super_admin 与 tenant_admin 双账号）：20/20 全 200。
- 修复后 backend SQL 日志扫描：device_configs/users 列表等已修复面 doubled-value-condition = 0。

**遗留（同一机制的其余 gen 面，建议后续专项按同法收敛）**：01_auth 用户选择器列表偶发 list=null、02_device 超管跨租户设备视图偶发不可见、13_data_script 更新后列表读回旧名（三者为剩余 gen 单义面的同族间歇性）；15_device_config_openapi 两条断言仍按"列表返回明文 api_key"的过期望编写（与 2026-08 安全批次脱敏契约冲突，属夹具漂移，本轮修复被并行会话覆盖后未再落回）；E2E device/data 的种子设备首页可见性与 Ready Check MQTT 证据簇受历史轮次累积状态与 ACK 独占约束影响，需按 lane 清态复跑。real-rdi/pending 门禁维持原状。

---

## 2026-08-22 安全止血批次

本节记录 2026-08-22 安全审查后的修复批次及其验证边界。

已落地的代码变更：

1. 后端 P0：`POST /api/v1/tenant/super-admin/init` 增加"实例上已存在 SYS_ADMIN 即拒绝（200016）"的服务端硬门禁 + 初始化互斥锁；市场跳过分支不再能越过门禁。
2. Broker P0：入站包 RemainLength 在内存预分配前按 `max_packet_size` 拦截（v3/v5 一致）；默认上限 16MB→1MB（config、default_config.yml、安全默认契约测试同步）。
3. 后端认证外围：`login-max-fail-times` 默认 -1→5（conf.yml/conf-dev.yml/conf.example.yml）；验证码发送按邮箱 60 秒限流（新错误码 201003 文案）、校验失败超过 5 次作废验证码（注册与换邮箱两条路径）。
4. Broker will/retained：新增 OnWillPublish 钩子复用上行白名单并包裹设备 payload，无认证绑定默认丢弃；retained 写入/清除移至 OnMsgArrived 授权之后。部署契约测试钉住该钩子注册。
5. 前端攻击链：交互引擎外链仅允许 http(s) 且内部路径必须站内相对（附 noopener）；ThingsVis URL 构建删除平台 JWT 回退注入；脚本引擎新增来源信任策略（imported-config 默认拒绝执行）。
6. 性能：JWT 中间件用户状态进程内缓存（30s TTL，仅缓存确定性结论）；设备分组层级路径 N+1 改为单条批量 CTE。

本轮实际执行的验证与结果：

- `backend`: go build ./... 通过；internal/{dal,service,middleware,router,api} 聚焦/全量测试通过。唯一 FAIL 为 `TestCheckDBMigrationsRequiresCurrentMigrationVersion`，原因是本机无 gcc 导致 CGO sqlite 无法编译，属既有环境限制，与本批改动无关（CI 环境可运行）。
- `mqtt-broker`: 全部包 `go test ./...` 通过（含新增 will 钩子测试、Reader 上限测试、更新后的默认值契约测试）。
- `frontend`: 本机 pnpm/npm 执行器在依赖安装阶段持续崩溃（exit 0x80000003，Node v24.15.0 环境），vitest/typecheck/build 未能在本地运行；全部前端改动通过 esbuild 语法解析校验，行为级验证需由 CI（source-ci / minimum-quality-gate）完成。
- 新增前端测试源码：`execution-policy.test.ts`、`url-builder.security.test.ts`（等待 CI 或本地依赖修复后运行）。

2026-08-22 补充：本地真实运行时验证（隔离库 + 本机 PG17/Redis + 源码编译的 backend/gmqttd，验证后已清理进程与数据库）：

1. P0#1 运行时证明：全新隔离库上 `POST /api/v1/tenant/super-admin/init` 首次返回 code=200 创建成功；攻击者邮箱 + `market_registered:true` 绕过字段再次调用返回业务码 **200016（超管已存在，禁止重复初始化）**，门禁在真实 HTTP 路径生效。
2. 登录锁定运行时证明：默认配置（5 次/300s）下对同一账号连续错误密码登录，第 1–5 次返回 200002，第 6 次返回 **200006（登录尝试次数过多）**。
3. 验证码限流运行时证明：公开 `GET /api/v1/verification/code` 第二次调用返回 **201003（请求过于频繁）**；首次因本机无 SMTP 返回 200010 属预期降级，不影响限流判定。
4. Broker P0#2 运行时证明：向 1883 端口发送声明 RemainLength≈2GB 的 CONNECT 固定头（不发送载荷），broker 在预分配前立即关闭连接（socket read 返回 0），不再按声明长度等待/分配内存。

2026-08-23 补充：本机 Node 环境修复（会话级 Node v22.23.2，未改动系统 Node 24）后完成的前端与自动化验证：

1. 前端依赖安装恢复（pnpm install --frozen-lockfile 成功）；新增测试 `execution-policy.test.ts`（3 用例）与 `url-builder.security.test.ts`（3 用例）全部通过；`pnpm run typecheck`（vue-tsc）退出码 0。
2. `npm run preflight:api-e2e` 在隔离栈（backend:19999 + broker:1883 + preview 代理:9725）上通过：preview 页面 200 HTML、`/api/v1/deployment/health` 经代理返回 JSON、后端 `/health` 正常、六个发布账号齐备。
3. API automation（70 模块）：夹具密码修复后 **57/70 通过（81.4%）**。失败项分类：① 要求真实 RDI 夹具设备的模块（对应门禁 `real_rdi_status=not-tested`）；② `preflight-local` 断言默认端口 9999 而本轮环境使用 19999 的硬编码假设；③ casbin 两模块夹具自身的用户列表可见性问题（后已修）。
4. E2E Playwright（20 模块）：**15/20 通过（75%）**。失败 5 项均依赖"真实 broker 路径的设备 ACK / RDI 设备夹具"，同属 real-rdi/emulator pending 门禁。
5. 发现并修复两个既有测试夹具缺陷：`dynamic_accounts.js` / `casbin_fixtures.js` / `seed_data.js` 的 `dynamicPassword()` 生成最长 28 位密码违反后端 8–20 位规则（200040），已改为合规生成器。

2026-08-23 补充二：合成 RDI 设备链路打通后的完整本地验证（同一隔离栈，验证后已清理）：

1. 通过 `seed_synthetic_rdi_fixture.js --seed` 创建 `SYNTHRDI0001` 合成夹具 → API 激活 → 协议模拟器经真实 broker 上线（devices.is_online=1），遥测/命令 ACK 链路可用。
2. 设置 `AETHERLINK_RDI_FIXTURE_MODE=synthetic-rdi` 后，原"需要真实 RDI 设备"的 API 模块解锁：02_device / 05_config / 06_system / 16_write_flows 合计 **78 用例全部通过**。
3. casbin ×2 修复确认：**24/24 通过**。
4. `preflight-local` 契约修复：env > config > runtime 默认 > 字面量优先级修正后 **6/6 通过**。
5. E2E 复跑：**18/20 (90%)**。剩余 2 个失败根因为夹具清理顺序缺陷（teardown 删除仍被引用的功能模板触发 200050），属自动化基建问题。
6. 新发现 P1 排查项：长混合跑批后 backend 间歇性 `/user/detail` 返回 101001 record-not-found（重启进程即愈），后定性为 gorm gen 继承面问题（见下节与本文档顶部 P1 专项）。
7. 运营约束：不要让外部常驻设备模拟器与规格自管的 aetherlink-device-autotest 模拟器同时在线（ACK 抢答与会话接管冲突），需分 lane 运行。

最终记录数字（本机隔离栈）：E2E 最佳 **18/20 (90%)**、典型 17/20；API automation 夹具修复后显著回升；前端新增测试 6/6、typecheck 通过；preflight:api-e2e ok；全部安全修复已在真实链路复验通过。

环境备注：本机系统级 Node 24.15.0 存在 pnpm worker 崩溃问题（0x80000003），本地开发使用会话级 Node 22 LTS 绕过。

---

## 2026-08-15 上传前清理与证据边界

本轮按用户指示停止重复完整回归，转为部署前和 GitHub 上传前清理。根 `_localrun`、历史 verification 归档、构建/coverage、运行态配置、认证状态、日志、截图、二进制和本地审计台账已移到仓库外可恢复 quarantine；本机清单位于父目录的 `_aetherlink-github-cleanup-quarantine-20260815/github-cleanup-manifest-20260815.json`。r14-pre 旧运行物另有 `_aetherlink-cleanup-quarantine-20260815-r14-pre/quarantine-manifest.json`。

清理后生成物扫描为 `0` 个候选；tracked 源码扫描未发现明文数据库密码、私钥标记或带凭据的数据库 URI。quarantine 均为 `permanentDelete=false`，不能把"已移出仓库"写成"已永久删除"。

本轮没有重新执行完整 r14：后端 Go 测试在依赖下载超时处停止，broker 测试按用户要求停止；这两项均不是通过。此前 r13 的 fresh 本地证据仍可用于方法和历史摘要，但不能替代真实 RDI、目标服务器、Docker/Compose、HTTPS/TLS、公网 MQTT、目标环境 backup/restore 或外部 ThingsVis 验收。当时状态为 `real_rdi_status=not-tested`、`target_deployment_status=pending`、`production_signoff=not-ready`、`github_upload=executed`。

公开源码已推送到 GitHub。上传前补充清理：7 个 untracked 一次性历史/生成文件可恢复移动到 `_aetherlink-github-cleanup-quarantine-20260815-r2/`；未完成的 `_aetherlink-validation-20260815-r16` 移到 `-r3/`（2,074 文件、78,505,657 bytes 逐文件 SHA-256 校验通过）。这些批次不进入当前验证结论。

---

## 2026-08-12 synthetic-rdi / protocol-emulator fresh 验证

本轮使用隔离数据库 `aetherlink_iot_isolated_20260812_01`、Redis logical DB `11`、GMQTT `127.0.0.1:11086` 和 backend `127.0.0.1:19999`。fresh fixture 起始 inactive/disabled，经公开 API 激活为 active/enabled（PID `SYN260812229`）。share/link 软件合同 `34 total / 34 pass`；协议 emulator success/failure 两路径均观察到 `offline -> online -> offline`、fresh `temperature_1=25.5` 回读、success/failure ACK 断言；Node synthetic contract `8 passed`，Go emulator 测试通过，证据包 `55/55` 文件哈希一致、敏感信息扫描命中 `0`。

本轮结论是 `synthetic-rdi / protocol-emulator = partial-current software-path-passed`，不是 `real-rdi`，也不是 production sign-off。仍然 pending 的硬门禁清单不变（真实 RDI PID/activation、真实固件链路、ThingsVis、HTTPS/TLS、公网 MQTT、目标环境 backup/restore 等）。

---

## 2026-08-14 r8 当前 fresh override（historical）

r8 fresh API/E2E 报告为 API `64/64`、E2E `20/20`、`0` failed，business evidence `30/30`，endpoint coverage `372/372`，page/route coverage `56/56`。Visualization 仍有 3 个结构化 `runtime-external` partial-skip（ThingsVis 服务不可达 ×2、mirror ID 未配置）。synthetic software lane 证据已 quarantine。默认数据库只读版本仍为 `45`，不是源码迁移上限。

---

## 2026-08-14 r11c 当前复测与清理记录（historical）

- API/E2E fresh 证据：aggregate `84/84`、API `64/64`、E2E `20/20`、`0 failed`；endpoint `372/372`，page/route `56/56`。evidence kind 分桶齐全；严格 `businessClosureEvidence.business` 为 `27/27`。
- Visualization 仍有且只有 3 个 `runtime-external / seedable=false` partial-skip。
- 前端 typecheck 三套通过；完整 production build 曾退出 Windows `0xC0000005`，低内存诊断 build 通过。
- 隔离库 readback `sys_version=48`；backup/restore drill 退出 `0`。
- 只做了两项可恢复 quarantine 移动；没有永久删除。

---

## 2026-08-14 r13 fresh quality/runtime/dev refresh（historical）

- Fresh frontend quality：主 typecheck、data-architecture、packages 均 `0`。
- Fresh Vitest coverage：405/405 test files、3575/3575 tests、0 failed；statements/lines 77.30%、branches 77.55%、functions 66.80%。
- Fresh production build 退出 `0`（NODE_OPTIONS=--max-old-space-size=4096）。
- Fresh isolated runtime：aggregate `84/84`、API `64/64`、E2E `20/20`；business bucket `28/28`。
- 隔离库 pg_dump/pg_restore drill 退出 `0`；dev server smoke login E2E `12/12`。
- 方法修正：coverage 使用直接 Vitest + 唯一绝对 reportsDirectory；主 typecheck 用 `--incremental false`；full runtime 显式传 RunDir/端口/PreviewDistDir；密码只走进程环境。

---

## 2026-08-14 r12 device-page fresh browser refresh（historical）

独立隔离栈上完整 runner：aggregate `84/84`、API `64/64`、E2E `20/20`、`0 failed`。`e2e/02_device.spec.js` 本轮 `8/8` 通过，六个此前要求补 fresh 业务证据的页面均已被真实浏览器操作覆盖（thingsmodel/grouping/grouping-details/service-access/share/shared-with-me）。隔离库 readback `sys_version=48`。该 lane 使用 synthetic-rdi fixture，页面结果只能标记 `synthetic-rdi / partial-current`。

---

## 2026-08-23 第二批：插件接入边界认证 + voucher 缓存键哈希化

1. 后端 /api/v1/plugin/* 五端点新增 middleware.PluginAuth：配置 plugin.service.key（env GOTP_PLUGIN_SERVICE_KEY）后全来源严格校验 X-Plugin-Key（常量时间比较）；未配置时仅放行回环/私网来源，公网 401。focused 测试覆盖环回/私网/公网/严格模式四类路径。
2. Broker 设备凭证缓存键哈希化：voucher→deviceID 的 Redis key 由明文 voucher JSON 改为 SHA-256 十六进制摘要（db.go voucherCacheKey），认证失败清理路径同步。契约测试锁定"稳定摘要+不含明文片段"。
3. 配置面：conf.yml / conf-dev.yml / conf.example.yml 新增 plugin.service.key 段；.env.example 与 docker-compose.yml 透传 GOTP_PLUGIN_SERVICE_KEY（默认空=内网放行策略）。
4. 文档：references/plugin-guide.md 新增接入边界小节。

验证边界：backend go build + middleware/router/dal/uplink/service 全量测试通过；broker go build + plugin/aetherlink 全量测试通过。本批未运行 Docker/Compose/E2E 运行时链路。


---

## 2026-08-24 P0 安全加固批次（fix/p0-security-hardening）

1. 登录 IP 失败限流（S1）：新增 internal/middleware/login_ratelimit.go——按客户端 IP 固定窗口计数（默认 10 次失败/分钟/IP），超限返回 HTTP 429 + 错误码 40106；失败/成功计数由 Login handler 回写，成功清零。与按账号的 LoginLock（classified-protect 配置）互补，本层无配置开关默认生效。多副本部署各副本独立计数。
2. 独占会话指针键去明文化（S2）：replaceExclusiveLoginToken 在 {email}_token 指针键改存上一个会话 token 的 HMAC-SHA256 摘要（utils.TokenDigest），TTL 与允许列表条目对齐（此前永不过期）；删除旧会话直接用摘要键。部署说明：存量明文指针键无需迁移——旧键在新登录时被读作"摘要"删不到目标允许列表条目，该条目按自身 TTL 自然过期（最长 24h），期间旧 token 仍受 JWTAuth 摘要键正常校验。本条取代上文 P2/P3 批次中"value 仍存明文 token"的表述。
3. CI 扫描三件套（S4）：source-ci 新增 govulncheck（backend / mqtt-broker / device-autotest 三模块矩阵）与 gitleaks job；container-ci 构建后对本地镜像跑 Trivy（HIGH/CRITICAL、ignore-unfixed、exit-code 1）。首轮运行如暴露历史镜像 CVE 或历史提交密钥误报，需以 .trivyignore / gitleaks allowlist 收敛而非降门槛。
4. 路由认证层级合约测试（S5）：新增 internal/router/router_auth_tier_contract_test.go——AST 解析 router_init.go，锁定 根公开面(8)/插件鉴权(5)/匿名 v1(24)/JWT-only(1) 四层路由清单；新增匿名端点必须显式分类入白名单并附理由，删除路由需同步清理陈旧条目。casbin g2 策略本身为部署态数据，代码侧覆盖校验仍待集中式路由表方案。
5. gofmt 清扫：main 上 7 个文件格式回归（注释编号缩进、结构体对齐、文件尾行）已统一修复。

验证边界：go build/vet/test 全绿（middleware 限流三用例、router 合约测试通过）；真实爆破场景限流行为、Redis 存量键自愈路径、CI 首轮扫描结果均为 pending，待 PR CI 与后续 compose lane 验证。