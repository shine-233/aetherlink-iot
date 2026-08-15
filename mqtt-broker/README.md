# AetherLink IoT MQTT Broker

`mqtt-broker` 是 AetherLink IoT 的 MQTT 接入网关，基于 GMQTT 演进，负责设备连接、认证鉴权、Topic 映射、上下行路由、调试日志、管理 API 与可观测性扩展。

## 目录定位

- `cmd/gmqttd/`：broker 守护进程入口，读取配置、装配监听器、导入插件并管理启动/停止生命周期。
- `cmd/gmqctl/`：broker 控制命令和插件模板生成命令。
- `config/`：YAML 配置解析、默认值、监听器、API、MQTT、持久化和插件配置注册。
- `server/`：GMQTT 运行时核心，包括 listener、client、hook、publish 路由、保留消息、统计、持久化工厂和 topic alias。
- `plugin/aetherlink/`：AetherLink 专属集成层，处理设备认证、ACL、Topic 映射、DB/Redis 访问、内部 MQTT 转发和设备调试日志。
- `plugin/`：admin、auth、federation、prometheus 等 broker 插件。
- `persistence/`：内存和 Redis 后端的队列、会话、订阅、未确认 QoS 流程存储。
- `pkg/`：协议相邻工具，包括 packets、返回码、位图工具和 pidfile。
- `docs/`：broker 运维说明；历史或生成文档不应被当作运行时代码。

本仓库没有 `pkg/server` 或 `pkg/plugin` 目录；server 契约位于 `server/`，插件实现位于 `plugin/`。

## 关键文件关系

- `cmd/gmqttd/main.go` 注册守护进程命令并导入核心运行后端。
- `cmd/gmqttd/plugins.go` 由 `plugin_imports.yml` 生成；需要新增内置插件时，应编辑 YAML 后重新生成，不要手改生成导入文件。
- `cmd/gmqttd/default_config.yml` 是可发布的 broker 配置示例，必须和 `config/` 下结构体保持一致，且不能包含私有部署凭证。
- `config/config.go`、`config/api.go`、`config/mqtt.go`、`config/persistence.go` 定义 `cmd/gmqttd/command/start.go` 消费的配置契约。
- `server/server.go`、`server/client.go`、`server/hook.go`、`server/plugin.go`、`server/persistence.go` 定义插件和持久化后端使用的扩展缝隙。
- `plugin/aetherlink/plugin.go`、`hooks.go`、`db.go`、`mqtt.go`、`topicmap_*.go` 将 broker 事件桥接到 AetherLink 设备和 Topic 映射行为；`mqtt_session_revocation*.go` 订阅 SW3 解绑控制消息，并通过 `ClientService.TerminateClientIfCurrent` 原子终止仍属于目标设备的当前连接。
- `persistence/memory.go` 与 `persistence/redis.go` 注册后端工厂，并实现 `persistence/*` 下的存储接口。

## 运行与配置

- 新部署应启用 AetherLink 插件，并使用环境专属配置注入数据库、Redis、MQTT、管理 API 和插件凭证。
- `aetherlink.yml` 属于本地/部署环境配置，可能包含私有 DB、Redis、MQTT 和凭证信息，不应提交真实生产值。
- `cmd/gmqttd/aetherlink.example.yml` 是可跟踪的安全结构模板；根目录 Compose 将它挂载为运行时 `aetherlink.yml`，所有秘密和稳定 broker ID 由 `.env`/`GMQTT_*` 覆盖。手工部署不得直接使用模板中的 `CHANGE_ME` 或空 broker ID。
- `cmd/gmqttd/default_config.yml` 应保持为安全示例，适合文档说明和本地开发参考。默认 MQTT 资源预算按 128 MiB 单机容器设置为 16 MiB 最大报文、100 个入站 QoS1/2 并发、10,000 条单客户端出站队列和 100 条未确认出站消息；代码默认与部署配置由契约测试保持一致。只有在测量载荷大小、客户端数、保留/离线消息和内存后才可调高。
- 插件名称、配置 key、Topic 映射语义、protobuf/API 生成面都属于兼容契约，修改前需要评估前后端、设备和自动化用例影响。

