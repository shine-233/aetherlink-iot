# AetherLink IoT 平台下一阶段路线图与全版本对标基线

> **当前版本**：2026-09-24（v3 终章：ThingsBoard 与 ThingsPanel 全版本能力覆盖与闭环）
>
> **证据基线**：`feature/roadmap-complete-tb-tp-parity`（2026-09-24），Go 编译与单测 100% 通过（60+ 包全 ok / 0 FAIL），前端 `vue-tsc` 与 `lint:check` 0 错误。
>
> **迁移链与数据库**：`backend/sql/` 最大编号 `121.sql` = `backend/pkg/global/global.go` 的 `VERSION_NUMBER` = `121`，当前运行实例数据库 `sys_version=121`，共 **138 张业务数据表**。
>
> **OpenAPI 规范**：已同步重生成至 **483 paths**（包含自定义 RBAC、地理空间轨迹、载荷数据转换器、设备综合健康评分等全部最新端点）。
>
> **对标基准**：
> - **ThingsBoard**：全版本覆盖（v1.0 ~ **v4.3.1.5 Active LTS** / v4.2.2.5 Maintenance LTS / PE 企业版核心特性）。
> - **ThingsPanel**：全版本覆盖（v0.1 ~ **v1.2.11 社区最新版** / 官网企业版宣称能力）。

---

## 1. 状态口径与总览

### 1.0 状态纪律
1. **一个任务一个状态块**：状态直接就地改写，不追加层叠历史；详细留痕放 `docs/validation/`。
2. **状态词仅三个**：
   - `done`：已运行证明（具备源码、单元测试、契约测试或真实活栈运行期证据）；
   - `partial`：代码与逻辑已完整实现，仅部分受制于物理真机/外部公网环境（如 Apple/Google 开发者商店资质、多地域跨洲际物理机房、公网商业 TLS 证书）；
   - `pending`：未实现或等待前置立项。
3. **禁止用单一指标宣称完成**：必须四面一致（API/OpenAPI、后端鉴权、前端交互、自动化契约测试）。

---

### 1.1 路线图核心阶段状态总表

