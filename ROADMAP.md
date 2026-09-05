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
| 多租户 RBAC | ✓ | ✓ | ✓ | Casbin + 租户隔离 + 空租户 fail-closed；RBAC 已激活（63.sql 全量授权 quo 显式化） |
| 自动化场景 | ◐ | ✓ | ✓ | 联动/定时/条件编辑 |
| 看板 | ◐ | ✓ | ✓ | 原生看板 + 发布分享 |
| 脚本引擎 | ◐ | ✓ | ✓ | 数据处理脚本 |
| 通知服务 | ◐ | ✓ | ✓ | 邮件/Webhook 通知组 |
| 可视化规则链编辑器 | ✓ | ✓ | ◐ | B2 已合入：DAG 引擎+Vue Flow 画布（56.sql graph 列） |
| API 限流(per-tenant) | ✓ | ✓ (PE) | ✗ | 默认 600 rpm/租户（env 可调），429+Retry-After；backend=memory 单机 / backend=redis 集群共享（2026-09-04 补齐，fail-open 降级） |
| 3D 可视化/SCADA | ✓ | ✓ | ◐ | C3 已落地：设备详情「3D 预览」tab，遥测驱动材质、WebGL 降级 |
| Modbus | ✓ | ◐ | ✓ | B1 已合入：独立插件+点表 UI（55.sql + modbus-plugin） |
| 设备影子 | ✓ | ✓ | ✓ | Phase A3 已交付：离线命令缓存+上线投递 |
| CSV 批量导入设备 | ✓ | ✓ | ✓ | 后端建档/导出与前端导入向导均已落地（自动生成/CSV 双模式、一次性凭证、脱敏导出）；产品选择列表 `/product` 同步补齐 |
| 资产管理层级 | ✓ | ✓ | ✗ | C2：60.sql + CRUD/树 API + RBAC/DAL 级联（expandTenantIDScope 已覆盖 device/alarm/board/ota/rule_chain 等十余模块） |
| CoAP / LwM2M / SNMP | ✓ | ✓ | ✗ | C6 闭环：CoAP/LwM2M UDP 网关 + 端点凭证映射 + 遥测汇入 uplink（P1-C）；SNMP v2c 采集接入（protocol_type=SNMP 点表轮询→uplink，含配置页动态表单与保存校验，2026-09-04） |
| OPC UA | ✓ | ✗ | ✓ | 库级 client（gopcua）+ 连接器/点位采集接入（protocol_type=OPCUA，连接 TTL 缓存/懒重连，含配置页动态表单与保存校验）；**运行期 E2E 已过（2026-09-04，opcuastub 本地 opc.tcp 服务器，遥测 5s 周期落库 tenant 归属正确）** |
| 移动端 App | ◐ | ✓ (PE) | ✓ | MVP 落地（2026-09-05）：uni-app 骨架补完为可用应用——登录/设备列表（在线徽标）/最新遥测查看，H5 构建通过；API 契约与 backend 对齐并实跑验证（发现并修正原骨架误用 /device/list 未绑定设备接口）；小程序/APP 目标构建脚本就绪，上架属发布事项 |
| AI / LLM 集成 | ✓ | ✗ | ✓ | C4：NL 查询遥测 + AI 告警分析（service 单测 4/4）均落地 |
| 计算字段 | ✓ | ✓ | ✗ | B3 已合入：govaluate 安全表达式派生遥测（54.sql） |
| TimescaleDB / TDengine | ✓ | ✓ | ✓ | C1 闭环：57.sql 条件化 hypertable + 7 天压缩；**三态运行期全部验证（2026-09-05）**：auto（无扩展条件跳过）/ off（显式跳过）/ **on 正向**（真实 timescaledb 2.29.2 + EDB PG17.10 隔离实例——hypertable 转换、compress_after=7 chunks 压缩策略、遥测读写全过）与 **on 负向**（扩展缺失→启动失败+可操作指引）。on 态验证发现并修复 57.sql 缺陷：bigint 时间列的 compress_after 必须用 chunk 数而非 INTERVAL（SQLSTATE 22023，auto 模式从未触达故此前不可见） |
| 白标定制 | ✓ | ✓ (PE) | ✗ | C5：主题色/favicon 已入主线（59.sql）；运行期验证完成（28/28×2 轮 + 前端 branding 11/11） |
| 行业模板 | ◐ | ✓ (PE) | ✗ | MVP 落地（2026-09-05，65.sql）：工业传感器/电力监测/智能家居三个开箱即用模板入种子（租户守卫插入，生产库零孤儿行），模板库 API 验证可见；模板分类目录/模板市场/按行业导出属产品迭代 |
| 模板市场 | ◐ | ✓ (PE) | ✗ | MVP 落地（2026-09-05，0bb8620）：导出（GET export/:id 可移植描述符）/导入（POST import，租户幂等 created 标记）/行业分类目录（type_key 过滤），运行期 E2E 全过（含坏 kind 拒绝 100002）；66.sql 登记+授权（g2 注册行 + SA/TA p 行 v2=allow），审计 289 路由；市场浏览页/按行业打包下载属产品迭代 |
| 边缘计算 | ◐ | ✓ (PE) | ✗ | MVP 落地（2026-09-05）：边缘网关遥测云转发（uplink 总线订阅→云 MQTT，断连缓冲/重连重投/fail-open）+ 实时与断网重投 E2E 证据；边端自治规则已运行期实证（断云下采集→存储→场景规则→告警全闭环）；边缘 RPC 命令下行已落地（0bb8620：command-topic 订阅→CommandSink→CommandPutMessageWithTracking，edge-relay 审计）；实体下发（模板经命令通道推送）属下一步 |