## 部署能力边界

- `local-default`：默认单机配置只加载 `aetherlink` 与 `prometheus`。`aetherlink` 自身注册 `OnBasicAuth`，在 `:1883` 上校验 `root`/插件内部账号及本地设备 voucher；独立 `auth` 插件关闭不代表匿名接入。broker 在认证 hook 缺失时也保持 `allow_anonymous: false` 的拒绝默认。它们覆盖设备接入和本地可观测性，是当前 Compose broker 的默认运行路径。
- `optional-external`：`admin`、`auth`、`federation` 继续由 `plugin_imports.yml` 编译进 broker，以保留插件名、配置 key、gRPC/API 和生成代码契约，但默认 `plugin_order` 不加载它们。独立 `auth` 只用于额外的密码文件策略，不替代 AetherLink 本地设备认证；启用前必须显式修改部署配置，并分别补齐管理凭据、认证文件或联邦节点/端口/网络规划。`admin` 默认保持关闭，不能以固定 cookie/凭据方式启用。
- `blocked-external`：修改 protobuf 后重新生成 gRPC、grpc-gateway、Swagger 或 mock 时，需要 `protoc`、`protoc-gen-go`、`protoc-gen-go-grpc`、`protoc-gen-grpc-gateway`、`protoc-gen-swagger` 与 MockGen。缺少这些工具只阻断维护者再生成流程；普通 build/test 使用已提交生成文件，不应被阻断。
- `federation` 依赖 Serf、gRPC 和额外 gossip/federation 端口，是高成本集群能力，不属于默认单机部署。不得为缩小依赖而删除其接口或生成代码；若不使用，保持默认关闭即可。

## 审查重点

- 改启动流程时，先看 `cmd/gmqttd/command/start.go` 的 listener、pidfile、signal 和 shutdown 逻辑。
- 改 MQTT 报文流、认证交接、QoS、订阅或断开逻辑时，先看 `server/client.go`。
- 改插件扩展语义时，先看 `server/hook.go` 与 `server/plugin.go`。
- 改设备认证、ACL、发布路由、连接状态或调试日志时，先看 `plugin/aetherlink/hooks.go`。
- 改 SW3 解绑后的在线连接撤销时，同时检查 `plugin/aetherlink/mqtt_session_revocation*.go`、认证连接绑定和 `server/client_service.go` 的原子终止语义；不要在 `IterateClient` 回调里访问 Redis 或重新获取 broker 锁。
- 改 Topic 转换规则时，先看 `plugin/aetherlink/topicmap_service.go` 与 `topicmap_matcher.go`。
- 改持久化语义时，先看 `persistence/queue/queue.go`、`session/session.go`、`subscription/subscription.go` 与 `unack/unack.go`。

## 代码审查与重构建议

- 问题：broker 同时承载标准 MQTT 行为和 AetherLink 业务集成，随意重构容易破坏协议兼容或设备接入链路。
- 改进方案：把协议核心、插件 hook、AetherLink 业务桥和持久化后端分层验证，避免一次 PR 同时改多个风险面。
- 实施步骤：先为具体风险点补 focused 测试，再做小 helper 抽取，最后只在集中验证阶段运行更宽 broker suite。
- 预期效果：协议行为、插件契约和产品接入逻辑边界更清楚，GitHub 维护者能快速判断改动影响面。

## 验证建议

- `plugin/aetherlink/` 改动：优先运行认证、ACL、Topic 映射、内部 MQTT、DB/Redis 和设备调试日志相关 focused tests。
- `server/` 改动：优先运行 broker 生命周期、hook、client、packet 或协议级 focused tests。
- `persistence/` 改动：同时覆盖 memory 与 Redis 后端差异。
- `plugin/federation/` 改动：先用窄测试定位 `EventStream`、peer stream 和 stop/replay 行为，不要一开始跑全量。

完整发布验证仍需在集中阶段统一执行并归档；当前 README 不能替代真实运行证据。
