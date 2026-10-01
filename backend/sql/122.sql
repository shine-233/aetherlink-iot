-- 122.sql — ThingsBoard 核心实体：客户管理体系 (Customer Management & Entity Hierarchy)
--
-- 背景：
--   1. 对标 ThingsBoard CE/PE 核心实体层级体系（Tenant 租户 -> Customer 客户 -> Device 设备 / Asset 资产）；
--   2. 支持租户管理员创建并维护业务客户档案（公司名称、联系地址、电话、邮箱、扩展属性 JSONB）；
--   3. 支持将设备和资产分配至指定客户（customer_id 关联与分配流水）；
--   4. 提供 Casbin 路由登记与多租户权限隔离：
--      - POST /api/v1/customer (创建/更新客户)
--      - GET /api/v1/customers (分页拉取租户客户列表)
--      - GET /api/v1/customer/:id (获取客户详情)
--      - DELETE /api/v1/customer/:id (删除客户)
--      - POST /api/v1/customer/:id/devices (分配设备到客户)
--      - DELETE /api/v1/customer/:id/device/:device_id (从客户解绑设备)
--      - GET /api/v1/customer/:id/devices (查询客户名下设备列表)

-- ---- 1. 客户主表 (customers) ----
CREATE TABLE IF NOT EXISTS public.customers (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    country VARCHAR(100),
    state VARCHAR(100),
    city VARCHAR(100),
    address VARCHAR(255),
    address2 VARCHAR(255),
    zip VARCHAR(32),
    phone VARCHAR(64),
    email VARCHAR(128),
    additional_info JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_customers_tenant_name UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_customers_tenant_id ON public.customers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_customers_name ON public.customers(name);

-- ---- 2. 客户-设备关联表 (customer_devices) ----
CREATE TABLE IF NOT EXISTS public.customer_devices (
    id VARCHAR(36) PRIMARY KEY,
    customer_id VARCHAR(36) NOT NULL,
    device_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_customer_devices_tenant_device UNIQUE (tenant_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_customer_devices_customer_id ON public.customer_devices(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_devices_device_id ON public.customer_devices(device_id);

-- ---- 3. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/customer'),
  ('api/v1/customer/:id'),
  ('api/v1/customers'),
  ('api/v1/customer/:id/devices'),
  ('api/v1/customer/:id/device/:device_id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 4. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/customer'),
  ('SYS_ADMIN',    'api/v1/customer/:id'),
  ('SYS_ADMIN',    'api/v1/customers'),
  ('SYS_ADMIN',    'api/v1/customer/:id/devices'),
  ('SYS_ADMIN',    'api/v1/customer/:id/device/:device_id'),
  ('TENANT_ADMIN', 'api/v1/customer'),
  ('TENANT_ADMIN', 'api/v1/customer/:id'),
  ('TENANT_ADMIN', 'api/v1/customers'),
  ('TENANT_ADMIN', 'api/v1/customer/:id/devices'),
  ('TENANT_ADMIN', 'api/v1/customer/:id/device/:device_id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);

-- ---- 5. 客户管理菜单 (sys_ui_elements，对标 TB Customers 顶级菜单) ----
-- 本项目的授权路由由 sys_ui_elements 驱动（前端 VITE_AUTH_ROUTE_MODE=dynamic），
-- 页面路由 /customer/list 已注册，必须补菜单行否则路由守卫判定 403（见 100.sql 注释）。
INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'f3a8b6c7-2d14-4e5a-9f80-6b1c2d3e4f50',
    '0',
    'customer', 1, 117, '/customer', 'mdi:card-account-details-outline', 'self',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '客户管理', CURRENT_TIMESTAMP,
    'Customer management (ThingsBoard customers parity)', 'route.customer',
    'layout.base'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'customer'
);

INSERT INTO public.sys_ui_elements (
    id, parent_id, element_code, element_type, orders, param1, param2, param3,
    authority, description, created_at, remark, multilingual, route_path
)
SELECT
    'a4b7c8d9-3e25-4f6b-8a91-7c2d3e4f5a61',
    (SELECT id FROM public.sys_ui_elements WHERE element_code = 'customer'),
    'customer_list', 3, 1, '/customer/list', 'mdi:format-list-bulleted', '0',
    '["SYS_ADMIN","TENANT_ADMIN"]'::json, '客户列表', CURRENT_TIMESTAMP,
    'Customer list with device assignment workbench', 'route.customer_list',
    'view.customer_list'
WHERE NOT EXISTS (
    SELECT 1 FROM public.sys_ui_elements WHERE element_code = 'customer_list'
);
