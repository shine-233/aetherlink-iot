-- 144.sql：设备在线趋势（dal/device_trend.go GetDeviceTrend）的"区间前基数"索引
--
-- 背景：
--   1. before_online CTE 需要"每台设备在区间起点之前的最后一条状态（MAX(id)）"。
--      旧写法 GROUP BY device_id + MAX(id) 后再按 id 回表，执行计划是整张
--      device_status_history 顺序扫描 + 哈希回连，成本随历史总行数线性增长。
--   2. 新写法改为按设备 LATERAL ... ORDER BY id DESC LIMIT 1，需要以
--      (tenant_id, device_id) 开头、按 id 降序的索引；现有 idx_tenant_device_time
--      是 (tenant_id, device_id, change_time)，按 id 取一时只能把该设备全部旧历史
--      读出来再排序（实测比旧写法更慢）。
--   3. INCLUDE (status, change_time) 让探测走 Index Only Scan，不回堆。
--   4. 实测（PG 17.5，5 万设备 / 180 万状态历史）：GetDeviceTrend 1347ms -> 573ms，
--      before_online 单段 960ms -> 260ms；结果逐行一致。
--   5. 兼容性：只加索引，不改表结构/约束/语义。状态历史为低频写（仅在线状态跃迁），
--      写放大可接受。
--   6. 幂等：IF NOT EXISTS；不用 CONCURRENTLY（迁移在单事务中执行）。

CREATE INDEX IF NOT EXISTS idx_device_status_history_tenant_device_id_desc
    ON public.device_status_history (tenant_id, device_id, id DESC)
    INCLUDE (status, change_time);

COMMENT ON INDEX public.idx_device_status_history_tenant_device_id_desc IS
    '服务 GetDeviceTrend before_online：按设备取区间起点前最后一条状态（id DESC LIMIT 1）';
