-- =============================================================================
-- rollback_partition_telemetry_datas.sql — 回滚 partition_telemetry_datas.sql
--
-- 用途：把分区形态还原成迁移前的普通表。
--   迁移后：telemetry_datas = 分区父表（不存数据）；telemetry_datas_legacy = 持有全部行的分区
--   回滚后：telemetry_datas = 普通表（持有全部行）
--
-- 为什么回滚是安全的：迁移用 ATTACH PARTITION 把既有表直接挂成分区，**没有搬运数据**，
--   所以回滚同样只是元数据操作 —— DETACH + 两次 RENAME + DROP 空父表。行数不变。
--
-- 用法（必须用 psql）：
--   psql -h <host> -p <port> -U <user> -d <db> -f deploy/maintenance/rollback_partition_telemetry_datas.sql
--
-- 幂等：已回滚时直接跳过。
--
-- ⚠️ 回滚会**丢弃迁移后新建的未来分区**（telemetry_datas_YYYY_MM）。若这些分区里
--    已经写入了数据，回滚前必须先把它搬回主表，否则会随 DROP 一起丢失。
--    脚本会**真实计数**这些分区，非空则拒绝执行。
--    （不用 pg_stat_user_tables.n_live_tup —— 该统计可能长期未刷新而恒为 0，不可靠。）
-- =============================================================================

\set ON_ERROR_STOP on
\pset pager off

-- 前置 1：确认当前确实是「已迁移」形态
SELECT (relkind = 'p') AS is_partitioned
FROM pg_class WHERE relname = 'telemetry_datas'
\gset

\if :is_partitioned
\else
  \echo 'telemetry_datas 不是分区父表，无需回滚。'
  \quit
\endif

-- 前置 2：legacy 分区必须存在
SELECT EXISTS (
    SELECT 1 FROM pg_class WHERE relname = 'telemetry_datas_legacy' AND relkind = 'r'
) AS has_legacy
\gset

\if :has_legacy
\else
  \echo '!! 找不到 telemetry_datas_legacy 分区，形态不符合预期，拒绝回滚。'
  \quit
\endif

-- 前置 3：未来分区必须为空 —— 用真实 COUNT 而不是统计视图
DO $wave7d_rb_check$
DECLARE
    r      record;
    cnt    bigint;
    total  bigint := 0;
BEGIN
    FOR r IN
        SELECT c.relname
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relkind = 'r'
          AND c.relispartition
          AND c.relname ~ '^telemetry_datas_[0-9]{4}_[0-9]{2}$'
    LOOP
        EXECUTE format('SELECT count(*) FROM public.%I', r.relname) INTO cnt;
        IF cnt > 0 THEN
            total := total + cnt;
            RAISE WARNING '未来分区 % 有 % 行', r.relname, cnt;
        END IF;
    END LOOP;

    IF total > 0 THEN
        RAISE EXCEPTION '未来分区共 % 行数据，回滚会丢失它们。请先搬回主表或导出备份。', total;
    END IF;

    RAISE NOTICE '未来分区均为空，可安全回滚';
END
$wave7d_rb_check$;

-- -----------------------------------------------------------------------------
-- 回滚三步（同一事务内）
--   1. 把 legacy 分区从父表摘下来
--   2. 父表让出 telemetry_datas 这个名字
--   3. legacy 接管这个名字
-- -----------------------------------------------------------------------------
BEGIN;
  SET LOCAL lock_timeout = '5s';
  ALTER TABLE public.telemetry_datas DETACH PARTITION public.telemetry_datas_legacy;
  ALTER TABLE public.telemetry_datas RENAME TO telemetry_datas_p;
  ALTER TABLE public.telemetry_datas_legacy RENAME TO telemetry_datas;
COMMIT;

-- 4. 丢弃已空的分区父表（连同它的未来分区）
DROP TABLE IF EXISTS public.telemetry_datas_p;

-- 5. 清掉迁移时加的 CHECK 约束（它是为 ATTACH 服务的，回滚后不再需要）
ALTER TABLE public.telemetry_datas DROP CONSTRAINT IF EXISTS telemetry_datas_ts_partition_ck;

\echo ''
\echo '=== 回滚结果自检 ==='
SELECT
  c.relname,
  c.relkind,
  c.relispartition,
  pg_size_pretty(pg_total_relation_size(c.oid)) AS size
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relname LIKE 'telemetry\_datas%'
  AND c.relkind IN ('r', 'p')
ORDER BY c.relname;

\echo ''
\echo 'telemetry_datas 应恢复为普通表（relkind=r、relispartition=f），且持有全部行。'
