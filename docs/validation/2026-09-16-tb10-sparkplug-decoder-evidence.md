# TB-10 Sparkplug B：解码内核证据

> 日期：2026-09-16
> 仓库：`aetherlink-iot`
> 对应路线图：§7.1 TB-10（此前**从未立项**，本轮由竞品全量核对补入）
> 基准：Eclipse Tahu `sparkplug_b/sparkplug_b.proto`（字段号逐条核对）

---

## 一、缺口回顾

Sparkplug B 是 MQTT 上的工业互操作规范（Eclipse Foundation 托管，Tahu 为参考实现），
ThingsBoard 的 MQTT 传输层长期支持它。本地核对时**全仓 0 命中**，属 `未实现`。

---

## 二、交付物：`pkg/sparkplug`

纯标准库叶子包，**不依赖 protoc 生成代码**。理由：Sparkplug 的 `Payload` 是一个字段数很少、
长期稳定的消息，手写 wire-format 解码比引入 protobuf 代码生成链更可控，也避免把 `protoc`
带进构建链（本机 Go module proxy 不可达，加依赖本身就有风险）。

### 2.1 话题解析

`spBv1.0/<group_id>/<message_type>/<edge_node_id>[/<device_id>]`

严格校验：段数必须为 4 或 5、命名空间精确等于 `spBv1.0`、`message_type` 在 9 种规范类型白名单内、
各段不得为空。**不做大小写归一、不做别名猜测**——工业现场的话题是机器生成的，
容错只会把"接错线"变成"看起来接通了"。

### 2.2 载荷解码

`Payload { timestamp=1, metrics=2, seq=3, uuid=4, body=5 }`，
`Metric { name=1, alias=2, timestamp=3, datatype=4, is_historical=5, is_transient=6, is_null=7,
metadata=8, properties=9, value oneof 10~19 }`。

字段号已与官方 proto **逐条核对**（含 `DataType` 枚举 1~19）。

### 2.3 核心不变量：fail closed，且**不把非数值当 0**

| 情形 | 行为 |
| --- | --- |
| 话题命名空间/段数/类型不符 | 报错，不猜 |
| 未知 wire type / 截断 / 长度越界 / 非法 UTF-8 | 报 `ErrMalformedPayload` |
| `is_null` 为真 | **不产出数值**，也不产出 0 |
| 非数值 datatype（布尔/字符串/字节/DataSet/Template/数组/未知） | 跳过，**不转成 0** |
| 数值型 datatype 但缺值 | 跳过 |
| 指标名为空（纯 alias 上报） | 跳过（alias→name 需会话上下文，本包不持有，故不猜） |

**为什么"不转成 0"是关键**：把字符串或空值读成 `0`，在仪表盘上是一个完全合法、
完全看不出来的错误读数。宁可少一个点，也不能给一个假数。

### 2.4 前向兼容

未知字段按 wire type 跳过，规范未来新增字段不会让整包解码失败。

---

## 三、验证证据

```
cd backend && GOTOOLCHAIN=local go vet ./pkg/sparkplug/     → 0 问题
cd backend && GOTOOLCHAIN=local go test -p 1 ./pkg/sparkplug/ -count=1
ok  aetherlink-iot/backend/pkg/sparkplug  0.451s
```

**41 个用例**，分五组：

| 组 | 内容 |
| --- | --- |
| 话题解析 | 13 子用例（节点级/设备级/STATE、空白裁剪、命名空间错、段数多与少、空段、空话题、未知类型、小写不归一） |
| **规范锚点** | 手工按字段号推导的 11 字节向量 `08 01 12 07 0A 01 61 20 03 50 07`，同时约束编码与解码两侧 |
| 往返 | 覆盖全部标量 oneof（double/float/int32/bool/string/bytes）与公共字段（alias/timestamp/is_historical/is_transient/is_null） |
| 数据类型判定 | 9 种数值型 + 2 种浮点型返回数值；9 组非数值/缺值/`is_null` 必须返回 `ok=false` |
| 负向对照 | 11 组畸形载荷 + 前向兼容 + 抗 panic |

### 为什么要有"规范锚点"这一条

编码器与解码器**共享同一个错误字段号**时，往返测试会全绿而缺陷完全不可见。
锚点用**手工推导的字节**断言，是这份测试里唯一能打破"自洽但错误"的防线。
字段号另与官方 proto 逐条核对作为第二重依据。

---

## 四、MQTT 上行接线（同日补齐）

解码内核若没有消费方就是死代码，故同日完成接线。

### 4.1 新增 `initialize.GetDeviceByNumber`

