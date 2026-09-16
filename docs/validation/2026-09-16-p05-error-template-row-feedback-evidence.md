# P0.5 剩余任务 1 闭环：CSV 逐行错误反馈不再被模板吞掉

> 日期：2026-09-16
> 仓库：`aetherlink-iot`，基线 `main@9ec969a`
> 对应路线图：P0.5「CSV 浏览器 E2E」缺口类型 `阻断缺陷` → 其中「错误模板吞掉子原因」一项
> 上游依据：`docs/validation/2026-09-16-p05-remaining-tasks.md` 任务 1

---

## 一、缺陷回顾

`backend/configs/messages.yaml` 的 `100005` 模板只插值 `${field}`：

```yaml
100005:
  zh_CN: "${field}不能为空"
  en_US: "${field} cannot be empty"
```

而 CSV 导入链路的调用方传的是三件套（`internal/service/device_pre_register.go`）：

```go
errcode.WithVars(100005, map[string]interface{}{
    "field":   "batch_file",
    "csv_row": i + 2,
    "message": "device_number and name are required",
})
```

`message` 与 `csv_row` **传了但模板不用**，最终渲染成「batch_file不能为空」。

危害不止是文案：它把排查者**系统性地指向错误方向**——上游文档自述因此被误导两轮
（先怀疑 FormData 被清空、再怀疑字段名不匹配、又怀疑代理损坏请求体），
而**后端从头到尾都是对的**。

---

## 二、为什么不能按原方案"让渲染优先使用调用方 message"

上游建议的修法是在渲染层做全局覆盖（有 `message` 就用 `message`）。**实测否决了这个方案**：

`100002`（`CodeParamError`）同样被以 `message` 形式传入具体原因：

| 位置 | 传入内容 |
| --- | --- |
| `internal/api/board.go:372` | `start_time must be less than or equal to end_time` |
| `internal/api/board.go:379` | `time range must not exceed 30 days` |
| `internal/service/device_group.go:199` | `old group id is same as new group id` |

而**两处自动化用例硬断言了 100002 的通用文案**：

- `automation_tests/tests/17_api_boundary_smoke.test.js:96`
  → `expect(downloadBody.message).to.equal('请求参数验证失败')`
- `automation_tests/tests/helpers/casbin_fixtures.js:19`
  → `expect(resp.message).to.equal('请求参数验证失败')`

**全局覆盖会让这两条立刻变红**，且会把大量既有端点的报错文案一次性改掉。

---

## 三、实际采用的方案：新增两个能承载上下文的错误码

**渲染层零改动**，改为把"能承载行号/原因"的责任交给专用错误码。

### 3.1 新增错误码（`backend/configs/messages.yaml`）

```yaml
  100006:
    zh_CN: "第 ${csv_row} 行：${message}"
    en_US: "Row ${csv_row}: ${message}"
  100007:
    zh_CN: "CSV 文件不合规：${message}"
    en_US: "Invalid CSV file: ${message}"
```

两者都沿用既有的 `${var}` 替换机制（`internal/middleware/response/response.go:136` 的
`replaceVariables`），因此**不需要任何渲染层改动**。

### 3.2 调用点改造（`backend/internal/service/device_pre_register.go`）

| 位置 | 场景 | 原 | 新 |
| --- | --- | --- | --- |
| `buildFilePreRegisterRows` 缺字段 | 逐行 | `100005` | **`100006`** |
| `buildFilePreRegisterRows` 编号超长 | 逐行 | `100005` | **`100006`** |
| `buildFilePreRegisterRows` 超行数上限 | 文件级 | `100005` | **`100007`**（`max` 此前同样被吞掉） |
| `readPreRegisterImportCSV` 路径不合规 | 文件级 | `100005` | **`100007`** |
| `readPreRegisterImportCSV` 解析失败 | 文件级 | `100005` | **`100007`** |
| `readPreRegisterImportCSV` 空文件 | 文件级 | `100005` | **`100007`** |
| `readPreRegisterImportCSV` 表头不符 | 文件级 | `100005` | **`100007`** |

顺带修掉一个**死变量**：表头错误此前传 `actual_first`（实际表头），
但没有任何模板消费它，同样被丢弃。现折进 `message`：
`csv header must be device_number,name (actual: sn,name)`。

**保留 `100005` 的 4 处调用点**（`create_type` / `device_count` / `batch_file` 为空 ×2）
行为一字不变——它们本就只传 `field`，模板语义正确。

---

## 四、验证证据

### 4.1 编译

```
cd backend && GOTOOLCHAIN=local go build -p 1 ./...
BUILD_EXIT=0
```

### 4.2 定向测试

```
cd backend && GOTOOLCHAIN=local go test -p 1 ./internal/service/ -run 'PreRegister|TrimCSV' -count=1 -v
```

```
--- PASS: TestReadPreRegisterCSVRejectsUnsafePaths
--- PASS: TestReadPreRegisterCSVRequiresStrictHeader
--- PASS: TestTrimCSVCellsTrimsWhitespaceOnly
--- PASS: TestBuildFilePreRegisterRowsReportsCsvRowOnMissingField      (新增)
--- PASS: TestBuildFilePreRegisterRowsCsvRowFollowsDataIndex           (新增)
--- PASS: TestBuildFilePreRegisterRowsReportsHeaderMismatchWithActualHeader (新增)
--- PASS: TestBuildFilePreRegisterRowsNeverUsesGenericEmptyFieldCode   (新增)
--- PASS: TestCreateDevicePreRegisterFileRejectsBadHeaderAndMissingField  (既有，未回归)
ok  aetherlink-iot/backend/internal/service  0.594s
```

