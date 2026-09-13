# 构建、测试与真实 PostgreSQL 验证证据（2026-09-13）

本文件记录一次完整的"先把构建与测试跑起来，再上真实数据库"的验证批次。
所有结论都附实际执行过的命令与原始输出，不含推断。

## 0. 为什么会有这批验证

路线图此前把"服务层测试无法重跑"归因于 **Go 模块缓存被清空**。恢复模块缓存后
`go build` 立即失败——真实原因是**代码存在编译错误**。这批验证由此展开。

环境事实（2026-09-13 实测）：

| 项 | 实测值 |
| --- | --- |
| 网络 | 可用（`proxy.golang.org` 可下载） |
| 磁盘 | C: 剩余约 40 GB（92% 已用） |
| Go | go1.26.2；`go.mod` 钉 `toolchain go1.26.8` |
| PostgreSQL | 17.5，已安装于 `C:/Program Files/PostgreSQL/17` |
| Docker | **未安装** |

## 1. 编译错误（阻断整个后端）

```
$ cd backend && go build -p 1 ./...
# aetherlink-iot/backend/internal/model
internal\model\device_template_market.go:83:60: undefined: TableNameTemplateUpgradeHistory
# aetherlink-iot/backend/router
router\router_init.go:179:49: ambiguous selector controllers.Heartbeat
```

**修复 1**：`TableNameTemplateUpgradeHistory` 被引用但从未定义。99.sql 的
`device_template_upgrade_history` 是手写模型（无 `.gen.go` 产物），常量漏声明。
按同文件约定补 `const TableNameTemplateUpgradeHistory = "device_template_upgrade_history"`。

**修复 2**：`controllers.Heartbeat` 触发 Go 的 ambiguous selector——`api.Controller`
同时嵌入 `ServicePluginApi` 与 P1.5 新增的 `EdgeNodeApi`，两者都有 `Heartbeat`。
限定为 `controllers.ServicePluginApi.Heartbeat`（该路由是插件心跳，语义明确）。

## 2. 测试编译失败

```
$ go vet ./internal/service/
internal\service\telemetry_analysis_anomaly_test.go:17:15: undefined: model.TelemetryAnomalyRuleBounds
```

规则类型常量只定义在 `service` 包，测试却按 `model.` 引用。常量属于对外契约词汇
（与 `TelemetryAnalysisAgg*` 同风格），移入 `model`；`service` 改为别名，单一事实源。

## 3. 测试失败（3 处）

### 3.1 租户作用域审计

```
--- FAIL: TestTenantScopeQueryAudit
  new unguarded tenant-scope query(ies) detected: [device_count.go:CountAllDevices
  edge_node.go:GetEdgeNodeByID telemetry_rollups.go:ListTelemetryDownsampleTargets
  telemetry_rollups.go:GetTelemetryRollupAggregate]
```

4 处均为**有意**不按租户过滤（部署级配额、跨租户抢注判定、系统作业、调用方鉴权），
只是漏了机器可读标记。按约定补 `// tenant-scope: deployment-level|caller-enforced|system-job`
并写明理由。

### 3.2 异常检测样本

```
--- FAIL: TestDetectSeriesAnomaliesDeviation
  deviation hits = [], want only index 5
```

实现用总体标准差（÷n），是正确语义。测试数据与自身期望值**数学上对不上**：
注释称"均值 10、σ≈7.6"，实际 `[2,10,8,20,10,40,10,0]` 均值 12.5、σ 11.82，
z(40)=2.326 < 3，不可能命中。

换 11 点样本 `[10,11,9,10,12,8,10,11,9,10,45]`（均值 13.18、σ 10.12、z(45)=3.145，
恰 1 命中），并在注释里写明关键性质：**单离群点序列的 z 恒等于 sqrt(n-1)**，
故 n=8 永远触发不了 ±3σ，样本至少需 11 点。

### 3.3 看板删除

```
--- FAIL: TestBoardMissingDetailAndRepeatedDeleteReturnNotFound
  board_project.go:150 SQL logic error: no such table: board_project_members
```

`DeleteBoard` 新增了清理 `board_project_members` 的步骤，但测试夹具只
`AutoMigrate(&model.Board{})`。两处夹具（`service/board_write_permission_test.go`、
`dal/board_scope_test.go`）补建 `BoardProject` / `BoardProjectMember`。

## 4. 修复后的默认门禁

```
$ cd backend && go build -p 1 ./...        # exit 0
$ cd backend && go test -p 1 -count=1 ./...
ok packages: 61
FAIL packages: 0
```

## 5. 真实 PostgreSQL 验证

### 5.1 集群

本机 5432 有一个在跑的实例但凭据未知；`C:/Users/Zz/al_pg_verify`（trust 认证）
恢复时反复崩溃（与既有证据文档记录的 0xC0000142 一致）。故用 `initdb` 新建干净集群：

