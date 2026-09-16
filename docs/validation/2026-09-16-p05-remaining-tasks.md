# P0.5 剩余两项任务（可直接执行）

> 背景与完整证据：`docs/validation/2026-09-15-p05-csv-browser-evidence.md`
> 用例：`automation_tests/e2e/28_p05_preregister_csv.spec.js`（当前 **3 passed / 2 skipped**）
>
> 这两项是 P0.5 从"阻断"走到"结案"的最后一段。**前三个缺陷（上传全站失效、
> 凭证不渲染、错误不展示）已修并端到端验证**，这里只剩下面两条。

---

## 任务 1：错误模板吞掉子原因（后端，需配套测试）

### 现象
提交含坏行的 CSV 时，后端**正确**识别出问题行，但用户和调用方看到的却是：

```json
{"code":100005,"message":"batch_file不能为空"}
```

真实原因（`message: "device_number and name are required"`、`csv_row: 2`）被完全丢弃。

### 根因（已定位到具体一行）
`backend/configs/messages.yaml:26`

```yaml
100005:
  zh_CN: "${field}不能为空"
  en_US: "${field} cannot be empty"
```

模板**只插值 `${field}`**。而调用方在 `internal/service/device_pre_register.go` 里
传的是三件套：

```go
errcode.WithVars(100005, map[string]interface{}{
    "field":   "batch_file",
    "csv_row": i + 2,
    "message": "device_number and name are required",
})
```

`message` 与 `csv_row` 传了但模板不用，于是渲染成「batch_file不能为空」。

### 为什么这个缺陷值得修（不只是文案问题）
它把排查者**系统性地指向错误方向**。本轮我自己被它误导了两轮：
先怀疑 FormData 被清空、再怀疑字段名不匹配、又怀疑代理损坏请求体，
而**后端从头到尾都是对的**。

### 建议修法
让渲染**优先使用调用方提供的 `message`**，无 `message` 时回退到 `${field}不能为空`；
并把 `csv_row` 一并透出（例如「第 2 行：device_number and name are required」）。

### ⚠️ 必须注意的影响面
`100005` 是**全站共享错误码**，被多处使用。改模板会同时改变所有这些端点的报错文案。

**必须做的配套动作**：
1. 全仓搜 `WithVars(100005` / `New(100005)`，列出所有调用点；
2. 确认每个调用点是否都传了 `message`；没传的会走到回退分支，行为不变；
3. 跑一遍后端测试（`go test -p 1 ./...`）与相关 API 契约用例；
4. 更新 `pkg/errcode/error_language_manager_test.go`（如涉及渲染规则）。

### 完成判据
`e2e/28_p05_preregister_csv.spec.js` 的
「坏行逐行反馈：缺字段的行要带上 csv_row 行号」去掉 `.fixme` 后通过。

---

## 任务 2：`products` 没有任何创建路径（需独立立项）

### 现象
预注册必须选一个产品，但 `products` 表**全系统 0 行**，且**没有任何办法创建**。

### 穷尽核查结果
| 检查项 | 结果 |
|---|---|
| 后端产品路由 | **只有 `GET /product`**（下拉数据源，`router/apps/product.go:16`）。无 POST/PUT/DELETE |
| 前端调用 | `src/service/product/list.ts:18` 的 `addProduct` → `POST /product` → **404** |
| 该前端封装被谁用 | **只有单测引用，没有任何 UI 用它** |
| `CreateProductReq` 模型 | `internal/model/products.http.go:8` 定义了，但**没有任何 handler 使用**（死类型） |
| DAL 写入 | `internal/dal/` 下只有 `product_select.go`（只读） |
| 迁移种子 | 无任何 `INSERT INTO products` |
| 前端产品管理页 | 无。`views/product/` 只有 pre-register / update-ota / update-package；其 README 自述"暂未包含手写 .vue，作为后续页面归档入口" |
| **全后端唯一写 `products` 的地方** | `internal/service/device_preregister_cleanup_postgres_test.go:55` —— **测试用裸 SQL 插一行** |

### 后果
预注册页的产品下拉**永远为空** → `canSubmit` 恒为 false →
**「创建设备」按钮永久禁用**。

也就是说：**P0.5 的导入流程在真实环境里仍然不可达** ——
用户没有任何途径造出一个产品来。

### 为什么本次没顺手做
- 这是一个独立特性：后端 CRUD + DAL + 路由 + Casbin 授权 + 前端页面
- 它需要**新迁移**加 Casbin 策略，而当前迁移编号被并行工作流占用
  （`104.sql` / `105.sql` 在其工作树中未提交，此时提交 `106.sql` 会**断裂迁移链**）
- 需要产品决策：产品管理放平台级还是租户级

### 立项时需要一并决定
1. 产品管理的归属层级（平台级 / 租户级）
2. 是否需要产品管理 UI，还是只开放 API
3. 与既有 `device_config`（设备配置）的关系 —— 二者概念高度重叠，需明确边界

### 完成判据
预注册页的产品下拉能列出至少一个由**正常业务路径**（非 SQL）创建的产品，
且「创建设备」按钮可点击。

---

## 附：本用例的运行前置

`e2e/28_p05_preregister_csv.spec.js` 用 psql 种产品前置数据（沿用项目自己的做法，
见上面那个 Go 测试）。fail-closed 设计：

- 只允许 `127.0.0.1`
- **必须显式提供 `AETHERLINK_DB_PASSWORD`**，否则该文件整体 skip 并说明原因（不做假绿）

```bash
cd automation_tests
export AETHERLINK_DB_PASSWORD=<本地测试库密码>
PLAYWRIGHT_REUSE_EXISTING_SERVER=1 npx playwright test e2e/28_p05_preregister_csv.spec.js --reporter=list
```
