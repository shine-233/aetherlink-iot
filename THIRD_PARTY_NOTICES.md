# 第三方声明

AetherLink IoT 是独立的 IoT 平台工作区。仓库中包含带有各自 license 文件的组件；公开发布时，应让这些 license 文件随对应代码一起保留。

## 已包含组件

- `frontend/` 随现有 Apache License 2.0 文件 `frontend/LICENSE` 一起分发。
- `backend/` 随现有 Apache License 2.0 文件 `backend/LICENSE` 一起分发。
- `mqtt-broker/` 基于 GMQTT，随现有 MIT License 文件 `mqtt-broker/LICENSE` 一起分发。

## 受兼容性管理的接口

Broker plugin loading、ThingsVis embed/SSO identifiers 和生成的 telemetry gRPC symbols 应被视为外部合约表面，而不是普通品牌文案。

`COMPATIBILITY.md` 记录这些表面未来发生合约变化时应如何处理。部署专用 endpoint 和占位符属于配置问题，不属于第三方声明范围。

## 维护与审查建议

- 引入新依赖或复制第三方代码时，请确认 license、NOTICE 或 attribution 要求，并把必要文件放在对应代码旁边。
- 发布前请检查 `frontend/LICENSE`、`backend/LICENSE`、`mqtt-broker/LICENSE` 是否仍随代码存在。
- 如果未来替换 GMQTT、ThingsVis 集成或 telemetry 生成链路，请同步复核本文件、`COMPATIBILITY.md` 和发布说明。
