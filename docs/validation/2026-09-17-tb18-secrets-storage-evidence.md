# TB-18 通用 Secrets Storage 全链路闭环（Universal Secrets Management & Storage End-to-End）运行期证据

> 日期：2026-09-17  
> 责任范围：ROADMAP TB-18 通用 Secrets Storage 全链路闭环（对标 ThingsBoard PE 核心企业级安全特性 Universal Secrets Management：联合唯一键 `(tenant_id, key)`、AES-256-GCM 信封加密密文、AAD 租户绑定防篡改搬运、脱敏掩码、/reveal 审计解密记录至 `operation_logs`、NeedsReseal 与 Reseal 在线重加密轮换、service.ResolveSecret 内部下游动态解析、前端工作台 15s 倒计时销毁查看、vitest 单测与端到端自动化契约测试）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=109`）+ Redis（`127.0.0.1:6379`）+ MQTT Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 架构设计与代码变更清单

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 数据库迁移与 Casbin 权限 | `backend/sql/109.sql`<br>`backend/pkg/global/global.go` | ① 创建 `sys_secrets` 数据表（联合唯一约束 `(tenant_id, key)`、AES-256-GCM 信封密文存储、脱敏前缀 `masked_preview`、轮换重密标记 `needs_reseal`、PG 索引优化）；② 向 `casbin_rule` 注册 `/api/v1/secrets*` 路由并按最小特权赋权 `SYS_ADMIN`、`TENANT_ADMIN`、`TENANT_USER`（TENANT_USER 仅允许读脱敏列表，禁止 `/reveal`、`/reseal` 与写操作）；③ 挂载前端菜单 `management_secrets` 到系统管理模块；④ `VERSION_NUMBER` 递增至 `109` |
| 后端密码学内核 | `backend/pkg/secrets/envelope.go`<br>`backend/pkg/secrets/envelope_test.go` | 复用成熟的 `pkg/secrets` 信封加密（AES-256-GCM，AAD 绑定租户 ID 防止跨租户密文搬运攻击，主密钥版本标记与动态密钥轮换检测）；8 组密码学单测 100% 通过 |
| 实体模型与 DTO | `backend/internal/model/secret.go` | 定义 `SysSecret` 实体模型、`CreateSecretReq`、`UpdateSecretReq`、`SecretListReq`、`SecretResp`、`RevealSecretResp`、`SecretPageResult`，配置 JSON 与表结构映射 |
| 数据访问层（DAL） | `backend/internal/dal/secret.go` | 实现 `CreateSecret`、`GetSecretByID`、`GetSecretByKey`、`UpdateSecret`、`DeleteSecret`、`ListSecrets`，包含严格的租户隔离前置查询防护，避免跨租户越权操作 |
| 业务服务层（Service） | `backend/internal/service/secret.go`<br>`backend/internal/service/secret_test.go` | ① 强参数校验（Key 正则 `^[a-zA-Z0-9_-]{2,128}$`、类型白名单 `generic|api_key|token|password|certificate|oauth_client`）；② 掩码脱敏函数（前 2 后 2 或超短值全掩码）；③ 审计解密（`/reveal` 解密原始明文并异步落盘 `operation_logs` 审计日志，日志中严防明文泄漏）；④ 在线轮换重加密（`ResealSecret` 检查并重新封包）；⑤ `ResolveSecret` 内部解析函数，支持 `${secret.KEY}`、`secret:KEY`、`KEY` 三种主流下游引用语法；⑥ 6 组服务层单测 100% 通过 |
| API 控制器与路由层 | `backend/internal/api/secret.go`<br>`backend/internal/api/enter.go`<br>`backend/router/apps/secret.go`<br>`backend/router/apps/enter.go`<br>`backend/router/router_init.go` | ① 暴露 `POST /secrets`、`GET /secrets`、`GET /secrets/:id`、`PUT /secrets/:id`、`DELETE /secrets/:id`、`POST /secrets/:id/reveal`、`POST /secrets/:id/reseal`；② 挂载 `SecretRouter`；③ Casbin 路由覆盖自动化审计通过（400 条受保护路由 100% 覆盖） |
| 前端 API 客户端与多语言 | `frontend/src/service/api/secret.ts`<br>`frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/route.json` | ① 封装 7 个强类型 API 方法；② 补齐四国语言路由名称国际化翻译 |
| 前端管理工作台 | `frontend/src/views/management/secrets/index.vue`<br>`frontend/src/views/management/secrets/__tests__/index.test.ts` | ① 支持关键字搜索、类型过滤、分页与表格展示；② 新增/编辑模态框（支持密码生成器、明文掩码切换）；③ 解密查看安全抽屉（15 秒倒计时内存销毁、一键复制剪贴板、安全警告提示）；④ 在线轮换重加密与物理删除；⑤ 组件单元测试 2/2 通过，`pnpm run typecheck` 0 错误 |
| 自动化端到端契约测试 | `automation_tests/tests/56_secrets_storage.test.js` | 10 组端到端契约用例：创建密钥与信封加密验证、脱敏列表检索与过滤、详情脱敏回显、同租户重复 Key 冲突校验、跨租户隔离与同名命名空间互不干扰、管理员解密审计、TENANT_USER 角色 403 阻断但允许脱敏列表、PUT 更新与轮换、Reseal 在线重加密、物理删除与 404 验证 |

---

## 2. 单元测试与密码学验证

### 2.1 后端服务与密码学单元测试
```powershell
$env:GOTOOLCHAIN="local"
go test -v ./internal/service -run TestSecret
go test -v ./pkg/secrets/...
```
输出：
```text
=== RUN   TestValidateSecretKey
--- PASS: TestValidateSecretKey (0.00s)
=== RUN   TestValidateSecretType
--- PASS: TestValidateSecretType (0.00s)
=== RUN   TestMaskSecretValue
--- PASS: TestMaskSecretValue (0.00s)
=== RUN   TestResolveSecretReferencePattern
--- PASS: TestResolveSecretReferencePattern (0.00s)
=== RUN   TestSecretEnvelopeEncryptionAndTenantIsolation
--- PASS: TestSecretEnvelopeEncryptionAndTenantIsolation (0.00s)
=== RUN   TestSecretTamperDetection
--- PASS: TestSecretTamperDetection (0.00s)
PASS
ok      aetherlink-iot/backend/internal/service 0.046s

