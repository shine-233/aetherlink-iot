# 2026-09-25 路线图缺口批次三实施证据（两程 15 项：第一程 9 + 第二程 6）

> 对应 `ROADMAP.md` §9"2026-09-25 批次三实施记录"；缺口编号沿用缺口体系（§1.5）。
> 分支 `feature/roadmap-complete-tb-tp-parity`。
>
> **口径声明（重要）**：批次三分两程实施——**第一程 9 项**（TB-45、TB-46、TP-03、TB-19、TB-23、TB-11、TB-22、TP-20、TP-21）的交付说明转录自编排方提供的批次三第一程实施结果摘要（工作树已包含其改动，按约定无需重跑）；**第二程 6 项**（TB-47、TB-48、TB-15R、TB-49、TP-22、TB-17R）的逐项内容（交付说明 / 改动文件 / 测试 / residual / 门禁明细）**转录自第二程实施结果 JSON**。本文档撰写会话（路线图与证据更新员，实跑日 2026-09-26）仅对关键落点做了**存在性抽查与迁移链核验**（见 §6 实跑记录），**未复跑任何测试**。凡本文出现的"通过/全绿"结论，其执行主体均为各实施会话或编排方收尾门禁，更新会话不为其背书。
>
> 第一程实施结果仅含交付摘要（feature + summary 一句话），**未附逐项 residual 明细与逐文件清单**；本文第一程各小节的"关键落点"为更新会话按摘要抽查实测存在的文件，residual 一栏如实标注"明细未随材料提供"（TB-11 例外，其专项证据文档由实施会话本人整理，含完整 residual 与实跑记录）。

## 0. 批次总览与状态口径

- 本批次实施结果共 **15 项（两程合计）**：第一程 **9 项全部 delivered**；第二程 **6 项全部 delivered（TB-47、TB-48、TB-15R、TB-49、TP-22、TB-17R——TB-17R 见 §1.15 的 2026-09-26 状态更正）**。
- **状态口径**：按 ROADMAP §2 四面一致定义，第④面（自动化测试运行）缺——批次三契约测试（落号 87/88/90/91/92/93/94/95/96/99/100/101 共 **12 个**，89/97/98 三个号位未占用）已就位、按批次约定**未在活栈运行**（本机无 docker）——故 ROADMAP §4.1 批次三十四个已交付行与 §3 新增十三行状态一律记 **partial†**，待活栈回归后升 done。
- **迁移链**：批次三新增 `130.sql`~`138.sql`。对应关系（更新会话实测，含编号重排）：
  - TB-45 → `130.sql`（integrations）；TB-46 → `131.sql`（用户组 GPE 三表）；TB-47 → `134.sql`（白标，前一程交付）；TB-48 → `137.sql`（scheduler_events）；TB-15R → `138.sql`（行级 data_policy）。
  - **TB-19 首轮落 `132.sql`、TB-23 首轮落 `133.sql`；2026-09-26 批次三编号重排后分别改挂 `135.sql`/`136.sql`**，`132.sql`/`133.sql` 转为 **NO-OP 占位迁移**（两文件头注释实测载明重排背景：pg_init.go 迁移循环对缺文件 fail-fast，故保留号位保证 1..N 连续可执行；对已执行过旧 132/133 的库无需回滚——135/136 的 ADD COLUMN IF NOT EXISTS / CREATE TABLE IF NOT EXISTS 幂等，两条升级路径结果一致）。
  - TB-11、TB-22、TP-20、TP-21、TP-22 无新迁移；TB-49 无 DDL（`78.sql` 仅注释行同步格式口径，VERSION_NUMBER 未动）；TB-17R 无迁移。
  - 更新会话实测：`backend/pkg/global/global.go:21` `VERSION_NUMBER = 138`；`backend/sql/` 下 .sql 文件 **138 个**，编号最小 1、最大 138、1~138 逐号 diff **无缺号**。
- **收尾门禁（任务书下达口径）**：`openapi=0`、`goTest=0`、`vue-tsc=0`、`vitest=0`（见 §2）。

## 1. 逐项交付记录

### 1.1 TB-45 集成连接器与数据转换器框架（Integration 实体纳管管线）—— delivered（第一程）

**交付说明**（第一程材料转录）：`130.sql` integrations 表 + Casbin/菜单种子；四层 CRUD；OPC UA 采集路径上行转换钩子（存量采集器改造为 Integration 实例）；统一集成管理页；契约测试 87。

**迁移**：`backend/sql/130.sql`

**关键落点**（更新会话实测存在）：`backend/sql/130.sql`、`backend/internal/service/integration.go`、`backend/internal/api/integration.go`、`backend/router/apps/integration.go`、`frontend/src/views/integration/list/`、`automation_tests/tests/87_integration_management.test.js`（仅存在性核验，未运行）。

**residual**：第一程材料未含本项 residual 明细（见头部口径声明）。

### 1.2 TB-46 用户组与组权限（GPE v1）—— delivered（第一程）

**交付说明**（第一程材料转录）：`131.sql` 三表 + Casbin + 组共享 fail-closed scope；用户组管理页；契约测试 88。**范围边界：customer 用户不接入组授权（v1 边界，与 §4.1 验收点一致）。**

**迁移**：`backend/sql/131.sql`

**关键落点**（更新会话实测存在）：`backend/sql/131.sql`、`backend/internal/dal/user_group.go`、`backend/internal/service/user_group.go`、`backend/internal/api/user_group.go`、`frontend/src/views/management/user-group/index.vue`、`automation_tests/tests/88_user_group_management.test.js`（仅存在性核验，未运行）。

**residual**：第一程材料未含本项 residual 明细；customer 用户不接入为交付摘要明示的 v1 范围边界，非缺陷。

### 1.3 TP-03 TCP 协议接入 —— delivered（第一程）

**交付说明**（第一程材料转录）：门控入站 TCP 网关（帧解析/注册簿/断网缓冲）+ 设计文档 + 契约测试 90。

**迁移**：无新迁移。