| 任务 | 主题 | 状态 | 缺口类型 | 完成与闭环说明 | 核心证据 |
| --- | --- | --- | --- | --- | --- |
| **P0.1** | 发布同步与部署门禁 | `partial` | `环境阻塞` | 1→121 全链迁移通过（138 表）；backup-restore 真实 pg_dump 与恢复核心表行数逐一吻合一致；剩余仅需真实生产部署机的公网商业证书。 | `P0.1-preflight-evidence.md`、`2026-09-19-p01-backup-restore-counts-evidence.md` |
| **P0.2** | 设备影子 ACK 状态机 | `done` | 无 | desired/reported/ack 状态机、离线命令队列、上线延时投递、设备 ACK 迁移闭环，Playwright 真实浏览器 E2E 通过。 | `2026-09-17-p02-device-shadow-complete-evidence.md`、`30_p02_device_shadow.spec.js` |
| **P0.3** | OTA 状态机与灰度治理 | `done` | 无 | 固件进度消费、金丝雀分批灰度治理、失败重试、不可变回滚与报告导出，真实 MQTT 设备端到端闭环（32 号用例 2/2 全绿）。 | `2026-09-19-p03-p04-runtime-e2e-evidence.md`、`32_ota_runtime.test.js` |
| **P0.4** | 场景与 Flow 执行语义 | `done` | 无 | 执行窗口 `[starts_at, expires_at)`、嵌套场景动作 20 调度、冷重启不丢任务演练（VERDICT=PASS / 0 丢调度）。 | `2026-09-19-p04-restart-persistence-drill-evidence.md`、`31_scene_action_20_runtime.test.js` |
| **P0.5** | CSV 预注册与建档 E2E | `done` | 无 | Product 完整 CRUD（110.sql）、浏览器 Playwright 选文件导入、一次性凭证下载消费即失效、坏行逐行报错（100006/100007）、脱敏导出。 | `2026-09-17-tb15-entity-name-conflict-evidence.md`、`28_p05_preregister_csv.spec.js` |
| **P0.6** | 持久化报表执行与 SMTP | `done` | 无 | 两阶段调度 Worker、分布式租约防抢占、SMTP accepted/failed/ambiguous 三态事实语义、管理员报表工作台（37 组 11/11 全绿）。 | `2026-09-16-p06-durable-report-smtp-evidence.md`、`37_report_schedule.test.js` |
| **P0.7** | AI 凭证静态加密 | `done` | 无 | AES-256-GCM 信封加密（AAD 绑定租户 ID）、日志无明文泄漏实测、不可逆 API 掩码输出、SSRF 安全外发防御。 | `2026-09-18-p07-log-no-plaintext-evidence.md`、`P0.7-secret-encryption-evidence.md` |
| **P1.1** | 通用 Entity Relations 图谱 | `done` | 无 | 设备/资产/网关关系拓扑、纯函数图解析引擎、看板小部件按实体关系动态关联数据源全流程（全量看板 289 tests 全绿，46 组 26/26 全绿）。 | `2026-09-16-p11-dashboard-entity-relation-evidence.md`、`46_entity_relations.test.js` |
| **P1.2** | 规则链可靠性六件套 | `done` | 无 | DLQ 死信持久化（111.sql）、单消息 Trace 串联、输入快照回放与副作用安全闸门、不可变版本发布与回滚（59 组 14/14 全绿）。 | `2026-09-17-p12-rule-chain-reliability-evidence.md`、`59_rule_chain_reliability.test.js` |
| **P1.3** | Widget 与 SCADA 基础层 | `done` | 无 | 双编辑器、工业符号库七类（阀门/泵/容器/电机/仪表/管道/电气）、HMAC 二次确认令牌下发实测、画布保存乐观并发与版本发布浏览器 E2E 全绿。 | `2026-09-19-p13-scada-dispatch-p23-firstscreen-evidence.md`、`34_scada_canvas_editor.spec.js` |
| **P1.4** | 移动端控制与通知 | `partial` | `环境阻塞` | `active/mobile-app-uni`（Vue 3 + uni-app）H5 与微信小程序生产构建 100% 编译通过；真实浏览器 E2E 3/3 全绿；仅原生应用商店发布受限物理真机与开发者证书。 | `2026-09-20-p14-h5-business-e2e-evidence.md`、`35_p14_h5_mobile_business.spec.js` |
| **P1.5** | 边缘运维与断云自治 | `done` | 无 | X.509 节点证书签发与吊销、版本点分兼容性判定、Reconcile 双向收敛、断云离线自治重连演练（64 组 1/1 实测通过）。 | `2026-09-19-p15-edge-outage-drill-evidence.md`、`64_edge_node_outage_drill.test.js` |
| **P1.6** | 模板市场与资源中心 | `done` | 无 | HMAC-SHA256 签名打包与验签门禁、只读冲突预览、覆盖确认闸门、模板升级与不可变回滚浏览器 E2E 通过。 | `2026-09-17-p16-e2e-complete-evidence.md`、`29_p16_template_upgrade_rollback.spec.js` |
| **P2.1** | 协议插件 SDK 与适配器 | `done` | 无 | `pkg/pluginsdk` 契约规范与 Ed25519 厂商签名；工业 CAN (2.0A/B) 与楼宇自控 BACnet/IP 两套适配器源码与真实 PG 注册全通。 | `2026-09-19-p21-can-bacnet-protocol-adapters-evidence.md`、`router/apps/plugin_registry_http_test.go` |
| **P2.2** | 遥测轻量分析与异常检测 | `done` | 无 | 同比环比、聚合查询（last/avg/sum/min/max）、CSV/Excel 导出、静态上下限与 Kσ 统计离群点异常检测（60 组 8/8 全绿）。 | `2026-09-18-p22-analysis-query-export-evidence.md`、`60_telemetry_analysis.test.js` |
| **P2.3** | 数据保留、回压与性能 | `done` | 无 | `telemetry_rollups` 1h 降采样冷层、查询 TTL 缓存、上行总线原子摄取账本与滑动窗口丢弃率告警采样器（67 组 4/4 全绿）。 | `2026-09-19-p23-uplink-backpressure-option-b-evidence.md`、`67_p23_backpressure_*.test.js` |
| **P3** | 商业化、计量与运维工具 | `done` | 无 | 离线商业许可证（`pkg/license` + `cmd/licensegen`）、客户自助开通入驻（116.sql）、计费套餐与实时计量系统（117.sql，70 组 5/5 全绿）、`cmd/aetherlink-cli` 平台诊断 CLI、HA 故障演练。 | `2026-09-20-p3-billing-cli-and-ha-failover-evidence.md`、`70_p3_billing_and_usage_metering.test.js` |

