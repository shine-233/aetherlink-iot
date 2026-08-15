# 前端工具总览

## 目录职责

集中存放前端共享工具，包括存储、日志、路由标签、服务适配、ThingsVis、WebSocket、脚本编辑器适配和通用数据处理。

## 文件关系

工具函数应尽量保持小而明确；凡是涉及浏览器存储、网络、全局实例或兼容协议的文件，都要在文件头说明副作用。

- `clipboard.ts` 集中 Clipboard API 与临时 textarea 回退，设备分享等客户入口应复用它，不要各自直接调用 `navigator.clipboard`。

## 维护建议

后续维护时继续把业务规则留在 service/core/view 层，避免 utils 成为隐藏业务逻辑仓库。
