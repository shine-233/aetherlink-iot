# 产品路线图（Roadmap）

> 对标 ThingsBoard 4.3 CE 与 ThingsPanel 最新版，按竞争力差距排序。
> 每个阶段完成后在下方追加实际交付记录。

## 竞品核对结论（2026-08-25，依据双方官网/GitHub）

- ThingsPanel **社区版已自带 Modbus TCP/RTU** 接入与规则引擎（数据转发/实时计算）；其 **OTA 在社区版缺失**（企业版功能）。移动端 App 社区版可用（uniapp）。
- ThingsBoard 的边缘能力有开源的 **ThingsBoard Edge CE**，并非纯付费功能；OPC UA 可经其开源 IoT Gateway 接入。
- 双方的白标定制、行业模板均为付费版功能；TimescaleDB 支持两家社区版均可用。
- 结论：Phase B 优先补 Modbus 与可视化规则链的方向成立；"设备影子"作为差异化卖点成立（ThingsBoard 无同名一等能力，最接近的是共享属性+RPC 队列组合）。

---

## 竞品对标矩阵

图例：✓ 完整可用 · ◐ 部分实现 · ✗ 未实现 · (PE) = ThingsBoard 仅付费版提供

| 功能域 | AetherLink | ThingsBoard CE | ThingsPanel | 说明 |
|---|---|---|---|---|
| 设备管理 | ◐ | ✓ | ✓ | 接入/配置/物模型/共享已齐；资产层级缺失 |
| MQTT 接入 | ◐ | ✓ | ✓ | 自带 GMQTT Broker、ACL、主题映射、持久会话 |
| HTTP 接入 | ◐ | ✓ | ✓ | 设备网关式 HTTP 上行已通 |
| Telemetry + WS | ◐ | ✓ | ✓ | 实时推送、聚合统计、死信队列 |
| 告警系统 | ◐ | ✓ | ✓ | 规则/历史/通知组；多级严重度 |
| OTA | ◐ | ✓ | ✓ | 整包升级任务与进度跟踪 |
| 多租户 RBAC | ◐ | ✓ | ✓ | Casbin + 租户隔离 + 空租户 fail-closed |
| 自动化场景 | ◐ | ✓ | ✓ | 联动/定时/条件编辑 |
| 看板 | ◐ | ✓ | ✓ | 原生看板 + 发布分享 |
| 脚本引擎 | ◐ | ✓ | ✓ | 数据处理脚本 |
| 通知服务 | ◐ | ✓ | ✓ | 邮件/Webhook 通知组 |
| 可视化规则链编辑器 | ✓* | ✓ | ◐ | B2 分支 `origin/feat/rule-chain-b2` 就绪（DAG 引擎+Vue Flow 画布），待合并 |
| API 限流(per-tenant) | ✓ 单机 | ✓ (PE) | ✗ | 默认 600 rpm/租户（env 可调），429+Retry-After；集群 Redis 版待 C 阶段 |
| 3D 可视化/SCADA | ✓ | ✓ | ◐ | C3 已落地：设备详情「3D 预览」tab，遥测驱动材质、WebGL 降级 |
| Modbus | ✓* | ◐ | ✓ | B1 分支 `feat/modbus-b1` 就绪（独立插件+点表 UI），待合并 |
| 设备影子 | ✓ | ✓ | ✓ | Phase A3 已交付：离线命令缓存+上线投递 |
| 资产管理层级 | ✗ | ✓ | ✗ | Phase C2 |
| CoAP / LwM2M / SNMP | ✗ | ✓ | ✗ | Phase C6 |
| OPC UA | ✗ | ✗ | ✓ | 远期评估 |
| 移动端 App | ✗ | ✗ | ✓ | 远期评估 |
| AI / LLM 集成 | ✓ | ✗ | ✓ | C4 已落地 NL 查询遥测；AI 告警分析待做 |
| 计算字段 | ✓* | ✓ | ✗ | B3 分支 `calcfield-lane` 就绪（53.sql+govaluate 安全表达式），待合并 |
| TimescaleDB / TDengine | ✗ | ✓ | ✓ | Phase C1 |
| 白标定制 | ✗ | ✓ (PE) | ✗ | Phase C5 |
| 行业模板 | ✗ | ✓ (PE) | ✗ | 远期 |
| 边缘计算 | ✗ | ✓ (PE) | ✗ | 远期 |

结论：核心设备管理链路已接近主流水平。Modbus（B1）、规则链可视化（B2）、计算字段（B3）三条 lane 已完成实现待合并；合并后剩余差距收敛为「时序存储后端（C1 TimescaleDB）、资产层级（C2）、CoAP/LwM2M（C6）、白标（C5）」四项远期线。

---

## Phase A — 短期（1-2 个迭代）

