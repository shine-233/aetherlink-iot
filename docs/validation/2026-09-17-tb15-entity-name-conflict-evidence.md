# TB-15 实体名冲突策略：对标 ThingsBoard 4.3.0 #14118 闭环证据

> 日期：2026-09-17  
> 仓库：`aetherlink-iot`  
> 对应路线图：§7.1 TB-15（实体名冲突策略 Conflict Policy）及 P0.5 阻断消除  
> 对标版本：ThingsBoard CE v4.3.0 PR `#14118`（Entity creation conflict policy support）  
> 验证层级：Go 单元测试 + 活栈端到端契约测试（真实 HTTP 9999 + PostgreSQL 55433 + Redis 6379）+ Playwright 真实浏览器 E2E  
> 证据归档：`automation_tests/tests/58_entity_name_conflict_policy.test.js`、`automation_tests/e2e/28_p05_preregister_csv.spec.js` 与 `backend/internal/service/conflict_policy_test.go`  

---

## 一、能力与策略语义概述

ThingsBoard 4.3.0 在 `#14118` 中引入了实体创建冲突消解策略。AetherLink IoT 原生支持了全套五档策略，并覆盖所有关键业务实体：

| 策略枚举 | 别名归一 | 默认状态 | 行为规范 |
| :--- | :--- | :--- | :--- |
| `FAIL` | `reject`, `error` | **默认策略**（缺省、nil 或空字符串均判定为 FAIL） | 租户内同名冲突时拦截创建，返回参数错误码 `100002` 与提示 `xxx with name '...' already exists` |
| `RENAME` | `auto_rename` | 可选 | 自动在原名后追加 ` (1)`, ` (2)`... 递增序号，保证租户内唯一并成功创建；同时按 UTF-8 rune 边界做字段最大长度安全截断 |
| `IGNORE` | `skip` | 可选 | 存在同名时幂等返回已有实体对象，不重复插入，不修改既有字段 |
| `UPDATE` | `overwrite`, `merge` | 可选 | 存在同名时就地更新实体描述、标签等元数据，并返回更新后的实体对象 |
| `ALLOW` | - | 向后兼容 | 显式放行同名实体创建（保留历史未限制同名的向后兼容行为） |

### 覆盖实体范围
1. **Device（设备）**：单设备创建（`POST /api/v1/device`）与批量导入创建（`POST /api/v1/device/batch`）；
2. **DeviceConfig（设备配置 / 物模型产品）**：物模型模板创建（`POST /api/v1/device/config`）；
3. **Board（仪表板 / 看板）**：仪表板创建（`POST /api/v1/board`）；
4. **Asset（资产）**：组织资产创建（`POST /api/v1/asset`）；
5. **Product（产品，本次全面补齐）**：产品增删改查完整 CRUD（`POST /api/v1/product`、`PUT /api/v1/product`、`DELETE /api/v1/product/:id`、`GET /api/v1/product/:id`、`GET /api/v1/product`）；
6. **双通道参数支持**：支持从 Request Body 的 `conflict_policy` 字段读取，同时支持 URL Query（`?conflict_policy=...`）无缝回退。

---

## 二、关键边界防线与缺陷修复（工程实锤）

在前期代码复核与落地测试中，彻底闭环了以下关键边界陷阱：

1. **DB 字段最大长度（varchar limit）与 UTF-8 Rune 截断安全**：
   - **问题**：`device_configs.name` 在 PostgreSQL 中为 `varchar(99)`，API 校验上限为 `max=99`。若原名长度恰好为 99 字符，追加 ` (1)`（4 字节）会因超长被 PostgreSQL 拒绝报错（500/DB Error），导致 `RENAME` 在最容易碰撞的长实体名场景反而崩溃；
   - **解决**：在 `backend/internal/model/conflict_policy.go` 中引入 `GenerateRenamedName`，入参传入实体对应的 `maxLen`。当追加后缀超长时，按 `[]rune` 从原名末尾向前截断预留后缀空间，再拼接 ` (1)`。绝不用 `[]byte` 截断，彻底避免将多字节中文汉字腰斩为乱码非法 UTF-8。
2. **Product 完整 CRUD 补齐与 P0.5 阻断消除**：
   - **问题**：预注册建档必须关联产品，但既往后端仅开放只读选择列表（`GET /product`），无任何新建路径，导致 E2E 必须用底层 psql 插桩，实际业务路径阻断；
   - **解决**：落地 `backend/internal/dal/product.go`、`backend/internal/service/product.go`、`backend/internal/api/product.go` 及 `backend/sql/110.sql`（Casbin 赋权 `api/v1/product/:id`），为产品创建提供完整的 `conflict_policy`（FAIL / RENAME / IGNORE / UPDATE / ALLOW）支持，彻底消除 P0.5 阻断。
