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
| 可视化规则链编辑器 | ✓ | ✓ | ◐ | B2 已合入：DAG 引擎+Vue Flow 画布（56.sql graph 列） |
| API 限流(per-tenant) | ✓ 单机 | ✓ (PE) | ✗ | 默认 600 rpm/租户（env 可调），429+Retry-After；集群 Redis 版待 C 阶段 |
| 3D 可视化/SCADA | ✓ | ✓ | ◐ | C3 已落地：设备详情「3D 预览」tab，遥测驱动材质、WebGL 降级 |
| Modbus | ✓ | ◐ | ✓ | B1 已合入：独立插件+点表 UI（55.sql + modbus-plugin） |
| 设备影子 | ✓ | ✓ | ✓ | Phase A3 已交付：离线命令缓存+上线投递 |
| CSV 批量导入设备 | ✓ | ✓ | ✓ | 后端建档/导出与前端导入向导均已落地（自动生成/CSV 双模式、一次性凭证、脱敏导出）；产品选择列表 `/product` 同步补齐 |
| 资产管理层级 | ◐ | ✓ | ✗ | C2：60.sql + CRUD/树 API 已入；RBAC/DAL 级联待接 |
| CoAP / LwM2M / SNMP | ◐ | ✓ | ✗ | C6 闭环：UDP 网关 + 端点凭证映射 + 遥测汇入 uplink（P1-C，2026-09-04）；SNMP/OPC UA 库级，待管理侧接入 |
| OPC UA | ◐ | ✗ | ✓ | 库级 client（gopcua）已入；连接器/点位接入待立项 |
| 移动端 App | ✗ | ✗ | ✓ | 远期评估 |
| AI / LLM 集成 | ✓ | ✗ | ✓ | C4 已落地 NL 查询遥测；AI 告警分析待做 |
| 计算字段 | ✓ | ✓ | ✗ | B3 已合入：govaluate 安全表达式派生遥测（54.sql） |
| TimescaleDB / TDengine | ◐ | ✓ | ✓ | C1 部分落地：57.sql 条件化 hypertable + 7 天压缩（检测扩展自动启用） |
| 白标定制 | ◐ | ✓ (PE) | ✗ | C5：主题色/favicon 已入主线（59.sql）；运行期验证待做 |
| 行业模板 | ✗ | ✓ (PE) | ✗ | 远期 |
| 边缘计算 | ✗ | ✓ (PE) | ✗ | 远期 |

结论：核心设备管理链路已接近主流水平。Modbus（B1）、规则链可视化（B2）、计算字段（B3）、CSV 批量导入（fleet）已合入；剩余差距收敛为「时序存储后端（C1 TimescaleDB）、资产层级（C2）、CoAP/LwM2M（C6）、白标（C5）」四项远期线。

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
- [x] E2E spec 已编写（`automation_tests/pending-e2e/shadow-offline-delivery.spec.js`，标识 `AETHERLINK_E2E_A3_SHADOW`）；执行需真实栈 broker 设备 ACK 路径（纳入 pending-e2e，接入时同步 e2e 元数据契约）

