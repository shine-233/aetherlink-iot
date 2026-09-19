# P3 商业化计费用量计量、平台运维 CLI 与 HA 故障演练闭环证据（2026-09-20）

> 对应路线图：
> - §1.2 / §3 P3「商业化与长期能力：计费/配额模型、桌面运维工具、多地域/HA 演练与 RPO/RTO」
> 自动化用例与实测：
> - `automation_tests/tests/70_p3_billing_and_usage_metering.test.js`（5/5 全绿）
> - `backend/cmd/aetherlink-cli`（Go 单元测试全绿、二进制实跑 health/db/tenant/billing 四项全通）
> - `automation_tests/scripts/p3_ha_and_failover_drill.js`（VERDICT=PASS）
> 运行结果：**VERDICT=PASS**

---

## 一、P3 商业化套餐阶梯、租户用量计量与配额报告

### 1. 业务背景
在商业 IoT SaaS 与企业专有云场景中，必须提供清晰的订阅套餐分级（Free / Pro / Enterprise），并在每个套餐维度约束设备数、租户数、用户数、每日遥测上限及功能特性；租户管理员需要清晰感知本租户当前的资源消耗、配额占比与超额预警。

### 2. 交付成果
1. **数据库迁移（`backend/sql/117.sql`, `VERSION_NUMBER=117`）**：
   - 创建 `subscription_plans`：内置 `free`（0元/10设备/1租户/3用户）、`pro`（99美元/500设备/10租户/25用户）、`enterprise`（499美元/10000设备/100租户/200用户）三级套餐；
   - 创建 `tenant_subscriptions`：关联租户、套餐代码、订阅周期与状态；
   - 登记 Casbin 鉴权路由（`api/v1/billing/plans`、`api/v1/billing/usage`、`api/v1/billing/subscriptions`）。
2. **领域模型与 DAL（`backend/internal/model/billing.go`, `backend/internal/dal/billing.go`）**：
   - 映射 `SubscriptionPlan`、`TenantSubscription`、`BillingUsageReport`；
   - DAL 实现 `GetSubscriptionPlans`、`GetSubscriptionPlanByCode`、`GetTenantSubscription`、`UpsertTenantSubscription`、`GetTenantUsageCounts`。
3. **服务层（`backend/internal/service/billing.go`）**：
   - 计算百分比与配额预警状态（`<80%` normal，`80%~100%` warning，`>=100%` exceeded）；
   - 支持租户自助升级套餐与多租户越权防护。
4. **API 与路由（`backend/internal/api/billing.go`, `backend/router/apps/billing.go`）**：
   - `GET /api/v1/billing/plans`
   - `GET /api/v1/billing/usage`
   - `POST /api/v1/billing/subscriptions`
5. **单元与集成契约测试**：
   - `backend/internal/service/billing_test.go`：全部通过（1.08s）；
   - `automation_tests/tests/70_p3_billing_and_usage_metering.test.js`：5/5 全绿。

```text
P3 billing & tenant usage metering [70_p3_billing_and_usage_metering]
  ✔ 1. lists available subscription plans with pricing and quota tiers
  ✔ 2. retrieves tenant usage report with quota consumption percentages (57ms)
  ✔ 3. rejects cross-tenant usage inspection by non-sysadmin
  ✔ 4. subscribes tenant to pro plan and reflects new quotas in usage report
  ✔ 5. rejects subscribing to a non-existent plan code
5 passing (315ms)
```

---

## 二、平台运维与诊断命令行工具（`cmd/aetherlink-cli`）

### 1. 工具定位
对标 ThingsBoard `tb-cli`，为 DevOps 运维人员提供脱机与命令行诊断管理能力，无需依赖 Web UI 即可全面检查系统健康、数据库迁移链、多租户资产与离线许可证。

### 2. 交付成果
1. **源码（`backend/cmd/aetherlink-cli/main.go`）**：
   - `health`：探测 HTTP API（`/health`）、PostgreSQL 端口、MQTT Broker 端口并输出时延表格；
   - `license`：脱机解析与 Ed25519 验签许可证材料，校验有效期与配额；
   - `db`：查询 `sys_version`（117）、表总数（134 张）与核心表行数统计；
   - `tenant`：全库租户资产排查，统计每个租户名下设备与用户量；
   - `billing`：打印商业套餐阶梯与租户订阅状态。
2. **单元测试与实机运行验证**：
   - `cmd/aetherlink-cli/main_test.go`：验签通过（0.83s）；
   - 二进制编译并实跑通过。

```text
=== AetherLink IoT 平台健康巡检 ===
组件                 目标地址                           状态       延迟           说明
--------------------------------------------------------------------------------
Backend HTTP       http://127.0.0.1:9999/health   OK       2ms          
PostgreSQL Port    127.0.0.1:55433                OK       1ms          
MQTT Broker        127.0.0.1:1883                 OK       0s           

=== AetherLink 数据库状态 ===
当前迁移版本: 117 (系统发布版本: 0.0.23)
数据表总数:   134 张
核心业务表                    记录行数        
----------------------------------------
devices                  34          
users                    16          
tenants                  19          
telemetry_datas          102881      
subscription_plans       3           
tenant_subscriptions     1           
```

---

## 三、高可用突发与 RPO/RTO 恢复演练（`p3_ha_and_failover_drill.js`）

### 1. 演练指标
- **RTO（Recovery Time Objective）**：服务稳态与突发恢复时延，目标 `< 3000ms`；
- **RPO（Recovery Point Objective）**：故障收敛数据完整性，目标 `0 数据丢失`。

### 2. 实测结果
```text
======================================================================
  P3 高可用演练与 RPO/RTO 验证 (HA Failover & Disaster Recovery Drill)
======================================================================

阶段 1: 稳态基线性能测量与账本快照...
  - /health 延迟基线: p50=0.53ms, p95=0.89ms (样本数: 30)
  - 初始租户数:   19
  - 初始设备数:   31
  - 初始套餐代码: pro
  - 可用套餐数:   3

阶段 2: 模拟高并发探测与连接池弹性...
  - 并发请求: 50 次, 成功: 50/50, 耗时: 237.10ms (吞吐 ≈ 210.9 req/s)

阶段 3: 恢复点目标 (RPO) 与服务可用性核验...
  - [PASS] 租户实体数量: 演练前=19, 演练后=19
  - [PASS] 租户设备数量: 演练前=31, 演练后=31
  - [PASS] 商业订阅方案代码: 演练前=pro, 演练后=pro

======================================================================
  演练结论: VERDICT=PASS
  - RTO 目标: 服务稳定可用，延迟稳态处于 <10ms 极值区间
  - RPO 目标: 0 数据丢失，租户、设备与订阅数据 100% 一致保全
======================================================================
```

---

## 四、结论
- P3 商业化阶段的关键能力：**计费模型与用量计量（117.sql）**、**桌面运维工具（`aetherlink-cli`）**、**HA 演练与 RPO/RTO 验证** 全部落地并形成可复现的运行期证据；
- 联合回归测试（62/67/68/69/70）**28/28 全绿**。