**关键落点**（更新会话实测存在）：`backend/internal/protocolgw/tcp/`（framing.go、registry.go、gateway.go、ingest.go、downlink.go 及配套 `*_test.go`）、`backend/internal/app/tcp_gateway.go`、`automation_tests/tests/90_tcp_gateway.test.js`（仅存在性核验，未运行）。第一程摘要所称"protocols.tcp 门控入站 TCP 网关"对应上述 `internal/protocolgw/tcp` 包（更新会话实测路径）。

**residual**：第一程材料未含本项 residual 明细。

### 1.4 TB-19 Protobuf 载荷编解码 —— delivered（第一程；迁移号位重排 132→135）

**交付说明**（第一程材料转录）：proto_schema 列 + protoparse/dynamicpb 动态解码 + Dry-Run + converter 页 PROTOBUF 模式 + 契约测试 91。第一程摘要记迁移为 `132.sql`；**更新会话实测该迁移现挂 `135.sql`**（2026-09-26 批次三编号重排，`132.sql` 为 NO-OP 占位，见 §0）。

**迁移**：`backend/sql/135.sql`（首轮落 `132.sql`，重排后 132 转占位）

**关键落点**（更新会话实测存在）：`backend/sql/135.sql`、`backend/sql/132.sql`（占位 NO-OP，头注释载明重排背景）、`backend/internal/service/data_converter_protobuf.go`（grep 实测 protoparse/dynamicpb 引用）、`frontend/src/views/device/converter/index.vue`（:7/:49 实测 PROTOBUF 模式与示例 schema）、`automation_tests/tests/91_protobuf_converter.test.js`（仅存在性核验，未运行）。

**residual**：第一程材料未含本项 residual 明细。

### 1.5 TB-23 移动应用中心 —— delivered（第一程；迁移号位重排 133→136）

**交付说明**（第一程材料转录）：mobile_app_bundles 表 + 状态机八端点 + 应用中心页 + 契约测试 93。第一程摘要记迁移为 `133.sql`；**更新会话实测该迁移现挂 `136.sql`**（重排后 `133.sql` 为 NO-OP 占位，见 §0）。

**迁移**：`backend/sql/136.sql`（首轮落 `133.sql`，重排后 133 转占位）

**关键落点**（更新会话实测存在）：`backend/sql/136.sql`、`backend/sql/133.sql`（占位 NO-OP）、`backend/internal/model/mobile_app_bundle.go`、`backend/internal/dal/mobile_app_bundle.go`、`backend/internal/service/mobile_app_center.go`、`backend/internal/api/mobile_app_center.go`（及各自 `_test.go`）、`frontend/src/views/mobile-app/app-center/`、`automation_tests/tests/93_mobile_app_center.test.js`（仅存在性核验，未运行）。

**residual**：第一程材料未含本项 residual 明细；§4.1 本行验收点所列幂等 store 持久化与 uniapp 工程对接进展未随材料提供，维持开放口径。

### 1.6 TB-11 K8s/Helm 编排与多副本 —— delivered（第一程；本项有独立专项证据）

**交付说明**（第一程材料转录）：`deploy/helm` chart + 双副本 override + Go 渲染校验单测 + README K8s 章节。

**迁移**：无新迁移（chart 不含 SQL 面，schema 由 backend 启动迁移链承担）。

**关键落点**（更新会话实测存在）：`deploy/helm/`（chart 目录）、`backend/internal/helmchart/`（渲染器与校验单测）、`deploy/docker-compose.replicas.yml`。**本项能力面在部署/编排域：无 REST 契约面、无前端页面面**（专项证据文档 grep 实测 helmchart 包零 HTTP 接线），故无批次三契约测试落号。

**专项证据**：`docs/validation/2026-09-26-tb11-k8s-helm-multireplica-evidence.md`（实施会话本人整理，与本证据文档的"转录"口径不同——其"通过"结论来自实施会话实跑命令，含 helmchart 5 用例全 PASS、env 键集合与 compose 对齐由单测机械强制、双副本 broker 身份注入等）。**residual（转录自专项证据）**：真实集群安装验证与 broker federation 接线演练未做（无 helm/kubectl/docker 二进制；"渲染面通过不等于安装面通过"）；Ingress/TLS(8883)/TimescaleDB 变体/chart 打包未做。

### 1.7 TB-22 LwM2M/SNMP/CoAP 传输深度（SNMPv3 + 多客户端隔离）—— delivered（第一程）

**交付说明**（第一程材料转录）：SNMPv3 authNoPriv 接入采集链路 + per-endpoint ObjectStore 隔离 + 两个存量协议缺陷修复 + 契约测试 92。

**迁移**：无新迁移。

**关键落点**（更新会话实测存在）：`backend/internal/snmp/usm.go`（头注释实测："SNMPv3 USM（RFC 3414）最小安全层"，v3 模式必须显式选择认证协议、fail-closed）、`backend/internal/collector/snmp.go`、`automation_tests/tests/92_snmpv3_pointconfig.test.js`（仅存在性核验，未运行）。

**residual**：第一程材料未含本项 residual 明细。**注意口径**：§4.1 本行原验收点中的 LwM2M DTLS(PSK) 与 observe 服务端集成**未出现在交付摘要中，仍为开放项**；本项交付覆盖 SNMPv3 接入与多客户端 store 隔离两子项。

### 1.8 TP-20 国产 DB 方言适配层 —— delivered（第一程）

**交付说明**（第一程材料转录）：`internal/dialect` 纯函数包（101 表驱动用例）+ conf/docs 国产化部署章节。

**迁移**：无新迁移。

**关键落点**（更新会话实测存在）：`backend/internal/dialect/dialect.go`、`backend/internal/dialect/dialect_test.go`（实测 7 个测试函数，"101 用例"为表驱动用例数口径）、`docs/deployment-domestic-db.md`（grep 实测国产化部署文档）。

**residual**：更新会话实测 `backend/go.mod` 中 tdengine/kingbase **0 命中**——本项交付为方言适配层（纯函数抽象），§4.1 验收点"至少一个国产库驱动进 go.mod 并通过方言适配跑通迁移+读写"**未在本项交付摘要内，仍为开放项**。

### 1.9 TP-21 MSET 健康算法 —— delivered（第一程）

**交付说明**（第一程材料转录）：`internal/healthmset` 训练/推理/降级（16 用例）+ 健康评分扩展点接线（默认关）+ 前端 MSET 维度渲染。

**迁移**：无新迁移。

