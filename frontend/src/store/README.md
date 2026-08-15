# 前端 Store 目录

## 目录定位

`frontend/src/store` 是 AetherLink IoT 前端 Pinia 状态管理目录，负责承载登录会话、动态路由、标签页、主题、系统设置、设备状态、面板/编辑器状态和可视化编辑器状态。页面组件应把跨页面共享状态放在这里或专用 composable 中，而不是在多个视图里重复维护。

维护者可以把本目录视为“前端全局状态契约”：它连接路由守卫、服务层请求、布局组件和业务页面。

## 关键文件与子目录关系

- `index.ts` 创建并导出 Pinia 实例，是前端应用挂载 store 的入口。
- `plugins/index.ts` 为 setup syntax store 补充 `$reset` 行为，修改时要确认所有使用 setup 写法的 store 都能正确重置。
- `modules/auth/` 管理登录、token、用户信息、登出和会话清理，是路由守卫与接口鉴权的核心依赖。
- `modules/route/` 管理动态路由、菜单、缓存路由和权限派生状态，和 `src/router/guard` 强耦合。
- `modules/tab/` 管理多标签页状态，和路由切换、首页固定标签、国际化标题刷新相关。
- `modules/app/`、`modules/theme/`、`modules/sys-setting/` 管理布局、主题和系统设置类状态。
- `modules/device/`、`modules/widget.ts`、`modules/editor.ts` 和 `modules/visual-editor/` 管理设备、组件选择和可视化编辑器相关状态。
- `__tests__/` 和各模块内的 `__tests__/` 是判断状态契约是否被保护的第一入口。

## 运行与维护注意事项

- Store 中需要调用接口时，应通过 `src/service` 封装，不要在 store 里拼接裸 URL 或绕过请求层。
- `auth` 和 `route` 的初始化顺序会影响刷新、权限菜单和登录跳转；改动前要查看路由守卫测试和 `modules/auth`、`modules/route` 的现有 README。
- 登出和 token 失效处理必须清理相关集成状态，例如动态路由、标签页、用户信息以及可视化/ThingsVis 相关 token 或上下文。
- setup syntax store 的 `$reset` 行为依赖 `plugins/index.ts`，新增 setup store 时要确认是否需要纳入重置插件处理。
- Store 适合做小范围单元测试；修改全局状态不要用页面手工验证替代 store 行为测试。

## 代码审查与重构建议

- 审查 store 改动时，优先看状态生命周期：初始化、刷新、更新、重置、登出和异常恢复是否都有明确路径。
- 把复杂状态派生逻辑抽成 `shared.ts` 或纯 helper，保持 action 更像流程编排，减少页面和 store 之间的隐式耦合。
- 对 `auth`、`route`、`tab` 这类基础模块，新增字段时要补充刷新和重置场景测试，避免状态残留导致白屏或权限绕过。
- 可视化编辑器相关 store 应保持与 `frontend/src/views/visualization` 的边界清晰：store 管状态，视图负责路由上下文和用户交互。
- 如果多个业务页面需要同一份临时状态，先判断它是否真的是跨页面全局状态；只在需要跨路由共享、刷新恢复或布局协同时放入 store。
