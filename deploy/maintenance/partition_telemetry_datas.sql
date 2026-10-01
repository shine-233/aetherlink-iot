-- =============================================================================
-- partition_telemetry_datas.sql — 时序表原生分区（普通 PG 路径）（Wave7-D）
--
-- 为什么是带外脚本、而不是 backend/sql/NNN.sql：
--   1. 本脚本用了 psql 元命令（\gset / \if / \echo），**项目的自动迁移链跑不了**——
--      它按普通 SQL 执行，不认识反斜杠命令。
--   2. 计划文档明确把分区改造归为「需带外迁移步与停机/双写窗口」，不该在
--      pg_init.go 的单事务迁移链里静默执行。
--   3. 它改变的是物理存储形态（表 -> 分区父表），需要运维确认维护窗口后**主动**执行。
--
-- 用法（必须用 psql，不能用其它 SQL 客户端）：
--   pg_dump -h <host> -p <port> -U <user> -d <db> -t telemetry_datas -f td_backup.sql   # 先备份
--   psql -h <host> -p <port> -U <user> -d <db> -f deploy/maintenance/partition_telemetry_datas.sql
--
-- 实现上的两个坑（改脚本时务必保留）：
--   a. **psql 不会在 dollar-quoted 块（$tag$...$tag$）内替换 :var** —— 所以所有依赖
--      分区边界的语句都不能写在 DO 块里，要用 \gset + \if 走 psql 侧替换。
--      唯一例外是第 6 步的循环：它通过 set_config 把值塞进会话 GUC，DO 块内用
--      current_setting 读回，从而绕开该限制。
--   b. 所有条件判断都用 \gset 取一个布尔标志再 \if，保证**幂等**（可重复执行）。
--
-- 背景与边界：
--   1. 本迁移**只服务普通 PG 部署**。装了 timescaledb 的库由 57.sql 转 hypertable
--      （hypertable 本身就是分区表），再套声明式分区会冲突，故开头有硬守卫。
--   2. 只解决「分区裁剪 + 避免全表 vacuum」，**不改变过期语义**。过期删除仍由
--      CleanSystemDataByCron 的分批 DELETE（138.sql 行级 TTL）负责。
--      原因见 docs/wave7-d-partitioning-spec-2026-10-01.md 第 2 节：
--      138.sql 支持档案级/租户级保留期，而一个时间分区覆盖所有租户，
--      按全局保留期 DROP 分区会误删长保留租户的数据。
--      故本迁移**只建分区、不建 DROP 任务**。
--   3. 用 ATTACH PARTITION 把既有表直接挂成第一个分区，**不做全量回填**：
--      先加 CHECK 约束（NOT VALID → VALIDATE，均不阻塞读写），
--      使 ATTACH 时 PG 跳过全表扫描，只改元数据。换名在一个事务内完成，
--      锁窗口为毫秒级。回滚同样是元数据操作（反向换名即可），无数据搬运。
-- =============================================================================

\set ON_ERROR_STOP on
\pset pager off

-- -----------------------------------------------------------------------------
-- 第 0 步：守卫 —— 仅普通 PG 路径执行（本段不含 :var，可以放 DO 块里）
-- -----------------------------------------------------------------------------
DO $wave7d_guard$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
        RAISE EXCEPTION '检测到 timescaledb 扩展：该库走 hypertable 路径（57.sql），声明式分区不适用。'
            USING HINT = '设置 AETHERLINK_TIMESCALE_MODE=off，或改用 hypertable 的 retention policy。';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'telemetry_datas' AND relkind IN ('r', 'p')) THEN
        RAISE EXCEPTION '未找到 telemetry_datas（relkind=r/p 均不匹配），请确认库已初始化。';
    END IF;
END
$wave7d_guard$;

SELECT (relkind = 'p') AS already_partitioned
FROM pg_class WHERE relname = 'telemetry_datas'
\gset

\if :already_partitioned
  \echo 'telemetry_datas 已是分区表，本迁移无需执行。'
  \quit
\endif

