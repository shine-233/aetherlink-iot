-- 125.sql — 设备 Profile 档案级默认规则链（TB-18，对标 ThingsBoard PE Device Profile 的 Default Rule Chain）
--
-- 背景：
--   1. 对标 TB PE 设备档案（Device Profile）可指定默认规则链的能力：本迁移给设备配置
--      （device_configs，即本项目的"设备档案/Profile"）增加可空的 default_rule_chain_id 列；
--   2. 生效语义：档案绑定的链优先执行、租户级启用链兜底（service.GetEffectiveRuleChainsForDevice），
--      解绑后自动回落租户级链，不中断上行执行；
--   3. 执行解析查询端点：GET /api/v1/rule-chains/device-effective/:deviceId
--      （档案绑定/解绑复用既有 PUT /api/v1/device_config，不新增写端点）；
--   4. 档案级默认队列（平台当前为全局三队列，无按档案维度）与告警规则收敛为 Profile 字段
--      明确不在本批次范围（场景联动间接实现保留）。

-- ---- 1. device_configs 增加档案级默认规则链列 ----
-- rule_chains.id 在 53.sql 中为 UUID 类型，故本列同为 UUID 才能建立外键；
-- 可空 = 未绑定（执行面回落租户级链）。gorm 模型字段 DefaultRuleChainID 与之对应。
ALTER TABLE public.device_configs
    ADD COLUMN IF NOT EXISTS default_rule_chain_id UUID;

COMMENT ON COLUMN public.device_configs.default_rule_chain_id IS '档案级默认规则链id（可空；空=回落租户级启用链）';

-- 外键约束：幂等写法（PG 的 ADD CONSTRAINT 无 IF NOT EXISTS，先 DROP 再 ADD 收敛到同一终态）。
-- ON DELETE RESTRICT：链仍被档案引用时拒绝删除（service.DeleteChain 侧有友好预检，这里兜底 fail-closed）。
ALTER TABLE public.device_configs
    DROP CONSTRAINT IF EXISTS device_configs_default_rule_chain_fk;
ALTER TABLE public.device_configs
    ADD CONSTRAINT device_configs_default_rule_chain_fk
    FOREIGN KEY (default_rule_chain_id) REFERENCES public.rule_chains (id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_device_configs_default_rule_chain_id
    ON public.device_configs (default_rule_chain_id);

-- ---- 2. Casbin 路由登记 (g2) ----
-- 执行解析为读端点；路径含参数段，与 111.sql 的 rule-chains 子路径登记口径一致。
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/rule-chains/device-effective/:deviceId')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 3. 授权角色 (p)：与 63.sql 的 rule-chains 读面授权一致（三内置角色） ----
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/rule-chains/device-effective/:deviceId'),
  ('TENANT_ADMIN', 'api/v1/rule-chains/device-effective/:deviceId'),
  ('TENANT_USER',  'api/v1/rule-chains/device-effective/:deviceId')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
