# Route Store

## 目录职责

`frontend/src/store/modules/route` 管理动态路由、菜单、面包屑、缓存路由和本地化后的菜单状态。

## 文件关系

- `index.ts` 初始化授权路由并写入 Vue Router。
- `shared.ts` 保存菜单生成、权限过滤、排序、缓存路由和面包屑纯函数。
- `router/guard/permission.ts` 依赖本模块判断是否完成授权路由初始化。

## 重点文件

- `index.ts`: 动态路由初始化、重置和菜单状态入口。
- `shared.ts`: 可测试的路由/菜单转换规则。

## 审查建议

审查时先确认后端 route 数据、generated routes 和菜单展示是否一致。新增动态路由补丁应覆盖权限过滤、缓存路由和兼容 redirect。
