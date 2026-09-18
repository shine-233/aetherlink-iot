# P0.3 OTA 灰度治理：能力核对与缺口重判（2026-09-15）

> 基线：`main`，工作树见文末「工作树状态」。
> 目的：回答 ROADMAP §1.2 里 P0.3「灰度治理无运行期证据文档」到底是**"写好了但没跑过"**还是**"压根没有可运行链路"**。
> 结论：**C — 未实现**（不是"零代码"，而是"关键链路未接通 / 在真实 PostgreSQL 上跑不通"）。详见 §1。
> 约束：本轮**未执行** `go build ./...` / `go test ./...`（本机内存紧张且另有 worker 在跑 Go 构建），
> 全部结论来自读代码 + 真实 API 运行期取证（本地 backend + PostgreSQL）。

---

## 1. 判定结论

**C — 未实现。**

一句话：**灰度治理有"大脑"（纯规划器）也有"嘴巴"（两个 HTTP 端点），但没有"手"——执行面在真实库上必挂，且它要治理的那批设备根本不经过它。**

三条互相独立的硬事实支撑这个判定（每条都有 §3 的原始输出或 §2 的文件行号）：

| # | 事实 | 性质 |
| --- | --- | --- |
| F1 | `POST /api/v1/ota/task/:id/governance-apply` 只要落到写分支就在真实 PostgreSQL 上报 `SQLSTATE 42703`（`ota_upgrade_tasks` 无 `updated_at` 列），**四个写分支 `dispatch_batch`/`abort`/`timeout`/`complete` 无一可用** | 执行面不存在 |
| F2 | 任务创建即全推：`CreateOTAUpgradeTask` 建任务后立刻 `go pushOTAUpgradeTaskDetails`，明细行只可能落到 `pushed(2)` 或 `failed(5)`，**没有任何路径让明细行停留在 `pending(1)`**；而 `dispatch_batch` 只认 pending 行 | 分批放量的触发条件不成立 |
| F3 | 治理参数（`rollout_rate_per_minute` / `abort_failure_rate_percent` / `scheduled_at` / `timeout_at` / `timeout_seconds`）**在 HTTP 面上没有任何入参**，`CreateOTAUpgradeTaskReq` 与 `dal.CreateOTAUpgradeTaskWithDetail` 都不写这几列，建出来的 task 恒为 DB 默认值（实测 `rate=60 / abort=NULL / timeout_at=NULL / scheduled_at=NULL`） | 配置面未接线 |

配套结论（不是主判据，但影响"四面一致"）：

- **暂停 / 继续：OTA rollout 侧零代码。** ROADMAP P0.3 里写的 `PauseFleetCommandJob` / `ResumeFleetCommandJob` 属于**另一套子系统**（`command_jobs`，`internal/service/fleet_command_job_state_machine.go:71/112`，经 `internal/api/command_set_log.go:396/426` 暴露），与 `ota_upgrade_tasks` 无引用关系。
- **百分比放量 / 灰度观察窗 / 灰度转全量：零代码。** 只有"每分钟 N 台"的固定速率窗口，没有百分比、没有 soak/观察窗、没有 canary→full 的 promote。
- **前端 UI：零接线。** `frontend/src` 里 `governance` 的命中全部属于 command-center（fleet command jobs），没有一处指向 OTA 治理端点。
- **OpenAPI：未收录，且 swagger 注释里的路径与实际路由不一致。** `docs/swagger.json` / `docs/docs.go` 只有 `/api/v1/ota/task/detail` 与 `/api/v1/ota/task/{id}`；`internal/api/ota.go:228` 注释写 `/ota/task/{id}/rollout-governance`、`:250` 注释写 `/ota/task/{id}/rollout-governance-apply`，而实际路由是 `:id/governance-preview`（`router/apps/ota.go:43`）与 `:id/governance-apply`（`router/apps/ota.go:47`）。

> ⚠️ 反过来也要说清，避免过度纠偏：**纯规划器本身是真的、也是好的**。
> `PlanOTARolloutGovernance` 的决策优先级、失败率定义、限速窗口语义都有明确设计并有单测
> （`ota_rollout_governance_test.go`、`ota_rollout_governance_apply_test.go`、`ota_rollout_governance_preview_test.go`）。
> 本判定**不是**"这些代码要删掉重写"，而是"它没有可运行链路，不能记为已实现"。

