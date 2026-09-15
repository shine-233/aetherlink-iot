# 2026-09-15 竞品版本清单与 CE/PE 能力边界（联网调研证据）

> 数据源：GitHub Releases API（ThingsBoard 87 个 release / ThingsPanel 后端 51 + 前端 45）、
> 完整 git 文件树（ThingsBoard 10086 文件、ThingsPanel 后端 704 + 前端 917）、以及两家官网对比页。
> **CE/PE 结论一律以仓库真实路径为准，不采信官网对比表。**

## 0. 这份证据为什么重要

路线图 §2 此前采信了官网对比表，导致若干"社区版没有"的判断**是错的**，
而 §7 的差距项（TB-x / TP-x）正是从这些判断推导出来的——**前提错了，立项就错**。

本轮共发现 **10 处"官网说 CE 没有、仓库里真有"**，以及 12 处"官网说没有且确实没有"的一致性校验。
另有 1 处存疑（ThingsPanel 一型一密），已明确标注，未下结论。

---

## 1. ThingsBoard（`thingsboard/thingsboard`，Apache-2.0，Java）

默认分支 `master`，最近推送 2026-09-13。规则节点实现 87 个（`rule-engine/rule-engine-components/.../Tb*Node.java`）。

### 1.1 版本表（2023-09 起，倒序）

