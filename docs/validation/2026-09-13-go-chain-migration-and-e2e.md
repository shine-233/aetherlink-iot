# Go 侧迁移全链与五组 E2E 运行期证据

日期：2026-09-13（执行） / 2026-09-15（整理归档）
目标：回答两个此前只靠"文件已写"宣称的问题——
1. 全新空库上，后端自己的 `initialize.CheckVersion` 迁移链到底能跑到几？
2. 路线图 §6 第一优先里"5 组自动化 E2E（0/5）"能不能真的跑起来？

结论先行：**迁移链跑到 99（此前口径是 93，被推翻）；5 组 E2E 全部真实通过（38/38，0 pending）。**

---

## 1. Go 侧迁移全链（此前口径错误，本轮实测修正）

### 1.1 背景：为什么需要单独验这一条

路线图 §1.1 原文写作：

> 该路径是 `psql` 直灌，与 Go 侧 `initialize.CheckVersion` 的全链路径**不等价**；后者仍只验证到 93。

`psql` 直灌能过不代表 Go 侧能过——两者的执行器、事务边界、版本登记方式都不同。
把前者当成后者，等于拿一条没验证过的路径宣称"迁移已全链验证"。

### 1.2 环境

| 项 | 值 |
| --- | --- |
| PostgreSQL | 17.5，隔离集群 `C:\Users\Zz\al_pg_fresh`，监听 `127.0.0.1:55433` |
| 认证 | `pg_hba.conf` 为 trust（仅本机验证用） |
| 数据库 | 全新 `aetherlink_go99`（执行前 `CREATE DATABASE`） |
| 后端配置 | `backend/configs/conf-localdev.yml`（`db.psql.port=55433`、`dbname=aetherlink_go99`） |
| 环境变量 | `AETHERLINK_TIMESCALE_MODE=off` |
| Go | go1.26.8 windows/amd64 |

### 1.3 执行

```bash
# 起隔离集群（PowerShell，否则沙箱会在命令结束时杀掉 postmaster）
& "C:\Program Files\PostgreSQL\17\bin\pg_ctl.exe" -D C:\Users\Zz\al_pg_fresh \
  -o "-p 55433 -c listen_addresses=127.0.0.1" -l C:\Users\Zz\al_pg_fresh\startup.log start

# 建全新库
psql -w -h 127.0.0.1 -p 55433 -U postgres -d postgres -c "CREATE DATABASE aetherlink_go99;"

# 由后端启动流程自行迁移（不手工灌 SQL）
cd backend
AETHERLINK_TIMESCALE_MODE=off go run . -config ./configs/conf-localdev.yml
```

启动日志关键行：

```
数据版本： 0
程序版本： 99
```

### 1.4 结果

```sql
SELECT * FROM sys_version;                       -- 99|0.0.23
SELECT count(*) FROM information_schema.tables
  WHERE table_schema='public';                   -- 122
```

| 指标 | 结果 |
| --- | --- |
| `sys_version` | **99**（`0.0.23`） |
| 表数 | **122** |
| 迁移错误 | 无（日志中无 `database migration failed`） |

**`go build -p 1 ./...` → exit 0。**

### 1.5 口径修正（重要）

- 此前"Go 侧只验证到 93（`sys_version=93`、114 张表）"的口径**已被推翻，不要再引用**。
- `psql` 直灌为 121 张表、Go 全链为 122 张表。**表数差异不是谁对谁错**——Go 路径会额外创建应用启动所需的表，但两条路径**不等价**，不能用前者顶替后者。
- 2026-09-15 复核时，`VERSION_NUMBER` 已推进到 **103**（`backend/pkg/global/global.go`），
  `sql/` 下已有 `100.sql`–`103.sql`。**94–103 这一段尚未在全新库上跑过 Go 侧全链**，
  本报告的"已验证"结论**只覆盖到 99**，不要外推。

### 1.6 踩到的坑（留给下一轮）