=== RUN   TestEnvelopeEncryptDecrypt
--- PASS: TestEnvelopeEncryptDecrypt (0.00s)
=== RUN   TestEnvelopeAADMismatch
--- PASS: TestEnvelopeAADMismatch (0.00s)
=== RUN   TestEnvelopeTamperCiphertext
--- PASS: TestEnvelopeTamperCiphertext (0.00s)
=== RUN   TestEnvelopeTamperTag
--- PASS: TestEnvelopeTamperTag (0.00s)
=== RUN   TestEnvelopeDifferentTenants
--- PASS: TestEnvelopeDifferentTenants (0.00s)
=== RUN   TestEnvelopeKeyRotation
--- PASS: TestEnvelopeKeyRotation (0.00s)
=== RUN   TestEnvelopeEmptyPlaintext
--- PASS: TestEnvelopeEmptyPlaintext (0.00s)
=== RUN   TestEnvelopeLegacyFallback
--- PASS: TestEnvelopeLegacyFallback (0.00s)
PASS
ok      aetherlink-iot/backend/pkg/secrets      0.038s
```

### 2.2 Casbin 路由登记与权限全量审计
```powershell
$env:GOTOOLCHAIN="local"
go test -v ./router/apps -run TestCasbinRouteRegistration_AllProtectedRoutesRegistered
```
输出：
```text
=== RUN   TestCasbinRouteRegistration_AllProtectedRoutesRegistered
    casbin_route_registration_contract_test.go:41: Total protected routes registered in Gin engine: 400
