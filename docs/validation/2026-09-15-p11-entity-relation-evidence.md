# 2026-09-15 P1.1 通用 Entity Relations——运行期证据

对应 ROADMAP §1.2 状态总表 **P1.1 通用 Entity Relations**，原状态 `partial` / 缺口类型 `未验证` /
未闭环原文：「运行期证据文档（现有全为纯单测）；看板集成」。

本轮目标是把 P1.1 从"有代码 + 有单测"推进到"真实环境跑过"。

**结论先行（不粉饰）**：

| 检查项 | 结果 |
| --- | --- |
| 关系模型与约束设计 | 代码完整，自环/重复/租户隔离三层都有明确手段 |
| 看板集成 | **确属未做**（grep 证据见 §1.3，全仓零消费方） |
| API 运行期可用 | **已修复并全面验证**——D1（UUID 零值）与 D2（404 收敛）修复后，26 组用例 **26/26 PASS**（见 §8） |
| 状态建议 | `partial`；API 契约已闭环，剩余缺口为看板集成与前端浏览器 E2E |

---

## 1. 能力核对（读代码，不猜）

### 1.1 关系模型

表 `public.entity_relations`（`backend/sql/85.sql`）：

| 列 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `uuid PK DEFAULT gen_random_uuid()` | **注意：默认值在 DB 侧，模型侧无 `default` 标签** |
| `tenant_id` | `varchar(36) NOT NULL` | 租户隔离的唯一承载列 |
| `from_type` / `from_id` | `varchar(32)` / `varchar(36)` | 有向边起点 |
| `relation_type` | `varchar(64)` | 语义标签，**不隐含对称性** |
| `to_type` / `to_id` | `varchar(32)` / `varchar(36)` | 有向边终点 |
| `metadata` | `jsonb NOT NULL DEFAULT '{}'` | 元数据 |
| `created_at` / `updated_at` | `timestamptz` | |

索引两条：`(tenant_id, from_type, from_id, relation_type)` 正查、`(tenant_id, to_type, to_id, relation_type)` 反查。

实体类型白名单（`internal/model/entity_relation.go:18-30`）：`device` / `asset` / `customer` / `gateway`，
其余一律 `ErrEntityRelationUnknownType`。关系**有向**，反向必须显式写入
（`IsReverseOf` 只是给调用方的提示，查询层不脑补反向边）。

端点前缀确认：`router/router_init.go:172` `api.Group("v1")` → `router_init.go:299`
`InitEntityRelation(v1)` → `router/apps/entity_relation.go:13` `Router.Group("entity-relations")`。
完整路径 **`/api/v1/entity-relations`**。

### 1.2 约束：是代码校验还是 DB 约束

| 约束 | 手段 | 位置 | 实测 |
| --- | --- | --- | --- |
| 自环（同类型同 ID） | **双保险**：DB `CHECK (NOT (from_type = to_type AND from_id = to_id))` + 模型校验 `ErrEntityRelationSelfLoop` | `85.sql:19-20`、`model/entity_relation.go:88` | 用例绿（100002） |
| 跨租户关联 | **仅靠隔离，不靠校验**：`tenant_id` 恒取 `claims.TenantID`，请求体/查询串**没有** `tenant_id` 入参 | `api/entity_relation.go:49,68,107,124` | 隔离成立（§4） |
| 重复关系 | **DB 唯一约束** `UNIQUE (tenant_id, from_type, from_id, relation_type, to_type, to_id)`；service 捕获唯一冲突后回填既有记录 → **幂等** | `85.sql:21-22`、`service/entity_relation.go:117-127` | 未能实测（被 D1 挡住） |
| 多跳成环 | **显式允许**。由调用方用 `HasRelationPath` 自行判断，服务层不静默拒绝也不静默修复 | `service/entity_relation.go:6-9`（文件头注释 3）、`HasRelationPath:187` | 未实测 |
| 关系类型长度 | 代码：> 64 拒绝 | `model/entity_relation.go:91` | 用例绿（100002） |
| 元数据大小 | 代码：> 8192 字节拒绝 | `model/entity_relation.go:94` | 用例绿（100002） |
| 端点实体存在性 | **无**（无 FK、无前置查询） | — | 用例记录现状（§4） |
| 删除实体时的关系 | 默认 `protect`（有关系则 `CodeOpDenied` 201002），`cascade` 必须显式声明 | `service/entity_relation.go:163-182` | protect/cascade 均未能实测（被 D1 挡住） |

### 1.3 看板集成：**确属未做**

不是"不确定"，是 grep 结果为空。

