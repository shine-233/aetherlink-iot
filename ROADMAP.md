# 功能路线图（Roadmap）

> 基于 ThingsBoard CE/PE 与 ThingsPanel 最新功能对比分析，结合 AetherLink IoT 现有架构设计的分阶段演进计划。
> 每阶段完成后在 VALIDATION.md 中记录验证结果。

---

## 当前功能覆盖度

| 能力 | AetherLink | ThingsBoard(CE) | ThingsPanel |
|---|---|---|---|
| 设备管理 | ████████ | ██████████ | ████████ |
| MQTT 接入 | ████████ | ██████████ | ████████ |
| HTTP 接入 | ████████ | ██████████ | ████████ |
| Telemetry + WS | ████████ | ██████████ | ████████ |
| 告警系统 | ████████ | ██████████ | ██████████ |
| OTA | ████████ | ██████████ | ████████ |
| 多租户 RBAC | ████████ | ██████████ | ██████████ |
| 自动化场景 | ███████ | ██████████ | █████████ |
| 看板 | ███████ | ██████████ | █████████ |
| 脚本引擎 | ███████ | ██████████ | ███████ |
| 通知服务 | ███████ | ██████████ | ████████ |
| Modbus | ░░░░░░░░ | ████████ | █████████ |
| 可视化规则链编辑器 | ███░░░░░ | ██████████ | ███████ |
| 设备影子 | ░░░░░░░░ | █████████ | █████████ |
| 资产管理层级 | ░░░░░░░░ | █████████ | ░░░░░░░░ |
| CoAP/LwM2M/SNMP | ░░░░░░░░ | █████████ | ░░░░░░░░ |
| OPC UA | ░░░░░░░░ | ░░░░░░░░ | █████████ |
| 3D/SCADA | ██░░░░░░ | ████████ | █████████ |
| 移动端 App | ░░░░░░░░ | ░░░░░░░░ | █████████ |
| AI/LLM 集成 | ░░░░░░░░ | ░░░░░░░░ | ████████ |
| TimescaleDB/TDengine | ░░░░░░░░ | █████████ | █████████ |
| 白标定制 | ░░░░░░░░ | █████████(PE) | ░░░░░░░░ |
| 行业模板 | ░░░░░░░░ | █████████(PE) | ░░░░░░░░ |
| API 限流(per-tenant) | ███░░░░░ | █████████(PE) | ░░░░░░░░ |
| 边缘计算 | ░░░░░░░░ | █████████(PE) | ░░░░░░░░ |

---

## Phase A — 短期 · 安全收口 + 核心差异化（1–2 个迭代）

### A1. 空 TenantID 守卫扩展到所有 raw 链

**状态**：进行中

**背景**：PR #105 发现用户列表查询在 `claims.TenantID` 为空时返回全量数据。#126 已修复 users.go 列表，但其他 raw 链尚未加守卫。

**涉及文件**：
- `dal/alarm.go` — GetAlarmConfigListByPage / GetAlarmInfoListByPage
- `dal/device_config.go` — GetDeviceConfigListByPage
- `dal/device_query_reads.go` — GetDeviceTemplateChartSelect
- `dal/open_api_keys.go` — GetOpenAPIKeyListByPage
- `dal/operation_logs.go` — GetListByPage
- `dal/message_push.go` — GetUserMessagePushId

**改动模式**：在每个使用 `claims.TenantID` 的 raw 链函数入口添加：
```go
if strings.TrimSpace(tenantID) == "" {
    return 0, nil, fmt.Errorf("empty tenant id in claims")
}
```

### A2. message_push gen LeftJoin raw 化

**状态**：✅ 已完成（#130）

### A3. 设备影子（Device Shadow / 离线命令缓存）

**状态**：设计中

**痛点**：设备离线时下发命令会失败且无缓存重试机制。ThingsBoard 和 ThingsPanel 都支持离线命令缓存，设备重新上线后自动下发。

**设计**：
```
┌─────────────┐     command      ┌──────────────┐
│   平台       │ ──────────────→ │   设备在线    │ ─→ 直接下发
│             │                  └──────────────┘
│             │     command      ┌──────────────┐
│             │ ──────────────→ │   设备离线    │ ─→ 写入 device_shadows
│             │                  └──────────────┘         │
│             │                                    设备上线时
│             │                                    ← 重放 pending 命令
└─────────────┘
```

**数据库表**（新增迁移 50.sql）：
```sql
CREATE TABLE IF NOT EXISTS device_shadows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id VARCHAR(36) NOT NULL REFERENCES devices(id),
    shadow_type VARCHAR(20) NOT NULL DEFAULT 'command', -- command | property
    payload JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',      -- pending | delivered | expired
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_device_shadows_device_status ON device_shadows(device_id, status) WHERE status = 'pending';
```

**后端 API**：
- `GET /api/v1/device/{id}/shadow` — 查询设备影子状态
- `PUT /api/v1/device/{id}/shadow` — 更新期望状态（desired）
- `DELETE /api/v1/device/{id}/shadow/{shadowId}` — 删除待下发命令
- Broker OnConnected 钩子触发重放逻辑