结论：核心设备管理链路已接近主流水平。Modbus（B1）、规则链可视化（B2）、计算字段（B3）、CSV 批量导入（fleet）、资产层级（C2）、CoAP/LwM2M/SNMP/OPC UA 接入（C6，SNMP/OPC UA 采集器 2026-09-04 补齐）、白标（C5）均已合入。**2026-09-04 收敛终态**：可实现项全部以运行期证据闭环（SNMP/OPC UA 采集器真实协议 E2E、watcher 双实例同步、限流集群共享配额、OIDC 授权码流程双算法、Timescale auto/off 态）；剩余均为非工程缺口项——Timescale `on` 态（需扩展二进制，本地 PG 无此扩展）、第三方 IdP 互通验收（协议级已过，建议用户环境按 idpstub 同流程抽验）、RBAC 按角色收紧（产品迭代期）、行业模板/边缘计算/移动端（远期）。

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
- [x] SNMP 采集接入（管理侧，2026-09-04）：`internal/collector` 轮询采集器——devices⨝device_configs（protocol_type='SNMP'）发现启用设备，protocol_config 点表（target/community/points[{key,oid}]）一次 Get 批量采集，INTEGER/Counter/TimeTicks→数值、OCTET STRING→文本，汇入 uplink（source_protocol=snmp）；发现缓存 TTL 60s、fail-closed 计数；内嵌 UDP SNMP agent 全链路单测 + miniredis 无关；conf 键 `collectors.snmp.enabled`；**运行期 E2E（隔离栈真实 UDP，2026-09-04）：snmpstub agent（cmd/snmpstub）+ scratch 库启动后端，遥测 5s 周期落库 telemetry_datas（uptime=600 数值、hostname=collectore2e 文本，tenant_id 归属正确）；OPC UA 死端口设备同栈验证 fail-closed（每轮干净告警、不阻塞 SNMP 通道），测后 scratch 库/配置/进程全清** ✓
- [x] OPC UA 连接器接入（管理侧，2026-09-04）：同上骨架，protocol_type='OPCUA' + opcua.Config 连接段（复用 Validate/Normalize），按设备连接 TTL 缓存（5min）+ 读失败懒重连，节点值→遥测键；`collectors.opcua.enabled` 门控 ✓；**运行期 E2E（2026-09-04）：opcuastub（cmd/opcuastub，gopcua server 包本地 opc.tcp 服务器，None 安全/匿名）+ scratch 库启动后端，ns=1;s=temperature(26.5)/humidity(61.0) 每 5s 落库 telemetry_datas、tenant 归属正确、零采集失败；测后 scratch 库/配置/进程全清** ✓
- [x] 配置页动态表单与保存校验（2026-09-04 第二批）：协议下拉内置 SNMP/OPC UA 项（GetServiceSelect 基础列表）；`pointconfig` 子包收敛点表解析/校验（service→collector 导入环破除）；内置 config_form 动态表单（input/select/table 契约，dataKey=点表 JSON 键）；凭证表单回退平台标准方案；device_config 创建/更新按"生效值"校验点表结构（坏点表保存即拒，不再等采集器日志）✓
- [x] CoAP 服务端（RFC7252 子集：编解码/UDP 服务器/注册表/well-known + blockwise/observe 组件）
- [x] LwM2M 注册层 + 对象实例模型 + 观察者推送（UDP 服务器配置门控接入 application 生命周期）
- [x] 设备凭证映射 + 遥测汇入现有 uplink 管道（网关设备接入闭环，WORKPLAN P1-C）：lwm2m.ObjectStore.OnChange 写入回调 + /rd HandleRegisterWithNotify 注册通知；protocolgw.TelemetryBridge（DBNumberResolver 端点→device_number 凭证映射[60s TTL 缓存/fail-closed]、IPSO 键转换 3303/3304/3323/3325/3330 + lwm2m/{o}/{i}/{r} 回退、队列化汇入不阻塞写路径）→ 与 MQTT 同一 UplinkMessage/uplink.Bus（source_protocol=coap）；app 装配 DB/Bus 未就绪自动降级纯接入层（注：uplink.enable=false 时 WithFlowService 早退不建 uplinkService，网关仅纯接入——生产启用遥测汇入须同时开 uplink）；单测 +14（protocolgw+lwm2m 合计 31 PASS）；**运行期 E2E（2026-09-04，隔离栈真实 UDP）：外部客户端注册 2.01 / 写资源 2.04 / 读回 2.05 三步全过，遥测经凭证映射汇入 uplink 管道并按租户落库 telemetry_datas（temperature=26.5，tenant_id 正确），测后种子已回滚**；已知限制：coap 子集 codec 的 option 值仅支持内联 ≤12 字节（扩展编码 13/14 拒绝），超长端点名（如 urn:imei:xxx）需先行 codec 升级 ✓

