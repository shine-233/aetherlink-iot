# request service 测试目录

## 目录定位

`frontend/src/service/request/__tests__` 保存前端请求基础层的单元测试，是 API wrapper、鉴权、错误处理和响应 envelope 的底层保护。

## 文件用途

- `request.test.ts` 覆盖 request 封装的关键行为。

## 维护边界

本目录只测试请求基础设施，不应放具体业务接口场景。业务 service 测试应放在对应 service 子目录，避免基础层测试被业务细节污染。

## 审查发现

请求基础层影响面大，单文件测试需要重点覆盖 token、错误、超时、响应拆包和兼容返回结构。当前 README 补齐后能明确维护边界。

## 重构建议

如果 request 行为继续增加，建议按鉴权、错误处理、响应解析、重试或取消请求拆分测试块，保持失败定位清晰。

## 验证建议

修改请求基础层后运行该测试，并抽查若干 service wrapper 测试确认调用契约未被破坏。
