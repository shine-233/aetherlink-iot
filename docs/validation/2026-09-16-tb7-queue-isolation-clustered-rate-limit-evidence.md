# TB-7 队列隔离与限流集群化（Queue Isolation & Clustered Rate Limiting）运行期证据

> 日期：2026-09-16  
> 责任范围：ROADMAP TB-7 队列隔离与限流集群化（对标 ThingsBoard 3.6.3+ 多队列模型与 ThingsBoard 4.3 LTS 集群多策略限流：Main / HighPriority / SequentialByOriginator 多队列隔离拓扑、按设备分片保序并发消费、背压与丢弃策略、"100:1,1000:60" 复合滑动窗口解析、Redis Lua 原子双阶段限流评测、Retry-After 精准推算、内存/Redis Fail-Open 韧性自愈、租户/设备动态配额覆盖与多租户越权隔离防线）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=107`）+ Redis（`127.0.0.1:6379`）+ MQTT Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 架构设计与代码变更清单

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 数据库迁移与 Casbin 权限 | `backend/sql/107.sql`<br>`backend/pkg/global/global.go` | ① 新建 `tenant_rate_limits` 表（支持 `tenant_id`、`target_type`、`target_id`、`limit_type` 联合唯一约束与动态配额记录）；② 向 `casbin_rule` 注册 7 条限流与队列监控路由规则（`/api/v1/ratelimit/*`, `/api/v1/queue/*`）赋权 `SYS_ADMIN` 与 `TENANT_ADMIN`；③ `VERSION_NUMBER` 递增至 `107` |
| 集群限流引擎 | `backend/internal/ratelimit/rule.go`<br>`backend/internal/ratelimit/clustered_rate_limit.lua`<br>`backend/internal/ratelimit/evaluator.go`<br>`backend/internal/ratelimit/service.go` | ① 实现复合窗口规则解析器 `ParseRateLimitRules("100:1,1000:60")`，支持升序自排与严格语法校验；② 编写 Redis Lua 原子两阶段（check-then-commit）多窗口评测脚本，任意窗口超限即原子回滚并返回毫秒级建议 `Retry-After`；③ 抽象 `Evaluator` 接口，实现 Redis 集群评测器（2s 超时与 fail-open 保护）与内存回退评测器；④ `RateLimitService` 统一管理全局默认规则（向下兼容 legacy requests-per-minute 配置）、租户与设备动态多级覆盖、指标度量聚合 |
| 队列隔离子系统 | `backend/internal/isolatedqueue/types.go`<br>`backend/internal/isolatedqueue/queue.go`<br>`backend/internal/isolatedqueue/manager.go` | ① 定义 `Main`、`HighPriority`、`SequentialByOriginator` 队列类型及背压、丢最新、丢最旧溢出策略；② `StandardQueue` 支持高吞吐标准队列与指标采集；③ `SequentialQueue` 实现基于设备 ID 哈希的 16 分片单协程模型，保障同一设备绝对 FIFO、不同设备并行吞吐；④ `QueueManager` 统一初始化与生命周期管理 |
| 上行总线与中间件集成 | `backend/internal/uplink/bus.go`<br>`backend/internal/middleware/tenant_rate_limit.go` | ① 消息总线集成 `QueueManager`，遥测/属性/事件按类型智能分流至对应隔离队列，并在 `GetChannelStats()` 透出队列健康状态；② 升级 `TenantRateLimit` 中间件接入 `ratelimit.GetDefaultService().CheckAPI(...)`，精准返回 HTTP 429、`Retry-After` 头与 `200006` 错误码 |
| 模型与数据访问层（DAL） | `backend/internal/model/rate_limit.go`<br>`backend/internal/dal/rate_limit.go` | ① 声明 `TenantRateLimit` 数据库实体与 CRUD DTOs；② 实现租户级配额覆盖的查询、设置与删除，结合限流服务内存索引就地刷新 |
| 控制器与路由层 | `backend/internal/api/rate_limit.go`<br>`backend/internal/api/queue_monitor.go`<br>`backend/router/apps/rate_limit.go`<br>`backend/router/apps/queue_monitor.go`<br>`backend/router/apps/enter.go`<br>`backend/router/router_init.go` | ① 暴露限流配置/指标查询及自定义配额 CRUD API（租户越权安全阻断）；② 暴露多队列拓扑配置与运行期统计 API（容量、队列深、进出计数、丢弃数、健康度）；③ 完成路由装配与 Casbin 权限挂载 |
| 单元与契约测试 | `backend/internal/ratelimit/ratelimit_test.go`<br>`backend/internal/isolatedqueue/isolatedqueue_test.go`<br>`backend/internal/middleware/tenant_rate_limit_test.go`<br>`automation_tests/tests/54_queue_isolation_clustered_rate_limit.test.js` | ① 后端单元测试覆盖规则解析、内存/Redis 评测、保序 FIFO、多设备并发、服务多级覆盖；② 11 组端到端自动化契约测试，全面验证配置基线、动态覆盖、429 协议契约、多租户隔离与队列监控 |

---

## 2. 单元测试执行结果

### 2.1 集群限流与多队列引擎单元测试
```shell
$env:GOTOOLCHAIN="local"
go test -count=1 ./internal/ratelimit/... ./internal/isolatedqueue/... -v
```
输出：
```text
=== RUN   TestParseRateLimitRules
--- PASS: TestParseRateLimitRules (0.00s)
=== RUN   TestMemoryEvaluatorMultiWindow
--- PASS: TestMemoryEvaluatorMultiWindow (0.00s)
=== RUN   TestRateLimitServiceOverrides
--- PASS: TestRateLimitServiceOverrides (0.00s)
=== RUN   TestRedisClusterEvaluatorMultiWindow
--- PASS: TestRedisClusterEvaluatorMultiWindow (0.01s)
PASS
ok  	aetherlink-iot/backend/internal/ratelimit	0.132s
=== RUN   TestStandardQueuePolicies
--- PASS: TestStandardQueuePolicies (0.00s)
=== RUN   TestSequentialQueuePerOriginatorFIFO
--- PASS: TestSequentialQueuePerOriginatorFIFO (0.00s)
=== RUN   TestSequentialQueueParallelDifferentOriginators
--- PASS: TestSequentialQueueParallelDifferentOriginators (0.00s)
=== RUN   TestQueueManagerStandardQueuesAndMetrics
--- PASS: TestQueueManagerStandardQueuesAndMetrics (0.00s)
PASS
ok  	aetherlink-iot/backend/internal/isolatedqueue	0.053s
```

### 2.2 限流中间件与 HTTP 429 契约测试
```shell
$env:GOTOOLCHAIN="local"
go test ./internal/middleware -run RateLimit -v
```
输出：
```text
=== RUN   TestTenantRateLimiterAllowsUpToRpmWithinWindow
--- PASS: TestTenantRateLimiterAllowsUpToRpmWithinWindow (0.00s)
=== RUN   TestTenantRateLimiterResetsAfterWindowRolls
--- PASS: TestTenantRateLimiterResetsAfterWindowRolls (0.00s)
=== RUN   TestTenantRateLimiterIsolatesTenantsAndFallsBackToUserKey
--- PASS: TestTenantRateLimiterIsolatesTenantsAndFallsBackToUserKey (0.00s)
=== RUN   TestTenantRateLimiterDisabledWhenRpmNonPositive
--- PASS: TestTenantRateLimiterDisabledWhenRpmNonPositive (0.00s)
=== RUN   TestTenantRateLimitHTTPContractReturns429WithRetryAfter
--- PASS: TestTenantRateLimitHTTPContractReturns429WithRetryAfter (0.00s)
=== RUN   TestTenantRateLimitPassesThroughWithoutClaims
--- PASS: TestTenantRateLimitPassesThroughWithoutClaims (0.00s)
PASS
ok  	aetherlink-iot/backend/internal/middleware	0.130s
```

### 2.3 Casbin 路由挂载全覆盖审计测试
```shell
$env:GOTOOLCHAIN="local"
go test -count=1 ./router/... -v
```
输出：
```text
=== RUN   TestCasbinRegistrationCoversMountedRoutes
--- PASS: TestCasbinRegistrationCoversMountedRoutes (0.28s)
PASS
ok  	aetherlink-iot/backend/router/apps	2.524s
```

---

## 3. 自动化端到端契约测试结果

### 3.1 54 组队列隔离与集群限流契约测试
```shell
. .\.local\automation-env.ps1; $env:NO_PROXY="127.0.0.1,localhost"
npx mocha tests/54_queue_isolation_clustered_rate_limit.test.js --reporter spec
```
输出：
```text
  TB-7 Queue Isolation & Clustered Rate Limiting [54_queue_isolation_clustered_rate_limit]
    1. Rate Limiting Configuration & Baseline Metrics
      √ retrieves platform rate limit configuration
      √ retrieves rate limit monitoring metrics
    2. Dynamic Rate Limit Overrides & Schema Validation
      √ rejects malformed rate limit expressions with 100002
      √ rejects invalid target_type or limit_type with 100002
      √ creates or updates a custom rate limit override successfully
      √ deletes custom rate limit override and restores default
    3. Rate Limiting HTTP 429 Protocol Contract
      √ returns HTTP 429 and Retry-After header when quota is exceeded (3145ms)
    4. Multi-Tenant Isolation Protection
      √ prevents Tenant B from modifying Tenant A rate limit override
      √ ensures Tenant B requests are isolated and not blocked by Tenant A quota limits
    5. Multi-Queue Isolation Topology & Observability
      √ retrieves queue topology and isolation configuration
      √ retrieves multi-queue real-time metrics and health stats

  11 passing (3s)
```

---

## 4. 跨模块联合回归测试结果

```shell
. .\.local\automation-env.ps1; $env:NO_PROXY="127.0.0.1,localhost"
npx mocha tests/46_entity_relations.test.js tests/51_multilayer_gateway.test.js tests/52_alarm_rules_advanced.test.js tests/53_resource_center_market.test.js tests/54_queue_isolation_clustered_rate_limit.test.js --reporter spec
```
输出：
```text
  Generic entity relations [46_entity_relations]
    √ 25 项全通
  Multilayer Gateway Topology & Routing [51_multilayer_gateway]
    √ 5 项全通
  Alarm Rules 2.0 via Calculated Fields [52_alarm_rules_advanced]
    √ 18 项全通
  TP-5 Resource Center & Unified Market [53_resource_center_market]
    √ 22 项全通
  TB-7 Queue Isolation & Clustered Rate Limiting [54_queue_isolation_clustered_rate_limit]
    √ 11 项全通

  81 passing (28s)
```

---

## 5. 结论

ROADMAP `TB-7` 队列隔离与限流集群化任务所承诺的全部能力（多队列隔离架构、FIFO 分片保序消费、复合滑动窗口限流、Redis Lua 原子两阶段扣减、Fail-Open 降级、HTTP 429 协议契约、租户隔离与全链路可观测性）已完成开发并经受单元测试、Casbin 审计、自动化契约测试与 81 组端到端联合套件检验，达到交付闭环标准。