| 版本 | 日期 | 主要变更 | CE 真有？（仓库路径） |
|---|---|---|---|
| v4.3.1.5 | 2026-09-11 | CVE；时序实体聚合计算字段数字存成字符串；Sparkplug 别名下遥测名为空；Edge 断连通知防刷 | — |
| v4.2.2.5 | 2026-09-11 | 同批 CVE；Edge 事件拉取失败不再卡同步 | — |
| v4.3.1.4 / v4.2.2.4 | 2026-08-27 | CVE/CWE-407；图表轴刻度字体；Digital Gauge 零值校验 | — |
| v4.3.1.3 / v4.2.2.3 | 2026-06-30 | CVE；**新增 IoT Hub 集成**；AI 模型结构化输出；CF SUM 溢出 | ✅ `controller/IotHubController.java`、`service/iot_hub/DefaultIotHubService.java` |
| v4.3.1.2 / v4.2.2.2 | 2026-05-28 | CVE + SSRF（AI provider URL、TBEL 沙箱）；**租户配置审计日志**；OpenAPI 重构；Kafka LZ4 | ✅ `controller/AuditLogController.java`、`dao/audit/AuditLogDao.java` |
| v4.3.1.1 / v4.2.2.1 | 2026-03-31 | XSS/SSRF（DNS rebinding 白名单）；可配安全头与 CORS；Spring Boot 3.5；OTA 包数据清理 | ✅ `service/ota/DefaultOtaPackageStateService.java` |
| **v4.3.1 / v4.2.2** | 2026-03-10 | **Angular 20 迁移**；SSRF 保护开关；CVE | ✅ `ui-ngx/` |
| v4.3.0.1 / v4.2.1.2 | 2026-02-03 | CVE；Redis ACL 用户名认证；PG 非 public schema 分区清理 | — |
| **v4.3** | 2026-01-20 | **Alarm rules 2.0**；计算字段地理围栏/传播/时序聚合/关联实体聚合与输出策略；**API Keys**；**强制 2FA**；实体 displayName；基础镜像换 openjdk25 | ✅ 2FA `service/security/auth/mfa/provider/impl/{Totp,Email,Sms,BackupCode}TwoFaProvider.java`；API Key `service/security/auth/pat/ApiKeyAuthenticationProvider.java` |
| v4.2.1.1 | 2025-12-24 | 禁止删除最后租户管理员；预置设备校验；OTA 走 URL 时的固件更新 | — |
| v4.2.1 | 2025-10-15 | AI 请求节点支持附件；**本地模型 Ollama**；OpenAI base URL 可改；导出文件名 UTF-8 | ✅ `rule-engine/.../ai/TbAiNode.java` |
| **v4.2** | 2025-08-15 | **AI 请求规则节点**；告警计数查询支持实体/键过滤；OTA 包纳入版本控制；Sparkplug 3.0；每 Edge 统计 | ✅ `rule-engine/rule-engine-components/.../rule/engine/ai/` |
| **v4.1 / v4.0.2** | 2025-07-03 | Cassandra 5.0、ValKey 8.0；计算字段性能优化；**单位换算**；SCADA 单位换算；8 种新 UI 语言 | ✅ SCADA 符号 218 个 SVG 在 `application/src/main/data/json/system/scada_symbols/` |
| **v4.0** | 2025-04-15 | **计算字段 Calculated Fields**；**EDQS 内存实体数据查询**；保存属性/时序策略；**SCADA 布局与符号库**；新地图部件；动态表单取代 JSON Schema | ✅ `actors/calculatedField/`、`service/sync/vc/`、`application/src/main/data/json/system/scada_symbols/` |
| **v3.9** | 2024-12-31 | **移动应用中心**；告警状态 WebSocket API；高级调试模式；Angular 18 + Tailwind + esbuild；LwM2M 升 Leshan M15；Edge 事件改用 Kafka；4.0 起弃用非 Kafka/内存队列、弃用 Timescale | ✅ `controller/MobileAppController.java`、`service/mobile/secret/` |
| **v3.8** | 2024-10-03 | 事件与审计日志专用数据源；实体/属性/时序/关系 version 字段与缓存；规则引擎控制器 + REST API 回执节点；OAuth2 配置重构；**SCADA 布局与符号库**；看板布局与断点；时间窗重构 | ✅ `controller/AuditLogController.java`、`controller/OAuth2Controller.java` |
| **v3.7** | 2024-06-17 | 迁移 Java 17；属性存储优化；housekeeping 服务；核心队列按分区消费；新时序图表；状态图/饼图/雷达等部件；Spring Boot 3.1；SpringDoc OpenAPI 3.1 | ✅ `ui-ngx/` |
| v3.6.3 | 2024-03-18 | 移动端推送通知；WEEK/MONTH/QUARTER 分组间隔；设备连接状态粒度可配；时序图表/带标签条形图/开关/动作/命令按钮 | — |
| v3.6.2 | 2023-12-28 | 图片库；转换节点增强；WebSocket 会话去重；版本控制性能；工业/空气质量部件 | — |
| **v3.6** | 2023-09-21 | 隔离处理队列；Microsoft Teams 通知；滚动重启优化；邮件 OAuth2；过滤/富化节点重构；部件基础配置模式 | — |

> v1.0（2016-12-06）→ v3.5.1（2023-05-31）共 55 个 release，此处不逐一列出；
> 完整清单来源 `https://github.com/thingsboard/thingsboard/releases`。

---

## 2. ThingsPanel（后端 `thingspanel-backend-community` + 前端 `thingspanel-frontend-community`，Apache-2.0，Go+Vue3）

> **许可证更正**：网上多篇文章（如 2025-03 的对比文）称 ThingsPanel 为 AGPLv3.0，**已过时**——
> `LICENSE` 与 README 均已改为 **Apache-2.0**。