### C7. 安全与平台能力补齐（对标 ThingsBoard CE 免费能力，2026-08-25 审查补录）
- [x] 双因素认证 2FA（61.sql 后端 + 前端：登录两段式动态码页、个人中心绑定/停用/恢复码组件）
- [x] OAuth2/OIDC 单点登录后端（62.sql：租户 IdP 配置 CRUD、/sso/:id/start|callback、sessionIssuer 绑定本地用户）；登录页 SSO 入口待接
- [x] 实体版本控制（设备/看板/规则链等实体 JSON 导出 + Git 式备份恢复，对标 TB 3.5+ Version Control）
- [x] OIDC 登录页 SSO 入口已实现：`GET /api/v1/sso/providers`（平台级启用提供方公开发现）+ 登录页 SSO 按钮（跳 /sso/:id/start）
- [x] **Keycloak 真第三方 IdP 互通 E2E（2026-09-05）**：Keycloak 26.7.3 真实部署（start-dev 8180 + kcadm 建 realm/client/user）→ 平台 SSO 流程对真实 Keycloak 全链路通过：authorize 真登录表单提交 → 真实 RS256+JWKS 验签 → email 绑定本地用户 → 平台 JWT 调鉴权 API 200。**互通发现并修复平台缺陷**：authorize 的 redirect_uri 为相对路径会被规范 IdP 以 Invalid parameter: redirect_uri 拒绝——新增 sso.public-base-url 配置构造绝对回调（留空保持同域反代旧行为）✓
- [x] OIDC 授权码流程协议级 E2E（2026-09-04，隔离栈本地 idpstub）：`cmd/idpstub` 本地 OIDC Provider（Discovery + /authorize 自动批准一次性 code + /token + RS256 模式 JWKS）+ scratch 库 + 种子用户/租户提供方 → **完整授权码流程全链路通过**：start 302 IdP（state+nonce cookie）→ authorize 302 回调 → code 换 id_token → 验签（HS256 与 RS256+JWKS 两路均过）→ email 绑定本地用户 → 302 落地页携带平台 JWT → 该 JWT 调鉴权 API 返回 200+用户档案；负路径：篡改 state 400、无 state cookie 重放 400（防重放生效）。**遗留说明**：①真实第三方 IdP（Keycloak/Auth0 等）互通建议用户环境按同流程验收（协议面一致，风险在 IdP 侧实现差异而非平台代码）；②发现 redirect_uri 为相对路径 `/api/v1/sso/:id/callback`——同域反代部署可用，跨域外部 IdP 需 IdP 侧容忍或后续提供方配置支持绝对回调地址（改进项）。测后 scratch 库/配置/进程全清 ✓

