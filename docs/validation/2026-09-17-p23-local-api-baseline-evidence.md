# P2.3 本地 API 基线（第一批真实容量数字）

> 日期：2026-09-17
> 仓库：`aetherlink-iot`
> 对应路线图：§3 P2.3「数据保留与性能」——此前记 **`环境阻塞`，零容量数字**
> 脚本：`performance/scripts/api-load-baseline.js`（本次新增）
> 原始报告（本地工件）：`performance/reports/local-api-baseline-20260917.json`、
> `local-api-deployment-health-20260917.json`
>
> **注意**：`performance/reports/` 被 `.gitignore` 有意排除（生成物不入库），
> 因此原始 JSON **不随提交版本化**，仅保留在本机。本文件已完整复现其中的关键数字与
> 全部参数、环境信息与复现命令，故证据本身可独立引用；但"原始归档"这一层
> 在版本库里是缺失的——若需要可追溯的原始归档，应另定一个不受 ignore 规则约束的目录。

---

## 一、为什么之前"没有任何容量数字"

核对 `performance/` 后发现：**脚手架是齐的，尺子没造出来**。

已有：`tiers.json`（1c2g / 2c4g / 4c8g 三档 SLO）、`scenarios/`（三个场景定义）、
`scripts/run-tier-benchmark.ps1`、`capture-resource-snapshot.ps1`、`summarize-tier-report.js`。

但 `run-tier-benchmark.ps1` 实际只做两件事：抓 `/health` 与 `/api/v1/deployment/health`
的响应，然后跑一遍既有自动化测试套件。**它不产生任何负载**——`tiers.json` 里的
`apiConcurrentUsers` / `mqttClients` 从来没有被真正施加过。

所以"零容量数字"不是环境问题，是**缺一个负载生成器**。

---

## 二、本次新增：`api-load-baseline.js`

固定并发 N 个 worker 持续打目标端点，记录每次请求端到端延迟，算 p50/p90/p95/p99/max、
吞吐与错误率。**全程只用 Node 标准库**（`http`/`https`）——本机网络受限，引入
k6 / autocannon 这类依赖本身就是风险。

三个刻意的设计选择：

| 选择 | 理由 |
| --- | --- |
| 预热期样本**不计入统计** | 首轮请求要付连接建立、连接池填充与 JIT 的代价，混进 p95 会把基线抬到没有参考价值 |
| 百分位用**最近秩（nearest-rank）**而非插值 | 样本量小时插值会给出一个**实际从未发生过**的延迟值 |
| 错误**计入错误率而非丢弃** | 只统计成功请求的延迟会得出一个漂亮的假基线 |
| 输出显式标注 `evidenceKind: "local-baseline"` 且 `tierClaim: null` | **这不是 tier 结果**：`tiers.json` 描述的是资源限制，本脚本不施加任何配额。把笔记本数字冒充 tier 达标，比没有数字更糟 |

---

## 三、实测结果

环境：AMD Ryzen 5 5600X ×12，16,304 MB，win32/x64。
单实例，本地回环，测试期间另一并行工作流处于静默（无其它负载）。
参数：并发 4，预热 3s，测量 15s。

| 端点 | 请求数 | 失败 | 吞吐 | p50 | p90 | p95 | p99 | max |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `/health`（不触库） | 58,812 | 0 | **3,920 rps** | 0.32 ms | 0.59 ms | 7.18 ms | 12.13 ms | 24.65 ms |
| `/api/v1/deployment/health`（触库 + Redis + MQTT + 文件存储） | 35,643 | 0 | **2,376 rps** | 0.35 ms | 4.29 ms | 12.41 ms | 18.77 ms | 37.34 ms |

两个端点**全程 0 失败**。

### 3.1 一条可行动的观察

触库端点的 **p50 = 0.35 ms 而 p90 = 4.29 ms，相差 12 倍**——分布明显是双峰的，
不是"整体慢"而是"多数极快、少数明显慢"。`/health` 的 p50→p95 也有 22 倍跳变。

这类形态通常指向**连接池取用/回收**或**首次查询后缓存命中**，而不是数据库本身慢。
下一步应当按分位分桶看样本落点，而不是笼统地"优化 SQL"。

### 3.2 顺带确认的运行期事实

`/api/v1/deployment/health` 的响应确认了当前活栈状态：

```
database ok · db_migrations ok（最新迁移版本：108）· file_storage ok · mqtt ok · redis ok · status_redis ok
```

---

## 四、这批数字**不能**用来宣称什么（如实记录）

1. **不能宣称任何 tier 达标**。1c2g / 2c4g / 4c8g 描述的是 CPU/内存配额下的表现，
   本次未施加任何配额，也未在容器/虚拟机上限制资源。
2. **不能代表数据库容量**。`/health` 完全不触库；触库端点也只做轻量检查
   （`latency_ms` 自报 0~1 ms），**不是**遥测写入、聚合查询或大批量读的压力。
   路线图 §P2.3 明确要求"压测不使用假数据掩盖数据库瓶颈"——这批数字恰好不覆盖瓶颈。
3. **只有单实例**。路线图门禁要求"至少单实例和双实例报告"，双实例未做。
4. **两个场景完全没测**：`telemetry-ingest-mqtt`（MQTT 摄取）与 `browser-e2e-smoke`
   （浏览器首屏）本次未执行。
5. **本机为开发机**，有编辑器、浏览器等常驻负载，与干净压测环境不可比。

因此 P2.3 的状态从「零容量数字」变为「**有本地基线数字，tier 达标证据仍 pending**」，
**不宣称 P2.3 有任何门禁通过**。

---

## 五、复现命令

```bash
# 前置：活栈在跑（后端 9999）
cd <repo>
node performance/scripts/api-load-baseline.js \
  --base-url http://127.0.0.1:9999 --path /health \
  --concurrency 4 --duration 15 --warmup 3 \
  --out performance/reports/local-api-baseline-<date>.json

# 触库端点
node performance/scripts/api-load-baseline.js \
  --base-url http://127.0.0.1:9999 --path /api/v1/deployment/health \
  --concurrency 4 --duration 15 --warmup 3 \
  --out performance/reports/local-api-deployment-health-<date>.json
```

需要鉴权的端点用 `--header "x-token: <token>"`。

---

## 六、下一步（按性价比）

1. **给 MQTT 摄取路径造负载**（`telemetry-ingest-mqtt` 场景）——它才是物联网平台的真瓶颈，
   而当前完全无数据。
2. **按分位分桶定位双峰来源**（连接池 vs 查询缓存），再决定要不要优化。
3. **双实例报告**需先解决多实例部署前提（与 `TB-7` 队列隔离/集群化同源）。
4. tier 达标证据需要**带资源配额的容器环境**，本机做不到，保持 pending。
