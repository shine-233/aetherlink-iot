# AetherLink IoT 代码导航

> 本文件是代码结构导航索引，**不重复各权威文档内容**。各主题详细说明见下方对应文档。
> 文档体系与定位以 [references/文档地图.md](references/文档地图.md) 为准。

## 项目概览

AetherLink IoT 是面向物联网平台场景的完整源码仓库，包含 5 个顶层模块：

| 模块 | 技术栈 | 职责 |
| --- | --- | --- |
| `frontend/` | Vue 3 + TS + Vite + Pinia + Naive UI + elegant-router | 前端控制台与可视化页面 |
| `backend/` | Go 1.25 + Gin + GORM + PostgreSQL + Redis + MQTT | 后端 API、业务服务、DAL、消息上下行 |
| `mqtt-broker/` | Go + GMQTT | MQTT Broker 运行时、插件、协议实现 |
| `automation_tests/` | Node.js + Playwright + Vitest | API 自动化、E2E、预检脚本 |
| `deploy/` | Shell + PowerShell + Docker Compose | 一键部署、预检、首台设备闭环 |

## 技术栈速览

- **后端**：Go 1.25 + Gin + GORM + PostgreSQL 16 + Redis + Casbin + MQTT + Prometheus（TimescaleDB 扩展可选；未安装时使用普通 PostgreSQL 表）
- **前端**：Vue 3 + TypeScript + Vite + Pinia + Naive UI + elegant-router + UnoCSS + pnpm monorepo
- **Broker**：GMQTT（含 aetherlink/auth/admin/federation/prometheus 插件）
- **测试**：Vitest（前端单测）+ Playwright（E2E）+ go test（后端）

## 按主题查找权威文档

### 系统架构
- [backend/system_architecture.md](backend/system_architecture.md) — 后端架构图（Mermaid）与组件关系

### 目录结构
- [backend/docs/code_help/directory_structure.md](backend/docs/code_help/directory_structure.md) — 后端目录速览
- [backend/internal/README.md](backend/internal/README.md) — internal 分层说明
- [backend/internal/app/README.md](backend/internal/app/README.md) — 应用装配层
- [backend/internal/api/README.md](backend/internal/api/README.md) — 控制器层
- [backend/internal/service/README.md](backend/internal/service/README.md) — 服务层
- [backend/internal/dal/README.md](backend/internal/dal/README.md) — 数据访问层
- [backend/internal/storage/README.md](backend/internal/storage/README.md) — 持久化存储层
- [backend/internal/processor/README.md](backend/internal/processor/README.md) — 脚本处理器
- [backend/internal/adapter/mqttadapter/README.md](backend/internal/adapter/mqttadapter/README.md) — MQTT 适配器
- [backend/initialize/README.md](backend/initialize/README.md) — 初始化层
- [mqtt-broker/README.md](mqtt-broker/README.md) — Broker 总览
- [mqtt-broker/plugin/aetherlink/README.md](mqtt-broker/plugin/aetherlink/README.md) — Aetherlink 插件

### 开发与运行
- [START-HERE.md](START-HERE.md) — 傻瓜式部署入口
- [references/developer-guide.md](references/developer-guide.md) — 开发指南与轻量检查
- [backend/docs/README-DEV.md](backend/docs/README-DEV.md) — 后端开发说明
- [deploy/README.md](deploy/README.md) — 部署脚本
- [deploy/observability/README.md](deploy/observability/README.md) — 可观测性

### API 与插件
- [references/api-guide.md](references/api-guide.md) — Command Job / Direct Method 等 API 合同
- [references/plugin-guide.md](references/plugin-guide.md) — 前端插件管理与 Broker 插件边界
- [references/plugin-runtime-surface.md](references/plugin-runtime-surface.md) — Broker aetherlink 插件文件/hook 目录
- [frontend/src/service/api/README.md](frontend/src/service/api/README.md) — 前端 API 调用层

### 前端专题
- [frontend/src/locales/README.md](frontend/src/locales/README.md) — 国际化（zh-CN/en-US/fr-FR/es-ES）
- [frontend/src/locales/AetherLink国际化开发指南.md](frontend/src/locales/AetherLink国际化开发指南.md) — i18n 开发指南
- [frontend/src/components/thingsvis/README.md](frontend/src/components/thingsvis/README.md) — ThingsVis 集成
- [frontend/src/core/interaction-system/README.md](frontend/src/core/interaction-system/README.md) — 交互系统
- [frontend/src/core/interaction-system/API.md](frontend/src/core/interaction-system/API.md) — 交互系统 API
- [frontend/src/core/interaction-system/QUICK_START.md](frontend/src/core/interaction-system/QUICK_START.md) — 交互系统快速开始
- [frontend/packages/scripts/README.md](frontend/packages/scripts/README.md) — CLI 工具（gen-route 等）

### 规划与文档索引
- [ROADMAP.md](ROADMAP.md) — 公开能力规划入口
- [references/文档地图.md](references/文档地图.md) — 文档定位索引

### 安全、发布与合规
- [SECURITY.md](SECURITY.md) — 安全策略与本地密钥清单
- [PUBLICATION.md](PUBLICATION.md) — 公开源码范围
- [COMPATIBILITY.md](COMPATIBILITY.md) — broker plugin / ThingsVis / telemetry gRPC 外部合约
- [CONTRIBUTING.md](CONTRIBUTING.md) — 模块边界与提交规则
- [GENERATED_FILES.md](GENERATED_FILES.md) — 生成文件保留策略
- [VALIDATION.md](VALIDATION.md) — 验证门槛与证据边界
- [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) — 第三方 license 声明
- [CLAUDE.md](CLAUDE.md) — 项目配置与工作规则、权威文档索引

### 自动化测试
- [automation_tests/README.md](automation_tests/README.md) — 测试总览
- [automation_tests/e2e/README.md](automation_tests/e2e/README.md) — E2E 测试
- [automation_tests/scripts/README.md](automation_tests/scripts/README.md) — 预检脚本

## 后端分层调用链

```
HTTP/WS 请求 → router → api(Controller) → service(GroupApp) → dal → query/model/global.DB

设备消息上行：broker → internal/adapter → uplink.Bus → UplinkManager → *Uplink 处理器 → storage/dal/service
命令下行：    service → downlink.Bus.PublishXxx → Handler → MQTT publish → 设备
```

启动装配顺序（`main.go` 通过 `With*` Option 函数）：
配置 → RSA → 日志 → DB → Redis → Storage → Uplink → Heartbeat → Diagnostics → MQTT → Downlink → gRPC → HTTP → Cron → Workers → Telemetry

## 前端启动链

```
main.ts → setupStore(Pinia) → setupI18n → setupNProgress/Loading
→ sysSettingStore.initSysSetting(异步) → setupRouter → createRouterGuard
→ guard: progress → permission → title → app.mount
```

动态路由：`routeStore.initAuthRoute()` 按 `VITE_AUTH_ROUTE_MODE`（static/dynamic）选择前端生成或后端拉取。
