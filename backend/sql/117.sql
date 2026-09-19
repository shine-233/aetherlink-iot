-- 117.sql — P3 计费模型与租户用量计量系统（Billing, Subscription Plans & Tenant Usage Metering）
--
-- 背景：
--   1. ROADMAP P3 商业化闭环：计费与套餐分级（Billing / Subscription Plans / Usage Metering）；
--   2. 支持多层级套餐（free/pro/enterprise），按设备、租户、用户与数据点实施配额计算与超限预警；
--   3. 租户用量计量端点（GET /api/v1/billing/usage）：当前活跃设备数、用户数、子租户数及指标百分比；
--   4. Casbin 路由赋权：/api/v1/billing/plans, /api/v1/billing/usage, /api/v1/billing/subscriptions。

CREATE TABLE IF NOT EXISTS public.subscription_plans (
    id varchar(36) PRIMARY KEY,
    code varchar(50) NOT NULL UNIQUE,
    name varchar(100) NOT NULL,
    description text,
    price_monthly numeric(10, 2) NOT NULL DEFAULT 0.00,
    currency varchar(10) NOT NULL DEFAULT 'USD',
    max_devices int NOT NULL DEFAULT 10,
    max_tenants int NOT NULL DEFAULT 1,
    max_users int NOT NULL DEFAULT 3,
    max_telemetry_per_day int NOT NULL DEFAULT 10000,
    max_api_calls_per_day int NOT NULL DEFAULT 5000,
    features jsonb NOT NULL DEFAULT '[]'::jsonb,
    enabled smallint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS public.tenant_subscriptions (
    id varchar(36) PRIMARY KEY,
    tenant_id varchar(36) NOT NULL UNIQUE,
    plan_code varchar(50) NOT NULL,
    status varchar(30) NOT NULL DEFAULT 'active',
    current_period_start timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    current_period_end timestamptz NOT NULL DEFAULT (CURRENT_TIMESTAMP + INTERVAL '30 days'),
    cancel_at_period_end boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tenant_subscriptions_tenant_id ON public.tenant_subscriptions(tenant_id);

-- 默认内置套餐
INSERT INTO public.subscription_plans (id, code, name, description, price_monthly, currency, max_devices, max_tenants, max_users, max_telemetry_per_day, max_api_calls_per_day, features, enabled)
VALUES
  ('plan-free', 'free', 'Community / Free', 'Free community edition plan for testing and personal projects', 0.00, 'USD', 10, 1, 3, 10000, 5000, '["basic_telemetry", "rule_chains", "dashboards"]'::jsonb, 1),
  ('plan-pro', 'pro', 'Professional', 'Professional tier for growing IoT deployments and SMBs', 99.00, 'USD', 500, 10, 25, 500000, 100000, '["basic_telemetry", "rule_chains", "dashboards", "alarm_advanced", "units_conversion", "sparkplug_b", "secrets_storage"]'::jsonb, 1),
  ('plan-enterprise', 'enterprise', 'Enterprise', 'Full enterprise tier with unlimited capabilities, SCADA, and SLA support', 499.00, 'USD', 10000, 100, 200, 10000000, 5000000, '["basic_telemetry", "rule_chains", "dashboards", "alarm_advanced", "units_conversion", "sparkplug_b", "secrets_storage", "scada_canvas", "cloud_rule_nodes", "sla_support"]'::jsonb, 1)
ON CONFLICT (code) DO NOTHING;

-- Casbin 路由登记 (g2)
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/billing/plans'),
  ('api/v1/billing/usage'),
  ('api/v1/billing/subscriptions')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 授权角色 (p)
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/billing/plans'),
  ('TENANT_ADMIN', 'api/v1/billing/plans'),
  ('TENANT_USER',  'api/v1/billing/plans'),
  ('SYS_ADMIN',    'api/v1/billing/usage'),
  ('TENANT_ADMIN', 'api/v1/billing/usage'),
  ('SYS_ADMIN',    'api/v1/billing/subscriptions'),
  ('TENANT_ADMIN', 'api/v1/billing/subscriptions')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