---

## 2. 逐项能力核对（文件路径 + 行号）

### 2.1 存在的部分

| 能力 | 位置 | 说明 |
| --- | --- | --- |
| 治理动作常量（7 个） | `backend/internal/service/ota_rollout_governance.go:21-29` | `wait_schedule` / `dispatch_batch` / `hold_rate_window` / `abort` / `timeout` / `complete` / `hold` |
| 纯规划器 | `backend/internal/service/ota_rollout_governance.go:36-130` | 决策优先级：终态/取消 `:47-57` → 超时 `:60-68` → 失败率中止 `:71-79` → 未到计划时间 `:82-87` → 无待下发 `:90-103` → 限速窗口 `:106-117` → 分批下发 `:119-129` |
| 失败率定义 | `backend/internal/service/ota_rollout_governance.go:133-138` | 分母为已进入终态的设备数（succeeded+failed） |
| 只读预览服务 | `backend/internal/service/ota_rollout_governance_preview.go:24-38` | 读 task 行 + detail 分状态计数 → 喂规划器，不下发不改行 |
| 状态映射 | `backend/internal/service/ota_rollout_governance_preview.go:43-63` | `upgrading = pushed(2)+upgrading(3)`；`canceled(6)` 不计入 |
| 执行面服务 | `backend/internal/service/ota_rollout_governance_apply.go:57-89` | 读状态 → 规划 → 按分支执行；`is_simulation` 翻 false `:73` |
| 分批下发分支 | `backend/internal/service/ota_rollout_governance_apply.go:92-138` | 先记账再下发 `:118-131`；服务层自夹批次上限 `:106-108` |
| 失败率中止分支 | `backend/internal/service/ota_rollout_governance_apply.go:142-165` | 置 `canceled` + 取消剩余 pending 明细行 |
| 超时收尾分支 | `backend/internal/service/ota_rollout_governance_apply.go:168-191` | 同上，理由为 timeout |
| 完成收尾分支 | `backend/internal/service/ota_rollout_governance_apply.go:194-210` | 只改 task 行，不碰明细行 |
| DAL：领取待下发批次 | `backend/internal/dal/ota_rollout_governance.go:25-38` | 按主键排序 → **金丝雀选取确定性**；上限 `OTARolloutPendingClaimLimit=200` `:20` |
| DAL：取消剩余 pending | `backend/internal/dal/ota_rollout_governance.go:86-100` | 条件更新 `status='pending'` → `canceled` |
| DAL：task 治理写入 | `backend/internal/dal/ota_rollout_governance.go:59-81` | **`:61` 无条件写 `updated_at` ← F1 的根因** |
| 模型列 | `backend/internal/model/ota_upgrade_tasks.gen.go:28-34` | `timeout_at` / `rollout_rate_per_minute` / `abort_failure_rate_percent` / `rate_window_started_at` / `rate_window_dispatched` |
| 治理入参与决策结构 | `backend/internal/model/ota_upgrade_tasks.http.go:126-157` | 含 `IsSimulation` 字段 |
| 迁移 | `backend/sql/38.sql:2-13`（加列）、`:31-51`（CHECK：rate 1..300、abort 0<x<=100、dispatched>=0）、`:58-72`（两个 rollout 索引） | 状态词表含 `scheduled/running/completed/partially_failed/failed/canceled/aborted/timed_out` |
| HTTP 端点 | `backend/router/apps/ota.go:43`（preview）、`:47`（apply） | |
| API 处理器 | `backend/internal/api/ota.go:229-238`（preview）、`:251-260`（apply） | |
| Casbin 登记 | `backend/sql/63.sql:213,511`（governance-preview）、`backend/sql/91.sql:19,35`（governance-apply） | 权限面已接线，实测跨租户被拒（§3） |

### 2.2 不存在 / 未接线的部分

