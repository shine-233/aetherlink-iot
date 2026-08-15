# Playwright E2E 用例

本目录保存浏览器端 E2E 用例、登录态准备、页面覆盖率采集和稳定选择器辅助代码，用于验证用户可见流程。

## 目录定位

- 覆盖登录、设备、数据、告警、配置、系统、仪表盘、自动化、可视化等页面流程。
- 与 API 自动化互补：E2E 更接近用户行为，但必须依赖真实后端 JSON 响应和稳定前端预览。
- 本目录不是截图仓库，也不保存长期证据；报告应归档到 `verification/<时间戳>/`。

## 文件用途

- `*.spec.js` 是 Playwright 测试用例。
- `auth.setup.js` 负责登录态准备，生成的认证状态不得提交。
- `fixtures.js`、`selectors.js` 提供共享上下文和稳定选择器。
- 页面覆盖率只能证明访问过页面，不能单独证明业务流程正确。

## 验证命令

```powershell
cd automation_tests
npm run test:list
node run_tests.js --module e2e-device --list
```

执行真实 E2E 前先运行 `npm run preflight:api-e2e`，并确认 `FRONTEND_URL`/`PREVIEW_URL` 指向 `9725` 预览代理。预检会检查配置，并实际探测 preview HTML、代理后的 deployment health JSON 和 backend health JSON；它仍不会登录、运行浏览器流程或证明业务正确。运行前还应确认 `frontend/dist` 已由 `frontend/pnpm build` 生成；默认浏览器通道是 `msedge`，缺失时请设置 `PLAYWRIGHT_BROWSER_CHANNEL` 或 `PLAYWRIGHT_BROWSER_EXECUTABLE_PATH`。

## 审查发现

- 如果 `/api/v1/*` 没有代理到后端 JSON，E2E 可能只是在前端 HTML 上失败或误报。
- 单纯路由访问、`body` 可见或页面未崩溃不能算业务闭环证据。
- 登录态、截图和 trace 可能包含本地账号信息，必须保持忽略或人工脱敏。

## 重构/清理建议

- 优先使用稳定的 `data-testid`、可见业务状态、URL 变化、接口副作用和持久化状态作为断言。
- 新增页面时同步更新页面覆盖率目录和业务能力映射。
- 把跳过原因写清楚，不要用弱断言掩盖缺少账号、设备或后端数据的问题。