| 版本 | 日期（后端/前端） | 主要变更 | 开源仓库真有？ |
|---|---|---|---|
| v1.2.11 | 2026-09-03 / 08-31 | 无 release notes（前端：Telemetry 枚举描述、ThingsVis 数据源保存、iframe 尺寸抖动） | — |
| v1.2.10 | 2026-08-21 | 仅 changelog（前端：超管 Dashboard v2、设备库模型选择、SSO 场景租户首页配置丢失） | — |
| v1.2.9 | 2026-08-14 | 资源中心令牌自动续期与刷新；模板封面发布/下载/安装；Lua 沙箱恢复 `os.time()` 但仍禁命令执行/删文件 | ✅ Lua 沙箱 `internal/service/` |
| **v1.2.8** | 2026-08-04 | **新增资源中心**（设备模板 + 看板模板统一入口）；看板模板可按设备快速建实例；超管初始化加确认密码 + PG 自动迁移 | ✅ `internal/service/dashboard_template.go`、`internal/model/vis_dashboard.gen.go`、`internal/service/market_dashboard_bundle.go`；前端 `src/components/thingsvis/` |
| v1.2.3 | 2026-06-18 | **设备分组统计**（总数/在线/离线/告警）；按协议类型+设备类型取配置表单；协议插件统一为服务插件体系 | ✅ `internal/api/protocol_plugin.go`、`internal/api/service_plugin.go` |
| v1.2.2 | 2026-06-08 | 设备模拟数据初始化与发送（MQTT 遥测/属性/事件）；自动化联动支持按事件参数匹配；凭证表单多语言 | — |
| **v1.2.0** | 2026-05-15 / 05-18 | **遥测聚合查询**（avg/max/min/sum/diff，参数化 SQL + 空值/超大值过滤）；设备趋势按自然小时最长 30 天；**Redis Pub/Sub 指数退避重连**（SSE/WS）；设备状态不再依赖 Redis 预判，全量落状态历史 | ✅ `internal/service/`、`internal/adapter/mqttadapter/` |
| v1.1.14 | 2026-04-10 | 设备模板新增 `type`/`brand`/`modelNumber`；聚合查询过滤 NULL 与超大数；ThingsVis SQL 脚本 v0.0.17；修复设备激活未绑定默认根组 | ✅ `internal/service/device_template.go` |
| v1.1.13.5 | 2026（3 月档） | 集成 **ThingsVis 可视化引擎**；面板 V2 双渲染器（GridStack + 轻量网格）；多主题；腾讯地图增强；设备详情 App 视图 | ✅ 前端 `src/components/thingsvis/ThingsVisAppFrame.vue` 等 |
| **v1.1.13** | 2026-02-06 | 设备调试日志（Redis 存储）；设备诊断重构为点数计数；WS 心跳超时与 ping；消息推送支持多设备 | ✅ `internal/diagnostics/` |
| v1.1.12 | 2025-12-26 | 移动推送通知系统；WS 批量订阅设备在线状态；告警历史按 ID 删除；**MQTT 支持共享订阅，切到 gmqtt** | ✅ `internal/adapter/mqttadapter/` |
| v1.1.11 | 2025-11-21 | 设备诊断服务；**设备主题映射管理 API**；设备状态历史追踪 | ✅ `internal/api/device_topic_mapping.go` |
| **v1.1.10** | 2025-10-27 / 10-24 | **MQTT 消息处理架构全面重构**；**多层网关设备支持**（网关-子设备绑定，遥测/命令/属性/事件多层级穿透）；断线自动重订阅；**用 Redis Pub/Sub 替代 MQTT 做设备状态订阅** | ✅ `internal/adapter/mqttadapter/`、`internal/service/device.go` |
| v1.1.8 | 2025-06-12 | 新增一型一密；设备配置支持图片上传；网关子设备动态注册 | ⚠️ 见 §4 存疑项 |
| v1.1.5 | 2025-03-03 | **API Keys 管理**；租户级设备在线趋势 | ✅ `internal/api/`、`internal/service/` |
| v1.1.4 | 2025-01-16 | Prometheus 集成 + 系统监控看板；Redis 升 v9；手机号登录；API 响应标准化 + 国际化 | ✅ `pkg/metrics/` |
| **v1.0.0** | 2024-06-06 | **Gin + Vue3 + TS 全面重构**；设备插件改名设备功能模板、新增设备配置模板；接入框架升级（协议接入 + 服务接入）；**单独产品管理 + 固件升级**；权限细化到分组和设备；服务触发、自动化与告警解耦 | — |
| v0.3.0 | 2022-08-17 | Redis 缓存；casbin RBAC（粒度到按钮/接口）；规则引擎与数据转发；设备无限分组 | — |
| v0.2.0-beta | 2022-05-13 | **系统设置支持更换所有 logo 和系统名称** | ✅ `internal/api/logo.go` |

