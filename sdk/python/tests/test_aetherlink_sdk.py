"""AetherLink Python SDK 单测(stdlib unittest,不依赖 paho——MQTT 通道以桩客户端覆盖)。"""

import json
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

sys.path.insert(0, __file__.rsplit("/", 1)[0] + "/..")

from aetherlink import AetherLinkClient, ClientConfig, OfflineBuffer  # noqa: E402
from aetherlink.client import (  # noqa: E402
    ATTRIBUTES_TOPIC,
    COMMAND_RESPONSE_TOPIC,
    COMMAND_SUBSCRIBE_TOPIC,
    TELEMETRY_TOPIC,
)


class FakeMqttClient:
    """paho 客户端桩:记录 publish,驱动 connect 回调。"""

    def __init__(self) -> None:
        self.published: list = []
        self.subscribed: list = []
        self.on_connect = None
        self.on_message = None
        self.on_disconnect = None

    def username_pw_set(self, username, password=None):
        self.username = username

    def tls_set(self, **_kwargs):
        pass

    def connect(self, _host, _port, _keepalive):
        # 模拟 broker 连接建立完成
        self.on_connect(self, None, None, 0)

    def loop_start(self):
        pass

    def loop_stop(self):
        pass

    def disconnect(self):
        pass

    def subscribe(self, topic):
        self.subscribed.append(topic)

    def publish(self, topic, payload, qos=0):
        self.published.append((topic, payload, qos))

    def deliver(self, topic, payload):
        class Msg:
            pass

        msg = Msg()
        msg.topic = topic
        msg.payload = payload.encode("utf-8")
        self.on_message(self, None, msg)


def make_connected_client(**overrides) -> AetherLinkClient:
    config_kwargs = dict(host="127.0.0.1", username="dev-001", password="tok-abc")
    config_kwargs.update(overrides)
    client = AetherLinkClient(ClientConfig(**config_kwargs))
    fake = FakeMqttClient()
    # 替换 paho 构造:connect() 内 import paho —— 用桩注入并手工接线回调绕过。
    fake.on_message = client._on_message
    client._client = fake
    client._on_connect(fake, None, None, 0)
    return client


class TestContractConstants(unittest.TestCase):
    """契约对齐:主题与仓库代码断言一致(出处见各常量注释)。"""

    def test_uplink_topics_match_broker_acl(self):
        # mqtt-broker/plugin/aetherlink/util/check_pub_topic.go:48
        self.assertEqual(TELEMETRY_TOPIC, "devices/telemetry")

    def test_command_topics_match_adapter(self):
        # backend/internal/adapter/mqttadapter/topics.go:53 (devices/command/%s/%s)
        self.assertEqual(COMMAND_SUBSCRIBE_TOPIC.format(device_number="gw-1"), "devices/command/gw-1/+")
        self.assertEqual(COMMAND_RESPONSE_TOPIC.format(message_id="m1"), "devices/command/response/m1")

    def test_attributes_topic(self):
        self.assertEqual(ATTRIBUTES_TOPIC.format(message_id="m2"), "devices/attributes/m2")


class TestUplink(unittest.TestCase):
    def test_publish_telemetry_json_shape(self):
        client = make_connected_client()
        ok = client.publish_telemetry({"temperature": 26.5})
        self.assertTrue(ok)
        topic, payload, _qos = client._client.published[0]
        self.assertEqual(topic, "devices/telemetry")
        self.assertEqual(json.loads(payload), {"temperature": 26.5})

    def test_offline_publish_buffers_and_flushes(self):
        client = make_connected_client()
        client._connected.clear()  # 模拟断线
        self.assertFalse(client.publish_telemetry({"k": 1}))
        self.assertFalse(client.publish_telemetry({"k": 2}))
        self.assertEqual(len(client._buffer), 2)
        client._connected.set()
        sent = client.flush_buffer()
        self.assertEqual(sent, 2)
        self.assertEqual(len(client._buffer), 0)


class TestOfflineBuffer(unittest.TestCase):
    def test_drop_oldest_when_full(self):
        buffer = OfflineBuffer(capacity=3)
        for i in range(5):
            buffer.push("devices/telemetry", json.dumps({"i": i}))
        items = buffer.drain()
        self.assertEqual([json.loads(p)["i"] for _t, p, _q in items], [2, 3, 4])
        self.assertEqual(buffer.dropped_count, 2)

    def test_invalid_capacity(self):
        with self.assertRaises(ValueError):
            OfflineBuffer(capacity=0)


class TestCommandDownlink(unittest.TestCase):
    def test_command_subscribe_topic_uses_device_number(self):
        client = make_connected_client(device_number="dev-001")
        self.assertIn("devices/command/dev-001/+", client._client.subscribed)

    def test_command_handler_receives_json_payload(self):
        client = make_connected_client(device_number="dev-001")
        received = []

        def handler(payload, topic):
            received.append((payload, topic))

        client.on_command(handler)
        client._client.deliver("devices/command/dev-001/m-9", json.dumps({"identify": "set_speed", "params": {"speed": 60}}))
        payload, topic = received[0]
        self.assertEqual(payload["identify"], "set_speed")
        self.assertTrue(topic.startswith("devices/command/dev-001/"))

    def test_malformed_payload_wraps_raw(self):
        client = make_connected_client(device_number="dev-001")
        received = []
        client.on_command(lambda payload, topic: received.append(payload))
        client._client.deliver("devices/command/dev-001/m-9", "not-json")
        self.assertIn("_raw", received[0])

    def test_command_response_topic(self):
        client = make_connected_client(device_number="dev-001")
        client.publish_command_response("m-9", {"ok": True})
        topic, payload, _q = client._client.published[0]
        self.assertEqual(topic, "devices/command/response/m-9")
        self.assertEqual(json.loads(payload), {"ok": True})


class TestRestChannel(unittest.TestCase):
    def test_post_json_sends_api_key_header(self):
        captured = {}

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                captured["path"] = self.path
                captured["key"] = self.headers.get("X-API-Key")
                length = int(self.headers.get("Content-Length", 0))
                captured["body"] = json.loads(self.rfile.read(length))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b'{"code":0,"data":{}}')

            def log_message(self, *_args):
                pass

        server = HTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            client = AetherLinkClient(
                ClientConfig(host="127.0.0.1", api_base=f"http://127.0.0.1:{server.server_port}", api_key="key-1")
            )
            result = client.post_json("/device/data/report", {"k": 1})
            self.assertEqual(result["code"], 0)
            self.assertTrue(captured["path"].startswith("/device/data/report"))
            self.assertEqual(captured["key"], "key-1")
            self.assertEqual(captured["body"], {"k": 1})
        finally:
            server.shutdown()

    def test_post_json_requires_config(self):
        client = AetherLinkClient(ClientConfig(host="127.0.0.1"))
        with self.assertRaises(RuntimeError):
            client.post_json("/x", {})


if __name__ == "__main__":
    unittest.main()