-- -----------------------------------------------------------------------------
-- 第 1 步：计算分区边界（从数据推导，不硬编码日期）
--   lo = 最早数据所在月的月初；hi = 最晚数据所在月的下月初
--   ts 为 UnixMilli bigint，边界同样用毫秒整数表达
-- -----------------------------------------------------------------------------
SELECT
  (EXTRACT(EPOCH FROM date_trunc('month', to_timestamp(MIN(ts) / 1000.0))) * 1000)::bigint AS part_lo,
  (EXTRACT(EPOCH FROM date_trunc('month', to_timestamp(MAX(ts) / 1000.0)) + interval '1 month') * 1000)::bigint AS part_hi
FROM telemetry_datas
\gset

\echo ''
\echo '分区边界（UnixMilli）：lo =' :part_lo '  hi =' :part_hi

-- -----------------------------------------------------------------------------
-- 第 2 步：加 CHECK 约束标记既有数据的范围
--   NOT VALID 只写元数据、不扫表、不阻塞写入；VALIDATE 取 SHARE UPDATE EXCLUSIVE 锁，
--   不阻塞读写但会扫一遍表。这两步的目的是让第 4 步的 ATTACH 能跳过全表扫描。
-- -----------------------------------------------------------------------------
SELECT NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'telemetry_datas_ts_partition_ck'
      AND conrelid = 'public.telemetry_datas'::regclass
) AS need_ck
\gset

\if :need_ck
  ALTER TABLE public.telemetry_datas
    ADD CONSTRAINT telemetry_datas_ts_partition_ck
    CHECK (ts >= :part_lo AND ts < :part_hi) NOT VALID;
  \echo '  已添加 CHECK 约束（NOT VALID）'
\else
  \echo '  CHECK 约束已存在，跳过'
\endif

ALTER TABLE public.telemetry_datas VALIDATE CONSTRAINT telemetry_datas_ts_partition_ck;
\echo '  CHECK 约束已 VALIDATE'

-- -----------------------------------------------------------------------------
-- 第 3 步：建分区父表
--   LIKE ... INCLUDING ALL 复制列/默认值/NOT NULL/注释/索引。
--
--   ★ 必须 EXCLUDING CONSTRAINTS ★
--   INCLUDING ALL 会把第 2 步加的 CHECK（ts >= lo AND ts < hi）也复制到父表，
--   而父表的约束会**传播到每一个分区** —— 结果是迁移后任何落在 legacy 区间之外的
--   新数据都会被这个 CHECK 挡下（实测报 "violates check constraint
--   telemetry_datas_ts_partition_ck"），分区形同虚设。
--   该 CHECK 只对"被 ATTACH 的那张既有表"有意义（让 ATTACH 跳过全表扫描），
--   对父表毫无用处，必须排除。
--   NOT NULL 不受 EXCLUDING CONSTRAINTS 影响（它总是被复制）。
--
--   另注：UNIQUE 约束不会被 INCLUDING CONSTRAINTS 复制（只复制 CHECK），
--   分区表的唯一索引必须显式建，且必须包含分区键 —— 本表 UNIQUE(device_id,key,ts) 含 ts，满足。
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.telemetry_datas_p (
    LIKE public.telemetry_datas INCLUDING ALL EXCLUDING CONSTRAINTS
) PARTITION BY RANGE (ts);

-- 兜底：若父表上仍残留迁移加的 CHECK（例如由旧版本脚本产生），显式删掉
ALTER TABLE public.telemetry_datas_p
  DROP CONSTRAINT IF EXISTS telemetry_datas_ts_partition_ck;

SELECT NOT EXISTS (
    SELECT 1 FROM pg_class WHERE relname = 'telemetry_datas_p_device_id_key_ts_key'
) AS need_uq
\gset

\if :need_uq
  CREATE UNIQUE INDEX telemetry_datas_p_device_id_key_ts_key
      ON public.telemetry_datas_p (device_id, key, ts);
  \echo '  已在分区父表上建立唯一索引'
\else
  \echo '  分区父表唯一索引已存在，跳过'
\endif

-- -----------------------------------------------------------------------------
-- 第 4 步：把既有表 ATTACH 成第一个分区
--   因第 2 步的 CHECK 约束已 VALIDATE，PG 会跳过全表扫描（否则会全表扫一遍做校验）。
-- -----------------------------------------------------------------------------
SELECT NOT EXISTS (
    SELECT 1 FROM pg_inherits WHERE inhrelid = 'public.telemetry_datas'::regclass
) AS need_attach
\gset

