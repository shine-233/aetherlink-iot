# 部署前清理与复测方法

本文是 AetherLink IoT 的部署前复测 runbook。它记录的是可重复的检查方法、证据边界和清理边界，不把一次本地 green 结果扩大解释为真实 RDI 或生产环境验收。

## 1. 结论口径

每一轮结果必须把下面几类分开记录：

```text
source/static       源码、契约、迁移链和脚本检查
local-core          本地 PostgreSQL/Redis/GMQTT/Backend/Frontend 核心链路
synthetic-rdi       软件生成的 RDI 协议/身份/激活夹具链路
generic-emulator    非 RDI 设备的 MQTT 在线、命令和 ACK 链路
native-visualization 本地 Native/local 看板 provider 链路
external-optional   ThingsVis/HTTP adapter 等显式可选外部服务
real-rdi            真实 RDI PID、硬件、固件、遥测和设备 ACK
target-deployment   目标机 Docker/Compose、公网、HTTPS、反向代理和恢复演练
```

`source/static`、`local-core`、`synthetic-rdi`、`generic-emulator` 或 `native-visualization` 通过，都不能关闭 `real-rdi`、`external-optional` 或 `target-deployment` 门禁。

ThingsVis 不是 Native/local provider 的实现来源。AetherLink 默认核心路径是 Native visualization；ThingsVis Server/Studio、HTTP adapter、SSO、mirror dashboard 和 negative-menu 属于显式 optional/external 集成。没有真实外部服务时应记录 `disabled`、`configuration-required` 或 `external-blocked`，不能用 Native 页面、mock API、route fulfill 或静态 iframe 冒充 ThingsVis runtime。

## 2. 运行隔离和敏感信息

本地复测使用独占的运行资源，命名中包含日期和序号。例如：

```text
database: aetherlink_iot_isolated_YYYYMMDD_NN
backend:  127.0.0.1:19999
preview:  127.0.0.1:9725
GMQTT:    127.0.0.1:11086
frontend: 127.0.0.1:5002（如已有用户进程，先识别后保留）
```

数据库密码只允许来自当前 PowerShell 进程环境变量或交互式 secret 输入。禁止写入以下对象：

- 本文、`VALIDATION.md`、测试报告、manifest、截图、trace、命令文件和 git diff；
- `.env`、`.env.local`、Playwright storage state 或日志归档；
- PowerShell 历史、带密码的命令行参数和提交内容。

推荐在当前 PowerShell 会话内设置一个任务专用变量，例如 `$AetherLinkDbPassword`，并通过 `$env:GOTP_DB_PSQL_PASSWORD = $AetherLinkDbPassword` 传给子进程；结束时清空这两个变量。不要把密码直接写入脚本。

当前 PostgreSQL 17 的 `pg_dump.exe` 不接受本项目旧脚本中的 `-X` 参数。本方法的 `pg_dump` 命令不使用 `-X`。PowerShell 生成 `DROP DATABASE` 时使用字符串拼接，不使用错误的反斜杠转义：

```powershell
$dropSql = 'DROP DATABASE IF EXISTS "' + $restoreDb + '";'
```

## 3. 清理前冻结和分类

清理不是 `git clean`、`git reset --hard`、`docker compose down -v` 或批量杀进程。清理前按以下顺序操作：

1. 保存本轮最终必要的命令、退出码、服务地址、代码状态和结果摘要。
2. 读取项目规则、`GENERATED_FILES.md`、`verification/README.md` 和本方法。
3. 检查 `git status --short`，把已有用户修改视为不可覆盖内容。
4. 检查目标目录是否为 junction/reparse point，拒绝跟随外部目标。
5. 检查监听端口和进程命令行，先停止本轮自己的进程；不要批量停止所有 `node`。
6. 对每个候选逐项确认：是否可再生成、是否被当前服务使用、是否是唯一证据、是否含敏感数据、是否能从源码和同一命令恢复。
7. 只对已经确认的精确绝对路径执行删除，删除后复核路径不存在、端口已释放和磁盘空间变化。

### 必须保留

源码、lockfile、`AGENTS.md`、部署脚本、Compose 文件、迁移、生成源码、测试脚本、Native provider、本地看板组件、ThingsVis 兼容源码、optional 配置契约和必要的部署/兼容文档必须保留。`frontend/build/**/*.ts` 是构建配置源码，不是可删的 dist。

### 通常可清理但必须逐项复核

确认不再被进程或当前证据引用后，可清理：

```text
frontend/coverage/
frontend/dist/
frontend/dist-lite/
automation_tests/reports/
automation_tests/test-results/
automation_tests/.e2e-auth/
各模块 _localrun/ 和本地运行日志/截图/临时 dump
.playwright-cli/、playwright-report/、test-results/
```

本轮 clean-room 复测前可以删除旧的运行输出，但不要删除源码、部署契约、唯一的用户业务数据或无法恢复的持久卷。PostgreSQL、Redis、broker persistence、上传文件、telemetry spool 和 uplink spool 不是普通构建缓存；没有备份/恢复证据时不得执行 `docker compose down -v`。

`_isolated/thingsvis-upstream-YYYYMMDD` 若仅为可从其 upstream 重新取得的上游源码副本，并且没有需要保留的源代码修改、活动进程、reparse link 或核心项目引用，可以精确删除整个副本。删除它不会删除 AetherLink 的 Native provider，也不会删除核心目录的 ThingsVis 兼容契约。

## 4. 源码和构建门禁

在启动长时间 runtime 前先完成语法、类型、单元测试和构建检查。建议使用仓库锁定的 pnpm 版本和 Go toolchain：

```powershell
cd <repository-root>\aetherlink-iot

node --check automation_tests/lib/seed_data.js
node --check automation_tests/e2e/20_command_jobs.spec.js
node --check automation_tests/e2e/21_ready_check_command_draft.spec.js

cd backend
go test ./...
go build ./...

cd ..\mqtt-broker
go test ./...
go build ./...

cd ..\frontend
pnpm typecheck
pnpm test:run
pnpm build
```

如果磁盘空间不足，先清理已经确认的 `dist`、coverage 和旧运行输出；不要为了节省空间删除 Go/pnpm 依赖缓存后立刻运行大规模测试，除非已经确认依赖可重新下载且网络/空间足够。

前端 coverage 使用独立目录，避免覆盖旧报告：

```powershell
$coverageDir = '<repository-root>\aetherlink-iot\_localrun\predeploy-runtime-<run>\frontend-coverage'
cd frontend
pnpm exec vitest run --coverage --maxWorkers=1 --minWorkers=1 --no-file-parallelism --coverage.reportsDirectory=$coverageDir
```

同时记录 test count、exit code、覆盖文件数及 statements/branches/functions/lines。只报告阈值通过还不够，还要注明测试是否真实执行、是否存在 mock-only 或 route-only 边界。

## 5. 迁移和数据库恢复

### 迁移链

静态扫描 `backend/sql/29.sql` 至当前最高版本：

- 文件号不能缺失或重复；
- 迁移文件不应错误地自行写入 `sys_version`；
- 最高迁移号应和目标数据库 `sys_version` 一致；
- 在隔离数据库上执行后检查关键表、索引和服务启动日志。

### backup/restore 演练

不要使用真实密码落盘。流程为：

