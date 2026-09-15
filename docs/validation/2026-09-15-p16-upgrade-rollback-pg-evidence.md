# P1.6 升级 / 回滚运行期证据（2026-09-15）

> 对应 ROADMAP §6 第一优先级第 5 项剩余部分：**98/99.sql 在真实 PostgreSQL 复跑**；
> 同时是 P1.6（模板市场产品化）"未闭环"里最后一个未取证项（原文：升级/回滚运行期证据（98/99.sql 未复跑））。
>
> 提交基线：`main@94f7ee3`（2026-09-15）。本轮只新增两个文件：
> `automation_tests/tests/45_template_upgrade_rollback.test.js` 与本文档。
> 未改动被测代码、未改动 `ROADMAP.md`、未改动 `frontend/`。

## 0. 环境

| 项 | 值 |
| --- | --- |
| 日期 | 2026-09-15 |
| 分支 | `main` |
| PostgreSQL | `127.0.0.1:55433`，库 `aetherlink_go99`（`backend/configs/conf-localdev.yml` 的 `db.psql` 段） |
| `sys_version` 实测 | `max(version_number) = 103` |
| Backend | `127.0.0.1:9999` |
| psql CLI | **本机没有**，`automation_tests` 里也没有 pg 客户端 |

### 一个必须先说明的环境事实

接手时 **后端 9999 端口没有在监听**（`netstat -ano | grep LISTENING` 只有 `5432` / `55433` / `6379` / `1883`，
没有 `9999`），`curl 127.0.0.1:9999/health` 返回的是代理的 502。
因此本轮**自行拉起了后端**：

```
cd backend && nohup go run . -config ./configs/conf-localdev.yml > /tmp/aetherlink-backend-45.log 2>&1 &
```

约 20 秒后 `curl -s --noproxy '*' -o /dev/null -w "%{http_code}" http://127.0.0.1:9999/health` → `200`。
跑的是工作树当前代码（含 99.sql 之后的 100–103 迁移），不是某个旧构建。

> 注意：本机 `HTTP_PROXY/HTTPS_PROXY=http://127.0.0.1:3526` 会拦本地请求。
> 后端未启动时它会返回 502（`upstream connect failed`），看起来像"接口挂了"实际是环境。
> 命令行探测请加 `--noproxy '*'`。

---

## 1. 98/99.sql 在真实 PG 上的复跑

### 1.1 手法

本机没有 psql CLI，沿用上一轮 `103.sql` 的办法：写一个**一次性 Go 小程序**，复用后端已有的
`initialize.ViperInit` + `initialize.LoadDbConfig` + `initialize.PgConnect` + `initialize.ExecuteSQLFile`
对活库执行 SQL 文件，并在执行前后各打一次快照（表/索引是否存在、历史行数、`sys_version`、
相关路径的 `casbin_rule` 分组计数）。

程序落在 `backend/cmd/pg-sql-rerun/main.go`，**取证完即删，未提交**（`git status` 干净，见 §4）。
`ExecuteSQLFile` 就是后端迁移链用的同一个函数（`db.Exec(整个文件内容)`），
因此"能跑通"等价于"后端启动时跑这两个迁移文件能跑通"。

### 1.2 命令

```
cd backend
go run ./cmd/pg-sql-rerun -config ./configs/conf-localdev.yml -file ./sql/98.sql
go run ./cmd/pg-sql-rerun -config ./configs/conf-localdev.yml -file ./sql/99.sql
# 第二轮（连续复跑，验证"可重复执行"而非"恰好这次没炸"）
go run ./cmd/pg-sql-rerun -config ./configs/conf-localdev.yml -file ./sql/98.sql
go run ./cmd/pg-sql-rerun -config ./configs/conf-localdev.yml -file ./sql/99.sql
```

### 1.3 第一轮：98.sql（原始输出）

