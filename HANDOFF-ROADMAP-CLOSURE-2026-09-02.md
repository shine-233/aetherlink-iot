# AetherLink — ROADMAP 收口与方向裁决交接（2026-09-02）

> 承接范围：PR #200（分支 `feat/full-c2c6c7-20260902`，基于隔离 clone `C:\Users\Zz\aetherlink-impl-work`）。
> 本文件目的是让下一个会话/并行实现者无需重推结论即可继续剩余的环境绑定工作。

## 0. 当前状态速览

| 项 | 值 |
|---|---|
| GitHub | https://github.com/shine-233/aetherlink-iot/pull/200（分支 `feat/full-c2c6c7-20260902`） |
| 分支最近提交 | `7bbc47d fix(front): typecheck clean for 2FA/SSO wiring` |
| 工作树 | `C:\Users\Zz\aetherlink-impl-work`（独立 .git，可自由 commit/push 到 gh 远端） |
| 主线构成 | 官方主线(实体版本#195) ⊕ 12 项 roadmap 增量(60da666) ⊕ c5 白标/实体版本/安全线 → 单一 `main` 语义 |
| 后端验证 | `go build ./...` 0；`go test ./... -count=1` **全包 ok、0 FAIL**（54+ 包含 app/middleware/router/oidc/totp/coap/lwm2m/snmp/opcua/protocolgw） |
| 前端验证 | `pnpm run typecheck`（vue-tsc --noEmit）**EXIT 0**（在 impl frontend 安装依赖后；esbuild 构建脚本被沙箱安全策略拦截，不影响类型检查；`vite build` 需在放行 esbuild 的环境执行） |
| ROADMAP.md | 仅剩 3 个环境绑定项未勾（见 §3），其余全部勾选带证据 |

## 1. 关键裁决（已执行，勿再回退除非产品推翻）

### 1.1 RBAC / 数据作用域方向 = 自上而下
- 依据（ThingsBoard 官方/实践）：Parent Customer / Tenant Admin 可见**整棵子树**；子级租户仅自身；跨层共享走显式授权。
- 实现：`hierarchy.Descendants` + `ScopeDown = {self} ∪ Descendants`；service `expandTenantIDScope`/`assetScope` 用 `ScopeDown`；`InheritedAuthorityRoles` 域角色标记 = `authority@descendant`（`inheritedRoleMarkersFor`）；DAL 评论与 ROADMAP 表述全部更新为 "self∪子孙（自上而下）"。
- 推论：**若产品要「子可见父共享数据」，改为显式 Share 能力，不要改回向上展开**（会重新引入越权泄漏面）。

### 1.2 前端验证环境 = 隔离 clone 内安装
- `cd aetherlink-impl-work/frontend && pnpm install`（≈3min；esbuild/vue-demi postinstall 被安全策略拦截但 vue-tsc 可用）。
- 用 `pnpm run typecheck`（不要直接 `cross-env`；`pnpm` 提供仓库内 .bin PATH）。

## 2. 本分支已交付（供代码锚点）

| 区域 | 要点 | 关键文件 |
|---|---|---|
| 主线合入 | 三线合并为单一主线，迁移版本号=62 | `backend/pkg/global/global.go`、`backend/sql/README.md` |
| C2 资产 | assets CRUD/树 + Scope DAL | `internal/{model,dal,service,api}/asset.go`、`dal/tenant_link.go` |
| C2 层级作用域 | Descendants/ScopeDown + RBAC 接缝 | `hierarchy/hierarchy.go`(+down_test)、`service/tenant_scope.go`(+test) |
| C2 列表级联 | device/alarm/config/info/history/board → `ForScopes`（IN self∪子孙） | `dal/devices_list_read_model*.go`、`dal/alarm.go`、`dal/board.go`；service 同名文件 |
| C7 2FA | 61.sql + 绑定/激活/解绑/第二因子登录（防重放+恢复码） | `sql/61.sql`、`service/user_totp.go`、`api/user_totp.go`、登录页 `pwd-login.vue`、`TwoFactorSetting` 组件 |
| C7 OIDC/SSO | 62.sql + 提供方 CRUD + /sso/:id/start\|callback + 公开 providers | `sql/62.sql`、`service/oidc_sso.go`、`dal/oidc_provider.go`、登录页 SSO 按钮 |
| C6 网关 | CoAP/LwM2M UDP 配置门控接入 | `internal/protocolgw/`、`app/coap_gateway.go`、`main.go` |
| B2 规则链 | action.alarm 节点（引擎+画布+i18n） | `service/rule_chain_{graph,engine}.go`、`editor.vue` |
| 工具 | A3 影子 E2E spec、A4/B4 空态审计 | `automation_tests/pending-e2e/…`、`frontend/scripts/audit-empty-states.mjs`(+report) |

## 3. ROADMAP 剩余（全部环境绑定，未假完成）

1. **C5 运行期全流程验证**（两租户白标互不串扰 + 品牌表单回归）→ 重建隔离栈跑 API/E2E。
2. **C6 设备凭证→遥测上行管道**（网关设备映射到现有 MQTT 凭证/uplink 数据面）→ 真实联调。
3. **真实 IdP E2E**（OIDC /sso 全流程）→ 外部 IdP 沙箱。

另附（非 ROADMAP 勾选项但建议处理）：
- B4 空态报告 `frontend/scripts/empty-state-audit-report.json`：listy=146、缺失 56、覆盖率 61.6%，逐条人工补 `n-empty`（勿无审阅批量自动补）。
- **Casbin g2 资源登记与 automation_tests endpoint/capability 目录登记**未做（新路由启动时 Casbin 覆盖审计/契约计数需在重建栈后按既有机制登记：新端点 assets/totp/oidc/rule-chain action.alarm 对应 g2 行 + `catalog.js`/`business-capabilities.js`）。
- 真机上 ThingsVis/rdi/外部 8000 依赖的 integration-blocked 用例仍为显式 skip。

## 4. 重建/验证配方（复用于剩余项）
- 后端：`cd backend && go build ./... && go test ./... -count=1`
- 前端：`cd frontend && pnpm install && pnpm run typecheck`（vite build 需放行 esbuild）
- 运行栈：沿用 `aetherlink-dev-local` 隔离栈 bring-up 配方（PG5433 trust / Redis / Broker1883 / Backend9999 / Preview9725；`.env.local` 需 `aetherlink_iot_local` + external OTA 模式；E2E 复用预览代理需 `PLAYWRIGHT_REUSE_EXISTING_SERVER=1`；注意清 `.webm` 残留与 sandbox 批量删除护栏）。

## 5. 工程经验（防重复踩坑）
- **沙箱 heredoc 会把反斜杠转义改成正斜杠**（`\n`→`/n`）污染被写文件：凡含换行/制表符的补丁一律用 Write 落盘 `.py` 再 `python file`，字符串内避免反斜杠序列。
- 隔离 repo 放 `projects` 之外可避免被并行会话的 git 清扫干扰；refs 写入若被重置用 `git update-ref` + 手动写 refs 文件修复 HEAD。
- `go vet` 对 `cron_test.go` 既有 lock-copy 告警与本批无关（勿误判）。
