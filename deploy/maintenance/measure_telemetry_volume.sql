-- =============================================================================
-- measure_telemetry_volume.sql — Wave7-D（时序表原生分区）立项度量脚本
--
-- 用途：一次性取齐「是否值得做原生分区」所需的全部事实。
--       计划文档 docs/deep-refactor-wave7-plan-2026-09-30.md 第 50-51 行列的前置条件里，
--       「真实数据量压力数据」与「与 138/141.sql 的交互口径」两项可由本脚本回答；
--       剩下两项（停机/双写窗口、Timescale 与非 Timescale 双路径）是运维/产品决策，
--       分析见 docs/wave7-d-partitioning-spec-2026-10-01.md。
--
-- 安全性：全脚本只读（SELECT + 系统视图），不改任何数据与结构。
--         除第 7 段（默认注释掉）外均秒级返回。
--
-- 兼容性：目标库的迁移版本可能落后于代码（实测本地 aetherlink_go99 就停在 138.sql 之前），
--         因此每段都先做「能力探测」再决定是否执行，缺特性时打印说明而不是抛错。
--
-- 用法：
--   psql "$AETHERLINK_DSN" -f deploy/maintenance/measure_telemetry_volume.sql
--   建议业务低峰执行；输出整段回贴到 Wave7-D 立项 issue。
-- =============================================================================

\set ON_ERROR_STOP off
\pset pager off

-- -----------------------------------------------------------------------------
\echo ''
\echo '==================================================================='
\echo '第 0 段：能力探测（决定后面各段是否执行）'
\echo '==================================================================='

SELECT
  EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb')                     AS has_ts,
  to_regclass('public.telemetry_datas') IS NOT NULL                                     AS has_telemetry,
  to_regclass('public.alarm_info') IS NOT NULL                                          AS has_alarm,
  EXISTS (SELECT 1 FROM information_schema.columns
          WHERE table_name = 'data_policy' AND column_name = 'tenant_id')               AS has_rowlevel_ttl,
  to_regclass('public.data_retention_registry') IS NOT NULL                             AS has_retention_registry
\gset

\echo '  TimescaleDB 扩展已安装        : ' :has_ts
\echo '  telemetry_datas 表存在        : ' :has_telemetry
\echo '  alarm_info 表存在             : ' :has_alarm
\echo '  138.sql 行级 TTL 已应用       : ' :has_rowlevel_ttl
\echo '  141.sql 保留注册表已应用      : ' :has_retention_registry
\echo ''
\echo '  读法：'
\echo '    has_ts = t  → 走 Timescale 路径，57.sql 已把表转成 hypertable，本项优先级大幅下降'
\echo '    has_ts = f  → 普通 PG 路径，无任何原生分区，本项才有实际价值'
\echo '    has_rowlevel_ttl = f → 不存在租户级/档案级保留期，整分区 DROP 无冲突，可直接用方案 B'

-- -----------------------------------------------------------------------------
\echo ''
\echo '==================================================================='
\echo '第 1 段：telemetry_datas 体量与时间跨度'
\echo '==================================================================='

\if :has_telemetry

SELECT
  c.relname                                                               AS table_name,
  c.reltuples::bigint                                                     AS est_rows,
  pg_size_pretty(pg_total_relation_size(c.oid))                           AS total_size,
  pg_size_pretty(pg_relation_size(c.oid))                                 AS heap_size,
  pg_size_pretty(pg_total_relation_size(c.oid) - pg_relation_size(c.oid)) AS indexes_size,
  c.relkind                                                               AS relkind,
  c.relispartition                                                        AS is_partition
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relname = 'telemetry_datas'
  AND n.nspname = 'public';

\echo '  relkind: r=普通表 p=分区父表   is_partition: t=本表自身是分区'
\echo '  若 relkind=p 或 is_partition=t → 已分区，本项已落地，勿重复改造'

