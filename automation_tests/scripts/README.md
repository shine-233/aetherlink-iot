# 自动化辅助脚本

本目录保存本地账号准备、预检、预览代理和预览页面可达性检查脚本。它们用于准备或验证环境，不应被当作业务测试本身。

## 目录定位

- 在 API 自动化和 Playwright E2E 前，提前暴露账号、URL、代理模式和预览页面问题。
- 普通脚本支持 release-style 本地预检，但 `predeploy_full_retest.ps1` 是明确例外：它负责启动本轮隔离的 GMQTT、Backend、Preview，并编排 strict preflight 与 broad API/E2E；只应使用显式隔离端口和数据库。
- 生成的本地 env 文件只服务当前机器，不能提交。

## 文件用途

- `prepare_local_accounts.js`：通过应用 API 创建或校验本地账号，并写入被忽略的本地 env 文件。
- `preflight_api_e2e.js`：检查占位账号、预览端口、API 目标、预览代理和 Playwright server 复用设置。
- `serve_preview_with_api_proxy.js`：在预览端口提供前端构建产物并代理 API 请求到后端。
- `verify_preview_9725.js`：检查预览端口返回 HTML。
- `verify_preview_login_render.js`：用 Playwright 检查登录页面能渲染。
- `generate_repository_inventory.js`：从 Git tracked、缺失 tracked 与未忽略 untracked 集合生成稳定的全仓文件台账；不扫描 ignored 依赖/运行产物，敏感候选只读取元数据。
- `check_supply_chain.js`：离线核对 Go module/lockfile、Docker builder、pnpm 版本和 frozen-lockfile 边界；漏洞数据库、完整 resolved/image SBOM 与托管审查保持外部状态。
- `generate_local_sbom.js`：不联网、不安装依赖，读取三份 `go.mod`、对应的三份 `go.sum` 和 `frontend/pnpm-lock.yaml`。`--source-only` 生成带完整输入哈希的 `source-manifest-only` JSON；不带该参数时，把仓库内声明与 Go/pnpm 锁定条目生成 `declared-and-locked-components` JSON。两种模式都不代表完整 resolved dependency 或 image SBOM。
- `check_generated_artifacts.js`：只读核对 `_localrun`、前端构建输出、缓存、验证归档和二进制的 Git tracked/ignored 边界；保留期限与归档内容审查保持外部状态。
- `visual-page-sweep.js`：在 Playwright 页面上下文中逐页检查页面、控制台错误和失败请求并写入截图；输出目录通过 `VISUAL_OUTPUT_DIR` 注入，默认写入 `verification/visual-page-sweep-<timestamp>/`，不再写入源码目录。
- `seed_synthetic_rdi_fixture.js`：只在显式允许的隔离 PostgreSQL 中创建、查看或清理 `synthetic-rdi` 预注册 fixture；支持 `--status`、`--seed`、`--cleanup --confirm`。它不是实体 RDI 设备，不能用于宣称真实 PID、固件、设备 MQTT 或真实 ACK 已通过。
- `activate_synthetic_rdi_fixture.js`：使用 `tenant_admin` 通过公开 `POST /api/v1/rdi/devices/activate` 激活已经由 seed 创建的 synthetic 预注册行，并严格回读 `active/enabled`；输出只包含非敏感 activation evidence，不能用于宣称真实 RDI 激活。
- `run_synthetic_rdi_lane.ps1`：编排 fresh synthetic seed、账号准备、公开 API activation、34 条 share/link API 合同、isolated GMQTT/backend 协议、SQL 回读、脱敏扫描和 SHA-256 归档；emulator 不执行 activation，整条 lane 仍只属于 `synthetic-rdi / protocol-emulator`。
- `predeploy_full_retest.ps1`：完整部署前隔离复测入口；启动显式端口的 GMQTT、Backend、Preview，准备本地账号和 synthetic fixture，执行 strict `preflight:api-e2e`，再运行 `node run_tests.js --include-e2e --workers=1 --archive` 并归档报告。它与只做配置/连通性检查、不会替你启动服务的 `preflight_api_e2e.js` 不同。
- `../deploy/tests/redis-memory-contract.test.sh`：核对共享 Redis 的 `maxmemory`、`noeviction` 和 AOF 配置边界；实际压力、拒写、重启和恢复验证仍需部署环境。

