# 全新库迁移 + DAL 数据库用例运行期证据

日期：2026-09-11
环境：Windows / go1.26.2 windows-amd64 / PostgreSQL 17.5（本地临时实例 127.0.0.1:55432）

## 结论

| 项 | 状态 | 依据 |
| --- | --- | --- |
| **全新库迁移通过**（P0.1 门禁项） | **有运行期证据** | 空库执行完整链，落到 `sys_version=88`，113 张表 |
| 迁移上界与 `VERSION_NUMBER` 一致 | **有运行期证据** | 迁移后 `sys_version` = 88 = `global.VERSION_NUMBER` |
| DAL 数据库用例（10 条） | **有运行期证据** | `go test ./internal/dal/` 全部 PASS，45.847s |
| P0.3 进度回写（迁移 87） | **有运行期证据** | `TestCommandJobDetailProgressMigration87Postgres` PASS |
| 凭证双模式 / 明文清理 | **有运行期证据** | voucher dual-mode 与 plaintext purge 用例 PASS |
| P0.6 迁移 83 | **有运行期证据** | 见 `P0.6-postgres-migration83-evidence.md`，本次一并复跑 PASS |

此前这些用例全部因缺少 `AETHERLINK_TEST_PSQL_DSN` 被 SKIP；本轮解除。

## 环境搭建（可复现）

1. 用用户目录下 7 月的验证数据目录 `C:\Users\Zz\al_pg_verify`（trust 认证，PG 17）起临时实例：

```
pg_ctl -D "C:\Users\Zz\al_pg_verify" -o "-p 55432 -c listen_addresses=127.0.0.1" -l <log> -W start
```

2. 建空库并应用整条迁移链（调用 `initialize.CheckVersion`，一次性临时程序，用完即删）：

```
psql -h 127.0.0.1 -p 55432 -U postgres -c "CREATE DATABASE aetherlink_fresh_test;"
AETHERLINK_TEST_PSQL_DSN=postgres://postgres@127.0.0.1:55432/aetherlink_fresh_test?sslmode=disable \
  go run ./tmp_freshmigrate     # 内部仅 gorm.Open + initialize.CheckVersion
```

结果：

```
sys_version = 88
public schema 表数量 = 113
关键表存在：devices / command_job_details / entity_relations / report_schedule_runs
```

3. 跑 DAL 数据库用例：

```
AETHERLINK_TEST_PSQL_DSN=postgres://postgres@127.0.0.1:55432/aetherlink_fresh_test?sslmode=disable \
  go test ./internal/dal/ -run 'Isolation|Postgres|DualMode|Purge|Migration83' -count=1 -v
--- PASS: TestCheckVoucherExistsDualMode (0.02s)
--- PASS: TestDeviceVoucherDualModeAgainstPostgres (1.31s)
--- PASS: TestPurgeDeviceVoucherPlaintextKeepsRowsWithoutHash (0.00s)
--- PASS: TestPurgeDeviceVoucherPlaintextIdempotent (0.00s)
--- PASS: TestPurgeDeviceVoucherPlaintextRespectsMaxRows (0.00s)
--- PASS: TestPurgeDeviceVoucherPlaintextRejectsNilDB (0.00s)
--- PASS: TestPurgeDeviceVoucherPlaintextPostgres (0.34s)
--- PASS: TestCommandJobDetailProgressMigration87Postgres (1.25s)
--- PASS: TestCommandJobRetryAfterUpdateUsesPostgresTimestampType (0.00s)
--- PASS: TestReportMigration83Postgres (42.73s)
ok      aetherlink-iot/backend/internal/dal  45.847s
```

## 过程中的一个负向发现（重要）

**同样这批用例，在"空库"上是 FAIL 的**：

```
device_voucher_dual_mode_test.go:302: ensure voucher_hash column: ERROR: relation "devices" does not exist
fleet_command_job_progress_postgres_test.go:48: apply migration 87: ERROR: relation "public.command_job_details" does not exist
```

即这些用例**假定目标库已应用迁移**，空库会失败。本轮先建好 schema 再通过。
这说明"有 DSN"不等于"能跑"，基线 schema 是前置条件——记此以免后续会话踩同样的坑。

另：`aetherlink_iot`（同实例内的旧库，55 张表）**不可用**作基线：
它只有 `devices`，缺 `command_job_details` 与 `sys_version`，是旧的不完整 schema。

## 边界

- 单次运行（`-count=1`），未做多轮稳定性验证。
- 仅覆盖 `internal/dal` 中标为需要 PostgreSQL 的用例；`internal/service/scada_postgres_test.go` 未跑。
- 使用的是临时实例与临时库，**未触碰主实例 5432**。

## 清理动作

- 一次性迁移程序 `backend/tmp_freshmigrate/` 已从仓库删除，未提交。
- 验证数据目录与实例均在仓库外；临时实例停止命令：
  `pg_ctl -D "C:\Users\Zz\al_pg_verify" stop -m fast`
