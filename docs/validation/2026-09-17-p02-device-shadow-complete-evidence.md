# P0.2 设备影子状态机与 ACK 闭环：端到端与真实浏览器完整闭环证据

> 日期：2026-09-17  
> 仓库：`aetherlink-iot`  
> 对应路线图：§1.1 & §3 P0.2（设备影子状态机与 ACK 闭环）  
> 验证层级：真实活栈端到端（PostgreSQL 17.5 55433 + 本地 MQTT Broker 1883 + 后端 9999 + 前端 Preview 9725 + Microsoft Edge Playwright E2E）  
> 关联自动化测试：  
> - API 契约与 MQTT 上线延时投递闭环：`automation_tests/tests/27_shadow_messages.test.js`  
> - 浏览器 E2E 取证：`automation_tests/e2e/30_p02_device_shadow.spec.js`  
> - 数据层单测：`backend/internal/dal/device_shadow_postgres_test.go`  

---

## 一、验证目标与关键不变式

根据路线图 §1.1 与 §3 P0.2 要求，设备影子系统必须满足以下不可违背的不变式：
1. **下发只到 sent，绝不直接 delivered**：下发成功仅代表命令已通过网络发出，设备尚未响应确认前状态必须保持为 `sent`，严禁把“已发送”冒充为“已送达”；
2. **设备显式 ACK 闭环**：必须由设备上行 ACK 触发迁移至终态 `delivered`，且落库真实时间戳 `ack_at`（与 `sent_at`/`delivered_at` 清晰区分）；
3. **离线自动延时投递**：设备离线时写入 `pending` 状态消息队列；设备上行（遥测/心跳）上线后，后端上线钩子自动检索该设备全部 `pending` 消息并按序投递，推进到 `sent`；
4. **终态行防御**：终态（`delivered`, `failed`, `expired`, `canceled`）消息不可被二次确认，终态消息不可被取消；
5. **四面一致（DB / Backend / API / Frontend E2E）**：数据表字段、ORM 实体模型、后端调度逻辑、前端界面展示以及真实浏览器人机交互完全对齐。

---

## 二、本次修复与核心突破

在最终闭环过程中，通过日志、真实 trace 与网络抓包定位并彻底解决了三处隐蔽根因：

1. **业务上行无法触发上线与延时投递（后端链路）**：
   - **根因**：`backend/internal/uplink/telemetry.go` 中的 `refreshHeartbeat` 此前在执行 `dal.UpdateDeviceStatus(device.ID, 1)` 之前先校验了 `if config == nil { return }`。导致无自定义心跳配置的模型设备在发送业务遥测时无法自动上线，进而阻塞了 `deliverShadowMessagesAfterOnlineDelay`。
   - **修复**：调整时序，让 `if device.IsOnline != 1` 自动上线及 `notifyDeviceOnline` 优先执行，心跳 key 刷新后置。
2. **ORM 模型字段对齐（数据面）**：
   - `backend/internal/model/device_shadow.gen.go` 补齐了 `84.sql` 迁移的 5 个关键字段：`Attempts int`, `SentAt *time.Time`, `AckAt *time.Time`, `NextAttemptAt *time.Time`, `LastError *string`，彻底消除了 ORM 查询丢失字段的隐患。
3. **Vue 3 原生 DOM change 事件冒泡误刷新全页（前端链路）**：
   - **根因**：`frontend/src/views/device/details/index.vue` 模板对子 Tab 统一挂载了 `@change="getDeviceDetail"`。因为子组件根节点为普通 `div`，用户在子组件中点击 `<input type="radio">` 切换状态单选框时，原生 DOM `change` 事件向上冒泡并触发 `@change`，导致整个详情页被强制重载、Tabs 重新初始化，使得设备影子 Tab 的状态选择被强制重置为 `pending`。
   - **修复**：在 `index.vue` 中增加类型守卫 `handleChildComponentChange`，对属于原生 DOM `Event`（包含 `target` 与 `bubbles`）的事件予以直接拦截忽略，仅响应自定义业务 emit。
