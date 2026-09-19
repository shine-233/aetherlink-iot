# P2.1 真实协议适配器（CAN 与 BACnet/IP）实现与运行期证据

> 日期：2026-09-19 · 状态：**已闭环（done）** · 对应路线图任务：**P2.1 协议插件 SDK**
> 关联代码：`backend/pkg/pluginsdk/can_adapter.go`、`can_adapter_test.go`、`bacnet_adapter.go`、`bacnet_adapter_test.go`、`router/apps/plugin_registry_http_test.go`

---

## 1. 任务背景与缺口消除

路线图 P2.1（协议插件 SDK）此前的实现状态为 `partial`，缺口类型为 `未实现`：
> `未闭环：真实外部协议适配器（CAN/BACnet/BLE/LoRaWAN 按客户需求接入）。`

虽然 `pkg/pluginsdk` 契约包与 HTTP 注册验签链路已实现，但缺少真正的协议适配器实现。
本批次正式交付两套工业级与楼宇自控级真实协议适配器，全面满足 `ProtocolAdapter` 接口与 `Manifest` 契约：
1. **工业 CAN 协议适配器（Industrial CAN 2.0A/B）**：面向车载、PLC 及重工 ECU，支持 11 位标准帧与 29 位扩展帧编解码、波特率校验、节点发现、物理量转换与控制下发；
2. **楼宇自控 BACnet/IP 协议适配器（ISO 16484-5 / ANSI/ASHRAE 135）**：面向 HVAC 与冷水机组、风阀控制，支持 22 位设备实例 ID、对象属性读写（AI/AO/BI/BO/AV）与安全上下限控制。

---

## 2. 交付物明细

### 2.1 适配器核心实现

1. `backend/pkg/pluginsdk/can_adapter.go`:
   - 接口契约：完整实现 `pluginsdk.ProtocolAdapter`（`Name`、`ValidateConfig`、`Connect`、`Discover`、`ReadTelemetry`、`WriteCommand`、`Health`、`Close`）；
   - 参数校验：`interface_name`（长度 3~32）、`baud_rate`（白名单 125k/250k/500k/1M）、`node_id`（1~127）；
   - 遥测解码：
     - `0x18F00400`: Engine RPM（0.125 rpm/bit）、Coolant Temp（-40°C 偏移）；
     - `0x18F00500`: Oil Pressure（2 kPa/bit）、Battery Voltage（0.01 V/bit）；
     - `0x18F00600`: Relay Bitfield（继电器闭合状态）；
   - 指令下发：支持 `set_relay`（指定闭合/断开）、`emergency_stop`（广播急停帧 `0x18EF00FF`）以及标准 `<= 8` 字节原生 CAN 载荷；
   - 自描述清单：`CANAdapterManifest()` 声明完整配置 Schema、点表与脱敏凭证映射。

2. `backend/pkg/pluginsdk/bacnet_adapter.go`:
   - 接口契约：完整实现 `pluginsdk.ProtocolAdapter`；
   - 参数校验：`ip_address`（IPv4 强解析校验）、`port`（1024~65535，默认 47808 即 0xBAC0）、`device_instance`（0~4194303 22-bit 上限）；
   - 属性映射：
     - AI:1 `room_temp`（室内温度，°C）；
     - AI:2 `air_flow`（风量，cfm）；
     - AI:3 `chilled_water_temp`（冷冻水温，°C）；
     - BI:1 `fan_running`（风机运转状态，bool）；
     - AO:1 `cooling_valve_pos`（水阀开度，%）；
   - 命令控制：`set_room_temp`（带 16.0°C~32.0°C 安全区间防御拦截）、`override_fan`（风机强制启停）；
   - 自描述清单：`BACnetAdapterManifest()`。

---

## 3. 运行期验证结果

### 3.1 单元测试与协议编解码测试

执行命令：
```powershell
go test -v ./pkg/pluginsdk/ -count=1
```

输出日志：
```
=== RUN   TestBACnetAdapterManifestValidAndSigned
--- PASS: TestBACnetAdapterManifestValidAndSigned (0.00s)
=== RUN   TestBACnetAdapterValidateConfig
--- PASS: TestBACnetAdapterValidateConfig (0.00s)
=== RUN   TestBACnetAdapterConnectAndDiscover
--- PASS: TestBACnetAdapterConnectAndDiscover (0.00s)
=== RUN   TestBACnetAdapterReadTelemetry
--- PASS: TestBACnetAdapterReadTelemetry (0.00s)
=== RUN   TestBACnetAdapterWriteCommand
--- PASS: TestBACnetAdapterWriteCommand (0.00s)
=== RUN   TestCANAdapterManifestValid
--- PASS: TestCANAdapterManifestValid (0.00s)
=== RUN   TestCANAdapterValidateConfig
--- PASS: TestCANAdapterValidateConfig (0.00s)
=== RUN   TestCANAdapterConnectAndDiscover
--- PASS: TestCANAdapterConnectAndDiscover (0.00s)
=== RUN   TestCANAdapterReadTelemetryDecoding
--- PASS: TestCANAdapterReadTelemetryDecoding (0.00s)
=== RUN   TestCANAdapterWriteCommand
--- PASS: TestCANAdapterWriteCommand (0.00s)
=== RUN   TestCheckHostCompatibility
--- PASS: TestCheckHostCompatibility (0.00s)
=== RUN   TestManifestValidate
--- PASS: TestManifestValidate (0.00s)
=== RUN   TestManifestValidateConfigSchema
--- PASS: TestManifestValidateConfigSchema (0.00s)
=== RUN   TestParseManifest
--- PASS: TestParseManifest (0.00s)
=== RUN   TestSchemaIntegerRejectsFloat
--- PASS: TestSchemaIntegerRejectsFloat (0.00s)
=== RUN   TestSignAndVerifyManifestRoundtrip
--- PASS: TestSignAndVerifyManifestRoundtrip (0.00s)
=== RUN   TestVerifyManifestSignatureRejects
--- PASS: TestVerifyManifestSignatureRejects (0.00s)
=== RUN   TestCanonicalManifestExcludesSignatureFields
--- PASS: TestCanonicalManifestExcludesSignatureFields (0.00s)
PASS
ok  	aetherlink-iot/backend/pkg/pluginsdk	0.933s
```
**结论：18/18 全部通过，用时 0.933s。**

### 3.2 真实 Gin + 真实 PostgreSQL HTTP 注册与验签闭环

测试用例：`backend/router/apps/plugin_registry_http_test.go`
- `TestPluginRegistryManifestHTTPPathOnPostgres`
- `TestRegisterRealIndustrialAdaptersHTTPPathOnPostgres`

执行命令：
```powershell
$env:AETHERLINK_TEST_PSQL_DSN="host=127.0.0.1 port=55433 user=postgres dbname=aetherlink_migchain_115 sslmode=disable"
go test -v ./router/apps/ -run "TestPluginRegistryManifestHTTPPathOnPostgres|TestRegisterRealIndustrialAdaptersHTTPPathOnPostgres" -count=1
```

输出日志：
```
=== RUN   TestPluginRegistryManifestHTTPPathOnPostgres
--- PASS: TestPluginRegistryManifestHTTPPathOnPostgres (0.09s)
=== RUN   TestRegisterRealIndustrialAdaptersHTTPPathOnPostgres
--- PASS: TestRegisterRealIndustrialAdaptersHTTPPathOnPostgres (0.06s)
PASS
ok  	aetherlink-iot/backend/router/apps	1.312s
```
**结论：真实 PostgreSQL 上成功持久化 CAN 与 BACnet 两个适配器的签名清单，验签通过。**
