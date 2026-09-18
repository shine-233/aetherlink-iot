# P1.2 规则链可靠性（Rule Chain Reliability）：5 大硬核可靠性门禁全面闭环证据

> 日期：2026-09-17  
> 仓库：`aetherlink-iot`  
> 对应路线图：§5.1 P1.2 规则引擎可靠性（Rule Chain Reliability 护城河）  
> 对标能力：ThingsBoard Rule Engine 3.x/4.x DLQ/Trace/Replay 架构 & 企业级工业高可靠消息流  
> 验证层级：PostgreSQL 17.5（`111.sql` 自动迁移） + Go 服务层单元测试 + 真实活栈端到端自动化契约测试（`automation_tests/tests/59_rule_chain_reliability.test.js`）  
> 状态：**`done`**（5/5 门禁全通，契约测试 14/14 全部 PASS）

---

## 一、5 大可靠性门禁与架构设计

P1.2 规则链可靠性是全站核心护城河能力，针对工业物联网场景中规则引擎常见的“节点失败丢消息”、“网络抖动引发级联雪崩”、“线上配置变更无法回滚”、“调试链路黑盒”以及“故障回放重复产生外部副作用”五大痛点，构建了完整的防御体系：

```
+-----------------------------------------------------------------------------------+
|                            Rule Chain Reliability Matrix                          |
+-------------------+---------------------------------------------------------------+
| Gate 1: DLQ       | 失败节点终局重试耗尽后下沉至持久化死信队列 (rule_chain_dead_letters) |
| Gate 2: 可观测     | 记录节点重试次数 attempts、等待退避耗时、执行耗时 elapsed_ms 与错误摘要 |
| Gate 3: Trace     | 单执行批次全局唯一 exec_id，按时序串联 DAG 图中各节点执行状态与错误 |
| Gate 4: 版本回滚   | 草稿/发布状态机、图哈希去重、历史不可变，回滚生成新草稿，防并发多发布 |
| Gate 5: 安全回放   | 留存输入快照，回放前经 Side-Effect 闸门拦截，显式确认方可放行并打标 |
+-------------------+---------------------------------------------------------------+
```

### 1. Gate 1: 失败节点下沉死信队列（DLQ Sink）
- **持久化表设计（`111.sql`）**：
  创建 `rule_chain_dead_letters` 表，包含 `id`、`tenant_id`、`chain_id`、`exec_id`、`node_id`、`node_type`、`device_id`、`error`、`attempts`、`created_at`。
  建立 `(tenant_id, chain_id, created_at DESC)` 复合索引以支撑规则链级死信回读，建立 `(tenant_id, exec_id)` 索引以支撑单批次快速溯源。
- **异步安全 Sink（`service.InstallRuleChainDeadLetterPersistence`）**：
  死信持久化运行在独立异步 goroutine 中，并配备 `defer recover()` 保护，绝不阻塞或抛异常影响上行消息主链路。
- **查询端点**：
  - `GET /api/v1/rule-chains/:id/dead-letters`：按链查询死信；
  - `GET /api/v1/rule-chains/dead-letters`：租户级全局死信查询（支持 `exec_id` 过滤与分页）。

### 2. Gate 2: 重试次数与延迟可观测
- **策略引擎集成**：
  在节点 policy 中支持配置 `timeout_ms`、`max_attempts`、`backoff_ms`、`dead_letter` 与 `retry_safe`。
- **可观测字段落地**：
  - 死信记录回写实际重试次数 `attempts`；
  - Trace 记录精确毫秒耗时 `elapsed_ms` 与退避等待时延。

### 3. Gate 3: 同一消息 Trace 全链路串联
- **单批次标识 `exec_id`**：
  消息进入规则引擎时分配全局唯一执行 ID `execID = uuid.New()`；
- **全图节点时序追踪**：
  消息在 DAG 流转过程中，每个节点执行完毕自动调用 `recordRuleChainNodeTrace`，记录执行结果、节点类型、耗时、pass 判定与错误摘要（截断防超长）。
- **查询端点**：
  - `GET /api/v1/rule-chains/:id/executions/:execId/traces`：链级溯源单次执行链路；
  - `GET /api/v1/rule-chains/executions/:execId/traces`：租户级按 `exec_id` 溯源跨节点时序轨迹。

