# 后端数据访问层说明

`backend/internal/dal` 是后端的数据访问层，负责把业务域需要的持久化操作收敛成稳定、可复用的访问边界。这里不直接承载产品权限判断，也不面向前端组织 API 响应，而是专注于数据库读写、事务、条件拼装、模型转换以及少量与存储语义强绑定的聚合逻辑。

这个目录的存在意义，是让 `service` 层可以围绕“业务动作”编排流程，而不是在每个服务函数里重复拼接查询、维护事务和处理 JSON 字段。对于准备上传 GitHub 的项目而言，这里也是最需要明确“边界”和“约束”的位置，因为一旦数据访问层职责失控，后续设备、告警、自动化和遥测模块都会变得难以维护。

## 目录职责

- 封装 `GORM Gen` 生成查询对象与手写查询逻辑之间的衔接。
- 统一管理事务开启、提交、回滚等数据库原子操作入口。
- 为设备、告警、遥测、自动化、通知、用户与权限等业务域提供聚焦的数据访问 helper。
- 在必要处承担与数据库结构强相关的过滤组合、分页拼装、聚合窗口计算和 JSON 字段合并。
- 为 `service` 层提供“可被编排”的持久化接口，减少上层直接感知表结构和 join 细节。

## 典型依赖关系

典型调用方向如下：

`api handler -> service -> dal -> query/model/global.DB`

其中：

- `service` 层负责租户边界、权限校验、业务前置判断、缓存失效时机和跨模块编排。
- `dal` 层负责把这些业务动作落到具体表、事务和查询条件上。
- `query` 与 `model` 目录提供生成代码和实体模型，是本目录的基础依赖。
- `global.DB`、远端时序查询客户端、日志组件等基础设施由 DAL 内部按需接入。

## 目录内文件关系

这个目录文件较多，建议按业务域理解，而不是逐个文件孤立阅读。

### 事务与公共基础

- `common.go`
  - 提供事务启动、提交、回滚等基础能力，是多表写入路径的公共底座。
- `action_info.go`、`operation_logs.go`
  - 负责操作留痕、动作记录等通用持久化访问。

### 设备域

- `devices.go`
  - 设备创建、批量写入、默认分组绑定和设备主流程写入逻辑。
- `device_identity_queries.go`
  - 集中设备编号与凭证的唯一性预检；保留全局精确匹配、空批量返回非 nil map 和排除当前设备语义，不替代数据库唯一约束。
- `device_protocol_plugin.go`
  - 协议插件直连设备列表查询，集中协议标识过滤、分页读取、协议配置 JSON 解析和返回模型转换。
- `device_selector.go`
  - 设备选择器分页查询，集中租户过滤、设备配置存在性筛选、名称搜索、更新时间排序和分页。
- `device_service_access_queries.go`
  - 集中服务接入点关联设备的读取、计数、编号筛选和批量分组；保持原导出函数与查询契约，由 service 层继续负责接入点的租户和权限前置校验。
- `device_auth.go`
  - 设备认证信息相关访问。
- `device_config.go`
  - 设备配置、配置查询和与产品/模板绑定相关的持久化逻辑。
- `device_groups.go`、`r_group_device.go`
  - 设备分组及设备与分组关系维护。
- `device_model*.go`
  - 产品模型、属性、遥测、自定义命令、自定义控制等模型定义访问。
- `device_status_history.go`
  - 设备上下线与状态历史留痕。
- `device_topic_mapping.go`
  - 设备主题与协议映射关系，通常和上行/下行消息编排有关。
- `device_template.go`
  - 模板类设备定义持久化。

### 遥测与属性域

- `telemetry_datas.go`
  - 原始遥测、历史分页、聚合统计、差值窗口和批量写入，是本目录最复杂的热点之一。
- `telemetry_current_datas.go`
  - 最新遥测值读取与维护。
- `telemetry_data_aggregate.go`
  - 聚合结果相关访问。
- `telemetry_set_logs.go`
  - 遥测下发/设置日志。
- `attribute_datas.go`
  - 属性数据读取与写入。
- `attribute_set_logs.go`
  - 属性设置日志。
