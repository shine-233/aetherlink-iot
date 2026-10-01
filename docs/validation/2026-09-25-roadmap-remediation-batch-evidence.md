# 2026-09-25 路线图重制实施批次证据（TB-30 / TP-05 / TB-57 客户实体 / TP-19 交叉编译）

> 依据 `docs/roadmap-draft-20260924.md`（现 ROADMAP.md）§5.1 近期计划交付。
> 分支 `feature/roadmap-complete-tb-tp-parity`；本轮全部工作在此工作区完成并逐项留痕。

## 1. 交付清单

### 1.1 TB-30 告警状态实时 WebSocket 订阅（P0，missing → partial：缺活栈运行面）

- 后端：
  - `backend/internal/api/alarm_status_ws.go`（新增）：`GET /api/v1/alarm/status/ws`，首帧 `{token}` 认证（复用 `upgradeTelemetryWSSession`/`validateAuth`），订阅 `alarm:tenant:{tenant_id}` Redis 频道（`startRedisWSPubSubForwarder`），订阅建立即推送 `{"type":"snapshot","items":[...]}`（最近 20 条告警历史摘要，`dal.ListRecentAlarmHistoryForSnapshot`）；ping/pong 心跳与既有 WS 一致。
  - `backend/internal/service/alarm_realtime.go`（新增）：`PublishAlarmEvent` 发布 trigger/recovery/status 三类生命周期事件；发布失败仅记日志（尽力而为，不阻断告警事务）。Go 单测 `alarm_realtime_test.go` 6 例（纯函数与 nil-REDIS 守卫）。
  - 发布点接线：`AlarmExecute`（trigger）、`AlarmRecovery`（recovery）、`UpdateAlarmInfo`/`UpdateAlarmInfoBatch`（status）。
  - 路由：`backend/router/router_init.go` 公共 WS 块注册 `alarm/status/ws`（与遥测/在线状态 WS 同级，首帧鉴权）。
  - 静态守卫同步：`internal/api/context_background_budget_test.go` 预算 3→4（连接级订阅 ctx 豁免，就地注释）；`internal/dal/tenant_scope_audit_test.go` 基线无新增（快照查询带租户过滤）。
- 前端：
  - `frontend/src/hooks/thingsvis/useAlarmPush.ts`：30s `setInterval` 轮询 → WS 订阅 + 初始 REST 拉取 + WS 不可用自动降级轮询。
  - `frontend/src/hooks/alarm/useAlarmStatusSocket.ts`（新增）：通用订阅 hook；`views/alarm/warning-message/components/alarm-configuration.vue` 接入，实时事件去抖 800ms 刷新列表，`onUnmounted` 清理。

### 1.2 TP-05 一型一密产品级交叉校验（安全快赢）

- `backend/internal/service/device_auth.go`：新增 `ensureAuthProductMatchesConfig`——product_key 对应产品必须与命中档案同租户、且 `products.device_config_id` 绑定关系一致，否则拒绝注册（错误码 **200087**）；`dal.GetProductByID` 带 `tenant-scope: caller-enforced` 评审标记（取行即比对租户，属刻意设计）。
- 修复前漏洞：任意存在的 product_key（含跨租户）都能与 A 租户档案密钥组合成功建档。

### 1.3 TB-57 客户实体收尾（122.sql 进行中工作补全四面）

- DAL 修正（`backend/internal/dal/customer.go`）：`AssignDevices` ① 校验设备真实存在且属于当前租户（防跨租户挂接/悬挂引用）；② 分配即移动（`uk_customer_devices_tenant_device` 唯一约束下先清旧关系再挂新客户）；③ 入参去重。
- 前端：`frontend/src/service/api/customer.ts`（7 端点封装 + `__tests__/customer.test.ts` 7 用例）；`frontend/src/views/customer/list/index.vue`（列表/搜索/新建/编辑/删除 + 设备分配抽屉）；菜单 `sys_ui_elements` 种子（`122.sql` 追加 customer/customer_list 两行，NOT EXISTS 守卫）；elegant-router 生成文件手工同步（customerRoutes.ts + routes/imports/transform/elegant-router.d.ts）；四语言 locale（route.json + custom.json page.customer.*）。
- API barrel：`service/api/index.ts` 导出 `./customer`，`index.exports.test.ts` 内联快照随契约扩面更新（vitest -u）。

### 1.4 TP-19 国产化 OS 交叉编译（把旧稿"夸大"变为可复现事实）

- `backend/Makefile` 新增 `build-linux-amd64` / `build-linux-arm64` / `build-linux-loong64` / `cross-compile`（`CGO_ENABLED=0`）。
- 实测：三架构二进制产出，`file` 确认 `ELF 64-bit LSB executable, LoongArch, statically linked`（amd64 98MB / arm64 92MB / loong64 97MB）。注：loong64 首次构建出现 Windows 临时文件偶发竞争报错，重试成功。
- 旧稿 TP-7"交叉编译支持"证据失真（Makefile 无目标）已闭环。

### 1.5 顺手修复的存量缺陷

- `es-es/fr-fr route.json` 缺 `route.device_converter`/`view.device_converter`（上一批 converter 功能只补了中英文），locale-completeness 契约测试恢复绿。
- `views/device/details/__tests__/index.test.ts` 共享视图期望未同步 TP-6 `health-assessment`（登记处已标 `sharedReadOnlySafe: true`），期望列表更新为 chart/device-3d/health-assessment。
- `README.md:127` 迁移链口径 99 → 122。
- `docs/openapi/README.md` 重生成命令 `-out` 路径修正（`../docs/openapi/openapi.json`；旧写法会把产物写到 `backend/docs/` 下）。

## 2. 门禁实测（2026-09-25）

| 门禁 | 命令 | 结果 |
| --- | --- | --- |
| 后端构建 | `go build ./...` | exit 0 |
| 后端全量单测 | `go test ./...` | **REAL_EXIT=0，66 包 ok / 0 FAIL**（含新增 6 用例；两个静态守卫审计测试在补齐豁免注释与 marker 后通过） |
| 前端类型检查 | `vue-tsc --noEmit --skipLibCheck` | exit 0（0 错误） |
| 前端全量 vitest | `vitest run`（441 文件） | **3970 用例全绿**（3 个失败均为存量联动缺口，已修复：locale 完整性、API 导出面快照、device-details 期望） |
| OpenAPI | `go run ./cmd/openapigen -out ../docs/openapi/openapi.json` | **489 paths**（483 + 客户 5 端点 + `alarm/status/ws`）；新增路径已实测断言存在 |
| 交叉编译 | `make cross-compile`（等价目标） | 三架构二进制产出（见 1.4） |

## 3. 未覆盖 / 局限（诚实边界）

1. **契约测试 75/76/77 未运行**：本机无 docker（`docker: command not found`），活栈（PG+Redis+broker+backend）不可用。三个用例已就位（客户 CRUD/隔离 9 例、告警 WS 快照+鉴权+心跳 3 例、一型一密交叉校验 4 例），待活栈回归后方可把对应行状态从 partial 升 done。
2. **Playwright E2E 未运行**（同上，需前端+后端活栈）。
3. **`pnpm gen-route` 工具链损坏**（`unicorn-magic@6` 与 Node 24 ESM 解析冲突，`pnpm install` 后仍复现；属环境既有问题，与本次改动无关）：elegant-router 生成文件以手工同步代替，一致性由 vue-tsc + 全量 vitest 验证。
4. 告警 WS 的 Redis 事件转发契约（订阅→触发→推送）依赖活栈验证，单测仅覆盖纯函数与守卫。