### 4. Gate 4: 发布版本生命周期与安全回滚
- **状态机与不可变历史**：
  - 创建版本：生成 `status = draft`，计算并校验 `graph_hash`，相同哈希幂等去重不产生空版本；
  - 发布版本：调用 `POST /rule-chains/versions/publish`，状态翻转为 `published`；
  - 只读防御：已发布版本一律禁止再次发布或修改；
  - 部分唯一索引防御（`104.sql` / PostgreSQL partial unique index）：确保同一租户同一链在数据库层**物理上至多存在一个 published 版本**，消除并发双发布脑裂；
  - 安全回滚（`POST /rule-chains/versions/rollback`）：从目标发布版本恢复图定义，但在版本表中**创建全新的草稿版本（例如 v3）**，原发布版本（v1）及其审计日志保持只读不可变，回滚新草稿带标记 `rolled_back_from = 1`。

### 5. Gate 5: 输入快照回放与副作用安全闸门
- **输入快照留存（Replay Retention）**：
  当开启 `rule_chain.replay.retention_enabled` 时，引擎按节点记录输入快照（包含 payload 与 metadata）。
- **不可逆副作用安全闸门**：
  外部动作节点（如 `action.webhook`、`action.command`、`external.mqtt`、`external.kafka` 等）属于有副作用节点。回放时若调用方未显式传入 `confirm_side_effects: true`，回放请求直接被 `CodeOpDenied` 拦截拒绝，明确提示包含副作用节点，一个节点都不跑。
- **安全放行与元数据打标**：
  显式确认后放行执行，引擎自动在消息 metadata 中注入 `_rc_replay: true` 与 `_rc_replay_of: <exec_id>`，供下游动作节点做幂等去重防重复扣费/发指令。

---

## 二、测试矩阵与活栈实跑结果

### 2.1 自动化端到端契约测试（`59_rule_chain_reliability.test.js`）

在真实活栈（PostgreSQL 17.5 + Redis + EMQX Stub Broker + AetherLink 后端 9999 端口）运行实跑，**14/14 全部用例 100% PASS**：

```text
  P1.2 Rule Chain Reliability [59_rule_chain_reliability]
    1. 版本生命周期与回滚审计 (Gate 4)
      √ 创建规则链并建立首个草稿版本 v1
      √ 相同图哈希再次创建草稿时幂等去重（不产生空版本）
      √ 发布草稿版本 v1 并拦截重复发布
      √ 创建草稿 v2 并从已发布的 v1 回滚，生成新草稿 v3
      √ 跨租户无法操作或查看他人规则链版本
    2. 死信队列（DLQ Sink）持久化与观测 (Gate 1 & 2)
      √ 触发节点终局失败，自动下沉并持久化至死信队列 (642ms)
      √ 全局租户级死信端点支持 exec_id 过滤查询
      √ 跨租户无法读取死信记录
      3. 单消息 Trace 串联与耗时可观测 (Gate 3 & 2)
        √ 按 exec_id 能够完整串联消息在 DAG 节点间的时序轨迹与耗时
        √ 跨租户查询执行批次 Trace 返回空列表
      4. 输入回放执行面与不可逆副作用闸门 (Gate 5)
        √ 能够成功读取捕获的输入快照记录 (replay records)
        √ 未确认副作用 (confirm_side_effects=false) 时，回放被安全闸门拒绝拦截
        √ 显式确认副作用 (confirm_side_effects=true) 时，回放放行执行并完成节点重跑 (461ms)
        √ 跨租户回放他人执行批次被拒绝

  14 passing (3s)
```

### 2.2 Go 内部服务单元测试（`backend/internal/service`）

