# AetherLink IoT 本地自动化测试

本目录是本地 API 自动化、Playwright E2E、预览代理验证与运行报告生成的入口。这里的脚本可以帮助收集证据，但证据是否能支撑发布结论，仍取决于账号、数据、后端、前端预览代理和断言质量是否真实满足要求。

## 目录定位

- `tests/`：Mocha API 自动化与覆盖率契约检查。
- `e2e/`：Playwright 浏览器端流程验证。
- `lib/`：API 客户端、覆盖率采集、报告生成、运行配置、种子数据等共享辅助模块。
- `scripts/`：本地账号准备、预检、预览代理和预览可达性检查脚本。
- `reports/`：运行时临时报告目录，内容会被后续运行覆盖。
- `run_tests.js`：自动发现并执行 API/E2E 模块的统一运行器。
- `config.json`：提交到仓库的配置模板，真实账号应通过环境变量或本地忽略文件注入。

## 文件用途

- `package.json` 提供轻量脚本入口，例如列出模块、API 自动化、E2E、预检和本地账号准备。
- `playwright.config.js` 读取本地运行配置，控制浏览器、前端地址和预览代理模式。
- `reports/summary.json`、`reports/summary.html`、端点覆盖率、页面覆盖率和 Playwright 报告都是运行产物，不应被当作长期发布证据。
- 需要长期保留的成功验证结果，应归档到项目根目录的 `verification/<时间戳>/` 并附带命令、退出码、目标环境和阻塞项。

## 环境模板

- `automation_tests/.env.example` is the committed release-style automation env template. Copy it into an ignored local env file or export the same keys in your shell before running release API/E2E evidence collection.
- `npm run preflight:local` is the local convenience gate: it requires `frontend/dist/index.html` and a reachable backend, starts an isolated preview proxy for the check, runs the `local-lite` configuration/connectivity checks, and always shuts the proxy down. It does not validate release accounts or prove release readiness.
- `npm run preflight:api-e2e` remains the strict full release configuration/connectivity gate. It probes preview HTML, proxied deployment-health JSON, and backend health JSON, but does not start services, log in, run browser flows, or prove business correctness.
- The default Playwright browser channel is msedge. If that browser is unavailable on the machine, set PLAYWRIGHT_BROWSER_CHANNEL or PLAYWRIGHT_BROWSER_EXECUTABLE_PATH before running E2E.

## 规范入口与证据边界

- `npm run preflight:local` 是本地便利门，只验证本地构建、代理和基础连通性。
- `npm run preflight:api-e2e` 是严格的发布配置与连通性门，但仍不证明业务流程正确。
- `node run_tests.js --list` 只列出自动发现的模块，不执行测试或生成业务证据。
- `node run_tests.js --include-e2e` 通过统一 runner 执行 API/E2E，并生成带证据分层和阻断原因的报告。
- `npm run test:e2e` 或直接运行 `playwright test` 属于 raw 调试入口，不会自动等同于统一 evidence report，也不能单独支撑发布结论。

## 验证命令

以下命令只列出或检查环境，不会启动 broad E2E：

```powershell
cd automation_tests
npm run test:list
npm run preflight:api-e2e
node run_tests.js --list
```

真正执行 API/E2E 前，必须确认后端、数据库、预览代理和账号数据已准备好。不要把 `CHANGE_ME_*` 占位账号、开发服务器页面或错误的 API 目标当成有效验证。

## 审查发现

- API/E2E 报告容易被误读：端点命中、页面访问或进程未崩溃，不等于业务逻辑已经被证明正确。
- `reports/` 是共享临时输出，后续运行会覆盖旧文件，因此不能单独支撑发布结论。
- `config.json` 保留占位账号是正确做法；真实密码、浏览器状态和本地 env 文件必须继续保持未提交。

## 重构/清理建议

- 新增测试时同步补充业务能力元数据，区分业务证据、边界冒烟、页面覆盖、目录契约和阻塞用例。
- 避免宽泛状态码断言、只检查 `body` 可见、或将跳过用例算作通过。
- 如果运行器输出 schema 变化，先补契约测试，再更新本目录 README、`lib/README.md` 和验证归档规则。
