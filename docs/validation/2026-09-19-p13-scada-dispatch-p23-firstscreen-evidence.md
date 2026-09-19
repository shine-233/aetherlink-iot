# P1.3 SCADA 控制真实下发联调 + P2.3 浏览器首屏测量证据

> 日期：2026-09-19
> 对应路线图：§1.2 P1.3（Widget 与 SCADA 基础层）、P2.3（数据保留与性能）
> 结论：**P1.3 的「真实下发联调」闭环**——过程中抓出并修复一个让"确认后的控制
> 执行在运行期 100% 失败"的 schema 缺陷；**P2.3 的「浏览器首屏未测」闭合**
> （本地基线数字，warm cache，如实标注边界）。

## 一、P1.3：SCADA 控制真实下发联调（契约测试 63 号，3/3 全绿）

### 抓出的 schema 缺陷（115.sql 修复）

`scada_control_audits.confirmation_token` 在 88.sql 建成 **varchar(64)**，而
`ConfirmationIssuer` 签发的令牌格式是 `<过期秒>.<64位HMAC-SHA256 hex>`（约 75 字符）。
审计先于执行落库是 P1.3 的刻意设计（写不进去就拒绝执行），于是：

- **每一条带确认令牌的控制命令都在审计写入处 22001 失败** → 确认后的控制执行
  在运行期 100% 不可用；
- 讽刺的是"无令牌"的拒绝路径反而正常（denied 审计的空令牌列装得下）；
- API 单测从未同时覆盖"带令牌执行 + 审计落库"，缺陷一直潜伏。

修复：`115.sql` 加宽为 varchar(255)（VERSION_NUMBER=115）。

### 活栈链路（63_scada_control_dispatch.test.js，3/3，3s）

```text
  ✔ refuses execution without a confirmation token and records the failure
  ✔ issues a confirmation token, executes the command, and the device receives it (1027ms)
  ✔ rejects a command whose widget command is not registered
```

全链路：种子设备 + 命令模拟器（真实 MQTT 会话，订阅 devices/command/<device_number>）
→ 建 SCADA 项目/文档（valve 控件画布）→ 发布 → 无令牌执行被拒
（201002 + 审计 outcome=denied + 拒绝原因留痕）→ 签发 HMAC 确认令牌 →
带令牌执行成功 → 命令经标准下发通道到达设备（回执 method=open_valve、
params.target=open、ack_payload 回应答信封）→ 审计 outcome=success 落库。

审计语义如实记录：确认闸门拒绝的 outcome 是 **denied**（不是 failed），
两类失败在审计里分账。

### 浏览器 E2E（同日补齐，P1.3 正式翻 done）

`automation_tests/e2e/34_scada_editor.spec.js` **1 passed（3.1s，真实 Edge）**：

1. 编辑器页加载（save 初始禁用——「无变化往返不产生空版本」契约在 UI 侧成立）；
2. **通过 UI 走 new project / new canvas 创建**（顺带取证项目 CRUD 的 UI 面；
   不走下拉——项目列表分页不一定包含新建项，第一版用下拉选择即因此超时）；
3. 展开符号面板（NCollapse 默认折叠）→ `+ timeseries` 添加节点 → 画布变脏、save 激活；
4. save 走 expected_version 乐观并发 → 「已保存」→ API 直读画布确认 timeseries 节点落库；
5. publish → 「已发布」→ versions 端点确认发布快照 ≥1。

### P1.3 门禁对账（终版）

项目 CRUD 不再 unsupported✓（含 UI 面）；画布保存/加载可往返✓；遥测断线判陈旧✓；
控制命令权限/确认/审计✓ + 真实下发✓；3D/WebGL 逐 Widget 降级✓；
**四面一致齐备**（API/OpenAPI ✓、后端权限 ✓、UI 行为 ✓、自动化 E2E ✓）→
**P1.3 翻 `done`**。

### E2E 踩坑记录（诚实入档）

- chai 语法两次误用到 playwright expect（`.to.be.an('object')` → `toBeTruthy()`、
  `.not.to.equal` → `.not.toBe`）——与 31/32 号同坑；
- naive-ui 下拉选项分页（10/页）导致种子项不可选，改走 UI 创建路径；
- NCollapse 默认折叠，符号面板需先展开。

## 二、P2.3：浏览器首屏测量（prod 构建 + preview 代理，warm cache）

测量方法：Playwright(msedge) 打开 prod 构建（frontend/dist，含本日全部 UI 交付），
经 preview API 代理（9725）；DCL = domcontentloaded，networkIdle = 网络静止或 12s 超时。

| 路由 | DCL | 网络静止 | 资源请求数 | 资源体积 |
| --- | --- | --- | --- | --- |
| /device/manage | 109ms | 4,246ms | 76 | 2,866 KB |
| /home | 94ms | 1,295ms | 85 | 2,844 KB |
| /management/solutions | 61ms | 1,251ms | 43 | 2,413 KB |

**如实边界**：① warm cache（同一 context 二次导航），冷首载更高；② localhost
代理，无真实网络延迟；③ /device/manage 的 4.2s 主要是列表/分组/筛选等并发 API
（P2.3 摄取瓶颈的 UI 侧体现，与摄取管道发现一致）；④ 单次采样，非统计口径。
 tier 达标/双实例/容量模型仍按原状 pending。

## 三、清理

SCADA 项目/文档、种子设备、模拟器进程均通过 API/进程回收；本地配置的确认密钥
（conf-localdev.yml，gitignore）仅隔离库使用。