```powershell
pg_dump -Fc --host 127.0.0.1 --port 5432 --username <user> --dbname <source-db> --file <run-dir>\source.dump
pg_restore --list <run-dir>\source.dump

# 创建临时 restore database 后：
pg_restore --no-owner --exit-on-error --single-transaction `
  --host 127.0.0.1 --port 5432 --username <user> `
  --dbname <restore-db> <run-dir>\source.dump
```

恢复后比较 `sys_version`、关键表行数和稳定 ID signature；记录 restore-list entries、signature_equal 和退出码。删除临时恢复库和 dump 时使用精确数据库名/文件路径，删除动作完成后再次查询确认。

这只能证明隔离 PostgreSQL 的可读/可恢复。目标 Compose volume、Redis、broker persistence、文件卷、telemetry spool、uplink spool 仍需目标环境专项演练。

## 6. 启动隔离核心 runtime

先确认 PostgreSQL 和 Redis 可达，然后用独占数据库和显式端口启动 GMQTT、Backend、Preview proxy。GMQTT 测试端口必须显式设置为 `11086`；不要让测试默认回落到 `1883`，因为 `1883` 可能是用户已有 broker 或目标发布端口，回落会造成“测试看似通过但实际连错 broker”。

启动后逐项检查：

```text
PostgreSQL 127.0.0.1:5432
Redis      127.0.0.1:6379
GMQTT      127.0.0.1:11086
Backend    127.0.0.1:19999
Preview    127.0.0.1:9725
```

请求并记录 `/health`、`/ready`、`/api/v1/deployment/health` 和 preview 代理后的真实 JSON。`/health` 只证明进程存活；`/ready` 才是核心依赖就绪门槛。ThingsVis、SMTP、Market、地图等 optional 状态为 disabled/configuration-required/external-blocked 时，不能把它们改写成核心失败，也不能改写成 optional 通过。

## 7. API、Native 和浏览器复测

### API

在 `automation_tests` 中先执行：

```powershell
pnpm test:list
pnpm preflight:api-e2e
```

然后使用真实 backend JSON 和隔离数据库执行 API 全量：

```powershell
node run_tests.js
```

如需包含浏览器：

```powershell
node run_tests.js --include-e2e --workers=1
```

必须从新报告提取模块数、case 数、passed/failed/skipped 和每个 skip 原因；不能把历史 `20/20`、`63/63`、`372/372` 直接当作本轮结果。

### Native/local visualization

Native 是默认验收面。至少验证真实前端请求和持久化业务状态：看板创建/编辑/保存/发布/查看、设备目录、字段绑定、遥测展示和必要的写入结果。Native 组件/服务路径主要位于：

```text
frontend/src/service/visualization-provider/
frontend/src/components/local-visualization-viewer/
```

页面 route hit、静态组件 mount 或仅有 mock provider 不等于业务闭环；断言必须包含 API 响应、数据库/页面状态变化或可回读的业务结果。

### Ready Check command draft

代码改动后必须单独重跑：

```powershell
pnpm exec playwright test e2e/21_ready_check_command_draft.spec.js --workers=1 --reporter=list
```

它使用独占 generic online emulator 时，报告应明确“非 RDI 设备、真实本地 MQTT status/command/ACK”；不能写成真实 RDI。

### Command Jobs online emulator

`e2e/20_command_jobs.spec.js` 的合规本地方法是：

1. 本轮独占创建非 RDI 设备、template、command model 和 device config；
2. 绑定 config 并启动仓库已有 Go command emulator；
3. emulator 连接隔离 GMQTT `127.0.0.1:11086`，发布 `devices/status/{device_id}` 的在线状态；
4. 浏览器提交命令后，emulator 从真实 command topic 读取并发布 ACK；
5. failure case 使用明确的 `e2e_forced_failure` ACK，不伪造浏览器响应；
6. 测试结束逆序删除 job/config/template/device 并停止 emulator；
7. 记录 preview、历史、cancel、失败 ACK、retry、support preview/bundle 等业务证据。

该路径证明本地软件 MQTT command pipeline，不证明实体 RDI PID、voucher、硬件身份、固件协议或物理 ACK。

### 浏览器

Playwright 默认使用 Edge；先确认 `npx`/Edge 可用和 `PLAYWRIGHT_BROWSER_CHANNEL` 或 `PLAYWRIGHT_BROWSER_EXECUTABLE_PATH`。真实 E2E 必须指向 `127.0.0.1:9725` preview proxy，而不是把 API 请求返回成前端 HTML。重新 snapshot 后再使用 locator；结果归档到本轮 `_localrun` 或 `verification/<timestamp>/`，不要提交 auth storage state。

## 8. Synthetic RDI 和外部 ThingsVis 边界

仓库已有 synthetic RDI lane 可以验证软件协议形状、PID/激活状态转换、voucher/硬件身份字段和 MQTT packet 处理。运行时必须标记：

```text
AETHERLINK_RDI_FIXTURE_MODE=synthetic-rdi
```

synthetic fixture 只关闭软件路径门禁。它不能提供不可伪造的真实 PID、真实 voucher、真实硬件身份、真实固件 MQTT session、物理遥测、物理在线状态或真实设备 ACK。generic MQTT emulator 同理。

ThingsVis 专项只有在真实 `thingsvis-server`、`thingsvis-studio`、HTTP adapter、secret、SSO/账号和真实 mirrored dashboard 都存在时才运行。至少配置并验证：

```text
127.0.0.1:8000 ThingsVis Server
127.0.0.1:3000 ThingsVis Studio
THINGSVIS_AUTH_SECRET
THINGSVIS_MIRRORED_DASHBOARD_ID
```

缺任一运行前置就记录 3 个 external-blocked 场景（ThingsVis、negative-menu、mirror），不创建 mock 服务以消除 skip。

## 9. 每轮结束清场

测试结束后按逆序清理：

1. 停止本轮 emulator、preview、backend、GMQTT；保留用户已有的前端/服务进程。
2. 删除临时设备、配置、模板、数据库恢复库和临时 dump。
3. 清除当前 PowerShell 的数据库密码变量和敏感认证状态。
4. 确认 `127.0.0.1:8000`/`3000` 若未启用 ThingsVis 仍保持未监听，并记录 external-blocked，而非失败伪装。
5. 复查工作树：只保留源代码和明确授权的本轮文档/测试改动；不使用 reset/checkout/clean 覆盖其他人的修改。
6. 复查端口、进程、磁盘、敏感文件和 reparse point。

最终报告至少包含：运行时间、checkout/worktree 状态、命令和退出码、服务端口、数据库版本、coverage、API/E2E 数量、skip 分类、证据目录、清理对象、未关闭的真实 RDI/目标部署/外部 ThingsVis 门禁。

## 10. 生产签署前仍必须现场验证

以下项目不能由本地 synthetic/emulator 代替：

```text
真实 RDI PID 与 activation
真实 voucher、硬件身份、固件 MQTT session
真实物理遥测、在线状态和设备 ACK
真实 RDI share/link 与生产跨租户权限链
目标环境 Docker/Compose runtime
公网 MQTT、HTTPS/TLS、反向代理、防火墙和端口
真实 ThingsVis Server/Studio/SSO/embed/mirror/negative-menu
目标环境 PostgreSQL、Redis、broker、文件和 spool backup/restore
```

因此本方法可以把“部署前的软件准备和本地可复核链路”做完整，但不能在没有实体设备和目标环境凭证时生成生产 sign-off。