> 来源：`https://github.com/ThingsPanel/thingspanel-backend-community/releases`、
> `https://github.com/ThingsPanel/thingspanel-frontend-community/releases`

---

## 3. CE / PE 边界：10 处「官网说 CE 没有、仓库里真有」

基线：TB 对比页 `thingsboard.io/ce-vs-pe-diff/`；TP 对比页 `www.thingspanel.cn/business`。

| # | 平台 | 争议项 | 官网说法 | 仓库证据 |
|---|---|---|---|---|
| 1 | **TB** | **SSO / OAuth2** | CE = No | 完整 OAuth2 客户端体系：`controller/OAuth2Controller.java`、`OAuth2ConfigTemplateController.java`、`config/CustomOAuth2AuthorizationRequestResolver.java`、`common/data/.../oauth2/`，内置模板 `application/src/main/data/json/system/oauth2_config_templates/{apple,facebook,github,google}_config.json`。**Controller 头部无任何 PE 授权/License 校验。最明确的矛盾。** |
| 2 | **TB** | **白标 / 自定义域名** | CE = No | Domain 实体 + 全套 CRUD：`common/data/.../domain/Domain.java`（`name`/`oauth2Enabled`/`propagateToEdge`）、`controller/DomainController.java`、`service/entitiy/domain/DefaultTbDomainService.java`。但 `white.?label`/`custom.?translation` 在 10086 个路径中命中 **0**，UI 只有静态 `ui-ngx/src/assets/logo_*.svg` → **域名级白标有，品牌级白标确实没有**，官网属"部分失真"。 |
| 3 | **TB** | **Solution Templates** | CE = No | 安装引擎在 CE：`service/solutions/DefaultSolutionService.java` + `service/solutions/data/definition/` 下 20+ 定义类 + `service/iot_hub/`。**框架在 CE，模板内容从云端 Hub 拉（可能多为 PE 内容）** → 部分矛盾。 |
| 4 | **TB** | **2FA** | 对比表未列，市场普遍认为 PE | `service/security/auth/mfa/provider/impl/` 下 Totp / Email / Sms / BackupCode 四种 Provider 齐全，v4.3 还加 Enforced 2FA。**CE 有完整 2FA。** |
| 5 | **TB** | **审计日志** | CE = Yes（对比表已承认） | `controller/AuditLogController.java`、`dao/audit/AuditLogDao.java`。列此条是因为业界普遍误传为 PE 专属。 |
| 6 | **TP** | **大屏** | 社区版"无大屏" | `internal/service/dashboard_template.go`、`internal/model/vis_dashboard.gen.go`、`internal/repo/dashboard_template_repo.go`、`internal/service/market_dashboard_bundle.go`、`internal/api/dashboard_menu.go`；前端 `src/components/thingsvis/{ThingsVisAppFrame,ThingsVisViewer,ThingsVisWidget}.vue`、`src/hooks/thingsvis/`。前端 v1.2.8 notes 明写"新增大屏模板市场，支持浏览、发布和安装"。**明确矛盾。** |
| 7 | **TP** | **产品管理 + OTA** | 社区版"无产品管理 OTA" | `internal/api/ota.go`、`internal/dal/ota_upgrade_packages.go`、`internal/dal/ota_upgrade_tasks.go`、`internal/model/ota_upgrade_tasks.gen.go`；产品 `internal/{api,service,dal,model,query}/product*`；前端 `src/service/product/update-ota.ts`。v1.0.0 公告自述"简化了产品管理和固件升级"。**明确矛盾。** |
| 8 | **TP** | **白标（换 logo / 系统名）** | 社区版"不支持白标" | `internal/{api,service,dal}/logo.go`、`internal/model/logo.gen.go`、`internal/query/logo.gen.go`、`router/apps/logo.go`；前端 `src/components/common/system-logo.vue`、`src/layouts/modules/global-logo/index.vue`。v0.2.0-beta（2022）notes 就写了"支持更换系统上所有 logo 和系统名称"。**明确矛盾**（企业版可能只是定制更完整 + 授权允许）。 |
| 9 | **TP** | **Redis 实时数据** | 社区版"无"、企业版"有" | v1.1.10 notes："使用 Redis Pub/Sub 替代 MQTT 进行设备状态订阅"；v1.2.0："Redis Pub/Sub 监听增加指数退避重连机制，提升 SSE、WebSocket 实时推送恢复能力"。**明确矛盾。** |
| 10 | **TP** | **技术文档** | 社区版"不提供" | `docs/` 下 `README-DEV.md`、`code_help/`（AI_code、注释/日志/命名规范、目录结构、golang 规范）、`demand-community/`（33 份需求设计文档）、`docs/设计/`、`docs/发布版本.md`。**明确矛盾**（只是不如企业版体系化）。 |

