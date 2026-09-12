# 验证证据索引

用途：本目录存放**运行期证据**。判断某个功能做到什么程度时，**先看这里，再看 ROADMAP.md**。

> ⚠️ **ROADMAP.md 没有「实现状态」小节 ≠ 功能未实现。**
> 2026-09-11 曾据此把 P1.2（规则链可靠性）和 P0.5（CSV 预注册）误判为「没开始」，
> 实际上两者代码都已完成大半（P1.2 还有 60 条既有用例全过）。**判断前必须查代码，不能只看文档。**

> 📌 **2026-09-12 已对 ROADMAP.md 做过一次全量排查**：逐个核实所有「未完成 / 未做 / 不算完成」表述，
> 发现 **8 处已过时**（P0.3 灰度与报告导出、P0.4 定时器持久化、P0.5 导出清理接线、P1.1 CRUD 与租户守卫、
> P1.2 版本化、P1.3 前后端接线，等），均已就地追加「2026-09-12 更新」说明并保留原表述
> （更正用追加、不删改，以便看出判断是怎么被推翻的）。
> 但 **ROADMAP 仍可能再次滞后**——并行会话推进很快。发现表述与实际不符时请顺手更正。

## 状态口径（ROADMAP §4）

- **done** —— 已真实运行并留有证据
- **partial** —— 代码在，但关键闭环或运行期证据缺失
- **pending** —— 等环境或尚未实现

禁止用单测绿、路由可访问、覆盖率数字或页面截图单独宣称完成。

## 证据清单

| 文件 | 对应项 | 关键结论 |
| --- | --- | --- |
| `P0.1-preflight-evidence.md` | P0.1 | 三个门禁脚本实跑；并记录一次真实拦截（迁移上界落后） |
| `P0.2-shadow-ack-evidence.md` | P0.2 | 影子 ACK 闭环的数据库层证据；仍缺 MQTT 端到端 |
| `P0.5-export-cleanup-evidence.md` | P0.5 | 导出/清理接线核查 + Casbin 漏登记修复 |
| `P0.5-cleanup-execution-evidence.md` | P0.5 | 清理**真实删除路径**证据（与注入式单测互补） |
| `P0.6-postgres-migration83-evidence.md` | P0.6 | 迁移 83 实跑，17 组子用例；SKIP 解除 |
| `P0.6-P0.7-evidence.md` | P0.6 / P0.7 | 单元层证据（另一会话），并含一条已更正的环境判断 |
| `P0.7-secret-encryption-evidence.md` | P0.7 | 凭证静态加密数据库层证据：库里不含明文等 5 项 |
| `P1.2-rulechain-version-evidence.md` | P1.2 | 草稿/发布/回滚：语义 + 迁移 + 端点 + 持久化证据 |
| `fresh-migration-and-dal-postgres-evidence.md` | 通用 | 全新库迁移通过；16 条依赖数据库的用例取得证据 |

## 本目录的约定

1. 每份证据必须写明：命令、版本/环境、结果、日志关键行、清理动作、**仍未验证项**。
2. 需要真实数据库的用例写成常驻 `*_postgres_test.go`，**缺 DSN 或缺表一律 Skip，不得把 Skip 当通过**。
3. 一次性验证脚本用完即删，不得提交到仓库。
4. 结论变化时在文末**追加带日期的更正章节**，不删改原结论——保留判断如何被推翻的痕迹。

## 复现环境（本机可用，别再误判为"没有数据库"）

- 装有 **PostgreSQL 17**：`C:\Program Files\PostgreSQL\17\bin`
- 用户目录下的 `al_pg_verify` 是验证用数据目录，PG 17，`pg_hba.conf` 为 **trust**
- **Redis 默认就在跑**（6379）

```
pg_ctl -D "C:\Users\Zz\al_pg_verify" -o "-p 55432 -c listen_addresses=127.0.0.1" -l <log> -W start
pg_ctl -D "C:\Users\Zz\al_pg_verify" stop -m fast
```

> 注意 `pg_ctl start` 默认会等服务器就绪，父进程被 SIGTERM 时实例会被带走，要用 `-W`。
>
> 需要落库验证时，先 `CREATE DATABASE` 再用一次性程序调 `initialize.CheckVersion(db)` 把迁移链跑完，
> 然后 `go test` 时设置 `AETHERLINK_TEST_PSQL_DSN=postgres://postgres@127.0.0.1:55432/<db>?sslmode=disable`。
> **直接指空库会 FAIL**（`relation "devices" does not exist` 等）——有 DSN 不等于能跑。

## 写数据库测试的三个必备动作

1. `global.DB = db` **之外还要 `query.SetDefault(db)`**（gorm gen 查询对象否则为 nil，一调用就 panic，
   而堆栈会误导性地报在被测函数里）
2. 插设备前先插产品：`devices.product_id` 有外键 `fk_product_id`
3. 主键多为 `uuid` 类型，用 `uuid.New()`，自定义字符串会报 `22P02`
