# P1.6 模板市场与资源中心产品化：端到端浏览器 E2E 闭环证据

> 日期：2026-09-17  
> 仓库：`aetherlink-iot`  
> 对应路线图：§1.2 & §3 P1.6（模板市场与资源中心产品化）  
> 验证层级：真实 Playwright 浏览器 E2E（Microsoft Edge + 前端 Vite / Preview + 后端 9999 + PostgreSQL 55433）  
> 证据用例：`automation_tests/e2e/29_p16_template_upgrade_rollback.spec.js`  

---

## 一、闭环目标与测试背景

此前，P1.6 在后端数据面（98/99.sql 在真实 PostgreSQL 复跑）、API 契约面（45 组 15/15、41 组 5/5、53 组 21/21）以及前端组件单元测试（vitest 34/34）已全部就绪，但路线图要求"四面一致"，仍保留了最后一项未闭环项：**升级/回滚的真实浏览器 E2E 取证**。

本轮在完整在线活栈上运行 Playwright，针对物模型模板版本升级、不可变回滚点捕获与二次确认回滚全流程进行了真实 UI 交互验证。

---

## 二、真实浏览器 E2E 验证矩阵

自动化测试套件：`automation_tests/e2e/29_p16_template_upgrade_rollback.spec.js`

| 序号 | 测试用例 | 浏览器行为与断言点 | 实测耗时 | 结果 |
| :--- | :--- | :--- | :--- | :--- |
| 1 | 列表视图「版本历史」入口与抽屉挂载 | 打开 `/device/thingsmodel`，切换列表视图，按物模型名称精确检索，点击表格操作列「版本历史」按钮，验证 `TemplateUpgradeDrawer.vue` 成功展开并展示空历史说明与上传升级入口 | 2.2s | **PASS** |
| 2 | 「回滚到此点」二次确认与历史行数不变式 | 在已有 2.0.0 回滚点的抽屉中，点击「回滚到此点」弹出二次确认气泡（NPopconfirm），点击确认后弹出成功消息「物模型版本回滚成功」，断言历史记录行数保持 1 行不增（严格锁定回滚是幂等重放、不建行的实测业务语义） | 4.8s | **PASS** |
| 3 | 真实选文件升级与回滚点生成 | 通过抽屉中真实 `input[type="file"]` 注入合法的 2.0.0 新版本 JSON 文件，系统调用升级接口完成升级，界面吐出「物模型升级成功」Toast，历史列表中出现 2.0.0 的不可变回滚点并记录原始版本 1.0.0，空历史提示消失 | 6.9s | **PASS** |

**实测结果**：**3 passed (15.5s)**。

---

## 三、四面一致性验收

1. **数据库迁移面**：`98.sql`、`99.sql`、`106.sql`（表 `device_template_upgrade_history`、Casbin 路由、联合唯一索引）。
2. **后端服务与 API 面**：
   - `POST /api/v1/device/template/upgrade`
   - `POST /api/v1/device/template/upgrade/:history_id/rollback`
   - `GET /api/v1/device/template/upgrade/history`
   - 45 组 API 契约测试 15/15 全部通过。
3. **前端代码与单元测试面**：
   - `src/views/device/template/components/template-upgrade-drawer.vue`
   - `src/views/market/browse/bundle-import-model.ts`
   - vitest 34/34 全部通过。
4. **浏览器 E2E 面**：
   - `e2e/29_p16_template_upgrade_rollback.spec.js` 3/3 全部通过。

---

## 四、结论与路线图状态更新

P1.6 的全部缺口（升级/回滚运行期证据、验签/预览/覆盖闸门、TP-5 统一市场、前端视图以及真实浏览器 E2E）已全面闭环且取证完毕。

- **路线图状态**：P1.6 由 `partial` 正式更新为 **`done`**。
