# ThingsVis SDK 客户端

## 目录职责

提供前端嵌入 ThingsVis Studio 时使用的 iframe/postMessage 客户端。

## 文件关系

`client.ts` 管理 iframe URL、target origin、消息监听、事件分发和不可信来源过滤；测试覆盖 origin 与消息边界。

## 维护建议

修改 SDK 时优先审查 `targetOrigin`、`contentWindow`、hash/query 拼接和 message source 校验。
