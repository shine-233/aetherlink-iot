# 可视化视图目录

## 目录定位

`frontend/src/views/visualization` 是 AetherLink IoT 前端的可视化与 ThingsVis 集成页面集合。它负责把平台导航、独立预览、仪表盘列表、编辑器和菜单内嵌仪表盘连接起来，是前端跨 iframe、token、路由和兼容性契约的集中区域。

维护者需要特别注意：本目录不只是普通 Vue 页面，还承担宿主系统与 ThingsVis 可视化运行时之间的边界适配。涉及集成标识、预览地址、SSO/token 或 host-side key 的改动，应同时参考根目录 `COMPATIBILITY.md`。

## 关键文件与子目录关系

- `thingsvis/` 是 ThingsVis 主入口页面，用于承接平台内可视化入口。
- `thingsvis-dashboards/` 管理仪表盘列表和预设数据，`rdi-preset.ts` 与 RDI/预置仪表盘场景有关。
- `thingsvis-editor/` 是可视化编辑器入口，通常涉及编辑器加载、上下文传递和外部运行时交互。
- `thingsvis-menu-dashboard/` 负责菜单内嵌仪表盘页面，审查时要关注路由参数和宿主菜单上下文。
- `thingsvis-preview/` 是独立预览入口，常用于查看或嵌入单个仪表盘。
- `native-boards/`、`native-board/` 和 `native-board-editor/` 是默认本地 provider 的列表、查看和编辑页面；它们通过 `src/service/visualization-provider/` 访问数据，不应直接调用 ThingsVis API。
- `__tests__/thingsvis-route-flows.test.ts` 与各子目录 `__tests__/` 用来确认路由流、页面入口和集成行为没有被无意破坏。

## 运行与维护注意事项

- 修改本目录时，不要只确认页面能渲染；还要确认路由参数、预览模式、菜单嵌入模式和编辑模式下的上下文来源是否一致。
- 涉及 ThingsVis token、iframe URL、host key、预览代理或 SSO 行为时，要同步检查 `COMPATIBILITY.md` 和相关路由/守卫逻辑。
- `thingsvis-preview` 可能作为独立页面被外部或空白布局访问，改路由元信息、布局或鉴权逻辑时要避免破坏独立预览。
- 可视化页面和 store 的编辑器状态可能存在耦合，修改编辑器入口时应同时查看 `frontend/src/store/modules/visual-editor/`。
- 本目录适合用静态检查和目标路由/组件测试验证；不要把普通页面渲染成功等同于 ThingsVis 集成契约已被完整验证。

## 代码审查与重构建议

- 审查可视化改动时，重点看“平台路由 -> 页面上下文 -> ThingsVis 运行时参数 -> 用户可见仪表盘”的链路是否完整。
- 把 URL、token、dashboard id、预览参数等兼容性字段集中封装，避免多个页面各自拼接导致集成行为漂移。
- 对预览和菜单内嵌场景补充负向用例，例如缺少 dashboard id、token 失效、外部运行时不可用和路由刷新。
- 若改动影响 RDI 预设仪表盘，要同时审查设备详情 RDI 面板和可视化仪表盘预设之间的字段一致性。
- 大型集成页面重构时，优先把宿主适配、路由解析和编辑器状态管理拆成可测试 helper，再调整 UI 结构。
