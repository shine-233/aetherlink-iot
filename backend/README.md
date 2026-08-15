# AetherLink IoT 后端

## 目录定位

`backend/` 是 AetherLink IoT 的 Go 后端工作区，负责对外 HTTP API、租户与权限、设备与遥测、告警与通知、自动化场景、可视化看板、OTA、OpenAPI 以及与 MQTT 设备链路相关的服务编排。

该目录既包含真实业务源码，也包含生成代码、配置模板、初始化逻辑、静态资源和历史设计资料。上传 GitHub 前，需要同时关注源码质量、生成文件边界、配置脱敏和目录说明完整性。

## 目录结构

| 路径 | 说明 |
| --- | --- |
| `cmd/` | 后端命令行入口、生成器、设备自动化辅助工具。 |
| `configs/` | 本地配置模板、示例配置和 RSA 等安全材料目录。 |
| `docs/` | 后端设计说明、编码约束和接口标准化资料。 |
| `data/` | 私有运行期持久化目录；当前配置使用 `telemetry-spool/` 和 `uplink-spool/` 作为数据库与 dead-letter 写入失败后的文件兜底，不属于公开文件服务边界。 |
| `files/` | 公开文件服务边界，当前源码资产包括 `logo.png`；本地导出表格、升级包等运行期文件应保持在忽略路径中，不能把 telemetry spool 放入这里。 |
| `initialize/` | 初始化逻辑、定时任务、自动化缓存等启动装配代码。 |
| `internal/` | 核心业务实现，包含 API、Service、DAL、Model、Middleware、适配器和上下行处理。 |
| `mqtt/` | 后端侧 MQTT 发布、订阅和模拟消息协助逻辑。 |
| `pkg/` | 通用工具、错误码、常量、公共能力和指标。 |
| `router/` | HTTP 路由注册、应用分组和静态文件暴露。 |
| `sql/` | 数据库结构、初始化 SQL 和迁移辅助资料。 |
| `static/` | 静态资源目录。 |
| `test/` | 后端专项测试资源与多库验证辅助目录。 |
| `third_party/` | 第三方 gRPC 生成代码与外部协议依赖。 |

## 核心模块说明

### `internal/service/`

承载主要业务服务编排，负责权限校验后的实体创建、更新、联动、副作用触发和缓存/通知协调。近期静态整理已经覆盖多个热点文件，例如设备、遥测、看板、自动化和市场服务接入。

### `internal/dal/`

负责查询条件拼装、分页过滤、数据库读写入口和局部缓存协作。这里既是性能热点，也是最容易积累重复条件拼装和历史兼容逻辑的区域，适合持续做小步低风险重构。

### `internal/api/`

承载请求绑定、响应返回、接口级权限入口和参数校验，是前端/自动化最直接依赖的契约面。

### `third_party/grpc/`

这里的生成代码属于外部协议契约面。涉及遥测 gRPC 方法名、消息结构、Broker 插件识别符或 ThingsVis 运行时 Key 时，必须同步检查根目录 [COMPATIBILITY.md](../COMPATIBILITY.md)。

## 维护边界

- 部署能力状态保留 `enabled`、`configured`、`healthy` 兼容字段，并用 `status` 区分 `disabled`、`configuration-required`、`blocked` 和 `available`。PostgreSQL、Redis、MQTT Broker 与 Native Visualization 属于核心/本地能力；ThingsVis、HTTP adapter、Market、SMTP 和地图 provider 属于外部可选能力。
- 用户验证码邮件已有显式注入的本地 adapter，可在开发/测试中捕获完整邮件；默认仍使用生产 SMTP adapter。本地 receiver 未配置时必须失败，生产 SMTP 失败也禁止自动降级成本地成功。
- 当前批次以“静态源码整理 + 中文文档化 + 低风险重构”为主，不把局部绿灯等同于发布就绪。
- `configs/` 下的示例配置应保持可读，但不能内置真实密钥、证书、数据库账号或公网地址。
- 生成代码、协议文件、历史兼容常量要优先确认契约归属，再决定是否改名或删除。
- 涉及设备、遥测、自动化、告警、Broker 协同时，需优先保守处理，避免为了注释或命名整理误改业务行为。

## 当前已识别的重构热点

| 模块 | 问题描述 | 改进方向 | 预期效果 |
| --- | --- | --- | --- |
| `internal/dal/` | 条件拼装和分页过滤在多个查询对象中重复，部分文件仍保留历史兼容写法。 | 抽取可复用的过滤 helper，统一字段表达式与错误包装。 | 降低维护成本，减少条件分支复制。 |
| `internal/service/` | 一些大文件同时承担参数兜底、权限判断、状态同步和副作用触发。 | 继续按“权限校验 / 数据准备 / 持久化 / 副作用”拆分本地 helper。 | 让核心路径更易审查，也更利于后续补测试。 |
| `internal/api/` | 接口层与服务层之间的错误语义并不总是足够统一。 | 收敛错误码包装、字段名提示和参数错误文案。 | 提升前后端联调体验和自动化断言稳定性。 |
| `third_party/` | 生成代码与手写文档之间的契约边界容易模糊。 | 明确“可编辑/不可编辑”规则并持续更新说明文档。 | 避免手改生成文件导致的回归。 |

## 建议的阅读顺序

1. 先看 [backend/internal/README.md](internal/README.md) 了解业务分层。
2. 再看 [backend/internal/api/README.md](internal/api/README.md)、[backend/internal/service/README.md](internal/service/README.md) 与 [backend/internal/dal/README.md](internal/dal/README.md) 理解入口层、业务层和数据层分工。
3. 涉及启动、装配和请求链时，继续看 [backend/internal/app/README.md](internal/app/README.md) 与 [backend/internal/middleware/README.md](internal/middleware/README.md)。
4. 修改配置或部署前，查看 [backend/configs/README.md](configs/README.md)。
5. 触及协议兼容面前，查看 [COMPATIBILITY.md](../COMPATIBILITY.md) 与 [GENERATED_FILES.md](../GENERATED_FILES.md)。

## 静态审查结论

- 后端目录结构完整，但顶层 README 之前过于简略且偏英文，无法支撑 GitHub 级别的维护说明。
- `internal/service/` 与 `internal/dal/` 仍是后续最值得持续投入的真实业务热点，优先级高于测试辅助目录。
- 当前文档已明确“不在本批次运行测试/编译”，因此所有重构建议都属于静态建议，仍需后续统一验证后才能形成发布结论。