- `command_set_logs.go`
  - 命令请求、平台下发和设备响应的持久审计日志；按 device/message ID 读取支持可取消 context。
  - 平台 `sent/send_failed` 更新必须使用排除设备终态 `3/4` 的单条条件更新，不能用读后保存覆盖快速设备回包。
- `expected_datas.go`
  - 期望值/目标值相关持久化。
- `event_datas.go`
  - 事件上报历史。

### 告警与通知域

- `alarm.go`
  - 保留告警配置、告警历史查询、确认/重置备注、事务更新和告警名称缓存主流程。
- `alarm_history_devices.go`
  - 集中告警历史设备 ID 展开、当前活动告警设备过滤和设备摘要装配；保持原有租户/owner 条件、SQL 与响应字段契约，避免设备后处理继续挤在查询主文件中。
- `latest_device_alarm.go`
  - 设备最近告警视图或快捷查询。
- `message_push.go`
  - 推送相关的数据访问。
- `notification_groups.go`
  - 通知组定义。
- `notification_history.go`
  - 通知发送历史及通知-设备关联查询/写入。
  - `CreateNotificationHistory` 会去空白、去重设备 ID，在同一事务中写入历史与关联，并先确认所有设备都属于历史记录的租户；任何越界 ID 都使整个写入失败。
- `notification_services_config.go`
  - 第三方通知服务配置。

### 自动化与场景域

- `scene.go`
  - 场景定义相关访问。
- `scene_automations.go`
  - 保留场景自动化主定义 CRUD、启停状态检查和租户读取。
- `scene_automation_list_queries.go`
  - 集中名称、设备和设备配置筛选、分页排序，以及设备/配置到场景 ID 的解析；保持现有租户过滤、空结果与错误处理契约。
- `scene_automation_log.go`
  - 场景自动化执行日志。
- `scene_log.go`
  - 场景执行记录。
- `device_trigger_condition.go`
  - 设备触发条件定义，常被自动化流程复用。
- `one_time_tasks.go`、`periodic_tasks.go`
  - 一次性任务与周期任务。

### 系统配置与平台支撑域

- `users.go`、`roles.go`
  - 用户与角色。
- `data_policy.go`、`data_script.go`
  - 数据策略与数据脚本。
- `service_access.go`、`service_plugin.go`
  - 服务接入、插件或扩展能力配置。
- `dict.go`、`dict_language.go`
  - 字典与国际化字典项。
- `board.go`、`default_board.go`、`dashboard_menu.go`、`ui_elements.go`
  - 看板、菜单与 UI 元素元数据。
- `ota_upgrade_packages.go`、`ota_upgrade_tasks.go`
  - OTA 升级包与升级任务。
- `open_api_keys.go`
  - OpenAPI 密钥管理。
- `logo.go`、`sys_function.go`
  - 平台配置类元数据。

## 典型调用链

### 设备创建链路

典型路径通常是：

`api -> service/device -> dal/devices -> transaction + device/group relation`

关键点：

- service 层先做租户、产品、权限、参数完整性校验。
- `devices.go` 负责真正落库，并在同一事务中处理默认根分组绑定等副作用。
- 如果未来再叠加设备初始化属性、默认模板继承等逻辑，优先继续放在 service 编排，不要把业务判断反向塞回 DAL。

### 遥测历史查询链路

典型路径通常是：

`api -> service/telemetry -> dal/telemetry_datas -> local SQL or remote query client`

关键点：

- 遥测查询可能同时存在本地数据库路径和远端时序查询客户端路径。
- 时间范围、分页语义、聚合窗口、差值边界必须保持一致，否则前端图表和导出结果容易不一致。
- 这一块是最需要补 focused 用例的区域之一。

### 告警确认与历史查询链路

典型路径通常是：

`api -> service/alarm -> dal/alarm -> alarm history + remark JSON merge + device expansion`

关键点：

- 告警历史常同时依赖租户、设备、时间、状态和类型筛选。
- `remark` 字段是 JSON 结构，确认、重置、附加说明时必须基于旧值安全合并。
- 告警设备列表需要从 ID 列表展开为设备摘要，避免上层重复做模型组装。