## 11. 2026-08-14 实际复测记录与本轮修正

本节是 2026-08-14 本地复测的当前摘要。它只代表本次 checkout、当前本机服务和当前测试账号，不替代目标环境验收。

### 11.1 ThingsVis 的保留边界

ThingsVis 不是 Native visualization 的启动依赖，也不是 Native provider 的实现来源。Native 是默认本地 provider；ThingsVis 兼容层用于承接已有 ThingsPanel/ThingsVis 看板、iframe、SSO、dashboard、bridge、旧菜单和旧配置。因而：

- 保留 `frontend/src/components/thingsvis/`、`frontend/src/hooks/thingsvis/`、`frontend/src/utils/thingsvis/`、`frontend/src/views/visualization/thingsvis*/`、`frontend/src/service/visualization-provider/` 和 optional Compose/Nginx 契约；
- 保留兼容源码不等于声称 ThingsVis 服务已运行；
- 独立的 `_isolated/thingsvis-upstream-20260802` 上游下载副本已经不存在，本轮没有发现可继续删除的 ThingsVis 上游副本；
- `127.0.0.1:8000` ThingsVis、negative-menu 以及 `THINGSVIS_MIRRORED_DASHBOARD_ID` 仍分别记录为 `category=runtime-external`、`seedable=false`、`status=external-blocked`；不创建 mock 服务消除这些阻断。

### 11.2 本轮已经验证的结果

| 验证面 | 本轮结果 | 证据边界 |
|---|---|---|
| PostgreSQL native backup/restore | `pg_dump -Fc`、恢复库和关键数据核对已通过；源库与临时恢复库的关键计数一致 | 本机 PostgreSQL 17；不是 Compose `postgres:16-alpine` 或目标机恢复证据 |
| strict preflight | 配置和核心连通性通过；3 个 ThingsVis optional 场景 external-blocked | 只证明本地 release-style preflight |
| synthetic RDI network lane | activation、遥测、在线/离线转换、命令成功/失败 ACK、API/SQL 回读通过 | `isolated-software-path-only`；`real_rdi_status=not-tested` |
| 既有选定 API | 9/9 通过 | 不是当前全部 API 模块的最终全量结果 |
| 既有选定 E2E | 9/9 通过；visualization 的 ThingsVis 外部路径仍 partial-skip | 页面覆盖率不能替代业务闭环 |
| 剩余 E2E 首轮 | 10/11 通过；唯一失败为 OTA 夹具串库 | 失败证据保留用于排障，不当作产品缺陷 |
| OTA 针对性真实浏览器复跑 | 1/1 business E2E 通过 | 创建真实 OTA package/task fixture、API 回读、浏览器下载和 JSON 一致性均断言 |

### 11.3 本轮发现并修正的串库坑

E2E runner 的 Backend/Broker 使用了临时恢复库，但 `automation_tests/lib/seed_data.js` 的 `psql` 夹具进程没有继承同一个数据库目标，于是回退到默认 `aetherlink_iot_local`。症状是 OTA task fixture 写入失败，根因不是 OTA 表 schema：源库和恢复库都为迁移 48，且同一 SQL 在统一目标库下通过。

以后启动隔离 API/E2E 时，必须在“启动 Backend/Broker 的子进程”和“启动 `run_tests.js` 的同一父进程”中同时设置同一组运行时数据库变量：

```text
GOTP_DB_PSQL_HOST
GOTP_DB_PSQL_PORT
GOTP_DB_PSQL_DBNAME
GOTP_DB_PSQL_USERNAME
GOTP_DB_PSQL_PASSWORD
AETHERLINK_DB_HOST
AETHERLINK_DB_PORT
AETHERLINK_DB_NAME
AETHERLINK_DB_USER
AETHERLINK_DB_PASSWORD
AETHERLINK_PSQL_PATH
```

密码只能通过当前进程环境传递，不能写入本文、报告、manifest、命令文件或命令行参数。启动后应在测试 runner 进程内打印脱敏的 `database_name`（只允许库名，不打印密码），并用 `SELECT current_database()` 对 runner 的目标库做一次只读确认。任何 Backend 目标库与 fixture `psql` 目标库不一致时，必须在 E2E 前置阶段直接失败，不能让夹具回退到默认库。

最终隔离 API/E2E 运行还必须设置 `AETHERLINK_STRICT_DB_TARGET=1`。严格模式下，`automation_tests/lib/seed_data.js` 在 OTA `psql` 夹具没有从 `AETHERLINK_DB_NAME`、`GOTP_DB_PSQL_DBNAME` 或 `PGDATABASE` 得到显式数据库目标时直接拒绝运行；当 `AETHERLINK_DB_NAME` 与 `GOTP_DB_PSQL_DBNAME` 不一致时也直接拒绝。非严格模式仍保留旧的本地兼容回退，但不能用于 release-style 隔离证据。

此外：GMQTT 的业务监听端口来自 `gmqttd.yml` 的 TCP listener；环境变量不会覆盖该 YAML 端口。启动前必须读取配置并确认 `127.0.0.1:11086` 可达，不能仅凭 HTTP readiness 或自定义环境变量假定 broker 已在监听。

Backend 必须从 `backend/` 工作目录启动，以保证 `./configs/casbin.conf` 等相对路径可解析；Preview proxy 必须先启动，再做 frontend connectivity preflight。Windows PowerShell 不支持 `Start-Process -Environment`，应使用子进程脚本显式设置环境；Node 的提示性 stderr 不能被外层 PowerShell 当作退出失败，最终只依据 runner 退出码和 JSON summary 判定。

### 11.4 本轮清理策略

清理前先保留一份脱敏的最终 summary、方法文档和当前失败/修复证据索引；然后只删除精确确认的、已停止进程使用、可由同一源码和命令重建、且不被权威文档引用的运行产物。以下对象永远不因“ThingsVis 未启动”而删除：Native provider、ThingsVis 兼容源码、optional Compose/Nginx 契约、迁移、测试脚本、`node_modules`、pnpm/Go 依赖缓存、PostgreSQL 源库、Redis 数据、broker persistence、用户已有 dirty 修改和用户手册。

最终报告必须分别给出：

```text
native_core_status
synthetic_software_path_status
thingsvis_optional_status
real_rdi_status
target_deployment_status
production_signoff
```

2026-08-14 在真实 RDI PID、真实 voucher/硬件身份、真实固件 MQTT session、真实物理遥测/在线状态/ACK、真实生产 share/link、ThingsVis Server/Studio/SSO/mirror、Docker/Compose 目标环境和公网 TLS/MQTT 均未具备时，`production_signoff` 仍必须是 `not-ready`。

### 11.5 2026-08-14 复测增量：Native 默认路径、清理和 fresh coverage（historical；已被 11.8 r8 覆盖）

本小节记录 r4 前后的中间复测，现仅用于复盘；当前 automation aggregate、coverage 和外部阻断以 11.8 的 r8 证据为准。该中间复测使用过 Backend `127.0.0.1:19999`、Preview `127.0.0.1:9725`、GMQTT `127.0.0.1:11086`；默认 MQTT `127.0.0.1:1883` 和 ThingsVis `127.0.0.1:8000/3000` 均未监听。

#### ThingsVis 为什么仍在仓库中

