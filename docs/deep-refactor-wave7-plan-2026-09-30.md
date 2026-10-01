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
