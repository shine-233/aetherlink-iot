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

## 五、UI 面闭环（2026-09-19 同日补齐，并顺带修复 TB-18 死菜单）

前端交付：

- `src/views/management/solutions/index.vue` 管理控制台（方案分页列表 / 新建方案
  弹窗含有序资源引用编辑 / 一键安装确认框与逐项结果面板 / 安装流水回查 / 删除）；
- `src/service/api/solution.ts` 5 个 wrapper 并入 barrel；
- 路由四件套（imports.ts / systemRoutes.ts / transform.ts / typings）登记
  `management_solutions`；114.sql 登记 sys_ui_elements 菜单行（orders 48）；
- 组件测试 4/4（创建裁剪与空值拦截、确认框触发、逐项 applied/failed、流水展示）。

**重要发现——TB-18 Secrets 页面此前实际不可达**：运行期控制台警告
`[route-adapter] skip invalid menu route: management_secrets`，即 109.sql 的菜单行
一直存在但 imports.ts 等路由四件套从未登记，页面挂不上（正是路线图 §1.3-B-2 记录
过的「菜单有了、路由没挂」陷阱）。本批同补 secrets 的四件套登记并加进 e2e 断言：
`skip invalid menu route` 警告数必须为 0。

**测试与证据**：typecheck 0 错误；服务导出快照更新通过；
**浏览器 E2E `e2e/32_tb19_industry_solution.spec.js` 1 passed（真实 Edge，2.7s）**：
菜单直达 /management/solutions → 种子方案行渲染 → 一键安装确认 →
「安装完成：共 2 项，成功 2 项，失败 0 项」逐项展示 →
控制台无任何 skip invalid menu route 警告。全程真实活栈。

## 六、仍未接线（如实）

- 方案内容目前覆盖 device_template + board_template 两类资源；规则链等更多资源
  类型需要在资源中心白名单扩展后再纳入方案引用。
