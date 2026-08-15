# 路由定义目录

## 目录职责

`frontend/src/router/routes` 汇总常量路由、自定义兼容 redirect、fallback 路由和授权路由生成逻辑，并把 Elegant Router 元数据转换为 Vue Router 可用结构。

## 文件关系

- `index.ts`：合并 root、兼容 redirect、fallback、ThingsVis 预览路由和 generated elegant routes。
- `../elegant/routes.ts`：提供生成路由元数据。
- `../elegant/imports.ts`：提供布局和页面组件映射。
- `../elegant/transform.ts`：负责 route record 转换。
- `../guard/permission.ts`：依赖这里生成的 route name、meta 和 fallback 行为做权限判断。

## 重点文件

- `index.ts`：根路由、旧路径兼容、404 fallback、授权路由转换入口，改动会影响菜单、权限守卫和老链接可达性。

## 审查建议

- 问题描述：兼容 redirect 和 generated route 混在同一个输出中，如果没有测试，很容易把旧入口误删或让 fallback 抢占真实路由。
- 改进方案：保留 route coverage contract，明确哪些路径属于历史兼容，哪些路径属于正式菜单入口。
- 实施步骤：新增或删除路径时同步更新 `COMPATIBILITY.md`、路由测试和 README；必要时在受控 release/evidence manifest 中记录发布边界。仓库当前没有固定的 `current-baseline.md`，不要为了满足示例而新建第二份基线文件。
- 预期效果：避免 GitHub 发布后出现旧链接失效、菜单不可见或权限绕过。

## 使用注意

- `thingsvis-preview-standalone` 是无需登录的独立预览常量路由，不能和授权路由混淆。
- `device-config-bridge-redirect` 这类桥接路由属于兼容契约，删除前必须确认外部入口已迁移。
- 修改 fallback 路由顺序时要重新验证 not-found 动态路由恢复逻辑。