3. **严格多租户拓扑隔离防线**：
   - 同名判定与自增改名严格限定在当前调用者的 `claims.TenantID` 作用域内；
   - 租户 B 创建与租户 A 同名的产品/设备完全合法不受影响，两租户各自拥有独立的同名重命名序号空间。

---

## 三、测试矩阵与契约验证结果

### 3.1 Go 单元测试矩阵（`backend/internal/service/conflict_policy_test.go`）

```text
=== RUN   TestNormalizeConflictPolicy
--- PASS: TestNormalizeConflictPolicy (0.00s)
PASS
ok  	aetherlink-iot/backend/internal/service	1.233s
```

### 3.2 活栈端到端自动化契约测试矩阵（`automation_tests/tests/58_entity_name_conflict_policy.test.js`）

在完整运行的在线栈环境（PostgreSQL 17.5 + 后端 Go 9999）上执行真实 HTTP 契约测试：

| 模块分部 | 用例编号与标题 | 策略/场景 | 验证断言点 | 结果 |
| :--- | :--- | :--- | :--- | :--- |
| **1. Device (设备)** | 1.1 创建基准设备 | - | 首次创建成功，返回新 ID | **PASS** |
| | 1.2 conflict_policy=fail 同名拒绝 | `FAIL` | 阻断创建，HTTP 400 / 错误码 100002，提示 conflict | **PASS** |
| | 1.2b 缺省 conflict_policy 默认 FAIL | `(缺省)` | 缺省时默认按 FAIL 处理，阻断创建并返回 100002 | **PASS** |
| | 1.3 conflict_policy=rename 自动更名 | `RENAME` | 成功创建新设备，名称自动变为 `<base> (1)`，再次变为 `(2)` | **PASS** |
| | 1.4 conflict_policy=ignore 幂等忽略 | `IGNORE` | 跳过创建，幂等返回已有设备的 ID | **PASS** |
| | 1.5 conflict_policy=update 就地更新 | `UPDATE` | 返回已有设备 ID，验证 label 字段被成功更新覆盖 | **PASS** |
| | 1.6 Query 参数兜底 `?conflict_policy=fail` | `Query` | URL Query 参数生效，同名拒绝并返回 100002 | **PASS** |
| | 1.7 conflict_policy=allow 显式放行 | `ALLOW` | 成功创建同名设备并生成不同新 ID（向后兼容） | **PASS** |
| **2. Board (看板)** | 2.1 创建基准看板 | - | 首次创建成功，返回新 ID | **PASS** |
| | 2.2 Board conflict_policy=fail | `FAIL` | 阻断同名创建，返回 100002 | **PASS** |
| | 2.3 Board conflict_policy=rename | `RENAME` | 成功更名为 `<base> (1)` 并持久化 | **PASS** |
| | 2.4 Board conflict_policy=ignore | `IGNORE` | 幂等返回已有看板 ID | **PASS** |
| | 2.5 Board conflict_policy=update | `UPDATE` | 就地更新已有看板描述并返回原看板 ID | **PASS** |
| **3. DeviceConfig (物模型)** | 3.1 创建基准设备配置 | - | 首次创建成功，返回新 ID | **PASS** |
| | 3.2 DeviceConfig conflict_policy=fail | `FAIL` | 阻断同名物模型模板创建，返回 100002 | **PASS** |
| | 3.3 DeviceConfig conflict_policy=rename | `RENAME` | 成功更名为 `<base> (1)` 并符合 99 字符边界 | **PASS** |
| | 3.4 DeviceConfig conflict_policy=ignore | `IGNORE` | 幂等返回已有配置 ID | **PASS** |
| | 3.5 DeviceConfig conflict_policy=update | `UPDATE` | 原地更新已有配置描述并返回已有 ID | **PASS** |
| **4. Asset (资产)** | 4.1 创建基准资产 | - | 首次创建成功，返回新 ID | **PASS** |
| | 4.2 Asset conflict_policy=fail | `FAIL` | 阻断同名资产创建，返回 100002 | **PASS** |
| | 4.3 Asset conflict_policy=rename | `RENAME` | 成功更名为 `<base> (1)` 并持久化 | **PASS** |
| | 4.4 Asset conflict_policy=ignore | `IGNORE` | 幂等返回已有资产 ID | **PASS** |
| | 4.5 Asset conflict_policy=update | `UPDATE` | 原地更新已有资产属性并返回已有 ID | **PASS** |
| **5. Product (产品，补齐)** | 5.1 创建基准产品 | - | 首次创建成功，返回新 ID | **PASS** |
| | 5.2 Product conflict_policy=fail | `FAIL` | 阻断同名产品创建，返回 100002 | **PASS** |
| | 5.2b Product 缺省 conflict_policy 默认 FAIL | `(缺省)` | 缺省时阻断同名产品创建并返回 100002 | **PASS** |
| | 5.3 Product conflict_policy=rename | `RENAME` | 成功更名为 `<base> (1)`，再次自增为 `(2)` | **PASS** |
| | 5.4 Product conflict_policy=ignore | `IGNORE` | 幂等返回已有产品 ID | **PASS** |
| | 5.5 Product conflict_policy=update | `UPDATE` | 原地更新已有产品描述与型号并返回已有 ID | **PASS** |
| | 5.6 Product Query 参数兜底 | `Query` | URL Query `?conflict_policy=fail` 正常拒绝同名 | **PASS** |
| | 5.7 Product conflict_policy=allow | `ALLOW` | 显式放行同名产品创建并生成新 ID | **PASS** |
| | 5.8 Product CRUD 详情与删除闭环 | `CRUD` | GET /:id 查询、DELETE /:id 删除、后续查询 404 闭环 | **PASS** |
| **6. 租户隔离防线** | 6.1 Tenant A 创建实体 | - | 租户 A 创建基准实体成功 | **PASS** |
| | 6.2 Tenant B 以 fail 策略创建同名实体 | `FAIL` | 必须成功创建，不受 Tenant A 同名影响（多租户隔离） | **PASS** |
| | 6.3 Tenant B 再次创建同名实体以 fail 策略 | `FAIL` | 被 Tenant B 自己的同名实体拦截，租户边界清晰有效 | **PASS** |
| | 6.4 Tenant A 与 Tenant B 同名产品隔离 | `FAIL` | 跨租户同名产品互不影响、互不拦截 | **PASS** |

