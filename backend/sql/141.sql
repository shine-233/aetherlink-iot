-- 141.sql — 保留期注册表（TB-22）：把"只增不删"表的保留期从硬编码 cron 泛化成数据驱动的注册表
--
-- 背景：
--   1. 约 20 张日志/历史/回执/死信表只有 INSERT 没有 DELETE 出口，随设备数与时间线性膨胀：
--      event_datas、*_set_logs、device_status_history、alarm_history、scene_*_log、
--      上行回执与各类死信表。此前只有两类数据有保留出口——data_policy（data_type='1'
--      设备数据、'2' 操作日志）与 TB-15 的 TimescaleDB retention（仅 telemetry_datas），
--      其余表全靠人工 SQL 清理。
--   2. 为什么不直接往 data_policy 插行：data_policy 的 data_type 是封闭枚举（'1'/'2'），
--      且被两处硬编码消费——initialize/timescale_retention.go 只认 data_type='1' 装配
--      drop_after，CleanSystemDataByCron 只处理 deviceDataPolicyType / operationLogPolicyType。
--      往里塞第 20 张表会让"设备数据"语义与"表级保留"语义在同一张表里互相污染。
--      故另立 data_retention_registry：以 (表名, 时间列) 为主体的通用注册表，
--      data_policy 与 TB-15 语义完全不变（本迁移不触碰 data_policy 任何行/列）。
--   3. 客户数据默认关闭（enabled='2'）：清理不可逆，是否丢历史数据只能由部署方决定；
--      迁移只登记"可清理"与推荐天数，不替客户做删除决定。
--   4. 唯一默认开启的是幂等回执 uplink_storage_receipts（category='idempotency_receipt'）：
--      它是纯去重凭证，主事务确认后即被 releaseWriteAheadReceipts 删除，只有写入失败
--      留下的孤儿才需要回收；45 天窗口显著长于任何重放租约（死信重放租约是分钟~小时级，
--      attributeEventDeadLetter* 的结算超时是秒级），保证"回执先于业务数据消失"永不发生——
--      反过来的顺序会让重放变成重复写入。
--   5. 删除执行：由现有 CleanSystemDataByCron 在同一轮里遍历本注册表，按 batch_size 分批
--      DELETE（避免单条大 DELETE 的长事务与 WAL 尖峰）。表名/时间列来自注册表行，
--      Go 侧以标识符白名单正则 + to_regclass 二次校验后才拼进 SQL，不接受任意 SQL 片段。
--   6. time_kind 区分两类时间列：timestamptz（本迁移全部 19 行种子都是这一类）与
--      unix_ms（UnixMilli bigint，如 telemetry_datas.ts）。两类都用"早于
--      now()-retention_days"的同一语义，只是边界换算不同。unix_ms 目前无种子行——
--      telemetry_datas 由 data_policy data_type='1' 与 TB-15 retention 两条既有出口
--      覆盖，不应再进注册表（两套出口同时删同一批行会放大删除范围）。Go 侧
--      (dal/data_policy_retention.go) 已实现 unix_ms 分支，供后续登记 bigint 时间表时直接用。
--      注意：不要把 event_datas.ts 误当作 unix_ms——它是 timestamptz
--      （model/event_datas.gen.go 的 T 是 time.Time），与 telemetry_datas.ts 的 bigint 同名不同型。
--   7. resolved_only：死信类表只回收已解决（status='resolved'）的行，未解决的 pending/
--      retrying/dead 是可重放资产，绝不能按时间删。
--   8. 种子写入带守卫：表或时间列不存在时静默跳过（不同部署的存量库可能缺表），
--      已存在同名表行则跳过（ON CONFLICT DO NOTHING），可重复执行。
--   9. 全部 IF NOT EXISTS / DO NOTHING，可重复执行；不改任何既有表结构、约束与数据。