当前代码已经把 Native 设为默认本地 provider：未指定 provider 时选择 `native-board`，Native 通过 AetherLink 自己的 `/board` API、`vis_type=native` 数据和本地 viewer 工作；只有显式启用 `VITE_ENABLE_THINGSVIS_COMPAT=Y` 或明确选择 `legacy-thingsvis` 时，才选择外部兼容 provider。相关测试在本轮 `5` 个文件、`34` 个测试中全部通过。

因此 ThingsVis 不是 Native 的运行依赖，而是兼容层，继续承接已有 ThingsPanel/ThingsVis 项目、dashboard、iframe/Studio、SSO、mirror dashboard、旧菜单和旧配置。删除 `frontend/src/components/thingsvis/`、`frontend/src/hooks/thingsvis/`、`frontend/src/utils/thingsvis/`、`frontend/src/views/visualization/thingsvis*/`、`frontend/src/service/api/thingsvis.ts`、`frontend/src/service/visualization-provider/legacy-thingsvis-adapter.ts` 或 optional Compose/Nginx 契约，会改变旧数据和兼容路由的产品范围，不属于缓存清理，当前不删除。

本地 Native 可以独立运行并已通过 provider/route contract；ThingsVis 的 `8000`、`3000`、SSO、mirror 和 negative-menu 场景仍必须记录为 `runtime-external` / `external-blocked`。不能用 Native 页面、mock API、静态 iframe 或 simulation device 把它们改写为 ThingsVis 业务通过。

#### 类似问题的处理边界

| 项目 | 结论 | 处理方式 |
|---|---|---|
| Native board/provider | 默认核心 | 保留源码；继续做 API、数据库和浏览器业务验证 |
| ThingsVis/HTTP adapter/negative-menu | 可选兼容/外部服务或负向场景 | 保留契约和配置；未启用时记录 external-blocked，不创建 fake 服务 |
| MQTT `1883` | 默认核心接入端口 | 不删除；本地软件测试使用显式隔离 broker `11086`，真实设备仍需目标可达地址 |
| Docker/Compose、HTTPS、反向代理、公网、防火墙 | 目标环境门禁 | 不以本机 loopback 通过替代；在目标机单独验收 |
| coverage、Vite cache、tsbuildinfo、Playwright test-results | 可再生输出 | 仅在保存结果、确认无进程使用后移动到可恢复 quarantine |
| 源码、迁移、`frontend/build`、node_modules、数据库/Redis、最终 evidence | 不属于可安全缓存 | 保留，不执行 `git clean`、`reset --hard`、`down -v` |

`negative-menu` 不是一个需要清理的服务、容器或镜像，而是验证 dashboard menu ownership rejection 的负向测试标签。当前没有发现需要删除的 ThingsVis upstream 副本。Market、SMTP、地图 provider、外部遥测存储和 PostHog 同样属于可选能力：可以不进入默认启动集合，但不能因为当前没有配置就删除能力状态、配置合同或测试分支。

#### 本轮已经验证的 fresh 结果

- `pnpm run test:coverage`：退出码 `0`；覆盖输入 `832` 个源文件，statements `134813/174404 = 77.30%`，branches `15154/19540 = 77.55%`，functions `4561/6828 = 66.80%`，lines `134813/174404 = 77.30%`。当前配置只声明 `text/json/html/lcov` reporter，因此不生成 `coverage-summary.json`；数字来自 `coverage-final.json` 和 `lcov.info`，不是历史报告。
- provider/route focused Vitest：`5/5` 文件、`34/34` 测试通过。
- synthetic RDI protocol contract：`8/8` 单元契约通过；仍只证明 synthetic protocol-emulator path。
- deploy contract scripts：POSIX `15/15`、PowerShell `2/2` 通过；涉及 Docker Compose 的动态校验按合同记录为 skipped，因为当前主机没有 Docker/Compose，不是伪造通过。
- 真实浏览器已登录隔离租户管理员并打开 `/device/command-center`：命令中心标题、返回 Fleet、preflight/preview 引导、submit 控件和非登录状态均断言通过，页面视觉状态已检查。该证据只证明页面壳和当前空目标态可渲染；没有设备 ACK，因此不升级为 command-job business closure。
- 当前 r8 aggregate 的原始目录已移到仓库外 quarantine：API `64/64`、E2E `20/20`、`0 failed`、business evidence `30/30`、endpoint `372/372`、page/flow `56/56`；仅 visualization 有 `3` 个 `runtime-external` partial-skip。旧 `predeploy-full`/r4 的 `63/64`、`11` 个 partial-skip、`55/56` 仅保留为历史复测记录。本节的中间浏览器检查不能改写 r8 aggregate。

#### 本轮可恢复清理记录

在 coverage 结果已解析、相关进程已结束、目标不是 reparse point 后，执行了精确移动而非删除：

```text
frontend/coverage
  -> ../_aetherlink-cleanup-quarantine-20260814/frontend-coverage-predeploy-20260814
  2072 files, 77071939 bytes

frontend/node_modules/.vite
frontend/.tsbuildinfo
automation_tests/test-results
  -> ../_aetherlink-cleanup-quarantine-20260814/retest-regenerated-caches-20260814/
```

### 11.6 2026-08-14 r2-r4 复测闭环与重复踩坑记录（historical；已被 11.8 r8 覆盖）

本节把“启动器失败”“测试目标漂移”和“测试夹具对列表排序的错误假设”分开记录；r4 是历史中间 aggregate，当前可用 aggregate 以 11.8 的 r8 为准：

| 运行 | API | E2E | 其他结果 | 真实原因 |
|---|---:|---:|---|---|
| r2 | 64/64 | 0/20 | E2E 全部在业务开始前失败 | 已有 `127.0.0.1:9725` 被 `PLAYWRIGHT_REUSE_EXISTING_SERVER=0` 再次启动，属于 harness 启动配置错误 |
| r3 | 63/64 | 19/20 | endpoint `366/372`，page `55/56` | API boundary helper 从 `.env.local` 读取了旧 `API_TARGET=9999`；home 测试把 seed fixture 错当成列表第一项 |
| r3 窄回归 | 1/1、22 assertions | 1/1 | 两个失败点均通过 | API 显式设置 `API_TARGET=19999`；home 只断言真实 API 第一条的稳定身份/名称，再验证浏览器加载和 Refresh |
| r4 | 64/64 | 20/20 | endpoint `372/372`，page `55/56` | 全量无失败；剩余 24 个场景为 `runtime-external` partial-skip |

r4 的证据目录为：

```text
_localrun/predeploy-post-clean-20260814-r4/full-api-e2e/reports/summary.json
_localrun/predeploy-post-clean-20260814-r4/full-api-e2e/reports/endpoint-coverage.json
_localrun/predeploy-post-clean-20260814-r4/full-api-e2e/reports/page-coverage.json
_localrun/predeploy-post-clean-20260814-r4/full-api-e2e/verification/automation-run-20260814-042951/
```

重跑前必须同时固定以下目标：

```text
API_BASE_URL=http://127.0.0.1:19999/api/v1
API_TARGET=http://127.0.0.1:19999
HEALTH_URL=http://127.0.0.1:19999/health
FRONTEND_URL=http://127.0.0.1:9725
PREVIEW_URL=http://127.0.0.1:9725
AETHERLINK_DB_NAME=<同一个隔离数据库>
GOTP_DB_PSQL_DBNAME=<与 AETHERLINK_DB_NAME 完全相同>
AETHERLINK_STRICT_DB_TARGET=1
```