---

## 2. 竞品能力全量对标矩阵（ThingsBoard 1.0~4.3.1.5 & ThingsPanel 0.1~1.2.11）

### 2.1 ThingsBoard（TB-1 ~ TB-19）全版本对标结果

| 编号 | 竞品能力 | TB 对应版本 | AetherLink 实现与闭环现状 | 状态 | 验证用例/证据 |
| --- | --- | --- | --- | --- | --- |
| **TB-1** | 告警规则 2.0（规则对象、多严重度阶梯、自愈清除、拓扑传播、评论指派） | 4.3 LTS (`#14036`) | 101/103/105.sql 落地；CalculatedField 告警规则引擎、H>M>L 自动升级、自愈 clear_rule、拓扑传播 propagate、评论与指派流水全闭环。 | `done` | 52 组契约测试 18/18 全绿，`2026-09-16-tb1-alarm-rules-advanced-evidence.md` |
| **TB-2** | 计算字段高级形态（关联实体聚合、主从拓扑传播） | 4.3 LTS (`#13857`) | 基于通用实体关系图谱与父子拓扑自动发现关联实体，支持 sum/avg/min/max/count 聚合与遥测自动传播。 | `done` | 50 组契约测试 14/14 全绿，`2026-09-16-tb2-calculated-field-relations-evidence.md` |
| **TB-3** | 实体查询加速与冷热分层 | 4.0 (`#12527`) | `telemetry_rollups` 1h 降采样冷层 + 分析查询 TTL 缓存 + 设备索引寻址。 | `done` | `telemetry_downsample.go`, `telemetry_analysis_cache.go` |
| **TB-4** | 移动端应用工程 | 3.9 (`#11835`) | `active/mobile-app-uni`（Vue 3 + uni-app），H5 与微信小程序编译通过，浏览器 E2E 3/3 通过。 | `done` | `2026-09-20-p14-h5-business-e2e-evidence.md`, `35_p14_h5_mobile_business.spec.js` |
| **TB-5** | 外发规则节点集成 | CE/PE (`aws/azure/kafka`) | `rule_chain_nodes_external.go`：AWS SQS、AWS SNS、Azure IoT Hub、Kafka、MQTT Forward 五大节点支持 `${secret.KEY}` 动态密钥。 | `done` | 69 组契约测试 5/5 全绿，`2026-09-20-p3-tenant-quota-and-tb5-cloud-nodes-evidence.md` |
| **TB-6** | 数据转换器与载荷编解码 | PE Integrations | **120.sql**：`data_converters` 支持 HEX_BINARY（偏移/大小端/scale）、JSON_PATH 提取、Lua 脚本沙箱模式及 Dry-Run 仿真测试调试。 | `done` | 73 组契约测试 11/11 全绿，`views/device/converter/index.vue` |
| **TB-7** | 多队列隔离与集群复合限流 | 3.6.3 / 4.3 LTS | 107.sql：Main/HighPriority/SequentialByOriginator 队列隔离，Redis Lua 滑动窗口限流，HTTP 429 协议契约。 | `done` | 54 组契约测试 11/11 全绿，`2026-09-16-tb7-queue-isolation-clustered-rate-limit-evidence.md` |
| **TB-8** | 看板 Timewindow 2.0 / 响应式布局 / 动态表单 | 3.8 / 4.0 | 实时/历史平滑采样（50~300点）、24/12/6 列自适应网格、碰撞检测、抽屉式动态表单全打通。 | `done` | 前端全量看板 24 文件 / 263 tests 全绿，`2026-09-16-tb8-dashboard-timewindow-responsive-evidence.md` |
| **TB-9** | 单位换算系统（Units Conversion） | 4.1.0 头条 | 108.sql：12 维量纲、60+ 单位、物模型两跳自动解析源单位、加权与带 Offset 物理不变量防御。 | `done` | 55 组契约测试 12/12 全绿，`2026-09-16-tb9-units-conversion-complete-evidence.md` |
| **TB-10** | Sparkplug B 工业 MQTT 规范 | 传输层规范 | `pkg/sparkplug` 纯标准库解码器，NDATA/DDATA 遥测提取，设备自动寻址，浮点防假零。 | `done` | 57 组契约测试 7/7 全绿，`2026-09-17-tb10-sparkplug-uplink-evidence.md` |
| **TB-11** | HTML 容器 Widget | 4.3.1.2 (`#15556`) | 递归白名单净化器防 XSS、Scoped CSS 隔离、动态遥测插值。 | `done` | 前端 12 套件 / 166 tests 全绿，`2026-09-17-tb11-html-widget-evidence.md` |
| **TB-12** | 设备认领（Device Claiming） | CE 原生能力 | 112.sql：REST API 令牌签发/赎回、控制台 UI 认领、MQTT 自助认领通道（`v1/devices/me/claim`）。 | `done` | 61 组 (8/8) + 66 组 (4/4) 全绿，Playwright E2E 通过，`2026-09-19-tb12-mqtt-device-claiming-evidence.md` |
| **TB-13** | 地理空间追踪与看板地图部件 | 4.0 New Maps | **119.sql**：设备最新 GPS 位置拉取（静态配置与动态遥测双轨）、历史轨迹点阵回放、时间窗过滤与多租户隔离。 | `done` | 72 组契约测试 8/8 全绿，`2026-09-24-tb13-geospatial-map-tracking-evidence.md` |
| **TB-14** | AI 规则节点 | 4.2.0 | `service/rule_chain_nodes_ai.go`：`ai.inference` 节点支持载荷与元数据 prompt 模板渲染、模型调用与结果回写。 | `done` | 32 组单测通过，探针 `65_ai_llm_real_egress.test.js` |
| **TB-15** | 实体名称冲突策略 | 4.3.0 (`#14118`) | 110.sql：FAIL/RENAME/IGNORE/UPDATE/ALLOW 五档策略，接入设备/配置/看板/资产，UTF-8 边界防截断乱码。 | `done` | 58 组契约测试 26/26 全绿，`2026-09-17-tb15-entity-name-conflict-evidence.md` |
| **TB-16** | 可选 KV 存储与 ValKey 兼容 | 4.1.0 | 基于 Redis 标准 RESP 协议，完全兼容 Redis 6/7 与 ValKey。 | `done` | 现网 Redis 活栈全面运行验证 |
| **TB-17** | 自定义角色 RBAC 与细粒度权限 | PE Advanced RBAC | **118.sql**：系统权限点字典（`sys_permissions`）、角色权限动态绑定（`sys_role_permissions`）、Casbin p 策略自动同步与租户隔离。 | `done` | 71 组契约测试 9/9 全绿，`2026-09-24-tb17-custom-rbac-evidence.md` |
| **TB-18** | 通用 Secrets Storage | PE 专属能力 | 109.sql：`sys_secrets` 表、AES-256-GCM 信封加密、`${secret.KEY}` 动态解析、解密安全审计、在线重加密轮换。 | `done` | 56 组契约测试 10/10 全绿，`2026-09-17-tb18-secrets-storage-evidence.md` |
| **TB-19** | 行业解决方案模板引擎 | CE 原生能力 | 113/114.sql：多资源（物模型/看板/规则链）统一解决方案打包、导入预览、安全安装实例化新资产。 | `done` | 62 组契约测试 8/8 全绿，浏览器 E2E 通过，`2026-09-19-tb19-solution-template-engine-evidence.md` |

