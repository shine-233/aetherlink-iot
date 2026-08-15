# product service 测试目录

## 目录定位

`frontend/src/service/product/__tests__` 保存产品、OTA 和升级包前端 API wrapper 的契约测试。

## 文件用途

- `list.test.ts` 覆盖产品列表相关请求封装。
- `update-ota.test.ts` 覆盖 OTA 请求封装。
- `update-package.test.ts` 覆盖升级包请求封装。

## 维护边界

本目录只验证前端 service wrapper 的请求路径、参数和响应处理，不直接测试后端业务逻辑。后端行为应由 Go 测试、API 自动化或 E2E 覆盖。

## 审查发现

测试贴近 service 层，有利于防止 OTA 字段、升级包路径和列表参数在重构中漂移。需要持续避免只断言“函数被调用”而不检查关键请求内容。

## 重构建议

后续可统一 service mock 工具，减少各测试重复构造 request spy，并显式断言 method、url、query/body 和错误分支。

## 验证建议

修改 product service 后运行本目录 Vitest，并在必要时补充 API 自动化验证真实后端协议。
