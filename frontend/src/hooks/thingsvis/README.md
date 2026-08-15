# ThingsVis 推送 Hook

## 目录职责

封装 ThingsVis 嵌入场景下的告警推送和实时数据推送订阅逻辑。

## 文件关系

该目录直接处理 WebSocket、设备鉴权、平台字段映射和消息解析，属于嵌入联动边界。

## 维护建议

改动时需要同步验证 token、设备 ID、字段映射和断线清理，避免只覆盖 happy path。
