## 2026-07-09 Coverage Hygiene Refresh

- The lightweight harness command caught a real fake-coverage residue before it was fixed: `RdiDeviceOperationsView.test.ts` used `toBeTruthy()` for the RDI data collection interval input.
- The assertion now verifies concrete `NumberInputStub` props (`value: 60`, `min: 45`, `max: 60`), and the harness trust pack is back to `51 passing`.
- A later cleanup in `tests/06_system.test.js` removed local nullable/conditional-empty list assertions and replaced them with concrete paged-list shape plus per-row checks.
- A later cleanup in `tests/02_device.test.js` made the device list prove the seeded device row is present instead of accepting an empty list or arbitrary first row.
- The same `tests/02_device.test.js` slice now creates a group and proves it appears in the group tree while recursively validating node shape.
- Keep this pattern: when `00_oracle_contract` fails, prefer strengthening the business assertion in the source test over relaxing the contract scanner.
- Release API/E2E is still not done. `preflight-api-e2e` continues to fail closed on missing proxy/environment, which is expected until real release config is provided.

# AetherLink IoT 开发指南

## 2026-07-09 Compatibility Boundary

Compatibility cleanup is split into three buckets:

- API/DB/SQL seed names, including `device_service_details`, `template_secret`,
  `templateId`, `device_template_id`, `sample_limit`, `sample_devices`,
  `sample_topic`, and `sample_payload`.
- Route/menu/old-link compatibility, including `/first-device`,
  `/device/config`, `/tv-preview`, `/terms`, and `/privacy`.
- Protocol/runtime/persisted-config compatibility, including ThingsVis embed/SSO
  keys, RDI JSON fields, `x-token`, `additional_info` bridges, GMQTT upstream
  module names, broker plugin hooks, and MQTT packet/topic contracts.

New code should prefer current AetherLink naming and semantic aliases, but these
compatibility names must not be deleted or renamed without a migration plan,
dual-read/write where needed, and fresh focused verification.

## 2026-07-09 Latest Verification Note

- Focused frontend verification improved again this round: `telemetry.test.ts` is now `35/35 passing`, `automationDryRunPreview.test.ts` is `9/9 passing`, and `homeFirstDeviceWorkbench.test.ts` is `17/17 passing`.
- `/first-device` internal onboarding focus naming now prefers `focus=test`; legacy `focus=sample`, `tester`, and `browser_test` inputs are still accepted and mapped to the same onboarding section.
- Automation dry-run wording was cleaned from old `Device-template` text to `Thing model` wording in the touched preview slice.
- Harness trust pack was re-run after fixing an RDI weak assertion and is `51 passing`, but this remains harness trust evidence only, not release runtime closure.
- Release API/E2E is still environment-blocked by missing real release accounts, `PREVIEW_URL`, `API_TARGET`, the `9725` preview proxy, and `PLAYWRIGHT_USE_PREVIEW_PROXY=1` / `PLAYWRIGHT_REUSE_EXISTING_SERVER=0`.

## 2026-07-11 Home Secondary Panel Rule

- When slimming `frontend/src/views/home/index.vue`, treat the first-device
  workbench as the parent-owned orchestration surface and peel out only the
  second-screen state panels first.
- Keep layout resolution, route pushes, onboarding focus, and
  `useViewportDeferredMount` ownership in the parent page.
- Presentational states such as resolving cards, compat notices, and the
  ThingsVis iframe container can move into a child component as long as the
  parent still decides when the section mounts and when the iframe should load.
- This pattern reduces homepage file size without changing the first-device
  customer task flow or claiming runtime performance proof that was not
  measured.

## 2026-07-12 Lower Panel Extraction Rule

- After the major workbench/state composables are already split, prefer the
  next seam in lower customer-visible panels before touching decision logic.
- In `command-center`, keep route/query parsing, preview/submit orchestration,
  history loading, and draft lifecycle in the parent. Safe child seams are
  history placeholders, draft notices, and other read-only lower panels.
- In `onboarding-ready-check`, keep collectors, CTA priority resolution, and
  navigation side effects in the parent. Safe child seams are the bottom action
  surface and other render-only evidence or handoff blocks.
- If a child only renders and emits actions, keep the parent as the sole owner
  of priority chains and side-effectful handlers to avoid customer-visible
  behavior drift.
- For the first-device workbench, delayed lower sections can move together only
  if the parent still owns viewport refs, focus mapping, support-summary open
  sequencing, and proof download side effects. Keep support summary available
  outside the `firstDevice`-only branch so empty or blocked states do not lose a
  help path.