4. **Vue-i18n 消息语法解析异常（国际化面）**：
   - **根因**：`shadowPayloadHint` 在四种语言（zh-cn, en-us, fr-fr, es-es）字典中包含了未转义的 JSON 花括号 `{"method":"set",...}`，导致 vue-i18n message parser 抛出 `SyntaxError` 并中断表单项渲染。
   - **修复**：规范化占位符文案，避免裸花括号与插值标记冲突。

---

## 三、真实运行期验证证据

### 1. 数据库与业务逻辑自动化测试（`27_shadow_messages.test.js`）

在本地 PostgreSQL 17.5（端口 55433）与活栈 MQTT Broker 上运行：

```bash
node run_tests.js --module 27_shadow_messages
```

**测试结果**：**5/5 passed (12.54s)**
- `rejects invalid shadow message payloads at the binding layer` (PASS - 55ms)
- `queues an offline device command as a pending shadow message with counts` (PASS - 31ms)
- `cancels a pending shadow exactly once` (PASS - 45ms)
- `rejects acknowledging a shadow message that is not ackable` (PASS - 46ms)
- `delivers pending shadows automatically after the device comes online` (PASS - 11820ms)

### 2. 真实浏览器 Playwright E2E 测试（`30_p02_device_shadow.spec.js`）

在真实 Microsoft Edge 浏览器与前端预览服务（9725）中无头运行：

```bash
npx playwright test e2e/30_p02_device_shadow.spec.js
```

**实测输出**：
```
Running 1 test using 1 worker

  ok 1 [msedge] › e2e\30_p02_device_shadow.spec.js:26:3 › P0.2 设备影子状态机与 ACK 闭环浏览器 E2E › 设备详情页设备影子 Tab：新建离线命令、取消、以及上线 ACK 状态同步 (13.5s)

  1 passed (15.4s)
```

**E2E 覆盖路径**：
1. 打开设备详情页设备影子 Tab，加载 `.shadow-panel`；
2. 点击新建影子命令，弹窗填写 JSON Payload 并成功入队 pending 消息；
3. 表格正确呈现 pending 状态及重试/确认时间占位符；
4. 新建第二条消息并通过二次确认气泡（NPopconfirm）成功取消；
5. 设备通过真实 MQTT 发送遥测上线，后端延时投递并推进为 `sent`，随后通过 API 发送 ACK 推进为 `delivered`；
6. 浏览器单选框切换至「已确认送达」视图，断言表格即时呈现 `delivered` 终态行。

---

## 四、四面一致性验收清单

| 检查项 | 证据文件 / 验证位置 | 验收结论 |
| :--- | :--- | :--- |
| **1. 数据库面** | `backend/sql/84.sql`，表 `device_shadow_messages`（`attempts`, `sent_at`, `ack_at`） | **PASS** |
| **2. 后端服务面** | `backend/internal/dal/device_shadow.go`, `backend/internal/service/device_shadow.go`, `backend/internal/uplink/telemetry.go` | **PASS** |
| **3. API 契约面** | `POST/GET/DELETE /api/v1/device/shadow/:deviceId` 及 `POST /ack`；契约单测 5/5 全部通过 | **PASS** |
| **4. 前端与 E2E 面** | `frontend/src/views/device/details/modules/device-shadow.vue` 与 `30_p02_device_shadow.spec.js` 15.4s 跑通 | **PASS** |

---

## 五、结论与路线图更新

P0.2 设备影子状态机与 ACK 闭环的所有门禁要求（包括此前缺失的真实 MQTT 上报、上线延时投递端到端与真实浏览器 E2E 证据）均已完整就绪并通过验证。

- **路线图状态**：P0.2 由 `partial` 正式更新为 **`done`**。
