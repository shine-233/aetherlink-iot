# API wrapper 测试目录

## 目录职责

`frontend/src/service/api/__tests__` 负责验证前端 API wrapper 的请求合同，重点检查请求方法、路径、query 参数、请求体、URL 编码和少量错误回退逻辑。这里的测试运行在 mock request 层之上，用来发现前端封装漂移，不证明真实后端业务已经通过。

## 文件关系

- 每个 `*.test.ts` 通常对应 `frontend/src/service/api` 下同名或同领域的 wrapper 文件。
- 测试通过 mock `@/service/request` 记录 wrapper 发出的 HTTP 调用，再断言方法、路径和参数。
- `device.test.ts`、`rdi.test.ts`、`auth.test.ts` 覆盖高风险业务入口，应该优先跟随对应 wrapper 改动同步维护。
- `management.adapter.test.ts` 验证路由数据适配，不依赖 HTTP mock。

## 重点文件

- `auth.test.ts`: 覆盖登录、用户信息、登出、验证码、用户管理和初始化回退合同。
- `device.test.ts`: 覆盖设备管理、设备配置、模型、遥测、属性、事件、命令、RDI 邻近入口和调试接口合同。
- `automation.test.ts` 与 `scene.test.ts`: 覆盖自动化、场景、菜单、日志、启停和开关合同。
- `rdi.test.ts`: 覆盖 RDI 激活、配置、历史、命令、分享、固件和共享设备合同。
- `route.test.ts` 与 `management.adapter.test.ts`: 覆盖后端菜单路由适配和未知路由边界。

## 审查建议

新增或调整 API wrapper 时，先在对应测试中锁定方法、路径、参数位置和错误分支，再改实现。审查时重点看 mock 断言是否足够精确，避免只断言“被调用过”。如果要证明真实业务行为，还需要后端 API 自动化、集成测试或 Playwright E2E 的新证据。