`API_TARGET` 必须与 `API_BASE_URL` 的 origin 完全一致；只设置 `API_BASE_URL` 不足以覆盖 `api_closure_helpers.js` 的 root/static/websocket 边界请求。已经有归属明确且健康的 preview 代理时使用 `PLAYWRIGHT_REUSE_EXISTING_SERVER=1`；由本轮 runner 自己启动 preview 时使用 `0`，并且只能由该 runner 管理它。

`ensureDevice()` 的作用是保证有可用设备或复用现有 fixture，不保证该 fixture 是 `/device?page=1&page_size=1` 的第一条。首页测试必须先读取实际第一条，再对该条做 API、浏览器初始加载和 Refresh 回读断言；禁止把数据库排序或历史数据顺序写死成“seed 必须第一”。

本轮 fresh frontend coverage（`pnpm run test:coverage` exit `0`）为 832 个源文件：statements `134813/174404 = 77.30%`，branches `15154/19540 = 77.55%`，functions `4561/6828 = 66.80%`，lines `134813/174404 = 77.30%`。数字分别由 `coverage-final.json` 与 `lcov.info` 独立核对；不要复用旧报告中的 `77.29%/66.79%`。

r4 的 24 个 partial-skip 场景必须单独计数，不能包含在“20/20 E2E 模块通过”里当作完整覆盖：API data `9`、telemetry-extra `2`、mqtt-device-pipeline `2`；E2E device `1`、visualization `3`、command-jobs `6`、ready-check-command-draft `1`。其中 visualization 的 3 项就是 ThingsVis project/dashboard、negative-menu 和 mirror；MQTT `1883` 不监听导致 telemetry、Ready Check、真实 ACK 场景未执行。所有这些结果均为 `category=runtime-external`、`seedable=false`。

截至 r4，状态仍为：

```text
native_core_status       = local-core-verified
synthetic_software_status= partial-current
thingsvis_optional_status= external-blocked / optional-disabled
real_rdi_status          = not-tested
target_deployment_status = pending
production_signoff       = not-ready
```

### 11.7 本轮复测后的可恢复清理

r4 完成后只停止了本轮明确拥有的 PID `24704`（GMQTT）、`25136`（Backend）和 `25832`（Preview）。复查确认 `19999`、`9725`、`11086`、`8000`、`3000` 和 `1883` 均无本轮残留监听；没有批量停止 Node、PostgreSQL、Redis 或其他用户进程。

以下对象从项目目录精确移动到仓库外可恢复目录 `../_aetherlink-cleanup-quarantine-20260814-r4/`，没有直接删除：

| 项目内路径 | quarantine 名称 | 文件数 | 字节数 |
|---|---|---:|---:|
| `frontend/coverage` | `frontend-coverage-r4` | 2072 | 77071953 |
| `frontend/node_modules/.vite` | `vite-cache-r4` | 1 | 40797 |
| `automation_tests/test-results` | `automation-test-results-r4` | 3 | 91319 |
| `_localrun/predeploy-post-clean-20260814-r2` | `predeploy-r2` | 381 | 10392377 |
| `_localrun/predeploy-post-clean-20260814-r3` | `predeploy-r3` | 393 | 7322589 |
| `_localrun/predeploy-post-clean-20260814-r3-fix` | `predeploy-r3-fix` | 31 | 1571469 |
| `_localrun/predeploy-post-clean-20260814-r4/e2e-auth` | `r4-auth-state` | 6 | 9232 |
| **合计** |  | **2887** | **96499736** |

历史 r4 最终证据（移除 auth state 后 389 个文件、7211974 字节）已移到仓库外 quarantine；方法文档和源码仍在工作树。`frontend/build` 是 Vite 构建配置源码；`verification/templates` 是公开模板；带日期的历史 reports、`verification` 历史证据、`node_modules`、Compose/ThingsVis 契约、数据库/Redis 数据和用户已有 dirty 修改不应批量清理。

本轮仅用 provenance-protected cleanup 删除隔离数据库中的两条历史 synthetic RDI 行：`SYN260814108`、`SYNTHRDI0001`。两条记录在 cleanup 前均确认 `fixture_provenance=synthetic-rdi`、数据库为隔离库，cleanup 后逐条 status 均为 `absent`；没有删除普通设备或声称真实 RDI 资产。

秘密扫描只输出计数，不输出命中内容：本轮配置的数据库密码精确字符串在项目目录中命中 `0` 次；通用 `password=`、`secret=`、`authorization=` 和 JWT/Bearer 关键词仍有源码/测试合同中的预期命中，因此不能把关键词计数当成“项目没有任何秘密”的证明。密码没有写入本文、报告、manifest、auth state 或命令文件。

r4 历史快照中的 `page coverage 55/56` 唯一未覆盖路由是 `/device/command-center`；r8 `page-coverage.json` 已记录 `56/56`，当前该路由已有页面壳/空目标态证据，但仍没有真实设备 ACK，因此不能把它改写为 command-job business closure。

这些对象可由同一源码和命令重新生成；quarantine 保留，未删除。`frontend/build`、`verification/templates` 和源码仍在工作树；历史 `_localrun/predeploy-full-20260814`、数据库 dump、synthetic manifest 和最终报告已按当前上传边界移到仓库外，不能作为清理后 fresh evidence。

清理之后的全量 API/E2E 和必要构建复测必须使用新的独立 `AUTOMATION_REPORT_DIR`、`AUTOMATION_VERIFICATION_DIR`、`E2E_AUTH_DIR`，并在报告中保留本节的 fresh coverage 数字和外部阻断分类；不得把 quarantine 中的旧报告当作清理后的 fresh evidence。

## 11.8 2026-08-14 r8 fresh 重跑与清理边界（historical；已由 11.10 r11c 覆盖）

本节覆盖此前所有较早的运行快照；数字只对应本次 r8 checkout 和本节列出的证据路径。

### ThingsVis 为什么保留，以及什么不能清理

Native visualization 是 AetherLink 的默认、本地核心 provider；ThingsVis 是可选的 legacy compatibility provider。两者解决的不是同一个产品边界：Native 负责本地看板的创建、编辑、发布、查看和持久化；ThingsVis 兼容链保留旧 ThingsPanel/ThingsVis 看板、iframe、SSO、mirror dashboard、旧菜单和旧配置的适配合同。因此不能因为 Native 能运行就删除 ThingsVis 源码或 optional 部署合同。

必须保留 `frontend/src/components/thingsvis/`、`frontend/src/hooks/thingsvis/`、`frontend/src/utils/thingsvis/`、`frontend/src/service/api/thingsvis.ts`、`frontend/src/service/visualization-provider/legacy-thingsvis-adapter.ts`、`frontend/src/views/visualization/thingsvis*/`、`deploy/docker-compose.optional-integrations.yml`、`frontend/nginx.thingsvis.conf` 以及对应测试合同。`negative-menu` 不是服务、容器或孤儿模块，而是 ThingsVis dashboard ownership rejection 的负向测试标签，不能按服务残留清理。

本机 `127.0.0.1:8000`、`127.0.0.1:3000` 未监听，`THINGSVIS_MIRRORED_DASHBOARD_ID` 未配置；这三个场景仍是 `runtime-external / external-blocked / seedable=false`。Native 页面、mock API、静态 iframe 或 simulation device 都不能把它们改写成 ThingsVis 运行时通过。独立的 `_isolated/thingsvis-upstream-*` 若再次出现，也必须逐项确认无源码引用、无活动进程、无 reparse link 且可从上游重取后，才可移动到可恢复 quarantine；不能删除核心兼容源码。

