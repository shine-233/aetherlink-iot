# P1.5 边缘运维：边缘节点证书与远程升级/回滚运行期证据

> 日期：2026-09-15  
> 责任范围：ROADMAP P1.5 边缘运维（节点证书签发、远程升级与回滚、前端操作工作台、自动化契约测试）  
> 执行基线：PostgreSQL 17.5（`127.0.0.1:55433`，数据库 `aetherlink_go99`，`sys_version=104`）+ GMQTT 回环 Broker（端口 1883）+ 后端服务（端口 9999）

---

## 1. 交付物清单

| 层次 | 文件路径 | 变更说明 |
| --- | --- | --- |
| 数据库迁移 | `backend/sql/104.sql` | 新增 `edge_node_certificates` 与 `edge_node_upgrade_history` 物理表及索引；登记 4 条新路由 Casbin 规则 |
| 全局版本 | `backend/pkg/global/global.go` | `VERSION_NUMBER = 104` |
| 数据模型 | `backend/internal/model/edge_node.go` | 新增 `EdgeNodeCertificate` 与 `EdgeNodeUpgradeHistory` 模型及 HTTP 请求/响应 DTO |
| 数据访问层 | `backend/internal/dal/edge_node_certificate.go` | 证书创建、租户内有效证书查询、批量/按状态吊销、历史查询 |
| 数据访问层 | `backend/internal/dal/edge_node_upgrade.go` | 升级历史流水写入、单条历史查询、节点版本条件更新 |
| 服务层 | `backend/internal/service/edge_node_service.go` | 复用平台 CA 签发 ECDSA P-256 X.509 客户端证书、安全轮换、私钥脱敏；点分版本严格递增校验、在线升级与不可变历史一键回滚 |
| API 控制器 | `backend/internal/api/edge_node.go` | 增加证书签发/查看/吊销、升级/回滚/历史 6 个 RESTful 处理函数 |
| 路由映射 | `backend/router/apps/edge_sync.go` | 在 `api/v1/edge/nodes` 路由组下挂载 6 条端点 |
| 前端 API | `frontend/src/service/api/edge-node.ts` | 导出证书与升级/回滚完整 API 请求函数与强类型定义 |
| 前端界面 | `frontend/src/views/management/edge-nodes/index.vue` | 增强操作列；挂载证书管理模态框（支持私钥一次性复制与吊销）与版本升级/回滚抽屉（支持历史流水与一键安全回滚） |
| 国际化 | `frontend/src/locales/langs/{zh-cn,en-us}/page.json` | 补齐 `page.edgeNodes.*` 多语言字典 |
| 契约测试 | `automation_tests/tests/48_edge_node_ops.test.js` | 13 项自动化契约测试，覆盖正常、失败、越权、幂等、轮换与回滚路径 |

---

## 2. 自动化测试执行结果

### 2.1 48 组专项契约测试
```shell
$env:NO_PROXY="127.0.0.1,localhost"
npx mocha tests/48_edge_node_ops.test.js
```
输出：
```text
  Edge node operations [48_edge_node_ops]
    Part 1: Edge Node X.509 Certificate Lifecycle
      √ issues a new X.509 certificate for the registered edge node
      √ queries active certificate details without private key (masked)
      √ rejects issuing certificate for an unregistered edge node
      √ rejects cross-tenant certificate query and issuance
      √ rotates certificate: issuing a new certificate revokes the old one
      √ revokes the edge node certificate manually
    Part 2: Edge Node Remote Upgrade and Rollback
      √ rejects upgrade with invalid target version format
      √ rejects downgrade attempt via upgrade endpoint (must strictly be newer)
      √ rejects upgrade to the identical current version
      √ successfully upgrades edge node to a newer version and records history
      √ queries edge node upgrade history list
      √ rejects cross-tenant rollback or upgrade attempt
      √ successfully rolls back edge node to previous version based on history record

  13 passing (303ms)
```

### 2.2 联合回归测试（43 ~ 48 组全量）
```shell
npx mocha tests/43_alarm_comment.test.js tests/44_alarm_assignment.test.js tests/45_template_upgrade_rollback.test.js tests/46_entity_relations.test.js tests/47_ota_gray_governance.test.js tests/48_edge_node_ops.test.js
```
输出：
```text
  85 passing (17s)
  0 failing
```

---

## 3. 运行期语义与设计约束（契约事实）

1. **证书私钥仅此一次暴露**：
   - 私钥在 `POST /edge/nodes/:node_id/certificate` 响应体中作为 `private_key` 一次性返回；
   - 数据库 `edge_node_certificates` 表与查询端点 `GET /edge/nodes/:node_id/certificate` 严格脱敏，不存也不出私钥；
   - 再次调用签发接口将触发**安全轮换**，原有处于 `active` 状态的证书被更新为 `revoked`（`revoked_at` 记录为当前时间戳）。
2. **升级版本单向严格递增**：
   - 升级端点 `POST /edge/nodes/:node_id/upgrade` 强制使用 `compareVersionSegments` 校验目标版本严格高于节点当前版本；
   - 尝试升级到相等版本或低于当前版本的请求直接拒绝（返回 `100002` 并提示降级需走回滚通道）。
3. **回滚基于历史溯源与双向留痕**：
   - 回滚端点 `POST /edge/nodes/:node_id/rollback` 传入 `history_id`，系统提取历史的 `from_version` 作为回滚目标；
   - 回滚操作不会物理删除任何升级历史，而是追加一条 `status=rolled_back` 的审计流水，并将节点版本恢复至旧版本。
4. **多租户强隔离**：
   - 租户 ID 强制来自请求上下文 `claims.TenantID`；
   - 跨租户查询/签发证书、跨租户升级/回滚均收敛为 `100002 edge node not registered` 或对应越权拒绝，绝对不泄露跨租户状态。
