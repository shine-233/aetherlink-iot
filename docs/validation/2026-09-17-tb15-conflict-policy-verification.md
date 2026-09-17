# TB-15 实体名冲突策略：验收与两处修复

> 日期：2026-09-17
> 仓库：`aetherlink-iot`
> 对应路线图：§7.1 TB-15（对标 ThingsBoard 4.3.0 #14118）
> 上游需求：设备 / 产品创建接口支持 `conflict_policy`——FAIL（默认）/ RENAME / IGNORE / UPDATE

---

## 一、先说结论：**功能已经实现，不需要重写**

接到需求后先做源码核对（这是 §7.4 检查清单第 6 条的教训——**写过"未实现"之前先 grep 代码**），
结果是：**并行工作流已经按我加的路线图条目把 TB-15 实现了**。已落地的部分：

| 层 | 位置 | 内容 |
| --- | --- | --- |
| 策略模型 | `internal/model/conflict_policy.go` | `ConflictPolicy` 常量、`NormalizeConflictPolicy`（含 `reject`/`error`/`skip`/`overwrite`/`merge`/`auto_rename` 等别名）、`GenerateRenamedName` |
| 产品（`device_config`） | `service/device_config.go` | FAIL / RENAME / IGNORE / UPDATE 四个分支齐全 |
| 设备 | `service/device_create.go` | 同上；批量创建 `device_batch_create.go` 也已接 |
| 看板 / 资产 | `service/board.go`、`service/asset.go` | 一并接入 |
| API 绑定 | `api/device.go`、`api/asset.go`、`api/board.go` | body 与 query 双通道读取 `conflict_policy` |
| HTTP 结构体 | `devices.http.go`、`device_configs.http.go`、`boards.http.go` | `validate:"omitempty,oneof=fail rename ignore update allow"` |
| 测试 | `service/conflict_policy_test.go` | 24 例（策略归一化 20 + 更名 4） |

需求里的四条语义逐条对上：

- **FAIL（默认）**：`NormalizeConflictPolicy(nil)` 与空串都回落 `fail`；冲突时返回
  `CodeParamError`。✓
- **RENAME**：`GenerateRenamedName` 自增追加 ` (1)`、` (2)`…（含跳号场景）。✓
- **IGNORE**：返回既有实体。✓
- **UPDATE**：就地更新并返回。✓

**关于"产品"的落点**：本仓库里 `products` 表**没有任何创建端点**（全仓 0 命中），
而 `device_configs` 的路由就是 `device_config`，CSV 预注册里也叫 products。
因此需求中的"产品"对应 **`device_configs`**，该路径已实现。

---

## 二、修复 1：RENAME 在边界上必然失败（**真实缺陷**）

### 问题

`GenerateRenamedName` 原实现**没有长度上限**。而：

- `device_configs.name` 是 **`varchar(99)`**，HTTP 校验是 **`max=99`**；
- 其余实体（`devices` / `boards` / `device_templates`）是 `varchar(255)` / `max=255`。

PostgreSQL 的 `varchar(N)` 按**字符**计数并在超长时报错
（`value too long for type character varying(N)`）。于是：

> 用户提交一个**合法的 99 字符名称** → 冲突 → RENAME 生成 `99 字符 + " (1)"` = **103 字符**
> → **INSERT 被数据库拒绝**。

也就是说 **RENAME 会在它唯一该起作用的场景下退化成一次数据库错误**，与
「RENAME 保证租户内唯一并**成功创建**」的语义直接冲突。这是最不容易在手工测试里
撞到、最容易在生产上炸的组合（需要名称恰好接近上限）。

影响面是 **5 个调用点**：`device_config.go`、`device_create.go`、`device_batch_create.go`、
`board.go`、`asset.go`。

### 修复

`GenerateRenamedName` 增加 `maxLength` 参数，并在候选构造时**先截断 base 再追加后缀**，
保证结果始终放得进列宽：

```go
func GenerateRenamedName(baseName string, maxLength int, nameExists func(string) bool) string
```

两个关键细节：

1. **截断按 rune 计数，不是字节**。中文名在 `varchar(N)` 里算 N 个字符但占 3N 字节，
   按字节截断会把一个汉字砍成半个，产出**非法 UTF-8**（数据库会再报一次错，
   而且错误信息完全指不到根因）。
2. **上限小于后缀本身时不产出空名**，退回后缀本身——空名比超长更难排查。

