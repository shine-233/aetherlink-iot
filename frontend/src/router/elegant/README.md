# Elegant Router 集成目录

## 目录职责

`frontend/src/router/elegant` 保存 Elegant Router 生成或半生成的路由元数据、布局/页面组件 import 映射，以及把 Elegant Route 转换为 Vue Router route record 的工具。它是文件路由和运行时 Vue Router 之间的适配层。

## 文件关系

- `routes.ts`：生成的路由定义，描述 route name、path、component key 和 meta。
- `imports.ts`：生成的布局和页面组件映射，供转换工具按 component key 找到真实组件。
- `transform.ts`：把 Elegant route 转换为 Vue Router 可用的 `RouteRecordRaw`。
- `../routes/index.ts`：主要消费方，负责把本目录输出与自定义兼容路由合并。

## 重点文件

- `transform.ts`：route name/path、component key、children 和 redirect 规则集中在这里，修改后必须跑 route contract 测试。
- `imports.ts`：页面懒加载映射，任何 key 漂移都会导致路由无法解析。
- `routes.ts`：生成边界文件，通常不应手工编辑。

## 审查建议

- 问题描述：生成文件和手写兼容路由混用时，route key、component key、path 容易漂移。
- 改进方案：将生成边界写清楚，并用 route coverage contract 锁定关键路径和兼容别名。
- 实施步骤：先确认变更来自路由生成配置还是手写兼容层；再修改对应源；最后运行路由转换和守卫测试。
- 预期效果：减少手改生成文件造成的不可复现问题。

## 使用注意

- 不要直接删除 generated route，除非确认源页面、菜单、权限和兼容路径都已经迁移。
- `layout.*` 与 `view.*` component key 必须能在 `imports.ts` 中找到对应映射。
- `transform.ts` 应保持无副作用，避免在转换阶段写 store、localStorage 或全局状态。
