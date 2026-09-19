# P0.3 / P0.4 真实运行期 E2E 证据（OTA 与场景联动，live stack）

> 日期：2026-09-19
> 对应路线图：§1.2 P0.3（OTA 状态机）、P0.4（场景与 Flow 语义）
> 结论：**两条"真实设备/broker 或协议 stub E2E"门禁均已闭环**——此前多年未闭环的
> 原因不是"缺跑一遍"，而是**两个真实缺陷把运行期链路堵死**。修复后各自的既有
> strict 运行时用例第一次真正跑通。

## 一、修复的两个真实缺陷（本次 E2E 的前提）

### 缺陷 1：OTA 下发通道在主程序从未接线（P0.3 根因）

`mqtt/publish.CreateMqttClient()`（共享发布客户端）**在整个应用装配中零调用**，
`mqttClient` 恒为 nil → `PublishOtaAddress` 永远返回
`ErrPublisherUnavailable: shared client is not connected` → OTA 任务的
broker 下发（`ota/devices/inform/<device_number>`）**在运行期整体不可用**。
API 面测试（47 组灰度治理等）不经过 broker 发布，因此全绿掩盖了这条断链。
修复：`internal/app/mqtt_service.go` 在 Adapter 初始化处调用 `publish.CreateMqttClient()`
（进程内一次；启动日志出现 `mqtt connect success`，此前为 0 条）。

### 缺陷 2：动作 20「激活场景」的校验对象与运行期不一致（P0.4 根因）

`validateSceneAutomationActionSceneReference` 校验的是 **scene_automations** 表，
而运行期（`AutomateTelemetryActionScene`）执行的是 **scenes** 表
（`GetSceneInfo` + `ActiveSceneExecute`）。指向真实场景的合法动作在创建期
即被 record not found 拒绝（后端日志 `dal/scene_automations.go:51`）——
"自动化里激活场景"这条产品链路从开源初始版起就创建不出来。
修复：校验改走 `ensureSceneReadAccess`（scenes 表 + 租户边界不变），
并加 SQLite 内存库回归测试（真实场景通过 / 跨租户拒绝 / 缺失拒绝）。

### 测试基建：模拟器信封开关

设备模拟器（cmd/aetherlink-device-autotest）历史上行是裸载荷，依赖真实 gmqtt
broker 的 aetherlink 插件补 `{device_id, values:<base64>}` 信封；本地 stub broker
不补，后端以 `Invalid status payload` 丢弃。新增 `AUTOTEST_WRAP_UPLINK_ENVELOPE`
开关让模拟器按**同一线上契约**自行包装（status / ota_progress），与
seed_data 既有 `uplinkEnvelope` 选项同思路。另：长时运行的 stub broker 在承载过
200 连接压测后需要重启再跑（fan-out 状态劣化）。

## 二、实测结果（活栈：PG 17.5@55433 + Redis + stub broker + 后端 9999）

**P0.3 — `automation_tests/tests/32_ota_runtime.test.js` 2/2 全绿（5s）**：

```text
  ✔ creates a task through the public API and persists a successful
    device-reported OTA rollout (2752ms)
  ✔ persists a device-reported OTA failure and exposes it through
    the support bundle (2433ms)
```

覆盖：公网 API 建任务 → 认证设备经真实 MQTT 收到 inform → 上报 0/10/50/100 进度
（信封契约）→ 终态明细行落库可读；第二例设备上报失败 + 支撑包读回。
P0.3 门禁逐条对账：状态转移非法拒绝✓（状态机单测）、同事件幂等✓、
失败设备筛选重试✓、回滚留审计✓、真实 stub E2E✓、灰度治理 47 组 9/9✓（09-15）。

**P0.4 — `automation_tests/tests/31_scene_action_20_runtime.test.js` 1/1 全绿（15.2s，
strict 模式）**：

真实 MQTT 在线迁移触发自动化 → 动作 20 激活嵌套场景 → 触发告警配置 →
automation / scene / alarm 三路执行日志经 strict psql 直查独立核验。
strict 环境变量：`AETHERLINK_STRICT_DB_TARGET=1`、`AETHERLINK_STRICT_DB_CLEANUP=1`、
`AETHERLINK_PSQL_PATH`、`AETHERLINK_DB_NAME`/`GOTP_DB_PSQL_*`（隔离集群 55433）。
P0.4 门禁逐条对账：边界时间表驱动测试✓（单测 7 例 11 行边界）、重复触发幂等✓
（FlowTriggerKey 单测）、停止动作可审计✓（单测）、真实 E2E✓。
"服务重启后调度不丢任务"由 DB 持久化执行窗口（91.sql）+ 启动时 cron 重载保证，
属结构性保证而非专门的重启演练——如实注明。

## 三、回归

- `go build ./...` exit 0；`internal/service`（含新增回归测试）全包 ok；
- 模拟器模块 `cmd/aetherlink-device-autotest` 构建通过；
- 同活栈上 60/61/62/55/40 等契约套件此前一轮全绿，本轮改动不触及它们的代码路径。

## 四、仍未闭环（如实）

- P0.3 剩余：真实 OTA 固件下载/刷写端到端（需真实设备或完整固件仿真）；
- P0.4 剩余：专门的后端重启中途恢复演练（当前为结构性保证）。
