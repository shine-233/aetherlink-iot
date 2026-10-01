-- 142.sql — 告警历史设备列表规范化 + 遥测/设备层级索引治理（数据库层优化，无语义变更）
--
-- 背景：
--   1. alarm_history.alarm_device_list 是 jsonb 数组（1.sql 口径：jsonb NOT NULL），
--      DAL 侧三个高频路径都在它上面做集合运算：
--        - internal/dal/alarm.go 的租户/owner 可见性 EXISTS（jsonb_array_elements_text
--          展开后 JOIN devices，逐行展开、无法走索引）；
--        - internal/dal/alarm.go 的告警历史月度趋势同一形态 EXISTS；
--        - internal/dal/alarm.go 按设备过滤 jsonb_exists(...)；
--        - internal/dal/alarm_rules_cf.go 的 alarm_device_list::text LIKE '%id%'
--          （既无法走索引，又存在子串误命中：设备 id 是另一 id 的前缀时错配）。
--      本迁移把这些集合运算落到关联表 alarm_history_devices 上，用普通 btree join 替代
--      jsonb 展开。
--   2. 兼容性（关键）：alarm_device_list 列保留且继续由应用写入，本迁移不改列名/类型/
--      语义；关联表由触发器与列同步，读写两侧都不需要一次性切换。既有行的 JSON 语义
--      （顺序、重复、空数组）不受影响。
--   3. 为什么用触发器而不是改 Go 写入路径：alarm_history 的写入分布在多条告警路径，
--      漏写一处就会出现"jsonb 有、关联表没有"的静默不一致，进而让 join 版本漏查历史告警；
--      触发器保证两者在同一事务内一致（AFTER INSERT/UPDATE，失败即整事务回滚）。
--   4. 遥测索引卫生（deep-refactor-backlog-2026-09-28：删 `_copy1` Navicat 残留）：
--      telemetry_datas_ts_idx_copy1 建在 telemetry_current_datas 上（1.sql），是 Navicat
--      复制索引时留下的 "_copy1" 命名残留，与 telemetry_datas 上的正式索引
--      telemetry_datas_ts_idx 毫无关系。telemetry_current_datas 的全部读路径
--      （dal/telemetry_current_datas.go、devices_list_read_model.go、device_query_reads.go、
--      calcfield/relation_resolver.go）都先按 device_id 等值过滤、由 (device_id, key)
--      唯一约束的索引驱动，没有任何查询做全表 ts 扫描/排序；该单列 ts 索引只给每行
--      遥测 upsert 增加写放大，故以 DROP INDEX IF EXISTS 删除（幂等：不存在即跳过）。
--      telemetry_datas 本体的索引不在本迁移范围内（TimescaleDB 部署下它是 hypertable，
--      维度索引由扩展按 chunk 维护，不在迁移事务里动它）。
--   5. devices(parent_id, sub_device_addr)：网关子设备按 (父设备, 子设备地址) 精确定位
--      （既有 idx_devices_parent_id 只有 parent_id 单列，子设备地址仍需回表过滤）。
--   6. 全部 IF NOT EXISTS / DROP IF EXISTS / DO NOTHING，可重复执行；不改任何列与约束，
--      不动 alarm_device_list 的 jsonb 语义（关联表是它的投影，列保留继续由应用写入）。

-- ---- 1. 告警历史设备关联表 ----
CREATE TABLE IF NOT EXISTS public.alarm_history_devices (
    alarm_history_id varchar(36) NOT NULL,
    device_id        varchar(36) NOT NULL,
    tenant_id        varchar(36) NOT NULL,
    CONSTRAINT alarm_history_devices_pkey PRIMARY KEY (alarm_history_id, device_id),
    CONSTRAINT alarm_history_devices_history_fk FOREIGN KEY (alarm_history_id)
        REFERENCES public.alarm_history (id) ON DELETE CASCADE
);

COMMENT ON TABLE public.alarm_history_devices IS
    '告警历史命中设备的规范化关联表（alarm_history.alarm_device_list 的关系型投影）';
COMMENT ON COLUMN public.alarm_history_devices.alarm_history_id IS '告警历史 id，ON DELETE CASCADE 跟随告警删除';
COMMENT ON COLUMN public.alarm_history_devices.device_id IS '命中设备 id（由 JSON 数组元素展开而来）';
COMMENT ON COLUMN public.alarm_history_devices.tenant_id IS '冗余告警历史租户，便于按 (租户, 设备) 直接过滤';