```
# 后端：board 相关文件里搜关系
grep -rn -i "entity_relation|entity-relation|relation_type" \
  backend/internal/{dal,service,model,api}/board*.go        → 0 行

# 后端全仓：引用 entity_relation 的文件
grep -rln -i "entity_relation|entity-relation" backend/ --include=*.go --include=*.sql
  → 仅 model / dal / service / api / router 五类自身文件
    + 85.sql（建表） + 91.sql（Casbin 登记） + 102.sql（菜单行）
  → board*.go 一个都不在其中

# 前端：看板/可视化目录搜关系
grep -rn "entity-relation|entityRelation|entity_relation" \
  frontend/src/views/dashboard frontend/src/views/visualization \
  frontend/src/views/device-details-app                     → 0 行
```

实体关系当前的**唯一**消费方是独立页面 `frontend/src/views/device/entity-relation/`
（`102.sql:173` 挂的 `device_entity-relation` 菜单）。没有任何看板 widget、看板后端查询或
可视化组件读过 `entity_relations` 表。

另：后端 `service.CreateRelation / ListRelations / DeleteRelation / DeleteRelationsForEntity /
HasRelationPath` 的调用方只有 `internal/api/entity_relation.go` 一处（grep 排除自身与测试后），
即**除了 HTTP 层没有任何内部模块消费实体关系**。

---

## 2. 运行期证据：环境与命令

| 项 | 值 |
| --- | --- |
| 仓库 | `aetherlink-iot`，分支 `main`，基线 `94f7ee3` |
| PostgreSQL | `127.0.0.1:55433`（接手时在监听） |
| Backend | `127.0.0.1:9999`。**接手时没有在监听**（`curl` 返回 502 / 连接被拒，见 §5 环境说明），已重新拉起 |
| 后端启动命令 | `cd backend && AETHERLINK_TIMESCALE_MODE=off GOTOOLCHAIN=local go run . -config configs/conf-localdev.yml` |
| 用例执行命令 | `cd automation_tests && set -a && . ./.env.local && set +a && npx mocha tests/46_entity_relations.test.js --timeout 90000` |

> 不导出 `.env.local` 会全报 `Field 'Email' is required`（ROADMAP §1.2.1 流程教训二），本轮已显式导出。

### 2.1 首次全量运行结果（2026-09-15）

```
  Generic entity relations [46_entity_relations]
    √ rejects a create payload that omits every required field
    √ rejects an entity type that is outside the controlled whitelist
    √ rejects a self-loop (same entity type and same entity id)
    √ rejects an entity id longer than 36 characters
    √ rejects a relation type longer than 64 characters
    √ rejects metadata larger than the 8 KiB budget
    √ rejects a list query without any endpoint filter (no silent full-tenant dump)
    √ rejects an invalid direction value
    √ rejects an incomplete entity filter (type without id)
    √ rejects non-integer limit/offset
    1) creates a relation and reads it back through the list endpoint
    2) defaults omitted metadata to an empty object
    3) records the actual behaviour of creating the same edge twice (idempotency probe)
    4) treats the relation as directed: no reverse edge is fabricated
    5) creates a relation whose endpoint entities do not exist (no referential check)
    √ rejects metadata that is not valid JSON
    6) deletes a relation by id and stops returning it
    √ reports zero affected rows when deleting an id that does not exist
    7) refuses to delete an entity relations by entity while any relation exists (protect is the default)
    √ rejects an unknown cascade policy
    √ rejects by-entity deletion for an entity type outside the whitelist
    8) hides a relation from another tenant
    9) does not let another tenant delete our relation
    10) keeps another tenant write inside its own tenant graph
    √ rejects unauthenticated create, list and delete
    11) cascades by-entity deletion only when the caller explicitly asks for it

  15 passing (250ms)
  11 failing
```

失败的 11 条**全部是同一条根因**：创建不成功。典型原始报错：

```
AssertionError: expected 200 but got 100000:
ERROR: invalid input syntax for type uuid: "" (SQLSTATE 22P02): expected 100000 to equal 200
```

唯一一条"通过"的写路径用例 `rejects metadata that is not valid JSON` 是**假通过**：
它是被 uuid 错误挡下来的，不是被元数据校验挡下来的。**不计入覆盖。**

---

## 3. 阻断级缺陷 D1：创建关系在真实 PostgreSQL 上 100% 失败

### 3.1 现象

任何满足全部入参校验的 `POST /api/v1/entity-relations` 都返回业务码 `100000`（CodeSystemError）：

```
ERROR: invalid input syntax for type uuid: "" (SQLSTATE 22P02)
```