调用点按实体列宽传参：`device_configs` 传 99，其余传 255。

---

## 三、修复 2：冲突策略检查插错位置，破坏了既有契约

### 问题

并行实现把冲突策略块放在了 `CreateDeviceConfig` 的**最前面**，早于 JSON 字段校验。
而既有用例 `TestDeviceConfigCreateRejectsInvalidJSONFieldsBeforeDAL` 的契约是
**"非法 JSON 字段必须在触达 DAL 之前被拒"**。策略块会调 `dal.GetDeviceConfigByNameAndTenant`
去查重，于是：

```
panic: runtime error: invalid memory address or nil pointer dereference
  gorm.io/gorm.(*DB).getInstance
  dal.GetDeviceConfigByNameAndTenant          dal/device_config.go:473
  service.(*DeviceConfig).CreateDeviceConfig  service/device_config.go:95
  service.TestDeviceConfigCreateRejectsInvalidJSONFieldsBeforeDAL
```

**单测环境下 DB 未初始化 → 空指针 panic。** 这条用例在本次改动前是绿的。

### 修复

把策略块移到 `additional_info` / `protocol_config` 两处 JSON 校验**之后**，
同时把 `deviceconfig.Name = req.Name` 一起下移——否则策略块改名后，
名称赋值仍用的是旧值，**RENAME 会静默不生效**（这种"改了但没生效"比报错更难发现）。

调整后的顺序：

```
JSON 字段校验（非法即拒，不触 DAL）
  → 冲突策略消解（可能改写 req.Name）
    → deviceconfig.Name = req.Name
```

---

## 四、验证证据

```
go build -p 1 ./...                                            BUILD=0
go test -p 1 ./internal/service/ -run 'ConflictPolicy|GenerateRenamedName|TestDeviceConfigCreateRejectsInvalidJSONFieldsBeforeDAL' -count=1
--- PASS: TestNormalizeConflictPolicy                          (20 子用例)
--- PASS: TestGenerateRenamedName                              (4 子用例)
--- PASS: TestGenerateRenamedNameRespectsLengthLimit           (8 子用例，本次新增)
--- PASS: TestDeviceConfigCreateRejectsInvalidJSONFieldsBeforeDAL  ← 修复前 panic，现通过
ok  aetherlink-iot/backend/internal/service  0.300s

go test -p 1 ./internal/service/ ./internal/api/ ./internal/model/ -count=1
ok  internal/service  2.713s / internal/api  0.302s / internal/model  0.177s
```

新增的 8 条长度边界用例：

| 用例 | 防的是什么 |
| --- | --- |
| 满长 ASCII 名追加后缀不得超限 | 99 字符 + ` (1)` = 103 超长（本次修复的主目标） |
| 满长中文名不得被截成非法 UTF-8 | 按字节截断把汉字砍成半个 |
| 连续冲突时每一个候选都不超限 | 序号增大时后缀变长，仍要放得下 |
| 原名未占用且刚好满长时原样返回 | 不该无谓改名 |
| 原名本身超长时收敛到上限内 | 历史数据 / 绕过 HTTP 校验的写入 |
| 上限小于后缀本身时不产出空名 | 退化输入 |
| 空名与纯空白名有确定行为 | 原实现会产出 `unnamed_<unix秒>`，同秒两次会撞名 |
| 全部候选被占用时可终止 | 防死循环 |

---

## 五、仍未闭环（如实记录）

1. **前端未接**：全仓 `frontend/src` 对 `conflict_policy` **0 命中**，
   界面上没有"名称冲突时如何处理"的选择项。当前只能由 API 调用方传参。
2. **无运行期证据**：本次只跑了单测与编译，**没有在活栈上用真实 HTTP 请求
   验证四种策略的端到端行为**（活栈在本次工作中处于停止状态）。
   因此缺口类型为 **`未验证`**，不宣称 TB-15 完成。
3. **`allow` 策略的定位**：`ConflictPolicyAllow` 可显式放行同名，
   但 `devices` / `boards` 等表的名称列**没有唯一约束**，放行后是否会产生歧义
   取决于下游查询如何取用；本次未追查。
4. **并发创建同名**：两个请求同时以 RENAME 创建同名实体时，查重与插入之间存在
   竞态窗口，可能产出两个 `xxx (1)`。本次未加锁或唯一约束兜底。
