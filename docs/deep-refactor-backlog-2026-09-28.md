# 深度重构续作决策(2026-09-28)

依据:9-27 深度重构 marathon 的 11 次 workflow 运行日志复盘 + 三份扫描报告
(`_aetherlink-opt-backup-20260927/scan-scan_{backend-architecture,database-schema,hot-path-and-broker}.json`)。
本文档记录:哪些继续做、哪些明确降级为 backlog 及其理由。

## 已落地(9-27 marathon 交付,门禁 75 包 go test / broker / tsc / vitest 4151 全绿)

- 三条 uplink 管线合并为 pipeline.go;storage 双 spool 统一为 file_spool 族;authz 包收敛访问守卫;
  Automate 无状态化+分片并发池;automation_pool;mqtt-broker hotpath_cache;safelua 程序缓存/状态池;
  automatecache 从单文件拆包;API handler_adapter 骨架;前端 13 个 god 组件拆分、useListPage.ts;
  139.sql(操作日志索引)、140.sql(六索引);SCADA 定时器泄漏、ListTenants/设备健康 N+1、
  processor NULL panic 等真修复。
- 本日复跑补修:internal/uplink/bus.go 阻塞记账竞态(blockedStart 必须先于计数,
  否则全量 -p 2 负载下 blocked_seconds_total 四舍五入为 0 导致账本测试偶发失败)。

## 本批继续做(wave5 未竟轨道,并发 2、无自动重试、每轨全绿)

优先级按收益排序,前两条最高:

1. **be-db-schema-lifecycle**(database-schema 扫描目标 2/3/4 的低价子集):
   141.sql 保留期注册表(约 20 张无界增长表,默认关闭+幂等回执短窗,批次删除走既有 cron);
   alarm_history_devices 拆表+回填+查询改 JOIN(保留 jsonb 双写过渡);
   遥测索引卫生(删 `_copy1` Navicat 残留、(parent_id, sub_device_addr) 索引);
   latest 值查询改走 telemetry_current_datas。真实 PG17 集群实测迁移链(55440 端口配方)。
2. **be-api-handler-adapter**:在 handler_adapter.go 骨架上批量迁移 627 个复制粘贴 handler,
   字节级同包络 golden 测试;流式/SSE/WS/上传保留手写。
3. **be-dal-repositories**:dal god 文件按聚合拆分,共享分页/租户谓词/排序白名单。
4. **fe-list-pages**:剩余大列表页迁 useListPage + 拆子组件(<400 行)。
5. **be-service-domains-split**:authz 迁移收尾 + 最大 service 文件按聚合拆。

其余未竟轨道(fe-service-state / fe-core-icons-types / fe-remaining-views-materials /
fe-locales-styles-assets / broker-core-modbus-deploy)在上述五条交付且门禁绿之后,按额度再排。

## 明确降级为 backlog(本批不做,理由如下)

### A. GroupApp god-singleton → 构造注入(backend-architecture 扫描目标 1)

**不做,降级。** 理由:707 处 `service.GroupApp` 引用、95 个 service 嵌入、515 文件单包 72k LOC,
扫描自评 risk=High(触碰每个 handler 与跨服务调用,同包私有助手需导出,会出现 import cycle)。
这类全仓宽触碰重构正是 9-27 配额风暴中反复失败的形态。若要做,必须按其方案走"门面保留、
一次一个域、composition root 收口"的专线,单独排期,不与其它轨道并行。

### B. 时序表原生分区(database-schema 扫描目标 1)

**不做,降级。** 理由:扫描自评 risk=High——需要建新分区父表+按天回填+换名,无法在
pg_init.go 单事务 CheckVersion 迁移链内执行(需带外迁移步与停机/双写窗口),Timescale 部署
还要独立路径。且现有行级 TTL(138.sql)与 Timescale 保留策略的交互需要产品层确认。
保留期问题先由上面第 1 条的保留期注册表(默认关闭、批次删除)缓解,分区改造等有真实
数据量压力与维护窗口再立项。同报告的低价邻接项(删 `_copy1` 冗余索引、BRIN(ts) 评估、
tenant 计数改计数器表)中,仅索引卫生随第 1 条轨道做;计数器表涉及计费口径,需产品确认,暂缓。

### C. 其余扫描目标的状态

- 反向依赖(service→initialize/mqtt)、sql_error 泄漏规范化(356 处)、DeviceContextCache 热路径:
  有价值但非本批,列入 backlog 待排期;sql_error 泄漏(把原始 DB 错误文本返回客户端)建议
  下批优先,属安全卫生。
- 迁移链改造(golang-migrate/squash baseline)、时间类型统一(jsonb/boolean/tenant FK):
  结构性大活,backlog。

## 配额教训(执行纪律)

9-27 四轮 120+ agent 启动几乎全灭于 403:本批并发 4→2、取消 3 连自动重试、单 agent 长命令
一律后台落盘轮询。先落库(本批提交 520 文件)再开工。