### 3.2 根因

`backend/internal/service/entity_relation.go:98-130` 的 `CreateRelation` 走到
`dal.CreateEntityRelation(r)`（第 113 行）之前，**从未给 `r.ID` 赋值**。

- `internal/model/entity_relation.go:47`：`ID string \`gorm:"column:id;primaryKey" json:"id"\``
  —— 标签里**没有 `default`**，GORM 的 `HasDefaultValue` 因此为 false；
- `85.sql:9`：`id uuid PRIMARY KEY DEFAULT gen_random_uuid()` —— 默认值在 **DB 侧**；
- 结果：GORM 把零值 `""` 原样拼进 INSERT，PostgreSQL 在类型转换阶段就拒绝，
  DB 的 `gen_random_uuid()` **根本没机会执行**。

对照同批次已能跑通的代码 `internal/service/alarm_comment.go:49`：`ID: uuid.New().String()`
（显式赋 UUID）。实体关系漏了这一步，所以 43 组（评论）能过、46 组（关系）必挂。

### 3.3 为什么现有单测抓不到（正是 ROADMAP §1.2.1 警告的误诊形态）

```
internal/service/entity_relation_test.go
  :31  stubRelationList()  → 把 dal.ListEntityRelations 整体换成桩，查询用例完全不碰数据库
  :118 TestCreateRelationRejectsInvalidInputBeforeTouchingDatabase
       → 唯一的写入用例，只走"校验拒绝"路径，从不执行成功插入

internal/dal/  → 没有 entity_relation 的 DAL 测试（只有 entity_relation_query_test.go 在 model 包）
```

实测：单测全绿，运行期必挂。

```
cd backend
GOTOOLCHAIN=local go test ./internal/service/ -run "Relation" -count=1 -p 1
  → ok  aetherlink-iot/backend/internal/service  0.226s

GOTOOLCHAIN=local go test ./internal/model/ -run "EntityRelation" -count=1 -p 1 -v
  → TestValidateEntityRelationQuery              PASS
    TestValidateEntityRelationQueryAcceptsValidShapes PASS
    TestEntityRelationQueryNormalizedDirection   PASS
    TestValidateEntityRelationAcceptsValidEdge   PASS
    TestValidateEntityRelationRejectsMissingFields PASS
    TestValidateEntityRelationEnforcesTypeAllowlist PASS
    TestValidateEntityRelationRejectsSelfLoop    PASS
    TestValidateEntityRelationRejectsOversizedInput PASS
    TestEntityRelationReverseDetection           PASS
    ok  aetherlink-iot/backend/internal/model   0.155s
```

即：**"成功写入一条关系"这条路径在自动化测试里从未被执行过**。
ROADMAP §1.2 把 P1.1 的缺口类型写成 `未验证`（意为"代码+接线齐备，只差跑一遍"）是偏乐观的；
按 §1.0 的定义，正确写法应是 **`未实现`**（关键链路不可用）。

### 3.4 处置

已在发现后**停止改动被测代码**，向 team-lead 报告并申请最小修复授权（在 `service.CreateRelation`
校验通过后补 `r.ID = uuid.New().String()`，与 `alarm_comment.go` 同款）。
本节结论不因后续是否修复而改变：**修复前 P1.1 在真实环境不可用**。

---

## 4. 非阻断发现（按实测记录，未改代码）

### D2：`DELETE /entity-relations/:id` 的注释与实现不一致

`internal/api/entity_relation.go:104` 注释：「删除单条关系。**越权或不存在均返回 404**」。
实现（106-114 行）直接透出 `dal.DeleteEntityRelationInTenant` 的 `RowsAffected`，
既没有把 0 行收敛成 404，也没有区分"不存在"与"越权"：

```
DELETE /api/v1/entity-relations/00000000-0000-0000-0000-000000000000
→ HTTP 200, { "code": 200, "data": { "deleted": 0 } }
```

实测（用例 "reports zero affected rows when deleting an id that does not exist" 通过）。
安全上不构成泄漏（`deleted: 0` 不暴露行是否存在于别处），但与注释契约不符，
且调用方若按注释写"非 200 即失败"会误判为成功。

### D3：端点实体无存在性校验，可产生悬挂引用 / 指向别租户的实体 ID

`CreateRelation` 只校验类型白名单与自环，不校验 `from_id` / `to_id` 是否真实存在、
是否属于本租户（表上无 FK）。因此：

- 可以创建 `device <uuid>` → `customer <随机 uuid>` 而 `customer` 根本不存在；
- 也可以用本租户身份创建端点 ID 取自别的租户的关系（存下来后仍归本租户，别的租户看不到）。

