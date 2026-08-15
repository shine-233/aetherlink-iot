# 后端 Service 层

`backend/internal/service` 负责 AetherLink IoT 后端的大部分业务编排，是连接 API 入口、数据访问层、MQTT/设备链路、通知、自动化、看板和外部服务的核心层。

这里的文件大多不是简单的 CRUD 封装，而是同时承载权限校验、租户边界、状态转换、副作用触发和多模块协作，因此也是最需要中文注释、静态审查和后续聚焦验证的区域之一。

## 目录职责

- 承接 `api/` 传入的业务请求，完成参数后的业务校验和权限判断。
- 协调 `dal/`、MQTT/设备管线、缓存、通知、自动化、可视化、OTA 和外部客户端。
- 处理设备生命周期、告警、自动化、共享、遥测、看板、市场服务等用户可见业务流程。

## 重要文件

- `device.go`
  - 设备 service 文件簇的 facade 入口，当前只保留 `Device` 类型本身。
- `device_identity.go`
  - 集中历史 RDI PID / device_number 兼容归一化、设备 ID 基础校验和 RDI PID 格式规则。
- `device_config_assignment.go`
  - 设备实例与设备配置之间的绑定/解绑切换边界，集中 `UpdateDeviceConfig`、当前配置可否切换、目标配置租户授权和兼容解绑语义。
- `automate_telemetry.go`
  - 遥测触发场景执行、条件判断和自动化执行日志的重要路径。
  - 静态阅读建议优先沿 `Execute -> telExecute -> ExecuteRun -> prepareSceneExecution -> AutomateConditionCheck -> AutomateActionExecute` 主链路展开；`ExecuteRun` / 场景执行已拆到 `automate_telemetry_scene_execution.go`，事件参数匹配已拆到 `automate_telemetry_event_param.go`，设备条件求值已拆到 `automate_telemetry_device_condition.go`，动作分发已拆到 `automate_telemetry_action_dispatch.go`。
- `automate_telemetry_device_condition.go`
  - 承接设备类条件的目标解析、实时值读取、事件/状态特殊判断和结果文案拼装，避免入口文件继续夹带大量条件分支细节。
- `scene_automations.go`
  - 自动化场景启停、更新、缓存清理和用户可见业务流。
- `scene_automation_reference_validation.go`
  - 集中保存、更新和预验证共用的设备、设备配置、场景和告警引用校验，避免客户预览与真实保存走不同规则。
- `alarm.go`
  - 告警规则、告警记录和通知相关业务编排。
  - `TENANT_USER` 的告警历史列表与按 ID 入口都按设备 owner 收口：详情读要求至少拥有一台关联设备；描述修改、确认、Reset、删除和批量动作要求拥有全部关联设备；批量预加载路径必须复用同一已加载告警写守卫。
- `notification_history.go`
  - 通知发送历史的查询、写入编排和租户用户响应脱敏。
  - `TENANT_USER` 查询通过持久化的通知-设备关联交给 DAL 做 owner 收口：至少命中一台自己的设备，且所有关联设备都必须仍属于该用户；无关联、跨租户、混合 owner、设备已删除或 owner 无效的记录均 fail closed。`TENANT_ADMIN`/`SYS_ADMIN` 保留租户范围内的审计视图。
  - 普通用户请求会忽略 `send_target` 筛选，并在响应中清空 `send_target`、`send_content`、`remark`，避免收件地址、通知正文和内部备注泄漏，也避免通过分页计数探测收件目标。
- `sys_user_password.go`
  - 密码重置验证码与一次性链接的业务边界；`verify_code` / `reset_token` 必须二选一，Redis 与邮件副作用通过小 adapter 保持生产复用和失败路径可测试性。
- `rdi_share.go`
  - RDI 分享 token、接受、shared-with-me 和精确接收人元数据的事务边界；同租户与跨租户非 owner 都必须登记 recipient 才能获得只读访问，写权限和再次分享保持 owner-only。
- `mqtt_session_revocation.go`
  - RDI 物理解绑后的跨进程会话撤销发布合同；设备失效更新与分组关系删除先在同一事务提交，再清缓存并向固定 Redis channel 发布标准化设备 ID，供 broker 立即终止仍在线的旧账号会话。
