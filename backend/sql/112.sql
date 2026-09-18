-- 112.sql — TB-12 设备认领与自动注册（Device Claiming）
--
-- 背景（对标 ThingsBoard CE 的 Claiming Devices）：
--   1. 设备交付链路的"最后一公里"：当前持有租户为设备签发一次性认领令牌，
--      接收方租户凭 device_number + claim_key 认领，设备租户随之转移；
--   2. 安全边界（对应路线图 TB-12 行的"认领令牌、超时与跨租户边界"）：
--      - 明文 claim_key 仅在签发响应出现一次，库内只存 SHA-256 哈希（64 hex）；
--      - 一次性落点是数据库条件更新（WHERE status='active' AND expires_at > now()），
--        并发认领至多成功一个，其余以 RowsAffected=0 被拒；
--      - 超时即失效：redeem 与签发都以 expires_at 判活，过期不复活；
--      - 跨租户转移是事务内条件更新（WHERE tenant_id = 签发租户），
--        原租户在转移成功那一刻失去设备；认领自己租户的设备被显式拒绝。
--   3. 状态机：active -> consumed（认领成功）/ revoked（签发方撤销）/ replaced（重新签发覆盖）。
--      expired 是 expires_at 之后的**有效状态**（由读取方判定），不是落库状态——
--      过期行保持 active 落库值，靠 partial unique index 的"每设备至多一条 active"
--      与重新签发的 replaced 覆盖来维持不变量，避免后台扫表。
--   4. Casbin：登记 4 条路由（fail-fast 覆盖契约），授予 SYS_ADMIN 与 TENANT_ADMIN。
--      认领动作（redeem）允许 TENANT_ADMIN 触发；TENANT_USER 不授予——
--      认领会转移资产归属，属于租户管理员级操作。

-- ---- 1. 认领令牌表 ----
CREATE TABLE IF NOT EXISTS public.device_claim_tokens (
    id                    varchar(36)  PRIMARY KEY,
    tenant_id             varchar(36)  NOT NULL,
    device_id             varchar(36)  NOT NULL,
    device_number         varchar(64)  NOT NULL,
    claim_key_hash        varchar(64)  NOT NULL,
    status                varchar(16)  NOT NULL DEFAULT 'active',
    expires_at            timestamptz  NOT NULL,
    previous_tenant_id    varchar(36)  NULL,
    consumed_by_tenant_id varchar(36)  NULL,
    consumed_by_user_id   varchar(36)  NULL,
    consumed_at           timestamptz  NULL,
    created_at            timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT device_claim_tokens_status_check
        CHECK (status IN ('active', 'consumed', 'revoked', 'replaced')),
    CONSTRAINT device_claim_tokens_tenant_check
        CHECK (tenant_id <> ''),
    CONSTRAINT device_claim_tokens_device_check
        CHECK (device_id <> ''),
    CONSTRAINT device_claim_tokens_hash_check
        CHECK (length(claim_key_hash) = 64)
);

-- 每台设备至多一条 active 令牌：重新签发时旧行被标记 replaced 后才能插入新行。
CREATE UNIQUE INDEX IF NOT EXISTS device_claim_tokens_one_active_per_device
    ON public.device_claim_tokens (device_id) WHERE status = 'active';

-- 签发方的管理视图按设备/租户回查。
CREATE INDEX IF NOT EXISTS device_claim_tokens_tenant_device_idx
    ON public.device_claim_tokens (tenant_id, device_id, created_at DESC);

-- redeem 按令牌 id 条件消费（认领方不知道令牌行 id，由服务层先按设备寻址）。
CREATE INDEX IF NOT EXISTS device_claim_tokens_device_status_idx
    ON public.device_claim_tokens (device_id, status);

COMMENT ON TABLE public.device_claim_tokens IS
    'TB-12 设备认领令牌（明文仅签发响应出现一次，库内只存 SHA-256 哈希；一次性与过期由条件更新保证）';

-- ---- 2. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/device/claim-tokens'),
  ('api/v1/device/claim-tokens/:token_id'),
  ('api/v1/device/claim-tokens/redeem')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p) ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/device/claim-tokens'),
  ('TENANT_ADMIN', 'api/v1/device/claim-tokens'),
  ('SYS_ADMIN',    'api/v1/device/claim-tokens/:token_id'),
  ('TENANT_ADMIN', 'api/v1/device/claim-tokens/:token_id'),
  ('SYS_ADMIN',    'api/v1/device/claim-tokens/redeem'),
  ('TENANT_ADMIN', 'api/v1/device/claim-tokens/redeem')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
