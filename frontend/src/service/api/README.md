# 前端 API 封装目录

## 目录职责

`frontend/src/service/api` 负责把页面、store、组合式函数需要的后端调用封装成稳定的前端 API wrapper。这里的主要职责是统一请求方法、接口路径、参数位置、payload 字段名和兼容导出方式，而不是替代后端完成业务闭环。

换句话说，这一层更像“前端到后端的契约适配层”，而不是业务状态层。

## 按领域拆分的文件地图

- 认证与用户：
  - `auth.ts`
  - `personal-center.ts`
  - `apikey.ts`
- 设备与 RDI：
  - `device.ts`
  - `device-template-model.ts`
  - `rdi.ts`
- 自动化与告警：
  - `automation.ts`
  - `alarm.ts`（包含告警历史、确认/重置，以及按年份获取 12 个月告警次数的趋势接口）
  - `notification.ts`
  - `notification-services.ts`
- 平台管理：
  - `route.ts`
  - `roles.ts`
  - `setting.ts`
  - `system-management-user.ts`
  - `system-data.ts`
- 设备数据源与可视化：
  - `device-data-source.ts`
  - `dashboard-menu.ts`
  - `thingsvis.ts`
- 插件与扩展：
  - `plugin.ts`
  - `protocol-plugin.ts`
  - `market.ts`
- 兼容与适配：
  - `index.ts`
  - `management.adapter.ts`

## 文件关系与依赖视角

- `index.ts`
  - 统一导出入口，承接大量历史 `@/service/api` 汇总导入调用。
  - 重构时必须优先考虑兼容导出，不要随手删旧出口。
- `device.ts`
  - 当前体量最大，设备列表、详情、配置、命令、遥测、分享等契约集中在这里。
  - 命令契约包含普通 `/command/datas/pub` 和在线 `/command/datas/direct-method`；后者返回的 `published`、`device_responded`、`device_succeeded`、`outcome` 不能在 wrapper 层合并成一个 success。
  - 典型调用方：`views/device/*`、部分设备 store/composable。
- `automation.ts`
  - 场景、联动、规则相关接口集中在这里。
  - 典型调用方：`views/automation/*`、自动化编辑子模块。
- `rdi.ts`
  - RDI 激活、配置、命令、共享等契约集中在这里。
  - 典型调用方：设备域 RDI 视图、接入流程、部分管理页。
- `management.adapter.ts`
  - 不是请求发送器，而是把后端路由/管理数据转换成前端可消费结构。
  - 这类文件容易被误当成“普通 API wrapper”，文档里要单独标出来。

## 这层不负责什么

- 不负责页面状态管理。
- 不负责 store 持久化。
- 不负责真正的权限判断和业务语义裁决。
- 不应把 wrapper 测试当成业务闭环证据，它们只证明“前端会按预期发请求”。

## 当前静态审查结论

### 发现的问题

- `device.ts` 这类大文件容易同时承接多个业务域，后续再加接口时很容易继续膨胀。
- 一些文件虽然只是 HTTP wrapper，但被页面和 store 广泛引用，改字段名或返回结构时影响面很大。
- `index.ts` 的兼容导出对历史调用点很关键，拆文件时如果忽视这一层，很容易引发连锁回归。

### 改进方案

- 按业务域继续拆大文件，但拆分时要保留 `index.ts` 的兼容出口。
- 在 README 中明确“请求发送器”和“结构适配器”的区别，避免把 `management.adapter.ts` 这类文件混入普通 wrapper 讨论。
- 对关键 API 文件建立“页面/Store 典型依赖面”说明，便于改动前做影响分析。

### 建议实施步骤

1. 先继续梳理 `device.ts`、`automation.ts`、`rdi.ts` 三个大文件的拆分边界。
2. 再明确 `index.ts` 的兼容出口策略和迁移顺序。
3. 最后把高频调用文件的页面依赖关系补齐到 README。

### 预期效果

- 维护者能更快判断某个改动是在“接口契约层”还是“页面业务层”。
- 大文件拆分时更容易保证兼容出口不丢。
- GitHub 浏览者可以先通过 README 了解 API 地图，再进入具体 wrapper。
