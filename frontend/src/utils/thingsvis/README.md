# ThingsVis 工具

## 目录职责

封装 AetherLink 与 ThingsVis 嵌入、SSO、空间隔离、物模型字段、URL 构建和本地缓存相关的前端工具。

## 文件关系

核心文件包括认证服务、URL 构建、平台字段抽取、物模型预设、首页缓存和 iframe SDK。

## 维护建议

审查时重点防止 SSO key、space id、proxy path、iframe origin 和兼容 alias 漂移。