### 通知历史设备关联与迁移约束（本轮）

`backend/sql/29.sql` 建立 `notification_history_devices` 关联表，主键为
`(notification_history_id, device_id)`，并保留 `tenant_id` 作为租户一致性与查询条件。关联表只对历史记录设置
`ON DELETE CASCADE` 外键；它没有对 `devices` 设置级联删除或强制外键，因为设备删除后仍要保留历史审计引用。

- 写入路径会规范化设备 ID（去空白、去重、忽略空值），检查每个 ID 在同一租户的设备表中存在，再与历史主记录同事务提交。租户不匹配、数量不一致或关联插入失败时整个事务回滚。
- owner 范围查询使用“至少存在一台当前 owner 为请求用户的设备”并同时排除任何跨租户、跨 owner、owner 为空或设备行已删除的关联。无关联的旧历史记录也不会因为缺少数据而被普通用户看见，属于 fail-closed；管理员仍可查看租户内完整历史。
- 设备行删除不会级联抹掉 `notification_history_devices` 关联；缺失设备会在 owner 查询中按无效/外部关联处理并对普通用户隐藏。删除历史主记录时，关联行才由历史外键级联清理。
- `29.sql` 同时创建 `idx_notification_history_devices_history_tenant`、`idx_notification_history_devices_device_tenant` 和 `idx_notification_histories_tenant_send_time`，分别支撑历史展开/租户条件、设备 owner 范围和租户时间倒序列表。当前程序 `VERSION_NUMBER` 为 46：`30.sql` 建立 SW3 MQTT 撤销 outbox，`31.sql` 补 `claim_token` fencing，`32.sql` 新增告警邮件模板，`33.sql` 补 broker ACK 快照/明细并重排旧撤销事件，`34.sql` 为 Command Jobs 增加 `scheduled_at` 与到期索引，`35.sql` 按告警流派生设备当前活动告警视图，`36.sql` 增加 Command Job 全局/租户下发配额和 `next_dispatch_at`，`37.sql` 增加 attribute/event 幂等 receipt 与 PostgreSQL dead-letter 表，`38.sql` 增加 OTA rollout governance 字段与 dispatch lease，`39.sql` 为 attribute/event dead-letter 增加 claim token、过期 lease 回收和 fencing CAS，`40.sql` 增加告警触发持续时间，`41.sql` 增加 payload schema，`42.sql` 增加保存筛选器共享标志，`43.sql` 扩展告警备注并重建摘要视图，`44.sql` 兼容历史告警 JSON 形态，`45.sql` 注册隐藏的 Command Center 交接路由，`46.sql` 为设备配置增加可选 payload schema 绑定。不要覆盖或复用已经分配的迁移编号。
- 这些是源码与迁移脚本的静态约束。本轮没有执行真实 PostgreSQL 迁移或双账号 API 验证，索引命中、历史数据修复和删除后的查询行为仍需部署环境验收。

### 场景自动化定义读写链路

典型路径通常是：

`api -> service/scene_automations -> dal/scene_automations -> definition tables + trigger/action relations`

关键点：

- 自动化定义通常涉及主记录、触发组、条件和动作的多表协同更新。
- 更新操作必须保证事务原子性，避免只更新了一部分定义。
- 触发条件与动作写入逻辑宜继续拆成可单测的 helper，减少未来改动时的回归面。

## 静态审查重点

### 1. 租户与权限边界是否真的前置

这里的查询函数很多默认假设“上层已经完成权限和租户校验”。这一约束如果文档化不足，很容易在后续新增接口时被绕开，造成：

- 查询结果跨租户外扩。
- 管理员与普通用户看到的数据范围不一致。
- service 层和 DAL 层对“谁负责过滤”理解不一致。

建议：

- 所有新增 DAL 方法在命名或注释中明确入参是否要求携带 `tenantID`。
- 对高风险查询统一建立“租户条件必须出现”的审查清单。

### 2. 复杂查询函数承担职责过多

从 `alarm.go`、`telemetry_datas.go`、`devices.go`、`scene_automations.go` 等文件可以看出，部分函数同时承担了：

- 条件拼装
- JSON 解析/合并
- 事务控制
- 结果二次加工
- 兼容旧数据结构