-- ---- 1. 注册表 ----
CREATE TABLE IF NOT EXISTS public.data_retention_registry (
    id                     varchar(36)  NOT NULL,
    table_name             varchar(63)  NOT NULL,
    time_column            varchar(63)  NOT NULL,
    time_kind              varchar(16)  NOT NULL DEFAULT 'timestamptz',
    retention_days         int4         NOT NULL,
    category               varchar(32)  NOT NULL DEFAULT 'customer_data',
    enabled                varchar(10)  NOT NULL DEFAULT '2',
    batch_size             int4         NOT NULL DEFAULT 10000,
    resolved_only          bool         NOT NULL DEFAULT false,
    last_cleanup_time      timestamptz  NULL,
    last_cleanup_data_time timestamptz  NULL,
    remark                 varchar(255) NULL,
    created_at             timestamptz  NOT NULL DEFAULT now(),
    updated_at             timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT data_retention_registry_pkey PRIMARY KEY (id),
    CONSTRAINT data_retention_registry_table_uq UNIQUE (table_name),
    CONSTRAINT data_retention_registry_enabled_check CHECK (enabled IN ('1', '2')),
    CONSTRAINT data_retention_registry_time_kind_check CHECK (time_kind IN ('timestamptz', 'unix_ms')),
    CONSTRAINT data_retention_registry_category_check CHECK (
        category IN ('customer_data', 'idempotency_receipt', 'dead_letter', 'audit_log')
    ),
    CONSTRAINT data_retention_registry_days_check CHECK (retention_days > 0 AND retention_days <= 3650),
    CONSTRAINT data_retention_registry_batch_check CHECK (batch_size > 0 AND batch_size <= 100000)
);

COMMENT ON TABLE public.data_retention_registry IS
    '表级保留期注册表（TB-22）：登记只增不删表的 (表名, 时间列, 保留天数)，由 CleanSystemDataByCron 分批删除';
COMMENT ON COLUMN public.data_retention_registry.table_name IS '被清理的表名（Go 侧标识符白名单 + to_regclass 二次校验）';
COMMENT ON COLUMN public.data_retention_registry.time_column IS '判定过期的时间列名';
COMMENT ON COLUMN public.data_retention_registry.time_kind IS 'timestamptz=时间戳列；unix_ms=UnixMilli bigint 列';
COMMENT ON COLUMN public.data_retention_registry.category IS 'customer_data=客户数据（默认关闭）/idempotency_receipt=幂等回执/dead_letter=死信/audit_log=审计';
COMMENT ON COLUMN public.data_retention_registry.enabled IS '是否启用：1启用 2停用（客户数据默认停用，清理不可逆）';
COMMENT ON COLUMN public.data_retention_registry.batch_size IS '单轮分批删除的批大小，控制长事务与 WAL';
COMMENT ON COLUMN public.data_retention_registry.resolved_only IS '只删 status=''resolved'' 的死信行，未解决重放资产不按时间删';
COMMENT ON COLUMN public.data_retention_registry.last_cleanup_data_time IS '上次实际清理到的时间边界（该时间点之前的数据已删）';

-- 清理驱动扫描：启用行按"上次清理时间"排序，避免每轮全表扫注册表（注册表本身很小，
-- 但带上索引可与未来按表查询的管理接口共用）。
CREATE INDEX IF NOT EXISTS idx_data_retention_registry_enabled
    ON public.data_retention_registry (enabled, table_name);

-- ---- 2. 默认登记行 ----
-- 守卫：只有当 (表, 时间列) 在 information_schema 中同时存在、且列的真实类型与
-- time_kind 相容时才登记：
--   - 缺表的存量部署可干净升级（列不存在即跳过）；
--   - 类型不相容时跳过而不是硬登记——用 timestamptz 边界去比 bigint 列会直接报错，
--     反过来用 UnixMilli 整数比 timestamptz 列更危险（PG 会把整数当毫秒时间戳隐式
--     转换，删掉的是 1970 年前后的全部行）。类型契约在这里一次拦住，Go 侧不再二次判断。
--   - 已登记的同名表行不再改写（ON CONFLICT DO NOTHING，保留运维手工调整的天数）。
-- 'timestamp without time zone'（如 message_push_log.create_time）归入 timestamptz 分支：
-- PG 比较时按会话 TimeZone 把它提升为 timestamptz，与写入侧的 CURRENT_TIMESTAMP 同域，
-- 语义与 timestamptz 列一致（本项目 DSN 固定 TimeZone=Asia/Shanghai）。
INSERT INTO public.data_retention_registry (
    id, table_name, time_column, time_kind, retention_days, category, enabled, batch_size, resolved_only, remark
)
SELECT v.id, v.table_name, v.time_column, v.time_kind, v.retention_days, v.category,
       v.enabled, v.batch_size, v.resolved_only, v.remark
