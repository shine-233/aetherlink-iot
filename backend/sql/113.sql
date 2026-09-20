-- 113.sql — TB-19 解决方案模板引擎（Industry Solution Templates）
--
-- 背景（对标 ThingsBoard CE 的解决方案模板引擎，DefaultSolutionService）：
--   1. 「一键装一套行业方案」：一个 solution 是**有序资源引用清单**
--      （设备物模型模板 + 看板模板），安装 = 逐项走资源中心已验证的
--      export→import 应用管道（ApplyResource），并留逐项安装流水；
--   2. 与 TP-5 / P1.6 的关系：本表只存「引用」与「安装流水」，不复制资源内容——
--      打包/签名/冲突预览/覆盖闸门全部复用既有链路，不建第二套；
--   3. 语义边界（如实）：物模型模板导入是租户幂等的（P1.6 语义），看板导入
--      每次实例化新看板——即「每次安装装出一套新方案实例」，与 TB
--      解决方案模板「每次安装创建新资产」一致；重复安装不做合并。
--   4. Casbin：登记 3 条路由（fail-fast 覆盖契约），授予 SYS_ADMIN 与 TENANT_ADMIN。

-- ---- 1. 方案定义表 ----
CREATE TABLE IF NOT EXISTS public.industry_solutions (
    id          varchar(36)  PRIMARY KEY,
    tenant_id   varchar(36)  NOT NULL,
    name        varchar(128) NOT NULL,
    description varchar(512) NULL,
    -- 有序资源引用：[{"resource_type":"device_template|board_template","resource_id":"...","target_name":"可选"}]
    resources   jsonb        NOT NULL,
    status      varchar(16)  NOT NULL DEFAULT 'active',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT industry_solutions_status_check
        CHECK (status IN ('active', 'disabled')),
    CONSTRAINT industry_solutions_tenant_check
        CHECK (tenant_id <> ''),
    CONSTRAINT industry_solutions_name_check
        CHECK (length(name) > 0)
);

-- 同租户内方案名唯一（模板目录按名管理）。
CREATE UNIQUE INDEX IF NOT EXISTS industry_solutions_tenant_name_uniq
    ON public.industry_solutions (tenant_id, name);

CREATE INDEX IF NOT EXISTS industry_solutions_tenant_idx
    ON public.industry_solutions (tenant_id, status, created_at DESC);

-- ---- 2. 安装流水表（append-only 审计） ----
CREATE TABLE IF NOT EXISTS public.industry_solution_installs (
    id            varchar(36)  PRIMARY KEY,
    tenant_id     varchar(36)  NOT NULL,
    solution_id   varchar(36)  NOT NULL,
    solution_name varchar(128) NOT NULL,
    item_index    integer      NOT NULL,
    resource_type varchar(32)  NOT NULL,
    resource_id   varchar(36)  NOT NULL,
    target_id     varchar(36)  NULL,
    status        varchar(16)  NOT NULL,
    error         text         NULL,
    created_at    timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT industry_solution_installs_status_check
        CHECK (status IN ('applied', 'failed')),
    CONSTRAINT industry_solution_installs_tenant_check
        CHECK (tenant_id <> '')
);

CREATE INDEX IF NOT EXISTS industry_solution_installs_solution_idx
    ON public.industry_solution_installs (tenant_id, solution_id, created_at DESC);

COMMENT ON TABLE public.industry_solutions IS
    'TB-19 行业方案模板（有序资源引用清单；安装复用资源中心应用管道，不复制内容）';
COMMENT ON TABLE public.industry_solution_installs IS
    'TB-19 方案安装流水（append-only，逐项 applied/failed 与目标实例 ID）';

-- ---- 3. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/solutions'),
  ('api/v1/solutions/:id'),
  ('api/v1/solutions/:id/install')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 4. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/solutions'),
  ('TENANT_ADMIN', 'api/v1/solutions'),
  ('SYS_ADMIN',    'api/v1/solutions/:id'),
  ('TENANT_ADMIN', 'api/v1/solutions/:id'),
  ('SYS_ADMIN',    'api/v1/solutions/:id/install'),
  ('TENANT_ADMIN', 'api/v1/solutions/:id/install')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
