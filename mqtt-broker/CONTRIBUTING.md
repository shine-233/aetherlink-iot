# MQTT Broker 贡献指南

本文说明 `mqtt-broker` 子项目的贡献规则。该目录包含上游 GMQTT 演进而来的 broker 核心，同时叠加 AetherLink IoT 的设备接入、Topic 映射、管理 API 和可观测性插件。贡献时请同时尊重 MQTT 协议兼容性和 AetherLink 业务契约。

## 提交前原则

- 先判断改动属于协议核心、插件扩展、AetherLink 业务桥、持久化后端还是文档/生成物维护。
- 不要在同一次改动中混合协议行为、业务接入、生成文件和大范围格式化。
- 不要提交真实数据库、Redis、MQTT、管理账号或生产凭证。
- 不要手工修改 `*.pb.go`、`*_grpc.pb.go`、`*.pb.gw.go`、`*_mock.go` 等生成文件；应回到 `.proto`、mockgen 配置或生成脚本。
- 上游 GMQTT 名称可能仍出现在兼容协议、指标名或历史注释里；对外文档应优先使用 AetherLink IoT 口径，并明确哪些符号必须保留兼容。

## 代码风格

- Go 代码必须运行 `gofmt`，必要时运行 `goimports`。
- 新增函数应保持职责单一，优先抽取纯 helper，再让 hook/client/service 调用。
- 错误处理应返回结构化错误或使用结构化日志，不要新增临时 `fmt.Println` 或散落的 `log.Println`。
- 影响外部行为的配置项、Topic 规则、管理 API 和指标名必须在 README 或对应目录文档中说明。

## 测试要求

- 修改 `server/`：至少运行相关 client、hook、packet、lifecycle 或协议 focused tests。
- 修改 `plugin/aetherlink/`：至少运行认证、ACL、Topic 映射、内部 MQTT、DB/Redis 或设备调试日志相关 focused tests。
- 修改 `plugin/admin` 或 `plugin/auth`：至少验证 gRPC/REST handler、鉴权、分页、密码文件或会话边界。
- 修改 `plugin/federation`：先运行窄范围 federation/peer stream tests；若发现超时，优先用编译和单测试二进制区分编译卡住还是运行卡住。
- 修改 `persistence/`：同时考虑 memory 与 Redis 后端语义。
- 广泛的 `go test ./...`、race、协议互操作或端到端验证应放在集中验证阶段，不要在静态批量改动中途作为唯一证据。

## 文档要求

- 新增目录或重要插件能力时，要同步更新目录 README。
- README 需要使用中文、写清目录定位、关键文件关系、维护注意事项和重构建议。
- 对仍保留上游兼容名的能力，例如部分 `gmqtt_*` 指标或生成接口符号，应说明“保留原因”和“迁移风险”。
- 删除或替换上游文档链接时，优先改成本仓相对路径或 AetherLink 当前维护入口。

## PR 自检清单

- 是否只改了本次任务需要的文件，没有顺手大范围格式化？
- 是否保留了协议、配置、Topic、API 和生成物兼容边界？
- 是否为真实业务行为提供了 focused 验证，而不是只改文档？
- 是否明确记录了仍未运行的 broad suite、服务启动、API automation 或 Playwright E2E？
- 是否避免了生产凭证、私有地址和本机绝对路径进入 Git？