SELECT
  to_timestamp(MIN(ts) / 1000.0)  AS first_sample_at,
  to_timestamp(MAX(ts) / 1000.0)  AS last_sample_at,
  (MAX(ts) - MIN(ts)) / 86400000  AS span_days
FROM telemetry_datas;

\else
\echo '  telemetry_datas 不存在 —— 该库未初始化到含该表的迁移版本，本段跳过'
\endif

-- -----------------------------------------------------------------------------
\echo ''
\echo '==================================================================='
\echo '第 2 段：增长速度（决定分区粒度与保留窗口）'
\echo '==================================================================='

\if :has_telemetry

SELECT
  to_timestamp(ts / 1000.0)::date AS day,
  COUNT(*)                        AS rows_written
FROM telemetry_datas
WHERE ts >= (EXTRACT(EPOCH FROM now() - interval '30 days') * 1000)::bigint
GROUP BY 1
ORDER BY 1 DESC;

WITH daily AS (
  SELECT
    to_timestamp(ts / 1000.0)::date AS day,
    COUNT(*)                        AS rows_written
  FROM telemetry_datas
  WHERE ts >= (EXTRACT(EPOCH FROM now() - interval '30 days') * 1000)::bigint
  GROUP BY 1
)
SELECT
  COUNT(*)                                   AS days_observed,
  ROUND(AVG(rows_written))                   AS avg_rows_per_day,
  MAX(rows_written)                          AS peak_rows_per_day,
  ROUND(AVG(rows_written) * 30)              AS est_rows_per_month,
  CASE
    WHEN COALESCE(AVG(rows_written), 0) > 0
    THEN ROUND(1000000000.0 / AVG(rows_written) / 30)::text || ' 个月后达 10 亿行'
    ELSE '近 30 天无写入'
  END                                        AS time_to_1e9_rows
FROM daily;

\else
\echo '  telemetry_datas 不存在，本段跳过'
\endif

-- -----------------------------------------------------------------------------
\echo ''
\echo '==================================================================='
\echo '第 3 段：膨胀与清理可行性（分区改造的必要性证据）'
\echo '==================================================================='

-- 注意：n_live_tup / n_dead_tup 来自统计收集器，库刚建或长期未 ANALYZE 时会是 0，
--       此时本段数据不可用——先对该表跑一次 ANALYZE 再采集。
SELECT
  schemaname,
  relname,
  n_live_tup,
  n_dead_tup,
  CASE WHEN n_live_tup + n_dead_tup > 0
       THEN ROUND(100.0 * n_dead_tup / (n_live_tup + n_dead_tup), 2)
       ELSE NULL END                    AS dead_pct,
  last_analyze,
  last_autoanalyze,
  last_autovacuum,
  autovacuum_count
FROM pg_stat_user_tables
WHERE relname IN ('telemetry_datas', 'telemetry_current_datas', 'alarm_info')
ORDER BY relname;

\echo '  判定：dead_pct 持续 > 30% 且 autovacuum 追不上 → 批删代价高，分区收益明显'
\echo '        全为 NULL / 0 → 统计未收集，先 ANALYZE 再判读'

-- -----------------------------------------------------------------------------
\echo ''
\echo '==================================================================='
\echo '第 4 段：TimescaleDB 侧现状'
\echo '==================================================================='

\if :has_ts

SELECT
  hypertable_name,
  num_dimensions,
  num_chunks,
  compression_enabled,
  pg_size_pretty(total_bytes) AS total_size
FROM timescaledb_information.hypertables
WHERE hypertable_name IN ('telemetry_datas', 'alarm_info')
ORDER BY hypertable_name;

SELECT
  job_id,
  application_name,
  schedule_interval,
  hypertable_name,
  config
FROM timescaledb_information.jobs
WHERE application_name IN ('policy_retention', 'policy_compression')
ORDER BY application_name, hypertable_name;