`seed_synthetic_rdi_fixture.js` 的安全边界：默认拒绝写入；必须显式设置 `AETHERLINK_SYNTHETIC_RDI_ALLOW=1` 或使用确认参数；只允许明确允许的 loopback PostgreSQL 端口，且目标数据库名必须逐字出现在 `AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES` 提供的逗号分隔白名单中。数据库名包含 `local`、`test` 或 `isolated` 不能单独获得写入权限；cleanup 只能删除带有正确 `fixture_provenance=synthetic-rdi` 的记录。若某条 lane 选择 cleanup，必须在 cleanup 后再次执行 `--status` 并得到 `absent`；当前 `run_synthetic_rdi_lane.ps1` 默认保留 fixture 供后续复核，并在 manifest 中明确记录 `retained-for-follow-up`。数据库密码可以通过环境变量提供，但不能写入 README、测试文档、日志、JSON、备份清单或部署包。

## 2026-08-15 上传前清理状态

历史 `_localrun`、verification 归档、构建/coverage、运行态配置、认证状态、日志、截图和二进制已经移到仓库外可恢复 quarantine；这里保留方法和结果摘要，不再保留指向仓库内不存在目录的链接。当前清单是本机父目录下的 `_aetherlink-github-cleanup-quarantine-20260815/github-cleanup-manifest-20260815.json`，r14-pre 清单为 `_aetherlink-cleanup-quarantine-20260815-r14-pre/quarantine-manifest.json`，两者均未永久删除。

本轮没有重复完整 r14：后端 Go 测试在依赖下载超时处停止，broker 测试按用户要求停止，均不得标记为通过。清理后生成物候选为 `0`；tracked 敏感边界扫描未发现明文密码、私钥标记或凭据 URI。`node_modules` 只留在本机依 lockfile 重建，不进入公开 source package。真实 RDI、目标环境和外部 ThingsVis 仍按 `pending`/`external-blocked` 记录。

执行 synthetic-rdi API/E2E lane 时，除了 seed 脚本的 opt-in，还必须把同一个 PID 显式传给 automation harness；否则 harness 会 fail closed，不会猜测或把普通设备当成 RDI：

```powershell
$env:GOTP_DB_PSQL_HOST='127.0.0.1'
$env:GOTP_DB_PSQL_PORT='55432'
$env:GOTP_DB_PSQL_DBNAME='aetherlink_iot_local'
$env:AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES='aetherlink_iot_local'
$env:AETHERLINK_SYNTHETIC_RDI_ALLOW='1'
$env:AETHERLINK_RDI_FIXTURE_MODE='synthetic-rdi'
$env:AETHERLINK_RDI_FIXTURE_PID='SYNTHRDI0001'
$env:SYNTHETIC_RDI_PID='SYNTHRDI0001'
node .\scripts\seed_synthetic_rdi_fixture.js --seed --confirm
```

## 验证命令

```powershell
cd automation_tests
node -c .\scripts\preflight_api_e2e.js
node -c .\scripts\prepare_local_accounts.js
node -c .\scripts\serve_preview_with_api_proxy.js
node -c .\scripts\verify_preview_9725.js
node -c .\scripts\verify_preview_login_render.js
npm run sbom:local
```