### A4. 空态覆盖率提升
- [x] 6 个列表视图补 n-empty ✓（PR #134）
- [x] 空态审计工具化：`frontend/scripts/audit-empty-states.mjs`（只读扫描 + JSON 报告）；基线 listy=146、缺失=56、覆盖率 61.6%（≥50% 目标达成）。剩余 56 个 gap 见 `frontend/scripts/empty-state-audit-report.json`，逐条人工补 n-empty（自动批量补丁需人工复核避免误伤）

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
- [x] 新增独立进程模块 `modbus-plugin/`：grid-x/modbus 封装（按目标串行事务 + 懒重连）、轮询采集器、命令下行订阅 ✓
- [x] 寄存器地址映射点表（JSON）：holding/input/coil/discrete × u16/i16/u32/i32/f32 大端解码，multiplier/offset 缩放，`writable` 标记可写点 ✓
- [x] 数据上报走现有 telemetry uplink 管道：以设备自身 MQTT 凭证发布 `{key: value}` 到 `devices/telemetry`，设备归属由 broker 认证绑定 ✓
- [x] 命令下发：订阅 `devices/command/{number}/+`，按 `{identify, value}` 匹配可写寄存器执行 FC5/FC6/FC16 写入 ✓
- [x] Dockerfile + docker-compose 可选 profile（`--profile modbus`），点表含凭证只读挂载 ✓
- [x] 单元测试：内嵌 MBAP 从站闭环验证 FC3/FC4 读值缩放、f32 写读回环、只读拒绝 ✓
- [x] 决策记录（保留）：自定义 gRPC 插件接口暂缓——当前平台插件运行时为 HTTP 边界（pluginruntime）、数据面为 MQTT；待平台暴露插件 gRPC 网关后再评估，避免引入无消费方的私有通道
- [x] 前端：设备详情页「Modbus 点表」标签页（从站目标表单 + 点位行内编辑表格），保存后由插件经 OpenAPI Key 拉取生效 ✓

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
- [x] 安装 `@vue-flow/core` 依赖 ✓
- [x] 定义规则链数据模型（DAG 有向无环图）：迁移 `54.sql` 建表，graph={nodes:[{id,type,config}],edges} JSONB；校验含类型注册表、边完整性、触发器存在、Kahn 无环 ✓
- [x] 后端 CRUD API + 执行引擎：`/rule-chains` 租户内 CRUD（空租户 fail-closed）；引擎从零入度触发节点拓扑遍历，过滤剪枝分支、映射重塑载荷；60s 启用链缓存随写失效；遥测副作用与设备上线两处异步钩子接入 ✓
- [x] 内置节点类型注册机制：trigger.telemetry / trigger.device_online / filter.threshold / transform.mapping / action.webhook / action.command / **action.alarm（2026-09-02 补齐：engine + 前端画布 + 落库注入点 + 单测）** ✓
- [x] 前端画布组件：拖拽节点面板、连线定序、点击节点属性面板（阈值/映射/Webhook/命令表单）、列表页启用开关与删除确认；路由+四语言 i18n ✓
- [x] 与现有 scene automation 共存：独立 `/automation/rule-chain` 页面与场景联动并列，规则链作为更底层的通用能力由上行管道直接驱动 ✓

### B3. 计算字段（Calculated Fields）
在遥测写入管道中增加表达式计算步骤。

```sql
-- 落地形态见 51.sql/54.sql：重建 device_calculated_fields（tenant+device_template 维度），
-- 输出键/表达式/启用开关，安全表达式求值见 internal/calcfield（govaluate）。
```

- [x] 迁移 SQL（51.sql 建表 + 54.sql 重建到 tenant+device_template 维度）✓
- [x] 表达式求值器：`internal/calcfield`（govaluate 安全表达式派生遥测）✓
- [x] 遥测管道钩子：原始遥测到达后查关联计算字段 → 计算 → 合并进输出 ✓
- [x] 前端：计算字段配置界面 ✓

### B4. 空态覆盖率继续扩展
- [x] 见 A4 工具化扫描：覆盖率 61.6%（≥50%）；剩余 gap 清单随报告逐条补
- 目标覆盖率 ≥ 50%（已达成并持续跟踪）

---

## Phase C — 远期

### C1. TimescaleDB 可选存储后端
- [x] 时序表迁移为 hypertable（57.sql：telemetry_datas + alarm_info，检测扩展存在才执行）
- [x] 配置开关切换普通 PG vs TimescaleDB（timescale_mode=auto/on/off 显式开关，含 fail-fast；运行期三态验证待重建栈执行）
- [x] 压缩策略自动配置（telemetry_datas 7 天压缩 + segmentby device_id）

### C2. 租户客户层级
- [x] tenants 表增加 parent_tenant_id 自引用（60.sql）与资产树 assets（60.sql）
- [x] 资产 CRUD/树 API（dal/service/api/router，Scope=self∪子孙（自上而下：总部/父级可下钻），成环拒绝；assets 垂直切片）——2026-09-03 隔离栈运行期回归 12/12（HQ 见自身+子孙根、child 仅自身、跨租户父/读 404、删除守卫）
- [x] RBAC 继承接缝：`service.InheritedAuthorityRoles`（角色@祖先租户域展开 + 纯函数单测）；说明——现有 Casbin 角色策略为全局角色名，同角色天然跨租户生效，展开为 tenant-qualified 策略预留（配 TENANT_ADMIN/SYS_ADMIN）
- [x] 存量核心模块 DAL 租户过滤 =→IN(Scope) 替换（各模块带真实结果集测试）：assets（原生 Scope）→ device → alarm(config/info/history) → board；模式统一为 ForScopes 变体 + service 层 expandTenantIDScope，后续新模块沿用同模式

