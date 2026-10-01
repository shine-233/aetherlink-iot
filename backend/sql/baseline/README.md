# sql/baseline — 全新安装用的迁移基线

`<N>.sql` 等价于在空库上按 `AETHERLINK_TIMESCALE_MODE=off` 依次执行 `sql/1.sql .. sql/N.sql`
（schema + 全部种子行 + 序列 setval），由工具生成，**禁止手改**。

## 何时生效

Go 运行器 `initialize.CheckVersion` 只在以下条件全部满足时执行基线，然后从 `N+1` 继续原有增量循环：

- `sys_version` 版本号为 0（从未迁移过）。已有版本的库永远不会用基线；
- `AETHERLINK_MIGRATION_BASELINE` / `db.migration.baseline` 不是 `off`（默认 `auto`）；
- public 下除 `sys_version` 外没有任何表；
- 头部 `source-sha256` 与当前 `sql/1..N.sql` 一致（否则视为过期，打印警告并走增量）；
- 服务器主版本 >= 头部 `postgres-major`；
- TimescaleDB 的有效决策是不执行 `57.sql`：`mode=off`，或 `auto` 且扩展未安装。
  TimescaleDB 安装（`on`，或 `auto` 且扩展存在）一律走增量路径。

任一条件不满足时行为与没有基线时完全一致。取 `<= VERSION_NUMBER` 的最大编号，旧基线仍可用，只是多重放几个增量。

`sql/1..N.sql` 保留不动：已有库升级、TimescaleDB 安装和 docker initdb（`deploy/postgres/00-run-migrations.sh`）
都依赖它们。docker initdb 先写入 `sys_version`，所以 compose 部署不会走基线。

## 重新生成

改了任何 `sql/1..N.sql`，或想把基线推进到新的 `VERSION_NUMBER`：

```sh
cd backend
go run ./cmd/migbaseline -dsn-admin "host=127.0.0.1 port=55433 user=postgres dbname=postgres sslmode=disable" -verify
```

工具会建两个临时库：A 走纯增量，B 经同一 Go 运行器走基线，两者 `pg_dump` 后逐行比对（schema 有序、数据按行多重集），
差异为 0 才算通过。用部署所支持的**最低** PostgreSQL 主版本生成，否则更低版本的服务器会回落增量路径。
`-at K` 做升级演练（基线 K + 增量 K+1..N，基线写到临时目录）。

`initialize` 包的 `TestCommittedBaselineMatchesSources` 会在基线过期时失败。

## 已知取舍

- 种子行里 `now()` 的时间戳是生成时刻，不是安装时刻。
- 全新安装耗时收益很小（本地 PG 17.5：增量约 0.9–1.1s，基线约 0.7–0.9s）；主要价值是全新库只执行一份已验证的快照。