1. **旧库不能复用来做这项验证。** 先在已直灌过的 `aetherlink_verify` 上启动，直接报
   `database migration failed: ERROR: relation "alarm_config" already exists (SQLSTATE 42P07)`
   ——因为该库无 Go 侧版本登记，`CheckVersion` 从 0 开始重跑。必须建全新库。
2. `db.psql.password` **不能为空串**：`initialize.LoadDbConfig` 要求 `DbName`/`Username`/`Password` 三者非空
   （`pg_init.go:171`）。trust 认证下填任意非空值即可。
3. Bash 工具结束时会被 SIGTERM 并杀掉子进程树，`pg_ctl start` 起来的 postmaster 会随之消失
   （日志表现为 `database system shutdown was interrupted`）。**改用 PowerShell 启动**。

---

## 2. 五组自动化 E2E

### 2.1 前置：本地栈

| 组件 | 状态 |
| --- | --- |
| PostgreSQL 17（55433 / `aetherlink_go99`） | 已起，已迁移至 99 |
| Redis（`127.0.0.1:6379`，`--save "" --appendonly no`） | 已起，`PING` → `PONG` |
| 后端 API（`127.0.0.1:9999`） | 已起，`GET /health` → 200 |
| MQTT broker | **见 §2.4 的边界说明** |
| 账号 | `node scripts/prepare_local_accounts.js` 生成，写入 `automation_tests/.env.local` |

关键配置（`backend/configs/conf-localdev.yml`）：

- `secrets.active_key_id=k1` + `master_keys{k1}`（P0.7 AES-256-GCM 主密钥，**否则 AI 凭证写入 fail closed**）
- `market.active_bundle_signing_key_id=mk1` + `bundle_signing_keys{mk1}`
  （**否则打包导出拒绝出包**，41 组的两条往返用例只能 skip）
- `mqtt.enabled=true`

### 2.2 执行

```bash
cd automation_tests
set -a && . ./.env.local && set +a
node node_modules/mocha/bin/mocha.js \
  tests/38_edge_nodes.test.js tests/39_license_status.test.js \
  tests/40_telemetry_anomaly.test.js tests/41_market_bundle_import.test.js \
  tests/42_operation_logs_export.test.js --timeout 90000
```

### 2.3 结果

**38 passing / 0 pending / 0 failing**

| 组 | 覆盖 | 用例数 |
| --- | --- | --- |
| 38 `edge_nodes` | 注册、幂等重注册、跨租户抢注拒绝、列表、心跳、reconcile（含资源类型白名单与版本闸门）、未注册节点、非本租户网关 | 10 |
| 39 `license_status` | 平台管理员状态视图、边界未启用时的自解释、不回传许可证材料、仅有效时给指纹、非管理员拒绝、未认证拒绝 | 6 |
| 40 `telemetry_anomaly` | bounds/deviation 规则、空窗报"无数据"而非"无异常"、参数校验、单设备失败不中断整请求 | 11 |
| 41 `market_bundle_import` | **未签名包即使只预览也拒**、篡改拒、无载荷拒、预览不落库、同签名包重导幂等且不要求覆盖确认 | 5 |
| 42 `operation_logs_export` | 时间窗必填/顺序/超一年、空窗拒、CSV 导出、**不导出 request/response 载荷列** | 6 |

### 2.4 MQTT 边界（如实记录，不要外推）

本机无 Docker，且 Go module proxy 不可达：

```
go: downloading ...: Get "https://proxy.golang.org/...": Bad Gateway
```

因此**无法构建仓库自带的 gmqtt broker**。为让遥测类 E2E 能执行，本轮新增
`automation_tests/scripts/local_mqtt_broker.js`——一个最小 MQTT 3.1.1 回环夹具
（仅 CONNECT / SUBSCRIBE / PUBLISH QoS 0·1 / PINGREQ / DISCONNECT，无鉴权、无持久会话、无 QoS 2、无 retained）。