\else
\echo '  TimescaleDB 未安装 —— 本段跳过。'
\echo '  这恰好证明当前部署走的是普通 PG 路径：telemetry_datas 无任何原生分区，'
\echo '  清理压力全压在 CleanSystemDataByCron 的分批 DELETE 上。这正是 Wave7-D 要解决的场景。'
\endif

-- -----------------------------------------------------------------------------
\echo ''
\echo '==================================================================='
\echo '第 5 段：既有保留机制配置（回答「与 138/141.sql 的交互口径」）'
\echo '==================================================================='

\echo '--- data_policy ---'

\if :has_rowlevel_ttl

SELECT
  id,
  data_type,
  retention_days,
  tenant_id,
  device_config_id,
  CASE
    WHEN tenant_id IS NULL AND device_config_id IS NULL THEN '全局默认'
    WHEN device_config_id IS NULL                       THEN '租户级'
    ELSE '档案级'
  END AS scope_level
FROM data_policy
ORDER BY scope_level, id;

SELECT COUNT(*) AS scoped_policy_rows
FROM data_policy
WHERE tenant_id IS NOT NULL OR device_config_id IS NOT NULL;

\echo '  ★ scoped_policy_rows = 0 → 没有任何作用域行，整分区 DROP 完全安全，直接批方案 B'
\echo '  ★ scoped_policy_rows > 0 → 存在行级保留期，整分区 DROP 会误删长保留租户的数据，'
\echo '     只能走方案 A（分区用于裁剪，过期仍行级 DELETE）或 B（按最大保留期 DROP）'

\else

SELECT id, data_type, retention_days, enabled FROM data_policy ORDER BY id;

\echo '  138.sql 未应用（data_policy 无 tenant_id 列）→ 当前不存在行级作用域行，'
\echo '  整分区 DROP 无冲突风险 → 可直接采用方案 B（按全局保留期 DROP 过期分区）。'
\echo '  但注意：若计划随后应用 138.sql，本结论会失效，需重新评估。'

\endif

\echo '--- data_retention_registry ---'

\if :has_retention_registry

SELECT
  category,
  COUNT(*)                              AS tables_registered,
  COUNT(*) FILTER (WHERE enabled = '1') AS enabled_count,
  COUNT(*) FILTER (WHERE enabled <> '1') AS disabled_count
FROM data_retention_registry
GROUP BY category
ORDER BY category;

\else
\echo '  141.sql 未应用（无 data_retention_registry 表）—— 本段跳过。'
\echo '  该注册表覆盖的是另一批表（event_datas / 死信表等），不含 telemetry_datas，'
\echo '  与分区改造不冲突，也不需改 141.sql。'
\endif

-- -----------------------------------------------------------------------------
\echo ''
\echo '==================================================================='
\echo '第 6 段：按设备分布（判断是否需要二级分区维度）'
\echo '==================================================================='

\if :has_telemetry

SELECT
  COUNT(DISTINCT device_id) AS distinct_devices_24h
FROM telemetry_datas
WHERE ts >= (EXTRACT(EPOCH FROM now() - interval '1 day') * 1000)::bigint;

\echo '  设备数很大（>10 万）时，二级分区按 tenant 尚可，按 device 不可行（分区数爆炸）'

\else
\echo '  telemetry_datas 不存在，本段跳过'
\endif

-- -----------------------------------------------------------------------------
\echo ''
\echo '==================================================================='
\echo '第 7 段（默认关闭）：精确行数 —— 大表上全表扫描，仅在低峰执行'
\echo '==================================================================='
\echo '  需要精确值（而非 reltuples 估算）时，手动取消下面两行注释：'
-- SELECT COUNT(*) AS exact_rows FROM telemetry_datas;
-- SELECT device_id, COUNT(*) AS rows_per_device FROM telemetry_datas GROUP BY 1 ORDER BY 2 DESC LIMIT 20;

\echo ''
\echo '=== 采集完成。请把以上完整输出回贴到 Wave7-D 立项 issue。==='
