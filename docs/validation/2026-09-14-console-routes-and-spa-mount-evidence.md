# 2026-09-14 控制台路由可达性与 SPA 挂载证据

> 对应 ROADMAP §1.3-B-2。本文记录"页面存在却打不开"这一类问题的根因、修复与运行证据。
> 执行环境：Windows 本机，PostgreSQL 55433 + Redis 6379 + `scripts/local_mqtt_broker.js`(1883) + 后端 9999 + 预览代理 9725。

## 0. 结论速览

| 指标 | 修复前 | 修复后 |
| --- | --- | --- |
| `e2e/24_p1_console_surfaces.spec.js` | **1 通过 / 5 失败**（5 张全白截图） | **6 通过 / 0 失败** |
| 前端受影响单测 | 4 失败 | **83/83 通过** |
| `vue-tsc --noEmit` | 16 error | **0 error** |
| 后端 `go build -p 1 ./...` | — | **exit 0** |
| 后端 `go test -p 1 ./...` | — | **exit 0，61 包全 ok** |
| API E2E `tests/38–42` | 40 号 before-all 挂 | **38/38 通过** |

## 1. 根因一：全路由白屏（TDZ）

### 现象
`frontend/dist` 构建成功、HTTP 200，但浏览器里 `<div id="app">` 始终为空，所有路由白屏。

### 定位过程
用 `automation_tests/scripts/diag-spa-mount.js` 捕获 `pageerror`：

```
ReferenceError: Cannot access 'RefImpl' before initialization
    at createRef (assets/vendor-B1QIOx7b.js:1591:3)
    at ref$1    (assets/vendor-B1QIOx7b.js:1581:10)
    at assets/vendor-three-CXqZFHVJ.js:43785:19
```

（压缩构建下同一错误的绑定名被压成 `oN`，堆栈为 `MS`/`Nt`。）

### 根因
`frontend/vite.config.ts` 的 `manualChunks` 末尾有一句兜底 `return 'vendor'`，
把 vue / vue-router / pinia / vue-i18n / naive-ui 之外的所有 node_modules 塞进同一个
unprefixed chunk。这些包之间存在循环导出，被强制合并进单一 chunk 后，
Rollup 无法保证模块初始化顺序，运行期即触发 TDZ。

值得注意的是**该文件里紧邻的注释早就写明了这个风险**：
> "Forcing them into one manual chunk can trigger TDZ errors in production preview
> when circular exports initialize out of order."

代码与注释不一致，属于"知道坑但没改"。

### 修复
`frontend/vite.config.ts`：去掉 `return 'vendor'` 兜底（改为 `return undefined`），
其余具名 chunk（echarts / codemirror / grid / naive-ui / three / motion / utils / crypto）保持不变，
未匹配的依赖交回 Rollup 自动切分。

### 排除项
一度怀疑 `@tresjs/three` 与 `components/device3d/**` 的自动注册有关，并据此改过
`device-3d.vue` 与 `build/plugins/index.ts`。经回退验证：**这两处改动都不是必需的**，
真正的修复只有 `manualChunks` 一处。相关改动已全部回退，避免无谓 diff。

## 2. 根因二：页面存在但 403

### 现象
路由修好后，`/visualization/anomaly`、`/market/browse`、`/management/edge-nodes`、
`/management/license` 返回 **403 No Permission**（不是 404）。

### 根因
项目使用 `VITE_AUTH_ROUTE_MODE=dynamic`，授权路由由后端 `sys_ui_elements` 驱动。
`frontend/src/router/guard/permission.ts` 的判定是：

```
to.name === 'not-found' && getIsAuthRouteExist(path) === true  →  跳 403
```

即"前端 generatedRoutes 里有条目、但不在当前用户的授权菜单里" → 403。
两个页面的**前端文件与路由定义都在**，只是 `sys_ui_elements` 里没有菜单行。

### 修复
- 前端补注册（五处同步）：
  `routes.ts`（引入并展开 `marketRoutes`）、`imports.ts`（`market_browse` / `visualization_anomaly`）、
  `transform.ts`（路径映射）、`visualizationRoutes.ts`（anomaly 子路由）、
  `typings/elegant-router.d.ts`（RouteMap + `FirstLevelRouteKey` + `LastLevelRouteKey`）。
- 后端新增 `backend/sql/100.sql`，补 5 条菜单行：
  `visualization_anomaly`、`market`、`market_browse`、`management_edge-nodes`、`management_license`。
  全部带 `NOT EXISTS` 守卫；`management_license` 只授 `SYS_ADMIN`（平台级视图）。
- `backend/pkg/global/global.go`：`VERSION_NUMBER` 99 → 100（与 `sql/` 最大编号一致）。

### 幂等证据
```
$ psql -f sql/100.sql     # 首次
INSERT 0 1 ×5
$ psql -f sql/100.sql     # 复跑
INSERT 0 0 ×5
```

## 3. 根因三：打包导入闸门 UI 回退