### 4.3 真实配置渲染契约（新增测试文件）

`backend/internal/middleware/response/messages_config_test.go` 刻意读取**仓库真实**的
`configs/messages.yaml`（而非内联 YAML）——本次缺陷恰恰是"模板吞掉子原因"，
只测内联模板会漏掉真实模板写错的情况。

```
cd backend && GOTOOLCHAIN=local go test -p 1 ./internal/middleware/response/ -count=1 -v
```

```
--- PASS: TestRealConfigRendersCsvRowAndReason
--- PASS: TestRealConfigRendersCsvFileLevelReason
--- PASS: TestRealConfigKeepsGenericEmptyFieldTemplate
--- PASS: TestRealConfigKeepsSharedCodesUnchangedEvenWithCallerMessage
--- PASS: TestRealConfigRegistersCsvErrorCodes
--- PASS: TestRealConfigRendersCsvRowThroughFullHttpResponseChain
ok  aetherlink-iot/backend/internal/middleware/response  0.371s
```

其中 `TestRealConfigRendersCsvRowAndReason` 对 zh_CN / en_US 双语断言：
必须含真实原因、必须命中 `\b2\b`（与浏览器 E2E 的断言口径对齐）、不得回退成「不能为空」、
不得残留未替换的 `${`。

`TestRealConfigKeepsSharedCodesUnchangedEvenWithCallerMessage` 是**防回退锁**：
它钉死"不做全局覆盖"这个决定——若日后有人给共享码加上 message 覆盖，
100005/100002 会立刻变红，并直接指向那两处硬断言的用例。

### 4.4 HTTP 边界端到端（真实配置 + 真实中间件）

`TestRealConfigRendersCsvRowThroughFullHttpResponseChain` 用**仓库真实配置**装配
`Handler`，走真实 Gin 中间件，解析最终写出的 JSON 响应体，断言：

- `code == 100006`
- `message` 含 `device_number and name are required`
- `message` 命中 `\b2\b`
- 不含「不能为空」、不含残留 `${`

这条补上了"单测渲染正确"与"浏览器看得见"之间的最后一环
（服务层返回 `*errcode.Error` → 中间件按真实模板渲染 → 序列化成 JSON）。
它使**后端一半**的判据不再依赖活栈可用性。

### 4.5 回归

```
cd backend && GOTOOLCHAIN=local go test -p 1 ./internal/service/ ./internal/middleware/... -count=1
ok  aetherlink-iot/backend/internal/service            2.838s
ok  aetherlink-iot/backend/internal/middleware         2.236s
ok  aetherlink-iot/backend/internal/middleware/response 0.212s
EXIT=0
```

---

## 五、浏览器 E2E 复跑：完成判据达成（2026-09-17）

### 5.1 前置条件

后端已重启（新进程加载了新的 `messages.yaml`——该文件**只在启动时加载，无热重载**，
不重启会拿到旧模板而把修复误判为失败）。前端 prod 产物已重建并替换
（`vite build --outDir dist-new` → 备份旧 dist → 换名），预览代理 `127.0.0.1:9725` 已就绪。

### 5.2 结果

```
cd automation_tests
export AETHERLINK_DB_PASSWORD=<本地测试库密码>
PLAYWRIGHT_REUSE_EXISTING_SERVER=1 npx playwright test e2e/28_p05_preregister_csv.spec.js --reporter=list
```

```
5 passed (9.3s)
```

**两条 `.fixme` 已全部转正并通过**：

| 用例 | 修复前 | 现在 |
| --- | --- | --- |
| 坏行逐行反馈：缺字段的行要带上 `csv_row` 行号 | `.fixme`（渲染成「batch_file不能为空」，无行号） | **passed** |
| 表头不合规的文件被拒绝 | `.fixme` | **passed** |

判定口径与浏览器断言一致：坏行反馈必须命中 `/\b2\b/`，即真的把行号渲染出来了。

### 5.3 顺带验证

同一次活栈复跑也验证了 P1.6 的浏览器 E2E（`e2e/29_p16_template_upgrade_rollback.spec.js`）
**3/3 全绿**，确认「升级抽屉空态文案失效」的修复生效（该缺陷是 `NDataTable` 只有 `empty`
插槽、没有 `empty-text` prop 导致的）。

---

## 六、仍未闭环（如实记录）

1. **`products` 无任何创建路径**（上游文档任务 2）**未动**——它需要新迁移登记 Casbin，
   而当前 `104~107.sql` 均为未提交的在途文件，此刻新增迁移有断裂风险。
   **这是 P0.5 目前唯一的阻断项。**
2. `devices.voucher` 明文边界不变（本项不涉及）。

---

## 六、影响面

| 面 | 影响 |
| --- | --- |
| API 响应结构 | 无变化（仍是 `{code, message, data}`） |
| 错误码语义 | 新增 100006 / 100007；100005 / 100002 行为不变 |
| 前端 | 无需改动（`use-pre-register-import.ts` 直接展示 `message`） |
| OpenAPI | 无变化（错误码不在 schema 内） |
| 既有测试 | 无回归（service / middleware / response 三包全绿） |
