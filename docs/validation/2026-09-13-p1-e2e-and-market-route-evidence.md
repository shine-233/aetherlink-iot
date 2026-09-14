# 2026-09-13 P1 E2E 实跑与打包导入路由死门修复证据

> 目的：把"文档说已完成"换成"跑出来的结果"。本文只记录**实际执行过的命令与原始输出**，
> 未跑过的项一律标注"未验证"，不以单元测试绿色或路由可访问冒充业务闭环（ROADMAP §4）。

## 0. 结论速览

| 项 | 路线图此前口径 | 本轮实测 |
| --- | --- | --- |
| 5 组 API E2E（edge / license / anomaly / bundle-import / operation_logs-export） | 0/5 未写 | **5/5，38 个用例全绿** |
| anomaly 前端 UI | 未写 | 已存在，23 个前端用例通过 |
| 报表工作台前端 UI | 未写 | 已存在，10 个前端用例通过 |
| 打包导入前端 UI | 未接线 | 页面与闸门已实现、34 例通过，**但路由从未注册**（本次补上） |
| 后端 `go build ./...` | 通过 | 通过（exit 0）；首次因 OOM 失败 |
| `vue-tsc --noEmit` | — | exit 0（补路由后） |
| 前端全量 | — | 3 805 通过 / 2 失败（1 假失败、1 真失败 design-token） |

## 1. 环境与前置

- 后端 API：`http://127.0.0.1:9999`，`GET /health` 返回 200（注意健康检查端点是 `/health`，不是 `/api/v1/health`）。
- 账号凭据在 `automation_tests/.env.local`。**bash 下不会自动加载**，必须先导出。

```bash
cd automation_tests
set -a && . ./.env.local && set +a
```

不导出会报 `登录失败: {"code":100002,"message":"Field 'Email' is required"}`。
这不是用例缺陷，是环境变量未加载——曾据此误判"用例没写对"。

## 2. 5 组 API E2E（实跑）

```bash
npx mocha tests/38_edge_nodes.test.js --timeout 60000 --reporter spec
# 10 passing

npx mocha tests/39_license_status.test.js tests/40_telemetry_anomaly.test.js \
          tests/41_market_bundle_import.test.js tests/42_operation_logs_export.test.js \
          --timeout 60000 --reporter spec
# 28 passing
```

逐组覆盖：

| 文件 | 用例数 | 覆盖点 |
| --- | --- | --- |
| `38_edge_nodes.test.js` | 10 | 注册、缺参拒绝、同租户重复注册幂等、跨租户抢注拒绝、列表可见、心跳续命与 health 分类、Reconcile 资源类型/版本闸门（fail closed）/未注册节点/非本租户网关 |
| `39_license_status.test.js` | 6 | 平台管理员状态视图、未启用时的自解释、绝不返回许可证原文、仅有效时给指纹、非平台管理员拒绝、未认证拒绝 |
| `40_telemetry_anomaly.test.js` | 11 | bounds / deviation(k 默认 3)、空窗口语义、时间窗与 window_ms 校验、bounds 缺参与 min>max、k 非正、非法规则类型与聚合、逐设备错误不影响整体、空设备列表 |
| `41_market_bundle_import.test.js` | 5 | 未签名包即使预览也拒绝、签名与 digest 不匹配拒绝、缺 payload 拒绝、预览不落库、同包重导入幂等且不要求覆盖确认 |
| `42_operation_logs_export.test.js` | 6 | 缺时间窗拒绝、结束早于开始拒绝、超一年拒绝、空窗口拒绝而非产空 CSV、导出 CSV 信封、表头不含请求/响应体列 |

**合计 38 个用例，0 失败。**

## 3. 前端（实跑）

```bash
cd frontend
npx vitest run src/views/visualization/anomaly src/views/visualization/report src/views/market/browse
# anomaly 23 · report 10 · market/browse 34（含 bundle-import-model 30）
```

全量回归（单线程，避免 OOM）：

```bash
npx vitest run --maxWorkers=1 --no-file-parallelism
# Test Files 2 failed | 424 passed (426)
# Tests       2 failed | 3805 passed (3807)
```

### 3.1 失败一：locale 四语不一致 —— 假失败

```
zh-cn/route: EXTRA route.market / route.market_browse （es-es、fr-fr 同）
```

根因：本轮在**测试运行期间**修改了 locale 文件，Vite 对已加载过的 `en-us` 用了旧缓存，
其余语言读到新内容，于是基准语言"缺失"被算成其他语言"多余"。
单独重跑 `locale-completeness.test.ts` 后**通过**，非产品缺陷。

### 3.2 失败二：design-token 绊线 —— 真实失败（未修）

```
AssertionError: expected 740 to be less than or equal to 733
```

按修改时间定位，本轮（09-13）未改动任何含 `<style>` 的 `.vue`；最近为
09-12 22:48 的 `views/visualization/thingsvis/index.vue`（5 个 hex）与 09-12 08:15 的 `views/scada/index.vue`（3 个）。
属 09-12 批次引入的既有债。

