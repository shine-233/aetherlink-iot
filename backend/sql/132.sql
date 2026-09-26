-- 132.sql: 号位占位（批次三编号重排，2026-09-26）。
-- 背景：TB-19 通用 Protobuf 载荷编解码的 data_converters.proto_schema 迁移
--       （首轮实施曾落在本号位）按批次三统一编号分配改挂 135.sql；
--       initialize/pg_init.go 的迁移循环对缺文件 fail-fast（回滚并报“sql文件不存在”），
--       故本号位转为占位空迁移，保证 1..N 连续可执行。
-- 内容：无操作（NO-OP），仅占位。本迁移不得再承载新 schema 变更；
--       对已执行过旧 132.sql（proto_schema 列）的库无需回滚——135.sql 的
--       ADD COLUMN IF NOT EXISTS 幂等，两条升级路径结果一致。

DO $$ BEGIN NULL; END $$;
