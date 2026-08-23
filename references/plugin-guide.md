## 2026-07-09 Credential And Broker UX Anchors

- Keep broker/plugin documentation aligned with customer onboarding language: access token, MQTT basic credentials, and X.509/mTLS are distinct customer mental models, not interchangeable implementation details. ThingsBoard documents these modes explicitly in its MQTT getting-connected guide: https://thingsboard.io/docs/reference/mqtt-api/getting-connected/
- AetherLink currently has MQTT credential/voucher and broker plugin surfaces, but this round did not implement X.509 fleet provisioning or mTLS onboarding. Do not claim that until broker config, device credential storage, docs, and runtime tests all exist.
- If fleet provisioning is added later, keep claim-certificate onboarding, unique device certificate issuance, and broker credential validation as separate runtime evidence items. AWS IoT documents this as a first-connection provisioning flow: https://docs.aws.amazon.com/iot/latest/developerguide/provision-wo-cert.html
- Plugin evidence still needs hook-level runtime proof: auth, ACL, subscribe, message forwarding, device status, debug log, and negative credential assertions.

# AetherLink IoT 插件指南

## 2026-07-09 Latest Verification Note

- No live plugin or broker runtime contract changed this round.
- The touched work stayed in frontend onboarding, telemetry, and automation dry-run slices plus harness trust verification; none of that should be counted as fresh plugin runtime evidence.
- Topic/config compatibility boundaries remain the same: keep broker/plugin-facing compatibility fields such as `sample_topic` where they are still part of the real contract.
- `mqtt-broker/plugin/aetherlink` hook code is now documented as an already split runtime surface: `hooks.go` is the adapter, while auth, lifecycle, subscribe, message, and debug hooks live in focused files.
- See `references/plugin-runtime-surface.md` for the catalog boundary. It is not runtime proof until broker/device/ACL/forwarding/debug-log execution evidence exists.
## 插件覆盖证据口径

- 插件或 broker 相关覆盖不能只靠 catalog、路由烟雾或 source 字符串证明；业务闭环需要真实插件 hook、MQTT topic、认证/ACL、上下行响应和负向断言。
- `mqtt-broker/plugin/aetherlink` 的测试可以作为 broker 层证据，但不能替代后端 API/E2E 运行证据。
- 覆盖合同里出现插件、service access 或 OpenAPI 端点时，必须明确是 `business`、`boundary` 还是 `catalog`，避免把插件管理页面烟雾当成真实插件能力。

本仓库有两类插件相关能力：前端的服务/插件管理页面，以及 `mqtt-broker` 的 GMQTT 插件运行时。两者不要混为一谈。

## 前端插件管理

- 当前真实入口是 `frontend/src/views/apply/plugin`。
- 历史 `/plugin/*` demo 页面已经不属于当前路由或覆盖证据。
- `frontend/src/plugins/icon/icons.ts` 仍是管理权限图标选择器的共享资产，不是旧 demo 残余。

## Broker 插件

- Broker 插件实现位于 `mqtt-broker/plugin`。
- AetherLink 业务集成插件位于 `mqtt-broker/plugin/aetherlink`。
- 插件生命周期、hook 和持久化扩展入口主要在 `mqtt-broker/server`。
- `cmd/gmqttd/plugins.go` 由 `plugin_imports.yml` 生成；新增内置插件时先改 YAML，再重新生成。

## 修改风险

- 插件名称、配置 key、topic 映射、认证/ACL、protobuf/API 生成面都是兼容合同。
- 改认证、ACL、上下行路由或设备状态时，要同时检查后端、Broker、设备示例和自动化断言。
- 改 Prometheus、admin、auth、federation 插件时，优先跑对应插件窄测试，再考虑扩大范围。

## 当前清理口径

- 删除 `/plugin/*` demo 残余是清理假覆盖，不是删除真实插件能力。
- 保留 `apply/plugin` 是保留真实业务入口。
- 保留 Broker 插件目录是保留 MQTT 接入和扩展能力。
- Topic Mapping dry-run 已新增 `test_topic` 语义别名；Broker/插件侧仍要兼容旧 `sample_topic` 合同字段。

## 后端协议插件接入边界（2026-08-23）

- `/api/v1/plugin/*` 五个端点（heartbeat、device/config、devices、service/access×2）由
  `middleware.PluginAuth` 保护：配置 `plugin.service.key`（env `GOTP_PLUGIN_SERVICE_KEY`）后，
  所有来源必须携带等值 `X-Plugin-Key` 头（常量时间比较）。
- 密钥留空时的默认策略：仅放行回环与 RFC1918/ULA 私网来源；公网来源返回 401。
  判定只看 TCP 对端 RemoteAddr，不信任代理头。经 Nginx/容器代理的内部链路不受影响；
  直接暴露公网的部署必须配置共享密钥。
- 外部协议插件升级路径：部署侧生成随机密钥注入后端，插件侧在请求头附带该密钥即可。

## 2026-07-08 Latest Verification Note

- No plugin surface contract changed this round.
- Current verification work stayed in frontend focused suites and automation
  harness trust checks; do not treat that as fresh plugin runtime evidence.
