# AetherLink Python SDK(设备端)

最小依赖的设备端接入 SDK:MQTT 主通道(paho-mqtt 可选)+ REST 辅通道(标准库)。

## 快速开始

```python
from aetherlink import AetherLinkClient, ClientConfig

client = AetherLinkClient(ClientConfig(host="平台MQTT地址", username="设备凭证用户名", password="设备凭证密码", device_number="dev-001"))
client.connect()
client.publish_telemetry({"temperature": 26.5})
```

## 契约表(与平台代码对齐)

| 能力 | 主题/端点 | 出处 |
| --- | --- | --- |
| 遥测上报 | `devices/telemetry`,载荷 `{key: value}` | mqtt-broker/plugin/aetherlink/util/check_pub_topic.go:48 |
| 属性上报 | `devices/attributes/{message_id}` | 同上 :52 |
| 命令订阅 | `devices/command/{device_number}/+` | backend/internal/adapter/mqttadapter/topics.go:53 |
| 命令响应 | `devices/command/response/{message_id}` | check_pub_topic.go:56 |
| 设备凭证 | MQTT username/password = 设备接入凭证 voucher 的 username/password | hooks_auth.go buildMQTTVoucher |
| REST | `X-API-Key` 头携带 OpenAPI Key | internal/middleware/cors.go:20 允许头 |

## 测试

```
python -m unittest discover -s tests -v
```

MQTT 通道在测试中以桩客户端覆盖(不依赖 paho/broker);运行期真机联调由主会话集成阶段执行。
