# Wave7-D 立项方案：时序表原生分区（普通 PG 路径）

日期：2026-10-01 ｜ 分支 `feature-roadmap-complete-tb-tp-parity` ｜ 状态：**待决策，未动代码**

对应计划：`docs/deep-refactor-wave7-plan-2026-09-30.md` 第 48–53 行（Wave7-D）

---

## 0. 先纠正一处口径：这一项不是"未开工"

计划原文写的是「时序表原生分区立项（需产品/运维决策，非纯工程）」，容易被读成"一行没写"。
实测**Timescale 路径已经做完了**，缺的是**普通 PG 路径**。逐条列证据：

| 机制 | 位置 | 做了什么 |
|---|---|---|
| hypertable 转换 + 压缩 | `backend/sql/57.sql`（44 行） | `telemetry_datas` 直接转 hypertable（其 `UNIQUE(device_id,key,ts)` 含时间列，符合要求）；`alarm_info` 先改复合主键 `(id, alarm_time)` 再转。附 `add_compression_policy`（**只省存储，不删数据**） |
| 显式开关 | `backend/initialize/timescale_mode.go`（81 行） | `AETHERLINK_TIMESCALE_MODE=auto\|on\|off`。`on` 但扩展缺失 → **迁移启动即失败**（fail-fast，杜绝"以为开了实际没开"的静默降级） |
| 原生保留策略 | `backend/initialize/timescale_retention.go`（212 行） | 按 `data_policy` 的 `retention_days` 调 `add_retention_policy`；`ts` 是 UnixMilli bigint，故配 `set_integer_now_func('aetherlink_ts_now_ms')`，`drop_after` 按毫秒表达。装配幂等（查 `timescaledb_information.jobs` 守卫，漂移则 remove+re-add 收敛） |
| 行级保留 TTL | `backend/sql/138.sql`（67 行） | `data_policy` 增加 `tenant_id` / `device_config_id` 两列，作用域层级 **档案级 > 租户级 > 全局**。**行级一律走 `CleanSystemDataByCron` 分批 DELETE**，刻意不触碰 Timescale retention |
| 保留期注册表 | `backend/sql/141.sql`（138 行） | `data_retention_registry`，把 ~20 张"只增不删"表（`event_datas`、各类 `*_set_logs`、死信表…）的保留期泛化成数据驱动。**客户数据默认关闭**（`enabled='2'`），唯一默认开启的是幂等回执表 |

**所以真实的缺口是**：`AETHERLINK_TIMESCALE_MODE=off`（或未装扩展）的**普通 PG 部署，`telemetry_datas` 没有任何原生分区**。全部清理压力压在两处批删上：

- `CleanSystemDataByCron` 分批 DELETE（138.sql 口径）
- 57.sql 注释里写的"未启用 TimescaleDB 时由 CleanSystemDataByCron 兜底"

批删的代价是**死元组膨胀 + autovacuum 追不上 + 无分区裁剪**。这正是分区改造要解决的问题。

---

## 1. 需要决策的 4 项（含我的推荐，可只做批准）

计划原文列的 4 个前置条件，逐条给出**我的推荐答案**——如果认可，直接批即可，不需要额外调研。

### 决策 1：真实数据量压力 —— 用度量脚本取数，不靠猜

我写了只读度量脚本：**`deploy/maintenance/measure_telemetry_volume.sql`**

```bash
psql "$AETHERLINK_DSN" -f deploy/maintenance/measure_telemetry_volume.sql
```

8 段输出直接回答：部署形态（是否 Timescale）、体量与时间跨度、近 30 天日均/峰值增速、
死元组比例与 autovacuum 情况、Timescale 侧现状、138/141 的配置现状、按设备分布。

**推荐判定阈值**（脚本第 3/4 段出数后套用）：

| 指标 | 阈值 | 结论 |
|---|---|---|
| `telemetry_datas` 估算行数 | < 5 亿 且 死元组 < 20% | **不做**。批删够用，收益不抵风险 |
| 估算行数 | ≥ 5 亿 或 死元组持续 > 30% | **做**。分区收益（裁剪 + DROP 释放）显著 |
| 已达 10 亿行的预计时间 | > 18 个月 | 降级为"观察"，先只做查询侧优化 |

### 决策 2：停机或双写窗口 —— 推荐"零停机 + 元数据切换"

**推荐方案：不停机、不双写。** 用 PG 的 `ATTACH PARTITION` 把既有表直接挂成分区，
避免全量回填（详见第 3 节）。代价是切换瞬间一次极短的表级锁（毫秒级，仅元数据）。

若运维要求绝对零锁，退路是双写窗口，但**需要改写入路径代码**（`dal/telemetry_datas.go`
所有 INSERT 目标表），成本远高于元数据切换。**我不推荐**。