**处置：不修、不上调基线。** 上调 `HEX_BASELINE` 会让绊线失去"只降不升"的意义；
迁移 hex 涉及 thingsvis / scada 视觉改动，本轮无视觉基线，贸然替换可能引入无人察觉的回归。
已在 ROADMAP §1.3 B 记为独立小开发量项，由引入方在自己的迁移 lane 内收敛。

## 4. 本轮修复一：打包导入页面从未注册路由（死门）

### 4.1 现象

`views/market/browse/index.vue` 已实现预览 / 验签预检 / `confirm_overwrite` 闸门，单测 34 例通过。
但四个注册点全部查无此项：

```bash
grep -n "market" src/router/elegant/imports.ts    # 无
grep -n "market" src/router/elegant/transform.ts  # 无
grep -rn "market" src/router/                     # 无
```

即：后端 P1.6 的验签 + 预览 + 覆盖闸门写得再完整，用户在界面上**没有任何入口能走到它**。
这是"代码已写、测试已过、功能仍不可用"的典型形态。

### 4.2 修复

| 文件 | 改动 |
| --- | --- |
| `src/router/elegant/marketRoutes.ts` | 新增：`/market` → `/market/browse` |
| `src/router/elegant/routes.ts` | 引入并展开 `...marketRoutes` |
| `src/router/elegant/imports.ts` | 增加 `market_browse` 懒加载映射 |
| `src/router/elegant/transform.ts` | `routeMap` 增加 `market` / `market_browse` |
| `src/typings/elegant-router.d.ts` | `RouteMap` 加两条；`FirstLevelRouteKey` 加 `market`、**`LastLevelRouteKey` 只加 `market_browse`** |
| `src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/route.json` | 增加 `route.market` / `route.market_browse` |

类型坑：一级路由与末级路由是两个**硬编码的 Extract 联合**，不是自动派生。
第一次把 `market` 放进了 `LastLevelRouteKey`，`vue-tsc` 报 6 个错误（含 imports 缺 `market` 映射）。
修正后 `NODE_OPTIONS=--max-old-space-size=4096 npx vue-tsc --noEmit --skipLibCheck` → **exit 0，0 错误**。

## 5. 本轮修复二：打包导入单测的具名 slot 丢失

`index.test.ts` 断言页面含 "Import Template"，实际为空。排查确认**不是产品缺陷**：
按钮确实在 `<template #header>` 内的 `n-upload` 里。

根因：本页不像 `report/index.vue` 那样显式 `import { NCard } from 'naive-ui'`，依赖 `main.ts` 全局注册；
测试挂载时组件解析不到，`n-card` 被当作原生元素，具名 slot（`#header` / `#footer`）整块丢弃，
控制台可见 `[Vue warn]: Failed to resolve component: n-card`。

修复：挂载真实 naive-ui，仅对会 teleport 到 body 的 `n-modal` 打内联桩
（否则 `wrapper.text()` 取不到闸门内容；桩保留 `show` 语义，未打开时不渲染）。
这样"导入入口是否真挂在 header 上"仍是真实验证，而不是把断言改松。修复后 34/34 通过。

## 6. 两个会骗人的工程陷阱（已写入 ROADMAP §5 约定）

1. **`cmd | tail; echo $?` 拿到的是 `tail` 的退出码。**
   本轮 `go build -p 1 ./... | tail` 显示 `BUILD_EXIT=0`，实际 `go build` 因 `fatal error: out of memory` 退出码为 1。
   必须重定向到文件后取真实退出码：`go build ... > /tmp/build.log 2>&1; echo $?`。
2. **全量回归期间不要改源文件。** 会让 Vite 缓存产生"假失败"（见 §3.1）。

## 7. 仍未闭环（诚实清单）

- **浏览器证据：0**。新增 `automation_tests/e2e/24_p1_console_surfaces.spec.js`（5 条：anomaly / report / market-browse / edge-nodes / license 页面可达性与导出端点），
  **已写未跑**——缺前端 preview 服务与浏览器依赖。打包导入的路由是本次补的，单测不能证明"用户能点到"，必须在浏览器里真跑一次。
- **P0.1–P0.7、P1.1–P1.4、P2.1、P2.3 的运行期证据**：环境阻塞（无 Docker、磁盘与内存紧张，本轮两次 OOM）。
- **design-token 绊线**：见 §3.2。
- **规则引擎节点种类**：本轮未能从源码枚举出确切数字（`node_type` 为自由字符串、无集中枚举常量），差异分析报告中该格标注为"广度待量化"，未给出不可信的精确值。

## 8. 复现命令

```bash
# 后端构建（注意取真实退出码）
cd backend && go build -p 1 ./... > /tmp/build.log 2>&1; echo $?

# 5 组 API E2E
cd automation_tests && set -a && . ./.env.local && set +a
npx mocha tests/38_edge_nodes.test.js tests/39_license_status.test.js \
          tests/40_telemetry_anomaly.test.js tests/41_market_bundle_import.test.js \
          tests/42_operation_logs_export.test.js --timeout 60000 --reporter spec

# 前端三页面
cd frontend && npx vitest run src/views/visualization/anomaly src/views/visualization/report src/views/market/browse

# 类型检查
cd frontend && NODE_OPTIONS=--max-old-space-size=4096 npx vue-tsc --noEmit --skipLibCheck
```
