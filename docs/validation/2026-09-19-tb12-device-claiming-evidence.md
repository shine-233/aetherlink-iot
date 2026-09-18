# TB-12 设备认领与自动注册（Device Claiming）——后端全链路证据

> 日期：2026-09-19
> 对应路线图：§7.1 TB-12（对标 ThingsBoard CE Claiming Devices）
> 结论：**迁移/DAL/Service/API/Casbin/OpenAPI/契约测试/Postgres 单测八件齐备**；
> 四面一致中**前端 UI 面未接线**（缺口类型 `未接线`），如实记 partial，不得记 done。

## 一、立项前置检查（§7.4）

按 §7.4 第 6 条先 grep 后立项：`grep -rni claim backend/{internal,sql,router}` 仅命中
mobile 幂等键注释与 AI 层 `userClaims`（无关）；`认领` 仅命中 mobile.go 幂等键注释。
**确认认领流程零实现**，本行 `未实现` 判定成立。

## 二、交付物

| 层 | 文件 | 要点 |
| --- | --- | --- |
| 迁移 | `backend/sql/112.sql`（VERSION_NUMBER=112） | `device_claim_tokens` 表 + **每设备至多一条 active 的 partial unique index** + 状态 CHECK + 3 条 Casbin 路由（SYS_ADMIN/TENANT_ADMIN） |
| 模型 | `internal/model/device_claim.go` | 令牌行 + 三种请求/响应形状；`ClaimKeyHash` 带 `json:"-"`，任何出参都不带哈希 |
| DAL | `internal/dal/device_claim.go` | 一次性与转移全部是**条件更新**（RowsAffected 判定）；跨租户寻址查询带 `tenant-scope: caller-enforced` 标记（租户作用域审计通过） |
| 服务 | `internal/service/device_claim.go` | 签发（明文只出现一次，SHA-256 落库）/列表（无明文无哈希）/撤销（仅 active）/赎回（事务内锁令牌→常量时间比对→消费→转移，任一步失败整体回滚） |
| API | `internal/api/device_claim.go` | 4 端点 + swagger 注解；租户上下文只来自 claims |
| 路由 | `router/apps/device.go` | 全部挂静态前缀 `claim-tokens`（避免与 `device/:id` 通配冲突） |
| OpenAPI | `docs/openapi/openapi.json` | 重生成 **451 paths**，`claim-tokens` 3 条路径收录 |
| 单测 | `internal/service/device_claim_postgres_test.go` | PG 门控生命周期 2 例（缺 DSN 如实 Skip） |
| 契约测试 | `automation_tests/tests/61_device_claim.test.js` | **8 用例活栈全绿** |

## 三、安全边界设计（对应路线图"认领令牌、超时与跨租户边界"）

1. **明文一次性**：`claim_key = ack_<48hex>`（crypto/rand 192 位），仅签发响应出现；
   库内只有 SHA-256 hex（列约束 `length=64`），列表/详情接口无哈希无明文。
2. **一次性**：消费是 `UPDATE … WHERE id=? AND status='active' AND expires_at>now()`，
   并发赎回至多一条成功（RowsAffected=0 即拒）。
3. **超时**：TTL 1s~30 天（缺省 72h）；过期不复活，由读取方判 `effective=expired`。
4. **跨租户边界**：赎回 = 行锁令牌 + 常量时间哈希比对 + 条件消费 +
   `UPDATE devices SET tenant_id=认领方 WHERE id=? AND tenant_id=签发方` 条件转移，
   同一事务；转移同时清空 `owner_user_id`。原租户在转移成功那刻失去可见性。
5. **存在性防探测**：错 key / 无令牌 / 重放 / 过期 / 撤销后赎回——**同码（100404）同文案**
   `device is not claimable with this key`。这是本轮第一版实现后修掉的真缺陷：
   初版错 key 返回 201002、无令牌返回 100404，两者的差异本身就是
   "该设备有活跃令牌"的泄露通道。认领自己租户的设备显式 201002（不属于探测面）。

## 四、实测结果

**Go 单测（真实 PostgreSQL 17.5 @ 55433，`AETHERLINK_TEST_PSQL_DSN` 注入）**：
`TestDeviceClaimLifecycleOnPostgres`、`TestDeviceClaimIssueRequiresOwnershipOnPostgres`
**2/2 PASS**（1.69s）——签发明文唯一性、库内仅哈希、认领自己拒绝、错 key 拒绝、
转移正确性（响应记录原租户 + DB 租户实际变更）、重放拒绝、consumed 后可重签
（partial unique 不变量）、撤销后拒绝、过期拒绝、越权签发=统一 not found。

**活栈契约测试（后端 9999 + PG + Redis，`61_device_claim.test.js`）**：**8/8 全绿（2s）**

```text
  TB-12 device claiming [61_device_claim]
    ✔ issues a one-time claim token and never echoes the plaintext again
    ✔ rejects claiming your own tenant device
    ✔ rejects a wrong key without leaking whether the device exists
    ✔ transfers the device to the claiming tenant and consumes the token
    ✔ rejects replaying the same claim key
    ✔ allows re-issue after consumption and rejects a revoked token
    ✔ rejects an expired claim key (2023ms)
    ✔ validates parameters (empty body, unknown device)
```

**运行期审计**：后端启动日志 `程序版本： 112` → `执行sql文件： sql/112.sql` →
`casbin route audit passed: 410 protected routes registered`（新增 3 条认领路由全部登记）。
回归：`go build ./...` exit 0；`internal/service` + `internal/dal` 全包 ok。

## 五、仍未接线（如实）

- **前端 UI 面**：设备详情页无「生成认领令牌」入口、无「认领设备」表单（device_number +
  claim_key）。后端契约与权限面已闭环，UI 属小开发量（`未接线`）。
- **MQTT 认领通道**：TB 还支持 `v1/devices/me/claim` 设备侧自助认领话题；本实现是
  用户侧 API 认领，设备侧话题通道未做（可作为后续增强）。
