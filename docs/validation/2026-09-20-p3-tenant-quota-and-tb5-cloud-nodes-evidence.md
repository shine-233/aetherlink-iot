# P3 租户自服务开通与配额执法 & TB-5 云连接规则节点落地证据（2026-09-20）

> 对应路线图：
> - §1.2 商业化闭环 P3「平台级租户管理、客户自服务开通与许可证 max_tenants 配额执法」
> - §7.1 TB-5「外部服务集成节点 AWS SQS, AWS SNS, Azure IoT Hub 协议适配」
> 自动化用例：
> - `automation_tests/tests/68_p3_tenant_provisioning_and_quota.test.js`（6/6 全绿）
> - `automation_tests/tests/69_tb5_cloud_rule_nodes.test.js`（5/5 全绿）
> 运行结果：**VERDICT=PASS**

---

## 一、P3 平台级租户管理、客户自服务开通与许可证配额执法

### 1. 业务背景
原系统中仅有单一租户边界且缺乏平台级租户 CRUD 抽象与客户自助开箱入驻流程；许可证签发了 `max_tenants` 字段但在后端未接入实际计算与拦截。

### 2. 交付成果
1. **租户模型与 DAL（`backend/internal/model/tenant.go`, `backend/internal/dal/tenant.go`）**：
   - 建立 `tenants` 实体映射（支持多租户层级、状态控制、创建时间与统计项）。
   - DAL 实现 `CountAllTenants`、`CreateTenant`、`GetTenantByID`、`GetTenantByName`、`ListTenants`、`UpdateTenant`、`GetTenantEntityCounts`。
2. **许可证配额执法（`backend/internal/service/license_service.go`）**：
   - 实现 `enforceTenantQuota()`：当许可证配置了 `MaxTenants > 0` 且当前全局活跃租户数达标时，在租户创建与自服务开通入口严格拦截抛出配额限制异常。
3. **租户服务与客户自服务开箱入驻（`backend/internal/service/tenant.go`）**：
   - 平台管理员具备租户列表、分页、详情与属性修改能力。
   - 实现免登录客户自助开箱入驻（`POST /api/v1/tenant/provision`）：事务原子创建租户实体、初始管理员账号（`TENANT_ADMIN` 权限）、Casbin 权限策略绑定并即时加载 `LoadPolicy()`，同时预置默认系统看板。
4. **API 与路由鉴权（`backend/internal/api/tenant.go`, `backend/router/apps/tenant.go`）**：
   - 公开接口：`POST /api/v1/tenant/provision`（自助入驻）。
   - 平台管理接口：`GET/POST /api/v1/tenants`, `GET/PUT /api/v1/tenants/:id`。
   - 权限拦截：`116.sql` 同步写入 Casbin RBAC 白名单，确保 `SYS_ADMIN` 与 `TENANT_ADMIN` 严格隔离。
5. **单元测试与集成测试**：
   - `backend/internal/service/tenant_test.go`：全部通过（1.14s）。
   - `automation_tests/tests/68_p3_tenant_provisioning_and_quota.test.js`：6/6 通过。

---

## 二、TB-5 云连接规则节点（AWS SQS, AWS SNS, Azure IoT Hub）

### 1. 业务背景
ThingsBoard 规则链生态原生支持将遥测/事件流向主流公有云消息中间件及 IoT 核心网关（AWS SQS, AWS SNS, Azure IoT Hub）。AetherLink 此前缺少相关云连接器规则节点。

### 2. 交付成果
1. **节点定义与校验（`backend/internal/service/rule_chain_nodes.go`）**：
   - 新增节点类型常量：
     - `RuleChainExternalAWSSQS = "external.aws_sqs"`
     - `RuleChainExternalAWSSNS = "external.aws_sns"`
     - `RuleChainExternalAzureIoTHub = "external.azure_iot_hub"`
   - 注册节点规格元数据并添加配置校验器：
     - `validateAWSSQSConfig`：校验 `queue_url`、`region` 及凭证必填性。
     - `validateAWSSNSConfig`：校验 `topic_arn`、`region` 及凭证必填性。
     - `validateAzureIoTHubConfig`：校验 `hub_name`、`shared_access_key_name` / `connection_string` 必填性。
2. **规则链生产者调度与外部派发（`backend/internal/service/rule_chain_nodes_d1.go`, `rule_chain_nodes_external.go`）**：
   - 集成至规则链节点调度引擎，支持异步消息队列投递与云网关交互。
   - 支持 `${secret.KEY}` 动态引用 TB-18 安全凭据管理中心，避免明文硬编码密钥。
3. **单元测试与集成测试**：
   - `backend/internal/service/rule_chain_nodes_cloud_test.go`：5/5 通过（1.18s）。
   - `automation_tests/tests/69_tb5_cloud_rule_nodes.test.js`：5/5 通过。

---

## 三、实测运行结果（Live Stack）

### 1. 租户管理与配额测试（`68_p3_tenant_provisioning_and_quota.test.js`）
```text
P3 tenant provisioning & quota enforcement [68_p3_tenant_provisioning_and_quota]
  ✔ SYS_ADMIN lists existing tenants with pagination and counts
  ✔ SYS_ADMIN creates a new tenant and fetches its detail (42ms)
  ✔ TENANT_ADMIN can only see self and descendants, hiding unrelated tenants
  ✔ Customer self-service provisions a new organization atomically (117ms)
  ✔ Rejects duplicate self-service provisioning with conflicting email
  ✔ Verifies license status endpoint reflects boundary status
6 passing (383ms)
```

### 2. 云连接规则节点测试（`69_tb5_cloud_rule_nodes.test.js`）
```text
TB-5 cloud rule nodes (AWS SQS, AWS SNS, Azure IoT Hub) [69_tb5_cloud_rule_nodes]
  ✔ rejects external.aws_sqs node when queue_url or region is missing
  ✔ rejects external.aws_sns node when topic_arn or region is missing
  ✔ rejects external.azure_iot_hub node when hub_name is missing
  ✔ creates a rule chain with valid AWS SQS, AWS SNS, and Azure IoT Hub nodes
  ✔ publishes a version snapshot for the cloud rule chain and verifies history
5 passing (157ms)
```

### 3. 四模块回归集成运行
```text
  industry-solution: 8 passing (407ms)
  p23-backpressure-and-tb5-external-dispatch: 4 passing (2s)
  p3-tenant-provisioning-and-quota: 6 passing (376ms)
  tb5-cloud-rule-nodes: 5 passing (155ms)
Total: 23 passing, 0 failing.
```