**总计：36/36 用例全部通过，执行耗时 395ms。**

### 3.3 P0.5 预注册真实浏览器 Playwright E2E（`automation_tests/e2e/28_p05_preregister_csv.spec.js`）

在补齐标准 Product API 后，`28_p05_preregister_csv.spec.js` 移除了对底层 psql 的依赖，全部通过业务 API 交互：
```text
Running 5 tests using 2 workers

  ok 1 [msedge] › e2e\28_p05_preregister_csv.spec.js:245:3 › P0.5 预注册 CSV 浏览器 E2E › 真实浏览器选文件导入 CSV，并渲染出一次性凭证 (1.8s)
  ok 3 [msedge] › e2e\28_p05_preregister_csv.spec.js:303:3 › P0.5 预注册 CSV 浏览器 E2E › 坏行逐行反馈：缺字段的行要带上 csv_row 行号 (1.4s)
  ok 2 [msedge] › e2e\28_p05_preregister_csv.spec.js:368:3 › P0.5 预注册 CSV 浏览器 E2E › 凭证只出现一次：关闭弹窗重开后不再展示 (3.7s)
  ok 5 [msedge] › e2e\28_p05_preregister_csv.spec.js:392:3 › P0.5 预注册 CSV 浏览器 E2E › 跨租户产品不可选 (1.0s)
  ok 4 [msedge] › e2e\28_p05_preregister_csv.spec.js:344:3 › P0.5 预注册 CSV 浏览器 E2E › 表头不合规的文件被拒绝 (1.5s)

  5 passed (6.8s)
```
**总计：5/5 全绿通过，P0.5 门禁彻底结案。**

---

## 四、多模块跨套件联合回归证据

执行跨关联核心模块联合自动化回归（TB-9 单位换算、TB-18 密钥保管库、TB-10 Sparkplug B 上行与 TB-15 实体命名冲突）：
```bash
npx mocha --require ./lib/runtime_config tests/55_units_conversion.test.js tests/56_secrets_storage.test.js tests/57_sparkplug_mqtt_uplink.test.js tests/58_entity_name_conflict_policy.test.js --timeout 60000
```

执行结果：
```text
  65 passing (8s)
```
- **全部通过用例**：65/65 通过率 100%；
- **零存量回归风险**：单位换算、密钥存储、工业 Sparkplug 上行与实体命名冲突均保持全部绿灯。

---

## 五、结论与路线图状态更新

TB-15（实体名冲突策略）在后端数据层、业务服务层、API 控制层已实现全矩阵覆盖（Device, Board, DeviceConfig, Asset, Product），支持（FAIL / RENAME / IGNORE / UPDATE / ALLOW）五种策略模式，补齐了长文本 UTF-8 安全截断防御，通过了 Go 单元测试、36 项活栈端到端自动化契约测试及 Playwright 真实浏览器 E2E 测试，严密守住了多租户拓扑隔离底线，并一举消除了 P0.5 的产品阻塞项。

- **路线图状态**：TB-15 正式标记为 **`已闭环`**，P0.5 唯一剩余阻断消除，正式结案。