```text
=== RUN   TestRuleChainDeadLetterPersistenceInstallation
--- PASS: TestRuleChainDeadLetterPersistenceInstallation (0.00s)
=== RUN   TestRuleChainReplayRecordCarriesTenant
--- PASS: TestRuleChainReplayRecordCarriesTenant (0.00s)
=== RUN   TestRuleChainReplayJSONRoundTripPreservesTime
--- PASS: TestRuleChainReplayJSONRoundTripPreservesTime (0.00s)
=== RUN   TestRuleChainReplayRequiresSourceExecutionID
--- PASS: TestRuleChainReplayRequiresSourceExecutionID (0.00s)
=== RUN   TestRuleChainReplayRejectsEmptyRecords
--- PASS: TestRuleChainReplayRejectsEmptyRecords (0.00s)
=== RUN   TestRuleChainReplayRefusesSideEffectsWithoutConfirmation
--- PASS: TestRuleChainReplayRefusesSideEffectsWithoutConfirmation (0.00s)
=== RUN   TestRuleChainReplayRunsSideEffectsAfterConfirmation
--- PASS: TestRuleChainReplayRunsSideEffectsAfterConfirmation (0.00s)
=== RUN   TestRuleChainReplayAllowsNonSideEffectNodes
--- PASS: TestRuleChainReplayAllowsNonSideEffectNodes (0.00s)
=== RUN   TestRuleChainReplayRejectsNodeTypeDrift
--- PASS: TestRuleChainReplayRejectsNodeTypeDrift (0.00s)
=== RUN   TestRuleChainReplayReportsMissingNode
--- PASS: TestRuleChainReplayReportsMissingNode (0.00s)
=== RUN   TestRuleChainReplayMarkedMetadata
--- PASS: TestRuleChainReplayMarkedMetadata (0.00s)
=== RUN   TestRuleChainReplayRecorderBypassWhenUnwired
--- PASS: TestRuleChainReplayRecorderBypassWhenUnwired (0.00s)
=== RUN   TestRuleChainReplayRecorderCapturesInput
--- PASS: TestRuleChainReplayRecorderCapturesInput (0.00s)
=== RUN   TestRuleChainPublishedVersionIsReadOnly
--- PASS: TestRuleChainPublishedVersionIsReadOnly (0.00s)
=== RUN   TestRuleChainRepublishIsRejected
--- PASS: TestRuleChainRepublishIsRejected (0.00s)
=== RUN   TestRuleChainDraftIsEditable
--- PASS: TestRuleChainDraftIsEditable (0.00s)
=== RUN   TestRuleChainRollbackCreatesNewDraftAndKeepsHistory
--- PASS: TestRuleChainRollbackCreatesNewDraftAndKeepsHistory (0.00s)
=== RUN   TestRuleChainRollbackRejectsDraftTarget
--- PASS: TestRuleChainRollbackRejectsDraftTarget (0.00s)
PASS
ok  	aetherlink-iot/backend/internal/service	1.248s
```

### 2.3 全站路由与 Casbin 权限审计

```text
INFO casbin route audit passed: 407 protected routes registered
```
数据库版本无缝升级至 `VERSION_NUMBER = 111`，6 个新规则链端点已纳入 Casbin 鉴权体系并完成角色赋权（`SYS_ADMIN`, `TENANT_ADMIN`, `TENANT_USER`）。

---

## 三、关键工程陷阱与修复实锤

1. **`graph.ChainID` 历史致命静默丢失 Bug 根除**：
   - 历史代码 `dal.ListEnabledRuleChainGraphs` 仅调用 `Pluck("graph", &graphs)`，反序列化后 `graph.ChainID` 恒为空字符串 `""`；
   - 导致死信、Trace、回放快照在落库时因 `chain_id == ""` 被静默丢弃；
   - 重构为 `dal.ListEnabledRuleChains`，提取完整模型并将 `c.ID` 回填赋给 `graph.ChainID`，彻底治愈。
2. **初始化时序解耦与原子开关**：
   - 消除 `sync.Once` 在空 viper 状态下提前固化配置的隐患；
   - 规则链 Trace 与 Replay 留存检查采用原子布尔值、viper 与环境变量多级回退，启动后随配置即时生效。
3. **MQTT 上行信封协议与 Stub 兼容**：
   - 本地开发测试环境下适配 Go json `[]byte` base64 序列化信封结构 `{"device_id": id, "values": base64(payload)}`，保障直连设备上报与模拟遥测无缝衔接。
4. **模型 JSON Tag 规范化**：
   - 为 `RuleChainReplayRecordRow` 与 `RuleChainVersionAudit` 补全小写 snake_case `json` 结构体标签，统一前后端数据契约。

---

## 四、结论

P1.2 规则链可靠性 5 条门禁已彻底闭环并具备不可辩驳的运行期证据，全站 `done` 状态正式累积至 **6 项**（P0.2, P0.5, P0.6, P1.1, P1.2, P1.6）。
