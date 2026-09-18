# TB-19 解决方案模板引擎（Industry Solution Templates）——全链路证据

> 日期：2026-09-19
> 对应路线图：§7.1 TB-19（对标 ThingsBoard CE 的解决方案模板引擎，`DefaultSolutionService`）
> 结论：**后端全链路闭环（迁移 113 / DAL / Service / API / Casbin / OpenAPI / 契约测试 7/7）**；
> 前端 UI 面未接线，如实记 `未接线`（UI 面），不得记 done。

## 一、立项前置检查（§7.4）

`grep -rni solution backend/{internal,router,sql}` 仅命中 `WidgetResolution`（画布解析，无关）。
**确认方案模板引擎零实现**，本行 `未实现` 判定成立。

## 二、设计与交付物

一个 solution = **有序资源引用清单**（device_template / board_template，复用资源中心
类型白名单）；安装 = 逐项走 TP-5 资源中心已验证的 `ApplyResource` 管道
（export→import，含租户归属校验），逐项留 append-only 安装流水。
**不建第二套打包/签名/冲突闸门**——那是 TP-5 / P1.6 的既有链路，本引擎只做编排。

| 层 | 文件 | 要点 |
| --- | --- | --- |
| 迁移 | `backend/sql/113.sql`（VERSION_NUMBER=113） | `industry_solutions`（同租户名唯一 + jsonb 引用清单 + 状态 CHECK）+ `industry_solution_installs`（append-only 流水）+ 3 条 Casbin 路由 |
| 模型 | `internal/model/industry_solution.go` | jsonb 列按仓库惯例用 `json.RawMessage + type:jsonb`（切片类型没有 driver.Valuer，直接落 jsonb 会失败——这是实现中修掉的第 1 个坑） |
| DAL | `internal/dal/industry_solution.go` | 全部查询显式 tenant_id；流水 append-only |
| 服务 | `internal/service/industry_solution.go` | **创建即逐条校验引用，但用只读的导出路径**（实现中修掉的第 2 个坑：初版用 ApplyResource 探测会凭空创建实例）；安装逐项 applied/failed 如实汇报，ContinueOnError 缺省 true |
| API/路由 | `internal/api/industry_solution.go` + `router/apps/industry_solution.go` | 5 端点 + swagger 注解 |
| OpenAPI | `docs/openapi/openapi.json` | 重生成 **454 paths**，`solutions` 3 条路径收录 |
| 契约测试 | `automation_tests/tests/62_industry_solution.test.js` | **7 用例活栈全绿（290ms）** |

## 三、语义边界（如实）

- **物模型模板导入是租户幂等的**（P1.6 语义），**看板导入每次实例化新看板**——
  即「每次安装装出一套新方案实例」，与 TB 解决方案模板"每次安装创建新资产"一致；
  重复安装不做合并，流水可追溯每次装出了什么。
- 单项失败不回滚已应用的项：逐项结果是「一键安装」的既定语义，
  流水如实记录 `applied`/`failed` 与错误文本；流水写失败只打日志，不掩盖安装事实。
- 删除方案不删除已安装的实例（方案只是引用清单）。

## 四、实测结果

**活栈契约测试**（后端 9999 + PG 17.5@55433 + Redis，`62_industry_solution.test.js`）**7/7 全绿**：

```text
  TB-19 industry solution templates [62_industry_solution]
    ✔ creates a solution referencing existing tenant resources without side effects
    ✔ rejects a duplicate solution name in the same tenant
    ✔ rejects references to resources that do not exist in the tenant
    ✔ lists solutions inside the tenant only
    ✔ installs the whole solution with per-item results and audit rows
    ✔ hides the solution from other tenants and blocks cross-tenant install
    ✔ deletes the solution and validates parameters (39ms)
```

关键断言：创建后 `installs` 为空（**创建无副作用**——这是与 ApplyResource 探测误
创建实例缺陷的区别性证据）；安装后逐项 `target_id` 非空、详情接口流水 ≥2 条 applied；
他租户对方案 detail/install 均 100404；同租户方案名唯一。

**运行期审计**：启动日志 `程序版本： 113` → `执行sql文件： sql/113.sql` →
`casbin route audit passed: 413 protected routes registered`（较 112 的 410 新增 3 条）。
回归：`go build ./...` exit 0；vet 通过；`internal/service` + `internal/dal` 全包 ok。

## 五、仍未接线（如实）

- **前端 UI 面**：无方案管理页/「一键装方案」按钮（后端契约、权限、流水面已闭环）。
- 方案内容目前覆盖 device_template + board_template 两类资源；规则链等更多资源
  类型需要在资源中心白名单扩展后再纳入方案引用。
