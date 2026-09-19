# TB-19 剩余缺口闭环：规则链纳入方案模板可引用资源（2026-09-19）

> 对应路线图：§7.1 TB-19「剩余：规则链等更多资源类型未纳入方案引用」
> 自动化用例：`automation_tests/tests/62_industry_solution.test.js`（8/8 全绿，
> 新增第 7 用例「references and installs a rule chain as a solution resource」）
> 运行结果：**VERDICT=PASS**

---

## 一、背景

TB-09-19 落地时方案引用白名单只有 `device_template` 与 `board_template`，
路线图如实记录剩余缺口「规则链等更多资源类型未纳入方案引用」。本轮补齐
`rule_chain`，方案模板从此可以「一键装一套含规则链的行业方案」。

## 二、交付物

1. **只读导出（`backend/internal/service/rule_chain.go` ExportChain）**——
   `ruleChainPortableExport{Name, Description, Enabled, Graph}`，graph 以
   `json.RawMessage` 内嵌避免 base64 再编码；租户归属校验在 DAL
   （`GetRuleChainByID(id, tenantID)`，not-found 返回 (nil,nil) 已处理为 404）；
   不含版本历史/死信/Trace 等平台侧运行数据。
2. **ApplyResource 新分支（`backend/internal/service/resource_center.go`）**——
   导出走只读 `ExportChain`，导入复用 `CreateChain` 的全部校验（graph 规范化、
   名称校验、租户取自 claims）；**每次安装实例化一条新链，源链只读不动**——
   与看板模板语义一致（每次安装实例化一套新资产）。
3. **方案白名单（`backend/internal/service/industry_solution.go`）**——
   `solutionResourceTypes` 增加 `rule_chain`；创建探测路径增加
   `GroupApp.RuleChain.ExportChain`（仍是只读探测，绝不产生副作用实例）；
   参数错误文案同步为三类型枚举。
4. **前端（`frontend/src/views/management/solutions/index.vue`）**——资源类型
   下拉增加「规则链」选项；组件测试补断言（5/5 全绿）。

## 三、实测（活栈：新构建 backend.exe + stub broker + PG）

```
TB-19 industry solution templates [62_industry_solution]
  ✔ creates a solution referencing existing tenant resources without side effects
  ✔ rejects a duplicate solution name in the same tenant
  ✔ rejects references to resources that do not exist in the tenant
  ✔ lists solutions inside the tenant only
  ✔ installs the whole solution with per-item results and audit rows
  ✔ hides the solution from other tenants and blocks cross-tenant install
  ✔ references and installs a rule chain as a solution resource (TB-19 剩余缺口闭环)
  ✔ deletes the solution and validates parameters
8 passing (299ms)
```

第 7 用例锁定的事实：引用合法规则链创建方案成功（探测零副作用）；他租户引用
本租户规则链被 100002 拒绝（探测路径按租户校验归属）；安装返回 applied 且
`target_id ≠ 源链 id`（实例化新链）；新链按 `target_name` 命名且 GET 可读；
源链原样可读。安装流水 `/rule-chains/<target>` 进 cleanup 回收。

前端组件测试：`npx vitest run src/views/management/solutions/__tests__/index.test.ts`
→ **5 passed**（含新增 rule_chain 选项断言）。

## 四、结论

- §7.1 TB-19 行的「剩余」子项中**规则链资源类型已闭环**；
- 「更多资源类型」（如 SCADA 文档、告警配置等）仍未纳入，维持诚实口径；
- 无新迁移、无新路由（复用既有 `/solutions` 与资源中心管道，Casbin 面不变）。
