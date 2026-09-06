-- 70.sql: 计算字段高级类型(PHASE-D-D4,2026-09-06)。
-- 1) calculated_fields 增加 type/config 两列:简单表达式(type=simple,存量行为不变)之外,
--    新增 timeseries_agg / related_agg / geofence / propagation 四种高级类型(TB 4.3 对齐),配置入 config JSONB;
-- 2) calcfield_recompute_tasks 历史重算任务表(状态机 pending/running/done/failed + 进度,幂等重放)。
-- 幂等性:IF NOT EXISTS / NOT EXISTS 守卫,重放无副作用;回滚:DROP TABLE 任务表 + DROP COLUMN 两列即可。

ALTER TABLE calculated_fields ADD COLUMN IF NOT EXISTS type VARCHAR(32) NOT NULL DEFAULT 'simple';
ALTER TABLE calculated_fields ADD COLUMN IF NOT EXISTS config JSONB;

CREATE TABLE IF NOT EXISTS calcfield_recompute_tasks (
    id          VARCHAR(36) PRIMARY KEY,
    tenant_id   VARCHAR(36) NOT NULL,
    field_id    VARCHAR(36) NOT NULL,
    device_id   VARCHAR(36) NOT NULL,
    from_ts     BIGINT NOT NULL,
    to_ts       BIGINT NOT NULL,
    status      VARCHAR(16) NOT NULL DEFAULT 'pending',
    processed   BIGINT NOT NULL DEFAULT 0,
    emitted     BIGINT NOT NULL DEFAULT 0,
    error_msg   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cfrt_tenant_status ON calcfield_recompute_tasks (tenant_id, status, created_at DESC);