### r8 fresh runtime 证据

- API：`64/64` modules passed，`0` failed；E2E：`20/20` modules passed，`0` failed；business evidence `30/30`；endpoint coverage `372/372`，`uncovered=0`；page/route coverage `56/56`。
- 唯一 partial lane 是 visualization 的 3 个 ThingsVis external-blocked 场景，原因仍是 `8000` connection refused、`negative-menu` 同一外部服务不可达、mirror ID 未配置。
- Ready Check generic command emulator：`6/6`；`ready-check-command-draft`：`1/1`。这些是 generic/non-RDI 软件证据，不是实体 RDI 设备验收。
- fresh synthetic lane 的原始证据已移到仓库外 quarantine。`run_synthetic_rdi_lane.ps1` 本轮成功退出 `0`，share/link focused API `1/1`，success/failure ACK 均通过，均观察到 `offline -> online -> offline`，fresh `temperature_1=25.5` 和 SQL readback 通过，offline manifest/session/replay 通过，secret scan 全部为 `0`。
- synthetic manifest 明确为 `fixture_provenance=synthetic-rdi`、`evidence_class=protocol-emulator`、`device_execution=not-proven`、`real_rdi_status=not-tested`、`production_signoff=not-ready`。它只证明 `protocol-emulator -> isolated GMQTT -> backend -> API/SQL` 软件路径。
- 本轮 lane 结束后 `11086`、`19999`、`8000`、`3000` 无监听，`aetherlink-backend`、`gmqttd` 和 synthetic emulator 无残留进程。r8 隔离数据库保留 fixture 供后续复核，默认数据库未被该 lane 写入。

### 数据库与当前仍未通过的硬门禁

- 默认 `aetherlink_iot` 当前只读回 `sys_version=45`、`version=0.0.23`、`devices=11`；它仍未达到源码 `VERSION_NUMBER=48`，所以是 release blocker。隔离 r8 数据库为 `48` 不能替代默认目标库。
- 默认库的 custom-format backup、临时 restore signature equality 和隔离 45→48 migration drill 已通过；这证明本地 PostgreSQL 应用库路径可回放，不证明目标机 Compose/Redis/broker/files/spool 的完整灾备。
- Go `go test ./internal/service -run TestCheckDBMigrationsRequiresCurrentMigrationVersion` 本轮退出 `1`，失败在 `proxy.golang.org` 依赖下载 setup，目标测试没有执行；不能写成代码失败，也不能写成通过。
- 本轮仍未验证 Docker/Compose target runtime、HTTPS/TLS/反向代理、公网 MQTT、目标环境 restore、真实 RDI PID/activation/voucher/hardware identity/firmware MQTT/physical telemetry/online state/ACK、真实 RDI share/link 跨租户权限链和生产 ThingsVis/SSO/mirror。

清理决策：ThingsVis 源码、optional compose/nginx、测试合同、当前 r8 fresh 证据、数据库 backup/restore 证据和既有 quarantine 均保留；旧失败运行目录可作为复盘证据，只能在逐项引用/敏感信息/可恢复性核对后移动到带 manifest 的 quarantine。不要使用 `git clean`、`git reset --hard`、`git checkout --` 或 `docker compose down -v`。

本轮 r6 清理按上述边界执行了可恢复移动；manifest 随历史清理归档已移到仓库外：`frontend/.tsbuildinfo`、`frontend/node_modules/.vite`、未被当前文档引用的 r6/r7 失败中间日志和旧 `.playwright-cli` 快照，共 `52` 个文件、`1,665,266` bytes。移动前确认本轮测试/构建消费者不存在、`8000/3000/11086/19999/9725/9999/5002` 均无监听、目标不是 reparse point；移动后逐项确认源路径不存在、quarantine 路径存在并回读哈希。当前 r8、coverage、backup/restore、Native/ThingsVis 源码、`automation_tests/test-results`、verification 和数据库/运行态敏感证据未移动。

### 11.9 前端 coverage 复测的两个可复现陷阱

1. `pnpm test:coverage -- --coverage.reportsDirectory=...` 会把额外参数继续包成一个 `--`，本项目的 pnpm/Vitest 展开结果没有采用该目录，coverage 会落到默认 `frontend/coverage`。需要固定输出目录时，直接调用 `pnpm exec vitest run --coverage --maxWorkers=1 --minWorkers=1 --no-file-parallelism --coverage.reportsDirectory=<absolute-dir>`，并在运行前核对产物目录和实际命令。
2. 不要把 stdout/stderr 重定向文件放在 Vitest 的 coverage reports directory 内。Vitest 会先清空该目录，Windows 上重定向文件仍被占用时会报 `EBUSY unlink ...coverage.stdout.log`，此时用例根本没有开始执行。日志必须放在旁边的独立目录；先确认报告目录为空，再启动 Vitest。

本轮按上述修正后的直接 Vitest 命令通过：`405/405` Test Files、`3575/3575` tests，statements/lines `77.29%`、branches `77.55%`、functions `66.79%`，退出码 `0`。fresh coverage、`lcov.info` 和 stdout/stderr 已移到仓库外 quarantine。stderr 有 Vue warning/错误路径诊断，但 `0xC0000005`、`EPIPE` 和 `Unhandled Error` 命中均为 `0`。

## 2026-08-15 上传前清理状态

本轮按用户指示停止重复完整 r14 回归，转为部署前和 GitHub 上传前清理。历史 `_localrun`、verification 归档、构建/coverage、运行态配置、认证状态、日志、截图和二进制已移到仓库外可恢复 quarantine；当前清单为本机父目录下的 `_aetherlink-github-cleanup-quarantine-20260815/github-cleanup-manifest-20260815.json`，r14-pre 清单为 `_aetherlink-cleanup-quarantine-20260815-r14-pre/quarantine-manifest.json`。两者均 `permanentDelete=false`，没有执行 `git push`。

清理后 generated-artifact 候选为 `0`，tracked 敏感边界扫描未发现明文密码、私钥标记或凭据 URI；`node_modules` 仅保留在本机依 lockfile 重建。r14 后端 Go 测试因依赖下载超时未完成，broker 测试按用户要求停止，均不得标记为通过。真实 RDI、目标环境、HTTPS/TLS、公网 MQTT、目标 backup/restore 和外部 ThingsVis 仍是 `pending`/`external-blocked`。

## 11.10 2026-08-14 r11c 当前复测、ThingsVis 保留边界与清理结果

本节是当前部署前方法的优先记录；11.8 及更早章节的 r8/r9c/r10 数字只用于复盘。

### 当前证据和方法结果