### C7+. RBAC 激活工程（casbin 路由覆盖，2026-09-04 审计评估立项）
> 现状：启动审计报 286 条受保护路由未登记 casbin 资源表（`casbin.deny-unregistered` 未开 + dev 库无种子 → 运行期跳过角色校验）。严重度评估：JWTAuth 组中间件仍强制认证、DAL 层租户隔离是第二道防线，缺口是**角色级授权**（任意已登录用户可调全部业务 API），非匿名裸奔；但角色间权限边界（管理员 vs 普通用户）当前不存在。
- [x] 匹配器与 fail-closed 开关（本轮）：configs/casbin.conf matcher 增 `urlPatternMatch` 锚定模式通道（自实现，弃用内置 keyMatch2——其非锚定正则存在 `api/v1/devices` 误命中 `api/v1/devicesXYZ` 的越权放大）；GetUrl 双通道（精确 g2 + 锚定模式）修"参数路由永远无法被保护/识别"的潜伏缺陷；CasbinRBAC 增 `casbin.deny-unregistered` 开关（默认 false 保持现状，true 时未登记 403 fail-closed）；测试 +10（utils 表驱动 12 例/service 模式判定与 Enforce/中间件严格模式 3 例）
- [x] RBAC 激活（2026-09-04 用户授权代行裁决——**状态 quo 显式化**：3 内置角色 × 全部受保护路由全量授权，激活不改任何用户有效行为，收紧后续逐行删 p）：①矩阵裁决 = 全量授权；②`63.sql` 幂等种子（g2 登记 287 模式[含方法感知审计补出的 PUT /logo] + p 861 授权 + g 绑定存量 15 用户，ON CONFLICT 幂等）；③新建用户空 RoleIDs 时按 users.authority 兜底绑定 + 绑定后 LoadPolicy 刷新内存（sys_user_manage，adapter 判空防单测 panic）；④`route-audit-mode` 切回 **fail-fast**（实测 287 路由全登记通过）；⑤dev conf 开 `casbin.deny-unregistered: true`。运行期证据：audit passed(287)、登录→已登记路由 200、撤销单路由 p 授权→403 非法访问、C5 28/28 零回归。
- [x] 集群 casbin watcher（2026-09-04）：Redis Pub/Sub 最小 persist.Watcher（`internal/adapter/casbinwatcher`，自通知跳过/全量重载/订阅确认 fail-fast，miniredis 双实例通知契约测试）；`global.CasbinEnforcer` 切换 **SyncedEnforcer**（watcher 触发的 LoadPolicy 与并发 Enforce 互斥，顺带消除既有无锁竞态）；`casbin.watcher.enabled` 门控挂载于 Redis 就绪后（单实例默认关闭，无行为变化）；**双实例运行期 E2E（2026-09-04，隔离栈 scratch 库 + SSO 登录令牌）：实例 A `POST /casbin/user` 变更 g 绑定（200）→ 2s 后实例 B `GET /casbin/user` 即返回新绑定（内存重载生效），B 日志恰一条「收到跨实例变更通知，策略已重载」，A 零自通知** ✓
- [ ] 后续收紧（产品迭代期逐次进行）：按角色删除具体 p 授权行即每个收紧决策的 diff。

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
| 2026-09-04 | C6/C7+/限流 收尾批 | ①SNMP 采集接入：internal/collector 轮询采集器（devices⨝device_configs 发现、protocol_config 点表、内嵌 UDP agent 单测、uplink 汇入 source_protocol=snmp）+ OPC UA 连接器（连接 TTL 缓存/懒重连）；app.WithCollectors 门控装配（collectors.*.enabled）。②casbin 集群 watcher：Redis Pub/Sub persist.Watcher + SyncedEnforcer 切换（消除 Enforce/LoadPolicy 竞态）+ casbin.watcher.enabled 门控。③限流集群版：tenant_rate_limit 拆 store 接口，Redis 固定窗口 Lua（INCR+PEXPIRE+PTTL）+ fail-open 降级，api-rate-limit.backend=memory|redis。④ROADMAP 矩阵回填过时行（资产级联/白标/AI 告警分析） | 本地主线 8fdd51e |
| 2026-09-04 | C6/C1 运行期 E2E 批 | SNMP 采集器隔离栈运行期 E2E：snmpstub agent（cmd/snmpstub 新工具，复用 internal/snmp BuildGetResponse）+ scratch 库 + collectors.snmp.enabled 启动 → 遥测 5s 周期落库 telemetry_datas（uptime=600/hostname=collectore2e，tenant 归属正确）；OPC UA 死端口同栈验证 fail-closed 不阻塞；TimescaleDB 三态运行期：auto（无扩展跳过）+ off（显式跳过 57.sql，日志「跳过 sql/57.sql（TimescaleDB 显式关闭）」）已验证，on 环境绑定。测后 scratch 双库/配置/日志/进程全清 | 本地主线 |
| 2026-09-04 | OPC UA 运行期 E2E 批 | opcuastub（cmd/opcuastub，gopcua server 包本地 opc.tcp 服务器，None 安全/匿名，Start 非阻塞需挂主协程）+ scratch 库 + collectors.opcua.enabled 启动 → ns=1;s=temperature(26.5)/humidity(61.0) 每 5s 落库 telemetry_datas、tenant 归属正确、零采集失败。OPC UA 全链路（连接/读值/转换/汇入/落库）实证闭环，不再依赖外部真实服务器；测后全清 | 本地主线 |
| 2026-09-04 | C7 OIDC 协议级 E2E 批 | cmd/idpstub 本地 OIDC Provider（Discovery/authorize 一次性 code/token/RS256 JWKS）+ 隔离栈 scratch 库实测：start→authorize→callback 授权码流程全链路通过，HS256 与 RS256+JWKS 双验签路径均过，ID Token email 绑定本地用户签发平台 JWT（调鉴权 API 200+用户档案）；负路径（篡改 state/无 cookie 重放）双 400；发现并记录 redirect_uri 相对路径限制（跨域 IdP 需绝对回调地址，改进项）；测后 scratch 库/配置/进程全清 | 本地主线 |
| 2026-09-05 | 模板市场+边缘 RPC 批 | 0bb8620：接手树上遗留草稿并完成——模板导出/导入（租户幂等）+分类目录（type_key）+66.sql（g2 登记行+SA/TA p 行 v2=allow）+修复三处遗留缺陷（errcode.CodeParams 未定义/WaitTimeout 反转/YAML 重复 edge 根键）；边缘命令下行接线（CommandSink→CommandPutMessageWithTracking）；运行期 E2E 全过（审计 289 路由）；56 包 0 FAIL | 本地主线 |
| 2026-09-05 | 边缘计算 MVP 批 | internal/edgeforward：订阅 uplink 总线观察者 → 云 MQTT 转发（topic={prefix}/{type}/{device_id}，JSON 信封含 payload+metadata）+ 断连环形缓冲（buffer-limit 满丢最旧计数）+ 重连按序重投，fail-open 不影响本地入库；cmd/edgemqttbroker 验证工具；**LIVE E2E（snmpstub→uplink→边缘转发→云 broker）**：实时转发 10 条信封完整、断云 15s 缓冲→重连重投日志无缺口、恰 2 次建连、本地入库 20 条不受影响；单测 3 场景（回环 MQTT broker）全绿 | 本地主线 |
| 2026-09-05 | 移动端 MVP 批 | active/mobile-app-uni：API 契约对齐（修正误用 /device/list 未绑定接口→/device 分页）+ 设备列表在线徽标与点击展开最新遥测 + README 契约表；H5 构建 DONE；三端点实跑验证（login 200/分页列表字段命中/遥测空态数组） | 本地主线 |
| 2026-09-05 | 行业模板 MVP 批 | 65.sql 三行业模板种子（工业传感器/电力监测/智能家居，租户守卫+幂等）+ VERSION_NUMBER 65 → 模板库 API 列表验证三模板可见（total=3） | 本地主线 |
| 2026-09-05 | C1 on 态 + Keycloak 互通批 | ①Timescale on 态完整闭环：真实 timescaledb 2.29.2（EDB PG17.10 隔离实例，官方 issue #10348 指引 PG≥17.10）→ hypertable 转换/compress_after=7 chunks 压缩策略/遥测读写全过 + on 负向 fail-closed；**发现并修复 57.sql 缺陷**（bigint 时间列 compress_after 须用 chunk 数）。②Keycloak 26.7.3 真第三方互通 E2E 全链路通过；**发现并修复 redirect_uri 相对路径缺陷**（sso.public-base-url 绝对回调）。③RBAC 按角色收紧 64.sql 落库（TU 撤管理面 32 行/TA 撤平台面 12 行），三角色 32/32 矩阵运行期验证 | 本地主线 |
| 2026-09-04 | 集群双实例 E2E 批 | 双 backend 实例（9199/9200，同 scratch 库同 Redis，watcher 开 + api-rate-limit backend=redis rpm=5）：①watcher——A 经 POST /casbin/user 变更 g 绑定，2s 内 B 的 GET 即返回新绑定且日志恰一条「收到跨实例变更通知，策略已重载」（A 零自通知）；②限流共享配额——A 连续调用第 6 次起 429（固定窗口），**B 从未被压测直接 429+Retry-After:33**（Redis 共享计数跨实例生效）。测后 scratch 库/配置/进程全清 | 本地主线 |
| 2026-09-04 | C6 表单与校验批 | SNMP/OPC UA 配置页动态表单：GetServiceSelect 内置协议项 + builtinCollectorConfigForm（input/select/table 契约，dataKey=点表 JSON 键）+ 凭证表单回退平台标准方案 + device_config 创建/更新按生效值校验点表（pointconfig 子包破除 service→collector 导入环，保存即拒坏点表） | 本地主线 |