接线前发现一处**断链**：Sparkplug 话题携带的是设备编号（`edge_node_id` / `device_id`），
而现有缓存只有 `initialize.GetDeviceCacheById(deviceID)`（按主键），
**没有按 device_number 的查询**，直接接会在"编号 → 内部 ID"这一步断掉。

新增 `GetDeviceByNumber`，设计取舍：

| 决策 | 理由 |
| --- | --- |
| 复用既有 `dal.GetDeviceByDeviceNumber` | 它已存在，语义为**精确匹配**，且被 `TestCheckDeviceNumberExistsUsesGlobalExactMatch` 锁死 |
| **不接受租户参数** | `devices_unique UNIQUE (device_number)` 表明编号**全局唯一**；租户由解析出的设备自身决定。这也顺带解决了"Sparkplug 话题不含租户"的问题 |
| **刻意不做大小写归一** | 把"编号写错大小写"变成"静默接到另一台设备"，是身份解析路径上最坏的失败方式 |
| **刻意不加 Redis 缓存** | 身份解析路径优先正确性：宁可每次回源，也不引入"过期编号仍指向已改号设备"的窗口。如需缓存，必须与 `DelDeviceCache` 同步失效 |

### 4.2 订阅与处理器

- 话题常量 `TopicPatternSparkplug = "spBv1.0/+/+/+/#"`。
  **必须是 `#` 而不是 `+`**：MQTT 的 `#` 可匹配零层，因此同时覆盖 4 段节点级与 5 段设备级；
  写成 `spBv1.0/+/+/+/+` 会**漏订全部节点级消息**。用例锁死这一点。
- `SubscribeDeviceTopics` 注册（qos 1）+ `handleSparkplugMessage` 回调 + `HandleSparkplugMessage` 处理器。
- 处理器语义：NDATA/DDATA → 解码 → 按编号解析设备 → 数值遥测 JSON → 投递 `UplinkMessage`
  （`source_protocol = mqtt-sparkplug-b`，元数据带 group/edge/type）。
  会话类消息（NBIRTH/DBIRTH/NDEATH/DDEATH/NCMD/DCMD/STATE）**忽略且不报错**——
  设备在 birth 之后持续发 data 是常态，把 birth 当失败会刷满错误日志。

### 4.3 接线契约测试

```
go test -p 1 ./internal/adapter/mqttadapter/ -run 'Sparkplug' -count=1
```
```
--- PASS: TestHandleSparkplugMessageRejectsInvalidTopic            (8 组非法话题)
--- PASS: TestHandleSparkplugMessageRejectsMalformedPayloadBeforeBus
--- PASS: TestHandleSparkplugMessageIgnoresSessionMessagesWithoutError  (7 种会话消息)
--- PASS: TestHandleSparkplugMessageFailsClosedWhenDeviceLookupFails
--- PASS: TestSparkplugTopicPatternCoversBothNodeAndDeviceLevel
ok  aetherlink-iot/backend/internal/adapter/mqttadapter  0.236s
```

**这组用例刻意只测拒绝路径**——它们都不碰 bus 与数据库，因此能真实执行。
"成功投递"需要真实 broker + Redis + PostgreSQL，把不可执行的路径写成 skip
等于用绿灯掩盖"从未跑过"，所以不写。

### 4.4 回归

```
go build -p 1 ./...                                                  BUILD=0
go test -p 1 ./internal/adapter/... ./pkg/sparkplug/ ./pkg/units/ ./internal/service/ -count=1
ok  internal/adapter  0.620s / casbinadapter  1.158s / casbinwatcher  0.745s
ok  internal/adapter/mqttadapter  0.238s / pkg/sparkplug  0.153s / pkg/units  0.794s / internal/service  3.240s
```

---

## 五、仍未闭环（如实记录）

1. **未与真实 Sparkplug 设备联调**——无运行期证据。验证依据是规范字段号 + 字节向量 +
   拒绝路径用例，不是现场数据。因此路线图 TB-10 记 `未验证`（代码与接线齐备，缺运行期证据），
   **不宣称完成**。
2. **NBIRTH/DBIRTH 的别名表与会话状态机未做**：Sparkplug 允许 DBIRTH 建立
   `alias → name` 映射、后续 DDATA 只报 alias。当前实现遇到无名指标会跳过（不猜），
   因此**纯 alias 上报的设备会表现为"无遥测"**，而不是报错。
3. `DataSet` / `Template` / `PropertySet` / `MetaData` 按设计**只跳过不展开**——
   它们不承载遥测数值，展开会显著放大攻击面。