### A1. 空租户守卫移植到所有 raw 链
- [x] `dal/users.go` GetUserListByPageWithAddress — 已有守卫 ✓
- [x] `dal/alarm.go` GetAlarmConfigListByPage / GetAlarmInfoListByPage — 空租户且未显式授权全租户视角 → 拒绝 ✓
- [x] `dal/alarm.go` GetAlarmHistoryListByPage — 同上 ✓
- [x] `dal/device_config.go` GetDeviceConfigListByPage — claims 空租户拒绝 ✓
- [x] 其余 raw 链排查：board/ui_elements/ota/fleet 配额均已带守卫或显式约定 ✓

每处约 +5 行：检查 `claims.TenantID == ""` 时返回错误或空结果，防止跨租户泄漏。

### A2. message_push gen LeftJoin raw 化
- [x] `dal/message_push.go` GetUserMessagePushId — 已完成 ✓

### A3. 设备影子（离线命令缓存）
核心差异化功能：设备离线时下发命令不再失败，改为缓存；设备重新上线后自动投递。

设计要点：
```
┌─────────┐    命令下发     ┌──────────────┐
│ 用户/API │ ──────────────→│ 设备在线？    │
└─────────┘                │  Y → 直接下发 │
                           │  N → 写入影子 │
                           └──────┬───────┘
                                  │
                    设备上线事件触发
                                  │
                           ┌──────▼───────┐
                           │ 投递缓存命令  │──→ 设备 ACK → 标记已投递
                           └──────────────┘         │ 超时/过期 → 标记过期
```

新增表 `device_shadow_messages`：
```sql
CREATE TABLE device_shadow_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id VARCHAR(36) NOT NULL REFERENCES devices(id),
    message_type VARCHAR(20) NOT NULL DEFAULT 'command', -- command | property | notification
    payload JSONB NOT NULL,
    ttl_seconds INT NOT NULL DEFAULT 86400,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',       -- pending | delivered | expired | canceled
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_dsm_device_pending ON device_shadow_messages(device_id, status) WHERE status = 'pending';
```

实现步骤：
- [x] 新增迁移 `52.sql`：建表 + pending/expiry 部分索引 ✓
- [x] 新增 `dal/device_shadow.go`：CRUD + 待投递查询 + 全状态列表/计数 + 到期标记 + 7 天终态清理（时间参数化，PG/sqlite 双兼容）✓
- [x] 新增 `service/device_shadow.go` + `api/device_shadow.go`：设置（在线直发/离线缓存）、列表、取消 API，设备读写访问守卫 ✓
- [x] 路由：`GET/POST /api/v1/device/shadow/:deviceId`、`DELETE /api/v1/device/shadow/:deviceId/:msgId` ✓
- [x] 上线投递钩子：uplink 首条消息路径 + status_flow 状态切换路径，延时 3s 投递 pending ✓
- [x] cron：每 30 分钟到期标记 + 终态清理 ✓
- [x] DAL 测试：生命周期/过期排除/取消/清理（sqlite 内存库）✓
- [x] 前端：设备详情页新增"影子队列"标签页（状态筛选/新建/取消）✓
- [ ] E2E：离线→上线投递、TTL 过期、取消

### A4. 空态覆盖率提升
- [x] 6 个列表视图补 n-empty ✓（PR #134）
- [ ] 继续扫描剩余 ~230 个视图文件中缺失空态的列表组件

---

## Phase B — 中期（3-5 个迭代）

### B1. Modbus TCP 插件
参考 ThingsPanel modbus-protocol-plugin 架构，Go 独立进程通过 gRPC 与主平台对接。

架构：
```
AetherLink Backend ←──gRPC──→ Modbus TCP Plugin ←──Modbus TCP──→ PLC/RTU
     │                            │
     └── 设备注册/配置 API          └── 寄存器映射配置（保持寄存器/输入寄存器/线圈）
```

实现步骤：
- [ ] 定义协议插件 gRPC 接口 proto 文件（连接管理、数据采集、命令下发）
- [ ] 实现 Modbus TCP 连接池与轮询采集器
- [ ] 实现寄存器地址映射配置（JSON/YAML）
- [ ] 数据上报走现有 telemetry uplink 管道
- [ ] 前端：Modbus 寄存器映射配置界面
- [ ] 打包为 Docker 容器，通过 docker-compose 可选启用

参考仓库：
- https://github.com/ThingsPanel/modbus-protocol-plugin
- https://github.com/grid-x/modbus（Go Modbus 库）

### B2. 可视化规则链编辑器
基于 Vue Flow（Vue 版 ReactFlow）实现拖拽式节点连线。