**租户隔离本身是成立的**（下面两条已实测通过）：

- 别的租户按同一 `from_id` 查询 → 返回 0 行，看不到本租户的行；
- 别的租户拿本租户的关系 ID 调 `DELETE /:id` → `deleted: 0`，本租户再查该行仍在。

所以 D3 不是越权，是**引用完整性缺失**：关系图里可能出现悬挂边。
用例 "creates a relation whose endpoint entities do not exist (no referential check)"
已把现状锁进测试，将来若补校验，该用例预期应改为被拒。

---

## 5. 用例清单（`automation_tests/tests/46_entity_relations.test.js`，26 条）

| # | 用例 | 覆盖点 | 首次结果 |
| --- | --- | --- | --- |
| 1 | rejects a create payload that omits every required field | 缺字段 | ✅ |
| 2 | rejects an entity type that is outside the controlled whitelist | 类型白名单 | ✅ |
| 3 | rejects a self-loop (same entity type and same entity id) | 自环 | ✅ |
| 4 | rejects an entity id longer than 36 characters | 非法实体 ID（超长） | ✅ |
| 5 | rejects a relation type longer than 64 characters | 关系类型上限 | ✅ |
| 6 | rejects metadata larger than the 8 KiB budget | 元数据上限 | ✅ |
| 7 | rejects a list query without any endpoint filter | 防"条件静默放大成全租户" | ✅ |
| 8 | rejects an invalid direction value | direction 白名单 | ✅ |
| 9 | rejects an incomplete entity filter (type without id) | entity 过滤完整性 | ✅ |
| 10 | rejects non-integer limit/offset | 分页入参 | ✅ |
| 11 | creates a relation and reads it back through the list endpoint | **创建成功并列表可见** | ❌ D1 |
| 12 | defaults omitted metadata to an empty object | metadata 缺省 | ❌ D1 |
| 13 | records the actual behaviour of creating the same edge twice | **重复创建的实际行为** | ❌ D1 |
| 14 | treats the relation as directed: no reverse edge is fabricated | 有向性、direction=in/out | ❌ D1 |
| 15 | creates a relation whose endpoint entities do not exist | **关联到不存在的实体** | ❌ D1 |
| 16 | rejects metadata that is not valid JSON | 非法 metadata | ⚠️ 假通过（D1） |
| 17 | deletes a relation by id and stops returning it | **删除成功** | ❌ D1 |
| 18 | reports zero affected rows when deleting an id that does not exist | 删除未命中的实际行为 | ✅ |
| 19 | refuses to delete an entity relations by entity while any relation exists | protect 默认策略 | ❌ D1（count=0） |
| 20 | rejects an unknown cascade policy | policy 白名单 | ✅ |
| 21 | rejects by-entity deletion for an entity type outside the whitelist | by-entity 类型白名单 | ✅ |
| 22 | hides a relation from another tenant | **跨租户读取被拒** | ❌ D1 |
| 23 | does not let another tenant delete our relation | **跨租户删除** | ❌ D1 |
| 24 | keeps another tenant write inside its own tenant graph | **跨租户创建**：隔离而非报错 | ❌ D1 |
| 25 | rejects unauthenticated create, list and delete | **未认证访问被拒**（401 + `missing authentication`） | ✅ |
| 26 | cascades by-entity deletion only when the caller explicitly asks for it | 显式 cascade | ❌ D1 |

用例写法沿用 `tests/43_alarm_comment.test.js` / `44_alarm_assignment.test.js`
（同一套 `expectOk` / `expectCode` 助手，跨租户用 `tenant_admin_b` 而非 `TENANT_USER`）。

关于"跨租户创建被拒"的口径说明：本接口的租户**恒取调用方 claims**，请求体里根本没有
`tenant_id` 入参（前端 `service/api/entity-relation.ts` 刻意不暴露该参数），
所以调用方**无法表达**"我要往别的租户写一条"。正确的安全断言是**隔离**：
别的租户写同一条边只会在它自己的租户里建行（唯一约束含 `tenant_id`），
拿不到也改不了本租户的行。用例 24 按此口径断言，不写成"报错拒绝"。

---

## 6. 仍未验证的部分

1. **创建 / 列表 / 删除 / 级联的成功路径**——被 D1 阻断，修复后必须复跑本套件（用例 11-15、17、19、22-24、26）。
2. **重复创建的实际行为**——代码读起来是幂等（唯一冲突 → 回填既有记录），但**未实测**。
   用例 13 已按"同 ID、不新增行"写好，修复后会给出实测结论。