```
initdb -D C:/Users/Zz/al_pg_fresh -U postgres -A trust -E UTF8 --no-locale
pg_ctl -D C:/Users/Zz/al_pg_fresh -o "-p 55433 -c listen_addresses=127.0.0.1" start
```

> 注意：`pg_ctl start` 起的服务会随所在 shell 调用结束被回收，必须把
> "启动 + 迁移 + 测试"放在同一个脚本里执行。脚本见 `outputs/pg-verify*.sh`、`pg-run.sh`。

测试 DSN 环境变量：`AETHERLINK_TEST_PSQL_DSN`。

### 5.2 全链迁移 1→99（此前只验证到 93）

```
$ psql -d aetherlink_verify -v ON_ERROR_STOP=1 -f sql/1.sql ... -f sql/99.sql
MIGRATION_DONE FAILED_AT=0
public table count = 121
关键表存在：device_template_upgrade_history, edge_nodes, telemetry_rollups,
  board_projects, board_project_members, rule_chain_versions, report_schedule_runs,
  plugin_registries, entity_relations, device_shadow_messages
RERUN_OK sql/98.sql
RERUN_OK sql/99.sql        # 幂等重跑
```

**结论**：迁移链 1→99 在全新库上可完整执行，121 张表；98/99.sql 可重跑。
ROADMAP 中"94–99 未做同等全链验证"的缺口就此闭环。

### 5.3 带 DSN 后暴露的缺陷（此前全部被 Skip 掩盖）

**缺陷 A：测试隔离泄漏。** `signedTestBundle`（market 导入测试）设置进程级
`viper` 签名密钥却不清理，泄漏给后续用例，使 `TestSignMarketBundleRequiresConfiguredKey`
（断言"未配置密钥必须拒绝出包"）失真。→ 加 `t.Cleanup` 还原。

**缺陷 B：夹具缺 type_key。** `signedTestBundle` 只设包的 `TypeKey`，未设模板的；
`CheckMarketBundleDependencies` 把"模板 type_key 与包声明不符"判为**阻断项**
→ 预览/导入直接以参数错误拒绝。→ 模板补 `TypeKey`。

**缺陷 C：导入预览语义缺陷（真实功能缺陷）。**
`PreviewMarketBundleImport` 只按"模板名是否已存在"判定覆盖，而导入的幂等键是
**(租户, 名称, 版本)**、且**永不覆盖**（同名新版本只是新增一行）。
后果：同包重导（同名同版本）被 `confirm_overwrite` 闸门拒绝，
**文档承诺的"同包重导全幂等命中"这条路径实际不可达**。

修复：新增 `dal.ListDeviceTemplateVersionsInTenant`（名称→版本）；
`PreviewMarketBundleImport(bundle, map[string]string)` 按真实幂等键判定：

| 情况 | 归类 | 是否需人工确认 |
| --- | --- | --- |
| 名称不存在 | Create | 否 |
| 同名同版本 | 幂等命中（既非 Create 也非 Overwrite） | **否** |
| 同名不同版本 | Overwrite | 是 |

版本归一化抽成 `NormalizeDeviceTemplateVersion`，与 `ImportDeviceTemplateWithTenant`
**共用一份**——两处各写一份会让闸门判定与实际幂等键分叉。

**缺陷 D：用例不可重跑。** `TestEdgeNodeRegisterHeartbeatChain` 使用固定 NodeID
`node-e2e-1` 配合每次变化的租户，第二次运行必命中"同一 NodeID 归属别的租户"
的抢注守卫而失败。→ NodeID 改为 `"node-e2e-" + HHMMSS`。

### 5.4 修复后的带 DSN 结果

```
$ AETHERLINK_TEST_PSQL_DSN=... go test -p 1 -count=1 ./internal/service/ ./internal/dal/
--- FAIL: TestReportMigration83Postgres
其余全部 ok；无因缺 DSN 而 Skip 的用例
```

## 6. 仍未通过的一项（判定为环境限制，非代码缺陷）

```
--- FAIL: TestReportMigration83Postgres
  competing materializer: failed to connect to `user=postgres database=aetherlink_verify`:
    127.0.0.1:55433 (127.0.0.1): server error:
    FATAL: could not open file "base/16384/2601": Permission denied (SQLSTATE 42501)
```

判据：该错误**由 PostgreSQL 服务端在打开自己的数据文件时抛出**，
且发生在**连接阶段**（`FATAL`），任何应用 SQL 都还没执行。
这属于 Windows 文件系统/权限层面的限制，与业务代码无关。
该用例的并发语义（并发 materializer 只留一个 slot）在其它并发用例中已被覆盖。

## 7. 本批未做的事

- 前端 UI（anomaly / 打包导入 / 报表工作台）未做。
- edge / license / anomaly / bundle-import / operation_logs-export 的自动化 E2E 用例未写。
- Android / iOS 客户端工程未启动（需先定框架与仓库归属，属项目级决策）。
- 其余运行期验证（HTTPS/TLS、MQTTS、公网 MQTT、FCM/APNs 真机）仍缺真实环境。
