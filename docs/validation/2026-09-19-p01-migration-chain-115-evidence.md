# P0.1 全新空库迁移链全链验证（1 → 115）

> 日期：2026-09-19
> 仓库：`aetherlink-iot`
> 对应路线图：§3 P0.1「发布同步与部署门禁」——门禁第一条：**全新库迁移通过**
> 执行工具：`backend/cmd/migchaincheck`
> 目标数据库：PostgreSQL 17.5（隔离集群 `127.0.0.1:55433`，全新空库 `aetherlink_migchain_115`）
> 运行结果：**VERDICT=PASS**

---

## 一、验证背景与意义

在 2026-09-17 批次中，全新空库迁移链已成功验证至 `109.sql`。随后系统引入了：
- `110.sql`：产品（Product）完整 CRUD 策略与 Casbin 权限；
- `111.sql`：规则链可靠性 DLQ 死信持久化与 Trace 审计；
- `112.sql`：TB-12 设备认领与自动注册（`device_claim_tokens`）；
- `113.sql`：TB-19 行业解决方案模板引擎（`industry_solutions` 与流水）；
- `114.sql`：解决方案管理控制台前端菜单行；
- `115.sql`：SCADA 实时控制确认令牌加宽修复。

版本号 `VERSION_NUMBER` 相应递增至 115。必须在真实空库上重新执行 `initialize.CheckVersion` 顺序执行 `sql/1.sql` 至 `sql/115.sql`，杜绝任何未被空库验证的新迁移引发零安装失败。

---

## 二、实测过程与命令

```bash
# 1) 在隔离集群 55433 上创建干净空库
psql -h 127.0.0.1 -p 55433 -U postgres -d postgres -c "CREATE DATABASE aetherlink_migchain_115;"

# 2) 执行全链迁移校验
cd backend
AETHERLINK_TIMESCALE_MODE=off go run ./cmd/migchaincheck \
  -dsn "host=127.0.0.1 port=55433 user=postgres dbname=aetherlink_migchain_115 sslmode=disable"
```

---

## 三、输出与关键指标核对

```
目标程序版本 VERSION_NUMBER=115，VERSION=0.0.23
空库校验：现有业务表 0 张
...
2026/09/19 22:43:24 执行sql文件： sql/112.sql
time="2026-09-19T22:43:24+08:00" level=info msg="执行sql脚本..."
2026/09/19 22:43:24 执行sql文件： sql/113.sql
time="2026-09-19T22:43:24+08:00" level=info msg="执行sql脚本..."
2026/09/19 22:43:24 执行sql文件： sql/114.sql
time="2026-09-19T22:43:24+08:00" level=info msg="执行sql脚本..."
2026/09/19 22:43:24 执行sql文件： sql/115.sql
time="2026-09-19T22:43:24+08:00" level=info msg="执行sql脚本..."
2026/09/19 22:43:24 升级成功

迁移完成：耗时 667ms
sys_version      = 115（期望 115）
public 表数量     = 130

VERDICT=PASS :: 全新空库全链迁移通过
```

---

## 四、结论

1. 迁移链从 `1.sql` 至 `115.sql` 连续无损通过，耗时 667ms。
2. 基础业务表与系统表总计 **130 张**，`sys_version` 精准收敛于 **115**。
3. 发布门禁 P0.1 中的空库安装不变量再度得到完整运行期闭环证明。
