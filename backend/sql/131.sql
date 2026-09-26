-- 131.sql — TB-46 用户组与组权限（GPE v1）：租户内用户组 + 组权限元素绑定 + 组级共享
--
-- 背景：
--   1. 对标 ThingsBoard PE 高级 RBAC 的用户组形态：此前平台只有设备组树（groups）与
--      自定义角色（roles + sys_role_permissions，118.sql），无用户组实体，组级共享
--      仅限设备组，看板/资产无分组可见性控制（ROADMAP TB-46）；
--   2. 本迁移建立三张表：
--      - user_groups       用户组主表（租户内唯一命名）；
--      - r_group_user      用户-组成员关联（UNIQUE(group_id, user_id)，参照 r_group_device）；
--      - group_permissions 组-权限元素绑定（UNIQUE(group_id, element_code)，
--        列模式参照 sys_role_permissions（118.sql））。
--   3. element_code 为带命名空间的权限元素码，GPE v1 支持资源元素：
--      - board:<board_id>  看板资源元素（组共享授权）
--      - asset:<asset_id>  资产资源元素（组共享授权）
--      组共享语义：绑定到组的看板/资产默认对组外成员不可见（fail-closed 默认不可见），
--      组内成员经「用户所属组→组权限→资源可见映射」获得可见性；管理员不受限；
--      未绑定任何组的看板/资产维持既有租户内可见行为，不回归。
--   4. 边界（ROADMAP TB-46 v1，验收口径已定）：customer 客户（122.sql customers 表，
--      无登录账号）不接入组授权——r_group_user.user_id 外键限定 users 表内账号；
--      设备组（groups / r_group_device）不做任何改动。
--   5. Casbin 路由登记（g2）与角色授权（p）：仅 SYS_ADMIN / TENANT_ADMIN；
--      TENANT_USER fail-closed——组成员身份只带来资源可见性，不带来组管理能力
--      （口径同 101.sql 注释）。
--      - POST/PUT/DELETE /api/v1/user_group[/:id]      组 CRUD
--      - GET           /api/v1/user_groups             组分页列表
--      - GET/POST      /api/v1/user_group/:id/users        成员查询与批量绑定
--      - GET/POST      /api/v1/user_group/:id/permissions  组权限查询与批量绑定
--   6. 管理菜单（sys_ui_elements）：/management/user-group 页面行挂 management 目录
--      （父 id e1ebd134-53df-3105-35f4-489fc674d173）。前端四件套（下一阶段交付）
--      必须与本种子对齐：路由名 management_user-group、组件 view.management_user-group、
--      i18n 键 route.management_user-group、页面文案键 page.user_group.*。

-- ---- 1. 用户组主表 (user_groups) ----
CREATE TABLE IF NOT EXISTS public.user_groups (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_user_groups_tenant_name UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_user_groups_tenant_id ON public.user_groups(tenant_id);

COMMENT ON TABLE public.user_groups IS '用户组主表（TB-46 GPE v1，租户内用户分组与组级共享授权）';

-- ---- 2. 用户-组成员关联表 (r_group_user) ----
CREATE TABLE IF NOT EXISTS public.r_group_user (
    group_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_r_group_user_group_user UNIQUE (group_id, user_id),
    CONSTRAINT fk_r_group_user_group FOREIGN KEY (group_id) REFERENCES public.user_groups(id) ON DELETE CASCADE,
    CONSTRAINT fk_r_group_user_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_r_group_user_user_id ON public.r_group_user(user_id);
CREATE INDEX IF NOT EXISTS idx_r_group_user_tenant_id ON public.r_group_user(tenant_id);

COMMENT ON TABLE public.r_group_user IS '用户组成员关联（TB-46；user_id 仅限 users 登录账号，customer 客户不接入）';

-- ---- 3. 组-权限元素绑定表 (group_permissions) ----
CREATE TABLE IF NOT EXISTS public.group_permissions (
    id VARCHAR(36) PRIMARY KEY,
    group_id VARCHAR(36) NOT NULL,
    element_code VARCHAR(100) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_group_permissions_group_element UNIQUE (group_id, element_code),
    CONSTRAINT fk_group_permissions_group FOREIGN KEY (group_id) REFERENCES public.user_groups(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_group_permissions_group_id ON public.group_permissions(group_id);
CREATE INDEX IF NOT EXISTS idx_group_permissions_tenant_id ON public.group_permissions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_group_permissions_element_code ON public.group_permissions(element_code);

COMMENT ON TABLE public.group_permissions IS '组权限元素绑定（TB-46 GPE v1；element_code=board:<id>/asset:<id> 资源元素，组共享的可见性映射）';
COMMENT ON COLUMN public.group_permissions.element_code IS '权限元素码：v1 支持 board:<board_id> / asset:<asset_id> 资源元素（组共享）';

-- ---- 4. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/user_group'),
  ('api/v1/user_group/:id'),
  ('api/v1/user_groups'),
  ('api/v1/user_group/:id/users'),
  ('api/v1/user_group/:id/permissions')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 5. 授权角色 (p) ----
-- 仅授 SYS_ADMIN / TENANT_ADMIN：组管理能力对 TENANT_USER 是 fail-closed；
-- 组成员经组共享获得的是看板/资产数据可见性（dal 层 scope），不是接口授权。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/user_group'),
  ('SYS_ADMIN',    'api/v1/user_group/:id'),
  ('SYS_ADMIN',    'api/v1/user_groups'),
  ('SYS_ADMIN',    'api/v1/user_group/:id/users'),
  ('SYS_ADMIN',    'api/v1/user_group/:id/permissions'),
  ('TENANT_ADMIN', 'api/v1/user_group'),
  ('TENANT_ADMIN', 'api/v1/user_group/:id'),
  ('TENANT_ADMIN', 'api/v1/user_groups'),
  ('TENANT_ADMIN', 'api/v1/user_group/:id/users'),
  ('TENANT_ADMIN', 'api/v1/user_group/:id/permissions')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- 6. 管理菜单 (sys_ui_elements，挂 management 目录，对标 management_role) ----
-- 授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic），页面路由
-- /management/user-group 由下一阶段前端四件套注册，菜单行先行就位（NOT EXISTS 守卫）。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'd3a1b4c8-6e27-4f59-8a70-9b2c3d4e5f61',
    'e1ebd134-53df-3105-35f4-489fc674d173',
    'management_user-group', 3, 1182, '/management/user-group', 'mdi:account-group', '1',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '用户组', CURRENT_TIMESTAMP,
    'User groups and group permission elements (TB-46 GPE v1)', 'route.management_user-group',
    'view.management_user-group'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'management_user-group'
);