FROM (VALUES
    -- 幂等回执：唯一默认开启的一行；45 天 >> 最大重放租约，保证回执不早于业务数据消失
    ('r-uplink-receipts',   'uplink_storage_receipts',      'created_at', 'timestamptz', 45,  'idempotency_receipt', '1', 10000, false, '上行幂等回执，默认开启；窗口必须显著长于最大重放租约'),
    -- 客户数据：默认关闭，由部署方按需开启
    ('r-event-datas',       'event_datas',                  'ts',         'timestamptz', 180, 'customer_data', '2', 10000, false, '设备事件上报（ts 为 timestamptz，与 telemetry_datas 的 UnixMilli 列不同）'),
    ('r-telemetry-set-logs','telemetry_set_logs',           'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, '遥测下发记录'),
    ('r-attribute-set-logs','attribute_set_logs',           'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, '属性下发记录'),
    ('r-command-set-logs',  'command_set_logs',             'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, '命令下发记录'),
    ('r-device-status-hist','device_status_history',        'change_time','timestamptz', 180, 'customer_data', '2', 10000, false, '设备上下线历史'),
    ('r-alarm-history',     'alarm_history',                'create_at',  'timestamptz', 365, 'customer_data', '2',  5000, false, '告警历史'),
    ('r-alarm-info',        'alarm_info',                   'alarm_time', 'timestamptz', 365, 'customer_data', '2',  5000, false, '告警信息'),
    ('r-scene-log',         'scene_log',                    'executed_at','timestamptz', 180, 'customer_data', '2', 10000, false, '场景执行日志'),
    ('r-scene-auto-log',    'scene_automation_log',         'executed_at','timestamptz', 180, 'customer_data', '2', 10000, false, '场景联动执行日志'),
    ('r-command-job-events','command_job_events',           'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, '命令作业事件'),
    ('r-command-job-det',   'command_job_details',          'created_at', 'timestamptz', 180, 'customer_data', '2', 10000, false, '命令作业明细（终态后不再变更）'),
    ('r-device-user-logs',  'device_user_logs',             'created_at', 'timestamptz', 365, 'customer_data', '2', 10000, false, '设备用户操作日志'),
    ('r-message-push-log',  'message_push_log',             'create_time','timestamptz', 180, 'customer_data', '2', 10000, false, '消息推送日志'),
    ('r-rule-chain-traces', 'rule_chain_node_traces',       'created_at', 'timestamptz',  90, 'customer_data', '2', 10000, false, '规则链节点轨迹（调试用，可短保留）'),
    -- 审计日志：与 data_policy data_type='2'（操作日志）分开登记，避免两套出口互相覆盖
    ('r-operation-logs',    'operation_logs',               'created_at', 'timestamptz', 180, 'audit_log',     '2', 10000, false, '操作审计日志；与 data_policy data_type=''2'' 并存，两者都开启时删除幂等'),
    -- 死信：只回收已解决行，未解决行是可重放资产
    ('r-uplink-dl',         'uplink_storage_dead_letters',  'created_at', 'timestamptz',  30, 'dead_letter',   '2',  5000, true,  '上行死信，仅回收 resolved'),
    ('r-telemetry-dl',      'telemetry_dead_letters',       'created_at', 'timestamptz',  30, 'dead_letter',   '2',  5000, true,  '遥测死信，仅回收 resolved'),
    ('r-rule-chain-dl',     'rule_chain_dead_letters',      'created_at', 'timestamptz',  30, 'dead_letter',   '2',  5000, false, '规则链死信（无可重放载荷，审计最小化）')
    -- ota_upgrade_task_details 故意不登记：它没有 created_at（只有 updated_at /
    -- last_dispatch_started_at，语义是"最后变更"而非"发生时间"，按它删会误删进行中的
    -- 升级任务），且受 devices 的 RESTRICT 外键约束，回收前必须先确认设备已删除。
) AS v(id, table_name, time_column, time_kind, retention_days, category, enabled, batch_size, resolved_only, remark)
WHERE EXISTS (
    SELECT 1
    FROM information_schema.columns c
    WHERE c.table_schema = 'public'
      AND c.table_name = v.table_name
      AND c.column_name = v.time_column
      AND (
          (v.time_kind = 'timestamptz'
           AND c.data_type IN ('timestamp with time zone', 'timestamp without time zone'))
          OR (v.time_kind = 'unix_ms' AND c.data_type IN ('bigint', 'integer'))
      )
)
ON CONFLICT (table_name) DO NOTHING;