**前端**：
- 设备详情页新增"影子状态"标签页，展示 pending/delivered 命令列表

### A4. 空 TenantID 守卫 — DAL 测试补充

为每个新增守卫编写对应单元测试，验证空租户时返回空结果而非报错。

---

## Phase B — 中期 · 协议扩展 + 规则引擎升级（3–5 个迭代）

### B1. Modbus TCP 插件

参考 ThingsPanel `modbus-protocol-plugin` 架构：Go 独立进程通过 gRPC 与主平台对接。

**架构**：
```
┌──────────────┐     gRPC      ┌───────────────────┐     Modbus TCP      ┌────────┐
│  AetherLink   │ ◄──────────→ │  modbus-plugin     │ ──────────────────→ │  PLC   │
│  (MQTT Broker │               │  (独立 Go 进程)     │                     │ 变频器  │
│   + Backend)  │               └───────────────────┘                     └────────┘
└──────────────┘
```

**插件接口**：
- 订阅 `devices/modbus/{device_id}/command` 接收平台命令
- 发布 `devices/telemetry` 上报采集数据
- 配置文件定义寄存器映射（地址/功能码/数据类型/缩放因子）

**交付物**：
- `cmd/modbus-plugin/main.go` — 独立进程入口
- `internal/modbus/client.go` — Modbus TCP 客户端封装
- `configs/modbus-mapping.yaml` — 寄存器映射配置模板
- Docker Compose profile `modbus` 加入编排

### B2. 可视化规则链编辑器

基于 Vue Flow（Vue 版 ReactFlow）实现拖拽式节点连线。

**节点类型**：
| 类型 | 说明 | 对应现有能力 |
|---|---|---|
| Trigger | 设备遥测/属性/事件/生命周期 | scene automation trigger |
| Filter | 条件过滤（阈值/范围/表达式） | scene condition |
| Transform | 数据转换（脚本/JSONPath） | script engine |
| Enrichment | 元数据丰富（查设备详情/资产） | telemetry enrich |
| Action | 下发命令/告警/通知/webhook | notification + command |

**前端组件**：
- 基于 `@vue-flow/core` 实现画布
- 左侧节点面板拖拽 → 画布放置 → 连线
- 节点属性面板（右侧抽屉）
- 保存/加载 JSON 规则链定义

**后端**：
- 新增 `rule_chains` 表存储链定义（JSONB nodes + edges）
- 执行引擎复用现有 uplink pipeline 的分发点

### B3. 计算字段（Calculated Fields）

在遥测写入管道中增加表达式计算步骤。

**场景**：从 V 和 I 计算 P = V × I；从 CO2 和温度计算 AQI 等。

**实现**：
- 在 `storage/telemetry_writer.go` 批量写入前插入计算步骤
- 表达式使用现有脚本引擎（sandbox.ts 后端等价物，Go eval 或 expr-lang）
- 规则存储在 device_config 或独立表中

### B4. OpenAPI Key 吊销广播 + 负缓存 + 限流

**状态**：✅ 已完成（fix/five-action-items 分支）

---

## Phase C — 远期 · 差异化竞争力（规划中）

### C1. TimescaleDB 可选存储后端
- 遥测历史表迁移到 TimescaleDB hypertable
- 自动分区 + 压缩策略
- 通过配置开关切换 PostgreSQL / TimescaleDB

### C2. 租户客户层级
- Tenant → Customer → Sub-Customer 三级层级
- 支持客户下子客户分组管理设备
- RBAC 沿层级继承

### C3. TresJS 3D 可视化面板
- 设备详情页新增 "3D 视图" 页签
- TresCanvas + useGLTF 加载设备模型（glb）
- 实时遥测驱动材质颜色/旋转/数据标签
- WebGL 不可用时回落 2D 视图

### C4. AI 集成（自然语言查询遥测数据）
- MCP 协议接入 DeepSeek/Qwen/ChatGLM
- 自然语言 → 结构化查询 → 返回图表
- AI 辅助告警分析和根因定位

### C5. 白标定制（White-labeling）
- 每租户自定义 logo / favicon / 登录页标题 / 主题色
- 存储在租户配置表中

### C6. Edge 边缘计算节点
- 离线环境下本地运行规则链
- 恢复连接后同步数据和状态到云端

---

## 不做的事情（明确排除）

| 项目 | 原因 |
|---|---|
| LoRaWAN Network Server | 通过 ChirpStack/The Things Stack 外部集成，不做原生 LNS |
| Kubernetes Operator | 社区需求不足，Docker Compose 已覆盖绝大多数部署场景 |
| 微服务拆分 | 单体+模块化已满足目标规模（万级设备），拆分增加运维复杂度 |
| GraphQL API | REST + WebSocket 已够用，GraphQL 增加 learning curve 但收益有限 |