- `command_data.go`、`command_direct_method.go`
  - `command_data.go` 统一负责命令模型校验、payload/网关目标解析、日志先行和 downlink publish；共享日志 builder 会在存在已认证操作人时保存 `user_id`。
  - `command_direct_method.go` 是单设备在线请求/响应深模块：要求在线和日志已持久化，复用同一 message ID 等待状态 `3/4`，把下发失败与 1-30 秒超时作为独立结果；取消等待不会撤回命令。Fleet/offline 批量语义继续属于 Command Jobs。
  - 发布调用使用 3 秒截止时间，并把 Redis 返回的订阅者数量为 0 视为失败；发布错误或无订阅者都属于“设备状态已失效、在线会话撤销通知未确认”的部分失败，不能回滚成设备仍有效或记录为完整成功。
  - Redis Pub/Sub 的订阅者数量不是处理回执；真实验收仍要证明 broker 收到消息并实际关闭目标连接。
- `board.go`
  - 看板创建、更新、租户主页看板和设备统计聚合。
- `market_client.go`
  - 外部市场服务接入、URL 兜底和不可用时的错误边界。
  - Market 现在严格显式启用：只有 `market.enabled=true` 且 `market.base_url` 有效时才允许出站请求；配置键缺失、显式关闭或 URL 为空都返回结构化外部不可用错误。超级管理员初始化复用同一 fail-closed 边界，已由定向测试和 `internal/service` 全包测试覆盖。
- `telemetry_*.go`
  - 遥测查询、处理和用户可见数据服务。
  - `telemetry_dead_letters.go` 的 worker drain seam 接收 context，并将取消传播到 claim 查询、原子 claim 更新、逐条 replay 和 GORM 事务；人工 API 路径继续通过 background wrapper 保持原合同。
  - 取消不会被计成一次遥测 replay 失败；已经 claim 为 `processing` 但尚未完成的记录仍由现有 stale-processing 回收合同兜底，真实 PostgreSQL 停机验证仍需确认恢复时延。

### 通知历史的 owner/device 边界（本轮）

- 通知写入通过 `SaveNotificationHistory(req, deviceIDs...)` 传递本次事件涉及的设备 ID，并由 DAL 在同一事务中保存历史记录及关联。没有可验证设备关联的旧记录仍可供管理员审计，但不会被普通租户用户放行。
- 普通租户用户的列表查询只接受“至少一台关联设备属于本人、且不存在任何其他 owner/缺失设备关联”的记录；服务层再对敏感字段做响应级清空。这个边界按设备 owner 判断，不按 `send_target`、发送邮箱或通知类型推断归属。
- 该约束是源码层面的访问控制和脱敏合同，不是本轮真实 PostgreSQL、账号双开、SMTP 或 API 运行验收的替代品；后续验证仍需用实际租户和设备数据证明查询、计数与响应字段行为。

## 当前静态热点

| 文件/区域 | 当前问题 | 改进方向 | 预期效果 |
| --- | --- | --- | --- |
| `device.go` / `device_config_assignment.go` | 设备 create/update/delete/connect/read/voucher 等主路径已拆到独立文件，当前真正敏感的是“设备配置切换”边界和跨写路径副作用一致性。 | 后续优先围绕 `UpdateDeviceConfig`、配置切换策略、权限守卫复用和副作用一致性继续收口，而不是重复描述已经落地的 create/update/delete 拆分。 | 让维护者按“设备 service 文件簇 + 配置切换 seam”而不是按单文件巨块理解风险，减少后续继续拆分时的误判。 |
| `automate_telemetry.go` | 触发入口仍复杂，但设备条件求值、事件参数匹配、场景执行和动作分发已经拆出。 | 继续分离普通 operator 比较、缓存查询和执行入口日志整形。 | 让自动化回归定位更容易。 |
| `scene_automations.go` / `scene_automation_reference_validation.go` | 场景启停、缓存刷新、定义写入和引用校验已开始分开；当前主文件保留编排，引用校验已由预验证和保存共用。 | 下一步优先看缓存刷新/查询租户推导是否还能形成更小 seam，不要把 payload-state 预验证与 form-state UI summary 混在一起。 | 让客户预览、保存和后续执行日志之间的规则解释更一致，减少自动化保存前后行为差。 |
| `board.go`、`market_client.go` | 已做一轮减负，但仍有统计复用和外部服务契约收敛空间。 | 继续围绕低风险 helper 抽取和中文说明补齐推进。 | 提升可读性并减少未来维护成本。 |