`npm run sbom:local` 仅生成本地 source-manifest SBOM，默认写入仓库内的验证/忽略输出路径；可直接调用脚本并用 `--output <仓库内路径>` 指定临时输出。若要检查发布 workflow 使用的声明/锁定组件模式，可直接调用脚本而不传 `--source-only`；该模式仍不会联网解析完整 Go module graph 或生成 image SBOM。`prepare_local_accounts`、`serve_preview_with_api_proxy` 和预览检查脚本在真实执行时会访问本地服务；只做文档或静态审查时优先使用 `node -c`。

## 推荐顺序

如果目标是尽快拿到 release API/E2E 闭环证据，推荐按这个顺序跑：

前提：frontend/dist 已由 frontend/pnpm build 生成；automation_tests/.env.example 可作为本地 automation 环境变量模板。

1. 本地开发可先运行 `npm run preflight:local`：它要求已有 `frontend/dist` 和真实后端，自动启动并关闭一次性 preview proxy，只执行 `local-lite` 配置/连通性检查，不验证 release 账号，也不构成发布证据。
2. 完整发布验证先运行 `npm run preflight:api-e2e`，看清楚当前缺的是账号、`PREVIEW_PORT` 对应的预览代理（默认 9725）、`API_TARGET`，还是 preview/backend 连通性；该严格入口不会替你启动服务。
3. 如果缺本地 release 账号，先运行 `node .\scripts\prepare_local_accounts.js`，按输出写入忽略掉的本地 env 文件。若后端已经初始化并存在 super admin，请先提供 `SUPER_ADMIN_EMAIL` / `SUPER_ADMIN_PASSWORD`，否则该脚本不会替你猜现有管理员账号。
4. 再确认：`PREVIEW_URL`、`FRONTEND_URL` 都指向 `PREVIEW_PORT`（默认 `http://127.0.0.1:9725`；r8 实际使用 `9725`/Backend `19999`；若未来并行隔离运行使用 `19725`，必须同时设置 `PREVIEW_PORT=19725`），`API_TARGET` 指向真实后端 origin，并设置 `PLAYWRIGHT_USE_PREVIEW_PROXY=1`、`PLAYWRIGHT_REUSE_EXISTING_SERVER=0`。
5. 启动或检查 `serve_preview_with_api_proxy.js` 对应的 `PREVIEW_PORT` 预览代理，再重新执行严格 preflight。
6. 最后再进 Playwright E2E 或 release API 证据采集，不要跳过严格 preflight 直接跑。

推荐的证据 lane 顺序是：

1. static/config preflight；
2. 普通 `simulation` telemetry（seed、publish、readback 必须使用同一个 device id）；
3. `generic-emulator` command path；
4. 隔离 `synthetic-rdi` API/share/link contract；
5. full API/E2E aggregate；
6. fixture cleanup 和 `status=absent`；
7. 真实 PID 可用时才执行 `real-rdi` lane。

backend、broker 和 synthetic fixture 必须指向同一个隔离数据库；1883 TCP 可连接不等于 MQTT credential 认证和 publish 成功。页面 route hit、endpoint coverage、preflight 和静态 contract 都不能单独升级为业务闭环。

2026-08-11 定向证据属于 historical；当前 r8 已另行归档 `e2e/20_command_jobs.spec.js` generic emulator `6/6`。报告仍须分别归档，不能把 emulator ACK 或 synthetic share/link 解释成真实 RDI 设备证据。
## 审查发现

- 环境准备脚本和验证证据容易混在一起；必须先预检，再运行，再归档。
- 本地账号输出含敏感信息，必须继续放在 `.local/` 或 `.env.local` 这类忽略文件中。
- 预览可达不等于业务正确，只能说明前端入口可访问。
- 当前 page/route report 的 `56/56` 只说明路由被访问；`/device/grouping`、`/device/grouping-details`、`/device/service-access`、`/device/share`、`/device/shared-with-me`、`/device/thingsmodel` 必须另外记录 API 回读、业务状态、权限、错误态和清理断言。即使 synthetic-rdi 页面断言通过，也只能标为 `synthetic-rdi / partial-current`。