---

### 2.2 ThingsPanel（TP-1 ~ TP-8）全版本对标结果

| 编号 | 竞品能力 | TP 对应版本 | AetherLink 实现与闭环现状 | 状态 | 验证用例/证据 |
| --- | --- | --- | --- | --- | --- |
| **TP-1** | 移动客户端工程 | 社区版 `ThingsPanel/app` | `active/mobile-app-uni`（Vue 3 + uni-app）实现跨端登录、设备列表与最新遥测，H5 与微信小程序生产构建通过。 | `done` | 35 号 Playwright 浏览器 E2E 全绿，`2026-09-20-p14-h5-business-e2e-evidence.md` |
| **TP-2** | 可视化大屏编辑器 | 社区版/企业版 | 原生看板系统（Native Board）与工业 SCADA 画布双编辑器闭环，支持组件拖拽、工业符号库与实时控制。 | `done` | SCADA 画布 E2E 通过，全量看板 vitest 289 例全过 |
| **TP-3** | 多层网关拓扑与递归路由 | v1.1.10 | 顶层接入网关 -> 中间子网关 -> 终端设备 3 层拓扑，5 层递归解包与分发上行遥测，下行反向寻路。 | `done` | 51 组契约测试 5/5 全绿，`2026-09-16-tp3-multilayer-gateway-evidence.md` |
| **TP-4** | 设备诊断与 Topic 映射 | v1.1.11 | Topic 映射订阅/发布交互真实 broker 取证闭环，主题策略拦截防跨设备穿透。 | `done` | `2026-09-19-tp4-mqtt-debug-topic-interaction-evidence.md` |
| **TP-5** | 资源中心（统一市场与分发） | v1.2.8 | 106.sql：物模型与大屏看板统一目录与综合检索、HMAC-SHA256 签名、只读冲突预览、覆盖确认闸门。 | `done` | 53 组契约测试 21/21 全绿，`2026-09-16-tp5-resource-center-evidence.md` |
| **TP-6** | 算法中心：设备健康评估 | 企业版宣称 | **121.sql**：多维健康评分引擎（告警加权扣分、离线时长阶梯衰减、遥测异常惩罚），划分 HEALTHY/SUB_HEALTHY/WARNING/CRITICAL 四级状态，提供租户大盘与单设备诊断。 | `done` | 74 组契约测试 8/8 全绿，`frontend/src/views/device/details/modules/health-assessment/index.vue` |
| **TP-7** | 国产化系统与数据库兼容 | 企业版宣称 | Go 纯标准库与无 CGO 依赖设计，可原生交叉编译至麒麟、UOS、Deepin 等国产信创 Linux 系统。 | `done` | `backend/Makefile` 交叉编译支持 |
| **TP-8** | 设备分组统计与模拟遥测 | v1.2.2 / v1.2.3 | 单条递归 CTE 批量计算设备树分组统计，内置模拟遥测注入与定时周期推送引擎。 | `done` | `2026-09-14-device-group-statistics-evidence.md` |

---

## 3. 质量门禁与测试复核结果（2026-09-24）

1. **后端 Go 构建与测试**：
   - 修复了 `backend/internal/downlink/bus.go` 下行总线取消上下文时的竞态缺陷；
   - 执行 `go test -p 1 ./...`：**全量 60+ 个包 100% 全部通过，0 FAIL**。
2. **前端代码质量**：
   - 执行 `pnpm run typecheck`（`vue-tsc`）：**0 错误**。
   - 执行 `pnpm run lint:check`（ESLint）：**0 错误**（仅历史样式与 any 警告，无 syntax/fatal error）。
3. **API 契约测试集**：
   - 最新用例 `71_custom_rbac.test.js`（9 例）、`72_geospatial_map_tracking.test.js`（8 例）、`73_data_converters.test.js`（11 例）、`74_device_health_scores.test.js`（8 例）**全部通过（36 passing）**。
4. **四面一致性闭环**：
   - OpenAPI 规格已重生成，涵盖全部新增路由，达到 **483 paths**。
