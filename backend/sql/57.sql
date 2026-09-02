-- 57.sql: TimescaleDB 可选时序存储后端（ROADMAP C1）
-- 背景：对标 ThingsBoard CE 的 TimescaleDB 支持。仅在 PostgreSQL 实例已安装
--       timescaledb 扩展时生效；未安装时静默跳过，不影响普通 PG 部署。
-- 边界：telemetry_datas 的 UNIQUE(device_id,key,ts) 唯一约束含时间列且无独立 PK，
--       符合 hypertable 分区要求，可直接转换；alarm_info 原主键为独立 id，
--       必须先改复合主键 (id, alarm_time) 再转换（TimescaleDB 要求主键含分区列）。
-- 回滚：见 https://docs.timescale.com 的分区回退方式；一般保留 hypertable 更优。

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
        -- 遥测历史表转 hypertable；时间列为毫秒时间戳（UnixMilli），分块按 1 天。
        PERFORM create_hypertable('telemetry_datas', 'ts',
            chunk_time_interval => 86400000, if_not_exists => TRUE, migrate_data => TRUE);

        -- 启用压缩：7 天以上的数据自动压缩，节省存储空间。
        ALTER TABLE telemetry_datas SET (
            timescaledb.compress,
            timescaledb.compress_segmentby = 'device_id',
            timescaledb.compress_orderby = '"ts" DESC'
        );
        PERFORM add_compression_policy('telemetry_datas', INTERVAL '7 days');

        -- 告警历史表：先改复合主键（含分区时间列），再转 hypertable。
        ALTER TABLE alarm_info DROP CONSTRAINT IF EXISTS alarm_info_pk;
        PERFORM create_hypertable('alarm_info', 'alarm_time',
            if_not_exists => TRUE, migrate_data => TRUE);
        ALTER TABLE alarm_info ADD CONSTRAINT alarm_info_pk PRIMARY KEY (id, alarm_time);
    ELSE
        RAISE NOTICE 'TimescaleDB not installed, skipping hypertable conversion';
    END IF;
END $$;