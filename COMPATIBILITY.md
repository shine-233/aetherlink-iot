# 外部合约说明

## 部署边界

- Native visualization 是默认核心路径；它不是历史 ThingsVis 的 3D、SSO、embed 或插件生态的完美替代，不能以 Native 已构建或已通过本地测试推断这些外部契约已经闭合。
- `_isolated/thingsvis-upstream-20260802` 曾是用于 optional compatibility audit 的独立 ThingsPanel/ThingsVis 上游源码副本，已于 2026-08-13 清理；AetherLink 核心不依赖它。若需要 ThingsVis external runtime 验证，应重新从 upstream 取得独立副本并单独验收；它不是 AetherLink Native provider，也不在 Native-only 源部署包内，不能当作 Native 的无缝替代或直接复制进核心包。
- ThingsVis 仅通过 `deploy/docker-compose.optional-integrations.yml` 的 `optional-integrations` profile 提供。默认 `docker-compose.yml` 不启动 ThingsVis，`frontend/nginx.conf` 对相关路径保持 `503 THINGSVIS_OPTIONAL_SERVICE_DISABLED`；缺少 `THINGSVIS_AUTH_SECRET` 时，显式启用 profile 也必须失败。
- `AETHERLINK_PUBLIC_URL` 只表示操作者可打开的浏览器地址，`AETHERLINK_MQTT_ACCESS_ADDRESS` 只表示设备可到达的 `host:port`。部署模板中的 localhost 是本机/演示默认值，不是生产地址；server doctor 必须拒绝 localhost、loopback、`0.0.0.0` 和占位域名，并检查它们分别与 `GOTP_OTA_DOWNLOAD_ADDRESS`、`GOTP_MQTT_ACCESS_ADDRESS` 一致。
- 当前仓库没有真实服务器 IP/域名、HTTPS 证书、反向代理或对外 MQTT 参数；这些必须由部署环境提供，不能由文档或脚本猜测。前端 Dockerfile 在镜像构建阶段从源执行 `pnpm build`，本地 `frontend/dist` 只能作为当前构建快照证据，不能替代镜像构建和远程连通性验证。

当前公开项目命名、默认 broker 入口、现行 ThingsVis 运行时常量和生成的 telemetry client wrapper 已对齐到 AetherLink IoT。

本文件不再维护旧名称保留清单。它用于标记仍然具有外部合约属性的少数表面，避免未来把合约变更误当作普通文案清理。

## 合约类别

1. Broker plugin 加载和运行配置表面。
2. ThingsVis embed、SSO 标识符以及宿主侧 key。
3. HTTP protocol plugin 的注册、通知、请求和错误分类接口。
4. SMTP/本地邮件 adapter 的投递结果与错误分类接口。
5. 生成的 telemetry gRPC service symbol 和 method path。

## 当前规则

如果上面的任一合约类别再次变化：

1. 将重命名或路径变化视为 breaking migration；
2. 在同一轮工作中更新当前状态文档和相关技能/参考说明；
3. 针对受影响表面运行聚焦的 broker、frontend、backend、API automation 和 Playwright 验证；
4. 将证据归档到 `verification/<timestamp>/`；
5. 记录迁移说明后，才能声明 GitHub 发布或 release readiness。

当前几个最需要避免误改的合约敏感点如下：

- ThingsVis：`thingsvisAppFrameLifecycle.ts` 与 `thingsvisFrameTransportBridge.ts` 已把生命周期编排和 transport 安全边界分开，但 `tv:ready` 调度、`targetOrigin` 来源和 `tv:platform-data` 回推语义仍属于外部契约，不能当作普通清理随意调整。
- broker：`mqtt-broker/server/client.go` 与 `mqtt-broker/server/client_options.go` 已完成契约/协商选项拆分，但插件侧看到的 `Client` 接口和运行时选项字段仍是对外表面，后续重命名或删字段都应按 breaking migration 处理。
- telemetry：`automate_telemetry.go` 的拆分当前主要是内部可维护性优化；真正需要按外部合约对待的仍是生成的 gRPC symbol、method path 和任何对外可见的 telemetry client wrapper。
- 认证会话与 Casbin 审计（P1 加固批次，2026-08-26）：`<email>_token` 键的 value 已从明文 JWT 改为 `utils.TokenDigest` 摘要且 TTL 与会话超时对齐；刷新令牌现在会在新会话写入成功后吊销旧摘要。`casbin.route-audit-mode` 默认 fail-fast——存量库升级若启动报"protected routes not registered"，先在菜单/API 管理补登记对应路由，过渡期可临时设 `warn` 或 `off`。

## 当前工作树

当前活动仓库应使用现行 AetherLink 入口和文档。历史归档中仍可能出现退役标识符，因为它们保存的是旧检查点；当前操作文档和实时源码应只描述当前工作树。

## 外部依据

这遵循常见 API 兼容性实践：plugin loading ID、config key、embed identifier、service symbol 和远程 method path 都是合约。未来变更应按合约迁移验证，而不是按品牌文案直接替换。

参考资料：

- Google AIP-180: https://google.aip.dev/180
- gRPC core concepts: https://grpc.io/docs/what-is-grpc/core-concepts/
- Protocol Buffers updating guidance: https://protobuf.dev/programming-guides/proto3/#updating
- Semantic Versioning: https://semver.org/

## 维护与审查建议

- 审查时请区分“可以直接中文化/品牌化的说明文字”和“会影响外部调用方的 wire/config/service 合约”。
- 如果发现旧名称残留，先判断它位于历史归档、兼容路径还是当前运行路径；不要盲目全局替换。
- 修改本文件列出的合约类别时，请同步检查 `VALIDATION.md` 的验证门槛和 `PUBLICATION.md` 的发布边界。
