# Auth Store

## 目录职责

`frontend/src/store/modules/auth` 管理登录态、token、用户信息、RSA 密码处理和退出清理。

## 文件关系

- `index.ts` 是 Pinia store 主体。
- `shared.ts` 封装 token/userInfo 的 storage 读写和清理。
- `route`、`tab` store 依赖 auth 生命周期完成动态路由和标签页重置。

## 重点文件

- `index.ts`: 登录、按 token 初始化、退出、用户信息刷新。
- `shared.ts`: 认证 storage 契约。

## 审查建议

重点关注登录失败、token 过期、密码策略、退出清理和 ThingsVis token 清理。变更后应跑 auth store 单测和路由守卫相关测试。
