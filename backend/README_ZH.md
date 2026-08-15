# AetherLink IoT 后端

这是 **AetherLink IoT** 的 Go 后端服务。

它负责 HTTP API、多租户与权限、设备与遥测、告警与通知、自动化场景、
可视化接口、OTA/数据脚本/OpenAPI 服务，以及 MQTT/设备链路的后端逻辑。

生成的 telemetry gRPC client 代码属于外部契约面；如果后续还要再次改
broker、ThingsVis 或 telemetry 的 wire-level 标识，请先看仓库根目录
`../COMPATIBILITY.md`，并补齐针对性验证。

## 构建

```bash
go test ./...
go build ./...
```

Docker 构建产物入口名为 `AetherLink-Go`。