它的定位是**测试替身，不是生产组件**，也不替代 gmqtt。由此产生两处必须记住的取舍：

1. **gmqtt 的 aetherlink 插件会按认证身份把设备上行载荷转成内部信封**
   `{ device_id, values }`（`internal/adapter/mqttadapter` 的 `verifyPayload` 要求该形状）。
   最小夹具没有这层转换，直接转发的结果是后端报 `device_id cannot be empty`。
   → 解决方式是在 `lib/seed_data.js` 增加 `uplinkEnvelope` 选项，
   由调用方按平台自己声明的上行契约发送（`values` 为 `[]byte`，`encoding/json` 走 **base64 字符串**）。
   默认行为不变，只有显式开启才改写，避免影响 03/12/22/27 组。
2. **以下项仍必须在真实 gmqtt 上复验**，本证据不构成其证明：
   P0.2 的 `shadow_ack` 端到端、22 组 MQTT 设备管道、持久会话与 QoS 2 行为、broker 侧鉴权。

### 2.5 本轮修掉的用例缺陷（区分"实现问题"与"用例写错"）

| 组 | 现象 | 判定 | 处置 |
| --- | --- | --- | --- |
| 38 | 列表端点返回数组，`expectOk` 却断言 `data` 是对象 | **用例写错** | 新增 `expectOkPayload`，不再要求响应必须裹对象 |
| 38 | 断言 `device_config` 可被 reconcile | **用例先于实现的假设** | 改为断言"白名单外类型被拒"，并新增该负向用例；reconcile 主体改用 `rule_chain` |
| 38 | 断言"版本不兼容 → `blocked=true`" | **语义理解错** | 实测：版本闸门把计划项标为 `action=skip`，`blocked` 仅表示存在 `needs_attention`。改为断言"无任何项被下发 + `synced` 与 sync 项数一致" |
| 39 | 断言非管理员返回业务码 `201001`，实际拿到 `403` | **用例写错** | 越权由 Casbin 中间件在统一响应封装**之前**短路（body 为 `{"error":"非法访问"}`）。改为断言"被拒且码 ∈ {403, 201001}"，并注明这是全局中间件行为 |
| 41 | 两条往返用例恒 pending | **空租户无模板** | 导出返回 `100404 no templates for this type_key`（空态，非签名失败）。用例内先按导入契约建一条模板，往返路径才真正执行 |

> 附带发现（记录但不本轮改）：越权响应走裸 HTTP 403、业务错误走统一信封，
> 两种拒绝形态并存，与 §4"四面一致"有张力。若日后统一，39 组断言需同步改。

---

## 3. 前端侧

| 项 | 结果 |
| --- | --- |
| `views/market/browse/__tests__/bundle-import-model.test.ts` | **30 passing** |
| `views/market/browse/__tests__/index.test.ts` | **4 passing** |
| `vue-tsc --noEmit -p tsconfig.json` | **exit 0，0 error** |

挂载用例覆盖：导入入口存在、create/overwrite/blocking 三名单分离渲染、
未确认时提示需显式确认、未签名包被标记。

> 挂载测试的坑：本页依赖 `main.ts` 的全局 naive-ui 注册，vitest 下没有这层注册，
> 未解析组件的具名插槽（`#header` / `#footer`）会被整块丢弃，导入入口"消失"。
> 且真实 `NModal` 会 teleport 到 body，`wrapper.text()` 取不到闸门内容。
> 处理：挂载真实 naive-ui 插件，仅对 `n-modal` 打桩并保留 `show` 语义。

---

## 4. 本证据不覆盖什么

- 迁移 **100–103**（当时上界 99，之后新增的四条未跑）
- 真实 gmqtt 上的任何 MQTT 行为（见 §2.4）
- 浏览器 / Playwright 层面的 UI 验证（本轮只有 vitest 挂载）
- P0.1 部署门禁、P2.3 压测（环境阻塞）