| 能力 | 核对结论 | 证据 |
| --- | --- | --- |
| **执行面写库** | **不存在** | `dal/ota_rollout_governance.go:61` 写 `updated_at`；`sql/1.sql:445-459` 建表只有 `created_at`，38.sql 未补；`model/ota_upgrade_tasks.gen.go:15` 无 `UpdatedAt`。实测 42703（§3） |
| **分批放量的触发条件** | **不成立** | `service/ota_task.go:38` 建任务后 `go pushOTAUpgradeTaskDetails`；`service/ota_task_publish.go:161-179` 先把 pending 认领成 `pushed(2)`；离线设备在认领前就被 `ensureOTAUpgradeTaskPushable`（`:330-337`）拦下并落 `failed(5)`（`:203-215`, `:349-370`）。实测创建后 pending=0（§3） |
| **治理参数配置面** | **未接线** | `model/ota_upgrade_tasks.http.go:5-21` 的 `CreateOTAUpgradeTaskReq` 无相关字段；`dal/ota_upgrade_tasks.go:41-56` 只写 name/package/target/created_by，不写治理列。实测建任务时传 `rollout_rate_per_minute=7 / abort_failure_rate_percent=1 / timeout_seconds=120 / scheduled_at=+1h` 全部被忽略（§3 用例 1） |
| **暂停 / 继续** | **未实现（OTA 侧零代码）** | 在 `internal/service/ota*.go`、`internal/dal/ota*.go`、`internal/api/ota.go` 中检索 `pause|paused|resume` 结果为 0。ROADMAP 提到的 Pause/Resume 属于 `command_jobs`：`internal/service/fleet_command_job_state_machine.go:71,112` |
| **百分比放量** | **未实现** | 全仓 OTA 侧只有 `rollout_rate_per_minute`（台/分钟），无 percent / canary_size 概念 |
| **灰度观察窗（soak）** | **未实现** | 只有 1 分钟限速窗口与绝对截止时间 `timeout_at`；无"观察 N 分钟后再放量"的语义 |
| **灰度转全量（promote）** | **未实现** | 检索 `promote|full_rollout|rollout_mode` 在 OTA 相关文件 0 命中；`complete` 只是"没有待下发且没有升级中"时的收尾，不是 promote |
| **前端 UI** | **未接线** | `frontend/src` 中 `governance` 命中全部属于 command-center（`device-command-jobs-api.ts`、`commandCenterJob*.ts/vue`），无 OTA 治理端点引用 |
| **OpenAPI** | **未接线 + 注释路径错** | `docs/swagger.json` / `docs/docs.go` 只有 `/api/v1/ota/task/detail`、`/api/v1/ota/task/{id}`；`internal/api/ota.go:228` 与 `:250` 的 `@Router` 写的是 `/rollout-governance` / `/rollout-governance-apply`，与实际路由 `governance-preview` / `governance-apply` 不一致 |
| **自动驱动（cron / worker）** | **未接线** | `initialize/croninit/cron.go`、`internal/app/cron_service.go`、`main.go` 中检索 `ota` 0 命中；`ApplyRolloutGovernance` 全仓只有 `internal/api/ota.go:254` 一个调用方 |

### 2.3 已查过但确认无关 / 易混淆的路径

| 路径 | 说明 |
| --- | --- |
| `internal/service/fleet_command_job_*.go`（约 30 个文件） | 这是 `command_jobs` 子系统的批次治理（暂停/恢复/回滚/报告），`JobType` 恒为 `"command"`（`fleet_command_job_persistence.go:33`）。**它不是 OTA**，与 `ota_upgrade_tasks` 无引用关系。ROADMAP P0.3 的"暂停/恢复/回滚/报告"四项都来自这里，容易与 OTA 灰度治理混为一谈 |
| `internal/dal/ota_upgrade_tasks.go:33-92` | OTA 任务创建，确认不写治理列 |
| `internal/service/ota_task_status.go` | 只做**明细行**粒度的取消/重试（`UpdateOTAUpgradeTaskStatus`），不是批次治理 |
| `automation_tests/tests/32_ota_runtime.test.js` | 见 §4，与灰度治理无重叠 |
| `sql/1.sql`、`21/23/36/38/42/63/81/91/94/102.sql` | OTA 相关迁移全量核过；治理列只在 38.sql，Casbin 只在 63/91.sql |

---

## 3. 运行期证据（真实 backend + PostgreSQL）

### 3.1 命令

```bash
cd automation_tests
set -a && . ./.env.local && set +a
export NO_PROXY="127.0.0.1,localhost"     # 本机有 HTTP_PROXY，不绕过会拿到 502
npx mocha tests/47_ota_gray_governance.test.js --timeout 180000 --reporter spec
```

