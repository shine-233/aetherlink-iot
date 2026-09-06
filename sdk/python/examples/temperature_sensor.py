"""温度传感器模拟:MQTT 上行遥测 + 命令响应(需 pip install paho-mqtt)。"""
import random
import time

from aetherlink import AetherLinkClient, ClientConfig

config = ClientConfig(
    host="192.168.1.10",          # 平台 MQTT 地址
    mqtt_port=1883,
    username="dev-001",           # 设备接入凭证 voucher 的 username
    password="your-access-token", # 设备接入凭证 voucher 的 password
    device_number="dev-001",      # 命令订阅主题 devices/command/dev-001/+
)
client = AetherLinkClient(config)
client.connect()


def on_command(payload, _topic):
    print("收到命令:", payload)
    # 命令载荷含 message_id 时按契约回应 devices/command/response/{message_id}
    message_id = payload.get("message_id")
    if message_id:
        client.publish_command_response(message_id, {"ok": True})


client.on_command(on_command)

while True:
    client.publish_telemetry({"temperature": round(random.uniform(20, 30), 1)})
    time.sleep(5)
