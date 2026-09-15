# 2026-09-15 P0.5 预注册 CSV：浏览器 E2E 取证（发现 3 个缺陷）

## 0. 结论先说

P0.5 此前唯一的未闭环项是"真实浏览器 file chooser E2E（需活栈）"。
本轮把它补上，**一次抓到 3 个缺陷**，其中 2 个是阻断级：

| # | 缺陷 | 严重度 | 状态 |
|---|---|---|---|
| 1 | **CSV 上传在浏览器里失败**，导入流程走不通 | **阻断** | 未修（根因已隔离到前端） |
| 2 | **`products` 表没有任何创建路径**，产品下拉恒为空 | **阻断** | 未修（需独立立项） |
| 3 | 结果面板**从不渲染一次性凭证**，建完批拿不到凭证 | 高 | **已修**（本次） |

缺陷 1 和 3 都是"接口层与单测层全绿、浏览器里不可用"的类型 ——
**这正是 P0.5 门禁存在的意义**。

---

## 1. 缺陷 3（已修）：凭证从不渲染

### 现象
批量建档成功后，结果面板只显示一句"设备凭证仅在本结果面板展示一次"的提示，
**却从来不展示凭证本身**。

### 证据链（三层都通，只有渲染断了）
| 环节 | 状态 |
|---|---|
| 后端生成并回传 `voucher` | ✅ `internal/service/device_pre_register.go:39/95/136-141`（注释明写"一次性明文，仅本次响应可见"） |
| 前端 composable 接收并存入状态 | ✅ `use-pre-register-import.ts:136` `devices: data?.devices ?? []` |
| 类型定义 | ✅ `types.ts` `PreRegisterCreatedDevice.voucher` |
| **模板渲染** | ❌ **`index.vue` 里搜不到任何 `importResult.devices` 引用** |

### 后果
用户批量建了 N 台设备，**一台的凭证都拿不到**，这批设备实际无法接入。

### 修复
`views/product/pre-register/index.vue` 结果面板新增凭证区：
- 逐台展示 `device_number` + `voucher`，带复制按钮（`writeClipboardText`）
- 只提供"复制"，**不提供任何写回列表/本地存储的路径** —— voucher 是一次性明文，
  一旦落进持久状态就等于把"仅此一次"的约束作废
- 4 语言各补 3 个 i18n 键（`credentialsTitle` / `copyCredential` / `credentialCopied`）
- 样式用语义变量（`--border-color` 等），不引入硬编码 hex，避免触发 design-token 契约

> 注意：该修复的**浏览器侧最终确认被缺陷 1 阻断**（导入走不完就到不了结果面板）。
> 单测通过（`src/views/product/pre-register` 2 files / 5 tests），
> 待上传修好后 `e2e/28_*.spec.js` 的前四条转正即可端到端确认。

---

## 2. 缺陷 1（未修，根因已隔离）：CSV 上传在浏览器里失败

### 现象
点"创建设备"后：
- 只发出 `POST /api/v1/file/up`
- 响应 **HTTP 200**，但业务码是错的：
  ```json
  {"code":202001,"message":"请选择需要上传的文件"}
  ```
- **不会**发出 `POST /api/v1/device/preRegister`
- 页面既不显示结果面板、也不显示错误提示（只有一条 toast）
  → 用户感受就是"点了没反应"

> **排查陷阱**：只看"有没有 4xx/5xx"会完全漏掉这个 —— HTTP 状态是 200，
> 问题藏在业务码里。本仓库的响应约定是"业务错误也走 200"，检查必须看 body。

### 隔离结论：后端是好的，问题在前端
用 curl 直接打同一个端点（同一 token、同一表单字段）：

```bash
curl -X POST http://127.0.0.1:9999/api/v1/file/up \
  -H "x-token: $TOKEN" \
  -F "file=@p05.csv" -F "type=importBatch"
# → {"code":200,"message":"操作成功",
#     "data":{"path":"./files\\importBatch\\2026-09-16\\4cda47a4....csv"}}
```

后端 `UpFile` 读 `c.FormFile("file")` + `c.PostForm("type")`，字段名与前端一致。
所以是**浏览器发出的请求里没有 file 部分**。

### 2.1 2026-09-16 续查：拿到了请求报文，机制已明确，但源头仍未定位

**实测到的报文**（`page.on('request')`）：

```
content-type = application/json
postData     = "{}"        ← FormData 被序列化成了 JSON 空对象
```

**机制已经明确**。axios 的 `transformRequest` 对 FormData 是这样分支的：

```js
if (utils.isFormData(data)) {
  return hasJSONContentType ? JSON.stringify(formDataToJSON(data)) : data
}
```

即：**只要 Content-Type 里含 `application/json`，FormData 就会走 `formDataToJSON`**——
`File` 无法序列化，于是请求体退化成 `{}`，后端 `c.FormFile("file")` 自然拿不到文件。