> 注：`NO_PROXY` 是本机环境特有的额外一步——仓库 `AGENTS.md` 只提醒了 `.env.local` 的导出，
> 但本机 `HTTP_PROXY=http://127.0.0.1:3526` 会把 `127.0.0.1:9999` 也代理掉，
> 表现为 `登录请求失败 [502]: upstream connect failed`。建议把这条一并写进 §1.2.1 的流程教训。

### 3.2 原始输出

```

  OTA rollout gray governance [47_ota_gray_governance]
    √ creates the rollout with database-default governance settings because the create API has no governance fields
    √ leaves no pending detail rows after creation, which is why rate-limited dispatch is unreachable over HTTP
    √ previews without mutating the task row or the detail rows
    √ closes out a fully failed rollout instead of aborting it when no failure-rate threshold is configured
    √ fails the apply execution plane on a real database because ota_upgrade_tasks has no updated_at column
    √ exposes no updated_at column on the task read model, which is what the apply write path trips on
    √ rejects previewing a rollout from another tenant (53ms)
    √ rejects applying a rollout from another tenant
    √ rejects preview and apply for a task that does not exist


  9 passing (293ms)
```

### 3.3 关键报文（探针原始打印）

```
DETAILS  [{"s":5,"sd":"DEVICE_OFFLINE"}]

PREVIEW  {"code":200,"message":"操作成功","data":{
           "action":"complete","batch_size":0,"remaining_invalid":0,
           "failure_rate":100,
           "reason":"没有待下发设备,且没有升级中的设备",
           "warnings":["1 台设备升级失败,建议生成支持包复盘"],
           "next_steps":[],"is_simulation":true}}

APPLY_SELF            100000  ERROR: column "updated_at" of relation "ota_upgrade_tasks" does not exist (SQLSTATE 42703)
TASK_ROW_AFTER        {"status":"running","rate":60,"abort":null,"timeout_at":null,
                       "scheduled_at":null,"rwd":0,
                       "keys":"abort_failure_rate_percent,completed_at,created_at,created_by,created_by_authority,
                               description,device_count,id,name,next_dispatch_at,ota_upgrade_package_id,
                               preview_total,rate_window_dispatched,rate_window_started_at,remark,
                               rollout_rate_per_minute,scheduled_at,selected_count,started_at,status,
                               status_description,target_filter,target_mode,timeout_at,timeout_seconds"}
APPLY_OTHER_TENANT    201001  no permission to access ota package
PREVIEW_OTHER_TENANT  201001  no permission to access ota package
APPLY_MISSING         100000  record not found
PREVIEW_MISSING       100000  record not found
```

三条读法：

1. `TASK_ROW_AFTER.keys` 里**没有 `updated_at`**——这是"列不存在"最直接的物证，与 `sql/1.sql:445-459` 一致。
2. `failure_rate=100` 仍然走 `complete` 而不是 `abort`——因为 `abort_failure_rate_percent` 是 NULL，而 HTTP 面无法设置它（F3）。
3. `status` 在失败的 apply 之后仍是 `running`、`rate_window_dispatched` 仍是 `0`——执行面**没有留下任何状态**。

### 3.4 用例清单与性质

| 用例 | 性质 |
| --- | --- |
| 建任务时传治理参数被忽略，落库为 DB 默认值 | **缺陷见证**（F3）。修好配置面后此用例必须失败并改写 |
| 创建后 pending 明细行为 0，`dispatch_batch` 不可达 | **缺陷见证**（F2）。修好"创建不再全推/或允许分批"后必须失败并改写 |
| 预览只读：不改 task 行、不消耗限速窗口、不改明细行 | 正向契约（真实通过） |
| 全失败的 rollout 走 complete 而非 abort（阈值未配置） | 正向契约 + 暴露 F3 |
| apply 在真实库上以 42703 失败，且不留下任何状态 | **缺陷见证**（F1）。修复后必须失败并改写 |
| task 读模型无 `updated_at` 列 | **缺陷见证**（F1 根因）。补列后必须失败并改写 |
| 跨租户 preview / apply 被拒（201001） | 正向契约（真实通过）——权限面是好的 |
| 不存在的 task id 被拒 | 正向契约（真实通过） |

> 说明：9/9 通过**不等于**"灰度治理已验证"。其中 4 条是**缺陷见证用例**
> （characterization test，锁定当前坏行为），它们的作用是：一旦有人修好后端，
> 这些用例会立刻变红并强制改写为成功契约。文件头与用例内注释均已写明这一点。

---