- Vue's official performance guidance still matches this pattern: split chunks
  are most useful when the feature is not immediately needed at first paint,
  and an async component loader only runs when that wrapper is actually
  rendered. For lower sections, keep both `defineAsyncComponent` and viewport /
  `v-if` gates so the child tree is not fetched on the first pass by accident.
- For OTA task creation, keep filter parsing, preview requests, preflight, device
  selection, and save calls in the parent/composable. Safe child seams are the
  onboarding next-step card, modal's launch-context alerts, saved-filter picker
  UI, and read-only rollout subset summary. This preserves the distinction
  between a previewed scope and an actually submitted OTA task.
- For Topic Mapping, keep CRUD, ownership validation, duplicate detection, and
  cache invalidation in the mapping service. The public dry-run evaluator can
  live in a read-only sibling file, but must preserve `test_topic` precedence
  over `sample_topic`, capture resolution, diagnostic ordering, and exact
  `next_steps` copy.
- For pure first-device presentation mappers, move the function behind a
  re-export before changing caller imports. Preserve state precedence and data
  fallbacks exactly; split files improve navigation but do not by themselves
  prove browser-test, telemetry, or chart runtime behavior.
- For interactive visualization grids, keep the parent responsible for action
  factories and side effects. A child may render flow nodes or evidence cards
  and emit the selected key/card, but action-button propagation, card keys,
  test IDs, diagnostic routing, and support-bundle behavior must remain
  contract-preserving.
- For Device Model access checks, keep concrete generated-query branches and
  their query-before-permission error ordering. A private tenant-ID loader can
  reduce duplication, but should not replace those branches with reflection or
  alter the `CodeParamError`, `CodeDBError`, and `CodeNoPermission` contracts.
- For above-the-fold Command Center progress, keep the component synchronous.
  It may render precomputed steps and emit a preview intent, while the parent
  remains responsible for preview eligibility and request side effects.
- For Device Config updates, a pure preparation helper may compose JSON-map
  construction and `other_config` validation only after the existing empty
  template unlink. Keep authorization/load first, cache refresh after the DAL
  update, and plugin/voucher side effects last; do not change the current
  pointer-based `other_config` change predicate in a structural refactor.
- Known Device Config runtime risk: the existing empty-template unlink is a
  write before later validation, and voucher persistence follows cache reread.
  Do not silently change either ordering in a file-split pass; require focused
  API/runtime evidence before turning that into a transactional behavior fix.
- For tenant-filtered trees, keep DAL access and error mapping in the service
  method. A pure tree assembler may preserve input order and root rules, but
  must not silently change the existing missing-parent omission behavior.

## Frontend Slice Rules

- Prefer extracting view-local structure, deferred-mount gates, pure helpers,
  and export/download builders before touching route, auth, or identity
  orchestration.
- When the same deferred-mount behavior is needed across pages, move it to a
  shared hook and leave a thin compatibility re-export only if existing tests
  or page-local imports still depend on the old path.
- Keep route intent, device identity, auth context, scroll/focus targets, and
  cross-panel side effects in the parent page unless the entire interaction
  contract is moving with the child.
- When the goal is customer-visible performance, favor async components,
  viewport-deferred lower sections, and duplicate-request guards over "split
  for split's sake".
- Vue's official performance guidance treats smaller initial JavaScript bundles
  and on-demand chunks as page-load optimizations, and `defineAsyncComponent`
  only loads its inner component once that wrapper is rendered. Keep using
  `v-if` or viewport gates for secondary panels; an async component that is
  always rendered still fetches during the first page pass.
- If a secondary panel also owns its initial API request, gate that request on
  the same viewport/manual-load condition unless route state explicitly needs
  the panel opened immediately. This keeps command drafting, previews, and
  primary CTAs responsive before history/evidence tables are needed.
- Static proof/summary sections at the bottom of large pages can use the same
  split-plus-viewport pattern even when they do not own API requests. Keep the
  summary data and route/submit decisions in the parent, move the presentational
  section into a child component, and only mount it when it nears the viewport
  or when the operator explicitly requests it.
- Inside already-async workbenches, prefer splitting the preview-only analysis
  surface away from the draft-entry surface. Keep command inputs, route intent,
  primary preview/submit actions, and submit-gate readiness in the parent; move
  impact summaries, governance cards, preview result tables, and other
  post-preview read-only UI into a child component that can async-load only
  when preview evidence exists.
