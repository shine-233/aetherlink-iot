# P3 客户自助开通与租户配额执法（Tenant Provision & Quota）运行期证据（2026-09-20）

> 对应路线图：§1.2 P3「客户自助开通」「计费/配额（max_tenants 执法点）」、
> §6 执行顺序「客户自助开通（一并接 max_tenants 执法点）」
> 对应批次：tenant 服务五件套（api/dal/model/service/router）+ `sql/116.sql` +
> `license_service.enforceTenantQuota`；随批附 TB-5 外发规则节点
> （`rule_chain_nodes_external.go`：external.mqtt_forward/kafka/aws_sqs/aws_sns/azure_iot_hub）
> 运行结果：**VERDICT=PASS**

---

## 一、交付物

1. **租户服务（`backend/internal/service/tenant.go` 等）**——平台管理员租户
   创建/列表/层级查询（租户管理员仅可见 self+子孙）；**自助开箱入驻**
   `POST /api/v1/tenant/provision`（免登录，受配额约束）。
2. **max_tenants 执法点（`license_service.enforceTenantQuota`）**——只在
   「有效许可证声明 max_tenants>0」时执行：`CountAllTenants` 部署级计数，
   超配额拒绝并回显 max_tenants/current；边界未启用、材料无效、未声明配额一律
   放行（与设备配额执法同一语义）。接在 `CreateTenant` 与
   `SelfServiceProvisionTenant` 两条写路径前置。这补上了 P3 记录的
   「max_tenants 无执法点」缺口。
3. **迁移 `116.sql`**——`api/v1/tenants`、`api/v1/tenants/:id` Casbin g2/p 登记
   （SYS_ADMIN + TENANT_ADMIN），`VERSION_NUMBER` 115→116。
4. **TB-5 外发规则节点（`rule_chain_nodes_external.go`）**——五个 external 节点
   对标 ThingsBoard AWS SQS/SNS、Azure IoT Hub 规则节点形态（配置校验齐全，
   单测 `rule_chain_nodes_cloud_test.go` 4 组）。

## 二、实测（活栈：116.sql 由后端启动路径自动应用）

1. **迁移**：重建 `backend.exe` 冷启动 → `sys_version=116`（0.0.23）；
   `casbin_rule` 中 `api/v1/tenants`、`api/v1/tenants/:id` 相关 g2/p 行 6 条。
2. **单元/包测试**：`go test ./internal/service/ ./internal/api/ ./internal/app/
   ./router/apps/` → 4 包全 ok（含 `tenant_test.go`、`rule_chain_nodes_cloud_test.go`）。
3. **自助开通冒烟（真实 HTTP，免登录）**：

```
POST /api/v1/tenant/provision {"tenant_name":"drill-smoke-tenant",...}
→ 200 {"tenant_id":"99eb43cf-...","admin_id":"58291388-...","admin_email":"drill-smoke@example.com"}
POST /api/v1/login {"email":"drill-smoke@example.com",...}
→ 200 JWT{authority:"TENANT_ADMIN", tenant_id:"99eb43cf-..."}
```

入驻→登录闭环成立，签发令牌的 authority/tenant_id 与开通返回一致。
参数缺失时返回逐字段 required 校验错误（负向路径实测）。

4. **租户隔离防线（顺带取证）**：另一 TENANT_ADMIN 对该冒烟根租户执行
   `GET /tenants`（不可见，rows=0）与 `DELETE /tenants/:id`（404）——层级可见性
   作用域按设计工作。
5. **回归**：62（TB-19 方案）8/8、66（TB-12 MQTT 认领）4/4、
   67（P2.3 账本+TB-5 外发节点）4/4，共 16/16 全绿。

## 三、清理

冒烟租户/管理员/关联行已按行删除（users 1、tenants 1，复核 0|0）。

## 二·补记：g 角色绑定缺陷的发现与修复（2026-09-20）

首轮活栈冒烟中「入驻返回 200、新管理员可登录（JWT TENANT_ADMIN）」成立，但
**新管理员随后调用 `GET /tenants` 得 403**——Casbin 中间件鉴权主体是
`userClaims.ID`（`CasbinRBAC` → `Casbin.Verify(userID, url)` → `Enforce(user, url, "allow")`），
角色经 `casbin_rule ptype='g'` 行映射；provision 事务只写了
`users.authority` 列、漏插 g 行，导致"登录能过、授权全拒"。

修复（commit `3193954`）：provision 事务内补插 `(g, adminID, TENANT_ADMIN)` 并
`LoadPolicy()` 重载。修复后 **68 号契约 `automation_tests/tests/68_p3_tenant_provisioning_and_quota.test.js`
6/6 全绿**（SYS_ADMIN 分页/建租户/详情、TENANT_ADMIN 仅见 self+子孙、自助开通
原子性、重复 email 拒绝、license status 端点）。62/66/67 回归复核通过。
此前"入驻→登录闭环成立"的结论保留，但以本补记的 68 号 6/6 为最终口径。

## 四、边界（如实）

- 配额执法依赖离线许可证声明（`license.public_keys` 未配置=边界未启用，
  执法自动放行）——与 pkg/license 的既有语义一致，生产启用需注入许可证。
- AWS SQS/SNS、Azure IoT Hub、Kafka 节点为**规则节点形态**（配置校验+外发骨架
  就绪），真实云账号端到端联调仍需客户凭据与网络出口，TB-5 行维持该口径。