### C3. TresJS 3D 可视化面板
- [x] 安装 @tresjs/core + @tresjs/cientos + three（vue-tsc 全绿，组件按需加载不进首屏 bundle）
- [x] 设备详情页「3D 视图」标签页（见交付记录 #177 正式接线）
- [x] GLB 模型加载 + 遥测数据驱动材质颜色/旋转
- [x] WebGL 不支持时降级为 2D

### C4. AI 集成
- [x] 自然语言查询遥测数据（MVP 已落地）：`POST /api/v1/ai/telemetry/query`——LLM 仅解析结构化意图（设备/字段/时间范围），取数走白名单 DAL 并强制租户过滤；OpenAI 兼容端点，`ai.llm.api_key/base_url/model` 配置，未配置时显式报错 ✓
- [x] AI 告警分析（异常模式识别+根因建议）：`POST /api/v1/ai/alarm/analysis` + service 单测 4/4（跨租户/未配置 LLM 显式报错）；前端告警详情「AI 分析」入口与运行期 LLM 配置验证见 WORKPLAN P1-D

### C5. 白标定制
- [x] 租户级 logo/favicon/主题色配置（59.sql + logo API + CSS 变量 + branding 表单，已并入单一主线）
- [x] 运行期全流程验证（登录页/主题/favicon 两租户互不串扰）与前端表单回归 ✓（2026-09-04：c5_validate.py v4 **28/28 × 2 轮**隔离栈实跑、两租户互不串扰；前端 branding-setting vitest **11/11 PASS**）

### C6. CoAP / LwM2M 协议支持
- [x] CoAP 服务端（RFC7252 子集：编解码/UDP 服务器/注册表/well-known + blockwise/observe 组件）
- [x] LwM2M 注册层 + 对象实例模型 + 观察者推送（UDP 服务器配置门控接入 application 生命周期）
- [x] 设备凭证映射 + 遥测汇入现有 uplink 管道（网关设备接入闭环，WORKPLAN P1-C）：lwm2m.ObjectStore.OnChange 写入回调 + /rd HandleRegisterWithNotify 注册通知；protocolgw.TelemetryBridge（DBNumberResolver 端点→device_number 凭证映射[60s TTL 缓存/fail-closed]、IPSO 键转换 3303/3304/3323/3325/3330 + lwm2m/{o}/{i}/{r} 回退、队列化汇入不阻塞写路径）→ 与 MQTT 同一 UplinkMessage/uplink.Bus（source_protocol=coap）；app 装配 DB/Bus 未就绪自动降级纯接入层（注：uplink.enable=false 时 WithFlowService 早退不建 uplinkService，网关仅纯接入——生产启用遥测汇入须同时开 uplink）；单测 +14（protocolgw+lwm2m 合计 31 PASS）；**运行期 E2E（2026-09-04，隔离栈真实 UDP）：外部客户端注册 2.01 / 写资源 2.04 / 读回 2.05 三步全过，遥测经凭证映射汇入 uplink 管道并按租户落库 telemetry_datas（temperature=26.5，tenant_id 正确），测后种子已回滚**；已知限制：coap 子集 codec 的 option 值仅支持内联 ≤12 字节（扩展编码 13/14 拒绝），超长端点名（如 urn:imei:xxx）需先行 codec 升级 ✓

