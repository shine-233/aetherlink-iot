# 产品路线图（Roadmap）

> 对标 ThingsBoard 4.3 CE 与 ThingsPanel 最新版，按竞争力差距排序。
> 每个阶段完成后在下方追加实际交付记录。

---

## Phase A — 短期（1-2 个迭代）

### A1. 空租户守卫移植到所有 raw 链
- [x] `dal/users.go` GetUserListByPageWithAddress — 已有守卫 ✓
- [ ] `dal/alarm.go` GetAlarmConfigListByPage / GetAlarmInfoListByPage — 加空租户拒绝
- [ ] `dal/alarm.go` GetAlarmHistoryListByPage — 同上
- [ ] `dal/device_config.go` GetDeviceConfigListByPage — 同上
- [ ] 其余含 `claims.TenantID` 过滤的 raw 链函数逐一排查

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
- [ ] 新增迁移 `50.sql`：建表 + 索引
- [ ] 新增 `dal/device_shadow.go`：CRUD + 查询待投递列表 + 过期清理
- [ ] 新增 `service/device_shadow.go`：设置/查询/取消影子消息 API
- [ ] 修改命令下发路径：设备离线时写入影子而非报错
- [ ] 修改设备上线路径（telemetry uplink 首条消息）：查询并投递 pending 影子
- [ ] 前端：设备详情页新增"影子消息"标签页（查看/编辑/删除待发消息）
- [ ] 测试：离线→上线投递、TTL 过期、取消

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
- [ ] 自然语言查询遥测数据（NL→SQL 或 NL→API 调用）
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
| | | | |
