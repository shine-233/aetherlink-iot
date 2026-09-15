# 2026-09-15 菜单驱动路由：19 条 403 的修复与上线验收

## 1. 问题

本项目 `VITE_AUTH_ROUTE_MODE=dynamic`，**授权路由由后端 `sys_ui_elements` 菜单驱动**。
把前端路由表（`frontend/src/typings/elegant-router.d.ts` 的 `RouteMap`，76 条）
与菜单 `param1` 做差集，再用真实浏览器逐条验证，结果 **19 条路由返回 403**。

关键特征：这些页面**代码写完了、路由注册了，但用户无处可达**。
守卫逻辑是"路由表里有条目、菜单里没有 → 跳 403（不是 404）"，所以它不会表现为
404 那么显眼，而是伪装成"权限不足"。

其中包含 **规则链、Dashboard、SCADA** 等核心功能页——不是边角料。

> 复现方法：`SELECT distinct param1 FROM sys_ui_elements WHERE param1 LIKE '/%'`
> 与 `RouteMap` 做差集。**注意 psql 在 Windows 上输出 CRLF，`comm` 前必须 `tr -d '\r'`**，
> 否则会全部失配（本次在这上面白跑了一轮）。

## 2. 修复（提交 `34ea86f`，迁移 `backend/sql/102.sql`）

补 18 条菜单行。**当时全部设 `param3='1'`（隐藏但可达）**，理由：
`param3='1'` → `hideInMenu`（`frontend/src/service/api/management.adapter.ts:249`）。
这些页面此前从未在真实浏览器验证过，直接摆上侧边栏等于把未经验证的界面推给用户。

`VERSION_NUMBER` 101 → 102。迁移带 `NOT EXISTS` 守卫，复跑 18 行全部 `INSERT 0 0`。

## 3. 验收（本次，主内容区而非"能否挂载"）

用 `automation_tests/scripts/diag-page-content.js`（本次新建）逐个打开，
**排除侧边栏/顶栏后只看主内容区**，并统计可见控件数。

### 3.1 为什么要单独做这一步

只断言 `document.title` 或 `body.innerText` 会被全局外壳（侧边栏/顶栏/面包屑）填满，
把**空壳页误判成正常页**。第一轮扫描只证明了"不再 403"，不等于"能用"。

### 3.2 判定规则（脚本内）

| 判定 | 含义 |
|---|---|
| `OK` | 主内容区有实质内容且有交互控件 |
| `EMPTY-OK` | **组件设计的空态**：列表没数据，但新增/搜索/分页等交互都在 |
| `EMPTY-BAD` | 真的没内容：有占位文案但没有控件 |
| `EMPTY-SHELL` | 主内容区几乎为空（通常是 SPA 未挂载） |
| `403` | 当前角色无权访问 |

> **教训**：第一版判定把 `EMPTY-OK` 和 `EMPTY-BAD` 混为一谈，把
> `/automation/rule-chain`（有"New Rule Chain + 搜索 + 分页"，只是租户没数据）
> 误判成坏页。**空租户打开列表页本来就该是空的**——用有缺陷的判定去隐藏页面，
> 等于把好页面冤枉了。判定必须区分"功能完好只是没数据"与"页面坏了"。

### 3.3 结果

| 页面 | 判定 | 说明 |
|---|---|---|
| `/dashboard`、`/dashboard/rdi-overview` | EMPTY-OK | 内容相同（父级重定向到首个子页），RDI 总览卡片 |
| `/dashboard/workbench` | OK | 运维工作台 |
| `/dashboard/workspace` | OK | 可视化入口页 |
| `/product`、`/product/pre-register` | OK | 内容相同；预注册有 10 行表格 |
| `/product/update-ota` | EMPTY-OK | OTA 升级列表 + 新增任务 |
| `/product/update-package` | OK | 升级包管理 |
| `/device/asset` | EMPTY-OK | "No assets yet, create a root asset first" + 新建入口 |
| `/device/entity-relation` | EMPTY-OK | 完整的创建关系表单 |
| `/management/entity-version` | EMPTY-OK | 快照列表 + 创建快照 |
| `/management/role` | EMPTY-OK | 角色列表 + Add Role |
| `/automation/rule-chain` | EMPTY-OK | 规则链列表 + New Rule Chain + 搜索 + 分页 |
| `/automation/rule-chain/edit` | OK | 规则链编辑器（含节点面板） |
| `/system-management-user/equipment-map` | OK | 设备地图 |
| `/visualization/scada` | OK | SCADA 画布（项目/画布选择与保存） |
| `/visualization/scada-editor` | OK | SCADA 编辑器 |
| `/apply/service` | OK（super_admin） | 服务/协议插件管理 |

**18 条全部功能可用**，无 pageerror、无失败请求。

## 4. 上线（16 可见 / 2 隐藏）

按"列表/控制台页上线、编辑页隐藏"的规则放开 `param3='0'`：

- **可见 16 条**：`dashboard`、`product`（父级分组）+ 上述 13 个叶子页 + `apply_service`
- **保持隐藏 2 条**：`automation_rule-chain-edit`、`visualization_scada-editor`

隐藏编辑页的理由：它们从列表页导航进入，且**与既有惯例一致**——
`automation_scene-edit`、`automation_linkage-edit`、`visualization_thingsvis-editor`、
`device_config-detail`、`device_details` 等 12 条全部是 `param3='1'`。

> 注：核库时发现这两条编辑页曾被改成 `'0'`（与 102.sql 的 `'1'` 不符，
> 应为其它进程改动），本次按惯例收回 `'1'`。

### 4.1 浏览器验证（不是只看数据库）

侧边栏实测新增了两个顶级分组：**「Operations & Visuals」**（= `/dashboard`）
与 **「Product Management」**（= `/product`），展开后 13 个子项全部出现。

## 5. 回归门禁（`automation_tests/e2e/27_menu_route_reachability.spec.js`）

**菜单是运行期数据、不在 git 里**，任何人改错 `param3` 或删掉菜单行都会静默回退。
本 spec 把结论固化，**19/19 通过**：

1. 13 条可见页：可达 + 渲染 + 页面标记可见
2. 2 条编辑页：可达（不 403）
3. 侧边栏出现 13 条可见项
4. **反向断言**：侧边栏**不应**出现编辑页（防过度放开）
5. `/apply/service`：super_admin 可达；**tenant_admin 正确 403**（权限边界断言，不是缺陷）

### 5.1 写用例时踩到的两个坑

- **403 页的 "No Permission" 只在 `document.title` 里**，正文只有 "Logout / Back to Home"。
  用 `getByText(/No Permission/)` 会一直等不到，必须断言 `page.title()`。
- 侧边栏文案必须按**实际渲染文本**写正则：`Pre-registration` 不等于 `Pre-register`，
  `OTA update` 不等于 `OTA Upgrade`，`Update package` 不等于 `Upgrade Package`。
  猜文案会让用例假红。

## 6. 工具

| 脚本 | 职责 |
|---|---|
| `scripts/diag-spa-mount.js` | 能不能挂载：抓 pageerror / 请求失败 / `#app` 是否为空；支持 argv 传路由与 `DIAG_ROLE` |
| `scripts/diag-page-content.js` | 挂起来之后**主内容区有没有东西**：排除外壳、统计可见控件、判定空态性质 |

> 两者的分工是刻意的：`diag-spa-mount.js` 回答"页面在不在"，
> `diag-page-content.js` 回答"页面能不能用"。第一轮只用前者，就漏掉了空壳页。