- API/E2E 使用独立的 `_localrun/predeploy-retest-20260814-r11c/`、verification 和 report 目录，fresh 结果为 aggregate `84/84`、API `64/64`、E2E `20/20`、`0 failed`、endpoint `372/372`、page/route `56/56`。证据分类为 business `28/28`，严格 `businessClosureEvidence.business=27/27`；其余 `57/57` 是非业务证据，不能合并成全业务闭环。
- JSON 证据复核必须先识别 UTF-8 BOM、UTF-16LE BOM 和普通 UTF-8，再交给 JSON parser；本轮扫描当前 r11c、patched synthetic、低内存 build 和 quarantine manifest 共 `292` 个 JSON，按编码识别后 `292` 可解析、`0` 无效。直接用 Node `JSON.parse` 或 Windows PowerShell `ConvertFrom-Json` 可能把 BOM/UTF-16 或 Mochawesome report 误报为损坏，不能因此重跑或删除证据。
- 针对性合同复测：`node scripts/test_synthetic_rdi_protocol_validation.js` 为 `8 passed`；`npx mocha tests/00_synthetic_rdi_fixture_contract.test.js --reporter spec` 为 `8 passing`；`npm.cmd run preflight:release` 为 `10/10 pass` 且最终 JSON `ok=true`。其中 release preflight 的 Docker Compose、漏洞库、SBOM、托管依赖审查等运行时项目仍明确是 skipped/not-run，不能扩大解释为目标环境通过。
- Visualization 的 3 个场景仍必须记录为 `runtime-external / seedable=false / external-blocked`：ThingsVis service connection refused `127.0.0.1:8000`、negative-menu 同一外部 service 不可达、mirror ID 未配置。没有创建 mock 或用 Native route fulfill 消除 skip。
- `run_synthetic_rdi_lane.ps1` 已增加两道防复发门：启动前只为明确的隔离数据库执行 `CREATE DATABASE`，不 drop/覆盖默认库；启动前强制校验 runner、GMQTT listener、AetherLink broker target 的端口一致，并输出脱敏的 `raw/database-provision.json` 与 `raw/broker-port-contract.json`。数据库密码仅通过当前进程环境变量注入，不写 README、报告、manifest、日志、dump 清单或部署包。
- patched synthetic lane 的原始证据已移到仓库外 quarantine，只证明 `protocol-emulator -> isolated GMQTT -> backend -> API/SQL`。manifest 必须保持 `fixture_provenance=synthetic-rdi`、`real_rdi_status=not-tested`、`production_signoff=not-ready`。
- 前端 typecheck、data-architecture typecheck、packages typecheck 和 coverage 通过。原始完整 production build 为 `-1073741819` / `0xC0000005`；低内存诊断命令 `pnpm.cmd exec vite build --outDir dist-r11c-lowmem` 在 `NODE_OPTIONS=--max-old-space-size=2048` 下退出 `0`，原始产物和摘要已移到仓库外 quarantine。记录为“低内存本地构建通过、标准本轮构建访问冲突”，不能写成目标部署已通过。
- 默认数据库仍需按目标环境复核：当前只读 `aetherlink_iot` 为 `sys_version=45`、`version=0.0.23`、`devices=11`；源码迁移 `29.sql`–`48.sql` 连续；隔离 r11c 数据库 readback 为 `48`。隔离库不能替代默认目标库，local backup/restore 不能替代目标环境灾备演练。

### 清理决策

- 保留 `frontend/src/service/visualization-provider/`、ThingsVis components/hooks/utils/views/API、`deploy/docker-compose.optional-integrations.yml`、`frontend/nginx.thingsvis.conf`、测试合同和所有成功证据。Native 是默认 local provider；ThingsVis 是 legacy compatibility provider，不能因 Native 可运行而删除。
- `negative-menu` 是 ownership rejection 的负向测试场景，不是服务、容器或镜像，不清理。类似的 optional service 未启动、mirror ID 缺失、Market/SMTP/地图未配置，都应保留能力合同并记录 `external-blocked`，不创建假服务。
- 本轮仅移动两个已确认无引用、无活动进程、非 reparse、可由同一命令重建的失败 synthetic 中间目录到项目外 quarantine：`_localrun/synthetic-live-20260814-r11c` 与 `...-rerun`。详细哈希和回移规则记录在项目外 quarantine manifest 中。没有永久删除、没有 `git clean/reset --hard/checkout --`、没有 `docker compose down -v`。

### 当前最终门禁

