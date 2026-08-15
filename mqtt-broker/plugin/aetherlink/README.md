# AetherLink Broker 插件

`mqtt-broker/plugin/aetherlink` 是 AetherLink IoT 专用的 GMQTT 插件层，负责把 broker 协议事件连接到后端设备身份、ACL 校验、设备在线/离线状态、Topic 映射、内部 MQTT 转发、Redis/PostgreSQL 状态和设备调试日志。

## 目录职责

- `plugin.go`：注册 `aetherlink` 插件，读取 `aetherlink.yml`，初始化数据库/Redis，并启动内部 MQTT 客户端和设备会话撤销 monitor。
- `hooks.go`：保留 GMQTT 认证、订阅 ACL、消息到达、连接和关闭 hook 的稳定适配面。
- `hooks_auth.go`、`hooks_lifecycle.go`、`hooks_subscribe.go`、`hooks_messages.go`：分别承接认证、生命周期、订阅授权和消息路由行为。
- `db.go`：承接设备 voucher、设备状态、缓存 key 和设备元数据相关 Redis/PostgreSQL 访问。
- `mqtt.go`：管理内部 MQTT 客户端、发送队列、重连和停机。
- `mqtt_session_revocation.go`：在 broker 迭代在线连接时使用进程内认证绑定筛出目标设备，释放迭代锁后调用原子 `TerminateClientIfCurrent`，避免访问 Redis 时持有 broker 锁，也避免同 client ID 重连后误断替代连接。
- `mqtt_session_revocation_redis.go`：把共享 Redis Pub/Sub 控制 channel 适配成会话撤销消息流；必须使用本仓库实际依赖的 `gopkg.in/redis.v5` API。
- `topicmap_*.go`：管理 Topic 映射规则、缓存、匹配和服务层转换。
- `devdebug*.go`：记录设备侧调试证据。
- `util/`：保存小型插件 helper。

## 运行配置

插件要求在 broker 运行配置旁提供环境专属 `aetherlink.yml`。数据库、Redis、MQTT root/plugin 密码、证书和部署地址不得以真实生产值提交到 Git。

仓库提供 `cmd/gmqttd/aetherlink.example.yml` 作为无秘密结构模板。根目录 Compose 会把它只读挂载为 `/gmqttd/aetherlink.yml`，并通过 `GMQTT_*` 环境变量覆盖数据库、Redis、MQTT 密码和稳定 `broker_id`；不要把真实密码写回模板。手工部署必须复制模板为运行目录的 `aetherlink.yml`，并显式填写所有 `CHANGE_ME` 值及非空 broker ID。

每个 broker 实例还必须在 `aetherlink.yml` 中配置稳定且唯一的会话撤销身份：

```yaml
mqtt_session_revocations:
  broker_id: broker-01
```

`broker_id` 会去除首尾空白，长度不得超过 128，只允许字母、数字、`.`、`_`、`:` 和 `-`。缺失或非法时插件启动直接失败；不会回退到 hostname，也不会生成随机身份。多 broker 部署应使用由部署系统稳定分配的实例名，并确保不同并行实例不复用同一值。

broker 默认配置按名称启用 `aetherlink` 插件。插件名称、配置 key、认证凭据字段和 Topic 映射语义都属于外部兼容合同，修改前必须同步评估后端、设备和自动化用例。

## 订阅授权安全合同

