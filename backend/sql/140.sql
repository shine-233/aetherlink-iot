-- 140.sql — 场景联动/场景/分组关联表的外键与热路径查找索引补齐（数据库层优化，无表结构/语义变更）
--
-- 背景：
--   1. 1.sql 建表时 device_trigger_condition / action_info / scene_action_info 只有主键，
--      r_group_device 只有 (group_id, device_id) 唯一约束；后续迁移从未给这些列补二级索引。
--      它们都是 ON DELETE CASCADE 的子表：删除场景联动/场景/设备时，PG 需要按外键列查子表，
--      无索引即每删除一行父记录就对子表做一次顺序扫描。
--   2. 每条索引对应的 DAL 查询形态：
--      - device_trigger_condition (trigger_source, trigger_condition_type)：设备上报触发联动判定
--        GetDeviceTriggerConditionByDeviceId（type + source + enabled）、按设备/档案列出联动
--        （scene_automation_list_queries.go，type + source）、按设备取全部条件（source）。
--        设备上行热路径，trigger_source 为等值首列；时间范围类条件该列为 NULL，用局部索引排除。
--      - device_trigger_condition (scene_automation_id)：联动详情加载/编辑时整组删除，兼作外键级联查找。
--      - action_info (scene_automation_id)：联动动作加载/删除、告警联动 IN 查询，兼作外键级联查找。
--      - action_info (action_target, action_type)：按设备/档案列出"以其为动作目标"的联动；
--        单类设备动作该列为 NULL，用局部索引排除。
--      - scene_action_info (scene_id)：场景动作加载与编辑时整组删除，兼作外键级联查找。
--      - r_group_device (device_id)：按设备查所属分组/删除设备分组关系；现有唯一约束以 group_id
--        开头，无法服务 device_id 单列过滤，兼作 devices 删除时的外键级联查找。
--   3. 执行方式同 139.sql：pg_init.go CheckVersion 在单个事务内执行 N.sql，
--      不能使用 CREATE INDEX CONCURRENTLY；大表部署可升级前手工以 CONCURRENTLY 预建同名索引。
--   4. 全部 IF NOT EXISTS，可重复执行；不改任何列、约束、视图与数据。

-- ---- 1. 场景联动触发条件 ----
CREATE INDEX IF NOT EXISTS idx_device_trigger_condition_source_type
    ON public.device_trigger_condition (trigger_source, trigger_condition_type)
    WHERE trigger_source IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_device_trigger_condition_scene_automation_id
    ON public.device_trigger_condition (scene_automation_id);

-- ---- 2. 场景联动动作 ----
CREATE INDEX IF NOT EXISTS idx_action_info_scene_automation_id
    ON public.action_info (scene_automation_id);

CREATE INDEX IF NOT EXISTS idx_action_info_target_type
    ON public.action_info (action_target, action_type)
    WHERE action_target IS NOT NULL;

-- ---- 3. 场景动作 ----
CREATE INDEX IF NOT EXISTS idx_scene_action_info_scene_id
    ON public.scene_action_info (scene_id);

-- ---- 4. 设备分组关系 ----
CREATE INDEX IF NOT EXISTS idx_r_group_device_device_id
    ON public.r_group_device (device_id);
