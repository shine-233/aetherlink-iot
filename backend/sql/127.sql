-- 127.sql — 实体级审计日志（TB-10，对标 ThingsBoard Audit Logs 的动作/实体/状态模型）
--
-- 背景：
--   1. ROADMAP §4.1 TB-10：现有 operation_logs 仅为 HTTP 级操作日志（方法/路径/耗时/载荷），
--      无 TB 式实体级审计（动作、实体类型、实体 ID、响应状态）；
--   2. 本迁移给 operation_logs 增加四个可空列，全部由中间件在落库时从请求上下文推导：
--      - action       VARCHAR(32)：HTTP 方法映射动作（POST→create、PUT/PATCH→update、
--        DELETE→delete、GET/HEAD→read、其他→other，见 middleware/operation_entity.go）；
--      - entity_type  VARCHAR(64)：自请求路径 /api/v1/<entity>[/<id>] 解析的首段；
--      - entity_id    VARCHAR(36)：路径第二段且需为 UUID 形态（本平台实体主键均为 varchar(36)
--        UUID；动词形第二段如 /device/update/voucher 不会误记为实体 ID）；
--      - status_code  INT：中间件拿到的 c.Writer.Status()；
--   3. 旧数据兼容：四列全部 NULL 允许 + ADD COLUMN IF NOT EXISTS 幂等；存量行保持 NULL，
--      列表/导出接口对 NULL 显示为空，不回填（回填需要重放请求语义，超出 v1 范围）。

-- ---- 1. operation_logs 增加实体级审计列 ----
ALTER TABLE public.operation_logs
    ADD COLUMN IF NOT EXISTS action VARCHAR(32);

ALTER TABLE public.operation_logs
    ADD COLUMN IF NOT EXISTS entity_type VARCHAR(64);

ALTER TABLE public.operation_logs
    ADD COLUMN IF NOT EXISTS entity_id VARCHAR(36);

ALTER TABLE public.operation_logs
    ADD COLUMN IF NOT EXISTS status_code INT;

COMMENT ON COLUMN public.operation_logs.action IS '实体级动作（create/update/delete/read/other，映射自 HTTP 方法；旧数据为空）';
COMMENT ON COLUMN public.operation_logs.entity_type IS '审计实体类型（/api/v1/<entity> 首段；旧数据为空）';
COMMENT ON COLUMN public.operation_logs.entity_id IS '审计实体ID（路径第二段 UUID 形态；旧数据为空）';
COMMENT ON COLUMN public.operation_logs.status_code IS 'HTTP响应状态码（中间件 writer 状态；旧数据为空）';

-- ---- 2. 实体维度审计检索索引 ----
-- 列表筛选（租户 + 实体类型/ID）与契约测试"操作后查审计行"走该组合条件；
-- partial index 只覆盖已写入实体列的新数据，存量 NULL 行不进索引，体积可控。
CREATE INDEX IF NOT EXISTS idx_operation_logs_tenant_entity
    ON public.operation_logs (tenant_id, entity_type, entity_id)
    WHERE entity_type IS NOT NULL;