```
2026/09/15 13:42:57 viper加载conf.yml配置文件完成...
target database: postgres@127.0.0.1:55433/aetherlink_go99
2026/09/15 13:42:57 连接数据库完成...
sql file: ./sql/98.sql

===== BEFORE =====
device_template_upgrade_history table exists : true
device_template_upgrade_history_name_idx     : true
device_template_upgrade_history rows         : 6
sys_version max(version_number)              : 103
casbin_rule rows for the audited paths (14 groups):
  ptype=g2  v0=api/v1/device/template/upgrade v1=api/v1/device/template/upgrade                       v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade                       v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade                       v2=allow  count=1
  ptype=g2  v0=api/v1/device/template/upgrade/:history_id/rollback v1=api/v1/device/template/upgrade/:history_id/rollback  v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade/:history_id/rollback  v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade/:history_id/rollback  v2=allow  count=1
  ptype=g2  v0=api/v1/device/template/upgrade/history v1=api/v1/device/template/upgrade/history               v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=p   v0=TENANT_USER   v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=g2  v0=api/v1/operation_logs/export v1=api/v1/operation_logs/export                         v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/operation_logs/export                         v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/operation_logs/export                         v2=allow  count=1
  ptype=p   v0=TENANT_USER   v1=api/v1/operation_logs/export                         v2=allow  count=1

ExecuteSQLFile OK

===== AFTER =====
device_template_upgrade_history table exists : true
device_template_upgrade_history_name_idx     : true
device_template_upgrade_history rows         : 6
sys_version max(version_number)              : 103
casbin_rule rows for the audited paths (14 groups):
  ptype=g2  v0=api/v1/device/template/upgrade v1=api/v1/device/template/upgrade                       v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade                       v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade                       v2=allow  count=1
  ptype=g2  v0=api/v1/device/template/upgrade/:history_id/rollback v1=api/v1/device/template/upgrade/:history_id/rollback  v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade/:history_id/rollback  v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade/:history_id/rollback  v2=allow  count=1
  ptype=g2  v0=api/v1/device/template/upgrade/history v1=api/v1/device/template/upgrade/history               v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=p   v0=TENANT_USER   v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=g2  v0=api/v1/operation_logs/export v1=api/v1/operation_logs/export                         v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/operation_logs/export                         v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/operation_logs/export                         v2=allow  count=1
  ptype=p   v0=TENANT_USER   v1=api/v1/operation_logs/export                         v2=allow  count=1

RESULT: state UNCHANGED on this run (idempotent)
```

### 1.4 第一轮：99.sql（原始输出）

```
2026/09/15 13:43:10 viper加载conf.yml配置文件完成...
target database: postgres@127.0.0.1:55433/aetherlink_go99
2026/09/15 13:43:10 连接数据库完成...
sql file: ./sql/99.sql

===== BEFORE =====
device_template_upgrade_history table exists : true
device_template_upgrade_history_name_idx     : true
device_template_upgrade_history rows         : 6
sys_version max(version_number)              : 103
casbin_rule rows for the audited paths (14 groups):
  ptype=g2  v0=api/v1/device/template/upgrade v1=api/v1/device/template/upgrade                       v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade                       v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade                       v2=allow  count=1
  ptype=g2  v0=api/v1/device/template/upgrade/:history_id/rollback v1=api/v1/device/template/upgrade/:history_id/rollback  v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade/:history_id/rollback  v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade/:history_id/rollback  v2=allow  count=1
  ptype=g2  v0=api/v1/device/template/upgrade/history v1=api/v1/device/template/upgrade/history               v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=p   v0=TENANT_USER   v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=g2  v0=api/v1/operation_logs/export v1=api/v1/operation_logs/export                         v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/operation_logs/export                         v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/operation_logs/export                         v2=allow  count=1
  ptype=p   v0=TENANT_USER   v1=api/v1/operation_logs/export                         v2=allow  count=1

ExecuteSQLFile OK

===== AFTER =====
device_template_upgrade_history table exists : true
device_template_upgrade_history_name_idx     : true
device_template_upgrade_history rows         : 6
sys_version max(version_number)              : 103
casbin_rule rows for the audited paths (14 groups):
  ptype=g2  v0=api/v1/device/template/upgrade v1=api/v1/device/template/upgrade                       v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade                       v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade                       v2=allow  count=1
  ptype=g2  v0=api/v1/device/template/upgrade/:history_id/rollback v1=api/v1/device/template/upgrade/:history_id/rollback  v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade/:history_id/rollback  v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade/:history_id/rollback  v2=allow  count=1
  ptype=g2  v0=api/v1/device/template/upgrade/history v1=api/v1/device/template/upgrade/history               v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=p   v0=TENANT_USER   v1=api/v1/device/template/upgrade/history               v2=allow  count=1
  ptype=g2  v0=api/v1/operation_logs/export v1=api/v1/operation_logs/export                         v2=       count=1
  ptype=p   v0=SYS_ADMIN     v1=api/v1/operation_logs/export                         v2=allow  count=1
  ptype=p   v0=TENANT_ADMIN  v1=api/v1/operation_logs/export                         v2=allow  count=1
  ptype=p   v0=TENANT_USER   v1=api/v1/operation_logs/export                         v2=allow  count=1

RESULT: state UNCHANGED on this run (idempotent)
```

