-- P0.5 一次性凭证下载：预注册凭证的"只出现一次"下发通道。
--
-- 背景（先把缺陷说清楚，否则看不出这张表为什么必要）：
-- 1) 预注册凭证（devices.voucher）此前**永久以明文躺在库里**。它只在创建响应里
--    明文出现一次，但那之后任何人只要能读库、或拿到任意一次导出，就能反复取到
--    明文凭证——门禁要求的"凭证只出现一次"实际上并不成立。
-- 2) 缺一个**独立的、可审计的、消费即失效**的下载通道：创建时若没保存响应，
--    凭证就再也拿不到；而直接开放"再导一次"等于把一次性凭证变成永久可重复获取。
--
-- 设计：
--   - 一个批次同时只能有**一个** pending 许可（部分唯一索引交给数据库，
--     不做应用层"先查再写"——高并发必漏判）。重复签发被拒，杜绝无限次取明文。
--   - 下载用**条件更新** `status='pending' -> 'consumed'`，RowsAffected=0 即拒绝。
--     两个请求并发下载同一许可时只有一个能成功——"一次"由数据库守，不由 if 守。
--   - consumed_by / consumed_at 留痕：谁在什么时候取走了这批凭证，可审计。
--
-- 边界（如实记录，不在本表范围内）：
--   devices.voucher 列仍保留明文，因为 broker 侧 MQTT 基础认证要读它（双模式中的
--   plaintext 分支）。彻底消除明文需等 voucher_hash 模式全线切换，属另一项工作。
--   本表做的是：**明文的下发次数被限制为一次且可审计**，不是"库里没有明文"。

CREATE TABLE IF NOT EXISTS public.device_pre_register_credential_grants (
    id             varchar(36)  PRIMARY KEY,
    tenant_id      varchar(36)  NOT NULL,
    batch_number   varchar(36)  NOT NULL,
    device_count   integer      NOT NULL DEFAULT 0,
    status         varchar(16)  NOT NULL DEFAULT 'pending',
    expires_at     timestamptz  NOT NULL,
    created_by     varchar(36)  NULL,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    consumed_by    varchar(36)  NULL,
    consumed_at    timestamptz  NULL,
    CONSTRAINT device_pre_register_credential_grants_status_check
        CHECK (status IN ('pending', 'consumed', 'expired', 'revoked'))
);

-- 所有查询都带 tenant_id；批次维度单独建索引支持"这批有没有待消费许可"的查询。
CREATE INDEX IF NOT EXISTS device_pre_register_credential_grants_tenant_idx
    ON public.device_pre_register_credential_grants (tenant_id, created_at DESC);

-- 不变式：一个批次同一时刻至多一个 pending 许可。
CREATE UNIQUE INDEX IF NOT EXISTS device_pre_register_credential_grants_one_pending_idx
    ON public.device_pre_register_credential_grants (tenant_id, batch_number)
    WHERE status = 'pending';

-- ---- 新路由的 Casbin 资源与授权 ----
-- 不登记会让 casbin.route-audit-mode（默认 fail-fast）在启动期拒绝启动，
-- 与 90.sql / 94.sql 的两次同类修复同源。

INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/device/preRegister/credentials/grants'),
  ('api/v1/device/preRegister/credentials/grants/:id/download')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- 凭证明文属于敏感下发，只授予管理员角色；TENANT_USER 不得取走整批明文。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/device/preRegister/credentials/grants'),
  ('TENANT_ADMIN', 'api/v1/device/preRegister/credentials/grants'),
  ('SYS_ADMIN',    'api/v1/device/preRegister/credentials/grants/:id/download'),
  ('TENANT_ADMIN', 'api/v1/device/preRegister/credentials/grants/:id/download')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
