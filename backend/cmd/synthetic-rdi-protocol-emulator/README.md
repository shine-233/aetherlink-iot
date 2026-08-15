# Synthetic RDI protocol emulator

这个工具用于部署前的软件合同测试和协议回放，不是 RDI 硬件模拟器，也不是
ThingsVis Server。它明确输出并校验以下证据边界：

- `fixture_provenance: synthetic-rdi`：身份、PID、voucher 和硬件序列号都是隔离测试数据。
- `evidence_class: protocol-emulator`：帧是根据仓库已经观察到的通用 AetherLink direct-device MQTT 合同生成的。
- `device_execution: not-proven`：没有物理设备执行固件。
- `real_rdi_status: not-tested`：没有证明真实 PID 激活、真实 voucher、真实硬件 MQTT、真实遥测或真实固件 ACK。

因此，生成的 `SYN...` PID 只是满足软件校验形状的 synthetic PID，不能写入报告为“真实 RDI PID”。同理，ACK 只能说明
后端/协议合同能够处理一个 synthetic response，不能说明真实固件已经返回 ACK。

## 默认安全行为

默认命令全部离线运行，不连接 MQTT、不写 PostgreSQL、不调用 HTTP API。只有 `device` 模式或带 `-broker` 的 `replay`
模式才具备网络副作用，而且必须显式传入 `-allow-network`。

voucher password 只在进程内生成，用于显式配置的 live test connection；`manifest` 和 replay JSON 都不会包含 password。
网络模式只应连接隔离 broker 和明确标记为 `synthetic-rdi` 的测试身份，禁止连接生产 broker 或把真实 PID 交给这个工具。

## 命令

从 `backend` 目录执行：

```powershell
# 只打印公开 synthetic manifest，不泄露 password
go run ./cmd/synthetic-rdi-protocol-emulator -mode manifest

# 生成确定性的 status -> telemetry -> command -> ACK session
go run ./cmd/synthetic-rdi-protocol-emulator -mode session

# 校验仓库内的离线 replay 样例；network 必须为 false
go run ./cmd/synthetic-rdi-protocol-emulator `
  -mode replay `
  -replay-file .\cmd\synthetic-rdi-protocol-emulator\examples\synthetic-session.json

# 网络设备模式：仅用于隔离 MQTT broker，必须显式允许网络
go run ./cmd/synthetic-rdi-protocol-emulator `
  -mode device `
  -broker 127.0.0.1:1883 `
  -allow-network `
  -ack-mode fail-once
```

`device` 模式启动后会发布 online status 和一条遥测，订阅
`devices/command/{PID}/+`，再向 `devices/command/response/{message_id}` 发布 ACK；退出时发布 offline status。
`success`、`failure` 和 `fail-once` 由同一个 ACK responder 负责，`fail-once` 按 `message_id` 计数，避免两个 responder
同时回答同一命令而造成假阳性。

当前工具通过 `-seed` 生成独立身份，例如：

```powershell
go run ./cmd/synthetic-rdi-protocol-emulator -mode manifest -seed deployment-prep
```

它不会自动匹配 `seed_synthetic_rdi_fixture.js` 产生的数据库行，也不会执行 activation。完整隔离 lane 必须先由 `seed_synthetic_rdi_fixture.js` 创建 `inactive/disabled` 预注册行，再由独立的 API 步骤调用 `POST /api/v1/rdi/devices/activate`；只有明确记录 `activated-this-run`（或诚实记录 `reused-existing`）并回读 `active/enabled` 后，才能启动 emulator。需要数据库行、登录态和后台命令

当需要把 emulator 绑定到隔离数据库中的 fixture 时，可以显式覆盖合成 PID 和设备 ID；voucher 仍通过 `-username` / `-password` 传入，且所有字段仍带有 `synthetic-rdi` provenance：

```powershell
go run ./cmd/synthetic-rdi-protocol-emulator `
  -mode device -allow-network -broker 127.0.0.1:11086 `
  -pid SYNTHRDI0001 -device-id <fixture-device-id> `
  -username <fixture-voucher-username> -password <fixture-voucher-password>
```
在 network `device` 模式下，success 和 failure ACK 两条路径都必须观察 `offline -> online -> offline`，并且 fresh telemetry 与最终 offline 状态要分别回读。闭环时，应使用仓库已有的 synthetic fixture 脚本以及
`backend/cmd/aetherlink-device-autotest` 的通用 emulator，并在隔离数据库/隔离 broker 中运行；本工具可用于生成和校验
帧合同，但不能代替那条运行编排。

## 当前覆盖的帧合同

这些 topic/payload 来自仓库已有的 generic direct-device 实现，不应命名为 RDI 私有固件协议：

| kind | topic | payload |
| --- | --- | --- |
| status | `devices/status/{device_id}` | `1` 或 `0` |
| telemetry | `devices/telemetry` | JSON object |
| command | `devices/command/{pid}/{message_id}` | `{"method":"...","params":...}` |
| ack | `devices/command/response/{message_id}` | `{"result":0或1,"message":"success"或"failed","ts":...}`，可带 `method` |

replay 校验会拒绝：不连续的 sequence、session 与 frame 身份不一致、错误的 topic/PID/device ID、非法 status payload、非
JSON telemetry/command/ACK、未知 frame kind，以及 `real-rdi` provenance。这样可以防止把手工改写的 JSON 当成真实设备回放。

## 与部署前 sign-off 的关系

通过本目录的 Go tests 和离线 replay，只能提升以下软件证据：协议 frame 构造、ACK 成功/失败/重试分支、replay 顺序和
synthetic provenance gate。它不能关闭：

- ThingsVis Server/Studio、SSO、iframe、dashboard mirror 或 `negative-menu` 的真实外部服务证据；
- `THINGSVIS_MIRRORED_DASHBOARD_ID` 所需的真实 mirror/ownership 行；
- 真实 RDI PID、activation、voucher/hardware identity、固件 MQTT session、遥测和设备 ACK；
- 生产 Docker/Compose、HTTPS、反向代理、公网 MQTT 和目标环境备份恢复。

部署报告必须把这些条件保持为 `pending`/`external-blocked`，不能因为 synthetic session 通过而改成 `real-RDI passed`。