-- 关联表热路径：
--   (device_id, tenant_id) 服务"按设备查告警历史"与"设备可见性 EXISTS"（租户列与
--   devices.tenant_id 对齐，避免回表 alarm_history 取租户）；
--   (tenant_id, device_id) 服务告警历史删除/租户级统计时的按租户定位。
CREATE INDEX IF NOT EXISTS idx_alarm_history_devices_device_tenant
    ON public.alarm_history_devices (device_id, tenant_id);

CREATE INDEX IF NOT EXISTS idx_alarm_history_devices_tenant_device
    ON public.alarm_history_devices (tenant_id, device_id);

-- ---- 2. 同步触发器：jsonb 列继续是写入事实源，关联表随它同步 ----
-- 元素筛选：只取字符串元素（脏数据里混入数字/对象时不报错、直接忽略），
-- 与 DAL 侧 alarmHistoryDeviceIDsFromValue 的"解析失败按空列表继续"口径一致。
CREATE OR REPLACE FUNCTION public.alarm_history_devices_sync() RETURNS trigger
LANGUAGE plpgsql AS $aetherlink_ahd$
BEGIN
    IF TG_OP = 'UPDATE'
       AND NEW.alarm_device_list IS NOT DISTINCT FROM OLD.alarm_device_list THEN
        RETURN NEW;
    END IF;

    DELETE FROM public.alarm_history_devices WHERE alarm_history_id = NEW.id;

    INSERT INTO public.alarm_history_devices (alarm_history_id, device_id, tenant_id)
    SELECT NEW.id, btrim(elem.value), NEW.tenant_id
    FROM jsonb_array_elements_text(
             COALESCE(
                 CASE WHEN jsonb_typeof(NEW.alarm_device_list) = 'array'
                      THEN (SELECT jsonb_agg(e)
                            FROM jsonb_array_elements(NEW.alarm_device_list) AS e
                            WHERE jsonb_typeof(e) = 'string')
                 END,
                 '[]'::jsonb)
         ) AS elem(value)
    WHERE btrim(elem.value) <> ''
    ON CONFLICT (alarm_history_id, device_id) DO NOTHING;

    RETURN NEW;
END;
$aetherlink_ahd$;

DROP TRIGGER IF EXISTS trg_alarm_history_devices_sync ON public.alarm_history;
CREATE TRIGGER trg_alarm_history_devices_sync
AFTER INSERT OR UPDATE OF alarm_device_list ON public.alarm_history
FOR EACH ROW EXECUTE FUNCTION public.alarm_history_devices_sync();

-- ---- 3. 存量回填 ----
-- 只补关联表、不动 jsonb 列；已存在的行由主键冲突跳过，可重复执行。
INSERT INTO public.alarm_history_devices (alarm_history_id, device_id, tenant_id)
SELECT ah.id, btrim(elem.value), ah.tenant_id
FROM public.alarm_history ah
CROSS JOIN LATERAL jsonb_array_elements_text(
    COALESCE(
        CASE WHEN jsonb_typeof(ah.alarm_device_list) = 'array'
             THEN (SELECT jsonb_agg(e)
                   FROM jsonb_array_elements(ah.alarm_device_list) AS e
                   WHERE jsonb_typeof(e) = 'string')
        END,
        '[]'::jsonb)
) AS elem(value)
WHERE btrim(elem.value) <> ''
ON CONFLICT (alarm_history_id, device_id) DO NOTHING;

-- ---- 4. 遥测索引卫生：删 _copy1 Navicat 残留（幂等）----
-- 见文件头第 4 点。telemetry_current_datas 不是 hypertable（57.sql 只转换 telemetry_datas
-- 与 alarm_info），普通 DROP INDEX 安全；IF NOT EXISTS 语义由 DROP IF EXISTS 提供。
DROP INDEX IF EXISTS public.telemetry_datas_ts_idx_copy1;

-- ---- 5. 网关子设备定位 ----
-- 既有 idx_devices_parent_id 只有 parent_id 单列；子设备按地址精确定位需回表过滤。
CREATE INDEX IF NOT EXISTS idx_devices_parent_sub_addr
    ON public.devices (parent_id, sub_device_addr)
    WHERE parent_id IS NOT NULL;
