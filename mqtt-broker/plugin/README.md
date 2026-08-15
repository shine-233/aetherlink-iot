# Broker 插件目录

`mqtt-broker/plugin` 保存 broker 插件实现，用于扩展 AetherLink Topic 规则、管理 API、认证、集群联邦和可观测性能力。

## 目录定位

- `aetherlink`：AetherLink 专属 MQTT 集成，包含 Topic 映射、数据库持久化 Topic map 状态和 topic 工具。
- `aetherlink/util`：AetherLink 插件使用的发布/订阅 Topic 白名单校验器。
- `admin`：Broker 管理插件，提供 gRPC、REST gateway、轻量管理 UI、客户端控制、订阅控制和发布 API。
- `auth`：用户名/密码认证插件，提供账号管理 API 和密码文件持久化。
- `federation`：实验性多节点联邦插件，使用 Serf 成员发现和 gRPC event stream。
- `prometheus`：被动可观测性插件，导出 broker 统计指标并提供轻量 metrics 页面。

生成的 protobuf、grpc-gateway、Swagger 和 mock 文件会和手写插件代码放在同一目录。审查行为时优先看手写 Go 文件和 `.proto` 源；生成文件主要用于确认来源和漂移。

## 关键文件关系

- `plugin_imports.yml` 控制哪些插件会在 `go generate ./...` 后编译进 `cmd/gmqttd/plugins.go`。
- 每个插件通常都有 `config.go`，负责 YAML 默认值和校验。
- 插件入口通过 `server.RegisterPlugin` 注册自身，并通常通过 `config.RegisterDefaultPluginConfig` 注册默认配置。
- `hooks.go` 文件把插件行为挂接到 broker 生命周期和 MQTT 事件上。
- `admin` 提供 `Indexer`、分页、gRPC 错误处理等共享辅助，`auth` 也会复用部分能力。
- `federation` 组合成员发现、peer streaming、本地/联邦订阅状态和生成的 Federation/Membership RPC 绑定。
- `prometheus` 直接读取 broker 统计信息，正常情况下不应改变 broker hook 行为。

## 关键文件

- `aetherlink/util/check_pub_topic.go`：发布 Topic 白名单和单层通配符匹配。
- `aetherlink/util/check_sub_topic.go`：订阅 Topic 白名单和设备编号占位符处理。
- `admin/admin.go`：Admin 插件注册、API 注册和 broker service 装配。
- `admin/web.go`：内置管理登录/仪表盘 UI 和环境变量会话门禁。
- `admin/store.go`：管理 API 使用的客户端与订阅内存镜像。
- `auth/auth.go`：Auth 插件生命周期、凭证加载、hash 校验和 API 注册。
- `auth/grpc_handler.go`：账号列表、详情、更新、删除和密码文件写入。
- `federation/federation.go`：联邦生命周期、RPC handler、peer 鉴权和 event/session 处理。
- `federation/hooks.go`：向 peer 广播订阅和消息事件的 broker hook 包装。
- `federation/peer.go`：出站 peer event queue、stream 握手、重放和确认处理。
- `prometheus/prometheus.go`：HTTP exporter、dashboard handler、Prometheus collector 和指标转换。

## 插件开发流程

1. 在仓库根目录下安装本地 CLI：

```bash
go install ./cmd/gmqctl
```

2. 使用 `gmqctl gen plugin` 生成模板：

```bash
gmqctl gen plugin -n awesome -H OnBasicAuth,OnSubscribe -c true -o ./plugin
```

常用参数：

- `-n`：插件名称。
- `-H`：需要挂接的 hook，多个 hook 用英文逗号分隔。
- `-c`：是否生成配置结构。
- `-o`：输出目录，通常为 `./plugin`。

3. 将插件加入 `plugin_imports.yml`：

```yaml
packages:
  - admin
  - prometheus
  - federation
  - auth
  - awesome
```

外部插件才需要使用完整 import path；本仓内置插件优先使用相对包名。

4. 在 broker 项目根目录运行生成命令：

```bash
go generate ./...
```

该命令会重新生成 `cmd/gmqttd/plugins.go`，让插件在编译期被导入。

## 代码审查与重构建议

- 问题：插件目录同时包含手写逻辑和生成物，维护者容易误改生成文件或忽略插件注册链路。
- 改进方案：把“手写行为文件”“生成文件”“配置入口”“hook 入口”在 README 和文件头中明确区分。
- 实施步骤：先清理上游英文模板和过期链接，再补插件级 focused tests，最后统一核对 `plugin_imports.yml` 与生成导入文件。
- 预期效果：插件开发路径更清晰，生成物漂移风险更低，发布前审查更容易定位真实行为。
