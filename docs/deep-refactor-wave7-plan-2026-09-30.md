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

### Wave7-B / C / D：未开工

## 执行纪律（承 9-28 教训）

并发≤4、禁自动重试、单 agent 长命令后台落盘轮询、每轨全绿才收、先落库再开工。