### 1.5 第二轮（连续复跑，两轮各执行两次）

```
$ go run ./cmd/pg-sql-rerun -config ./configs/conf-localdev.yml -file ./sql/98.sql | grep -E "RESULT|rows  |table exists|version_number"
device_template_upgrade_history table exists : true
device_template_upgrade_history rows         : 6
sys_version max(version_number)              : 103
device_template_upgrade_history table exists : true
device_template_upgrade_history rows         : 6
sys_version max(version_number)              : 103
RESULT: state UNCHANGED on this run (idempotent)
########## ROUND2 99
$ go run ./cmd/pg-sql-rerun -config ./configs/conf-localdev.yml -file ./sql/99.sql | grep -E "RESULT|rows  |table exists|version_number"
device_template_upgrade_history table exists : true
device_template_upgrade_history rows         : 6
sys_version max(version_number)              : 103
device_template_upgrade_history table exists : true
device_template_upgrade_history rows         : 6
sys_version max(version_number)              : 103
RESULT: state UNCHANGED on this run (idempotent)
```

### 1.6 结论

- **98.sql、99.sql 各执行 2 次，全部 `ExecuteSQLFile OK`，零报错。**
- before/after 完全一致：`device_template_upgrade_history` 表与 `..._name_idx` 索引存在性不变、
  历史行数不变（6 → 6，回滚点没有被清掉）、`sys_version` 不变（103）、
  14 组 `casbin_rule` 计数全部保持 `count=1`（**没有变成 2**，说明 `WHERE NOT EXISTS` 守卫生效）。
- 两个文件都只由 `CREATE TABLE IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS` /
  `INSERT ... WHERE NOT EXISTS` 构成，结构上是幂等的，运行期也确实幂等。
- `device_template_upgrade_history` 已有 6 行，说明升级/回滚链路此前在活库上已被真实调用过
  （本轮 45 组用例又新增了若干行）。

---

## 2. 升级 / 回滚运行期证据（45 组用例）

文件：`automation_tests/tests/45_template_upgrade_rollback.test.js`
风格沿用本轮新写的 `43_alarm_comment` / `44_alarm_assignment`（同一个 `expectOk` / `expectCode` 骨架，
`tenant_admin` + `tenant_admin_b` 双租户对照，夹具失败抛错不跳过）。

被测端点（`backend/router/apps/device.go:173-175`，挂在 `deviceTemplateapi` 组下，完整前缀 `/api/v1/device/template`）：

| 方法 | 路径 | 语义 |
| --- | --- | --- |
| POST | `/api/v1/device/template/upgrade` | 目标版本严格新于当前 → 导入新版本行 + 落一条历史（含旧载荷作为回滚凭据） |
| POST | `/api/v1/device/template/upgrade/:history_id/rollback` | 重放历史里的旧载荷，幂等，不删行 |
| GET | `/api/v1/device/template/upgrade/history` | 列回滚点（可带 `template_name`） |

### 2.1 命令

```
cd automation_tests
set -a && . ./.env.local && set +a
npx mocha tests/45_template_upgrade_rollback.test.js --timeout 120000 --reporter spec
```

> 凭据必须显式导出。不导出会在 `before` 里拿到空账号，报 `Field 'Email' is required`——
> 长得像接口回归，实际是环境（ROADMAP §1.2.1 流程教训二）。

### 2.2 第一次运行（原始输出）

```

  Device template upgrade / rollback [45_template_upgrade_rollback]
    √ rejects an upgrade request without a payload
    √ rejects an upgrade payload without a template name
    √ rejects upgrading a template that does not exist in the tenant
    √ rejects a target version that is not dotted numeric
    √ upgrades to a strictly newer version and records a rollback point
    √ rejects repeating the upgrade to the same target version
    √ rejects downgrading through the upgrade channel
    √ lists the upgrade history newest first, with audit fields and no previous_payload
    √ rolls back by replaying the previous payload and deletes no rows
    √ keeps the current-version pointer on the newest row after a rollback
    √ rejects rolling back an unknown history id
    √ rejects upgrading another tenant template
    √ rejects rolling back another tenant upgrade history
    √ does not leak upgrade history across tenants
    √ rejects unauthenticated access to all three upgrade endpoints


  15 passing (215ms)
```

