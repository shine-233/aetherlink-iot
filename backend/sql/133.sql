-- 133.sql: 号位占位（批次三编号重排，2026-09-26）。
-- 背景：TB-23 移动应用中心（bundle/版本/发布管理后端面）的 mobile_app_bundles
--       迁移（首轮实施曾落在本号位）按批次三统一编号分配改挂 136.sql；
--       initialize/pg_init.go 的迁移循环对缺文件 fail-fast（回滚并报“sql文件不存在”），
--       故本号位转为占位空迁移，保证 1..N 连续可执行。
-- 内容：无操作（NO-OP），仅占位。本迁移不得再承载新 schema 变更；
--       对已执行过旧 133.sql（mobile_app_bundles 表 + Casbin/菜单登记）的库无需
--       回滚——136.sql 的 CREATE TABLE IF NOT EXISTS 与 NOT EXISTS 守卫幂等，
--       两条升级路径结果一致。

DO $$ BEGIN NULL; END $$;
