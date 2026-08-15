# API 测试辅助模块

本目录保存 Mocha API 用例使用的账号、权限、边界闭环等辅助模块。

## 目录定位

- 为测试准备可复用前置条件，减少重复账号和权限夹具代码。
- 只支撑测试，不直接构成业务闭环证据。
- 辅助模块必须让调用方清楚知道创建了什么数据、需要什么清理、失败代表什么。

## 文件用途

- `dynamic_accounts.js`：创建并清理权限/角色测试所需的临时账号。
- `casbin_fixtures.js`：准备授权策略相关夹具。
- `api_closure_helpers.js`：为 API 边界和闭环检查提供共享客户端、断言和上下文。

## 验证命令

```powershell
cd automation_tests
node -c .\tests\helpers\dynamic_accounts.js
node -c .\tests\helpers\casbin_fixtures.js
node -c .\tests\helpers\api_closure_helpers.js
```

## 审查发现

- 如果把关键断言藏在辅助模块里，测试文件会变成“只调用 helper”，不利于判断业务证据。
- 动态账号和权限夹具会修改本地后端状态，不能在未知环境中随意执行。
- helper 失败应暴露具体 API、账号或权限原因，而不是被上层宽泛跳过。

## 重构/清理建议

- helper 负责准备和清理，业务断言尽量留在测试文件中。
- 返回值应包含显式 ID、账号、权限或清理函数，方便失败时追踪。
- 新增 helper 时补中文四字段文件头，并在本 README 更新用途说明。