## 2026-08-14 r8 fresh protocol rerun（historical；已由下方 r11c 覆盖）

本轮使用 `run_synthetic_rdi_lane.ps1` 在 `aetherlink_iot_predeploy_retest_20260814_r8` 上重新执行，显式设置了 `HEALTH_URL=http://127.0.0.1:19999/health`；原始证据目录已移到仓库外 quarantine。结果为 share/link focused API `1/1`、success/failure ACK 均通过、两条 `offline -> online -> offline`、fresh `temperature_1=25.5`、SQL readback、offline manifest/session/replay 和 secret scan 通过。

此 lane 的 PID、voucher、hardware serial、MQTT session、telemetry、online state 和 ACK 全部是 `synthetic-rdi / protocol-emulator`。manifest 固定 `device_execution=not-proven`、`real_rdi_status=not-tested`、`production_signoff=not-ready`；不得把它们解释成真实 RDI 或生产 sign-off。lane 结束后应确认 `11086/19999` 无监听和本轮进程已退出。

ThingsVis 不属于可清理的测试缓存：Native 是默认本地 provider，ThingsVis 是 optional legacy compatibility provider；`negative-menu` 是 ownership rejection 的测试标签，不是独立服务。ThingsVis `8000/3000` 与 mirror ID 缺失时保持结构化 external-blocked，不要用 Native、mock 或 simulation fixture 消除 skip。

## 2026-08-14 r11c 当前复测记录

当前 full API/E2E 证据必须以 `_localrun/predeploy-retest-20260814-r11c/` 为准：aggregate `84/84`，API `64/64`，E2E `20/20`，`0 failed`，endpoint `372/372`，page/route `56/56`。证据分类为 `business 28/28`，严格 `businessClosureEvidence.business=27/27`；`boundary/catalog/preflight/page` 通过不能升级为业务闭环。

> 口径提示：endpoint catalog 当前为 373 条；本节归档证据生成时为 372 条。引用前先核对当前 catalog 与归档批次的条目差异。

Visualization 保留 3 个结构化 `runtime-external / seedable=false` partial-skip：ThingsVis `127.0.0.1:8000` 不可达、negative-menu 依赖同一 ThingsVis 服务、`THINGSVIS_MIRRORED_DASHBOARD_ID` 未配置。Native 是默认 local provider，不能替代 ThingsVis legacy compatibility provider；`negative-menu` 是 ownership rejection 测试场景，不是服务，不能清理或用 mock 消除 skip。

patched synthetic lane 的原始证据已移到仓库外 quarantine。它通过 synthetic activation、focused share/link、success/failure ACK、遥测、online/offline、SQL readback、offline manifest/session/replay 和敏感扫描；必须继续标记 `fixture_provenance=synthetic-rdi`、`real_rdi_status=not-tested`、`production_signoff=not-ready`。

`run_synthetic_rdi_lane.ps1` 的复测前置要求：隔离数据库不存在时只执行 `CREATE DATABASE`；runner、GMQTT listener 和 backend broker target 端口必须逐字一致；密码只通过当前进程环境注入；结束后检查进程和端口，报告 `database-provision.json`、`broker-port-contract.json`、`sensitive-scan.json`。本轮低内存前端 build 的原始结果已移到仓库外 quarantine。

本轮已把两个失败 synthetic 中间目录移到项目外 quarantine；清单随项目外历史归档保留。成功 lane、源码、依赖树、数据库和 ThingsVis/Native 合同均保留。

## 2026-08-14 r13 fresh quality/runtime/dev refresh

当前 fresh 复测以 r13 历史批次摘要为准，原始目录已移到仓库外 quarantine：aggregate `84/84`、API `64/64`、E2E `20/20`、`0 failed`、endpoint `372/372`、page/route `56/56`，严格 `businessClosureEvidence.business=27/27`。Visualization 仍保留 3 个 `runtime-external / seedable=false` partial-skip：ThingsVis `127.0.0.1:8000`、negative-menu 同一外部服务和未配置 mirror ID。

