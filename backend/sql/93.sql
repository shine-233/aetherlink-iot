-- P1.2 规则链草稿/发布版本与回滚：持久化表 + 新路由的 Casbin 登记。
--
-- 背景：
-- 1) 版本语义（draft -> published 单向、published 只读、回滚造新草稿而非回写）
--    已在 service/rule_chain_version.go 落地并有定向证据，但一直没有落库的地方。
-- 2) 本次新增的三个路由必须登记进 casbin_rule，否则 casbin.route-audit-mode
--    默认 fail-fast 会让后端在启动期直接拒绝启动。
--
-- 全部语句均幂等（IF NOT EXISTS / WHERE NOT EXISTS），可重跑。

CREATE TABLE IF NOT EXISTS public.rule_chain_versions (
    id                varchar(36)  PRIMARY KEY,
    tenant_id         varchar(36)  NOT NULL,
    chain_id          varchar(36)  NOT NULL,
    version           integer      NOT NULL,
    status            varchar(16)  NOT NULL DEFAULT 'draft',
    graph_hash        varchar(128) NOT NULL,
    graph             jsonb        NULL,
    rolled_back_from  integer      NULL,
    created_at        timestamptz  NOT NULL DEFAULT now(),
    published_at      timestamptz  NULL,
    CONSTRAINT rule_chain_versions_status_check
        CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT rule_chain_versions_chain_version_unique
        UNIQUE (chain_id, version)
);

-- 版本只在同一条链内递增，故唯一约束取 (chain_id, version)；
-- tenant_id 进入索引以保证所有查询都能走租户过滤。
CREATE INDEX IF NOT EXISTS rule_chain_versions_tenant_chain_idx
    ON public.rule_chain_versions (tenant_id, chain_id, status);

-- 一条链同一时刻只应有一个 published 版本，用部分唯一索引把这条不变式交给数据库，
-- 而不是靠应用层的"先查再写"（高并发下会漏判）。
CREATE UNIQUE INDEX IF NOT EXISTS rule_chain_versions_single_published_idx
    ON public.rule_chain_versions (chain_id)
    WHERE status = 'published';

-- ---- 新路由的 Casbin 资源与授权 ----

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/rule-chains/:id/versions'),
  ('api/v1/rule-chains/versions/publish'),
  ('api/v1/rule-chains/versions/rollback')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 发布与回滚会改变线上执行的图，只授予管理员角色。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/rule-chains/:id/versions'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/:id/versions'),
  ('SYS_ADMIN',    'api/v1/rule-chains/versions/publish'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/versions/publish'),
  ('SYS_ADMIN',    'api/v1/rule-chains/versions/rollback'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/versions/rollback')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