**关键落点**（更新会话实测存在）：`backend/internal/healthmset/`（mset.go、align.go、chi2.go、linalg.go 及 `mset_test.go` 11 个 + `align_test.go` 5 个测试函数，**合计 16 与摘要"16 用例"一致**）、`backend/internal/service/device_health.go`（评分扩展点）、`frontend/src/views/device/details/modules/health-assessment/index.vue`（grep 实测 MSET 引用）。

**residual**：第一程材料未含本项 residual 明细；评分扩展点**默认关**（交付摘要明示）；§4.1 验收点中预测性维护指标（剩余寿命/劣化趋势）未在交付摘要提及，仍为开放项。

### 1.10 TB-47 白标（租户翻译覆盖 + 自定义 CSS）——第二阶段收口 —— delivered（第二程）

**交付说明**（第二程实施结果转录）：门禁重验通过——vue-tsc 失败输出全部位于 `frontend/src/views/billing/api-quota/index.vue`（TRANSPORT_* 六个 TS2339，transport-quota 泳道工作区未提交改动 +89/−17），与本项文件无关；按约定等待 30 秒后重跑 `pnpm run typecheck` 得 EXIT=0、0 个 error TS，属别的泳道瞬时编译态自愈；本项无任何文件需修复，全项交付结论不变。

**迁移**：`backend/sql/134.sql`（前一程交付；收口阶段未新建迁移、未改 VERSION_NUMBER）。

**改动文件**（第二程 filesChanged 转录，共 31 个）：
- 后端：`backend/sql/134.sql`、`backend/internal/model/tenant_whitelabel.go`、`backend/internal/dal/tenant_whitelabel.go`（+_test）、`backend/internal/service/tenant_whitelabel.go`（+_test）、`backend/internal/api/tenant_whitelabel.go`（+_test）、`backend/router/apps/tenant_whitelabel.go`、`backend/router/apps/enter.go`、`backend/internal/api/enter.go`、`backend/internal/service/enter.go`、`backend/router/router_init.go`、`backend/pkg/errcode/code.go`、`backend/configs/messages.yaml`
- 前端：`frontend/src/locales/whitelabel-override.ts`（+__tests__）、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/custom.json`、`frontend/src/service/api/whitelabel.ts`、`frontend/src/service/api/index.ts`（+__tests__）、`frontend/src/store/modules/sys-setting/index.ts`（+__tests__）、`frontend/src/store/modules/route/index.ts`（+__tests__）、`frontend/src/store/modules/auth/index.ts`（+__tests__）、`frontend/src/views/management/setting/components/branding-setting.vue`（+__tests__）
- 契约测试：`automation_tests/tests/94_whitelabel_overrides.test.js`

**测试**（testsAdded 转录）：dal/service/api 三层 Go 单测；前端 locales、store（sys-setting whitelabel describe 块 5 用例）、branding-setting（TB-47 describe 块 8 用例）单测；契约测试 94（node --check 通过；覆盖写读闭环/UPSERT/删除/CSS 闭环/202007+202008 校验/角色边界 fail-closed/租户隔离与跨租户删除 0 命中；与 lib/api_client.js 签名逐一核对一致）。

**门禁**：go build + 定向 go test + vue-tsc 全过（**第 2 轮**，实施会话自报；首跑失败系他泳道 api-quota 瞬时编译态，30 秒重跑自愈后复跑定向 vitest 7 文件 68 用例全绿）。

**residual 与未做项**（实施会话回报转录）：
1. 【明确不做】内置菜单改名/隐藏/按角色（tenant_dashboard_menus 扩展）。
2. 契约测试 94 按约定只写好并过 node --check，未在活栈执行（本机无活栈，由编排方统一跑）。
3. 前端仅定向跑相关测试文件，后端仅定向跑 whitelabel 相关测试与静态守卫，未跑全量 vitest / 全量 go test（由编排方统一执行）。
4. overrides 为登录后可读端点，未提供匿名/登录页覆盖获取（如需需新立公开端点）。

### 1.11 TB-48 统一调度器（聚合 + 日历）—— delivered（第二程）

**交付说明**（第二程实施结果转录）：后端面（`137.sql` scheduler_events 注册面 + Casbin/菜单种子 + GET /scheduler/events 三源只读聚合（scene_automation_timers 联名/report_schedules 未软删/command_jobs 定时行 + 注册行，含 source_type/origin，scene 注册行与同 id 执行行去重）+ 注册面 CRUD（scene 事件同事务落既有 scene automation timer 机制执行，同 id 配对，report/rpc 仅登记））；前端面（`views/scheduler/calendar/index.vue` 月视图纯 CSS Grid 日历、事件按 next_run_at 聚合到日、来源类型着色（success/info/warning 设计令牌）、月份导航/来源过滤/图例、注册面管理弹窗 CRUD、schedulerRoutes.ts 手工四件套同步、四语言 locale 键集一致（route 3 键 + page.schedulerCalendar 37 键 ×4））；契约测试 95（12 用例）。

**迁移**：`backend/sql/137.sql`

**改动文件**（第二程 filesChanged 转录，共 28 个）：`backend/sql/137.sql`、`backend/pkg/global/global.go`、`backend/internal/model/scheduler_event.go`、`backend/internal/dal/scheduler_event.go`、`backend/internal/service/scheduler_events.go`、`backend/internal/service/enter.go`、`backend/internal/api/scheduler.go`、`backend/internal/api/enter.go`、`backend/router/apps/scheduler.go`、`backend/router/apps/enter.go`、`backend/router/apps/casbin_route_registration_contract_test.go`、`frontend/src/service/api/scheduler.ts`、`frontend/src/service/api/index.ts`、`frontend/src/views/scheduler/calendar/index.vue`、`frontend/src/router/elegant/{schedulerRoutes,routes,imports,transform}.ts`、`frontend/src/typings/elegant-router.d.ts`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/{route,page}.json`、`tsc-TB-48.log`