这会带来两个问题：

- 改一个筛选条件，容易影响最终返回结构。
- 很难只针对某个边界补最小测试。

建议：

- 优先提炼“纯条件拼装 helper”“纯 JSON 合并 helper”“纯模型映射 helper”。
- 保持外部函数签名稳定，先做内部拆分，再考虑进一步分文件。

### 3. 遥测双路径语义一致性风险

`telemetry_datas.go` 同时对接本地查询和远端时序查询客户端，这是典型的维护高风险点。

风险主要在于：

- 分页总数和结果条数语义不一致。
- 时间窗口开闭区间不一致。
- 聚合粒度与差值窗口边界不一致。
- JSON 解码失败或远端结构变化时，错误语义不统一。

建议：

- 抽取统一的时间范围与分页语义说明。
- 将远端响应解码独立成 helper，避免散落在多个分支里。
- README、文件头和函数注释中同步强调“双路径一致性”是修改前必查项。

### 4. 多表事务链路缺少显式阶段说明

设备创建、自动化更新、通知配置调整等路径都可能涉及多表写入。如果事务阶段不清晰，后续维护者很难判断：

- 哪一步失败允许回滚。
- 哪一步失败只能记录日志。
- 是否存在缓存失效和数据库提交顺序倒置。

建议：

- 在复杂写入路径旁补“事务阶段说明”注释。
- 如果函数超过一个屏幕仍在做多表协调，应优先拆出阶段 helper。

## 重构建议

### 第一优先级：拆热点，不改外部契约

优先处理：

- `telemetry_datas.go`
- `alarm.go`
- `devices.go`
- `device_protocol_plugin.go`
- `scene_automations.go`

实施方向：

- 把复杂筛选与结果后处理拆成纯 helper。
- 把 JSON 合并与结构转换从查询主干中拆离；协议插件设备列表已先迁入 `device_protocol_plugin.go`，设备选择器查询已迁入 `device_selector.go`。
- 保持 service 层调用方式不变，先做“内部可读性和可测性提升”。

预期效果：

- 减少回归风险。
- 更容易补 focused DAL 用例。
- 新维护者更容易定位问题落点。

### 第二优先级：按业务域继续细分文档

当前目录体量很大，后续建议继续补子域文档或在主 README 中保持索引：

- 设备域数据访问说明
- 遥测/属性域数据访问说明
- 告警与通知域数据访问说明
- 自动化与场景域数据访问说明

预期效果：

- 降低首次阅读成本。
- 让评审者更快找到关注文件。

### 第三优先级：统一命名与注释风格

目前部分文件已经有较完整中文文件头，但个别函数注释仍偏英文遗留或“只描述是什么、不描述为什么”。

建议：

- 统一采用“文件用途 / 核心逻辑 / 关键注意事项 / 重构建议”的文件头结构。
- 关键函数增加“对上游承诺什么、对下游依赖什么、失败时要注意什么”的说明。

## 维护注意事项

- 不要手工修改生成的 `query`、`model` 相关代码来绕过本目录逻辑，应该在 DAL 中补手写 helper。
- 新增查询前先确认租户、权限和范围过滤究竟归谁负责，避免边界混乱。
- 修改分页、聚合、统计逻辑时，必须同时检查前端展示语义是否依赖旧行为。
- 改动 JSON 字段读写时，要确认老数据兼容、空值处理和失败日志是否足够明确。
- 涉及事务的写入路径，要先明确哪些副作用必须与主记录同事务提交，哪些适合延后处理。
- 如果后续补测试，优先为热点文件增加 focused DAL 用例，而不是只靠上层集成路径兜底。

## 建议阅读顺序

第一次接手这个目录时，建议按下面顺序阅读：

1. `common.go`
2. `devices.go`
3. `device_protocol_plugin.go`
4. `device_selector.go`
5. `telemetry_datas.go`
6. `alarm.go`
7. `scene_automations.go`
8. 与当前任务直接相关的业务域文件

这样更容易先建立“事务 -> 设备主链路 -> 遥测热点 -> 告警热点 -> 自动化热点”的整体认知，再进入局部细节。