### 决策 3：Timescale 与非 Timescale 双路径 —— 推荐"双路径独立，不互相依赖"

- **Timescale 路径**：保持现状不动。57.sql + `timescale_retention.go` 已闭环。
- **普通 PG 路径**：新增声明式分区（本方案）。
- 两者由**同一个开关** `AETHERLINK_TIMESCALE_MODE` 分流：`off` → 走分区；`auto`/`on` → 走 hypertable。
  **绝不允许两条路径同时生效**（hypertable 本身是分区表，再套一层声明式分区会冲突）。

**这条要在迁移脚本里显式守卫**：分区迁移只在 `timescale_installed = false` 时执行，
否则跳过并记日志。与 57.sql 的"检测扩展自动启用"风格保持一致。

### 决策 4：与 138/141.sql 的交互口径 —— 这是本项**唯一的技术难点**，见下节

---

## 2. 核心约束：行级保留期与"整分区 DROP"语义冲突

这是我在核对 138.sql 时发现的**硬约束**，计划原文只写了"交互口径"四个字，没展开。

### 冲突在哪

138.sql 引入了**档案级/租户级保留期**：同一个时间点上，租户 A 可能配 30 天、租户 B 配 365 天、
设备档案 C 配 7 天。而**一个时间分区覆盖所有租户**。

于是：

- 若按全局保留期（如 30 天）`DROP` 掉过期分区 → **会误删配了 365 天的租户 B 的数据**。
- 若按最长保留期（365 天）才 `DROP` → 分区里堆积大量早已过期、只属于短保留租户的行，
  DROP 的收益被摊薄到几乎没有。

**换句话说：只要存在"保留期长于全局默认"的作用域行，分区 DROP 就不能作为主要过期手段。**

### 三种处理方式

| 方案 | 做法 | 评价 |
|---|---|---|
| **A. 只分区、不 DROP** | 分区仅用于**查询裁剪**与**避免全表 vacuum**；过期仍走行级 DELETE | **推荐**。风险最低，收益仍实在（裁剪 + 分区级 vacuum）；代价是过期行不会自动释放 |
| **B. 混合：按最大保留期 DROP** | 计算所有作用域行的**最大** `retention_days`，只有整分区超出该值才 DROP；区间内的过期行仍走行级 DELETE | 折中。需要额外维护"当前最大保留期"，配置变更时要能收敛 |
| **C. 按租户二级分区** | `PARTITION BY RANGE(ts)` 再 `SUBPARTITION BY LIST(tenant_id)` | **不推荐**。租户数动态变化 → 分区数 = 时间片 × 租户数，元数据爆炸；且 `tenant_id` 不在唯一约束 `(device_id,key,ts)` 里，无法直接做子分区键 |

**推荐落地：A 为主，B 作为可选的后续增强**。先把分区建起来拿到裁剪收益，
DROP 语义等 138.sql 的作用域行实际被用起来之后再评估——**不要一次做完**。

### 还需注意的两点交互

1. **141.sql 的注册表与分区无关**：`data_retention_registry` 覆盖的是另一批表
   （`event_datas`、死信表等），不含 `telemetry_datas`。**两者不冲突，也不需要改 141.sql**。
2. **行级 DELETE 的排他性**：138.sql 注释明确写了"行级覆盖的设备由 Go 清理路径排除，
   不与 TimescaleDB 的全局 `drop_after` 叠加误删"。分区方案必须沿用同一纪律：
   **任何 DROP 动作之前，先确认该分区内不存在保留期更长的作用域行**。

---

## 3. 迁移设计（普通 PG 路径）

### 关键取舍：不做全量回填，用 ATTACH 把既有表直接挂成第一个分区

朴素做法是"建分区父表 → 分批 `INSERT ... SELECT` 回填 → 换名"，这要**全量搬运数据**，
百亿行级不可接受。标准替代做法：

```
第 1 步（在线，无长锁）
  ALTER TABLE telemetry_datas
    ADD CONSTRAINT telemetry_datas_ts_range_ck
    CHECK (ts >= <下限> AND ts < <上限>) NOT VALID;
  -- NOT VALID 只加元数据，不扫表，不阻塞写入

第 2 步（在线，可中断）
  ALTER TABLE telemetry_datas VALIDATE CONSTRAINT telemetry_datas_ts_range_ck;
  -- 扫描但只取 SHARE UPDATE EXCLUSIVE 锁，不阻塞读写；耗时与表大小成正比

第 3 步（元数据，毫秒级锁）
  CREATE TABLE telemetry_datas_p (LIKE telemetry_datas INCLUDING ALL)
    PARTITION BY RANGE (ts);
  -- 注意：唯一索引 UNIQUE(device_id,key,ts) 含分区键 ts，符合 PG 要求，可保留

第 4 步（元数据，毫秒级锁）
  ALTER TABLE telemetry_datas_p
    ATTACH PARTITION telemetry_datas
    FOR VALUES FROM (<下限>) TO (<上限>);
  -- 因为第 1/2 步的 CHECK 约束已存在且有效，PG 跳过全表扫描，只改元数据
```