**尝试过的修复（未生效，已撤回）**：
`packages/axios/src/options.ts:43` 的 `createAxiosConfig` 里预设了
`headers: { 'Content-Type': 'application/json' }`。把它删掉、清空 `node_modules/.vite`
缓存后重建，**运行时仍然发出 `application/json`**。因为该改动无法验证有效，
且会影响全站所有请求，**已 `git checkout` 撤回，不留在仓库里**。

**一个必须避免的排查陷阱**：
`grep 'Content-Type":"application/json' dist/assets/*.js` **不能作为判据** ——
`packages/axios/src/shared.ts:10` 的读取兜底 `config.headers?.['Content-Type'] || 'application/json'`
会编译出**完全相同的字面量**。这与本项目此前 `grep 127.0.0.1:9999` 是同一类错误。

**影响面（比单个页面大）**：全站有 3 处文件上传，都走同一个共享 axios 配置，
推测全部失效：
| 位置 | 用途 |
|---|---|
| `views/product/pre-register/use-pre-register-import.ts:83` | 预注册 CSV |
| `views/product/update-package/use-ota-package-form.ts:115` | OTA 升级包 |
| `service/api/personal-center.ts:55` | 个人中心上传（`/file/up`） |

### 2.2 2026-09-16 续查二：Content-Type 已修正，暴露出**第二个叠加缺陷**

在调用点显式声明非 JSON 的 Content-Type（`src/service/product/list.ts` 的
`uploadImportBatchFile`，第三个参数传 `headers: { 'Content-Type': 'multipart/form-data' }`）
后，请求头**已经正确**：

```
content-type = multipart/form-data; boundary=----WebKitFormBoundaryhqiRLKeMSgGByh1B
```

（这里写死不带 boundary 是**对的**：axios 的 xhr 适配器对 FormData 会
`setContentType(false)` 把头交给浏览器，由浏览器补 boundary。）

**但上传仍然失败**，后端依旧返回 `202001 请选择需要上传的文件`。
即：**这是两个叠加的缺陷，修好第一个才暴露出第二个。**

第二个缺陷的位置基本可锁定在**文件对象本身没被正确附上**：

- `index.vue:68` 是 `fileListToFile(list) { selectFile(list[0]?.file ?? null) }`
  —— 依赖 NaiveUI `UploadFileInfo.file`
- 若该 `file` 不是真正的 `File`/`Blob`，`FormData.append` 会把它转成字符串
  或不产生文件部分，后端 `c.FormFile("file")` 就取不到
- 注意 **`page.on('request')` 的 `postData()` 对二进制 body 会返回 `null`**，
  所以"body 里没有 Content-Disposition"这个观察**不能直接当作结论**，
  要用 `postDataBuffer()` 的长度或直接抓包确认

**下一步（收敛到很小的范围）**：
1. 在 `use-pre-register-import.ts` 的 `uploadSelectedFile` 里临时打印
   `selectedFile.value` 的 `constructor.name` / `instanceof File` / `size`
   （或直接在浏览器 console 里对同一段逻辑打点），确认它到底是不是 `File`。
2. 若不是 `File`：改用 `NUpload` 的 `customRequest`，或直接监听原生
   `input[type=file]` 的 `change` 取 `event.target.files[0]`；
   也可在 `onFileListChange` 里退化为 `list[0]?.file ?? list[0]?.fileList?.[0]`。
3. 修好后 `e2e/28_p05_preregister_csv.spec.js` 前四条去掉 `.fixme` 即可转正。

> 另注：`src/service/product/list.ts` 的 Content-Type 修复**已提交**，
> 它是必要但**不充分**的一步 —— 单独它不足以让上传成功。

**下一步建议（按优先级）**：
1. 在浏览器里对 `/file/up` 请求打 `page.on('request')`，打印**完整 headers 对象**，
   确认 `Content-Type` 到底是 axios 自动加的、还是被某个拦截器加的。
2. 若来自 axios 自动加：检查 `createFlatRequest` 内部是否把 `Content-Type` 写死
   （包源码 `packages/axios/src/{index,options,shared}.ts`），
   以及 `axios-retry` 是否在重试时重建了 config。
3. 若来自拦截器：`src/service/request/request.ts` 的 `onRequest` 只合并了
   `x-token` / `Accept-Language`，但 `@aetherlink/axios` 内部可能还有别的钩子。
4. **兜底方案（不推荐但可先止血）**：上传处显式传
   `{ headers: { 'Content-Type': 'multipart/form-data' } }` —— 注意必须让浏览器
   自己补 boundary，不要手写 boundary 字符串。

> 修好后把 `e2e/28_p05_preregister_csv.spec.js` 前四条的 `.fixme` 去掉即可转正。

### 原记录的待查方向（保留，作为交叉验证）
1. `NUpload` 的 `default-upload=false` 下，`@update:file-list` 给出的
   `UploadFileInfo.file` 是否真被填充 —— `index.vue:68` 是
   `selectFile(list[0]?.file ?? null)`，若 `.file` 为空则 `selectedFile` 为 null。
   但注意：若为 null，`uploadSelectedFile` 会在 `if (!selectedFile.value) return false`
   直接返回、**根本不会发请求**；而实测**发了请求**，说明 `selectedFile` 非空
   却没能作为 multipart 部分发出 —— 更像是请求层改写了 FormData。