```text
native_core_status         = local-core-verified
synthetic_software_status  = software-path-passed / partial-current
thingsvis_optional_status  = external-blocked / optional-disabled
real_rdi_status             = not-tested
target_database_status      = pending (default database still sys_version 45)
target_deployment_status    = pending
production_signoff          = not-ready

## 11.12 2026-08-14 r13 fresh quality/runtime/dev refresh

本节是当前执行方法和 fresh 结果。它把“源码质量”“生产构建”“隔离软件运行路径”“本地 dev server”和“目标环境/真实设备门禁”分开记录，不能把其中一层的通过扩展成 release-ready。

### r13 使用的不可复用输出

| lane | 输出/运行态 | 结果 |
| --- | --- | --- |
| 主前端 typecheck | `vue-tsc --noEmit --skipLibCheck --incremental false` | exit 0 |
| data architecture | `pnpm.cmd run typecheck:data-architecture` | exit 0 |
| workspace packages | `pnpm.cmd run typecheck:packages` | exit 0；不能直接 `node scripts/typecheck-workspaces.mjs` |
| Vitest coverage | r13 历史目录已移到仓库外 quarantine | 405 files / 3575 tests，0 failed；832 source files，statements 77.30%、branches 77.55%、functions 66.80% |
| production build | r13 历史目录已移到仓库外 quarantine | 标准 4096 MB heap，exit 0；不覆盖 `frontend/dist` |
| full runtime | r13 历史目录已移到仓库外 quarantine | aggregate 84/84，API 64/64，E2E 20/20，endpoint 372/372，page/route 56/56，0 failed |
| database drill | r13 历史目录已移到仓库外 quarantine | pg_dump、new restore database、pg_restore、readback 全部 exit 0；source/restore `sys_version=48`, `version=0.0.23` |
| dev smoke | r13 历史目录已移到仓库外 quarantine | dev proxy health 200，login render 0，login E2E 12/12；结束后端口释放 |

### r13 不重复踩坑的执行顺序

1. 先确认没有 typecheck、Vitest、Vite、Backend、GMQTT、Playwright 进程，并读端口/PID/命令行；不因旧 agent 输出直接杀进程。
2. 前端命令从 `frontend` 目录执行，并使用 `pnpm.cmd`。主 typecheck 要显式加 `--incremental false`，避免把旧 `frontend/.tsbuildinfo` 当成 fresh 证据；data-architecture 和 packages 单独执行。
3. coverage 直接调用 `pnpm.cmd exec vitest run --coverage --maxWorkers=1 --minWorkers=1 --no-file-parallelism --coverage.reportsDirectory=<absolute-dir>`。日志必须放在 coverage 目录外；禁止使用嵌套 wrapper 转发临时 `--coverage.reportsDirectory`。
4. production build 使用显式绝对 `--outDir`，关闭 bundle report/trace；质量 lane 成功后，运行脚本必须消费同一个 fresh outDir，而不是静默消费旧 `frontend/dist`。
5. `predeploy_full_retest.ps1` 必须显式传 `-RunDir`、`-DatabaseName`、`-BrokerPort`、`-BackendPort` 和本轮新增的 `-PreviewDistDir`。脚本默认行为仍是 `frontend/dist`，传参只是保证 fresh lane 不消费旧产物。
6. PostgreSQL 密码只注入 `AETHERLINK_PREFLIGHT_DB_PASSWORD` 当前进程环境，不进入命令行参数、文档、日志、manifest、dump 名称或最终回复。隔离库只能 `CREATE DATABASE`，不对默认库做 destructive restore。
7. API/E2E 使用同一个隔离数据库、broker、backend、preview 和独立 report/auth/verification 目录；runner 结束后再查端口和进程，确认 `11092/19997/9725` 无监听。dev smoke 另用 `19996/9726`，并在 finally 中释放。
8. 最后独立读取 `summary.json`、`endpoint-coverage.json`、`page-coverage.json`、`coverage-final.json`、`lcov.info`、数据库 readback 和 backup/restore readback。`business 28/28` 仍要减去非业务证据口径，严格业务闭环本轮是 `27/27`。

### r13 ThingsVis 与清理边界

Native provider 通过不能替代 ThingsVis。当前 ThingsVis provider、旧 dashboard/project API、路由、iframe/SSO、menu/schema、optional Compose/Nginx 和合同测试均有活引用；`negative-menu` 是 ownership rejection 业务场景，不是独立服务。r13 仍将 ThingsVis 两项服务不可达和 mirror ID 缺失记为 `runtime-external / seedable=false` partial-skip，不允许通过 Native、simulation、generic emulator、mock 或 replay 升级为真实 ThingsVis。

没有发现可以直接删除的源码、迁移或 optional integration 模块。只有在运行态冻结、逐项核对引用、计算 SHA-256、确认可重新生成并建立 manifest 后，才可把 `.tsbuildinfo`、没有 authority 引用的旧 coverage/log、失败中间物等移到项目外 quarantine；不得使用 `git clean`、`git reset --hard`、`git checkout --` 或宽泛递归删除。成功 r13 证据、`frontend/build`、`frontend/dist`、`frontend/coverage`、`node_modules`、ThingsVis/Native 源码、迁移、数据库和外部部署资料继续 HOLD，直到最终清理策略完成。

真实 RDI PID/activation、voucher、硬件身份、固件 MQTT session、物理遥测/在线/ACK、生产 share/link/跨租户权限、真实 ThingsVis Server/Studio/SSO/mirror、HTTPS/TLS/反代/公网 MQTT、目标环境 backup/restore 仍是 `pending` 或 `external-blocked`，本地 dev 和 synthetic runtime 不改变这个结论。

## 2026-08-15 upload-boundary cleanup supplement

### Final upload-only closeout

This final closeout pass did not rerun frontend, backend, broker, database, API, browser, or E2E tests. It removed only the empty `automation_tests/reports/` and `automation_tests/e2e/.auth/` runtime directories; source, tests, migrations, deployment contracts, Native visualization, and the optional ThingsVis compatibility provider remain in the repository.

The offline publication gates were rerun against the current worktree: supply-chain `13/13` checks passed, generated-artifact candidates were `0`, the release preflight had `10/10` local checks pass, and `git diff --check` exited `0` with only existing CRLF normalization warnings. Docker Compose validation, vulnerability/advisory data, SBOM generation, hosted dependency review, and runtime API/E2E remain `not-run` or unavailable by design.

The final secret-boundary scan found `0` exact database-password hits, `0` private-key markers, `0` credential-bearing PostgreSQL URIs, and `0` tracked sensitive paths. No known test/runtime port remains listening. The release markers remain `github_upload=not-executed`, `real_rdi_status=not-tested`, `target_deployment_status=pending`, and `production_signoff=not-ready`.

The incomplete validation run `../_aetherlink-validation-20260815-r16` was not promoted to evidence. Without rerunning any test, build, service, API, browser, or E2E lane, its 2,074 files (78,505,657 bytes) were moved as one recoverable unit to `../_aetherlink-github-cleanup-quarantine-20260815-r3/`; the companion manifest records `allMovedVerified=true` and `permanentDelete=false` after per-file SHA-256 comparison.

本轮按用户指示停止重复完整回归，转为部署前和 GitHub 上传前清理。除既有 quarantine 外，以下 7 个一次性历史/生成文件已移到项目外 `../_aetherlink-github-cleanup-quarantine-20260815-r2/`：自动化 Ready Check 历史说明、dated deployment readiness/audit、迁移哈希快照和 repository inventory。`github-cleanup-manifest-20260815-r2.json` 记录了源/目标相对路径、哈希和清理状态，最终 `allMovedVerified=true`、`permanentDelete=false`；空的 `_aetherlink-validation-20260815-r15` 目录已确认无文件后移除。

本轮未重新执行测试、编译、服务启动或 E2E。`automation_tests/verification/`、dated readiness/audit、migration hash 和 repository inventory 已加入 `.gitignore`；`verification/templates/`、公开文档、测试代码、模拟器源码、Native provider、ThingsVis compatibility provider 和 optional deployment contract 继续保留。该清理不改变 `real_rdi_status=not-tested`、`target_deployment_status=pending` 或 `production_signoff=not-ready`。

## 11.13 2026-08-14 r12 device-page fresh browser refresh (historical; superseded by r13)

本轮使用全新 `RunDir`、数据库和 broker/backend 端口执行 `predeploy_full_retest.ps1`，避免复用 r11c 的报告、auth state、数据库和运行态：r12 device-pages 原始目录已移到仓库外 quarantine，数据库 `aetherlink_iot_predeploy_retest_20260814_r12_device_pages`，GMQTT `11091`，Backend `19998`，Preview `9725`。原始报告和归档不属于当前 source package。

运行结果为 aggregate `84/84`、API `64/64`、E2E `20/20`、`0 failed`、endpoint `372/372`、page/route `56/56`；严格业务闭环证据为 `27/27`，不能把 `84/84` 或 `56/56` 解释成全产品业务闭环。`02_device.spec.js` 的 `8/8` browser cases 全部通过，覆盖六个目标页：物模型创建后搜索；分组创建、筛选和详情统计；service-access 权限拒绝和空状态；share 有效/无效/空 token；recipient 接受 share 后的 shared-with-me 列表。每个有状态用例均执行了 finally cleanup，隔离库中的 device/group/template/dynamic-recipient marker readback 均为 `0`。

本轮隔离库 `sys_version=48`，默认 `aetherlink_iot` 仍为 `sys_version=45`；29.sql–48.sql 连续性不变。结束后 `11091/19998/9725` 无监听。本轮仍是 `synthetic-rdi / partial-current`：它证明真实浏览器与隔离 Backend/GMQTT/API/SQL 的软件路径，不证明真实 RDI PID、voucher、硬件身份、固件 MQTT session、物理遥测、物理 ACK 或生产跨租户权限链。

### 11.11.1 本轮清理

在确认 r12 服务、Backend、broker、Playwright 和 frontend build/typecheck 进程均退出后，只做可恢复移动：

- `_localrun/device-pages-focused-20260814-agent`：之前 API `9999` 不可达、浏览器尚未启动的阻断中间物；
- `frontend/_localrun/predeploy-clean-20260813`：无当前 authority 文档引用的旧 frontend test/typecheck 日志，其中 typecheck 日志与已有 dated log 重复。

清单和 SHA-256 仍在项目外历史 quarantine 中。没有永久删除，没有使用 `git clean`、`git reset --hard`、`git checkout --` 或 `docker compose down -v`。`frontend/dist`、`frontend/dist-r11c-lowmem`、`frontend/coverage`、`.tsbuildinfo`、`node_modules`、verification、成功报告、数据库和 ThingsVis/Native 源码继续 HOLD。
```
