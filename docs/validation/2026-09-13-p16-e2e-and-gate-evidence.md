# 2026-09-13 P1.6 打包导入闸门 + 5 组 API E2E 运行期证据

> 本文覆盖路线图 §6「第一优先」的剩余三项：打包导入 UI 接线、5 组自动化 E2E、
> Go 侧全链迁移。所有结果均为本次实际执行，命令与原始输出在文中留痕。

## 1. 环境

| 组件 | 值 | 说明 |
| --- | --- | --- |
| PostgreSQL | 17.5，隔离集群 `C:\Users\Zz\al_pg_fresh`，端口 **55433** | trust 认证；数据目录是既有集群，非生产 |
| Redis | `C:\Program Files\Redis\redis-server.exe`，端口 6379 | 无持久化（`--save "" --appendonly no`） |
| 后端 | `go run . -config ./configs/conf-localdev.yml`，端口 9999 | `AETHERLINK_TIMESCALE_MODE=off` |
| 前端 | vitest 4.1.10 / vue-tsc | 仅静态与单测，未跑浏览器 |
| 数据库 | `aetherlink_go99`（本次新建，空库） | 用于 Go 侧迁移链验证 |

`conf-localdev.yml` 由 `conf-dev.yml` 派生，本机专用、不入库（`.gitignore` 已覆盖）。
其中两项密钥是 fail-closed 开关，不配置会让相关路径直接拒绝：

- `secrets.master_keys`（P0.7 AES-256-GCM 信封加密主密钥）
- `market.bundle_signing_keys`（P1.6 打包 HMAC 签名密钥）

## 2. Go 侧迁移链 1→99（此前只验证到 93）

路线图 §1.1 原表述："该验证只到 93，94–99 未做同等全链验证"，且明确指出
"`psql` 直灌与 Go 侧 `initialize.CheckVersion` 的全链路径**不等价**"。
本次在空库上跑了 Go 侧路径：

```
CREATE DATABASE aetherlink_go99;
AETHERLINK_TIMESCALE_MODE=off go run . -config ./configs/conf-localdev.yml
```

结果：

```
数据版本： 0
程序版本： 99
...
select * from sys_version;          -> 99|0.0.23
select count(*) from information_schema.tables where table_schema='public';  -> 122
```

**结论：Go 侧 `CheckVersion` 全链 1→99 通过，`sys_version=99`。**
与 psql 直灌路径（121 张表）相差 1 张表，来源未逐条比对，
因此两条路径仍不可混用同一个"表数"口径，但"Go 侧只能到 93"的说法已被推翻。

## 3. 打包导入 UI 接线（P1.6）

此前 `views/market/browse/index.vue` 走 `device/template/import`（单模板回放），
后端 P1.6 的验签 + 预览 + 覆盖闸门在前端 **0 处引用**，是死门。本次：

- 新增纯模型 `src/views/market/browse/bundle-import-model.ts`
- `service/api/market.ts` 新增 `importMarketBundle`（`preview` / `confirm_overwrite`）
- 页面改为：上传 → 只读预览 → 三名单（create / overwrite / blocking）分开渲染
  → 覆盖项需勾选确认才启用导入按钮 → 导入
- 四语 i18n 各 20 键（en-us / zh-cn / es-es / fr-fr）

**刻意不做的事**：前端不验签。签名密钥在服务端，前端只做"包里有没有签名三字段"
的预检，决策值命名为 `unsigned` 而非 `valid`，避免把预检说成验签通过。

测试：

```
node node_modules/vitest/vitest.mjs run src/views/market/browse/__tests__
 ✓ bundle-import-model.test.ts (30 tests)
 ✓ index.test.ts (4 tests)
 Test Files  2 passed (2)      Tests  34 passed (34)
```

`vue-tsc --noEmit -p tsconfig.json` → exit 0。

## 4. 5 组 API E2E（38 / 39 / 40 / 41 / 42）

```
cd automation_tests
node scripts/prepare_local_accounts.js        # 生成本地账号 -> .env.local
set -a; . ./.env.local; set +a
node node_modules/mocha/bin/mocha.js tests/38_edge_nodes.test.js \
  tests/39_license_status.test.js tests/40_telemetry_anomaly.test.js \
  tests/41_market_bundle_import.test.js tests/42_operation_logs_export.test.js \
  --timeout 90000
```

