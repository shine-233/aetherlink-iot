# Broker Server 包说明

## 目录职责

`mqtt-broker/server` 承载 GMQTT broker 的核心运行时，负责监听器生命周期、客户端连接、会话恢复、订阅与发布、离线队列、保留消息、topic alias、插件钩子和停机清理。

## 关键文件

- `server.go`：broker 核心状态与服务门面，当前主要保留 `server` 主结构、少量共享装配状态和对外服务方法。
- `server_bootstrap.go`：broker 轻量启动装配入口，负责默认实例构造、选项应用、plugin hook wrapper 安装和整体初始化顺序。
- `server_persistence_bootstrap.go`：broker 持久化恢复装配模块，集中 persistence 打开、subscription/session store 初始化、离线 queue/unack 恢复和 topic alias manager 初始化。
- `api_registrar_bootstrap.go`：API registrar 初始化入口，负责按配置装配 HTTP/gRPC server 包装并挂到统一 registrar。
- `api_registrar.go`：监听器与 API 注册装配模块，集中 HTTP、gRPC、gateway/TLS 与 listener 注册。
- `api_transport_builders.go`：API 传输层 builder 模块，集中 endpoint 解析、TLS 配置加载、gRPC server 构造和 HTTP gateway server 构造。
- `server_runtime.go`：服务端运行时入口，负责 TCP/WebSocket 接入、websocket adapter、client 装配、runtime error、`Run` 和 `Stop`。
- `server_delivery.go`：服务端消息投递模块，维护默认订阅迭代参数、delivery mode、离线队列入队准备、共享订阅选择器、onlyOnce/overlap 策略和 `deliverMessage`。
- `server_session_lifecycle.go`：客户端 session 生命周期模块，维护重复 ClientID 接管、session 恢复/新建、will message、离线 session、清理聚合错误和过期检查。
- `client_service.go`：客户端会话读控适配层，集中 `ClientService` 的 session 遍历、在线客户端读取和 session 终止语义。
- `client.go`：broker 对插件暴露的客户端契约壳，当前只保留 `Connecting/Connected` 常量和 `Client` 接口。
- `client_options.go`：单客户端连接协商完成后的运行时选项结构，承接 ClientID、能力协商、包大小、topic alias 与 MQTT v5 相关配置。
- `client_connect.go`：客户端连接握手模块，维护 CONNECT/AUTH 超时、普通认证、MQTT v5 增强认证、协商参数合并和 CONNACK 写出。
- `client_control.go`：客户端控制包模块，维护 PINGREQ、MQTT v5 re-auth、主动 DISCONNECT、session expiry 更新和 will message 抑制。
- `client_inflight.go`：客户端 inflight 与离线队列轮询模块，维护 packet id limiter 初始化、inflight 重放、Message Expiry 剩余时间计算和新消息投递。
- `client_lifecycle.go`：客户端连接生命周期模块，维护 `serve`、`readHandle`、读写 worker 启停、关闭 hook 和队列/packet limiter 清理。
- `client_packet_io.go`：客户端 packet I/O 模块，维护 bufio 池、read/write loop、topic alias、quota、包日志、出站统计和入站最大包校验。
- `client_outgoing_publish.go`：客户端出站 PUBLISH 后处理模块，维护 topic alias、OnDelivered hook 和消息发送统计的既有顺序。
- `client_protocol_helpers.go`：客户端协议转换 helper，维护 MQTT v5 布尔/数值属性转换、错误属性回写和普通 error 到 reason code 的转换。
- `client_identity_helpers.go`：客户端身份 helper，维护空 ClientID 场景下的内部 ID 生成、主机指纹和递增计数。
- `publish_service.go`、`service.go`、`queue_notifier.go`：消息发布、服务接口和队列通知的协作层。
- `plugin.go`、`hook.go`、`options.go`：插件扩展点、hook 包装和 broker 启动配置。
- `persistence.go`、`topic_alias.go`、`stats.go`、`limiter.go`：持久化、topic alias、统计和限流等支撑能力。

## 文件关系