- For these slices, lightweight proof is usually enough: touched SFC
  parse/compile-script, helper static checks, and only the most focused tests
  that validate behavior changed by the seam.
- Do not claim browser timing wins, runtime closure, or release API/E2E
  completeness without matching runtime evidence.
## 覆盖可信度边界

- 覆盖合同必须同时跑 `00_coverage_contract` 和 `00_oracle_contract`；只跑 endpoint/preflight 三件套会漏掉业务 oracle、弱断言和显式能力清单问题。
- `toBeTruthy()`、`exists().toBe(true)`、Chai `to.exist` 不应出现在业务闭环证据里；要改成具体字段、数量、状态、placeholder、文本或响应体断言。
- 当前 harness 合同通过只能说明测试分类和静态 oracle 更可信，不能替代真实 API 服务、数据库、MQTT broker 和 Playwright E2E 运行证据。

本指南只记录当前源码维护入口，不替代部署文档。首次部署和首台设备闭环先看根目录 `START-HERE.md`。

## 代码边界

- `frontend/`：Vue 3 + TypeScript + Vite 控制台，主要页面在 `frontend/src/views`。
- `backend/`：Go API、业务服务、DAL、初始化和部署配置。
- `mqtt-broker/`：基于 GMQTT 的 MQTT 接入网关、插件和协议运行时。
- `automation_tests/`：API 自动化、Playwright E2E、覆盖契约和报告脚本。
- `deploy/`：一键初始化、预检、启动、打包和首台设备 closeout 辅助脚本。
- `references/`：当前清理、覆盖、发布边界和后续计划。

## 本地开发顺序

1. 先确定要改的是前端、后端、Broker、部署脚本还是自动化。
2. 读取对应目录的 `README.md`，再看目标文件附近测试或状态文档。
3. 小范围修改，保留 API、DB、路由、MQTT topic 和持久化配置的兼容字段。
4. 优先做轻量静态检查；运行全量 build/test 前先确认依赖和机器状态。

## 当前轻量检查

- 残余词扫描优先用 `rg`。
- Go 语法或编译问题优先跑目标包，不要一开始跑全仓库。
- 当前 `frontend/node_modules` 与 `automation_tests/node_modules` 已可安装；若重新清理依赖，分别用 `pnpm install` 和 `npm ci` 恢复。
- API/E2E 还需要真实账号、`PREVIEW_URL`、`API_TARGET`、预览代理和 Playwright 运行环境；缺这些时只能报告预检阻塞，不能写成业务闭环已通过。

## 首台设备主线

主线入口是 `/first-device`。维护时必须保护这些能力：

- 部署入口能指向 `AETHERLINK_PUBLIC_URL/first-device`。
- 页面能给出 MQTT / HTTP 参数和可复制测试命令。
- 浏览器能发测试遥测，设备也能按同一参数上报。
- 页面能显示在线状态、最新数据、首张图表和成功证明。
- closeout 脚本能把启动 manifest 与成功证明合成最终交付证据。

## 兼容字段

这些名字看起来旧，但当前仍是合同字段，不能为了清爽直接改名：

- `template_secret`
- `templateId`
- `device_template_id`
- `preview_sample_device_ids`
- `sample_limit`
- `sample_devices`
- `sample_topic`
- `sample_payload`

新代码应优先使用已经补上的新语义字段：

- `subset_limit`：批量筛选预览子集上限。
- `preview_devices`：批量预览代表设备列表。
- `test_topic`：Topic Mapping dry-run 测试主题。
- `test_payload`：首台设备测试遥测载荷。

旧 `sample_*` 字段只作为兼容输入/输出保留。只有有迁移计划、API 双写/双读策略和验证证据时，才能删除这些旧字段。
## 2026-07-08 Latest Verification Note

- Focused frontend repair this round closed two stale suites:
  `scene-edit` (`7/7`) and `thingsvis-dashboards` (`9/9`).
- Coverage harness trust pack was re-run and still reports `51 passing`.
- Do not over-read that as full runtime closure:
  release API/E2E still needs real release credentials, `PREVIEW_URL`,
  `API_TARGET`, and the 9725 preview proxy settings.
- Current largest remaining frontend test debt is
  `telemetry.test.ts` (`19 passed / 24 failed`), where many assertions still
  target pre-composable `setupState` internals instead of the live module
  boundaries.
