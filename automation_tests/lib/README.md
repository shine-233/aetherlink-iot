# 自动化共享库

本目录保存 API 自动化、E2E 支撑、覆盖率契约、报告生成、运行配置和种子数据相关的共享 JavaScript 模块。

## 目录定位

- 为 `run_tests.js`、`tests/` 和 `e2e/` 提供复用逻辑。
- 将账号、API 客户端、响应断言、覆盖率采集、报告输出和阻塞原因记录集中管理。
- 这里的模块可以产生证据摘要，但不能替代真实业务断言。

## 文件用途

- `api_client.js`：封装登录、Token、请求和端点命中记录。
- `runtime_config.js`：读取提交模板并叠加本地环境变量。
- `endpoint_coverage.js`、`page_coverage.js`：记录 API 端点和页面访问覆盖情况。
- `endpoint-coverage/catalog.js`：保存完整端点清单，稳定入口复用同一数组引用。
- `coverage_contract.js`、`oracle_contract.js`、`test_metadata.js`：把测试文件、证据类型和业务能力映射起来；`coverage_contract.js` 保持为稳定入口。
- `coverage-contract/business-capabilities.js`：保存父级路由和完整业务能力清单，供稳定入口按同一对象引用复用。
- `coverage-contract/readiness.js`：组合静态审计结果并计算 fail-closed readiness，不读取源码或启动外部服务。
- `runner/cli-policy.js`：集中维护命令行默认值、参数解析、报告归档判定和运行器退出码策略；`run_tests.js` 保持稳定 facade 引用。
- `runner/result-summary.js`：归类 Mocha/Playwright 结果、跳过和阻塞原因；显式报告优先，否则安全读取运行器提供的 JSON 报告路径。
- `runner/module-catalog.js`：集中维护模块标签与证据分类、API/E2E 文件发现、别名筛选和执行计划；发现根目录固定为 `automation_tests`。
- `reporter.js`：写入机器可读和人工可读运行摘要。
- `seed_data.js`、`test_data.js`、`process_lock.js`、`integration_blocked.js`、`response_assertions.js`：支撑可靠数据准备、并发保护、阻塞说明和精确断言。

## 验证命令

```powershell
cd automation_tests
node -c .\lib\runtime_config.js
node -c .\lib\coverage_contract.js
node -c .\lib\reporter.js
```

如需全面检查，可对本目录全部 `.js` 执行 `node -c`，这属于静态语法检查，不会启动服务。

## 审查发现

- 覆盖率契约和报告路径容易让读者把“执行过”误解为“业务正确”。
- 部分历史文件头存在英文或乱码说明，后续维护应统一为中文四字段头。
- 共享库如果隐藏断言，会降低失败可诊断性。

## 重构/清理建议

- 保持“采集覆盖率”和“证明业务正确”两个概念分离。
- 修改报告 schema 前先补契约测试，避免下游解析器被静默破坏。
- 纯分类逻辑、文件系统写入和运行副作用应逐步拆分，便于静态验证和单元测试。
