-- 143.sql — telemetry_current_datas 补齐"按设备取最新一行"的索引覆盖（数据库层优化，无语义变更）
--
-- 背景：
--   1. dal/telemetry_datas.go 的 GetCurrentTelemetrDetailData 与
--      dal/telemetry_current_datas.go 的 getCurrentTelemetryReadinessFromDB
--      是同一访问模式：对 telemetry_current_datas 执行
--        WHERE device_id = ? ORDER BY ts DESC LIMIT 1
--      取"该设备最近一次上报命中的那一行"（诊断/只读路径，非"全部 key 当前值"，
--      该契约已在 dal/telemetry_datas.go 的函数注释与
--      automation_tests/tests/12_telemetry_extra.test.js 中显式澄清）。
--   2. 该表唯一约束是 (device_id, key)（1.sql telemetry_current_datas_unique），
--      只能服务"按设备+key 等值查询"，不含 ts，无法服务上面的排序取一；
--      142.sql 已把本表上原有的单列 (ts DESC) 索引 telemetry_datas_ts_idx_copy1
--      作为 Navicat "_copy1" 命名残留删除——该判断本身没错（单列 ts 索引既不能让
--      device_id 等值过滤走索引，又对高频 upsert 有写放大），但遗漏了"按设备取
--      最新一行"这两条读路径确实需要一个以 device_id 开头、含 ts 的复合索引，
--      而不是任何单列 ts 索引。本迁移补上这个复合索引，不恢复旧的单列索引。
--   3. 兼容性：只加索引，不改表结构/约束/列语义；DB 查询计划变化但返回行与既有
--      契约（含上面澄清的"最新一行"语义）完全一致，调用方无需改动。
--   4. 幂等：IF NOT EXISTS，可重复执行。

CREATE INDEX IF NOT EXISTS idx_telemetry_current_datas_device_ts
    ON public.telemetry_current_datas (device_id, ts DESC);

COMMENT ON INDEX public.idx_telemetry_current_datas_device_ts IS
    '服务 GetCurrentTelemetrDetailData / getCurrentTelemetryReadinessFromDB 的 device_id 等值 + ts DESC 取一';