节点类型：
- 触发器节点：遥测到达 / 属性变更 / 设备上线 / 定时器
- 过滤节点：阈值比较 / 正则匹配 / JSON Path 提取
- 转换节点：字段映射 / 脚本计算 / 单位换算
- 动作节点：告警 / 控制设备 / 发送通知 / Webhook

实现步骤：
- [ ] 安装 `@vue-flow/core` 依赖
- [ ] 定义规则链数据模型（DAG 有向无环图）
- [ ] 后端 CRUD API + 执行引擎（拓扑排序遍历 DAG）
- [ ] 前端画布组件（拖拽节点、连线、属性面板）
- [ ] 内置节点类型注册机制
- [ ] 与现有 scene automation 共存（规则链是更底层的通用能力）

### B3. 计算字段（Calculated Fields）
在遥测写入管道中增加表达式计算步骤。

```sql
ALTER TABLE device_calculated_fields (
    id UUID PRIMARY KEY,
    device_config_id VARCHAR(36),
    output_key VARCHAR(64),        -- 输出字段名如 'power'
    expression TEXT,               -- 表达式如 '{voltage} * {current}'
    enabled BOOLEAN DEFAULT true
);
```

- [ ] 迁移 SQL
- [ ] 表达式求值器（复用 script-engine 或引入 expr-lang/expr）
- [ ] 遥测管道钩子：原始遥测到达后查关联计算字段 → 计算 → 合并进输出
- [ ] 前端：计算字段配置界面

### B4. 空态覆盖率继续扩展
- [ ] 扫描剩余 ~220 个视图文件，补齐缺失的 n-empty
- 目标覆盖率 ≥ 50%

---

## Phase C — 远期

### C1. TimescaleDB 可选存储后端
- [ ] 时序表迁移为 hypertable
- [ ] 配置开关切换普通 PG vs TimescaleDB
- [ ] 压缩策略自动配置

### C2. 租户客户层级
- [ ] tenants 表增加 parent_tenant_id 自引用
- [ ] RBAC 支持层级继承
- [ ] 数据隔离按层级级联过滤

### C3. TresJS 3D 可视化面板
- [ ] 安装 @tresjs/core + @tresjs/cientos + three
- [ ] 设备详情页"3D 视图"标签页
- [ ] GLB 模型加载 + 遥测数据驱动材质颜色/旋转
- [ ] WebGL 不支持时降级为 2D

### C4. AI 集成
- [x] 自然语言查询遥测数据（MVP 已落地）：`POST /api/v1/ai/telemetry/query`——LLM 仅解析结构化意图（设备/字段/时间范围），取数走白名单 DAL 并强制租户过滤；OpenAI 兼容端点，`ai.llm.api_key/base_url/model` 配置，未配置时显式报错 ✓
- [ ] AI 告警分析（异常模式识别+根因建议）

### C5. 白标定制
- [ ] 租户级 logo/favicon/主题色配置
- [ ] 登录页自定义

### C6. CoAP / LwM2M 协议支持
- [ ] CoAP 服务端（Go get RFC7252 库）
- [ ] LwM2M 客户端注册/上报流程

---

## 交付记录

| 日期 | 阶段 | 交付内容 | PR |
|---|---|---|---|
| 2026-08-26 | B4+ | per-tenant API 限流中间件（默认 600rpm，429+Retry-After，6 用例）；C3 3D 预览 tab 正式接线（遥测驱动+懒加载+i18n×4）；response 中间件契约测试 ×8 | #177 |
| 2026-08-26 | P1 | 安全加固批：LIKE 通配转义、刷新令牌吊销、IP 维度登录防爆破、Casbin 路由覆盖审计、DAL 租户强制删除、doctor 公网明文 MQTT 门禁 | #176 |
| 2026-08-25 | A1 | 空租户守卫：alarm 配置/信息/历史列表 + device_config 列表 fail-closed（含 all-tenants 显式授权与回归测试） | #155 |
| 2026-08-24 | A2 | message_push gen LeftJoin raw 化 | 已并入 main |
| 2026-08-25 | A3 | 设备影子全链路：迁移 52.sql、DAL/Service/API/路由、上线投递钩子、cron 清理、DAL 测试、前端影子队列标签页 | 待提 PR |
| 2026-08-25 | C4 | AI 自然语言查询遥测 MVP：意图抽取式 NL 查询服务 + `/ai/telemetry/query` 端点 + 单测 | 待提 PR |
| 2026-08-23/24 | — | users 列表 raw 链收敛 + 空租户守卫；alarm raw 链 P1 修复 | VALIDATION.md |
| 2026-08-25 | 质量 | 全库 GBK 乱码修复（17 文件，含 echarts-manager 被困代码释放）+ 源码编码契约测试绊线 + 影子离线投递 method/params 语义修复 | feat/phase-a-completion |
