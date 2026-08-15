# 设备详情页说明

## 目录职责

`frontend/src/views/device/details` 提供单设备详情页，负责聚合遥测、图表、事件、命令、自动化、告警、设置、诊断和 RDI 操作视图等子视图。

## 文件关系

- `index.vue` 是详情页壳层，负责路由设备 ID 解析、详情加载、Tab 可见性裁剪、WebSocket 在线状态订阅，以及公共 props 下发。
- `modules/*` 是各业务 Tab 的具体实现，依赖 `index.vue` 提供的设备 ID、在线状态和详情对象。
- `modules/rdi/*` 承载 RDI 专用的常量、composable 和面板逻辑。
- `device-tab-registry.ts` 为普通设备保留原有通用 Tab；识别为 RDI 设备时只暴露 `message/chart/give-an-alarm/rdi` 四个客户 Tab，其中 `message` 使用只读 `modules/RdiDeviceDetailsView.vue`，`chart` 使用 `modules/RdiDeviceHistoryView.vue`。

## 当前状态流

1. 页面从 `route.query.d_id` 读取当前设备 ID。
2. `getDeviceDetail` 拉取设备详情，并同步 `deviceData`、标签、在线状态和标题。
3. `createDeviceTabPlan` 根据设备类型、物模型图表能力和 RDI 能力生成可见 Tab 集合；RDI 计划由 registry 收口成四个客户 Tab，默认进入只读详细信息，并始终保留 RDI 专用历史模块。
4. 详情页向 `/device/online/status/ws` 发送订阅消息，持续接收在线状态帧。
5. 每个子 Tab 通过统一 props 获取当前设备上下文；当路由切换或手动刷新时，必要的 Tab 会递增 `refreshKey` 触发重载。

## 静态审查建议

### 发现的问题

- `index.vue` 仍然承担了路由、详情请求、Tab 规划、WebSocket 订阅、编辑弹窗和多处跳转，页面壳层职责偏重。
- 设备详情字段兼容逻辑较分散，例如在线状态、告警状态、RDI 能力和物模型图表能力分别在不同函数中处理，后续继续叠加字段时容易漏改。
- 国际化切换依赖“清空再恢复 tabValue”的方式触发重渲染，虽然低成本，但属于经验性修补，长期维护成本偏高。

### 改进方案

- 将“详情数据归一化”“Tab 可见性规划”“在线状态订阅”拆成独立 composable，页面只保留装配职责。
- 为设备详情建立更明确的视图层类型，逐步收敛 `Record<string, any>` 和 `any` 的使用范围。
- 给路由切换、Tab 裁剪和语言切换补最小行为测试，保证后续重构时能快速发现回归。

### 建议实施步骤

1. 先抽出 `useDeviceDetailState`，承接详情请求、字段归一化和保存载荷组装。
2. 再抽出 `useDeviceDetailTabs`，集中处理候选 Tab、隐藏规则和 `tabsRenderKey` 刷新策略。
3. 最后抽出 `useDeviceOnlineStatus`，统一管理 WebSocket URL、订阅发送和状态帧解析。
4. 每抽出一层后补对应的静态用例或最小组件级测试，再继续下一层。

### 预期效果

- 降低 `index.vue` 的认知负担，后续新增/下线 Tab 时更容易定位修改点。
- 收敛字段兼容逻辑，减少同一设备状态在多个分支里重复解释的风险。
- 让国际化、路由切换和刷新流程有更明确的边界，后续做功能扩展时更稳。
