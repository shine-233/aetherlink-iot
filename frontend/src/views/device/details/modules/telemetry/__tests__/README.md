# __tests__

## 目录职责

遥测总面板测试目录。

## 文件关系

- `telemetry.test.ts` 对应 `../telemetry.vue`，聚焦实时遥测、日志、历史入口和模拟数据上报。
- 测试 mock 应覆盖接口响应、WebSocket 消息和非法输入。

## 重点文件

- `telemetry.test.ts`: 遥测总面板关键交互测试。

## 审查建议

建议审查新增字段、非法 JSON、pong 心跳和模拟 payload 字符串化是否被覆盖。