**第 5 步：换名**（一个事务内完成，锁窗口极短）

```sql
BEGIN;
  ALTER TABLE telemetry_datas      RENAME TO telemetry_datas_part0;
  ALTER TABLE telemetry_datas_p    RENAME TO telemetry_datas;
COMMIT;
```

> ⚠️ 换名后 **`telemetry_datas` 这个名字指向分区父表**，原表成为其中一个分区
> `telemetry_datas_part0`。应用侧 SQL 完全不用改（GORM 按表名访问）。
> 但**必须先确认没有代码依赖"`telemetry_datas` 是普通表"**（如 `pg_class.relkind='r'` 判断）。

**第 6 步：建未来分区**（幂等，可放 cron）

按月建未来 3 个月的分区；到点前若未建，插入会因"no partition for row"直接报错——
所以**必须先建**。建议并入既有 `CleanSystemDataByCron` 的执行体，或另起一个日跑任务。

### 风险清单

| 风险 | 缓解 |
|---|---|
| 换名瞬间若有长事务持锁 → 阻塞 | 换名用 `lock_timeout` 兜底，失败则重试；选低峰执行 |
| 未来分区未及时创建 → 写入报错 | 提前建 3 个月；cron 失败要告警（不能静默） |
| `LIKE ... INCLUDING ALL` 会连带复制旧表的 CHECK 约束 | 明确列出要复制的项；`INCLUDING ALL` 含 constraints/indexes/defaults，需逐项核对 |
| 唯一索引在分区父表上创建方式 | 分区父表的唯一索引必须是**分区键的超集**；`(device_id,key,ts)` 含 `ts` ✓，可建 |
| GORM AutoMigrate 可能与分区父表冲突 | 迁移后跑一次后端启动，确认 `pg_init.go` 的 AutoMigrate 不对该表做 DDL |
| 回滚 | 换名反向执行即可（分区内数据就是原数据，无搬运，**回滚是元数据操作**）。这是本方案相对回填方案的最大优势 |

### 与 57.sql 的守卫

分区迁移脚本**必须**先判断：

```sql
-- 仅当未安装 timescaledb 扩展时才执行分区迁移
SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb');
```

为真则**整体跳过**（hypertable 已是分区表，再套声明式分区会冲突）。
同时 Go 侧 `initialize` 需把 `AETHERLINK_TIMESCALE_MODE=off` 与"执行分区迁移"对齐，
避免两处判断口径不一致。

---

## 3.5 迁移已实现并在真集群上验证（2026-10-01 08:4x）

上面第 3 节的设计**已经写成脚本并在真实 PG 17 集群上完整跑通**，不再只是纸面方案：

| 交付物 | 说明 |
|---|---|
| `deploy/maintenance/partition_telemetry_datas.sql` | 迁移脚本（幂等，含 Timescale 守卫） |
| `deploy/maintenance/rollback_partition_telemetry_datas.sql` | 回滚脚本（幂等，含"未来分区非空则拒绝回滚"的安全检查） |

**为什么放在 `deploy/maintenance/` 而不是 `backend/sql/NNN.sql`**：脚本用了 psql 元命令
（`\gset` / `\if`），项目的自动迁移链按普通 SQL 执行、不认识反斜杠命令；且计划本就把它
归为"需带外迁移步"。若将来要纳入自动迁移链，只需把 `\gset` 的边界计算改写成 DO 块内
动态 SQL，其余语句本身都是普通 SQL。

### 实测环境与结果

环境：隔离集群 PG 17.5（端口 55433），库 `aetherlink_go99`，`telemetry_datas`
**102881 行 / 34 MB / 10 个设备**，`UNIQUE(device_id,key,ts)`。