**测试**（testsAdded 转录）：
- Go 单测：`backend/internal/dal/scheduler_event_scope_test.go`（8 测试：四源聚合租户隔离/CRUD 双条件隔离/scene 配对写入/执行行同步与删除/唯一启用定时器计数/场景自动化存在性）；`backend/internal/service/scheduler_events_test.go`（5 测试：scene 事件落 timer 机制/registry-only 事件校验/聚合列表四源归并去重过滤排序分页/scene 更新同步执行行/删除与跨租户 404）。
- 契约测试：`automation_tests/tests/95_scheduler_events.test.js`（12 用例：聚合列表可达与 source_type/origin 字段契约、rpc 事件 CRUD 全生命周期、注册行进聚合/删除后消失、校验矩阵（rpc 缺 next_run_at/带 cron/过去时刻、report 缺 cron、scene 自带 next_run_at/指向不存在自动化）、source_type 过滤、租户隔离（B 不可 Get/Update/Delete A 事件一律 100404、B 聚合不含 A 注册行）；已过 node --check，按约定本机无活栈未运行）。

**门禁**：go build + 定向 go test + vue-tsc 全过（**第 1 轮**，实施会话自报）。实施会话另报：go test 六包全绿 + go build 通过；node --check 通过；vitest -u 全前端套件（index.exports 快照更新含 5 个 scheduler 导出、locale-completeness 四语 page.json 叶键集 841 一致、design-token-contract 初版硬编码 hex 失败后改设计令牌复测通过）；vue-tsc EXIT=0（tsc-TB-48.log）。

**residual 与未做项**（实施会话回报转录）：
1. 【明确不做】把三套存量调度迁移为统一执行器——scene/report/fleet 各自执行语义保持现网行为，聚合 API 为只读面，scene 事件仅按 ask 落到既有 scene automation timer 机制。
2. report/rpc 注册事件为纯登记展示（不生成 report_schedules/command_jobs 行）。
3. 注册面 v1 cron 统一按 UTC 评估（scene_automation_timers.timezone 固定 'UTC'），按事件自定义时区留后续。
4. 聚合为内存归并分页（单源 2000 行封顶）；场景执行行的落地同步由 Go 单测覆盖。
5. 已知非本项问题：全前端套件另有 3 个测试文件模块级失败（automation/linkage-edit、device/template model-definition、alarm-configuration），实为并行泳道未提交的 sys-setting/index.ts（+80 行含 `import { i18n } from '@/locales'`，git diff HEAD 证实非本项文件）破坏了这些用例的 @/locales mock；按约定重试一轮结果一致，本项文件均不在其模块图中，未动他人文件。

### 1.12 TB-15R 行级 TTL（residual：档案/租户粒度保留）—— delivered（第二程）

**交付说明**（第二程实施结果转录）：后端面——`138.sql` 给 data_policy 增加可空 tenant_id/device_config_id（NULL=全局默认，既有两行语义不变）+ 行级部分唯一索引 uq_data_policy_row_level（COALESCE 归一，仅约束 tenant_id IS NOT NULL 的行）+ 新端点 casbin 登记，VERSION_NUMBER=138；DAL/服务实现"档案级(租户,档案) > 租户级(租户) > 全局"的行级 TTL 清理（档案级行精确删、租户级行排除同租户被档案覆盖设备、全局回落排除被任何启用行级覆盖设备，冷层 telemetry_rollups 同口径，停用/非法行级不产生覆盖），新增 POST /api/v1/datapolicy（返回新建 id）与 DELETE /api/v1/datapolicy/:id（全局行拒绝删）；前端——data-clear-setting.vue 增加作用域列（全局/租户级/档案级标签+租户·档案 id 明细）、"添加行级策略"弹窗（租户必选下拉 + 设备档案可清空下拉按租户过滤 + 保留天数/启停/备注，data_type 固定设备数据）与行级行删除入口，service/api/setting.ts 新增 createDataClear/deleteDataClear/fetchTenantOptions/fetchDeviceConfigOptions，api.d.ts 补 tenant_id/device_config_id 类型。

**迁移**：`backend/sql/138.sql`