本轮 `predeploy_full_retest.ps1` 新增可选参数 `-PreviewDistDir`，默认仍为 `frontend/dist`；fresh build lane 应显式传入本轮绝对 outDir，避免 preview/predeploy 消费旧构建产物。仍必须显式传入新的 `-RunDir`、数据库名和 broker/backend 端口，密码只放当前进程环境。

前端 fresh 质量证据的原始 coverage、build 和日志目录已移到仓库外 quarantine；覆盖率为 405/405 files、3575/3575 tests、832 source files，statements 77.30%、branches 77.55%、functions 66.80%。主 typecheck 使用 `--incremental false`，packages typecheck 必须经 `pnpm.cmd run typecheck:packages`。

数据库 readback 为隔离库 `sys_version=48`、`version=0.0.23`；local pg_dump/pg_restore 和 dev smoke 的原始结果已移到仓库外 quarantine。这不能替代默认目标库或目标机灾备演练；历史 dev proxy health 为 200、login render 通过、login E2E `12/12`，结束后 `19996/9726` 已释放。

ThingsVis 不清理：Native 是默认本地 provider，ThingsVis 是可选 legacy compatibility provider；`negative-menu` 是 dashboard-menu ownership rejection 测试场景，不是服务。源码、optional Compose/Nginx、旧 SQL/schema、provider/API/route 和合同测试都有活引用。可清理对象仅限冻结后逐项审计、无当前引用且可恢复重建的缓存/旧中间物，必须带 SHA-256 manifest；不得把 synthetic、simulation、generic emulator 或 replay 写成 real-RDI。

## 2026-08-14 r12 device-page fresh browser refresh (historical; superseded by r13)

本轮使用新的 r12 device-pages 隔离运行目录（原始目录已移到仓库外 quarantine）、数据库 `aetherlink_iot_predeploy_retest_20260814_r12_device_pages`、GMQTT `11091` 和 Backend `19998` 执行完整 runner。结果为 aggregate `84/84`、API `64/64`、E2E `20/20`、`0 failed`、endpoint `372/372`、page/route `56/56`；`e2e/02_device.spec.js` 为 `8/8` passed。它新增了六个页面的 fresh browser evidence：`grouping`、`grouping-details`、`service-access`、`share`、`shared-with-me`、`thingsmodel`。

页面证据必须按类型读取：grouping/grouping-details 有创建、筛选、点击、详情统计和删除；service-access 有权限拒绝 API/浏览器响应和空状态，但没有创建 service 状态；share 有 valid/invalid/empty token、retry、accept/public 回读、revoke 和 fixture cleanup；shared-with-me 有 recipient context、首次/重复 accept、列表回读和动态账号 cleanup；thingsmodel 有创建、Search、响应/页面回读和模板删除。该批次使用 `synthetic-rdi` fixture，只能写成 `synthetic-rdi / partial-current`，不能写成真实 RDI 设备证据。

本轮隔离数据库 readback 为 `sys_version=48`，默认数据库仍为 `sys_version=45`；结束后 `11091/19998/9725` 无监听，测试临时 device/group/template/recipient marker 均为 `0`。Visualization 的三个 ThingsVis external-blocked 场景仍保留；Native 和这轮 device 页面通过都不能消除 ThingsVis/real-RDI/target-deployment 门禁。

本轮只将阻断的 focused 中间物和无当前引用的旧 frontend local 日志可恢复移动到项目外 quarantine。ThingsVis 源码、optional compose/nginx、simulation/emulator 源码、frontend build/coverage/dist、node_modules、验证归档和成功报告均保留。

## 重构/清理建议

- 保持账号准备、预检、预览代理、报告归档四个步骤清晰分离。
- 把失败原因写成明确 exit code 和中文说明，便于归档后复盘。
- 新增脚本时同步补中文四字段文件头和本 README 的用途说明。