## 4. 与既有 `tests/32_ota_runtime.test.js` 的关系（不重复造轮子）

`32_ota_runtime.test.js` 覆盖的是**单设备 OTA 运行时链路**：

- 通过公开 API 建包 + 建任务，启动**真实 MQTT 设备**；
- 断言设备侧收到 `inform` / `progress`（0/10/50/100）回执；
- 断言后端明细行落到 `succeeded(4)` 且 `progress>=100`，设备 `current_version` 更新；
- 第二条走**设备上报失败**：明细行 `failed(5)`、`progress=50`，并读回 `support-bundle` 的 `failed_count=1` / `failed_devices[0]`。

它**没有覆盖**（也是本轮 47 组补的部分）：

- 批次级治理决策（preview 的 7 个动作分支）；
- `governance-apply` 执行面；
- 治理参数的配置面与默认值；
- 跨租户对治理端点的访问；
- `ota_upgrade_tasks` 的 rollout 治理列（`rate_window_dispatched` 等）是否被真正推进。

一句话边界：**32 组证"一台设备能不能升上去"，47 组证"一批设备该不该被放出去"——后者当前不成立。**

---

## 5. 建议如何修正 ROADMAP §1.2 的 P0.3 行

现状：`P0.3 | OTA 状态机 | partial | 未验证 | 真实设备/broker 或协议 stub E2E；灰度治理无运行期证据文档`

问题：把灰度治理归为 `未验证`（"代码 + 接线齐备，仅缺运行期证据"）在本轮被证伪——
接线**不齐备**（配置面、OpenAPI、UI、自动驱动全缺），且真实运行证明执行面**不可用**。

建议改写（状态仍是 `partial`，但缺口类型必须拆分）：

```
P0.3 | OTA 状态机 | partial |
  未实现（灰度治理执行面与配置面）+ 未接线（OpenAPI/UI/自动驱动）+ 未验证（真实设备 E2E） |
  真实设备/broker 或协议 stub E2E；
  灰度治理：apply 写路径 42703、治理参数无法通过 API 配置、创建即全推致 pending 恒为 0、
  暂停/继续/百分比/观察窗/转全量零代码、无 UI 无 OpenAPI
```

对应 §1.3 的调整：把"P0.3 灰度治理证据"从 **A 类（只差跑一遍）** 移到 **E 类（需开发排期）**，
并在该行注明"只读预览已有运行期证据；执行面属未实现"。

> 这正是 §1.2.1 记过的同一类错误：把"没真正跑过的代码"当成"已完成"。
> 本轮的区别是——这次**真的跑了**，跑出来的是 42703。

---

## 6. 立项建议（按 ROADMAP §4 与 §7.4 四项格式）

### 条目：OTA 灰度（金丝雀）治理闭环

**缺口类型**：`未实现`（执行面 + 配置面 + 触发条件），附带 `未接线`（OpenAPI / UI / 自动驱动）。
**量级**：**M**（纯规划器已就绪且单测齐备，主要工作在接通与补语义，不需要重新设计决策逻辑）。

#### 6.1 交付物

1. **迁移**：新增迁移给 `ota_upgrade_tasks` 补 `updated_at timestamptz`（或改 DAL 去掉该列写入——推荐补列，治理状态本就该有更新时间）；同时确认 `aborted` / `timed_out` 两个 CHECK 状态词是否要有代码写入方（当前 abort/timeout 都写 `canceled`，状态词表里的 `aborted`/`timed_out` 无生产方）。
2. **DAL**：`internal/dal/ota_rollout_governance.go` 的 `UpdateOTAUpgradeTaskRolloutState` 改列名或补列；补一条 PostgreSQL 常驻用例直接打真实库（现有 `ota_rollout_governance_*_test.go` 都是纯单测/注入桩，正好是 §1.2.1 的盲区）。
3. **service / API**：
   - `CreateOTAUpgradeTaskReq` 增加治理入参（`rollout_rate_per_minute`、`abort_failure_rate_percent`、`scheduled_at`、`timeout_seconds`），`dal.CreateOTAUpgradeTaskWithDetail` 真正落库，并加参数边界校验（与 38.sql 的 CHECK 对齐：rate 1..300、abort 0<x<=100、timeout_seconds 60..604800）。
   - 明确"创建是否立即全推"的语义：若要灰度生效，创建后明细行必须保持 `pending`，由治理循环（见下）按批次领取；否则 `rollout_rate_per_minute` 永远只是个装饰。
   - 补暂停 / 继续（`paused` 状态 + 计划器 `hold_paused` 分支）、百分比放量（首批 N%）、观察窗（soak_seconds）、转全量（promote）。
