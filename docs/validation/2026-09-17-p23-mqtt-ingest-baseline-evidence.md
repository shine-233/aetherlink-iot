# P2.3 MQTT 摄取基线（第一批真实数字 + 一个必须说清的可信度分层）

> 日期：2026-09-17
> 仓库：`aetherlink-iot`
> 对应路线图：§3 P2.3「数据保留与性能」
> 新增工具：`backend/cmd/mqttbench`（Go，复用已 vendored 的 `paho.mqtt.golang`）
> 辅助脚本：`automation_tests/scripts/fetch-mqtt-connect-info.js`、`verify-telemetry-landed.js`
> 原始报告（本地工件，`performance/reports/` 被 .gitignore 排除）：
> `local-mqtt-ingest-20260917.json`、`local-mqtt-ingest-unthrottled-20260917.json`

---

## 一、为什么先做这条路径

`tiers.json` 里写着 `mqttClients: 50 / 200 / 500`，但**从来没有被施加过**。
上一批 API 基线只覆盖了 `/health` 与部署健康检查——那两条都不碰遥测写入。
路线图 §P2.3 明确要求"压测不使用假数据掩盖数据库瓶颈"，而**MQTT 摄取正是那条瓶颈路径**。

---

## 二、过程中踩到的两个坑（这是本次最有价值的部分）

### 坑 1：broker 的 PUBACK 不代表平台摄取

第一轮我用扁平载荷 `{"temperature":25.5,...}` 发，结果：**500 条全部"成功"、0 失败**。
但读回后端遥测表，数据还停在 2 天前、键名也对不上——**消息全被 adapter 丢弃了**。

原因：`adapter.verifyPayload` 要求的是**信封**载荷

```json
{"device_id":"<设备ID>","values":"<base64(扁平JSON)>"}
```

`base64` 不是随手定的：`publicPayload.Values` 是 `[]byte`，Go 的 `json` 包把 `[]byte`
编成 base64 字符串。扁平载荷只有在 gmqtt 的 `aetherlink` 插件于 broker 侧补信封时才成立。

**教训**：broker 对任何合法 MQTT 载荷都回 PUBACK。所以"发布成功、0 失败、吞吐很高"
完全可以在消息 100% 被丢弃的情况下出现——**不读回落库，吞吐数字就是假的**。
工具因此加了 `-envelope -device-id` 参数，并把这条写进输出的 `caveats`。

### 坑 2：延迟样本不可信（p50 恒为 0）

QoS 1 的本意是测 PUBACK 往返，但实测 **p50 恒为 0 ns**。localhost 的 TCP 往返最快也要
几十微秒，**0 ns 在物理上不可能**——说明 paho 的 QoS1 token 对相当一部分发布在 `Wait()`
之前就已完成，测到的是"本地入队"而不是"网络往返"。

因此本报告的 `ackLatencyMs` **不作为延迟结论引用**，只报吞吐与错误率。

---

## 三、实测结果（按可信度分层）

环境：AMD Ryzen 5 5600X ×12，16,304 MB，win32/x64。单实例，本地回环，broker 与压测进程
争用同一 CPU。载荷为 4 个遥测键的信封形式。

| 组 | 连接数 | 限速 | 发布数 | 失败 | 吞吐 | **落库验证** |
| --- | --- | --- | --- | --- | --- | --- |
| A | 4 | 200/s 每连接 | 11,989 | 0 | **799 msg/s** | ✅ **已确认** |
| B | 2 | 不限速 | 126,851 | 0 | **15,856 msg/s** | ❌ **未确认** |

### 3.1 A 组：可用

限速跑 15 秒。跑完立即读回 `GET /telemetry/datas/current/<device_id>`，
时间戳为 `2026-09-17T08:45:47`（当次），键名与载荷一致
（`temperature_1` / `temperature_2` / `switch_1` / `switch_2`）。**端到端确认落库。**

注意：799 msg/s 只证明"限速器按目标速率工作"，**不是容量上限**。

### 3.2 B 组：**不可用**

不限速跑 8 秒得 15,856 msg/s。但**跑完立即读回时接口返回 `code=-1`**，
此后整套栈停止（PG 55433 与后端 9999 均拒连），**无法再补验**。

因此 B 组只能表述为"**发布侧 15,856 msg/s，落库未确认**"。
它**不能**作为摄取容量结论——它恰好是那种"看起来漂亮、可能是假的"数字。

顺带记录一个未查清的现象：读回 `code=-1` 出现在 126k 条消息之后、而后端健康检查
（`/health` 与 `/api/v1/deployment/health`）当时仍返回 ok。是读路径在负载后出错、
还是别的偶发，**本次没有查清，不下结论**。

---

## 四、不能宣称什么（如实记录）

1. **不能宣称任何 tier 达标**。`tiers.json` 的 `mqttClients: 50/200/500` 描述资源限制，
   本次只用了 2~4 个连接，且未施加任何 CPU/内存配额。输出里显式
   `evidenceKind: "local-baseline"` / `tierClaim: null`。
2. **不能宣称摄取容量上限**。唯一确认落库的一组是**限速**的（799 msg/s）；
   不限速那组未确认落库。
3. **延迟无数据**（见坑 2）。
4. **只有单实例、单设备、单话题**，且设备是既有种子设备，不是新建的干净租户。
5. **本机为开发机**，且有其它常驻负载。

因此 P2.3 的表述是：**"已有 API 与 MQTT 两条路径的本地基线数字（其中 MQTT 仅限速组确认落库），
tier 达标证据仍 pending"**，不宣称任何门禁通过。

---

## 五、复现命令

```bash
# 0) 前置：活栈在跑（后端 9999、broker 1883、PG 55433）

# 1) 取一份真实设备连接信息（需要 tenant_admin）
cd automation_tests && node scripts/fetch-mqtt-connect-info.js
#   → 输出 device_id、broker 地址、遥测话题

# 2) 跑限速基线（推荐先跑这一档）
cd ../backend && go run ./cmd/mqttbench \
  -broker tcp://127.0.0.1:1883 -topic devices/telemetry \
  -device-id <上一步的 device_id> -envelope \
  -clients 4 -rate 200 -duration 15 -warmup 3 \
  -out ../performance/reports/local-mqtt-ingest-<date>.json

# 3) **必须**读回确认落库（否则吞吐数字无效）
cd ../automation_tests && node scripts/verify-telemetry-landed.js <device_id>

# 4) 不限速找上限（谨慎：会短时间内写入十几万行）
cd ../backend && go run ./cmd/mqttbench ... -clients 2 -rate 0 -duration 8
#   跑完**务必**再做一次第 3 步；读回失败时该组数字作废
```

---

## 六、下一步

1. **查清不限速组的 `code=-1`**——是读路径在负载后出错，还是别的偶发。
   这是把"15,856 msg/s"从"疑似"变成"可用"的唯一途径。
2. **多设备并发**才是 `tiers.json` 想要的形态（50/200 个连接需要 50/200 个设备凭据），
   当前只用 2~4 个连接复用了同一凭据。
3. **延迟测量要换法子**：绕过 paho 的 token 语义，用带序号的应用层往返，
   或在 broker 侧打点。
4. tier 达标证据需要**带资源配额的容器环境**，本机做不到，保持 pending。
