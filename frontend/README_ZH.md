[English](./README.md) | 中文

# AetherLink IoT Frontend

AetherLink IoT Frontend 是 AetherLink IoT 平台的 Vue 3 Web 客户端，
覆盖设备管理、遥测数据、告警通知、自动化联动、可视化看板和系统设置等
Web 功能。

## 技术栈

- Vue 3、TypeScript、Vite、Pinia、Vue Router、Naive UI
- Vitest 覆盖前端业务逻辑、路由契约和组件状态
- Playwright E2E 通过工作区级 `automation_tests/` runner 执行

## 常用命令

```bash
pnpm install
pnpm dev
pnpm typecheck
pnpm test:coverage
pnpm build
```

## 工作区说明

- `dist/`、`coverage/` 和本地报告等生成物由根目录 `.gitignore` 忽略。
- ThingsVis 是可选的可视化/看板集成模块；它的运行时标识统一由当前代码中的
  常量管理，而不是散落在文档里。
