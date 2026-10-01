# 深度重构 wave7 计划（wave5/6 之后的下一批）

依据:`docs/deep-refactor-backlog-2026-09-28.md` 未竟项 + 9-29/30 wave5/wave6 运行现场。
本文档由主会话于 2026-09-30 编写,供 wave5/wave6 交付后接续排期。

## 现行批次状态(编写时点)

- **wave5(五条"本批继续做"轨道)**:并行工作流运行中——be-db-schema-lifecycle(141/142.sql 已见雏形)、
  be-api-handler-adapter(侦察已锁定 rule_chain/role/device_config/device_templates/edge_node/
  service_access/sys_dict/customer 8 个文件)、be-dal-repositories、fe-list-pages(侦察已锁定
  edge-nodes/update-package/update-ota/report/device-manage/widget-bundles 6 页)、be-service-domains-split。
- **wave6(四条候补轨道)**:并行工作流运行中——fe-service-state、fe-core-icons-types、
  fe-locales-styles-assets、broker-core-modbus-deploy。
- **fe-remaining-views-materials**:因与 wave5 的 views/** 领地冲突延后,归入本批(见 Wave7-B)。
- 现场基线:checkpoint `5303af6`(marathon 落库)→ `9642304`(9 轨中断现场落库)。

## Wave7-A:sql_error 泄漏规范化(安全卫生,backlog 文档 :57 建议"下批优先")

- **现状**:扫描报告实测约 356 处把原始 DB 错误文本(gorm/pg driver)经 c.Error 透传到响应信封,
  泄漏表名/列名/约束名/驱动细节给客户端。
- **做法**:优先在 middleware/response 统一出口兜底——识别 `*gorm.ErrRecordNotFound` 之外的
  数据库驱动错误族 → 映射为 errcode 通用服务错误 + 完整原文只进服务端日志(request-id 关联);
  再按 api → service → dal 分批把显式 fmt.Errorf("%w"+原文) 的返回点收敛。
- **验收**:每批 golden 包络测试断言响应体不含表名/列名关键字;356 处清单按包归零。
- **领地**:backend/internal/api/**、backend/internal/middleware/response、backend/pkg/errcode——
  **必须等 wave5 的 be-api-handler-adapter 轨道落地后开工**,否则同文件冲突。

## Wave7-B:fe-remaining-views-materials(wave6 延后项)

- **内容**:可视化/SCADA 编辑器、集成、市场、计费页面的组件拆分(<400 行);
  echarts/three 实例泄漏、SCADA 定时器泄漏的剩余点位修复。
- **领地**:frontend/src/views/** 中 wave5 fe-list-pages 未触及的剩余页面。
- **前置**:wave5 fe-list-pages 集成门禁绿后开工,避免同目录互写。

## Wave7-C:GroupApp god-singleton → 构造注入(专线,单独排期,不与任何轨道并行)

- **现状**:707 处 `service.GroupApp` 引用、95 个 service 嵌入、515 文件单包 72k LOC(扫描报告 risk=High)。
- **方案**(沿用扫描报告 + 9-28 文档结论):门面保留、一次一个域、composition root 收口。
- **子阶段**:
  1. api→service 调用面冻结(新增调用一律走接口,禁止再加 GroupApp.X 直引);
  2. 按域抽接口(device/fleet/rule/alarm/report…按文件前缀分域),接口先行、实现后切;
  3. composition root(internal/app)集中构造注入,替换 global.DB/REDIS/CasbinEnforcer 的散引
     (实测非测试引用 global.DB×1070、REDIS×180、Casbin×75——收口优先级:DB 写路径 > REDIS > Casbin);
  4. 逐域迁移,每域一个独立 PR + 全量门禁;
  5. 全域完成后删除 GroupApp 门面。
- **纪律**:此线开启期间不并行其它后端轨道(import cycle 风险)。

## Wave7-D:时序表原生分区立项(需产品/运维决策,非纯工程)

- **前置条件**:真实数据量压力数据(当前 telemetry_datas 行数/增速)、可接受的停机或双写窗口、
  Timescale 部署与非 Timescale 部署的独立路径确认、与行级 TTL(138.sql)和保留期注册表(141.sql)的交互口径。
- **缓解现状**:138.sql 行级 TTL + 141.sql 保留期注册表(默认关闭+批次删除)已把无界增长压住,
  分区改造等有维护窗口再立项。

## 明确不做/待定(承 9-28 文档)

- tenant 计数器表:涉及计费口径,需产品确认;
- 迁移链 squash/golang-migrate 改造、时间类型统一(jsonb/boolean/tenant FK):结构性大活,单独排期;
- DeviceContextCache 热路径、反向依赖(service→initialize/mqtt)规范化:排在 Wave7-A 之后按额度再排。

## 执行记录

### Wave7-A：已完成（2026-10-01 00:2x，提交 `5a89602`）

- **做法**：未逐个改调用点，而是按本计划的建议在统一出口兜底——`response` 中间件是全局
  唯一出口（`router/router_init.go` 的 `router.Use(handler.Middleware())`），泄漏点只有两处：
  `handleContextError` 的 default 分支与 panic 恢复分支。
- **实现**：新增 `internal/middleware/response/sanitize.go`。类型判定优先
  （`*pgconn.PgError`、gorm 驱动级哨兵），类型判定失效时用驱动原文特征串兜底；
  `gorm.ErrRecordNotFound` 单独放行（文案固定、不含库内部细节）。命中即返回通用系统错误
  100000，原文只进服务端日志，带 request_id / method / path / route。
- **口径修正**：本计划原文写的「约 356 处」按当前 `c.Error(<裸 err>)` 形态实测为 **228 处**；
  两者都由这一个出口覆盖，无需逐点改动。
- **验证**：`go test ./... -count=1 -p 1` → **76 包全绿**；新增 8 个用例
  （PgError / 包装错误 / gorm 哨兵 / 裸 SQL 文本的脱敏、原文入日志且带 request-id、
  RecordNotFound 放行、业务错误放行、panic 两条分支）。
- **剩余**：`service` / `dal` 里显式 `fmt.Errorf("%w"+原文)` 的返回点仍可继续收敛
  （现在即使不收敛也不会再泄漏，属"纵深防御"而非必须）。

### Wave7-B：已完成（2026-10-01 07:5x–08:0x）

拆分目标（计划第 30 行列的"可视化/SCADA 编辑器、集成、市场、计费页面 <400 行"）实测结论：
**wave6 已把绝大多数页面拆完**，只余 market/browse 一个未达标。逐项落地：

- `views/market/browse/index.vue` **402 → 228 行**（提交 `cba05e22`）：导入闸门抽为
  `modules/bundle-import-modal.vue`（脚本 97 行 + 模板 76 行，状态自成一闭包）。
  保留同名 `handleImportFile` 转发入口，不破坏 `__tests__/index.test.ts` 的调用契约。
- `views/visualization/thingsvis/index.vue` **563 → 344 行**（提交 `0bac17a0`）：
  拆为 `useThingsVisProjectList` / `useThingsVisProjectForm` / `useThingsVisProjectDelete`
  三个 composable。**该拆分由并发的另一个 WorkBuddy 会话完成**（07:49–07:55），
  它因额度中断（429，22:01 重置）未提交；本次接手做的是核验 + 补 prettier + 落库。
  注意 index.vue 解构出的 `provider`/`isNativeProvider`/`onboardingDashboardQuery`/`allProjects`
  在模板里未直接用，是**刻意保留 setupState 形状**（测试通过 `wrapper.vm` 直接驱动），勿当死代码删。
- 资源泄漏：`components/common/grid/utils/responsive.ts` 的 `ResponsiveMediaQuery`
  **销毁时从不 removeListener**（只 clear 本地 Map，MediaQueryList 仍持有 handler 闭包与实例）
  → 改具名 handler + `addEventListener` + handler 引用表，提交 `b22c0c34`，补 5 例测试。
- 另发现并修复**源码有损编码乱码**一类缺陷（见下）。

泄漏扫描口径（全仓 echarts/three/setInterval/setTimeout/rAF/addEventListener/matchMedia）
得 24 处候选，逐条判定：**echarts 实例两个真实调用点均已正确 dispose**
（`hooks/chart/use-echarts.ts` 走 `onScopeDispose`、`RdiTemperatureAlarmAxis.vue` 有卸载钩子）；
**无 WebGLRenderer 实例**；其余 `setTimeout` 无 clear 的多为一次性定时器，**属良性**；
真泄漏只有 responsive.ts 一处。

#### 附带发现：源码有损编码乱码（提交 `90d6fe81`）

项目自带 `automation_tests/tests/00_source_encoding_contract.test.js` 专防"UTF-8 被按 GBK
重解释"，但**只有特征串表，漏掉带 PUA 字符的一类**，实测静默放行了 3 个文件：

| 文件 | 损坏 |
|---|---|
| `frontend/src/utils/echarts/echarts-manager.ts` | 6 行注释/日志（`初始化`→`鍒濆\ue750鍖`） |
| `frontend/src/utils/echarts/aetherlink-theme.ts` | 2 行注释（`面积图`、`柱状图`） |
| `frontend/src/hooks/use-count-up.ts` | **换行被吃掉**：3 行块注释挤成 1 行且原文被截断 |

还原方法：乱码是 UTF-8 字节被按 GBK 解码的产物，逆向须 `encode("gb18030")`（`gbk`/`cp936`
无法编码 PUA 字符）再 `decode("utf-8")`，末尾残缺字节需容忍 `unexpected end of data`。

**契约加固**：新增 **PUA 规则（U+E000–U+F8FF）**——GBK 用户自定义区双字节会映射到 PUA，
源码出现 PUA 基本只有"有损编码转换"一个来源。加规则后**当场又抓出** `use-count-up.ts`，
证明签名表确实漏这一类。

### Wave7-C：仍未开工（结论不变：必须独占）

计划第 35–46 行的结论经本次复核仍然成立，且**不建议现在开**：
707 处引用 / 95 个 service / 515 文件 72k LOC，且第 46 行明确"此线开启期间不并行其它后端轨道"。
本次实测期间确有并发会话在改 `backend/internal/service/`（它拆了 `telemetry_statistic.go`，
提交 `213caf7f`），直接印证了冲突风险。**开工前提：后端其它轨道全部停下 + 按 5 个子阶段走。**

### Wave7-D：立项材料已交付，待产品/运维决策

- 度量脚本：`deploy/maintenance/measure_telemetry_volume.sql`（只读，实测 0 错误跑完）
- 方案与决策简报：`docs/wave7-d-partitioning-spec-2026-10-01.md`
- **迁移脚本（已写并在真集群演练通过）**：`deploy/maintenance/partition_telemetry_datas.sql`
  + 回滚脚本 `rollback_partition_telemetry_datas.sql`。
  在隔离集群 PG 17.5 上对 102881 行 / 34 MB 实测：迁移 **2162 ms**、回滚 **1247 ms**，
  数据指纹三次完全一致；分区裁剪、写入路由、唯一约束传播均验证通过。
  演练中发现并修掉一个真 bug（`INCLUDING ALL` 会把 CHECK 约束复制到分区父表、进而
  传播到所有分区，导致新数据全插不进去）——**这正是不做演练直接上生产会踩的坑**。
  详见方案文档第 3.5 节。
  注意脚本用 psql 元命令，**不能进 backend/sql 自动迁移链**，必须 psql -f 带外执行。

**口径纠正**：本计划第 48 行把它记作"未开工"偏低——Timescale 路径其实已闭环
（57.sql 转 hypertable + 压缩、`timescale_mode.go` 显式开关、`timescale_retention.go`
原生 retention 含 UnixMilli 的 `set_integer_now_func` 与幂等收敛）。真实缺口是
**普通 PG 路径完全没有原生分区**。

**新发现的硬约束**（原计划只写了"交互口径"四字未展开）：138.sql 的档案级/租户级保留期与
"整分区 DROP"语义冲突——一个时间分区覆盖所有租户，按全局保留期 DROP 会误删长保留租户的数据。
故给出 A（只分区不 DROP，推荐）/ B（按最大保留期 DROP）/ C（按租户二级分区，不推荐）三案。
**关键判据**：脚本第 5 段 `scoped_policy_rows`——若为 0，直接批方案 B。

迁移设计用 `ATTACH PARTITION` 把既有表直接挂成首个分区（先加 CHECK NOT VALID 再 VALIDATE，
避免全表扫描），换名仅需毫秒级元数据锁 → 零停机、无数据搬运、回滚同样是元数据操作。

### wave5 / wave6 交付核查（2026-10-01 08:0x）

对 wave5 五条轨道逐条核对交付物，结论是**基本已完成**，不是"未收尾"：

| 轨道 | 核查证据 | 结论 |
|---|---|---|
| be-db-schema-lifecycle | `142.sql`（123 行）完整落地：`alarm_history_devices` 关联表 + 触发器同步（jsonb 列仍是写入事实源，保持兼容）+ 存量回填 + `DROP INDEX IF EXISTS telemetry_datas_ts_idx_copy1` + `idx_devices_parent_sub_addr` | ✅ |
| be-api-handler-adapter | 8 个锁定 handler 文件均在（rule_chain/role/device_config/device_templates/edge_node/service_access/sys_dict/customer） | ✅ |
| fe-list-pages | 六页实测：edge-nodes 99 / update-package 281 / update-ota 225 / report 264 / widget-bundles 191 行，均 <400 | ✅ |
| be-service-domains-split | 并发会话提交 `213caf7f`（telemetry_statistic.go 661→261/168/260） | ✅ |
| be-dal-repositories | `dal/alarm.go` 930 行/64 个顶层符号混了 4 个聚合 → 已按聚合拆为 4 文件（见下）；其余最大 ≤553 | ✅ |

门禁基线（本次实测）：后端 `go build` 通过、`go test ./... -p 1` **0 FAIL**；
前端 vitest **498 文件 / 4332 用例全绿**、`vue-tsc` 0 错误、eslint 0 错误。

#### wave6 四轨核查（2026-10-01 08:3x）

- **fe-core-icons-types**：`assets/svg-icon`(55 文件) / `icons-registry-core-buckets`(20 文件) /
  `core/data-architecture/types/enhanced`(7) / `core/interaction-system/components`(7) 均已落地。
  图标注册表做了**懒加载改造**：核心图标按首字母拆 20 个 chunk，访问 `coreIcons.Xxx` 时才拉取。
  **但发现一个可维护性缺口（已补，见下）**。
- **fe-locales-styles-assets**：`styles/scss`(4) / `styles/css`(2) / `public/rdi`(2) 已落地。
- **fe-service-state / broker-core-modbus-deploy**：本次 checkpoint 未见明显改动文件；
  broker 侧 `deploy/docker-compose.modbus.yml` 存在；仓库卫生正常
  （`mqtt-broker/gmqttd-dev.exe` 未被跟踪，`.gitignore` 已含 `*.exe`，全仓无跟踪的二进制）。

##### 补：图标清单生成器缺失（本次修复）

`icons-registry-core-manifest.ts`（1215 行）文件头写着「自动生成：请勿手工编辑」，
但**仓库里从来没有生成器**（全仓搜 `icons-registry-core-manifest` / `coreBucketLoaders`
无任何脚本命中），而且它声明的来源 `源：icons-registry-core.ts` 是**反的**——
`icons-registry-core.ts` 恰恰是 `import` 这个 manifest 的一方。后果：改图标只能手改一个
自称不该手改的文件，且 20 个分组 chunk 与清单的一致性无人保障。

修复：
- 新增 `frontend/scripts/generate-icons-registry.mjs`，把**源明确为 `icons-registry-core-buckets/`**
  （分组文件持有真实 import 与导出名单，是事实来源），据此重算清单、名字→分组映射、分组加载器
- 内置两条自检：分组名必须等于其成员首字母（否则加载器指向错误 chunk）、图标名不得重复
- 新增 `pnpm icons:gen` / `pnpm icons:check`，并把 `icons:check` 接进 `build:check` 前置
- 修正文件头的来源标注

**验证（关键）**：生成器输出与已提交的 manifest **逐字节比对，1215 行中仅差被修正的那 1 行**
（其余 1214 行完全一致）→ 证明生成器正确复现了历史产物。
另做反向验证：故意从清单里删掉一个图标，`icons:check` 立即以退出码 1 报「与分组文件不同步」。
门禁：全量 vitest 498 文件 / 4332 用例全绿、`vue-tsc` 0 错误。

**遗留观察（未处理，需你判断）**：`components/common/icons` 这个导出面在全仓
**没有任何 import 方**（初版就只有 15 行，仅被同目录 `icons.ts` 再导出，而 `icons.ts` 也无人用）。
但它的文件头明确警告「导出名称可能被路由元信息或共享组件**间接**引用，删除或改名需先完成
调用方迁移」——本项目是开源发布版，该导出可能面向下游 fork。**故本次不删**，
仅在此记录：若确认无下游消费者，整族（manifest 1215 行 + 20 桶 1284 行 + 2 个测试）可清理。

#### be-dal-repositories 收尾：`dal/alarm.go` 按聚合拆分（2026-10-01 08:2x）

`alarm.go` 930 行 / 64 个顶层符号，实际混了 4 个聚合——这是该轨道最后一块。按聚合拆为：

| 新文件 | 行数 | 内容 |
|---|---|---|
| `alarm_config.go` | 218 | 告警配置增删改查、分页与租户作用域收窄、按设备反查生效配置 |
| `alarm_info.go` | 222 | 当前告警增删改查、分页、remark(JSON) 展开与生命周期状态推导 |
| `alarm_history.go` | 576 | 告警历史分页/作用域/趋势/写入/确认重置动作/remark 合并/计数，及本文件用到的 5 个 SQL 片段常量 |
| `alarm_device_status.go` | 73 | 设备告警状态判定 + 告警名称缓存 |

**这是纯代码搬移**（同包内移动，签名与实现一字未改），验证方式：

1. **符号级核验**：对比 `git show HEAD:.../alarm.go` 与 4 个新文件的顶层名字集合
   → **64 → 64，丢失 0、新增 0、跨文件重名 0**
2. `go build ./...` 通过；`go vet ./internal/dal/...` 0 问题；`gofmt -l` 无输出
3. `go test ./internal/dal/...` 通过（该包有 31 个 alarm 相关测试函数）
4. 全量 `go test ./... -count=1 -p 1` → **0 FAIL**

**踩到的坑（值得记住）**：按行号提取函数块时，只匹配 `^func ` 会**漏掉包级 `const`/`var`**。
本例 `alarm.go` 里有 5 个 SQL 片段常量（`alarmHistoryOwnerExistsSQL` 等）服务于告警历史，
若直接删原文件就会丢。所以提取脚本必须先做**全符号盘点**（`^(func|const|var|type)`），
并在生成后做一次"名字集合对比"，确认零丢失再删原文件。

**注意**：`dal/` 目录本就已有 `alarm_history_actions.go`(251) / `alarm_history_devices.go`(177) /
`alarm_assignment.go`(50) / `alarm_comment.go`(90) / `alarm_sla.go`(118) 等拆好的文件，
`alarm.go` 是漏下来的那个大文件。拆分后**无重名冲突**（编译与符号核验均已确认）。

### Wave7-A 纵深防御收尾（2026-10-01 08:1x）

计划第 77–78 行留的"service/dal 显式 `fmt.Errorf` 返回点仍可继续收敛"已做。

**先区分两类包装**：`%w` 保留错误类型，出口 sanitizer 的主路径（`errors.As` 取
`*pgconn.PgError`）能拦住；**`%v` 会把类型抹平成字符串**，主路径失效、只剩
`databaseDetailMarkers` 特征串兜底。

实测 `service`/`dal` 共 17 处 `fmt.Errorf("...%v...", err)`，其中 **11 处包的是 DB 错误**
（`initialize.GetDeviceCacheById` ×7、`dal.GetDeviceConfigByID` ×4，分布在
`attribute_data.go` / `command_gateway_payload.go` / `telemetry_set.go`）→ 统一改 `%w`；
其余 6 处是 JSON 解析/序列化，与库无关，**保持 `%v` 不动**。

**诚实口径**：这不是修一个已发生的泄漏——实测 `%v` 摊平后的文案仍含 `ERROR: ` 与
`SQLSTATE`，会被特征串兜底拦住。改 `%w` 是把检测**从"依赖驱动文案格式的退路"恢复到
"类型判定主路径"**，属纵深防御。新增 `TestDatabaseErrorTypeSurvivesNestedWrapping`
固化两层包装下的类型判定，并显式断言 `%v` 会丢类型，防止有人改回去而无感。

## 执行纪律（承 9-28 教训）

并发≤4、禁自动重试、单 agent 长命令后台落盘轮询、每轨全绿才收、先落库再开工。

---

## marathon 33 个 target 对账（2026-10-01 09:1x）

此前各文档只覆盖了 marathon 的一部分。本次直接读回原始扫描报告
（`_aetherlink-opt-backup-20260927/scan-scan_{backend-architecture,database-schema,hot-path-and-broker}.json`，
各 11 个 target，共 **33 个**）逐条对账。

### 后端架构（11）

| # | target | 状态 |
|---|---|---|
| 1 | GroupApp god-singleton → 构造注入 | ❌ 未做（= Wave7-C，需独占后端） |
| 2 | Automate.Execute 全局互斥 | ✅ 无状态化 + 分片并发池 |
| 3 | 合并三条 uplink 管线 | ✅ pipeline.go |
| 4 | 统一存储栈 | ✅ file_spool 族 |
| 5 | 收敛 60+ `ensure*Access` | ✅ `service/access_helpers.go` + `internal/authz`（guard/rule/scope） |
| 6 | 反向依赖 service→initialize/mqtt | ❌ 未做 |
| 7 | DeviceContextCache（去每消息 Redis/DB 往返） | 🟡 已有进程内 TTL 缓存 `dal/device_config_routing.go`；本轮修掉一处遗留的永久缓存键（见下） |
| 8 | 627 个 handler 迁 adapter | 🟡 **454/653（69%）**，详见下 |
| 9 | sql_error 泄漏 356 处 | ✅ Wave7-A |
| 10 | 拆分 dal/service god 文件 | ✅ 含本轮 `dal/alarm.go` |
| 11 | 无界日志表保留 | ✅ 141.sql |

### 数据库 schema（11）

| # | target | 状态 |
|---|---|---|
| 1 | 时序表原生分区 | 🟡 脚本就绪 + 真集群演练通过（Wave7-D），待决策 |
| 2 | ~20 张只增不删表保留 | ✅ 141.sql |
| 3 | `alarm_history.alarm_device_list` 规范化 | ✅ 142.sql |
| 4 | 删冗余索引 + 补缺失索引 | ✅ 142.sql |
| 5 | 逐点/逐行 ORM insert 改集合化 | ❓ 未核实 |
| 6 | 时间表示统一（bigint-ms/timestamptz/int） | ❌ 未做（扫描自评：独立做=High） |
| 7 | 迁移链 squash + 真 migrator | ❌ 未做 |
| 8 | 统一重复存储子系统 | ✅ |
| 9 | jsonb 采纳（`json`→`jsonb`、varchar 当枚举） | ❌ 未做（1.sql 里 `users.authority` 仍是 `json`） |
| 10 | 去掉请求期全表扫描分析查询 | ✅ 走 `telemetry_current_datas` |
| 11 | op_log 同步 INSERT 移出热路径 | ❓ 未核实 |

### 热路径与 broker（11）

| # | target | 状态 |
|---|---|---|
| 1 | isolated queue 无消费者导致 uplink 卡死 | ✅ 有 `bus_isolated_queue_stall_test.go` 且 PASS |
| 2 | Automation 进程级互斥 | ✅ |
| 3 | telemetry WAL 每点 2 次 fsync | ✅ 改**组提交**（单次目录 fsync 覆盖整批） |
| 4 | 近似重复 spool 子系统 | ✅ |
| 5 | Lua 每消息重建 VM/重编译 | ✅ safelua 程序缓存 + 状态池 |
| 6 | 每消息冗余序列化 + Redis 往返 | ❓ 未核实 |
| 7 | 三份管线副本（3807 LOC） | ✅ |
| 8 | rule chain/heartbeat 每消息打 Redis | ❌ 未做 |
| 9 | broker OnMsgArrived 每发布 Redis GET | ✅ hotpath_cache |
| 10 | `uplink_storage_receipts` 无界增长 | ✅ 141.sql（幂等回执默认开启） |
| 11 | 子设备解析走未索引列 | ✅ 142.sql `idx_devices_parent_sub_addr` |

**汇总**：33 个 target 中 ✅ 已落地 **18 个**、🟡 部分/待决策 **2 个**（handler adapter、时序分区）、
❓ 未核实 **3 个**、❌ 未做 **10 个**。

---

## handler adapter 迁移：已到合理终点（2026-10-01 09:2x）

本轨道原始目标是"627 个复制粘贴 handler 迁到泛型适配器"。实测精确覆盖：

- api 目录 gin handler 总数 **653**，**已迁 454（69%）**，剩余 199
- 已迁文件 **73 个**

对剩余 199 个逐个分析阻碍原因（一个 handler 可命中多条）：

| 阻碍 | 数量 | 能否迁移 |
|---|---|---|
| 多路径参数（如 `:id` + `:board_id`） | 14 | ❌ 适配器无对应变体 |
| 流式 / SSE / WebSocket / 上传 | 11 | ❌ **设计上就排除**（需直接写 `c.Writer`） |
| 自定义绑定助手（`bindXxxURI` + `validateXxxReq` 三步顺序） | 11 | ⚠️ 强行迁移会改变绑定顺序与错误包络 |
| 自定义错误形状（`errcode.WithData/WithVars`） | 23 | ⚠️ 需逐点核对包络是否逐字节一致 |
| 操作日志副作用（`SetOperationLogSafeMetadata`） | 3 | ⚠️ 需保留副作用位置 |
| 手工构造 DTO / 需要 request context / 非变量 data 字面量 | 其余 | ✅ 可用闭包迁移，但需逐个判断 |

**关键判断：这条轨道应当在此收尾，而不是继续硬推。** 依据：

1. **剩下的恰恰是"有理由不标准"的那批**。例：`alarm_comment.go` / `alarm_assignment.go` 的注释
   **明确写明了**为什么必须走 `ShouldBindUri → ShouldBindJSON → ValidateStructLang` 三步、
   不能合并——把整结构体校验提前会把"正文必填"误报成"Field 'Content' is required"，
   让合法请求在绑定正文之前就被拒。迁到 `HandlePathBody` 会改掉这个顺序。
2. **适配器自己的文档就警告过包络差异**：`bindBodyLegacyParamError` 与
   `BindAndValidate → reportParamError` 的 JSON **逐字节不同**，两者不可混用。
   对余下 23 个自定义错误形状的 handler，强行迁移的收益（少几行样板）远小于包络漂移的风险。
3. **边际收益递减**：adapter 的价值是把"绑定→校验→取 claims→写响应"四步收敛；
   已迁的 454 个正是形态规整的那批。剩下的是特例，收敛空间本就有限。

**结论**：把这条轨道标记为 **"已达成合理覆盖（69%），余下为特例，不建议继续机械推进"**。
若将来确实要覆盖多路径参数场景，正确做法是**给适配器补一个双路径参数变体**
（如 `HandleTwoPaths`），而不是把 14 个 handler 各自手写。

本轮顺带完成的两处干净迁移（`device_topic_mapping.go` 5 个 + `open_api_keys.go` 4 个）：
两者都是标准 `BindAndValidate → claims → c.Error → c.Set("data", data)` 形态，
`go build` 通过、`internal/api` 包测试通过、全量 `go test ./...` 0 FAIL。

---

## 遗留永久缓存键修复（2026-10-01 09:4x）

排查 arch#7 / hot#8（"去掉热路径上每消息的 Redis/DB 往返"）时发现一个**遗留的真隐患**：

**`dal/device_config.go` 的 `GetDeviceConfigByID` 把设备配置写进 Redis 时用的是永久键（TTL=0）**，
是**全后端唯一**还在用永久缓存的地方（其余全部有 TTL，逐一核对过）。

- 设备缓存 `GetDeviceCacheById`、脚本缓存、`processor/cache.go` 早在
  **P2 修复（2026-08-25）** 就改用了 `constant.CacheFallbackTTL`（30 分钟）兜底，
  当时的理由写得很清楚：*"兜底 TTL 替代永久缓存——写路径主动失效仍是主机制，
  兜底过期确保任何遗漏失效的写路径最终自愈，不再产生永久脏读。"*
- **唯独设备配置缓存漏掉了这一步。** 只要有任意一条写路径漏调
  `initialize.DelDeviceConfigCache`（现有 4 处调用点），脏数据就会**永久**留在 Redis 里。
- 而且 `dal/device_config_routing.go` 的注释里已经记录过这个坑的副作用：
  默认规则链缓存会"把旧的默认规则链重新写进本缓存"，说明这个永久键确实造成过问题。

**修复**：`0` → `constant.CacheFallbackTTL`，并更新注释说明口径对齐。

**验证**：新增 `device_config_cache_ttl_test.go` 两例——
① 回填后缓存键必须带 TTL 且不超过兜底上限（防退回永久键）；
② 回填后确实走缓存命中（改库不改缓存时返回旧值，证明 TTL 改动没把缓存写坏）。
`go build` / `go vet` 0 问题；全量 `go test ./... -count=1 -p 1` → **0 FAIL**。