### 2.3 第二次运行（复跑，验证不是一次性偶然）

```

  Device template upgrade / rollback [45_template_upgrade_rollback]
    √ rejects an upgrade request without a payload
    √ rejects an upgrade payload without a template name
    √ rejects upgrading a template that does not exist in the tenant
    √ rejects a target version that is not dotted numeric
    √ upgrades to a strictly newer version and records a rollback point
    √ rejects repeating the upgrade to the same target version
    √ rejects downgrading through the upgrade channel
    √ lists the upgrade history newest first, with audit fields and no previous_payload
    √ rolls back by replaying the previous payload and deletes no rows
    √ keeps the current-version pointer on the newest row after a rollback
    √ rejects rolling back an unknown history id
    √ rejects upgrading another tenant template
    √ rejects rolling back another tenant upgrade history
    √ does not leak upgrade history across tenants
    √ rejects unauthenticated access to all three upgrade endpoints


  15 passing (220ms)
```

### 2.4 用例清单与实测行为（全部为实跑观察值，不是推断）

| # | 用例 | 实测结果 |
| --- | --- | --- |
| 1 | 空 payload | `100002` `Field 'Payload' is required` |
| 2 | payload 无 name | `100002` `Field 'Name' is required` |
| 3 | 不存在的模板升级 | `100002` `template not found in tenant; import it before upgrading` |
| 4 | 版本号非点分数字（`v2`） | `100002` `target version must be dotted numeric (e.g. 1.2.0)` |
| 5 | 1.0.0 → 2.0.0 升级成功 | `200`，返回 `history_id` + `template`（`version=2.0.0`，**新行 id ≠ v1 id**，即升级是加行不是改行） |
| 6 | **重复升级到同一版本** | `100002`，`data = {error: "target version must be strictly newer than the current one; use rollback to downgrade", from_version: "2.0.0", to_version: "2.0.0"}` —— **升级通道不是幂等重放，重复调用会失败** |
| 7 | **降级走 upgrade 通道**（2.0.0 → 1.5.0） | `100002`，同一条 `error`，`from_version=2.0.0 / to_version=1.5.0` |
| 8 | 历史可列出 | `200`，**裸数组**（无 `{list}` 包装），倒序；字段 `id / tenant_id / template_name / from_version / to_version / actor_id / created_at`；**`previous_payload` 不存在**（模型 `json:"-"`） |
| 9 | **回滚重放旧载荷且不删行** | `200`，返回 `version=1.0.0` 且 **id 等于原 v1 行 id**（幂等命中，未建新行）；v1 行与 v2 行 `detail` 均仍可查；**回滚后历史行数不增加**（它不是一次反向升级） |
| 10 | **回滚不移动"当前版本"指针** | 回滚后升级到 3.0.0，历史的 `from_version` 仍是 **2.0.0**（不是 1.0.0） |
| 11 | 不存在的 history_id 回滚 | `100002` `upgrade history not found` |
| 12 | 跨租户升级（B 用 A 的模板名） | `100002` `template not found in tenant; import it before upgrading` |
| 13 | 跨租户回滚（B 用 A 的 history_id） | `100002` `upgrade history not found` |
| 14 | 跨租户查历史 | `200` `[]`——**隔离而非拒绝**；A 的回滚点一个都不漏给 B |
| 15 | 未认证访问三个端点 | 全部 `code=401`（`data.code=40100` `missing authentication (x-token or x-api-key required)`） |

### 2.5 一个必须写下来的语义（不是缺陷，但极易被误读）

**"当前版本" = `created_at` 最新的一行，版本号不参与排序**
（`backend/internal/dal/device_template_upgrade.go:18` 的注释明确了理由：点分字符串在数据库里
排序会 `1.10 < 1.2`）。由此推出两条实测行为，用例第 9/10 条把它们锁死了：

1. 回滚是**幂等重放**：旧版本行已存在时直接命中返回，**不建新行**；
2. 因此回滚**不会**把"当前版本"拨回旧版本——回滚后下一次升级的 `from_version` 仍是新版本号。