4. **驱动**：把 `ApplyRolloutGovernance` 接到 cron / worker（参考 `initialize/croninit/cron.go`），而不是只靠人点一下 HTTP——否则"每分钟 N 台"这个语义无从体现。
5. **OpenAPI**：修 `internal/api/ota.go:228/250` 的 `@Router` 路径并重新生成 `docs/swagger.json` / `docs/docs.go`。
6. **前端**：OTA 任务详情里加治理面板（当前状态、下一步动作、限速窗口、失败率、中止原因、暂停/继续/转全量操作）。
7. **文档**：本文 + 修复后的复跑证据。

#### 6.2 契约测试（成功 / 失败 / 越权 / 幂等 / 超时 / 降级）

| 维度 | 需要补的用例 |
| --- | --- |
| 成功 | `dispatch_batch` 真的只放一批（N 台）而不是全推；`complete` 在全部收尾后落 `completed` |
| 失败 | 失败率越阈值 → `abort` → 任务置终态且剩余 pending 明细行被取消（当前**连触发都做不到**） |
| 超时 | `timeout_at` 越过 → 收尾并取消未收尾设备 |
| 越权 | 跨租户 preview/apply 被拒（**已有，实测 201001**）；TENANT_USER 名下设备过滤 |
| 幂等 | 同一窗口内重复 apply 不得重复放量（`rate_window_dispatched` 单调且窗口滚动清零） |
| 降级 | broker 不可用时批次不被标记为已下发；`abort_failure_rate_percent` 为 NULL 时不得自动中止（**已有**） |
| 数据库常驻 | 至少一条真实 PostgreSQL 用例覆盖 `UpdateOTAUpgradeTaskRolloutState` 与 `CancelOTAUpgradeTaskPendingDetails`（现有全为桩注入） |

`automation_tests/tests/47_ota_gray_governance.test.js` 已有 9 条（其中 4 条为缺陷见证），修复后按上表扩到 15 条以上，并把 4 条见证用例改写为成功契约。

#### 6.3 运行证据

- 环境：本地 backend + PostgreSQL（与本文同一套），必要时加 MQTT broker 让 `pushed/upgrading` 分支可达。
- 落地：`docs/validation/` 新建复跑文档，附命令、原始报文、task 行前后快照、`rate_window_dispatched` 的推进序列。
- 清理：删除本次与后续建的包/任务/设备（47 组已在 `after` 里清理）。

#### 6.4 四面一致

| 面 | 现状 | 需要补 |
| --- | --- | --- |
| API / OpenAPI | 端点存在但 OpenAPI **未收录**，且 swagger 注释路径与路由不一致 | 修注释 + 重新生成 |
| 后端权限 | Casbin 已登记（63.sql / 91.sql），跨租户实测被拒 ✅ | 保持 |
| UI 行为 | **零接线** | 新增治理面板 |
| 自动化 E2E | 47 组 9 条（含 4 条缺陷见证） | 修复后扩到覆盖 §6.2 全表 |

---

## 7. 本轮未做 / 仍 pending

- 未跑 `go build ./...` / `go test ./...`（内存约束，且另有 worker 在跑 Go 构建）。`ota_rollout_governance_apply_test.go` 等单测是否仍绿未复核——但它们是桩注入测试，**绿了也证明不了 F1/F2/F3**，这正是 §1.2.1 的教训。
- `dispatch_batch` 与 `hold_rate_window` 分支未取到真实运行证据：前者因 F2 不可达，后者需要一台在线设备停在 `upgrading`（需 MQTT broker）。
- `abort` 分支未取到真实运行证据：需要能设置 `abort_failure_rate_percent`，HTTP 面做不到（F3）。
- 修复方案未实现：本轮只判定，不改 `backend/`。
- 未 push；未改 `ROADMAP.md`、`backend/`、`frontend/`。

## 8. 工作树状态

本轮只新增两个文件：

- `automation_tests/tests/47_ota_gray_governance.test.js`（新增）
- `docs/validation/2026-09-15-p03-ota-gray-governance-evidence.md`（本文）