- 设备订阅标准下行 Topic 时，授权不能只校验 Topic 形状；Topic 中的 `{device_number}` 必须与当前连接已认证并加载的 active device 的 `DeviceNumber` 完全一致。空设备编号、通配符或其它设备编号必须拒绝，防止跨设备订阅。
- `devices/register/response/+`、`devices/config/down/response/+` 等没有可与当前认证设备明确绑定的 identity slot 的宽泛 gateway response Topic 必须 fail-closed；在协议提供可验证的设备身份绑定前，不得仅凭形状匹配放行。
- `root`、`plugin` 等系统用户继续沿用既有授权旁路，不受上述设备身份绑定规则影响。
- 自定义 Topic mapping 继续沿用既有匹配与授权合同；标准 Topic 的形状匹配不得成为绕过自定义 mapping 授权路径的放行条件。
- 该边界落实 OASIS MQTT 5.0 对认证、授权与安全通信的建议，以及 NISTIR 8259A 关于设备身份和逻辑接口访问控制的基线：[MQTT Version 5.0](https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html)、[NISTIR 8259A](https://csrc.nist.gov/pubs/ir/8259/a/final)。这些来源支持按认证身份授权，但不替代本仓库的 Topic 级负向测试。

## 审查与重构计划

- 当前 hook 已按职责拆分；`hooks.go` 只保留 GMQTT adapter surface，具体行为分别由认证、生命周期、订阅、消息和调试文件承接。
- 后续继续拆分时，应保持认证、订阅 ACL、消息转发、生命周期状态和调试日志行为不变，优先先用聚焦测试锁定外部合同，再抽取纯 helper。
- 当前数据库、`redisCache`、`DefaultMqttClient` 和 `runtimeInitOnce` 仍是全局依赖，插件 reload 与单元测试隔离较难。后续可引入拥有配置、数据库、Redis、日志和内部 MQTT 客户端的 runtime struct，但必须保持公开插件注册和部署配置兼容。

## SW3 会话撤销合同

- 设备认证以及每次新的 publish/subscribe 授权都必须要求 `activate_flag=active`、`is_enabled=enabled` 且租户绑定非空。
- 物理 SW3 解绑在同一 PostgreSQL 事务中提交设备失效、分组删除和 `mqtt_session_revocation_outbox` 事件，提交后清理 voucher/device 缓存并立即尝试向固定 channel `aetherlink:mqtt:device-session:terminate` 发布；失败或零订阅者由后端 worker 按租约重试。
- 新 outbox 消息使用必须同时包含非空 `event_id`、非空 `device_id` 和有效 `revoked_at` 的 v1 JSON 信封；monitor 仍兼容旧的纯设备 ID 消息。连接对象级认证绑定会保存认证时读取到的设备 `update_at` 版本，旧事件重复投递只终止版本不晚于 `revoked_at` 的连接，不能误断重新绑定后认证的新会话。
- broker monitor 用连接对象级进程内认证绑定筛选目标设备，并在退出 `IterateClient` 回调后通过 `TerminateClientIfCurrent` 原子终止仍为当前连接的 session。
- v1 命令的本机终止调用返回后，monitor 会向 `aetherlink:mqtt:device-session:terminate:ack` 发布处理 ACK。ACK 固定包含 `version`、`event_id`、`device_id`、`revoked_at`、`broker_id`、`status=processed`、`processed_at` 和 `terminated_sessions`；本机匹配会话数为 `0` 也表示该 broker 已完成扫描和处理，必须发送成功 ACK。
- 旧纯设备 ID 消息没有 `event_id`，只用于滚动升级兼容，绝不发送 ACK。ACK 发布失败或当时没有后端订阅者时只记录告警，不在 broker 内部忙重试；后端因缺少该 broker ACK 而重新投递 v1 命令，重复终止由 `revoked_at` 和当前连接原子检查保持安全。
- 后续认证、publish、subscribe 的失效设备拒绝仍是第二道防线，不能因为已有主动断连就删除。
- Redis Pub/Sub 是即时控制信号，不是持久队列；命令发布的 subscriber count 只表示 Redis 当时看到的订阅连接数，不能作为期望 ACK 数或处理证明。插件加载会先等待 SUBSCRIBE 确认；依赖 `gopkg.in/redis.v5` 的 `ReceiveMessage()` 会对网络错误自动重连并重新订阅，其它接收错误会记录结构化日志并退避重试。后端必须按稳定 `broker_id` 跟踪 processing ACK，生产验收仍必须覆盖 Redis 中断/恢复、ACK 丢失后的重投、monitor 日志/健康、真实连接断开和重新 claim，才能描述为完整可靠。
- 后端 ACK consumer 使用 `github.com/redis/go-redis/v9` 的 `PubSub.Channel()`；该实现带健康检查、网络重连和重订阅，临时 Redis 断开不会因为业务层缺少手写 subscribe loop 就永久停掉 consumer。channel 关闭仍会结束 worker，因为依赖只会在 PubSub/Redis client 被显式关闭时关闭该 channel；真实中断恢复仍需部署环境验证。
- 当前切片只提供显式稳定 `broker_id` 和逐条处理 ACK，不包含动态 broker 成员发现、租约、进程代际 fencing 或 self-fence。后端必须从部署配置维护期望 `broker_id` 集合，不能从 Redis subscriber count 推导；动态成员租约属于后续独立可靠性工作。

## 维护注意事项

- 不要在 `IterateClient` 回调内访问 Redis、调用 `TerminateSession` 或执行任何会重新获取 broker 全局锁的操作。
- 不要手工编辑相邻插件目录中的 protobuf 生成文件或 `*_mock.go`；接口变化应通过既有生成流程更新。
- DB/Redis/query、认证、ACL、Topic 映射或会话撤销变化都应配套最窄的 hook/topic-map/session focused tests，并明确记录未运行的完整 broker、真实 Redis 和真机验证。
- 当前用户明确要求停止构建和编译，因此最新事务、零订阅者门禁、SUBSCRIBE 确认/重试、进程内绑定、原子断连和安全关闭硬化仅完成源码静态复核，不能沿用更早测试结果宣称当前工作树已全量通过。
