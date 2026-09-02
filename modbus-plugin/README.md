# Modbus TCP 插件（ROADMAP B1）

独立进程的 Modbus TCP 协议适配器：周期采集 PLC/RTU 寄存器并按平台遥测契约上报；
订阅平台命令下发主题，把 `{identify, value}` 写回可写寄存器。

## 集成边界

数据面复用平台既有约定，不引入私有协议：

- **遥测上行**：以设备自身 MQTT 凭证连接 broker，向 `mqtt.telemetry.subscribe_topic`（默认 `devices/telemetry`）发布 `{key: value}` JSON 快照——与真实网关设备行为一致，设备归属由 broker 认证链路绑定。
- **命令下行**：订阅 `devices/command/{device_number}/+`，解析平台 `PutMessageForCommand` 形状（`{identify, value}`），按点表键名匹配可写寄存器执行 FC5/FC6/FC16 写入。
- ROADMAP 原计划的自定义 gRPC 通道暂缓：当前平台侧插件运行时（pluginruntime）为 HTTP 边界、数据面为 MQTT，gRPC 待平台暴露插件网关后再评估。

## 点表配置

参考 `config.example.json`：

```jsonc
{
  "mqtt": { "host": "broker", "port": 1883 },
  "poll_interval_seconds": 10,
  "devices": [{
    "device_number": "modbus-plc-01",       // 平台设备编号
    "username": "...",                       // 该设备的 MQTT 凭证
    "password": "...",
    "target": { "host": "192.168.1.50", "port": 502, "unit_id": 1 },
    "registers": [
      { "key": "temperature", "type": "input",   "address": 100, "data_type": "i16", "multiplier": 0.1 },
      { "key": "power",       "type": "holding", "address": 200, "data_type": "f32" },
      { "key": "switch_1",    "type": "coil",    "address": 10,  "writable": true }
    ]
  }]
}
```

- `type`: `holding | input | coil | discrete`
- `data_type`: `u16 | i16 | u32 | i32 | f32`（寄存器类，大端字序）；coil/discrete 固定 bool
- 读值 = `raw * multiplier + offset`；写值做逆变换；仅 `writable: true` 的 holding/coil 可被命令写入

## 运行

```bash
# 本地
MODBUS_PLUGIN_CONFIG=./config.json go run ./cmd/modbus-plugin

# Compose（可选 profile）
docker compose --profile modbus up -d modbus-plugin
```

健康检查：`GET :8090/healthz`。

## 安全注意

配置文件包含设备凭证，必须以只读挂载/密钥管理方式下发，禁止提交真实凭证或把凭证打进镜像。
