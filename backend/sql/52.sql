-- Phase C1: TimescaleDB 可选时序存储后端
-- 仅在 PostgreSQL 实例已安装 timescaledb 扩展时生效。
-- 未安装时此脚本静默通过，不影响普通 PG 部署。

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
        -- 将遥测历史表转为 hypertable（如果尚未转换）
        PERFORM create_hypertable('telemetry_datas', 'time', if_not_exists => TRUE, migrate_data => TRUE);

        -- 启用压缩：7 天以上的数据自动压缩，节省 90%+ 存储空间
        ALTER TABLE telemetry_datas SET (
            timescaledb.compress,
            timescaledb.compress_segmentby = 'device_id',
            timescaledb.compress_orderby = '"time" DESC'
        );
        PERFORM add_compression_policy('telemetry_datas', INTERVAL '7 days');

        -- 告警历史表也转 hypertable（可选）
        PERFORM create_hypertable('alarm_info', 'alarm_time', if_not_exists => TRUE, migrate_data => TRUE);
    ELSE
        RAISE NOTICE 'TimescaleDB not installed, skipping hypertable conversion';
    END IF;
END $$;