2. 检查 `@aetherlink/axios`（`createFlatRequest`）对 `FormData` 的处理：
   该包 dist 里**搜不到任何 `FormData` 字样**，即未做特殊处理；
   确认它是否给 `FormData` 设置了 `Content-Type: application/json` 或做了序列化。
3. 用浏览器 devtools / `page.on('request')` 直接看该请求的
   `Content-Type` 与 body 是否真的是 `multipart/form-data`。

---

## 3. 缺陷 2（未修）：`products` 没有任何创建路径

### 现象
预注册必须选一个产品，但 `products` 表**全系统 0 行**，且**没有任何办法创建**。

### 穷尽核查
| 检查项 | 结果 |
|---|---|
| 后端产品路由 | **只有 `GET /product`**（下拉数据源）。`router/apps/product.go:16`。无 POST/PUT/DELETE |
| 前端调用 | `service/product/list.ts:18` `addProduct` → `POST /product` → **404**（接口不存在） |
| 该前端封装被谁用 | 只有单测引用，**没有任何 UI 用它** |
| `CreateProductReq` 模型 | `internal/model/products.http.go:8` 定义了，但**没有任何 handler 使用**（死类型） |
| DAL 写入 | `internal/dal/` 下只有 `product_select.go`（读） |
| 迁移种子 | 无任何 `INSERT INTO products` |
| 前端产品管理页 | 无。`views/product/` 只有 pre-register / update-ota / update-package；其 README 自述"暂未包含手写 .vue，作为后续页面归档入口" |
| **唯一写 `products` 的地方** | `internal/service/device_preregister_cleanup_postgres_test.go:55` —— **测试用裸 SQL 插一行** |

### 后果
预注册页的产品下拉**永远为空** → `canSubmit` 恒为 false → "创建设备"按钮**永久禁用**。
也就是说：**即使缺陷 1 修好，P0.5 的导入流程在真实环境里仍然不可达。**

### 为什么本次不顺手实现
- 这是一个独立特性（后端 CRUD + DAL + 路由 + Casbin + 前端页面），不是"补个测试"
- 它需要**新迁移**加 Casbin 授权，而当前迁移编号被占用（见 §4），提交会断裂迁移链
- 建议**独立立项**，并在立项时一并决定产品管理放在哪一层（平台级还是租户级）

### 本用例的临时处理
按**项目自己的先例**（上面那个 Go 测试）用 psql 种一行产品做前置数据，
fail-closed（只允许 127.0.0.1 + 必须显式给 `AETHERLINK_DB_PASSWORD`，否则整体 skip）。
这样用例能验证导入逻辑本身，同时把"产品无法创建"作为独立缺陷上报，不在这里发明 CRUD。

---

## 4. 交付物

| 文件 | 内容 |
|---|---|
| `automation_tests/e2e/28_p05_preregister_csv.spec.js` | P0.5 门禁五条用例。前四条依赖上传 → 标 `.fixme` 并写明根因；第五条（跨租户产品不可选）**通过** |
| `frontend/src/views/product/pre-register/index.vue` | 缺陷 3 修复：渲染凭证 + 复制 |
| `frontend/src/locales/langs/{zh-cn,en-us,es-es,fr-fr}/page.json` | 各补 3 个键 |

### 用例覆盖与门禁对照
| 门禁 | 用例 | 状态 |
|---|---|---|
| 真实浏览器选择文件 | 第 1 条 | fixme（被缺陷 1 阻断） |
| 坏行逐行反馈 | 第 2 条 | fixme（同上） |
| 批量建档 + 凭证展示 | 第 1 条 | fixme（同上） |
| 凭证只出现一次 | 第 4 条 | fixme（同上） |
| 跨租户产品不可选 | 第 5 条 | ✅ 通过 |

第 5 条内含**反向对照**：不仅断言"他租户产品不出现"，还断言"本租户产品必须出现" ——
否则"不存在"这类断言可能只是因为下拉根本没加载，永远为真。

---

## 5. 为什么这批缺陷 API 级测试抓不到

三个缺陷的共同特征：**每一层单独看都是对的**。

- 缺陷 3：后端回传了 voucher、composable 存了 voucher、类型定义了 voucher —— 只有模板没渲染
- 缺陷 1：后端 curl 完全正常、前端代码逻辑看起来也对 —— 只有真实浏览器发出去的报文不对
- 缺陷 2：读接口 `GET /product` 工作正常、类型定义齐全 —— 只有"写"这一侧整个不存在

**要抓到它们，必须真的在浏览器里走完一条端到端路径。**
这就是 P0.5 门禁"真实浏览器 file chooser E2E"不可替代的原因，
也是本轮把 P0.5 从"只差跑一遍"改写为"**阻断：有 2 个必修缺陷**"的依据。
