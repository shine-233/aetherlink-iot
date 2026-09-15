# 2026-09-15 发现并修复：TENANT_USER 菜单授权与 API 撤权漂移

## 1. 现象

用 `tenant_user` 角色跑全量菜单路由内容验收（`scripts/diag-page-content.js`）时，
`/management/api`（API Keys）判定为 `EMPTY-OK` —— 也就是**页面能渲染**。

但换成能抓失败请求的 `scripts/diag-spa-mount.js` 复查，问题立刻暴露：

```
!! /management/api   http:1 pageerror:1
     403 /api/v1/open/keys
```

**页面渲染得出来，但它的数据接口对该角色返回 403，并抛出一个 pageerror。**
用户看到的是一个永远加载不出数据的空壳页。

> 这个案例说明：只断言"能不能渲染"是不够的。
> 一个页面可以渲染得很正常，而它的数据接口全被拒。
> **必须同时看失败请求，以及角色边界本身。**

## 2. 根因：两个迁移的意图直接冲突

### 2.1 设计意图是"菜单过滤"

`sql/64.sql`（RBAC 按角色收紧，2026-09-05）把 `/open/keys` 从 TENANT_USER 撤权：

```sql
-- ============ TENANT_USER：撤管理面 ============
DELETE FROM casbin_rule
WHERE ptype = 'p' AND v0 = 'TENANT_USER' AND v1 IN (
  ...
  'api/v1/open/keys',
  'api/v1/open/keys/:id',
```

并且它的文件头注释明确写下了**配套机制**：

> 收紧后 TENANT_USER/TENANT_ADMIN 调用被撤端点将收到 403（RBAC fail-closed），
> 这是预期行为；**console 对应页面按角色菜单过滤（`/ui_elements/menu`）不展示入口**。

即：**后端撤 API 权限，前端靠菜单不展示入口**，两者配套。

### 2.2 但另一个迁移把菜单过滤打穿了

`sql/47.sql` 做了一次**无差别 UPDATE**：

```sql
UPDATE public.sys_ui_elements
SET authority = (authority::jsonb || '"TENANT_USER"'::jsonb)::json
WHERE authority IS NOT NULL
  AND authority::jsonb ? 'TENANT_ADMIN'
  AND NOT (authority::jsonb ? 'TENANT_USER');
```

它的意图是"让租户用户有可用的菜单树"，但它**按 `TENANT_ADMIN` 一刀切地追加 `TENANT_USER`**，
没有考虑"某些页面的 API 恰恰对 TENANT_USER 撤权了"。

`management_api` 由 `sql/5.sql` 创建，原始 authority 就是 `["TENANT_ADMIN"]`，
因此被 47.sql 命中，变成 `["TENANT_ADMIN","TENANT_USER"]` —— 与 64.sql 的撤权决定矛盾。

### 2.3 为什么不是"线上库被手工改坏了"

先排查了"数据库与迁移定义漂移"的可能：`grep -ln "management_api" sql/*.sql` 只命中
`sql/5.sql`，看起来像手工改的。继续追才找到 47.sql 的**批量** UPDATE ——
它不是按 `element_code` 点名，而是按 `authority` 内容匹配，所以不会在按名字搜索时暴露。

**教训：排查"数据为什么和定义不一致"时，不要只按主键/名字搜，要搜批量 UPDATE 语句。**

## 3. 影响范围

对 `tenant_user` 逐条验证它能到达的页面（内容验收里判为 OK/EMPTY-OK 的 5 条），
并用 `diag-spa-mount.js` 检查失败请求：

| 路由 | 接口失败 | 结论 |
|---|---|---|
| `/management/api` | **403 `/api/v1/open/keys`** | **受影响** |
| `/device/manage` | 无 | 正常 |
| `/device/service-access` | 无 | 正常 |
| `/device/grouping` | 无 | 正常 |
| `/visualization/thingsvis` | 无 | 正常 |

实际受影响面比 47.sql 的波及面小，因为 64.sql 撤权的多数端点对应页面
（用户管理 / 角色管理 / casbin / oidc / 字典 / 插件写 / 审计日志等）的菜单行
是在 47.sql **之后**才创建的，没有被那条批量 UPDATE 命中。

## 4. 修复

按设计意图把菜单行改回 `["TENANT_ADMIN"]`（与 `5.sql` 原始定义一致）：

```sql
UPDATE sys_ui_elements SET authority = '["TENANT_ADMIN"]'::json
WHERE element_code = 'management_api';
```

已在本地库执行并验证：

| 角色 | 结果 |
|---|---|
| `tenant_user` | 403（正确拒绝，不再是"空壳页 + pageerror"） |
| `tenant_admin` | 正常（title `API keys`，无失败请求，76723 bytes） |

回归断言已加入 `automation_tests/e2e/27_menu_route_reachability.spec.js`（20/20 通过）。

## 5. 待办：修复尚未落成迁移

**本次只改了本地数据库，没有新增迁移文件**，原因是有硬阻塞：

- 仓库里最大**已提交**的迁移是 `103.sql`；
- `104.sql` / `105.sql` 存在于工作树但**未提交**（另一条并行工作流的在途工作）；
- `backend/pkg/global/global.go` 的 `VERSION_NUMBER` 也被对方改到 `105` 且未提交。

如果此时提交 `106.sql` 并把 `VERSION_NUMBER` 提到 106，会导致：
**别人 checkout 我的提交时，104/105 不存在而 106 存在 → 迁移链断裂。**

因此正确的收口方式是由**落地 104/105 的那次提交**顺带补上这条修复。建议的迁移内容：

```sql
-- 修 47.sql 无差别追加 TENANT_USER 与 64.sql 撤权决定的冲突。
-- 47.sql 给所有含 TENANT_ADMIN 的菜单行统一加了 TENANT_USER，
-- 但 64.sql 已把 api/v1/open/keys 从 TENANT_USER 撤权，并声明靠菜单过滤入口。
-- 这里按 5.sql 的原始定义把 management_api 收回到仅 TENANT_ADMIN。
UPDATE public.sys_ui_elements
SET authority = '["TENANT_ADMIN"]'::json
WHERE element_code = 'management_api'
  AND authority::jsonb ? 'TENANT_USER';
```

## 6. 更一般的风险（建议单独立项）

47.sql 那条批量 UPDATE 是**按 authority 内容匹配**的，任何**后续**新增的
"含 TENANT_ADMIN 但不对 TENANT_USER 开放"的菜单行，都不会再被它命中（因为它早已执行过），
所以不会继续扩大。但反过来：**任何在 47.sql 之后新增的租户级页面，如果其 API 对
TENANT_USER 撤权，就必须自己保证菜单不加 TENANT_USER** —— 这是个容易被遗忘的隐性契约。

建议加一条**一致性校验**（可做成后端启动期断言或测试）：
遍历 `sys_ui_elements` 中授予 TENANT_USER 的叶子行，对其 `param1` 对应的路由做一次
"该角色是否能通过其页面主接口"的探测，发现"菜单放行但接口 403"的组合就报错。
本次的 `diag-page-content.js` + `diag-spa-mount.js` 组合已经是这个校验的手工版本。