## 本轮已确认的代表性整理

- `automate_telemetry.go`
  - 降低高频正常路径日志噪音，保留错误与执行失败信息。
  - 补齐文件头、执行主链路、条件判断和事件参数匹配的中文静态说明。
  - 新增 `automate_telemetry_event_param.go` 和
    `automate_telemetry_scene_execution.go`，分别承载事件参数匹配与场景执行链路。
  - 新增 `automate_telemetry_action_dispatch.go`，集中动作类型分发、动作结果汇总和不支持动作的失败边界。
  - 新增 `automate_telemetry_device_condition.go`，集中设备目标解析、触发值读取、事件/状态条件求值和结果文案拼装。
- `scene_automations.go`
  - 补齐中文说明，提炼场景自动化缓存清理 helper。
- `scene_automation_reference_validation.go`
  - 从 `scene_automations.go` 拆出 trigger/action 引用校验，保留 `validateSceneAutomationReferences(...)` 作为保存、更新和预验证的共同入口。
- 本目录下高优先级真实源码文件
  - 中文文件头应优先覆盖自动化、设备注册、看板、共享、消息推送、期望数据等直接承载业务边界的 service 文件。
  - 关键注释优先解释租户权限、缓存副作用、状态推进和兼容旧接口的原因，而不是重复代码表面语义。
- `device.go` / `device_identity.go`
  - `device.go` 已不再承载主业务入口，只保留 `Device` facade。
  - `device_identity.go` 集中共享兼容归一化与基础校验 helper，供 create/update/activation/RDI 路径复用。
- `device_config_assignment.go`
  - 已把 `UpdateDeviceConfig`、当前配置可否切换、目标配置租户授权和兼容解绑语义收口到独立文件。
  - 已显式记录 `DeviceConfigID == ""` 属于兼容旧输入的解绑语义，避免后续在新逻辑里继续扩散。
  - 已恢复多处被坏注释吞到同一行的缓存清理语句，避免设备更新、删除、解绑、配置切换和凭证更新后的缓存继续保留旧状态。
- `board.go`
  - 收敛旧式作者注释，复用时间戳并补足中文边界说明。
- `market_client.go`
  - 取消把占位域名当默认可用地址，明确未配置市场服务时的行为边界。

## 审查与重构建议

### 发现的问题

- 大文件常常同时承载授权、持久化、外部调用和数据转换，多职责混杂。
- 一些改动点会直接影响前端可见行为、自动化断言或设备链路侧效果。
- 部分历史兼容逻辑存在，但缺少明确的“为什么还保留”的说明。

### 改进方案

- 先在文件头和关键函数周围补足中文边界说明，再做小步 helper 抽取。
- 对权限判断、失败路径、副作用触发和客户端可见文案建立单独验证清单。
- 密码重置、邮件和缓存链路优先通过可替换 adapter 验证失败边界，避免测试直接依赖真实 SMTP 或 Redis。
- 能拆成纯函数的先拆纯函数，能拆成本地 helper 的先拆本地 helper，避免直接引入大规模结构重写。
- “设备配置切换”与“设备配置实体编辑”要分开维护：前者校验设备当前状态、租户归属和解绑语义，后者才处理配置实体字段、物模型解绑、缓存和协议副作用。

### 建议实施步骤

1. 优先处理真实业务热点文件，而不是先扩张测试辅助目录。
2. 每处理一批 service 文件，同步更新目录 README 和当前状态文档。
3. 静态批次收敛后，再统一跑更窄但更强的行为验证。

### 预期效果

- Service 层职责边界更清晰，后续新增业务逻辑时更容易判断落点。
- 权限、自动化、告警、设备等高风险改动的审查成本更低。
- GitHub 维护者能更快理解哪些文件是高风险业务面、哪些只是辅助层。