- `server_bootstrap.go` 负责初始化顺序与 plugin hook wrapper 安装，`server_persistence_bootstrap.go` 负责 persistence 打开、session store 枚举与离线状态恢复，`api_registrar_bootstrap.go` 负责 API registrar 初始化，`api_registrar.go` 保留注册器接口、server 包装结构和 API 服务循环，`server.go` 保留核心状态与对外门面。
- `api_transport_builders.go` 为 `api_registrar_bootstrap.go` 提供 HTTP/gRPC/TLS builder，避免 transport 构造重新堆回注册器主文件。
- `server_runtime.go` 负责 listener 运行、WebSocket 适配、client 实例装配和停机清理。
- `server_delivery.go` 通过 `srv.subscriptionsDB` 读取订阅匹配结果，并在调用方持有 `srv.mu` 的前提下写入 `queueStore`；`publish_service.go`、client 注入 wrapper 和 will 发送路径都继续复用 `deliverMessage`。
- `server_session_lifecycle.go` 承载 session/will/offline 的实现；`server_runtime.go` 保持 `newClient` 的 register/unregister 绑定、`eventLoop` 的 `sessionExpireCheck()` 调用和 `Stop` 的 snapshot/wait 逻辑。
- `client_service.go` 承载 `ClientService` 的外部读控语义，保留 `TerminateSession` 的在线/离线双分支和锁顺序；`server.go` 只继续持有 `clientService` 字段与门面返回。
- `client.go` 负责插件可见契约；运行时协商结果已沉到 `client_options.go`，CONNECT 阶段的握手状态机已由 `client_connect.go` 承载，PINGREQ/re-auth/DISCONNECT 已由 `client_control.go` 承载，连接成功后的 inflight replay 和离线队列轮询已由 `client_inflight.go` 承载，生命周期、读写循环、出站 PUBLISH 后处理、协议小工具和身份生成已由 `client_lifecycle.go`、`client_packet_io.go`、`client_outgoing_publish.go`、`client_protocol_helpers.go`、`client_identity_helpers.go` 承载。
- `queue_notifier.go` 与 `stats.go` 共同维护离线队列丢弃、消息统计和插件通知。
- `hook.go` 定义 hook 类型，`plugin.go` 定义插件组合模型，`server_bootstrap.go` 在初始化阶段把 wrapper 安装进运行时。
- `persistence.go` 与 `topic_alias.go` 是外部实现的兼容接口，通常不应随业务需求频繁改签名。

## 代码审查与重构建议

- 问题：`server.go` 和 `client.go` 曾长期过大，网络生命周期、协议状态、插件回调和持久化行为交织在一起。
- 改进方案：已拆出 session 生命周期、delivery handler、connect auth flow、control packet flow、inflight replay、client lifecycle、packet I/O、outgoing publish、protocol helper、identity helper、publish flow、subscribe flow 和 packet ack flow；后续应转向 server/API 装配或更大的前后端业务热点。
- 实施步骤：先补中文文件头和目录文档，再按单一职责抽取纯函数或小协作者；涉及 wire-level 行为前必须先有 focused broker 协议测试。
- 预期效果：降低 MQTT 核心路径维护成本，减少修改插件、session 或投递逻辑时误伤协议兼容性的风险。

## 当前源码审查记录