\if :need_attach
  ALTER TABLE public.telemetry_datas_p
    ATTACH PARTITION public.telemetry_datas
    FOR VALUES FROM (:part_lo) TO (:part_hi);
  \echo '  已将既有表 ATTACH 为分区'
\else
  \echo '  既有表已是分区，跳过 ATTACH'
\endif

-- -----------------------------------------------------------------------------
-- 第 5 步：换名（同一事务内完成，锁窗口极短）
--   telemetry_datas   (原表) -> telemetry_datas_legacy
--   telemetry_datas_p (父表) -> telemetry_datas
--   应用侧 SQL 无需改动：GORM 按表名访问，名字仍指向"能看到全部数据"的对象。
-- -----------------------------------------------------------------------------
SELECT EXISTS (
    SELECT 1 FROM pg_class WHERE relname = 'telemetry_datas_p' AND relkind = 'p'
) AS need_swap
\gset

\if :need_swap
  BEGIN;
    SET LOCAL lock_timeout = '5s';
    ALTER TABLE public.telemetry_datas RENAME TO telemetry_datas_legacy;
    ALTER TABLE public.telemetry_datas_p RENAME TO telemetry_datas;
  COMMIT;
  \echo '  已换名：telemetry_datas 现为分区父表，原表成为 telemetry_datas_legacy 分区'
\else
  \echo '  已完成换名，跳过'
\endif

-- -----------------------------------------------------------------------------
-- 第 6 步：预建未来分区
--   必须提前建，否则新数据插入会因 "no partition of relation ... found for row" 报错。
--   默认预建 3 个月；生产应并入 cron 定期滚动创建。
--   本段用 set_config 把 part_hi 传进 DO 块（DO 块内不替换 :var，见文件头坑 a）。
-- -----------------------------------------------------------------------------
SELECT set_config('wave7d.part_hi', :'part_hi', false);

DO $wave7d_future$
DECLARE
    base  timestamptz := to_timestamp(current_setting('wave7d.part_hi')::bigint / 1000.0);
    i     int;
    lo_ms bigint;
    hi_ms bigint;
    pname text;
BEGIN
    FOR i IN 0..2 LOOP
        lo_ms := (EXTRACT(EPOCH FROM (base + (i || ' month')::interval)) * 1000)::bigint;
        hi_ms := (EXTRACT(EPOCH FROM (base + ((i + 1) || ' month')::interval)) * 1000)::bigint;
        pname := 'telemetry_datas_' || to_char(to_timestamp(lo_ms / 1000.0), 'YYYY_MM');

        IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = pname AND relkind = 'r') THEN
            EXECUTE format(
                'CREATE TABLE public.%I PARTITION OF public.telemetry_datas
                   FOR VALUES FROM (%s) TO (%s)',
                pname, lo_ms, hi_ms
            );
            RAISE NOTICE '已创建未来分区 %', pname;
        END IF;
    END LOOP;
END
$wave7d_future$;

-- -----------------------------------------------------------------------------
-- 第 7 步：自检
-- -----------------------------------------------------------------------------
\echo ''
\echo '=== 迁移结果自检 ==='
SELECT
  c.relname,
  c.relkind,
  c.relispartition,
  pg_size_pretty(pg_total_relation_size(c.oid)) AS size,
  pg_get_expr(c.relpartbound, c.oid)            AS partition_bound
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relname LIKE 'telemetry\_datas%'
  AND c.relkind IN ('r', 'p')
ORDER BY c.relispartition, c.relname;

\echo ''
\echo '分区父表上的索引（分区索引会传播到每个分区）：'
SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'telemetry_datas' ORDER BY indexname;

\echo ''
\echo '★ 父表上的 CHECK 约束必须为空 ★（非空说明第 3 步的 EXCLUDING CONSTRAINTS 失效，'
\echo '  新数据会被 legacy 区间的 CHECK 挡下，分区形同虚设）：'
SELECT conname, contype, pg_get_constraintdef(oid) AS def
FROM pg_constraint
WHERE conrelid = 'public.telemetry_datas'::regclass AND contype = 'c';

\echo ''
\echo '注意：换名后 telemetry_datas 是分区父表，本身不存数据；'
\echo '      telemetry_datas_legacy 分区持有迁移前的全部行，行数必须与迁移前一致。'