这与服务层注释"重放让版本共存、切换交给引用方"是一致的，不是 bug。
但如果后来者以为"回滚 = 版本指针回退"，就会写出错误的消费逻辑，所以用例必须固定住这个行为。
若将来引入显式的"当前版本指针"表，第 10 条期望需要同步改写（已在用例头注释里写明）。

---

## 3. 仍未验证 / 未做的部分

| 项 | 状态 | 说明 |
| --- | --- | --- |
| **全新空库跑 94–103 全链 `initialize.CheckVersion`** | 未验证 | 本轮是在 `sys_version=103` 的既有活库上复跑 98/99，不是从空库跑到 103。AGENTS.md 已声明"全新空库全链验证只做到过 93"，本轮**没有推进这个数字**。 |
| 98/99.sql 在**空库**上的首装行为 | 未验证 | 本轮只证明了"重复执行幂等"（`NOT EXISTS` 守卫生效）。首装路径（表不存在时建表）由后端多次启动间接覆盖，但没有从 `sys_version=97` 的干净快照上单独取证。 |
| 前端 UI 侧的升级/回滚交互 | 未验证 | 本轮只有 API 层证据。模板市场页的升级/回滚按钮是否接线、是否有浏览器证据，未查。 |
| `previous_payload` 超大载荷 / 损坏载荷的边界 | 未验证 | 只验证了正常往返。损坏载荷路径（`stored previous payload is corrupted`）未被触发。 |
| 并发升级同一模板（竞态） | 未验证 | 两个请求同时升级同名模板时 `GetLatestDeviceTemplateByName` 可能读到同一个旧版本，会落两条 `from_version` 相同的历史。无锁，未压测。 |
| 历史表的清理 / 保留策略 | 未验证 | `ListTemplateUpgradeHistoryInTenant` 硬编码上限 50，无归档或清理路径。 |
| P1.6 其余子项（签名密钥 CI 标配、市场登录/发布/安装链路） | 未在本轮范围 | 41 组已有"验签 → 预览 → 覆盖闸门"证据，本轮不重复。 |

### 本轮发现但**未修**的观察（均非阻塞）

1. `ROADMAP.md` 在工作树里处于 ` M ` 已修改状态（不是本轮改的，本轮明确不动它）。
2. 工作树里还有他人新增的未跟踪文件：`automation_tests/e2e/26_alarm_comment_panel.spec.js`、
   `automation_tests/scripts/diag-page-content.js`、`automation_tests/tests/46_entity_relations.test.js`、
   `automation_tests/tests/47_ota_gray_governance.test.js`。本轮均未触碰、未提交。
3. 后端是本轮自行拉起的（`go run .`），**不是长期驻留服务**。后续跑用例前需要确认 9999 在监听。

---

## 4. 变更清单

```
?? automation_tests/tests/45_template_upgrade_rollback.test.js   （新增，15 条用例）
?? docs/validation/2026-09-15-p16-upgrade-rollback-pg-evidence.md （本文档）
```

一次性取证程序 `backend/cmd/pg-sql-rerun/` 与探针 `automation_tests/probe-45.js` 已在提交前删除，
`git status` 中不存在。未 push。

---

## 5. P1.6 是否具备结案条件

**判断：P1.6 的"升级/回滚运行期证据（98/99.sql 未复跑）"这一条可以结案；
P1.6 整体仍应维持 `partial`。**

理由：

- 缺口类型 `未验证` 的两项前置工作本轮都已落到**已运行证明**：
  ① 98/99.sql 在真实 PG 上各复跑 2 次，`ExecuteSQLFile OK`、before/after 完全一致（§1）；
  ② 升级/回滚三条端点的 15 条 API 契约用例实跑两轮全绿（§2）。
- 但 `done` 按 ROADMAP §1.0 只允许"已运行证明"，而 §4 的门禁还要求**四面一致**。
  本轮只补齐了 API 面（源码 + 路由 + Casbin 登记已在库里），
  **UI 行为与浏览器 E2E 面仍然没有证据**——§3 表格第 3 行。
  因此按 §4 的口径，P1.6 不能整体翻成 `done`。
- 建议把 P1.6 状态块的"未闭环"改写为：
  `~~升级/回滚运行期证据（98/99.sql 未复跑）~~ → 已闭环（45 组 15/15 + 98/99.sql 复跑，2026-09-15）；
  剩余：升级/回滚的浏览器 E2E 与前端接线证据`。
  **本轮按硬约束未改 `ROADMAP.md`，留给 team-lead 决定。**