### C7. 安全与平台能力补齐（对标 ThingsBoard CE 免费能力，2026-08-25 审查补录）
- [x] 双因素认证 2FA（61.sql 后端 + 前端：登录两段式动态码页、个人中心绑定/停用/恢复码组件）
- [x] OAuth2/OIDC 单点登录后端（62.sql：租户 IdP 配置 CRUD、/sso/:id/start|callback、sessionIssuer 绑定本地用户）；登录页 SSO 入口待接
- [x] 实体版本控制（设备/看板/规则链等实体 JSON 导出 + Git 式备份恢复，对标 TB 3.5+ Version Control）
- [x] OIDC 登录页 SSO 入口已实现：`GET /api/v1/sso/providers`（平台级启用提供方公开发现）+ 登录页 SSO 按钮（跳 /sso/:id/start）
- [ ] 真实 IdP E2E（需外部 IdP 沙箱 + 重建栈，环境绑定；2026-09-04 复核：执行沙箱无外网路径——代理端口 7890/1089/1891 全部不通、直连 TLS 被拦，维持绑定待用户环境执行）

---

## 交付记录

| 日期 | 阶段 | 交付内容 | PR |
|---|---|---|---|
| 2026-08-26 | B4+ | per-tenant API 限流中间件（默认 600rpm，429+Retry-After，6 用例）；C3 3D 预览 tab 正式接线（遥测驱动+懒加载+i18n×4）；response 中间件契约测试 ×8 | #177 |
| 2026-08-26 | P1 | 安全加固批：LIKE 通配转义、刷新令牌吊销、IP 维度登录防爆破、Casbin 路由覆盖审计、DAL 租户强制删除、doctor 公网明文 MQTT 门禁 | #176 |
| 2026-08-25 | 安全 P0 | 高德 securityJsCode 去硬编码：改由 VITE_AMAP_SECURITY_CODE 构建期注入（index.html 清除明文密钥，map-sdk 增加 ensureAmapSecurityConfig 契约测试；**旧 key 已公开泄漏，必须在高德控制台轮换**） | sec/p0-guardrails |
| 2026-08-25 | 安全 P0 | JWT 密钥启动 fail-fast：占位符（CHANGE_ME_*）/空值/长度<32 拒绝启动，含修复指引；README 开发调试同步 GOTP_JWT_KEY 说明 | 同上 |
| 2026-08-25 | 安全 P1 | OpenAPI Key 默认权限 TENANT_ADMIN→TENANT_USER 最小降权（compose/.env.example/文档三面同步，需要写能力显式上调） | 同上 |
| 2026-08-25 | 可靠性 P1 | Broker 持久化默认切 Redis：GMQTT_PERSISTENCE_* 环境覆盖层（start/reload 双路径 + fail-fast 校验），Compose 默认 redis、文件默认保持 memory，契约测试锁定；broker 重启不再丢离线会话/QoS 队列 | 同上 |
| 2026-08-25 | 部署 P1 | server 模式明文暴露强警告（AETHERLINK_SKIP_TLS_WARNING=1 显式静默）+ HTTPS 公网入口自动下发 Secure 认证 cookie（init.sh/init.ps1 对等实现） | 同上 |
| 2026-08-25 | 验证状态 | 上述改动定向验证通过（backend app/middleware、broker command/contract 单测，前端 21/21 定向 vitest）；**compose 全栈启动、MQTTS 链路、真实部署回归为 pending**，发布前须按 VALIDATION.md 重跑 | — |
| 2026-08-25 | 质量 P0 | 修复 main 自 #159 起的 typecheck 断裂：补装 @tresjs/core@5/@tresjs/cientos@5/three（组件当前零引用，不进 bundle）+ MotionCard 显式 import motion-v；vue-tsc 全绿 | 同上 |
| 2026-08-25 | 性能 P2 | 前端首屏治理：语言包改按需加载（entry **1701KB→182KB，-89%**，fr/es 保留 en 兜底合并语义）、node-forge 动态 import 移出登录关键路径、chunkSizeWarningLimit 回调 1000 | 同上 |
| 2026-08-25 | 性能 P2 | 后端缓存与查询：设备/脚本缓存 TTL=0 改为 30min 兜底过期（pkg/constant.CacheFallbackTTL，主动失效仍为主机制）；AI 遥测查询 N+1 改单条 IN 批量并下沉租户过滤（GetDevicesByIDsForTenant）。OTA 包加载已有批量回退路径、device_metrics 已有 30s 进程内模板缓存，经复核无需改动 | 同上 |
| 2026-08-25 | A1 | 空租户守卫：alarm 配置/信息/历史列表 + device_config 列表 fail-closed（含 all-tenants 显式授权与回归测试） | #155 |
| 2026-08-24 | A2 | message_push gen LeftJoin raw 化 | 已并入 main |
| 2026-08-25 | A3 | 设备影子全链路：迁移 52.sql、DAL/Service/API/路由、上线投递钩子、cron 清理、DAL 测试、前端影子队列标签页 | #160/#161 |
| 2026-08-25 | C4 | AI 自然语言查询遥测 MVP：意图抽取式 NL 查询服务 + `/ai/telemetry/query` 端点 + 单测 | #160 |
| 2026-08-23/24 | — | users 列表 raw 链收敛 + 空租户守卫；alarm raw 链 P1 修复 | VALIDATION.md |
| 2026-08-25 | 质量 | 全库 GBK 乱码修复（17 文件，含 echarts-manager 被困代码释放）+ 源码编码契约测试绊线 + 影子离线投递 method/params 语义修复 | feat/phase-a-completion |
| 2026-08-25 | B1 | Modbus TCP 插件（独立模块 modbus-plugin）：JSON 点表采集/缩放、MQTT 上报、命令下行写入、内嵌从站单测、compose `--profile modbus`；平台点表存储 + 前端「Modbus 点表」界面 + 插件 OpenAPI Key 拉取闭环；gRPC 通道暂缓（见 B1 备注） | #162 |
| 2026-08-25 | B2 | 可视化规则链编辑器：迁移 56.sql（graph 列）、DAG 校验（Kahn 无环）、拓扑执行引擎 + webhook/command 动作、`/rule-chains` CRUD + 上行双钩子、Vue Flow 画布编辑器 + 四语言 i18n、引擎/CRUD 单测 | feat/rule-chain-b2 |
| 2026-08-26 | fleet | CSV 批量导入设备：`POST/GET /device/preRegister` + `/preRegister/export` 路由补齐（修复前端契约断裂），自动生成/CSV 双模式建档、一次性凭证+脱敏导出、产品租户校验、service 层 sqlite 全链路测试 ×5 | feat/device-preregister-import |
| 2026-09-02 | C2/C6/C7 实现批 | 三线合入单一主线（full-integ：main 12 项增量 ⊕ c5 白标/实体版本/安全 ⊕ gh/main）；C2 资产 CRUD/树 API（Scope=自身∪祖先+成环拒绝，DAL sqlite 测试）；C7 2FA 后端（61.sql 绑定/激活/解绑/第二因子防重放/恢复码）与 OIDC/SSO 后端（62.sql 提供方 CRUD + /sso/:id/start|callback + sessionIssuer）；C6 CoAP/LwM2M UDP 网关配置门控接入 application；ROADMAP 勾选回填 | full-integ |
| 2026-09-04 | C6 收尾（P1-C） | 网关设备接入闭环：lwm2m ObjectStore.OnChange 写入回调 + /rd HandleRegisterWithNotify；protocolgw TelemetryBridge（DBNumberResolver 端点→设备凭证映射[TTL 缓存/fail-closed]、IPSO 键转换+路径回退、队列化汇入 uplink.Bus source_protocol=coap）；app 装配（DB/Bus 未就绪降级纯接入）；单测 +14（protocolgw+lwm2m 合计 31 PASS、vet 干净）；同批完成 C5 运行期验证（28/28×2 零回归）与前端 branding vitest 11/11 | c2-tenant-scope-merge |
| 2026-08-26 | 设计 | 设计系统收敛 L1/L2：字号/圆角 token 落地 global.scss、断点三合一（删 --bp-* 双轨）、uno shortcuts 单源化（preset 导出 aetherlinkShortcuts）、共享 PageHeader 组件收敛 3 页 5 处重复页头、10 个表格页补 NEmpty 空态、linkage-edit 47 处 hex→token 迁移、UI emoji→SvgIcon、裸删除补 Popconfirm、html lang 随语言切换、hex 绊线契约测试（基线 1042→994 只降不升） | feat/device-preregister-import |