### 现象
`src/views/market/browse/__tests__/index.test.ts` 报
`No "importDeviceTemplate" export is defined on the "@/service/api/market" mock`。

### 根因
main 上的 `index.vue` 是**旧版**（135 行）：上传按钮的 `handleImportFile` 直接调用
`/device/template/import`——该端点**不做 `VerifyMarketBundle`、也没有 `confirm_overwrite` 闸门**。
而 `bundle-import-model.ts`（闸门判定模型，190 行）与测试都已按新版存在，
组件与测试契约不一致。带闸门的 `index.vue` 从未提交进 main（悬空对象中亦无）。

### 修复
- 按模型重写 `index.vue`：解析预检 → `preview=true` 只读预览 → 三类名单分开渲染 →
  覆盖项勾选确认（换文件重置）→ `confirm_overwrite` 提交 → 结果汇总。
- `src/service/api/market.ts`：补回 `importMarketBundle` 与三个结果类型；
  **移除** `importDeviceTemplate` 导出（消除"再被接回旧端点"的隐患）。
- 4 语言各补 21 个 `page.marketBrowse.import*` 文案键。

## 4. 根因四：anomaly 页文案全缺

`page.anomaly.*` 的 43 个键在 zh-cn / en-us / es-es / fr-fr 中**都不存在**，
`$t()` 回退成键名。已按 4 语言补齐 43 键（含 `{max}` / `{count}` 占位符）。

## 5. 根因五：遥测种子跨天 100% 失败

### 现象
`tests/03_data`、`12_telemetry_extra`、`40_telemetry_anomaly` 的 before-all 全部报：
`publish simulated telemetry failed: device credential test cache expired or absent`

### 根因（两层）
1. **24 小时凭证缓存**：模拟遥测走 `internal/service/telemetry_simulation.go` 的
   `loadSimulationVoucher`，它只认 ① 创建/轮换时写入 Redis 的 24h 测试缓存，
   ② 凭证哈希 Phase 2b 之前的 DB 明文 `voucher` 列。经 API 新建的设备明文列为空，
   缓存过期即必然失败，且**后端没有凭证轮换端点**。
   而 `ensureDeviceWithTelemetry` 复用既有设备 → 隔天必挂。
2. **载荷格式**：`publishSimulatedTelemetryAndReadCurrent` 只发扁平载荷
   `{"temperature_1":25.5,...}`。真实 gmqtt + aetherlink 插件会在 broker 侧补成
   `{"device_id":...,"values":...}` 信封，但本地 stub broker 没有该插件，
   `adapter.verifyPayload` 会因 `device_id` 为空丢弃消息（表现为"发出去了但读不回来"）。
   测试 40 传的 `{ uplinkEnvelope: true }` 选项此前**未被实现也未被转发**。

### 修复（`automation_tests/lib/seed_data.js`）
- `ensureDeviceWithTelemetry(accountKey, options)`：改为每次 `createSimulationDevice`
  新建带新鲜凭证的设备，失败时回收；并**转发 options**。
- `publishSimulatedTelemetryAndReadCurrent`：实现 `uplinkEnvelope`
  （`values` 用 base64，与 Go 侧 `publicPayload.Values []byte` 的 json 编码一致），
  并做**自动回退**：先发扁平载荷，读不回来再发信封——两种 broker 环境都对。

## 6. 复现命令

```bash
# 依赖
node automation_tests/scripts/local_mqtt_broker.js &            # 1883
cd backend && AETHERLINK_TIMESCALE_MODE=off go run . -config configs/conf-localdev.yml &

# 前端构建（必须带 VITE_SERVICE_ENV=prod：默认 dev 会把 baseURL 写成 127.0.0.1:9999 触发 CORS）
cd frontend && VITE_SERVICE_ENV=prod NODE_OPTIONS=--max-old-space-size=3072 npx vite build

# 预览代理 + 浏览器用例（webServer 会抢 9725，必须复用）
cd automation_tests && set -a && . ./.env.local && set +a
node scripts/serve_preview_with_api_proxy.js &
PLAYWRIGHT_REUSE_EXISTING_SERVER=1 npx playwright test e2e/24_p1_console_surfaces.spec.js

# API E2E
npx mocha tests/38_edge_nodes.test.js tests/39_license_status.test.js \
  tests/40_telemetry_anomaly.test.js tests/41_market_bundle_import.test.js \
  tests/42_operation_logs_export.test.js --timeout 90000
```

## 7. 仍未验证的部分

- `license` 页只验证了 super_admin 可达 + tenant_admin 落 403；**未验证**许可证签发工具（P3 仍未实现）。
- 打包导入闸门只验证了"UI 契约 + 端点可达"；**未验证**真实签名包的端到端导入与覆盖确认落库效果。
- `e2e/24` 只证明"能打开、能渲染、不是 404/403"，业务正确性由 `tests/38–42` 覆盖，两层不重复断言。
- 其余 20 个 Playwright spec 未在本次一并重跑。