| 项 | 结果 |
|---|---|
| 迁移耗时 | **2162 ms**（含 CHECK 约束的 VALIDATE 扫表） |
| 回滚耗时 | **1247 ms** |
| 数据指纹（迁移前 / 迁移后 / 回滚后） | `102881 \| -118365821522 \| 1788680163203 \| 1789833023455 \| 10` —— **三次完全一致** |
| 换名后形态 | `telemetry_datas` = 分区父表（0 字节）；`telemetry_datas_legacy` = 34 MB 持有全部行 |
| 未来分区 | 自动建 2026_10 / 2026_11 / 2026_12 |
| 唯一约束 | 传播到各分区（`telemetry_datas_2026_10_device_id_key_ts_key`），重复插入被正确拦下 |
| 分区裁剪 | 限定 ts 范围的查询 EXPLAIN 只扫 `telemetry_datas_legacy`，不碰空分区 |
| 写入路由 | 2026-10 的数据正确落到 `telemetry_datas_2026_10` |
| 缺失分区 | 2030 年的数据插入报 `no partition of relation "telemetry_datas" found for row` —— 印证第 6 步预建分区的必要性 |

### ⚠️ 实测中发现并修掉的一个真 bug（务必记住）

**现象**：迁移后**任何新数据都插不进去**，报
`new row for relation "telemetry_datas_2026_10" violates check constraint "telemetry_datas_ts_partition_ck"`。

**根因**：第 3 步建分区父表时用了 `LIKE telemetry_datas INCLUDING ALL`，而
`INCLUDING ALL` 含 `INCLUDING CONSTRAINTS`，会把第 2 步为 legacy 数据加的
`CHECK (ts >= lo AND ts < hi)` **一起复制到父表**。父表的约束会**传播到每一个分区**，
于是所有分区都被限制在 legacy 区间内 —— 分区形同虚设。

**修复**：父表改为 `LIKE ... INCLUDING ALL EXCLUDING CONSTRAINTS`，并额外加一句
`DROP CONSTRAINT IF EXISTS telemetry_datas_ts_partition_ck` 兜底（兼容旧版本脚本产生的形态）。
该 CHECK 只对"被 ATTACH 的那张既有表"有意义（让 ATTACH 跳过全表扫描），对父表毫无用处。

**自检已内置**：脚本第 7 步会列出父表上的 CHECK 约束，**必须为空**；非空即说明此坑复现。

> 这个 bug 在纸面设计阶段完全看不出来——`INCLUDING ALL` 的约束传播是 PG 的语义细节。
> **这就是为什么分区改造必须先在隔离库演练，不能直接上生产。**

### 另外两个实现细节（改脚本时别踩）

1. **psql 不在 dollar-quoted 块（`$tag$...$tag$`）内替换 `:var`**。第一版把依赖分区边界的
   语句写进了 DO 块，直接语法错误 `syntax error at or near ":"`。现在统一走
   `\gset` + `\if`（psql 侧替换）；唯一例外是第 6 步的循环，它通过 `set_config` 把值塞进
   会话 GUC、DO 块内用 `current_setting` 读回。
2. **不要用 `pg_stat_user_tables.n_live_tup` 做"分区是否为空"的判定**——该统计可能长期
   未刷新而恒为 0（本库实测：102881 行的表显示 `n_live_tup=0`）。回滚脚本改用真实 `count(*)`。

---

## 4. 建议的执行顺序

1. **现在**：跑 `measure_telemetry_volume.sql`，拿数（只读，无风险）
2. **决策 1 达标才继续**；不达标则本项结案为"观察"，把额度让给 Wave7-C
3. 达标则：
   - ~~写迁移脚本~~ → **已完成并在隔离库演练通过**（见第 3.5 节），脚本已就位
   - 加分区维护任务（提前建分区 + 告警）—— **这是唯一还没做的工程项**
   - **只做方案 A**（分区用于裁剪，过期仍走行级 DELETE）
   - 生产执行前，再在**与生产同版本的 PG 上**跑一遍演练（本次演练环境是 PG 17.5）
4. **方案 B（按最大保留期 DROP）单独排期**，等 138.sql 作用域行实际启用后再评估

---

## 5. 本方案回答不了、必须你来定的

| 问题 | 为什么工程定不了 |
|---|---|
| 生产 `telemetry_datas` 现在多少行？ | 我本地只有开发库，没有生产数据。**跑脚本第 2/3 段即可** |
| 能接受多长的维护窗口？ | 涉及运维值班与 SLA。方案已设计成零停机，但换名瞬间需一个极短锁窗口 |
| 是否允许"分区内过期行不自动释放"（方案 A 的代价）？ | 涉及存储成本与合规。若合规要求"到期必须物理删除"，则方案 A 不满足，需直接上 B |
| 租户是否真的会用档案级/租户级保留期？ | 若**从不使用**（只有全局默认行），则整分区 DROP 完全安全，**直接上 B 且收益最大** |

> 最后一条最关键：**如果 138.sql 的作用域行实际上没人用，本项难度立刻降一个量级。**
> 脚本第 6 段最后一行 `scoped_policy_rows` 就是答案——**如果它是 0，请直接批方案 B。**