--- PASS: TestCasbinRouteRegistration_AllProtectedRoutesRegistered (0.01s)
PASS
ok      aetherlink-iot/backend/router/apps      0.048s
```

### 2.3 前端组件单元测试与 TypeScript 类型检查
```powershell
pnpm --dir frontend run test:unit src/views/management/secrets
pnpm --dir frontend run typecheck
```
输出：
```text
 ✓ src/views/management/secrets/__tests__/index.test.ts (2 tests) 263ms
   ✓ Secrets Management View > renders correctly with table and action buttons
   ✓ Secrets Management View > supports secret type options correctly

 Test Files  1 passed (1)
      Tests  2 passed (2)
   Start at  07:44:18
   Duration  1.61s (transform 328ms,setup 399ms,collect 188ms,tests 263ms,environment 567ms,prepare 97ms)

> vue-tsc --noEmit --skipLibCheck
Done in 3.65s.
```

---

## 3. 自动化契约测试结果（`56_secrets_storage.test.js`）

```powershell
node run_tests.js --module secrets-storage
```
执行结果：**10/10 100% 通过**，耗时 0.7s。

详细用例结果：
```text
  TB-18 Universal Secrets Storage End-to-End [56_secrets_storage]
    √ 1. POST /api/v1/secrets: 创建通用密钥（写入 AES-256-GCM 信封密文，默认返回脱敏掩码） (10ms)
    √ 2. GET /api/v1/secrets: 列表检索支持分页、关键字模糊查询与类型过滤（全量脱敏） (10ms)
    √ 3. GET /api/v1/secrets/:id: 获取单条密钥详情，验证元数据回显与脱敏掩码 (10ms)
    √ 4. 同一租户下重复 Key 校验：冲突拒绝（CodeParamError） (10ms)
    √ 5. 跨租户命名空间与隔离：租户 B 可创建同名 Key，但无法访问租户 A 的密钥 (41ms)
    √ 6. POST /api/v1/secrets/:id/reveal: 管理员解密验证，精准还原原始明文并记录审计 (15ms)
    √ 7. 细粒度 RBAC 门禁：TENANT_USER 角色调用 /reveal 被 403 阻断，但可读取脱敏列表 (19ms)
    √ 8. PUT /api/v1/secrets/:id: 更新元数据与轮换明文凭据 (22ms)
    √ 9. POST /api/v1/secrets/:id/reseal: 在线轮换重加密成功 (11ms)
    √ 10. DELETE /api/v1/secrets/:id: 物理删除密钥，后续访问返回 404 (22ms)

  10 passing (170ms)
```

---

## 4. 跨模块多套件联合回归结果

涵盖关系图谱、多层网关、告警规则2.0、统一资源中心、队列隔离与集群限流、单位换算、通用密钥存储 7 大企业级模块：
```powershell
node run_tests.js --module entity-relations,multilayer-gateway,alarm-rules-advanced,resource-center-market,queue-isolation-clustered-rate-limit,units-conversion,secrets-storage
```
执行结果：**103/103 100% 全部通过**。

模块通过明细：
1. `entity-relations`（`46_entity_relations.test.js`）：**26/26 通过**
2. `multilayer-gateway`（`51_multilayer_gateway.test.js`）：**5/5 通过**
3. `alarm-rules-advanced`（`52_alarm_rules_advanced.test.js`）：**18/18 通过**
4. `resource-center-market`（`53_resource_center_market.test.js`）：**21/21 通过**
5. `queue-isolation-clustered-rate-limit`（`54_queue_isolation_clustered_rate_limit.test.js`）：**11/11 通过**
6. `units-conversion`（`55_units_conversion.test.js`）：**12/12 通过**
7. `secrets-storage`（`56_secrets_storage.test.js`）：**10/10 通过**

---

## 5. 结论

`TB-18` 通用 Secrets Storage（Universal Secrets Management & Storage）已完成数据库、后端密码学信封加密、DAL、Service、API、Casbin 鉴权、前端视图、国际化、单元测试与自动化契约测试全链路闭环，达成生产级交付标准。
