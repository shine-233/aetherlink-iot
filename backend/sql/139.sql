-- 139.sql — 高频日志/历史表热路径索引补齐（数据库层优化，无表结构/语义变更）
--
-- 背景：
--   1. 1.sql 建表时 operation_logs / alarm_history / alarm_info / *_set_logs / event_datas /
--      scene_log / scene_automation_log / ota_upgrade_task_details 只有主键，没有任何二级索引；
--      后续迁移只补了少量局部索引（127.sql 实体审计、126.sql SLA、38.sql OTA 派发）。
--      这些表随设备与时间线性增长，而 DAL 的列表/清理/级联路径都按下列列过滤+排序，
--      当前全部退化为顺序扫描 + 内存排序。
--   2. 每条索引对应的 DAL 查询形态（均为等值前缀 + 排序列，排序方向与查询一致）：
--      - operation_logs (tenant_id, created_at DESC)：GetListByPage 租户列表 ORDER BY created_at DESC、
--        ListOperationLogsForExport 时间窗导出；
--      - operation_logs (created_at)：DeleteOperationLogsByTime 保留期清理 created_at <= ?（全租户）；
--      - alarm_history (tenant_id, create_at DESC)：告警历史分页、月度趋势、活跃告警查询；
--      - alarm_info (tenant_id, alarm_time DESC)：告警信息分页（57.sql 可能已转 hypertable，
--        TimescaleDB 下 CREATE INDEX 自动下推到各 chunk，alarm_info 未启用压缩）；
--      - telemetry_set_logs / attribute_set_logs / command_set_logs (device_id, created_at DESC)：
--        设备下发记录分页；同时是 devices 删除时 ON DELETE CASCADE（2.sql）的外键查找路径；
--      - attribute_set_logs / command_set_logs (message_id)：设备响应回写按 message_id 定位日志；
--      - event_datas (device_id, identify, ts DESC)：事件列表与"按标识取最新一条"，兼作设备删除外键查找；
--      - scene_log (scene_id, executed_at DESC)、scene_automation_log (scene_automation_id, executed_at DESC)：
--        场景/场景联动执行日志分页，兼作 ON DELETE 外键查找；
--      - ota_upgrade_task_details (ota_upgrade_task_id, status)：任务明细/状态统计（38.sql 仅有 status=1 局部索引）；
--      - ota_upgrade_task_details (device_id)：设备删除前清理明细、设备最新 OTA 状态（外键 RESTRICT 检查）；
--      - ota_upgrade_tasks (ota_upgrade_package_id, created_at DESC)：升级包下任务分页；
--      - devices (parent_id) / devices (device_config_id)：网关子设备查询、按档案批量取设备/外键检查，
--        两列多数行为 NULL，用局部索引控制体积。
--   3. 执行方式：pg_init.go CheckVersion 把 N.sql 放进同一个事务 tx.Exec 执行，
--      CREATE INDEX CONCURRENTLY 不能在事务块内运行，故此处使用普通 CREATE INDEX（建索引期间
--      持有 SHARE 锁，只阻塞写不阻塞读）。存量数据很大的部署建议在升级前手工以
--      CREATE INDEX CONCURRENTLY IF NOT EXISTS 预建同名索引，本迁移随后因 IF NOT EXISTS 直接跳过。
--   4. 全部 IF NOT EXISTS，可重复执行；不改任何列、约束、视图与数据。

-- ---- 1. 操作日志（审计） ----
CREATE INDEX IF NOT EXISTS idx_operation_logs_tenant_created_at
    ON public.operation_logs (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_operation_logs_created_at
    ON public.operation_logs (created_at);

-- ---- 2. 告警历史 / 告警信息 ----
CREATE INDEX IF NOT EXISTS idx_alarm_history_tenant_create_at
    ON public.alarm_history (tenant_id, create_at DESC);

CREATE INDEX IF NOT EXISTS idx_alarm_info_tenant_alarm_time
    ON public.alarm_info (tenant_id, alarm_time DESC);

-- ---- 3. 设备下发记录（遥测/属性/命令） ----
CREATE INDEX IF NOT EXISTS idx_telemetry_set_logs_device_created_at
    ON public.telemetry_set_logs (device_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_attribute_set_logs_device_created_at
    ON public.attribute_set_logs (device_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_attribute_set_logs_message_id
    ON public.attribute_set_logs (message_id)
    WHERE message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_command_set_logs_device_created_at
    ON public.command_set_logs (device_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_command_set_logs_message_id
    ON public.command_set_logs (message_id)
    WHERE message_id IS NOT NULL;

-- ---- 4. 设备事件 ----
CREATE INDEX IF NOT EXISTS idx_event_datas_device_identify_ts
    ON public.event_datas (device_id, identify, ts DESC);

-- ---- 5. 场景 / 场景联动执行日志 ----
CREATE INDEX IF NOT EXISTS idx_scene_log_scene_executed_at
    ON public.scene_log (scene_id, executed_at DESC);

CREATE INDEX IF NOT EXISTS idx_scene_automation_log_automation_executed_at
    ON public.scene_automation_log (scene_automation_id, executed_at DESC);

-- ---- 6. OTA 任务 / 明细 ----
CREATE INDEX IF NOT EXISTS idx_ota_upgrade_task_details_task_status
    ON public.ota_upgrade_task_details (ota_upgrade_task_id, status);

CREATE INDEX IF NOT EXISTS idx_ota_upgrade_task_details_device_id
    ON public.ota_upgrade_task_details (device_id);

CREATE INDEX IF NOT EXISTS idx_ota_upgrade_tasks_package_created_at
    ON public.ota_upgrade_tasks (ota_upgrade_package_id, created_at DESC);

-- ---- 7. 设备外键/层级列 ----
CREATE INDEX IF NOT EXISTS idx_devices_parent_id
    ON public.devices (parent_id)
    WHERE parent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_devices_device_config_id
    ON public.devices (device_config_id)
    WHERE device_config_id IS NOT NULL;