### 3.1 一致性校验通过（官网说没有、仓库确实没有）

| # | 平台 | 项 | 仓库实测 |
|---|---|---|---|
| 11 | TB | LPWAN（LoRaWAN/Sigfox） | `lorawan` **0 命中**；`sigfox` 仅 1 个 UI 图标，无后端。一致 |
| 12 | TB | 平台集成中心（AWS IoT/Azure/PubSub/Kafka） | 全树 `integration` 153 命中**全是 `*IntegrationTest.java`**，无 `service/integration/`、无 UI 页面（0）。但**有 AWS/Azure 规则节点** `rule-engine/.../aws/{lambda,sns,sqs}/`、`rule-engine/.../mqtt/azure/TbAzureIotHubNode.java`，UI 还白送 30 个 `integration-icon/*.svg`。→ **"规则节点级云对接"CE 有，"集成中心 + 数据转换器"CE 没有**，对比表把两者合并表述，容易误导 |
| 13 | TB | 高级 RBAC（自定义角色） | `common/data/.../security/` 只有 `Authority.java` 枚举，无 Role 实体。一致 |
| 14 | TB | Secrets Storage | 全树 `secret` 仅 5 命中，全是 `service/mobile/secret/`，无通用密钥保管。一致 |
| 15 | TB | 报表与定时调度 | `report` 9 命中，全是 API 用量上报/地理围栏策略/安装报告，无报表引擎。一致 |
| 16 | TB | 400+ 编解码库 | `codec` **0 命中**。一致 |
| 17 | TB | LDAP | `ldap` **0 命中**。一致（OAuth2 见 #1） |
| 18 | TB | Edge | 服务端 `service/edge/` **129 个文件在 CE**；运行时独立仓库 `thingsboard/thingsboard-edge` **同样是 Apache-2.0** → Edge 侧代码也开源，商业点在其授权而非源码封闭 |
| 19 | TP | TCP 协议接入 | `internal/adapter/` 只有 `mqttadapter/`，无 TCP 适配器。一致 |
| 20 | TP | Kafka | `kafka` **0 命中**。一致 |
| 21 | TP | 集群部署 | 无 K8s/集群调度代码。一致 |

### 3.2 存疑（未下结论）

| # | 平台 | 项 | 说明 |
|---|---|---|---|
| 22 | TP | 一型一密 | 官网称社区版"无一型一密激活功能"，但 v1.0.0 公告与 v1.1.8 notes 都说支持/新增了一型一密。用 `secret`/`oneTypeOneSecret`/`voucher`/`product` 全量搜 704 个后端文件，**未找到独立模块**，`internal/model/products.gen.go` 也无 secret/voucher 字段。可能写在 `internal/service/device_auth.go` + 设备配置里，或用中文拼音命名。**建议人工在 `internal/service/device_auth.go`、`internal/api/device_auth.go` 复核后再下结论。** |

