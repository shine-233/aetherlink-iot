# TB-10 Sparkplug B：MQTT 上行接入与实时遥测入库闭环证据

> 日期：2026-09-17
> 仓库：`aetherlink-iot`
> 对应路线图：§7.1 TB-10（工业物联网标准互操作协议 Sparkplug B v3.0）
> 验证层级：活栈运行期端到端契约测试（真实 MQTT Broker 1883 + 后端 9999 + PostgreSQL 55433 + Redis 6379）
> 证据归档：`automation_tests/tests/57_sparkplug_mqtt_uplink.test.js`

---

## 一、闭环能力概述

此前 `docs/validation/2026-09-16-tb10-sparkplug-decoder-evidence.md` 完成了 `pkg/sparkplug` 解码内核与拒绝路径契约测试，但在运行期消费方尚未取得活栈遥测入库与端到端查询闭环证据。

本项任务彻底打通从工业 MQTT 上行至遥测存储与租户隔离的全链路：
1. **MQTT 自动订阅与路由**：后端 MQTT Adapter 订阅 `spBv1.0/+/+/+/#`，采用 `#` 完整覆盖节点级（4 段）与设备级（5 段）消息；
2. **设备编号无租户自动解析**：利用 `initialize.GetDeviceByNumber`（基于 `device_number` 全局唯一索引）在话题中解析 `device_id`（设备级）或 `edge_node_id`（节点级退回），安全映射至平台内部设备 ID；
3. **Protobuf 载荷解码与物理不变量**：精确按 Eclipse Tahu wire 规范解包标量度量，遵循"非数值绝不转为虚假 0 值"原则；
4. **总线投递与遥测持久化**：构造 `UplinkMessage`（`source_protocol = mqtt-sparkplug-b`），投递至 Uplink 总线完成入库与当前值缓存更新；
5. **端到端 API 查询与多租户权限校验**：经 `/api/v1/telemetry/datas/current/:id` 验证数值入库精度与跨租户隔离拦截（201001 / 403）。

---

## 二、运行期测试矩阵与断言项

自动化测试套件：`automation_tests/tests/57_sparkplug_mqtt_uplink.test.js`

| 序号 | 测试用例 | 话题格式 | 核心验证点 | 结果 |
| --- | --- | --- | --- | --- |
| 1 | 设备级 DDATA 遥测入库 | `spBv1.0/<group>/DDATA/<edge>/<device>` | 自动通过 `device_number` 寻址，验证 temperature (28.5), pressure_kpa (101.32), engine_rpm (1500) 写入与浮点精度 | **PASS** |
| 2 | 节点级 NDATA 遥测入库 | `spBv1.0/<group>/NDATA/<edge>` | 话题无 device_id 时自动回退至 edge_node_id 寻址，验证 bus_voltage (380.0), cpu_usage (42.5) 写入 | **PASS** |
| 3 | 非数值指标安全跳过 | `spBv1.0/<group>/DDATA/<edge>/<device>` | 验证字符串 (device_state)、布尔 (sensor_valid) 指标被安全跳过，绝不转为 0，仅保留数值指标 vibration_level | **PASS** |
| 4 | 会话控制消息优雅忽略 | `.../DBIRTH/...`, `.../NBIRTH/...`, `.../STATE/...` | 承载别名映射与在线状态的控制消息被优雅放行，不影响后续数据流正常接入 (flow_rate: 12.8) | **PASS** |
| 5 | 畸形载荷 Fail-Closed | `spBv1.0/<group>/DDATA/...` | 截断字节包触发 protobuf 解码拦截，不污染库且服务稳定，后续正常遥测 (battery_level: 98.0) 畅通 | **PASS** |
| 6 | 未注册设备号拦截 | `spBv1.0/.../<unknown_ghost>` | 未登记的设备号被安全丢弃并记录，系统内绝不生成幽灵设备 | **PASS** |
| 7 | 跨租户数据隔离 | `/api/v1/telemetry/datas/current/:id` | 租户 B 凭证无法读取租户 A 的 Sparkplug 设备遥测，严格返回无权访问/不存在 (201001 / 403) | **PASS** |

---

## 三、排查与调试记录（关键工程经验）

1. **Protobuf 字段标签核对**：
   - 依据 `backend/pkg/sparkplug/sparkplug.go`，Metric 标量字段定义为：
     - `int_value / long_value` = 10, 11
     - `float_value` = 12 (32-bit fixed)
     - `double_value` = 13 (64-bit fixed)
     - `boolean_value` = 14 (varint)
     - `string_value` = 15 (bytes)
   - 测试夹具中初始使用的字段号（14/15/16/17）存在错位，核对后彻底纠正。
2. **纯 Node.js MQTT 测试客户端优雅断开**：
   - 在 Windows TCP 栈中，若在 `socket.write(pubPacket)` 回调中立即连续写入 `DISCONNECT` (0xe0, 0x00) 并 `socket.end()`，可能引发 Broker 接收缓冲区截断。
   - 优化为在发布包完成写出后增加安全微秒排空延迟（150ms），确保单测执行稳定且不阻塞。
3. **租户隔离业务码**：
   - 验证了跨租户读取返回平台规范的 `errcode.CodeDeviceNotFound / CodeNoPermission`（201001）。

---

## 四、执行与回归证据

### 4.1 独立模块执行

```bash
node run_tests.js --module sparkplug-mqtt-uplink
```

输出摘要：
```text
> API module: Sparkplug B MQTT uplink (57_sparkplug_mqtt_uplink.test.js)
  Evidence label: business
--------------------------------------------------
    √ 1. 发布 Sparkplug B DDATA 设备级遥测，验证自动寻址入库与数值精确度 (173ms)
    √ 2. 发布 Sparkplug B NDATA 节点级遥测（话题无 device_id），验证回退至 edge_node_id 寻址入库 (1044ms)
    √ 3. 非数值指标安全过滤：字符串/布尔指标被安全跳过，严防转为假 0 值（核心物理不变量） (1036ms)
    √ 4. 会话控制类消息（NBIRTH / DBIRTH / STATE）优雅忽略且不报错 (1056ms)
    √ 5. 畸形载荷与截断字节 Fail-Closed 拦截：不污染数据库且服务不崩溃 (821ms)
    √ 6. 未注册 device_number 拦截：丢弃并记录，绝不生成幽灵设备 (670ms)
    √ 7. 跨租户多租户隔离：租户 B 无法越权查询租户 A 的 Sparkplug 遥测 (7ms)

  7 passing (5.29s)
  Pass rate: 100% (7/7)
```

### 4.2 跨模块联合回归

```bash
node run_tests.js --module sparkplug-mqtt-uplink,secrets-storage,device
```

输出摘要：
```text
  Summary:
    Total: 3 modules
    Passed: 3 modules (secrets-storage 10/10, sparkplug-mqtt-uplink 7/7, device passed)
    Failed: 0
    Pass rate: 100%
```

---

## 五、结论与路线图状态

至此，TB-10 包含解码内核（`pkg/sparkplug`）、设备编号查询（`initialize.GetDeviceByNumber`）、MQTT Adapter 上行集成（`handleSparkplugMessage`）以及端到端运行期测试（`57_sparkplug_mqtt_uplink.test.js`）已全部就绪且验证通过。

- **路线图状态**：TB-10 由 `未验证` 正式更新为 **`已闭环`**。
