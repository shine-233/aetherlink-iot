-- 64.sql: RBAC 按角色收紧（ROADMAP C7+ 产品决策落库，2026-09-05）。
-- 背景：63.sql 将 287 条受保护路由对 3 个内置角色全量授权（状态 quo 显式化）。
-- 本迁移按最小权限原则删除越权 p 授权行：
--   TENANT_USER  撤全部管理面：用户管理/角色管理/权限管理(casbin)/SSO 配置(oidc)/
--                OpenAPI 密钥/平台字典内容/插件注册与接入配置写/系统功能/平台监控/
--                审计日志/UI 元素与仪表盘菜单管理；保留自助与只读（个人资料、TOTP、
--                prefer-lang、refresh/logout、菜单读取、枚举字典读、插件信息读等）。
--   TENANT_ADMIN 撤平台级面：插件注册/删除与接入配置写、系统功能、平台监控、
--                平台字典内容管理；保留租户内用户/角色/SSO/白标/日志/业务全量。
--   SYS_ADMIN    保持全量（平台超级管理员）。
-- 幂等性：DELETE 天然幂等；重放无副作用。
-- 注意：收紧后 TENANT_USER/TENANT_ADMIN 调用被撤端点将收到 403（RBAC fail-closed），
--       这是预期行为；console 对应页面按角色菜单过滤（/ui_elements/menu）不展示入口。

-- ============ TENANT_USER：撤管理面 ============
DELETE FROM casbin_rule
WHERE ptype = 'p' AND v0 = 'TENANT_USER' AND v1 IN (
  -- 用户管理（保留自助：/user/detail /user/update /user/change-email /user/prefer-lang
  --          /user/refresh /user/logout /user/totp/* /user/tenant/id /user/selector）
  'api/v1/user',
  'api/v1/user/:id',
  'api/v1/user/transform',
  'api/v1/user/warning-email',
  -- 角色与权限管理
  'api/v1/role',
  'api/v1/role/:id',
  'api/v1/casbin/function',
  'api/v1/casbin/function/:id',
  'api/v1/casbin/user',
  'api/v1/casbin/user/:id',
  -- SSO 提供方配置（租户管理员职责）
  'api/v1/oidc/provider',
  'api/v1/oidc/provider/:id',
  'api/v1/oidc/provider/list',
  -- OpenAPI 密钥管理
  'api/v1/open/keys',
  'api/v1/open/keys/:id',
  -- 平台字典内容管理（保留只读：/dict/enum /dict/protocol/service）
  'api/v1/dict',
  'api/v1/dict/column',
  'api/v1/dict/column/:id',
  'api/v1/dict/language',
  'api/v1/dict/language/:id',
  -- 插件注册与接入配置写（保留只读：/service/list /service/detail/:id
  --          /service/plugin/select /service/plugin/info /service/access/list
  --          /service/access/device/list /service/access/voucher/form）
  'api/v1/service',
  'api/v1/service/:id',
  'api/v1/service/access',
  'api/v1/service/access/:id',
  -- 系统功能/平台监控/审计日志
  'api/v1/sys_function/:id',
  'api/v1/system/metrics/current',
  'api/v1/system/metrics/history',
  'api/v1/operation_logs',
  -- UI 元素与仪表盘菜单管理（保留读取：/ui_elements/menu /ui_elements/select/form）
  'api/v1/ui_elements',
  'api/v1/ui_elements/:id',
  'api/v1/dashboard-menu/:dashboardId',
  'api/v1/dashboard-menu/batch'
);

-- ============ TENANT_ADMIN：撤平台级面 ============
DELETE FROM casbin_rule
WHERE ptype = 'p' AND v0 = 'TENANT_ADMIN' AND v1 IN (
  -- 插件注册/删除与接入配置写（平台管理员职责；保留全部只读）
  'api/v1/service',
  'api/v1/service/:id',
  'api/v1/service/access',
  'api/v1/service/access/:id',
  -- 系统功能
  'api/v1/sys_function/:id',
  -- 平台监控
  'api/v1/system/metrics/current',
  'api/v1/system/metrics/history',
  -- 平台字典内容管理（保留只读）
  'api/v1/dict',
  'api/v1/dict/column',
  'api/v1/dict/column/:id',
  'api/v1/dict/language',
  'api/v1/dict/language/:id'
);

-- ============ SYS_ADMIN：保持全量（无删除） ============
