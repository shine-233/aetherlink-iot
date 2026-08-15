# 前端路由目录

## 目录职责

`frontend/src/router` 负责创建 Vue Router 实例、整合 Elegant Router 生成的路由元数据、注册全局路由守卫，并维护兼容跳转与 fallback 路由。它决定了前端页面能否被正确访问、菜单是否可见，以及刷新/登录/无权限场景如何跳转。

## 文件关系

- `index.ts`：创建 router 实例，选择 history 模式，注册全局守卫并等待 router ready。
- `guard/`：全局守卫目录，处理权限、标题、进度条和移动端布局候选判断。
- `elegant/`：Elegant Router 生成或半生成的路由元数据、组件 import 映射和转换工具。
- `routes/`：合并根路由、兼容 redirect、fallback 路由和授权路由生成逻辑。

## 重点文件

- `index.ts`：应用启动时的 router 安装入口，history mode、base URL 和守卫注册顺序都会影响部署路径与登录跳转。
- `routes/index.ts`：常量路由、兼容路由、fallback 和授权路由的合并入口。
- `guard/permission.ts`：权限边界最集中的文件，任何变更都应配套路由守卫测试。

## 审查建议

- 问题描述：路由生成、权限守卫、菜单可见性和历史兼容跳转容易混在一起，发布前如果缺少说明，后续维护者难以判断某条路由是否能删。
- 改进方案：把“生成路由”“兼容 redirect”“权限判断”“菜单显示”分别写在对应目录 README 和测试中。
- 实施步骤：保持 `routes/` 只负责路由结构合并；保持 `guard/` 只负责导航决策；新增兼容路径时同步更新 `COMPATIBILITY.md` 和 route contract 测试。
- 预期效果：降低误删旧入口、误改权限边界和部署路径不匹配的风险。

## 使用注意

- `management/*`、`manage/*`、设备详情和 ThingsVis 相关兼容路径属于发布敏感契约，不能只看当前菜单是否显示就删除。
- 修改 generated route 相关文件前，应确认它是否由 Elegant Router 生成；生成文件优先通过源配置重建，不建议手改。
- 新增守卫或 redirect 后必须确认不会造成重复 `next()`、循环重定向或首屏白屏。