**改动文件**（第二程 filesChanged 转录，共 21 个）：`backend/sql/138.sql`、`backend/pkg/global/global.go`、`backend/internal/model/data_policy.gen.go`、`backend/internal/model/data_policy.http.go`、`backend/internal/query/data_policy.gen.go`、`backend/internal/dal/data_policy.go`、`backend/internal/dal/data_policy_scope.go`（+_test）、`backend/internal/service/datapolicy.go`（+_test）、`backend/internal/api/data_policy.go`、`backend/router/apps/datapolicy.go`、`frontend/src/service/api/setting.ts`、`frontend/src/typings/api.d.ts`、`frontend/src/views/management/setting/components/data-clear-setting.vue`（+__tests__）、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/page.json`、`automation_tests/tests/100_row_level_data_policy.test.js`

**测试**（testsAdded 转录）：
- Go 单测：`backend/internal/dal/data_policy_scope_test.go`（TestTelemetryDeviceScopeFilterBuildsExpectedFragments / TestDeleteTelemetryDataBatchScopedOverrideSemantics / TestDeleteTelemetryDataBatchScopedDisabledPolicyDoesNotExclude / TestDeleteTelemetryRollupsBatchScopedFollowsHotLayerScope）；`backend/internal/service/datapolicy_test.go`（TestScopeOfDataPolicy / TestRowLevelPolicyAppliesTo / TestResolveEffectiveDeviceDataPolicyPriorityLadder / TestIsDuplicateDataPolicyErr / TestCleanSystemDataByCronRowLevelOverride）。
- 前端 vitest：`data-clear-setting.test.ts` 新增 8 个行级用例（作用域解析/选项加载/租户过滤/档案重置/缺租户拒绝/创建成功回刷/租户级空档案提交/删除确认回刷，全文件 23 用例）。
- 契约测试：`automation_tests/tests/100_row_level_data_policy.test.js`（11 个契约用例：行级 CRUD 全生命周期 + 唯一性重复拒绝 + data_type=2 拒绝 + 全局行保护 + TENANT_ADMIN 越权拒绝；node --check 通过，本机无活栈未运行）。

**门禁**：go build + 定向 go test + vue-tsc 全过（**第 1 轮**，实施会话自报）。

**residual 与未做项**（实施会话回报转录）：
1. 【明确不做】TimescaleDB 按行级动态重建 retention job：行级 TTL 一律走 CleanSystemDataByCron 分批 DELETE，全局 retention 仍按全局设备数据策略装配（57.sql 口径），138.sql 不触碰 add_retention_policy。
2. 行级 TTL 仅支持设备数据（data_type=1）：创建入口拒绝 data_type=2 行级行，cron 对存量此类行跳过并告警——操作日志行级保留不做。
3. 契约测试中"全局清理不越权删除被行级覆盖设备数据"的删除层覆盖语义由 Go 单测 TestCleanSystemDataByCronRowLevelOverride（sqlite 夹具）锁定，活栈契约测试不重复搭建设备+遥测夹具，只锁 API 层 CRUD/唯一性/保护/鉴权契约。
4. 契约测试与前端运行时行为未经真实栈验证（本机无活栈），编排方统一跑全量门禁时如有偏差需回补。
5. data-clear-setting 档案下拉按 device_config 列表行内 tenant_id 前端过滤（后端 GET /device_config 无租户筛选参数），若 SYS_ADMIN 下该列表为空则档案下拉为空（租户级创建不受影响）。

### 1.13 TB-49 报表 HTML/PDF 与附件投递 —— delivered（第二程）

**交付说明**（第二程实施结果转录）：后端 format 放开 csv/html/pdf（模型+HTTP 模型），report_render.go 以 html/template 内嵌表格模板与 go-pdf/fpdf 逐行表格渲染（复用 CSV 行/字节预算），report_smtp.go 全格式改 gomail 附件投递（文件名=清洗后 schedule 名+窗口结束日+格式扩展名，CSV 同样附件化）；前端报表页格式选择框由禁用 CSV 复选框改为 CSV/HTML/PDF NSelect（标签为组件内常量，编辑回填未知格式回落 csv，与后端 fail-closed 一致），API 类型同步 ReportScheduleFormat；Go 单测、前端单测、96 号契约测试就绪且定向门禁全部通过。

**迁移**：无新迁移（`backend/sql/78.sql` 仅注释行同步格式口径，无 DDL，VERSION_NUMBER 未动）。

**改动文件**（第二程 filesChanged 转录，共 14 个）：`backend/internal/model/report_schedule.go`、`backend/internal/model/report_schedule_http.go`、`backend/internal/service/report_run_processor.go`、`backend/internal/service/report_render.go`（+_test）、`backend/internal/service/report_smtp.go`、`backend/internal/service/report_schedule.go`、`backend/go.mod`、`backend/go.sum`、`backend/sql/78.sql`、`frontend/src/service/api/report.ts`、`frontend/src/views/visualization/report/index.vue`（+__tests__）、`automation_tests/tests/96_report_schedule_format.test.js`（更新会话实测 go.mod：`github.com/go-pdf/fpdf v0.9.0` :16、`gopkg.in/gomail.v2` :34）

**测试**（testsAdded 转录）：
- Go 单测 10 例（report_render_test.go）：TestReportGeneratorRendersHTMLWithHeaderAndRowCount / TestReportHTMLRenderingEscapesTelemetryValues / TestReportGeneratorRendersPDFWithHeaderBytes / TestReportGeneratorRejectsUnknownSnapshotFormat / TestReportRenderErrorCodeClassifiesByteLimit / TestReportAttachmentFilenameUsesScheduleNameAndDate / TestSanitizeReportFilenameStripsUnsafeCharacters / TestReportDeliveryEnvelopeAttachesArtifactInsteadOfInlining / TestDeliverClaimSendsPersistedPayloadAsAttachment / TestConfiguredReportSMTPAdapterAttachesArtifactAsMIMEPart。
- 前端 vitest：report 视图用例"carries the selected artifact format through create and edit round-trips"（11 用例全绿）。
- 契约测试：`automation_tests/tests/96_report_schedule_format.test.js`（6 例：html/pdf 创建回读、缺省回落 csv、xlsx 校验拒绝、PUT 更新格式推进 revision、跨租户隔离；已写好并过 node --check，本机无活栈未实际执行）。

**门禁**：go build + 定向 go test + vue-tsc 全过（**第 1 轮**，实施会话自报）。

**residual 与未做项**（实施会话回报转录）：
1. 【明确不做】富样式 PDF 模板与图表嵌入（按 ask 范围排除）；PDF 走 go-pdf/fpdf 核心字体仅支持 Latin-1，超集字符降级 '?'（HTML 模板全 UTF-8），后续如需 Unicode 字体/图表按 report_render.go 头注释的渲染器表方向扩展。
2. 前端格式标签 CSV/HTML/PDF 为组件内常量（frontend/src/views/visualization/report/index.vue:61-66），未新增四语言 locale 键（并行泳道纪律），如需本地化文案留待统一 locale 批次。
3. 契约测试 96 已写好并过 node --check，但本机无活栈未实际执行（属预期，由有活栈的环境跑）。
4. barrel 命令实际跑了全量前端套件：终局 444 文件/3974 用例全过，但另有 3 个文件级收集错误（alarm-configuration / model-definition / linkage-edit 均 0 test 收集失败，根因 src/store/modules/sys-setting/index.ts:41 等）——经 git status 核实为其他泳道在途的 store 模块改动（auth/route/sys-setting 均为 modified 未提交态）所致，重试 4 轮失败集合不变且与本项无关；本项相关测试均绿。
5. index.exports.test.ts 内联快照在工作树中的 diff 为其他泳道新增的运行时导出，本项新增的 ReportScheduleFormat 为纯类型导出不影响运行时快照。
6. 未跑全量 go test 与 OpenAPI 生成（由编排方统一执行）；本项无新页面，无需路由四件套与 sys_ui_elements 菜单种子。

### 1.14 TP-22 大屏固定分辨率与轮播投屏 —— delivered（第二程）

**交付说明**（第二程实施结果转录）：修复 serializeScadaCanvas 丢 displayMode 静默降级 bug；useCanvasEditor 增加 displayMode/setDisplayMode（fixed1080 自动哨兵 1920×1080）；scada 编辑器 fixed1080 transform:scale 等比适配（computeFitScale+ResizeObserver+拖拽反除）；/tv-preview 扩展多屏轮播（tokens+interval URL 参数、provider 契约 getDashboardsByShareTokens 批量解析、注入式定时切换、Fullscreen API、四语言 locale）。门禁已复跑全绿（EXIT=0 六包全 ok）；此前门禁失败的三个调度器用例均为并行调度器泳道瞬时未完成态（未跟踪新文件且仍在编辑，与本项文件零交集），按"等 30 秒重试验证"纪律重试后全部自愈。

**迁移**：无新迁移、未改 VERSION_NUMBER。

**改动文件**（第二程 filesChanged 转录，共 30 个）：
- 后端：`backend/internal/scadadoc/scadadoc.go`（+_test）、`backend/internal/api/board_carousel.go`、`backend/internal/service/board_carousel.go`（+_test）、`backend/internal/service/scada_document_display_mode_test.go`、`backend/internal/service/scada_document.go`、`backend/internal/dal/board.go`、`backend/router/router_init.go`、`backend/router/casbin_registration_report.go`（+_test）
- 前端：`frontend/src/views/scada/core/canvasDocument.ts`、`useCanvasEditor.ts`（及各自 __tests__）、`frontend/src/views/scada/index.vue`、`frontend/src/views/visualization/thingsvis-preview/index.vue`、`carousel.ts`（+__tests__×2）、`frontend/src/service/api/board.ts`、`frontend/src/service/api/__tests__/index.exports.test.ts`、`frontend/src/service/visualization-provider/{contracts,native-board-provider}.ts`、`frontend/src/locales/langs/{en-us,zh-cn,es-es,fr-fr}/rdi.json`
- 契约测试：`automation_tests/tests/99_tp22_canvas_display_mode.test.js`

**测试**（testsAdded 转录）：scadadoc 壳层契约（oneof/双键/尺寸/向后兼容）；board_carousel（token 清洗 + 批量解析行为锁）；scada_document_display_mode（四写路径接线）；carousel.test.ts（URL 参数解析/interval 边界 + 假时钟轮播计时器逐 tick）；canvasDocument.test.ts（displayMode 往返/冲突/尺寸拒绝 + computeFitScale）；useCanvasEditor.test.ts（display mode 5 用例）；thingsvis-preview index.test.ts（轮播 4 用例）；契约测试 99（fixed1080 1920×1080 创建/读回保留、尺寸不符/未知模式/双键冲突 fail-closed 拒绝、旧画布向后兼容、显示模式跨 publish/rollback 保留、公开轮播端点空 tokens 参数错误、发布原生看板取 share_token、无认证批量解析顺序保持 + missing_tokens 回填；已过 node --check → SYNTAX_OK，本机无活栈未运行）。

**门禁**：go build + 定向 go test + vue-tsc 全过（**第 2 轮**，实施会话自报）。门禁复跑实录：第一次完整跑 EXIT=1，失败点全在调度器泳道文件（与本项无关的未跟踪文件、仍在编辑）；30 秒后重试 dal ok，复跑完整门禁 EXIT=0 六包全 ok。另报：定向 vitest 8 文件 90 用例全 PASS；vue-tsc EXIT=0；design-token-contract PASS（引入 2 个硬编码 hex 超基线后改 rgb(var(--warning-color)) 复跑通过）；native-board-provider 13 用例 PASS；tenant-scope audit suspects=0、context.Background 预算、casbin 公开面契约均 PASS。

**residual 与未做项**（实施会话回报转录）：
1. 【明确不做】DataRoom 引擎收编（按 ask 排除）。
2. 公开轮播按 share token 取数而非 board id（公开面凭证语义，api/board_carousel.go:6-13 有注）。
3. interval 为纯展示参数（fail-soft clamp [3,3600] 默认 15s），不支持每屏独立停留时长。
4. legacy thingsvis provider 不支持批量轮播接口，页面显式提示仅支持原生看板。
5. 全屏需用户手势（浏览器策略），拒绝时静默降级继续轮播。
6. 编辑器模式切换文案为组件内常量（沿用该页既有硬编码英文惯例），轮播页新增文案已进四语言 locale（locale-completeness 12 用例 PASS）。

### 1.15 TB-17R 传输维度配额接线（residual）—— delivered（2026-09-26 状态更正，原单项门禁时序归因见下）

> **2026-09-26 状态更正（编排方终审）**：TB-17R 的全部代码实际保留在工作树（broker `mqtt-broker/plugin/aetherlink/transport_quota.go`、backend `internal/quota/transport_limit_publisher.go` + `transport_usage_reader.go` 及测试、契约测试 `tests/101_transport_quota.test.js`、api-quota 页 transport_* 字段与四语言键）。其单项 vue-tsc 门禁红系 TP-22 泳道在途编辑的时序归因（3 条错误全在 TP-22 文件，本项独立 typecheck EXIT:0）；TP-22 定稿后的**收尾全量门禁四项全绿（openapi=0 goTest=0 vue-tsc=0 vitest=0，全仓 vue-tsc/vitest 终审含 TB-17R 全部改动）**——故 TB-17R 状态由「blocked/已回退」更正为 **delivered（partial†，契约测试 101 待活栈）**。批次三最终口径：**15/15 项全部交付**。

**实施内容**（第二程实施结果转录，**不计入批次三交付面**）：传输维度租户日配额接线全量实施——broker 侧 MQTT CONNECT 认证路径按租户 Redis INCR 当日计数并与套餐限额缓存判定（fail-open，跨服务键契约双端钉死）；backend 侧 billing SubscribePlan 套餐变更时发布 MaxTelemetryPerDay 到共享 Redis 限额缓存，api-quota 报告与前端 Billing/API 配额页扩展 transport_* 同构字段（四语言 locale 键已补齐，$t() 接入）；契约测试 101_transport_quota.test.js 守护既有契约不回归。Go 门禁以 backend/ 相对路径 ../mqtt-broker/plugin/aetherlink（经 go.work workspace）三包全绿。

**blocked 原因与门禁记录**（实施会话回报转录）：vue-tsc 门禁连续失败 exit=2，按裁定以 60-120s 间隔重跑 5 次（ATTEMPT-1 至 ATTEMPT-5-FINAL，约 10 分钟窗口，日志 tsc-TB-17R-wait*.log），错误集恒为 3 条且全部位于 `src/views/visualization/thingsvis-preview/index.vue(115,41)(129,28)(130,33)`——TP-22 泳道在途编辑（'carousel-unsupported' 尚未注册进 VisualizationErrorCode）；grep 验证错误路径含 billing|api-quota|locales|transport = 0 命中，本项文件零错误；本项全部改动在 13:35-13:40 独立全量 typecheck 已 EXIT:0（tsc-TB-17R-phase2.log）。**门禁 3 轮未过，按批次约定登记为已回退**；未回退物理文件（回退无法消除他泳道错误反毁已验证交付）、未代修（裁定明确不授权触碰 TP-22 活跃编辑面）。

**改动文件**（第二程 filesChanged 转录，共 17 个，仅供回溯——不作为批次三交付面）：`backend/internal/quota/transport_limit_publisher.go`（+_test）、`backend/internal/quota/transport_usage_reader.go`（+_test）、`backend/internal/model/api_usage.go`、`backend/internal/service/billing.go`、`mqtt-broker/plugin/aetherlink/transport_quota.go`（+_test）、`mqtt-broker/plugin/aetherlink/hooks_auth.go`、`mqtt-broker/plugin/aetherlink/mqtt_diagnostic_recorder.go`、`frontend/src/service/api/billing.ts`、`frontend/src/views/billing/api-quota/index.vue`、`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/custom.json`、`go.work`、`go.work.sum`

**契约测试**：`automation_tests/tests/101_transport_quota.test.js`（node --check 通过；断言语义与 backend/internal/quota/decision.go:53-79 实读对齐；本机无活栈未运行）。

**residual**（实施会话回报转录）：CoAP/TCP 网关路径同构接线明确不做（ask 指定排除）；无迁移；扩展既有页面不涉及路由四件套。

**更新会话实测注记（2026-09-26）**：截至本文档撰写，上述 filesChanged 所列文件**仍存在于工作树**（抽查 11 个全部存在），且核心新文件（`backend/internal/quota/transport_limit_publisher.go`、`mqtt-broker/plugin/aetherlink/transport_quota.go`、`go.work`）经 `git status --porcelain` 实测为 **未跟踪（??）状态**——即物理回退未发生/未完成。本项"已回退"为按批次约定的**登记状态**（不计入批次三交付、对应 ROADMAP 行不升状态），工作树中残留改动的物理处置（彻底移除或重新立项）留编排方决定。

## 2. 收尾门禁（2026-09-25）

| 门禁项 | 结果 | 执行主体与口径 |
| --- | --- | --- |
| openapi | **0**（openapi=0） | 任务书下达的收尾门禁结果，编排方统一收尾执行；更新会话照录、未复跑 |
| goTest | **0**（goTest=0） | 同上（编排方统一收尾；各实施会话仅跑定向 go test，见 §1 各小节） |
| vue-tsc | **0**（vue-tsc=0） | 同上；第二程各实施会话自报 vue-tsc 过（TB-47/TP-22 为他泳道瞬时编译态重试后第 2 轮过） |
| vitest | **0**（vitest=0） | 同上（编排方统一收尾；TB-49 回报全量前端套件 444 文件/3974 用例全过 + 3 个他泳道文件级收集错误） |

- 各实施会话自报门禁形态：`go build ./...`（或定向包）+ 定向 `go test` + vue-tsc；第一程门禁明细未随材料提供（仅知已交付且工作树已含）。
- TB-17R 的 vue-tsc 门禁 3 轮未过 → blocked → 按约定登记为已回退（§4）。

## 3. 未运行面与局限（诚实边界）

1. **契约测试 87~101 未运行**：本机无 docker，活栈（PG+Redis+broker+backend）不可用。批次三新增契约用例实际落号 12 个（87/88/90/91/92/93/94/95/96/99/100/101；**89/97/98 三个号位未占用**），第一程 6 个 + 第二程 6 个，实施会话回报均通过 `node --check` 语法校验；按批次约定只写不跑，运行由编排方在活栈执行。TB-11 无 REST 契约面（helmchart 包零 HTTP 接线），本就无契约测试落号。
2. **OpenAPI 重生成不在实施会话范围**（编排方统一执行；收尾门禁 openapi=0 为编排方回报，更新会话未复核 paths 计数）。
3. **全量 `go test ./...` 与全量 vitest 未在各实施会话运行**（按约定交编排方统一执行；goTest=0/vitest=0 为编排方回报）。第二程个别会话的 barrel 快照命令实际触发全量前端套件（TB-49 回报 444 文件/3974 用例），其中 3 个文件级收集错误经 git status 核实为并行泳道在途 store 改动所致、与本批文件无关。
4. **更新会话（本文档撰写者）未复跑任何测试**：go test / go build / vitest / vue-tsc / 契约测试均未在本会话执行；本会话实跑仅限 §6 所列存在性、迁移链与文本核验（执行日 2026-09-26）。
5. **第一程 9 项仅有交付摘要**：逐项 residual 明细、逐文件清单与门禁明细未随材料提供；本文第一程各小节的"关键落点"为存在性抽查而非实现读码核验。
6. **并行泳道瞬时编译态事件**：第二程 TB-47（首跑失败→30 秒重跑自愈）、TP-22（首跑失败→重试自愈）、TB-17R（5 次重跑错误集不变→blocked）各自留有门禁事件记录（tsc-TB-47.log / tsc-TB-48.log / tsc-TB-17R-*.log）；最终一致性以编排方收尾门禁（vue-tsc=0）为准，更新会话未复核各 log 文件内容。
7. 本文逐项 residual 中标注"明确不做/按 ask 排除"的子项为任务书范围决定，非遗漏；汇总见 §5。

## 4. blocked / 回退记录

本批次实施结果中 **blocked 项 1 个**：**TB-17R（传输维度配额接线）**，按约定登记为 **已回退**：

- **原因**：vue-tsc 收尾门禁连续 3 轮未过（exit=2；5 次重跑错误集恒为 3 条，全部位于 TP-22 泳道在途编辑文件 thingsvis-preview/index.vue，与本项文件零交集——详见 §1.15）；按批次约定门禁 3 轮未过即回退。
- **回退范围（登记口径）**：本项不计入批次三 delivered 集合（两程合计 15 项中 14 项 delivered）；ROADMAP §4.1 TB-17 行 transport 维度子项仍标注为开放；其契约测试 101 不作为交付证据面。
- **实测注记**：截至更新会话（2026-09-26），本项改动文件仍存在于工作树且核心新文件为 git 未跟踪态（见 §1.15 实测注记）——"已回退"为登记状态，物理回退未执行；残留改动的处置（移除或重新立项）留编排方决定。

## 5. 本轮明确不做项汇总（留后续批次）

| 编号 | 明确不做子项 | 出处 |
| --- | --- | --- |
| TB-47 | 内置菜单改名/隐藏/按角色（tenant_dashboard_menus 扩展） | §1.10 residual① |
| TB-48 | 三套存量调度迁移为统一执行器；按事件自定义时区 | §1.11 residual①③ |
| TB-15R | TimescaleDB 按行级动态重建 retention job；操作日志行级保留（data_type=2） | §1.12 residual①② |
| TB-49 | 富样式 PDF 模板与图表嵌入（PDF Latin-1 核心字体边界） | §1.13 residual① |
| TP-22 | DataRoom 引擎收编 | §1.14 residual① |
| TB-17R | CoAP/TCP 网关路径同构接线（随本项回退一并搁置） | §1.15 residual |
| TB-11 | 真实集群安装/双副本演练、broker federation 接线、Ingress/TLS/TimescaleDB 变体 | §1.6（专项证据 §4） |
| TB-22 | LwM2M DTLS(PSK)、observe 服务端集成（本项交付未覆盖，仍开放） | §1.7 口径注记 |
| TP-20 | 国产库驱动进 go.mod 并连通验证（本项交付为方言适配层） | §1.8 residual |

另：第一程其余各项（TB-45/46/TP-03/TB-19/TB-23/TP-21）residual 明细未随材料提供，无法汇总，见 §3 第 5 条。

## 6. 文档更新会话核验记录（本会话实跑，2026-09-26）

本会话（路线图与证据更新员）为撰写本文与更新 ROADMAP.md 在仓库根 `C:/Users/Zz/Documents/projects/active/aetherlink-iot/` 下实际执行的核验：

1. **迁移链与版本号**：`grep -n "VERSION_NUMBER" backend/pkg/global/global.go` → `:21 VERSION_NUMBER = 138`；`ls backend/sql/*.sql | wc -l` = **138**；编号序列与 `seq 1 138` diff → **NO_GAPS**（1~138 连续无缺号）；`130.sql`~`138.sql` 逐一存在，`132.sql`/`133.sql` 头注释读码实测为 NO-OP 占位（批次三编号重排 2026-09-26，TB-19→135、TB-23→136）。
2. **契约测试就位**：`ls automation_tests/tests/ | grep -E "^(8[7-9]|9[0-9]|10[01])_"` → **12 个文件**（87_integration_management / 88_user_group_management / 90_tcp_gateway / 91_protobuf_converter / 92_snmpv3_pointconfig / 93_mobile_app_center / 94_whitelabel_overrides / 95_scheduler_events / 96_report_schedule_format / 99_tp22_canvas_display_mode / 100_row_level_data_policy / 101_transport_quota）；89/97/98 号位 grep **无文件**；`ls automation_tests/tests/*.test.js | wc -l` = **151**（批次二后 139 + 12），编号前缀去重 **98** 个。**仅确认文件存在，未运行任何用例**。
3. **关键落点存在性抽查**：约 60 个文件/目录逐一 `[ -e ]` 检查（backend：sql/130~138.sql、integration{service,api,router}、user_group{dal,service,api}、protocolgw/tcp/ 五件套+app/tcp_gateway、data_converter_protobuf、mobile_app_center 四层、snmp/usm+collector/snmp、dialect(+test)、healthmset 四件+tests、tenant_whitelabel 四层、scheduler_event{model,dal}+scheduler_events{service,api}+scheduler.go、data_policy{gen,http,scope,dal,api}、report_{render,smtp,schedule_http}、scadadoc、board_carousel{api,service}、quota/transport_{limit_publisher,usage_reader}、go.mod；mqtt-broker：plugin/aetherlink/transport_quota(+test)、hooks_auth；frontend：views/integration/list、management/user-group、mobile-app/app-center、scheduler/calendar、management/setting/branding-setting(+data-clear-setting)、visualization/report、visualization/thingsvis-preview/carousel.ts、scada/core/{useCanvasEditor,canvasDocument}、device/converter/index.vue、locales/whitelabel-override.ts；deploy/helm；docs/deployment-domestic-db.md）——全部存在，无 MISS（其中 3 个初次抽查 MISS 的路径经定位修正为实际实现路径：protocolgw/tcp、data_converter_protobuf.go、healthmset/mset.go）。
4. **计数口径复核**：`grep -c "^func Test"` 实测 healthmset mset_test.go **11** + align_test.go **5** = **16**（与第一程摘要"16 用例"一致）；dialect_test.go **7** 个测试函数（"101 表驱动用例"为用例数口径，未逐用例复核）。
5. **依赖与文本核验**：`grep -inE "fpdf|gomail|tdengine|kingbase" backend/go.mod` → go-pdf/fpdf v0.9.0（:16）、gomail.v2（:34）在册，tdengine/kingbase **0 命中**（TP-20 无驱动，与"方言适配层"口径一致）；`grep -n "PROTOBUF" frontend/src/views/device/converter/index.vue` → :7/:49 实测 PROTOBUF 模式与示例 schema；`grep -rln "mset|MSET" .../health-assessment/` → index.vue 实测 MSET 引用；`grep -n "authNoPriv|v3" backend/internal/snmp/usm.go` → 头注释实测 SNMPv3 USM RFC 3414、v3 认证协议 fail-closed。
6. **TB-17R 文件状态**：11 个 filesChanged 文件逐一 `[ -e ]` 全部存在；`git status --porcelain` 实测 `backend/internal/quota/transport_limit_publisher.go`、`go.work`、`mqtt-broker/plugin/aetherlink/transport_quota.go` 均为 **??（未跟踪）**——物理回退未发生（§4 实测注记）。
7. **未执行**：任何 go test / go build / vitest / vue-tsc / 契约测试 / OpenAPI 生成 / 竞品版本调研（故 ROADMAP §3 新增行的竞品版本列一律标"—（首引版本未逐版核验）"）。
