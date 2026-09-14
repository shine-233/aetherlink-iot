# 2026-09-14 设备分组统计（ROADMAP TP-8②）证据

> 对应 ROADMAP §7.2 TP-8。执行环境：PostgreSQL 55433 + 后端 9999（`AETHERLINK_TIMESCALE_MODE=off`）。

## 0. 结论

TP-8 拆两条，**两条都已闭环**：

| 子项 | 状态 | 依据 |
| --- | --- | --- |
| ① 模拟遥测数据初始化 / 发送 | **早已可用**（此前误记为"未实现"） | `router/apps/telemetry_data.go:36-39` 注册 4 条路由；`/telemetry/datas/simulation{,/init,/send}` 3 个 path 已入 OpenAPI |
| ② 设备分组统计 | **本轮补齐** | 见下 |

## 1. 补齐前的实际状态

- `dal.GetDeviceGroupStatistics`（单分组）已存在，且已随 `GET /device/group/detail/:id` 返回。
- 但 `GET /device/group/tree` 与 `GET /device/group`（分页列表）**都不带统计**。
  前端要么逐分组再查一次（N 次往返），要么干脆不显示计数。

## 2. 本轮实现

### 2.1 批量 DAL（消除 N+1）

`internal/dal/device_groups.go` 新增 `GetDeviceGroupStatisticsBatch(groupIDs, tenantID, ownerUserID)`。

关键点：**过滤条件与单分组版逐条对齐**（`tenant_id` / owner 作用域 / `activate_flag='active'` /
告警取 `latest_device_alarms` 的 H·M·L），差别只在把 3N 次查询压成 1 条递归 CTE：

```sql
WITH RECURSIVE group_tree AS (
    SELECT g.id AS root_id, g.id AS group_id FROM groups g WHERE g.id IN (...)
    UNION ALL
    SELECT gt.root_id, child.id FROM groups child
    INNER JOIN group_tree gt ON child.parent_id = gt.group_id
),
device_scope AS (
    SELECT DISTINCT gt.root_id, rgd.device_id
    FROM group_tree gt INNER JOIN r_group_device rgd ON rgd.group_id = gt.group_id
)
SELECT ds.root_id AS group_id, COUNT(DISTINCT d.id) AS device_total, ...
FROM device_scope ds
INNER JOIN devices d ON d.id = ds.device_id
LEFT JOIN latest_device_alarms lda ON lda.device_id = d.id AND lda.tenant_id = d.tenant_id
WHERE d.tenant_id = ? AND (? = '' OR d.owner_user_id = ?) AND d.activate_flag = 'active'
GROUP BY ds.root_id
```

无设备的分组不出现在结果集里，函数**补零值**，避免调用方把"没查到"与"零台设备"混为一谈。

### 2.2 挂载点（加法变更）

- `model.DeviceGroupWithStatistics`：内嵌 `Group` + `Statistics`，把 group 字段平铺，
  既有消费方读 `name` 等字段不受影响。
- `TreeNode` 新增 `Statistics *model.DeviceGroupStatistics json:"statistics,omitempty"`。
- `GET /device/group/tree`：每个节点带 `statistics`（含子孙分组的设备）。
- `GET /device/group`：列表项带 `statistics`。

统计属**增强信息**：批量查询失败时保持原样返回（树/列表仍可用），不让一个统计查询把接口打挂。

### 2.3 OpenAPI

响应包络在 spec 中只描述为"见平台 response 中间件"，不建模 `data`，
故本次无 spec 变更；重生成后仍为 **413 paths**（未新增端点）。

## 3. 运行期证据

```
$ node automation_tests/scripts/verify-group-statistics.js
[tree] parent.statistics = {"device_total":2,"online_total":0,"offline_total":2,"alarm_total":0}
[tree] child.statistics  = {"device_total":1,"online_total":0,"offline_total":1,"alarm_total":0}
[list] parent.statistics = {"device_total":2,"online_total":0,"offline_total":2,"alarm_total":0}
[list] child.statistics  = {"device_total":1,"online_total":0,"offline_total":1,"alarm_total":0}
[list] parent 保留原有 group 字段 (name) = VS-Parent-mu1dce4z

OK: tree 与 list 均带 statistics，且父分组统计正确汇总子孙设备
```

脚本自建"父分组 + 子分组 + 各挂一台设备"，断言：
父 `device_total` ≥ 2（自身 + 子分组设备）、子 = 1、父 > 子、list 与 tree 数值一致，
并校验列表项仍保留原有 group 字段；结束时清理分组与设备。

## 4. 单元测试

`internal/dal/device_groups_statistics_test.go`：

| 用例 | 覆盖 |
| --- | --- |
| `...RollsUpDescendants` | 父分组含子孙设备；子分组只统计自己 |
| `...ZeroFillsEmptyGroups` | 无设备分组返回零值而非缺失 |
| `...ScopesTenantOwnerAndActivation` | 他租户 / 他人 owner / 未激活设备均不计入 |
| `...CountsHighMediumLowAlarms` | 只有 H/M/L 计入 `alarm_total` |
| `...EmptyInput` | 空输入返回空 map 且不报错 |
| `...MatchesSingleGroupVersion` | 批量结果与逐分组调用单分组版**逐个相等** |

```
$ go test ./internal/dal/ -run TestGetDeviceGroupStatisticsBatch
--- PASS  ×5   --- SKIP  ×1（等价性用例需 DSN）

$ AETHERLINK_TEST_PSQL_DSN="host=127.0.0.1 port=55433 user=postgres password=... dbname=aetherlink_go99 sslmode=disable" \
    go test ./internal/dal/ -run TestGetDeviceGroupStatisticsBatch
--- PASS  ×6   （等价性用例在真实 PostgreSQL 上通过）
```

全量：`go build -p 1 ./...` exit 0；`go test -p 1 ./...` exit 0（61 包全 ok）。

## 5. 踩坑记录

- **`latest_device_alarms` 在 PostgreSQL 上是视图，不是表**（`sql/35.sql` / `sql/43.sql` /
  `sql/44.sql` 定义，链到 `current_device_alarm_streams`）。不能 `INSERT`/`DELETE`，
  写测试夹具时若按表处理会报 `SQLSTATE 55000`。SQLite 夹具里它是 AutoMigrate 出来的表，
  所以"SQLite 绿"不代表"PG 也能造数据"。
- **单分组版 `GetDeviceGroupStatistics` 在 SQLite 夹具下报
  `bad parameter or other API misuse`**（驱动层限制）。因此等价性用例改为
  `AETHERLINK_TEST_PSQL_DSN` 门控（与 `scada_postgres_test.go` 同一约定），
  没有 DSN 时明确 `SKIP`，不造恒真对照。

## 6. 仍未验证

- 统计口径中的 `alarm_total` 在真实 PostgreSQL 上**未验证**（本轮 PG 用例不造告警数据，
  因为要写 `latest_device_alarms` 的底层源表）。SQLite 用例覆盖了 SQL 逻辑，但视图链的
  真实语义仍需一条带告警的 PG 用例。
- 前端尚未消费 `statistics` 字段（本轮只补后端能力）；分组页显示计数属独立前端改动。
