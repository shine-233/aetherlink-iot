-- 138.sql — TB-15R 行级数据保留 TTL（档案/租户粒度保留）
--
-- 背景：
--   1. data_policy 原本只有两条全局默认行（1.sql 种子：id='a' 设备数据 30 天、
--      id='b' 操作日志 15 天），CleanSystemDataByCron 按它做全库分批 DELETE，
--      TimescaleDB retention 也按全局设备数据策略装配（57.sql 口径）。TB-15R 补上
--      "档案/租户粒度保留"：某个租户或某个设备档案（device_configs，即 Profile）可以
--      有自己的保留天数，精确作用域优先、全局默认回落；
--   2. 本迁移给 data_policy 增加两个可空列：
--        tenant_id         NULL=全局默认（既有两行语义不变）；非空=租户级行
--        device_config_id  NULL=租户内全部设备（配合非空 tenant_id）；非空=精确档案行
--      作用域层级（service/datapolicy.go 解析，精确优先）：
--        档案级 (tenant_id, device_config_id) > 租户级 (tenant_id, NULL) > 全局 (NULL, NULL)；
--   3. 清理执行面边界（明确不做，见缺口 residual）：TimescaleDB retention job 不按
--      行级动态重建——行级 TTL 一律走 CleanSystemDataByCron 的分批 DELETE（本迁移
--      不触碰 add_retention_policy）；全局 retention 仍按全局设备数据策略装配，行级
--      覆盖的设备由 Go 清理路径排除，不与 TimescaleDB 的全局 drop_after 叠加误删；
--   4. 行级策略只支持设备数据（data_type='1'）：操作日志无档案维度，服务层创建入口
--      拒绝 data_type='2' 的行级行，清理作业对存量此类行跳过并告警；
--   5. 唯一性：部分唯一索引只约束行级行（tenant_id IS NOT NULL），全局两行不受约束；
--      device_config_id 以 COALESCE 归一后参与唯一性，使 (租户, NULL) 租户级行也唯一
--      （PG 唯一索引默认把 NULL 视为互不相等，直接 UNIQUE(tenant_id, device_config_id)
--      挡不住同租户多条租户级行）。
--   6. Casbin：新增 POST /api/v1/datapolicy（创建行级）与 DELETE /api/v1/datapolicy/:id
--      （删除行级）。g2/p 幂等登记；服务层 requireDataPolicyAdmin 仍强制 SYS_ADMIN。

-- ---- 1. data_policy 行级作用域列 ----
ALTER TABLE public.data_policy
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36);
ALTER TABLE public.data_policy
    ADD COLUMN IF NOT EXISTS device_config_id VARCHAR(36);

COMMENT ON COLUMN public.data_policy.tenant_id IS '行级策略租户id（TB-15R；NULL=全局默认，既有两行语义不变）';
COMMENT ON COLUMN public.data_policy.device_config_id IS '行级策略设备档案id（TB-15R；NULL=该租户全部设备；tenant_id 为空时本列无意义）';

-- ---- 2. 行级唯一性（部分唯一索引，幂等） ----
-- 只约束 tenant_id IS NOT NULL 的行级行；COALESCE 把"无档案"归一为空串参与唯一性，
-- 使 (租户, 档案) 与 (租户, NULL) 两个粒度各自唯一，重复创建在 DB 层被 23505 拒绝。
CREATE UNIQUE INDEX IF NOT EXISTS uq_data_policy_row_level
    ON public.data_policy (tenant_id, COALESCE(device_config_id, ''))
    WHERE tenant_id IS NOT NULL;

-- ---- 3. Casbin 路由登记 (g2) ----
INSERT INTO casbin_rule (ptype, v0, v1)
SELECT 'g2', r.path, r.path
FROM (VALUES
  ('api/v1/datapolicy'),
  ('api/v1/datapolicy/:id')
) AS r(path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c WHERE c.ptype = 'g2' AND c.v0 = r.path AND c.v1 = r.path
);

-- ---- 4. 授权角色 (p)：与 63.sql 的 datapolicy 授权口径一致 ----
-- 行级策略是平台管理面，服务层 requireDataPolicyAdmin 仍强制 SYS_ADMIN。
INSERT INTO casbin_rule (ptype, v0, v1, v2)
SELECT 'p', r.role, r.path, 'allow'
FROM (VALUES
  ('SYS_ADMIN',    'api/v1/datapolicy'),
  ('SYS_ADMIN',    'api/v1/datapolicy/:id'),
  ('TENANT_ADMIN', 'api/v1/datapolicy'),
  ('TENANT_ADMIN', 'api/v1/datapolicy/:id')
) AS r(role, path)
WHERE NOT EXISTS (
  SELECT 1 FROM casbin_rule c
  WHERE c.ptype = 'p' AND c.v0 = r.role AND c.v1 = r.path AND c.v2 = 'allow'
);