- `client_connect.go` 已落地为纯搬迁，集中 `connectWithTimeOut`、普通认证、增强认证、认证参数协商、CONNACK 写出和 `clientConnectTimeout`。迁移时保持了超时不回失败 CONNACK、MQTT v3 错误码收敛和成功路径顺序。
- `client_control.go` 已落地为纯搬迁，集中 `pingreqHandler`、`reAuthHandler` 和 `disconnectHandler`。迁移时保持了 re-auth hook 调用、`ContinueAuthentication` 响应、DISCONNECT session expiry 限制和正常断连抑制 will message 的旧语义。
- `client_inflight.go` 已落地为纯搬迁，集中 `newPacketIDLimiter`、`pollInflights`、`messageForDelivery`、`pollNewMessages` 和 `pollMessageHandler`。迁移时保持了 packet id limiter 锁范围、QoS1/QoS2 重放、Message Expiry 剩余时间计算和未使用 packet id 释放语义。
- `client_lifecycle.go` 已承接 `serve`、`readHandle`、`internalClose`、goroutine 启停和 queue/packet limiter 清理，迁移时保持了读循环等待、store close 和 WaitGroup 等待顺序。
- `client_packet_io.go` 已承接 packet I/O、quota、读写日志、入站 PUBLISH 统计和包大小校验，迁移时保持了 `prepareOutgoingPacket -> writePacket -> packetSent` 与 read loop 的既有顺序。
- `client_outgoing_publish.go` 已承接出站 PUBLISH 的 topic alias、OnDelivered hook 和消息发送统计，迁移时保持了 alias 先于 hook、hook 先于统计的旧顺序。
- `client_protocol_helpers.go` 已承接 `bool2Byte`、`convertUint16`、`convertUint32`、`getErrorProperties` 和 `converError`，让错误属性和 MQTT v5 兼容转换不再堆在 `client.go` 中。
- `client_identity_helpers.go` 已承接 `readMachineID`、`getRandomUUID`、`pid/counter/machineID`，让匿名 ClientID 生成逻辑离开客户端状态主文件。
- `client_options.go` 现在是 CONNECT、认证 hook、服务端配置和 MQTT v5 能力协商的汇合点。后续如继续细拆 options，应先用 CONNACK 属性与错误码用例锁定行为。
- `client.go` 的 publish、subscribe、unsubscribe、ack、connect、control、inflight、lifecycle、packet I/O、outgoing publish、protocol helper、identity helper 和 `ClientOptions` 已拆出；`clientService` 也已独立到 `client_service.go`。下一步更适合转向 `api_registrar.go` 或 `server_bootstrap.go` 装配，而不是重复拆已迁出的协议 flow。
- `server_session_lifecycle.go` 已落地为纯搬迁，集中 `registerClient`、重复 ClientID 接管、session 恢复/新建、will message、离线 session、session 过期清理和 `sessionExpireCheck`。迁移时保持了 `srv.mu` 持有范围，尤其未在 `deliverMessage` 外再套新的锁。
- `server_session_lifecycle.go` 承接 will 延迟发送与重复 ClientID 接管等协议兼容边界。当前已在源码中标明锁顺序和 MQTT-3.1.3-9 约束，后续除非有 focused broker 用例，否则不要改信号和定时器逻辑。
- `server_delivery.go` 已落地为低风险纯搬迁，集中 `deliverHandler`、`addMsgToQueueLocked`、消息过期时间计算和共享订阅随机/TopicHash 均衡策略。后续如要改策略，必须先补 shared subscription focused broker 用例。
- `server_runtime.go` 已落地为纯搬迁，集中 TCP/WebSocket accept loop、websocket adapter、`newClient`、runtime error、`Run` 和 `Stop`。迁移时保持了 `deliverMessage` wrapper 的锁范围、WebSocket 同步 `client.serve()` 行为和 Stop snapshot/wait 顺序。
- `client_service.go` 已落地为低风险纯搬迁，集中 `IterateSession`、`IterateClient`、`GetClient`、`GetSession` 与 `TerminateSession`；迁移时保持了 `srv.mu` 锁持有范围、`forceRemoveSession` 置位顺序和离线 session 终止日志语义。
- `api_transport_builders.go` 已落地为低风险纯搬迁，集中 `splitEndpoint`、`buildTLSConfig`、`buildGRPCServer` 和 `buildHTTPServer`；迁移时保持了 TLS listener 包装、gRPC interceptor 顺序和 HTTP/gRPC serve 启动语义。
- `api_registrar_bootstrap.go` 已落地为低风险纯搬迁，集中 `initAPIRegistrar` 的配置遍历与 registrar 赋值时机；迁移时保持了 HTTP/gRPC 装配顺序与错误返回路径。
- `server_persistence_bootstrap.go` 已落地为低风险纯搬迁，集中 `initPersistence`、`initSessionStores`、`restoreOfflineSessionState` 与 `initTopicAliasManager`；迁移时保持了 persistence 打开、session 枚举、离线 queue/unack 恢复和 topic alias manager 初始化的既有顺序。

## 已识别的优先拆分点

- `server.go`：delivery handler 已拆到 `server_delivery.go`，session 生命周期已拆到 `server_session_lifecycle.go`，runtime/listener 已拆到 `server_runtime.go`，`clientService` 也已下沉到 `client_service.go`；下一步更适合继续压缩 API/统计边界，而不是再把协议/会话逻辑拉回主门面。
- `api_registrar.go`：transport builder 已拆到 `api_transport_builders.go`，初始化入口已拆到 `api_registrar_bootstrap.go`；下一步更适合继续压缩 HTTP handler 注册、gRPC 注册和 API 服务循环说明，而不是把 TLS/serve builder 拉回主文件。
- `server_bootstrap.go`：当前已进一步缩成初始化顺序壳，persistence 恢复链已落到 `server_persistence_bootstrap.go`；后续如继续细化，应优先把阶段编排说明做清，而不是打乱现有启动顺序。
- `client.go`：目前只保留契约壳，`ClientOptions` 已独立到 `client_options.go`；broker 后续重构重点应转向 `server.go` / `api_registrar.go`，而不是继续追着 `client.go` 做无效切片。
- `stats.go`：可按 packet/message/session 统计拆成小协作者。
- `queue_notifier.go`：可把日志、统计、drop hook 三步封装成更清晰的 dropped message helper。
- `api_registrar.go`：可按 HTTP、gRPC、gateway/TLS 三块拆分初始化逻辑。

## 维护提示

- 插件名称、配置 key、MQTT 错误码、QoS 状态流和 topic alias 都属于兼容边界。
- 不要在没有专门验证的情况下改变 CONNECT/PUBLISH/SUBSCRIBE/DISCONNECT 的返回码、包顺序或断连策略。
- 生成文件、mock 文件和测试文件只作为支撑资产；当前真实协议实现应优先从 `server_runtime.go`、`server_session_lifecycle.go`、`server_delivery.go`、`client_connect.go`、`client_control.go` 与 `client_lifecycle.go` 阅读，`server.go` / `client.go` 更适合用于理解门面与契约。