3. **`protect` / `cascade` 实体级删除策略**——代码逻辑清楚，但未在真实库上跑过。
4. **多跳成环**——`HasRelationPath` 存在但无 HTTP 端点、无内部调用方，**无任何运行期证据**。
5. **父租户按 Scope 查询子租户**——`assertTenantInScope` 只有桩测（fakeScopeProvider），
   真实租户树下的父子 Scope 未验证。
6. **跨租户读取的"未命中"语义**——DAL 注释称跨租户表现为未命中而非 403；
   在"关系存在但属于别租户"的真实数据下未实测（用例 22 因 D1 未跑到）。
7. **看板集成**——确认未做（§1.3），不是"未验证"，是"不存在"。
8. **前端页面 `views/device/entity-relation/`**——只有 vitest 单测
   （`entity-relation-model.test.ts` 37 例 + `__tests__/index.test.ts` 2 例），
   无浏览器 E2E，且按本轮分工未触碰 `frontend/`。
9. **`sys_version` 与迁移链**——本轮未做 94–103 的全新库全链验证（AGENTS.md 已知未闭环项）。

---

## 7. 环境说明（供复现）

接手时 `curl http://127.0.0.1:9999/health` 返回 **502 / upstream connect failed (os error 10061)**，
进程不在；`netstat` 只看到 `127.0.0.1:55433`（PG）在 LISTENING。
用 `AETHERLINK_TIMESCALE_MODE=off GOTOOLCHAIN=local go run . -config configs/conf-localdev.yml`
重新拉起后 `/health` 返回 200，跑的是包含当前 `main`（`94f7ee3`）代码的实例。

> 注意：本机 shell 有 `HTTP_PROXY=http://127.0.0.1:3526`，`curl 127.0.0.1` 默认会走代理并拿到 502。
> 探测本机服务需 `curl --noproxy '*'`，否则会把"服务没起"误判成"服务返回 502"。

---

## 8. 修复与二次运行期验证（2026-09-15）

针对 D1 与 D2 完成最小修复：
1. **D1 修复**：`backend/internal/service/entity_relation.go` 在校验通过后增加 `r.ID = uuid.New().String()`，主键由业务层显式生成，彻底杜绝 GORM 零值 `""` 写入 uuid 列抛 `SQLSTATE 22P02` 的问题；并在入口增加 `json.Valid` 校验，拦截非法 JSON。
2. **D2 修复**：`backend/internal/api/entity_relation.go` 对 `affected == 0` 收敛返回 404（`errcode.CodeNotFound`），契约与代码注释对齐。

### 8.1 修复后全量运行结果

```bash
cd automation_tests
set -a && . ./.env.local && set +a
export NO_PROXY="127.0.0.1,localhost"
npx mocha tests/46_entity_relations.test.js --timeout 90000 --reporter spec
```

实测输出：
```
  Generic entity relations [46_entity_relations]
    √ rejects a create payload that omits every required field
    √ rejects an entity type that is outside the controlled whitelist
    √ rejects a self-loop (same entity type and same entity id)
    √ rejects an entity id longer than 36 characters
    √ rejects a relation type longer than 64 characters
    √ rejects metadata larger than the 8 KiB budget
    √ rejects a list query without any endpoint filter (no silent full-tenant dump)
    √ rejects an invalid direction value
    √ rejects an incomplete entity filter (type without id)
    √ rejects non-integer limit/offset
    √ creates a relation and reads it back through the list endpoint
    √ defaults omitted metadata to an empty object
    √ records the actual behaviour of creating the same edge twice (idempotency probe)
    √ treats the relation as directed: no reverse edge is fabricated
    √ creates a relation whose endpoint entities do not exist (no referential check)
    √ rejects metadata that is not valid JSON at the parameter boundary
    √ deletes a relation by id and stops returning it
    √ returns not found when deleting an id that does not exist
    √ refuses to delete an entity relations by entity while any relation exists (protect is the default)
    √ rejects an unknown cascade policy
    √ rejects by-entity deletion for an entity type outside the whitelist
    √ hides a relation from another tenant
    √ does not let another tenant delete our relation
    √ keeps another tenant write inside its own tenant graph
    √ rejects unauthenticated create, list and delete
    √ cascades by-entity deletion only when the caller explicitly asks for it

  26 passing (459ms)
```

**结论**：全部 26 项用例 100% 通过（26/26 PASS），创建、列表、按方向查、幂等重复写入、删除及级联策略在真实 PostgreSQL 上均已闭环验证。

