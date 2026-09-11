# 下一阶段路线图（Phase D）—— 竞品对标差距与规划（2026-09-06）

> 调研方式：官网浏览（thingsboard.io / thingspanel.cn）+ 竞品源码克隆比对。
> 竞品源码快照（本次克隆，供后续实现参考）：
> - `C:\Users\Zz\Documents\projects\research\repos\thingsboard\`（main = 4.4.0-SNAPSHOT）
> - `C:\Users\Zz\Documents\projects\research\repos\ThingsPanel-Go\`
> - `C:\Users\Zz\Documents\projects\research\repos\thingspanel-frontend-community\`
> - 另有 2026-08-25 的 TB 功能全集调研：`C:\Users\Zz\Documents\projects\research\docs\thingsboard-feature-inventory.md`（仍然有效，TB 当前 LTS 仍是 4.3.x 线）
>
> 承接关系：ROADMAP.md 的 Phase A/B/C 已全部交付（tip `ac72114`，含 #211–#214）。本文提出 **Phase D**；
> 定稿后建议把 Phase D 章节并入 ROADMAP.md，本文件转为交付记录。
> 差距判定均经本地源码 grep 复核（2026-09-06），非凭记忆。

---

## 一、竞品现状快照（2026-09-06）

### ThingsBoard
- **版本**：CE 4.3.x 为 Active LTS（v4.3.1.4，2026-08-27，支持至 2027-07）；main 已进入 **4.4.0-SNAPSHOT**（GitHub Milestone #29 完成约 84%：WebSocket RPC 重构、Change Originator 未命中链等）。
- **近期主线**：
  - **AI-native**（4.2 起，持续加码）：AI Models 管理中心（8+ 提供商 + Ollama 本地）、AI Request 规则节点（Text/JSON/JSON Schema 输出）、AI Solution Creator、AI Assistant（自然语言生成计算字段/告警规则/看板/通知）、CLI + MCP Server（面向 AI 编码代理）。
  - **Alarm Rules 2.0**（4.3）：条件编排/时长/传播重构。
  - **计算字段高级类型**（4.3）：地理围栏、传播、关联实体聚合、时序聚合 + 输出策略。
  - **SCADA**（3.8→4.3 持续）：约 132+ 工业组态符号、HP 符号族。
  - **Edge 4.3**：实体/遥测同步、AI 模型同步、OTA 经边缘分发、Push-to-Cloud/Edge 节点。
- **对 AetherLink 的含义**：TB 的护城河正在从"功能全"转向"AI 原生 + 生态工具链"；规则引擎节点数量（90 个）与 widget/SCADA 生态是可见差距的最大来源。

### ThingsPanel
- **社区版边界**（官网 business 页 + 源码确认）：Go 单体 + **gRPC 协议插件**框架 + 物模型/设备模板（模板内嵌图表）+ ThingsVis 可视化（编辑器/大屏/模板/预览）+ 场景自动化 + **资源中心（模板/看板 bundle 市场发布与安装）** + 数据脚本 + 设备调试台 + 基础多租户；移动端 App（uniapp）社区可用。
- **社区版缺失**（企业版卖点）：OTA、白标、完整大屏、批量导入增强；OPC UA/SNMP/IEC104/GB28181/BACnet/LoRaWAN/KNX 等协议插件按个单卖。
- **对 AetherLink 的含义**：AetherLink 在协议接入（CoAP/LwM2M/SNMP/OPC UA 免费内置）、RBAC 深度、限流、2FA/SSO、边缘自治上已反超 TP 社区版；TP 领先的体验面主要是 **插件生态框架、模板市场运营页、设备调试台、可视化编辑器完成度**。

---

## 二、差距矩阵（仅列差距项，✓=已具备/无差距）

图例：✗ 缺失 · ◐ 部分实现 · (PE)=TB 仅付费版

| # | 功能域 | AetherLink 现状（已核实） | TB CE | TP 社区 | 差距定性 |
|---|---|---|---|---|---|
| 1 | 规则链节点 | 8 种（trigger.telemetry/device_online/telepathy、filter.threshold、transform.mapping、action.alarm/command/webhook） | 90 节点×7 类 | 规则引擎+自动化 | **最大硬差距**：缺 enrichment 整类（属性/关联实体富化）、flow（嵌套链/延迟/checkpoint）、transformation（script/dedup/split/rename）、external（Kafka/MQTT 外发）、analytics（聚合）、generator（模拟器）；无节点级调试事件 |
| 2 | 通知渠道 | SMTP + Webhook | 8+ 渠道（SMS/Slack/Telegram/Teams/移动推送…） | 邮件/短信为主 | 缺国内渠道（钉钉/企微/飞书）、SMS 网关、Telegram；缺告警分配 assignee |
| 3 | 计算字段 | govaluate 简单表达式 | Simple/Script + 4.3 四种高级类型 + 重算(PE) | ✗ | 缺时序聚合/关联实体聚合/地理围栏/传播/历史重算 |
| 4 | 看板 widget 体系 | 原生看板 + 3D 预览；无 widget 包(bundle)概念 | 31 官方包/600+ widget | 可视化插件+ThingsVis | 缺 widget 分类包与模板化交付 |
| 5 | SCADA 符号库 | ✗ | 132+ 符号 | FUXA 集成(企) | 缺组态符号库 |
| 6 | 地图 | 高德底座 + 标记组件（data-table-page/gaode-map） | 标记/行程/路线/多边形/围栏 | — | 缺轨迹回放、地理围栏、多边形图层 |
| 7 | 视频流 | ✗ | RTSP/HLS widget | go2rtc 集成 | 缺视频监控场景（摄像头类项目刚需） |
| 8 | 报表与导出 | 设备 CSV 建档导出；遥测历史无通用 CSV 导出；无报表 | Reporting 2.0 (PE) | — | 遥测 CSV 导出是低垂果实；轻量定时报表（看板→PDF 邮件投递）可差异化 |
| 9 | 设备接入安全 | Access Token/MQTT Basic（X.509 仅用于内部 RSA/OIDC） | X.509 证书 + 凭证轮换 | — | 缺 X.509 设备证书准入与轮换 |
| 10 | 边缘实体同步 | 遥测转发/命令下行/模板下发/断云自治 ✓ | 看板/规则链/OTA 全实体 gRPC 同步 | ✗ | 缺看板与规则链同步到边端、OTA 经边缘分发、遥测上报策略（省带宽） |
| 11 | AI | NL 查询遥测 + 告警分析 ✓ | AI 规则节点/模型中心/助手/CLI+MCP | 部分(企) | 缺 AI 规则节点、多提供商模型管理、AI 助手 |
| 12 | 生态/DevOps | OpenAPI Key；无设备 SDK/CLI/MCP；OpenAPI 文档未规范化 | SDK(Python/Arduino/Dart/Java)、CLI+MCP、OpenAPI 规范化 | — | CLI/MCP 是 TB 2026 重点投入方向，跟进窗口期红利 |
| 13 | 模板市场 | 导出/导入/分类目录 ✓；无市场浏览页/按行业打包下载 | Solution Templates (PE 目录) | 资源中心完整运营页 | 产品收口项（自家 ROADMAP 已列） |
| 14 | 设备运维体验 | 设备详情/影子/点表/3D | — | 设备调试台(device_debug)、数据转发服务接入(第三方云桥接) | 可选借鉴：调试控制台、service access 插件（对接第三方云） |
| 15 | LwM2M FOTA | OTA 整包任务 ✓（MQTT 通道） | LwM2M FOTA | — | 低优先 backlog |

无差距确认（本次复核过，此前矩阵已列）：设备分组 ✓、告警时长条件 ✓（alarm_trigger_duration）、告警备注 ✓、Casbin RBAC+收紧 ✓、限流集群版 ✓、2FA/OIDC ✓、实体版本控制 ✓、Timescale ✓、白标 ✓、行业模板 ✓、CoAP/LwM2M/SNMP/OPC UA/Modbus ✓、计算字段基础 ✓、CSV 批量导入 ✓。

---

## 三、Phase D 规划

排序逻辑：**先补数据面硬差距**（规则引擎、通知——每个用户的日常路径）→ **体验面**（可视化/报表）→ **差异化深化**（边缘/AI/安全/生态）。延续 house 规则：每项以运行期证据闭环验收，不做"仅代码合入"。

### D1 规则引擎 2.0（最高优先，1–2 迭代）
补齐 TB 七类节点中缺失的四类，节点数从 8 → 约 30。
- [ ] **Enrichment 富化**：originator attributes / originator latest telemetry / related device attributes / tenant metadata（参考 TB `rule-engine/rule-engine-components` enrichment 包；AetherLink 的资产树 Scope 可直接复用为 related 查询）
- [ ] **Flow 流控**：嵌套子链（rule chain 节点）、delay、checkpoint（异步切面）
- [ ] **Transformation 扩充**：script 节点（挂接现有 data_script 引擎，不新建沙箱）、rename keys、split array message、dedup（时间窗去重）、change originator
- [ ] **External 外发**：MQTT 外发（复用 broker 客户端）、HTTP 外发（action.webhook 已有，纳入节点分类规范）、Kafka 外发（可选依赖，编译门控）
- [ ] **Action/Analytics**：generator 模拟数据源（设备模拟器常驻化）、latest 聚合、message count
- [ ] **节点级调试事件**：执行 trace 落库（每节点最近 N 条输入/输出/耗时），画布点击节点即查——这是 TP 与 TB 的体验标配，也是自研引擎的可观测性兜底
- [ ] 告警规则补强核查：re-trigger 去重与 clear 条件的完备性（时长条件已有）
- 验收：每类节点 ≥1 条运行期 E2E（沿用隔离栈 scratch 库套路）；画布回归全绿；引擎单测零回退。

### D2 通知中心 2.0（1 迭代）
- [ ] 渠道抽象 `Channel` 接口（现有 SMTP/Webhook 迁移为内置实现）
- [ ] 新增渠道：钉钉/企微/飞书机器人（webhook 变体，最易落地）、SMS 网关（阿里云/腾讯云，配置化）、Telegram Bot
- [ ] 通知模板变量引擎 + 发送记录（重试次数/失败原因可查）
- [ ] 告警分配（assignee）与告警认领流转
- [ ] 触发器接入：规则链 `action.notification` 节点 + 场景自动化通知
- 验收：≥2 个真实渠道实发 E2E（钉钉/飞书 webhook 可无账号自建调试台）+ 模板渲染单测。

### D3 可视化与报表（2 迭代，可与其他线并行）
- [ ] **Widget 包体系**：建立 bundle 概念（卡片/图表/控制/状态 4 包先行），看板组件按包注册与交付；与模板市场打通（bundle 可导入导出）
- [ ] **SCADA 符号库 MVP**：SVG 符号 + 数据绑定（值/动作），首期泵/阀/罐/管道 20 个符号（参考 TB traditional fluid system 子集，自绘规避版权）
- [ ] **地图升级**：轨迹回放（Trip）、地理围栏（polygon + 进出事件）、多边形图层；高德底座已有，扩展图层模型
- [ ] **视频流 widget**：HLS/ go2rtc 外部集成（显式可选，未配置时报告 disabled——遵循现有集成边界原则）
- [ ] **遥测历史 CSV 导出**（设备详情，低垂果实，可先行单独交付）
- [ ] **轻量报表**：看板快照→PDF + 定时邮件投递（复用 D2 邮件渠道）
- 验收：地图轨迹/围栏、视频流、报表 PDF 端到端演示各 1；widget 包契约测试。

### D4 计算字段高级类型（1 迭代）
对齐 TB 4.3 的四种高级类型：
- [ ] 时序聚合（滚动窗口 min/max/avg/count/sum）
- [ ] 关联实体聚合（沿资产树 assets 向上汇总子设备遥测——AetherLink 有资产层级，实现路径比 TB 更直接）
- [ ] 地理围栏（基于位置遥测进出判定）
- [ ] 传播（结果沿实体关系上/下游传播）
- [ ] 历史重算（Task Manager 模式：后台任务 + 进度 + 幂等）
- 验收：四类型各 1 E2E + 重算幂等验证。

### D5 接入安全补齐（1 迭代）
- [ ] X.509 设备证书认证：注册/签发（或外部 CA 对接）/轮换/吊销，broker 认证插件扩展
- [ ] 设备凭证批量轮换 API + UI
- [ ] Spark Plug B 立项评估（broker 侧地址空间解码；先出评估结论再排期）
- [ ] （backlog，按客户需求排期，不进承诺）BACnet/CAN/BLE/OCPP/KNX 网关连接器——TP 企业版也按个单卖，说明需求是碎片化的
- 验收：X.509 全生命周期运行期 E2E（openssl 本地 CA + broker 实连）。

### D6 边缘计算 2.0（2 迭代）
在 edgeforward/命令下行/模板下发底座上补齐 TB Edge CE 的核心同步面：
- [ ] 看板与规则链实体同步到边端（扩展 template/import 命令通道的 kind 集合）
- [ ] OTA 经边缘分发（边缘代理固件下载，断云可用）
- [ ] 遥测上报策略（边端按变更/间隔过滤上报，省带宽）
- [ ] Push-to-Cloud / Push-to-Edge 规则链节点（与 D1 节点体系一体交付）
- 验收：断云-重连全场景回归（复用既有 E2E 套路）+ 新同步实体在边端可用性验证。

### D7 AI 2.0（1–2 迭代）
对标 TB AI-native 路线，守住"AI 产出必须人工确认"的安全边界：
- [ ] AI 规则节点：规则链内调用 LLM（Text / JSON Schema 输出，超时/失败降级路径）
- [ ] 模型管理中心：多提供商注册（OpenAI 兼容为主 + Ollama 本地），密钥集中管理与连通性测试
- [ ] AI 助手 MVP：自然语言→告警规则/计算字段**草稿**（生成后人工确认才生效）
- [ ] 评估 CLI + MCP Server（TB 2026 重点投入方向，早期跟进有窗口红利；本地 AI 编码代理运维平台是差异化叙事）
- 验收：AI 节点 E2E（stub LLM，含 JSON Schema 输出校验）；助手产物默认 draft 状态。

### D8 生态与 DevOps（1 迭代，可穿插）
- [ ] OpenAPI 规范化：swagger 完整注解生成 + 文档站（对齐 TB 4.3.1 的 OpenAPI 改造）
- [ ] 设备端 SDK：Python 最小集（telemetry/attributes/RPC）+ Arduino 示例；MicroPython 次之
- [ ] 移动端增强：推送通知 + 扫码配网（对标 TB Mobile App Center 的 QR 配网）
（模板市场收口已升级为独立工作流 D10；设备调试台已升级为 D11）

### D9 插件框架 gRPC 网关（1–2 迭代，2026-09-06 用户裁决：解除 B1 的"暂缓"决策，正式立项）
对标 ThingsPanel 的 gRPC 协议插件框架——这是 TP 生态扩张的核心机制，AetherLink 现有 HTTP pluginruntime 边界不足以承载第三方协议插件生态。
- [ ] **gRPC 契约**：插件注册（name/version/capabilities）、心跳、上行数据推送（插件→平台）、下行命令转发（平台→插件）、配置拉取；proto 定义入 `backend/internal/pluginruntime/proto/`
- [ ] **平台侧网关 server**：端口配置门控接入 application 生命周期；插件接入凭证（token，哈希存储对齐 OpenAPI Key 风格）
- [ ] **插件注册表**（74.sql）：name/version/transport/endpoint/status/last_heartbeat；健康检查与掉线自动标记；插件管理 API + 前端管理页（列表/详情/启用禁用）
- [ ] **数据面接线**：插件上行 → uplink 总线（复用 collector/protocolgw 的汇入模式，source_protocol=plugin）；下行：命令按设备绑定的插件路由转发
- [ ] **参考插件改造**：modbus-plugin 增加 gRPC 传输模式（配置切换，默认保持 MQTT 模式不变）——作为插件框架的首个真实消费方
- [ ] bufconn 进程内 gRPC 单测全链路（注册→心跳→上行→下行→掉线）
- 验收：bufconn 全链路单测 + modbus-plugin 双模式单测；运行期 E2E（真 gRPC 端口）由集成阶段补。

### D10 模板市场运营化（1 迭代，2026-09-06 新增：对标 TP 资源中心）
现有基础：模板导出/导入/分类目录 type_key（0bb8620/a6cc8a8）。缺的是"运营面"：
- [ ] 市场浏览页：分类 tab/卡片列表/详情页/一键导入（复用现有 import API）；搜索与排序
- [ ] 按行业打包下载：行业 → 模板集 zip（清单+模板 JSON），导出端点 + 前端下载入口
- [ ] 发布流水线：本地模板 → 发布为市场条目（版本/changelog 字段，75.sql 如需；审计沿用现有 middleware）
- [ ] 我的发布管理页 + 浏览统计（下载计数）
- 验收：发布→浏览→导入 全链路运行期 E2E（集成阶段执行）。

### D11 设备调试台（1 迭代，2026-09-06 新增：对标 TP device_debug）
现有底座：`internal/mqttdebug`（paho 传输/会话/主题策略）已具备，缺控制台面。
- [ ] 设备详情新增「调试」页签：实时遥测订阅流（SSE，复用 router/sse.go 先例）、命令下发面板（method/params 表单→CommandSink 语义，显示 ACK/超时）、原始上下行日志时间线（内存环形缓冲，可选落库 76.sql）、属性读写
- [ ] 会话管理：每设备并发调试会话上限 + 租户守卫 + 审计
- [ ] 文件边界：新建组件文件；device details 容器仅一行注册（标记 PHASE-D-D11）
- 验收：调试会话全链路单测（stub broker）+ 前端组件测试；运行期 E2E（隔离栈真设备）由集成阶段补。

---

## 四、明确不做 / 暂缓（延续既有决策）

- **gRPC 插件网关**：继续暂缓（B1 决策：等真实插件消费方再评估）。
- **EDQS 级内存查询 / TBMQ 级百万吞吐基准**：远期，规模指标非当前竞争力主战场。
- **License Server / 计费 / 多级客户层级**：商业版议题；租户层级已有资产树 + Scope 级联替代。
- **Reporting 全功能（子报表/模板设计器）**：TB 也是 PE；D3 只做轻量报表。
- **RBAC 逐角色收紧**：按 RBAC-TIGHTENING-RUNBOOK.md 持续进行，不占 Phase D 里程碑。

## 五、排期建议（总约 8–10 迭代）

| 迭代 | 内容 | 说明 |
|---|---|---|
| 1 | D1（上）+ D3 遥测 CSV 导出 | 规则引擎 enrichment/flow + 调试事件 |
| 2 | D1（下）+ D2 | external/analytics/generator + 通知中心 |
| 3 | D9 插件框架 gRPC 网关 | 用户裁决立项（解除 B1 暂缓） |
| 4 | D4 + D10 模板市场运营化 | 计算字段高级类型 + 市场浏览/打包 |
| 5 | D11 设备调试台 | mqttdebug 底座上加控制台 |
| 6–7 | D3 | widget 包/SCADA/地图/视频/轻量报表 |
| 8 | D5 | X.509 + Spark Plug B 评估结论 |
| 9–10 | D6 | 边缘 2.0 |
| 10–11 | D7 | AI 2.0（可与 D6 部分并行） |
| 穿插 | D8 | OpenAPI/SDK/移动端 |

里程碑门禁：每个 D 项合入前，后端 `go build/vet/test` + 前端 typecheck/vitest/build + automation 三契约全绿（现行门禁不变）。

## 六、信息源

- ThingsBoard：[发布总表](https://thingsboard.io/docs/releases/releases-table/) · [v4.3.x 说明](https://thingsboard.io/docs/releases/releases-table/v4-3-x/) · [4.2 发布博客](https://thingsboard.io/blog/thingsboard-4-2-release-reporting-2-0-ai-integration-secrets-management-and-more/) · [PE Roadmap](https://thingsboard.io/docs/pe/releases/roadmap/) · [Milestone 4.4](https://github.com/thingsboard/thingsboard/milestone/29) · 本地克隆源码
- ThingsPanel：[官网](https://www.thingspanel.cn/) · [产品功能页](https://www.thingspanel.cn/function) · [企业版页](https://www.thingspanel.cn/business) · GitHub/Gitee 组织 · 本地克隆源码（backend/frontend）
- 既有调研：`research/docs/thingsboard-feature-inventory.md`（2026-08-25，TB 功能全集）
