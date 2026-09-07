# AetherLink IoT 全平台实现真实性与测试覆盖审计

日期：2026-09-07

## 结论

当前不能证明“所有功能都已实现”“API/E2E 测试覆盖所有功能”或“没有假功能”。最强可成立的表述是：部分核心能力有源码、单元测试和历史 synthetic/compose 运行证据；当前主线没有新鲜的结构化 API/Page/summary 覆盖归档，真实设备、目标部署和若干业务闭环仍未证明。

## 当前仓库与测试规模

- 前端源码视图文件：559；前端测试文件：1849（静态文件计数，不等于执行过）。
- 后端 Go 源文件：916；Go 测试文件：365。
- MQTT Broker Go 源文件：206；测试文件：78。
- API 自动化测试文件：82；E2E spec：25；另有 1 个 `pending-e2e` spec 未纳入常规目录。
- 静态测试声明约 807 个，不能替代运行结果、业务断言和外部依赖证据。

## 运行证据边界

- 使用真实项目路径 `active/aetherlink-iot` 运行 inspector：当前 `selectedArchive=null`，`endpointCoverage/pageCoverage/summary` 均为空。
- 57 个前端叶路由可被当前路由目录识别，但没有当前归档覆盖证据；这不是说每个页面绝对没有历史访问，而是当前没有可审计的新鲜 page 证据。
- 当前可追溯的完整历史 Compose 证据是 2026-08-24 的归档：API 和 Playwright 20/20 模块通过，但 manifest 明确限定为 synthetic/compose，不是 real-RDI、目标生产部署或发布签字。
- `automation_tests` 当前缺少 `axios`，因此本轮无法重新运行 `npm run test:list` 或完整 API/E2E runner。文件存在不能当作本轮执行成功。

## 明确的占位或未实现代码

这些不是“覆盖不足”，而是代码本身明确表示尚未实现，需要从产品完成度中扣除：

1. `backend/internal/roadmap/skeletons.go`：`Unwired*` 契约骨架统一返回 `ErrNotImplemented`；这是路线图框架，不是业务实现。
2. `frontend/src/service/visualization-provider/native-board-provider.ts`：项目创建/更新/删除以及复杂 `canvasConfig/nodes/dataSources/variables/thumbnail` 路径显式返回 unsupported。不能把它当作完整 Dashboard/SCADA 编辑器。
3. `mqtt-broker/cmd/gmqttd/command/gen-plugin/tmpl.go`：存在 `panic("implement me")`，相关生成器路径不能宣称可用。
4. 多个生成 gRPC 服务返回 `codes.Unimplemented`。必须区分生成代码的可选服务接口与实际产品依赖，不能用接口存在证明功能可用。

## 测试覆盖的明显弱点

- `automation_tests/pending-e2e/shadow-offline-delivery.spec.js` 未进入常规 `e2e/` runner；即使执行，当前包含 `toBeTruthy()`、`resp.ok()`、`Array.isArray(...).toBeTruthy()` 等弱断言，不能证明设备 ACK、状态变更、重试和不重复投递。
- API/E2E 文件数量和路由命中数量不能证明业务覆盖；必须有状态、权限、租户边界、错误、幂等、设备/数据库可见结果断言。
- 历史报告与当前 `HEAD` 的对应关系没有通过新鲜 commit/SHA 门禁证明；旧归档不能直接用于当前主线发布结论。
- 当前 coverage inspector 只接受同时存在 `summary.json`、`endpoint-coverage.json`、`page-coverage.json` 的目录；仓库中存在只含 `archive-manifest.json` 的历史运行证据，因此 `selectedArchive=null` 表示“结构化 coverage 归档未匹配”，不等于完全没有历史运行记录。

## 审计工具/skill 本身的问题

`aetherlink-iot-test-coverage` 的原则部分是可信的：覆盖应分为源码、API、页面、业务断言、oracle/mutation；绿色数字不等于业务闭环；preview API 必须真实代理后端 JSON。

但其执行实现存在高风险漂移：

- 默认项目路径仍是 `C:\Users\Zz\Documents\projects\aetherlink-iot`，真实仓库是 `C:\Users\Zz\Documents\projects\active\aetherlink-iot`。
- 默认路径不存在时，inspector 使用 `SilentlyContinue` 并输出零源码/零路由，可能制造“没有缺口”的假阴性。
- skill 列出的多个 current short docs 在当前仓库不存在，包括 `live-short-status.md`、`coverage-short-status.md`、`current-baseline.md`、`file-work-plan.md` 和指定中文状态文件。
- inspector 只取含三件 JSON 文件且按目录时间最新的归档，不校验 archive-manifest、运行命令、commit SHA、成功状态或证据新鲜度。
- 路由扫描只识别单引号静态 `path`，父路由排除列表硬编码；弱断言扫描不覆盖 `pending-e2e`、fixtures、helpers 和所有脚本。
- `frontend/vitest-latest.json` 不存在时直接返回 null，skill 没有要求如何生成并验证该文件。

因此本次审计明确使用真实项目路径，并将 inspector 当作辅助信号，而不是最终 oracle。

## 当前状态分类

### 已有较强局部证据

- 部分 Go/Vue 单元测试。
- 部分协议隔离 E2E（SNMP、OPC UA、CoAP/LwM2M、OIDC、Timescale 等历史批次）。
- 历史 synthetic/compose API 与 Playwright 批次。
- 部分多租户、RBAC、边缘转发、规则链和 OTA API 业务测试。

### 只能判为 partial 或 pending

- 全部 57 个当前页面的业务 E2E。
- 当前主线的全量 API/E2E 新鲜运行结果。
- 设备影子真实 Broker ACK、超时/重试/重复投递控制。
- OTA 进度消费、失败重试、取消、回滚。
- 目标部署、TLS/MQTTS、公网 MQTT、backup/restore、real-RDI。
- Native Board 完整自定义 Widget/SCADA 画布。
- 通用实体关系、规则链死信/回放/节点级重试。

## 下一步审计门禁

1. 修正审计工具默认路径、错误处理和归档识别规则；不能继续使用静默空项目结果。
2. 安装并锁定 `automation_tests` 依赖，运行 `test:list`、preflight 和 API/E2E；归档命令、commit SHA、环境摘要、失败清单和清理结果。
3. 建立能力矩阵：每个 P0/P1 能力映射到源码、API、UI、E2E、负向、幂等和真实运行 oracle。
4. 将 `pending-e2e` 影子测试改为常规可执行测试，但先替换弱断言为精确 HTTP/MQTT/数据库/设备状态断言。
5. 对所有 `unsupported`、`ErrNotImplemented`、`panic("implement me")` 和 `codes.Unimplemented` 做分类：删除死代码、实现、或明确标记 optional/external-blocked。
6. 只有新鲜运行证据和负向控制同时通过后，才能提升某个能力从 partial 到 done；不能提升为“全平台全部完成”。