---

## 4. 对 AetherLink 的可借鉴项

1. **先做服务端派生数据层（CF），而不只是规则引擎。** TB 从 4.0 建 CF、4.3 补齐地理围栏/传播/时序聚合/关联实体聚合/输出策略，连续 4 个大版本投入。Go 侧可用 actor / 单飞 goroutine + 增量计算，比 Java 更轻。
2. **EDQS 内存实体查询服务值得抄架构。** TB 4.0 把实体查询从 DB 搬到内存服务，是它敢谈"百万设备"的关键。可做独立内存索引服务 + Kafka/Redis 事件流同步。
3. **实时推送通道要可替换 + 指数退避重连。** TP v1.1.10 把设备状态订阅从 MQTT 换成 Redis Pub/Sub，v1.2.0 又补退避重连；TB 4.x 反复修 WS 会话限制/重连风暴 → 一开始就抽象成接口。
4. **审计日志与 2FA 直接进社区版，别当商业卖点。** TB 两者都在 CE。企业客户 audit/合规是刚需，做成付费墙反而劝退。
5. **白标分两层：域名路由做在 CE，品牌定制（logo/名称/背景）单独控。** TB 的 `Domain` 实体（自定义域名 + 每域名 OAuth2 + 下发 Edge）是很优雅的解法。
6. **资源中心 / 模板市场是低成本高杠杆的生态抓手。** TP v1.2.8 一次上线设备模板市场 + 大屏模板市场 + 依赖完整性检查 + 安装向导；TB 4.3.1.3 上 IoT Hub。Go 后端 + 一个模板 REST 契约就能做。
7. **协议接入必须插件化，且"服务插件 vs 协议插件"分层从一开始就定死。** TP v1.2.3 还在"统一使用服务插件体系管理协议扩展"，说明它自己也踩了分层混乱的坑。
8. **多层网关是一等公民，不要后补。** TP v1.1.10 专门重构 MQTT 架构才支持网关-子设备多层穿透。工业现场刚需，后补代价极大。
9. **版本化 + Git 版控实体配置值得做轻量版。** TB 从 3.6.1 优化到 4.2（OTA 包纳入版控），说明"配置可回滚/diff/跨环境迁移"是企业验收项。Go 侧可用嵌入式 Git + JSON 序列化。
10. **AI 节点要内置 URL 白名单与沙箱。** TB 4.2 上 AI 节点、4.2.1 支持 Ollama、4.3.1.3 结构化输出，紧接着 4.3.1.2 就修了 AI provider URL 的 SSRF——"能力上线快，安全补丁紧跟"。第一版就该内置防护（可参考 TP 的 Lua 沙箱：禁命令执行/删文件）。

---

## 5. 对路线图的直接影响

§2 此前采信官网对比表，以下几处**已被本证据证伪，需修正**（详见 ROADMAP §2 的修正批注）：

- TB「PE 额外提供 SSO」→ **CE 有完整 OAuth2**（#1）
- TB「PE 额外提供白标」→ **域名级白标在 CE**，仅品牌级缺失（#2）
- TB「PE 额外提供解决方案模板」→ **安装框架在 CE**（#3）
- TB「PE 额外提供 2FA」→ **CE 有完整 MFA**（#4）
- TP「社区版无大屏」→ **CE 有**（#6）
- TP「社区版无产品管理 OTA」→ **CE 有**（#7）
- TP「社区版不支持白标」→ **CE 有**（#8）
- TP「社区版无 Redis 实时数据」→ **CE 有**（#9）

另需复核 §7 中由这些判断推导出的差距项（尤其 TP-2 大屏、TB-5 SSO/集成）是否还成立。