结果：**38 passing / 0 pending / 0 failing**。

| 文件 | 结果 |
| --- | --- |
| 38_edge_nodes | 8 passing |
| 39_license_status | 6 passing |
| 40_telemetry_anomaly | 11 passing |
| 41_market_bundle_import | 5 passing |
| 42_operation_logs_export | 6 passing |

首轮跑出 21 passing / 2 pending / 4 failing，逐个定位后修的是**用例自身的错误断言**，
不是放宽门禁：

1. **38 列表断言**：`GET /edge/nodes` 直接返回数组，`expectOk` 却强断言 `data` 是对象。
   改为 `expectOkPayload`。
2. **38 reconcile 资源类型**：用例用 `resource_type: 'device_config'`，而后端白名单只有
   `dashboard / rule_chain`（`model/edge_node.go` 的 `oneof`）。改为用 `rule_chain` 跑通，
   并**新增一条用例**守住"白名单外类型必须被拒"。
3. **38 reconcile 版本闸门语义**（重要）：`blocked` 仅表示存在 `needs_attention`（冲突），
   **不覆盖版本闸门**；版本不兼容时每项标 `action=skip` 且 `synced=0`。
   原断言"版本不兼容 ⇒ blocked=true"是错的，改为断言"没有任何一项被下发"，
   并新增 `synced === sync 项数` 的一致性断言。
4. **39 越权码**：非平台管理员访问 `/license/status` 由 Casbin 中间件在统一响应封装
   **之前**短路，返回裸 HTTP 403（`{"error":"非法访问"}`），不是业务码 201001。
   改为断言"非 200 且属于 {403, 201001}"。*顺带发现*：全局中间件与业务错误码不统一，
   是"四面一致"的一处既有缺口，未在本次改动。
5. **41 往返路径此前恒 skipped**：空租户没有模板，导出返回 `100404 no templates for
   this type_key`（空态，非签名失败），导致"预览"与"幂等重导"两条从未真正执行。
   现在 `before` 先按导入契约建一条 `automation` 模板，两条往返路径已实测通过。

## 5. MQTT 通道的性质与边界

本机无 Docker，且 Go module proxy 不可达（`proxy.golang.org` 返回 Bad Gateway），
**仓库自带的 gmqtt broker 无法构建**。遥测入库的唯一通道是 MQTT，因此新增
`automation_tests/scripts/local_mqtt_broker.js`——一个最小 MQTT 3.1.1 回环 broker。

**它是什么**：仅实现 CONNECT / SUBSCRIBE / PUBLISH(QoS 0,1) / PUBACK / PINGREQ /
DISCONNECT 的测试夹具。自检通过（订阅端确实收到发布内容）。

**它不是什么**：不是 gmqtt 的等价物。没有鉴权、持久会话、QoS 2、retained、集群、TLS，
也**没有 aetherlink 插件的载荷转换**。

由此产生一个必须记下的差异：

> gmqtt 的 aetherlink 插件会按认证身份把设备上行的扁平 JSON 转成内部信封
> `{ device_id, values }`（`adapter.verifyPayload` 要求的形式）；插件缺失时这一步
> 不会发生。因此种子数据在没有插件的 broker 上必须直接发平台声明的上行信封。
> `lib/seed_data.js` 增加 `options.uplinkEnvelope` 开关（默认关闭，保持历史行为），
> 40_telemetry_anomaly 显式开启。注意 `Values` 是 `[]byte`，encoding/json 走
> base64，所以传的是 `base64(JSON)` 而非裸对象。

**因此**：anomaly 的端到端链路是真实的（HTTP → MQTT → adapter → bus → 存储 → 库 →
分析接口），但 broker 本身是测试替身。P0.2 的 `shadow_ack`、
22_mqtt_device_pipeline 等依赖真实 broker 行为的用例**仍需在 gmqtt 上复验**。

## 6. 未闭环

- P0.2 真实 `shadow_ack` 端到端：需真实 gmqtt。
- 22_mqtt_device_pipeline：同上。
- 浏览器侧 E2E（Playwright）：本次未跑，前端证据止于 vitest 挂载与类型检查。
- 打包导入的**浏览器 file chooser** 路径未跑，页面逻辑靠直接驱动 `handleImportFile` 验证。
