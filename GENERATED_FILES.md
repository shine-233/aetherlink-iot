# 生成文件策略

AetherLink IoT 当前仍将若干生成产物保留在仓库中，因为仅凭公开工作区还不能完整、稳定地复现所有生成链路。

## 保留在源码中的生成文件

- `backend/internal/model/*.gen.go`
- `backend/internal/query/*.gen.go`
- `backend/docs/docs.go`
- `backend/docs/swagger.json`
- `backend/docs/swagger.yaml`
- `backend/third_party/grpc/**/*.pb.go`
- `mqtt-broker/` 下的 GMQTT protobuf 和 mock 文件

这些文件在可行处会通过 `.gitattributes` 标记为 generated，但它们仍属于当前公开源码候选范围。

## 不保留在源码中的生成文件

- `frontend/src/typings/components.d.ts`
- `frontend/dist/`、`frontend/dist-lite/` 等前端构建输出
- `.playwright-cli/`、`playwright-report/`、`test-results/` 等浏览器自动化日志、页面快照和报告
- `automation_tests/reports/` 等自动化运行报告
- 根目录与各模块的 `_localrun/` 中的本地日志、截图、缓存快照、临时测试输出和隔离实例运行态
- `verification/` 中的运行归档和压缩包；仅保留目录说明，选定证据仍由本地发布流程管理
- `*.exe`、`*.dll`、`*.so`、`*.dylib` 等本地编译二进制，以及 `*.tsbuildinfo` 等增量编译状态

`frontend/build/**/*.ts` 是 Vite 必需构建配置源码，不是构建输出；必须保持可见并纳入源码审查。

这类文件可由构建、组件扫描或自动化验证链路重新生成，已在 `.gitignore` 中忽略，不应作为公开源码的一部分长期保留。模块运行产物应直接写入对应模块的 `_localrun/`，不要先落到源码顶层或创建重复的 `automation_tests/automation_tests/`。需要留存的发布证据应按 `VALIDATION.md` 归档到本地 `verification/<timestamp>/`，而不是混入活动源码目录。

## 保留原因

- 后端 GORM model/query 生成能力存在，但 helper 还不是覆盖所有已提交生成文件的完整再生成流水线。
- gRPC Go 文件被后端代码直接 import；完整 `.proto` 输入和端到端再生成流程尚未整理成干净的公开流程。
- Swagger 文件已接入后端 router，并且对本地 API 检查有用。
- GMQTT mock/protobuf 文件属于 broker 测试和协议表面的一部分。

## 未来清理规则

不要为了缩小仓库体积直接删除生成文件。应先补齐必要的源输入和生成命令，验证可以从干净状态重新生成，再决定继续提交生成输出，还是改为开发/CI 阶段生成。

## 维护与审查建议

- 修改生成文件时，请同时说明它是手动同步、工具生成，还是跟随上游代码迁移产生的变化。
- 如果只改了生成产物而没有对应输入或命令证据，评审时应要求补充再生成说明或风险说明。
- `.gitattributes` 的 generated 标记只表示审查方式不同，不表示这些文件可以忽略兼容性或编译风险。
- 对于像 `frontend/src/typings/components.d.ts` 这类可稳定再生的前端生成文件，优先删除工作树副本并保留生成规则，而不是把产物一起提交。

## 运行截图归档边界

- `frontend/output/` 和 `automation_tests/output/` 只允许作为临时迁移来源，不是源码目录；新运行不得向这两个路径写入。
- 页面截图、页面 sweep 日志和运行期 JSON 应写入 `verification/<timestamp>/`，并由同一归档目录中的 manifest 记录命令、来源目录、文件数量和阻塞项。
- 逐页检查脚本位于 `automation_tests/scripts/visual-page-sweep.js`；使用 `VISUAL_OUTPUT_DIR` 可以为隔离运行指定归档目标，未指定时自动创建带 UTC 时间戳的 `verification/visual-page-sweep-<timestamp>/frontend-playwright/`。
