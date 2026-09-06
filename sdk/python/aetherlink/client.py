"""AetherLink 设备端客户端。

MQTT 为主通道(paho-mqtt 可选依赖,未安装时 connect() 抛出带指引的错误);
REST 为辅通道(标准库 urllib,X-API-Key 头携带 OpenAPI Key)。
命令回调:订阅 devices/command/{device_number}/+,收到命令后业务方可调用
publish_command_response 回应(主题 devices/command/response/{message_id})。
"""

import json
import ssl
import threading
import urllib.error
import urllib.request
from typing import Any, Callable, Dict, Optional

from .buffer import OfflineBuffer

TELEMETRY_TOPIC = "devices/telemetry"
COMMAND_RESPONSE_TOPIC = "devices/command/response/{message_id}"
COMMAND_SUBSCRIBE_TOPIC = "devices/command/{device_number}/+"
ATTRIBUTES_TOPIC = "devices/attributes/{message_id}"


class ClientConfig:
    def __init__(
        self,
        host: str,
        mqtt_port: int = 1883,
        username: str = "",
        password: str = "",
        device_number: str = "",
        api_base: str = "",
        api_key: str = "",
        keepalive: int = 60,
        tls: bool = False,
        offline_capacity: int = 1000,
    ) -> None:
        self.host = host
        self.mqtt_port = mqtt_port
        self.username = username
        self.password = password
        # device_number 用于命令订阅主题;缺省回落 username。
        self.device_number = device_number or username
        self.api_base = api_base.rstrip("/") if api_base else ""
        self.api_key = api_key
        self.keepalive = keepalive
        self.tls = tls
        self.offline_capacity = offline_capacity


class AetherLinkClient:
    """设备端客户端:遥测/属性上报、命令订阅与响应、离线缓冲补发。"""

    def __init__(self, config: ClientConfig) -> None:
        self._config = config
        self._client: Any = None
        self._connected = threading.Event()
        self._command_handler: Optional[Callable[[Dict[str, Any], str], None]] = None
        self._buffer = OfflineBuffer(config.offline_capacity)
        self._subscribe_topic = COMMAND_SUBSCRIBE_TOPIC.format(device_number=config.device_number)

    # ---- 生命周期 ----

    def connect(self) -> None:
        try:
            import paho.mqtt.client as mqtt  # type: ignore
        except ImportError as exc:  # pragma: no cover - 环境相关
            raise RuntimeError(
                "MQTT 通道需要 paho-mqtt:请执行 pip install paho-mqtt,"
                "或改用 HTTP 通道(post_json + 平台数据服务)"
            ) from exc

        client = mqtt.Client()
        client.on_connect = self._on_connect
        client.on_message = self._on_message
        client.on_disconnect = self._on_disconnect
        if self._config.username:
            client.username_pw_set(self._config.username, self._config.password or None)
        if self._config.tls:
            client.tls_set(cert_reqs=ssl.CERT_REQUIRED)
        self._client = client
        client.connect(self._config.host, self._config.mqtt_port, self._config.keepalive)
        client.loop_start()
        if not self._connected.wait(timeout=10):
            raise RuntimeError("mqtt connect timeout (check host/credentials)")

    def disconnect(self) -> None:
        if self._client is not None:
            self._client.loop_stop()
            self._client.disconnect()
            self._client = None
        self._connected.clear()

    @property
    def connected(self) -> bool:
        return self._connected.is_set()

    # ---- 上行 ----

    def publish_telemetry(self, data: Dict[str, Any], qos: int = 0) -> bool:
        """遥测上报:主题 devices/telemetry,载荷 {key: value}(契约 check_pub_topic.go:48)。"""
        return self._publish(TELEMETRY_TOPIC, data, qos)

    def publish_attributes(self, data: Dict[str, Any], message_id: str, qos: int = 0) -> bool:
        """属性上报:主题 devices/attributes/{message_id}。"""
        return self._publish(ATTRIBUTES_TOPIC.format(message_id=message_id), data, qos)

    def publish_command_response(self, message_id: str, result: Dict[str, Any], qos: int = 0) -> bool:
        """命令响应:主题 devices/command/response/{message_id}(契约 check_pub_topic.go:56)。"""
        return self._publish(COMMAND_RESPONSE_TOPIC.format(message_id=message_id), result, qos)

    def _publish(self, topic: str, data: Dict[str, Any], qos: int) -> bool:
        payload_json = json.dumps(data, ensure_ascii=False, separators=(",", ":"))
        if not self.connected or self._client is None:
            self._buffer.push(topic, payload_json, qos)
            return False
        self._client.publish(topic, payload_json, qos=qos)
        return True

    def flush_buffer(self) -> int:
        """补发离线缓冲,返回补发条数。"""
        sent = 0
        for topic, payload_json, qos in self._buffer.drain():
            if self.connected and self._client is not None:
                self._client.publish(topic, payload_json, qos=qos)
                sent += 1
            else:
                self._buffer.push(topic, payload_json, qos)
        return sent

    # ---- 下行 ----

    def on_command(self, handler: Callable[[Dict[str, Any], str], None]) -> None:
        """注册命令回调(连接后自动订阅 devices/command/{device_number}/+)。"""
        self._command_handler = handler

    def _on_connect(self, _client: Any, _userdata: Any, _flags: Any, _rc: Any) -> None:
        self._connected.set()
        if self._config.device_number:
            self._client.subscribe(self._subscribe_topic)
        self.flush_buffer()

    def _on_disconnect(self, _client: Any, _userdata: Any, _rc: Any) -> None:
        self._connected.clear()

    def _on_message(self, _client: Any, _userdata: Any, message: Any) -> None:
        if self._command_handler is None:
            return
        try:
            payload = json.loads(message.payload.decode("utf-8"))
        except (ValueError, UnicodeDecodeError):
            payload = {"_raw": message.payload.decode("utf-8", errors="replace")}
        self._command_handler(payload, message.topic)

    # ---- REST 通道(标准库实现) ----

    def post_json(self, path: str, payload: Dict[str, Any], timeout: int = 10) -> Dict[str, Any]:
        """以 OpenAPI Key(X-API-Key)调用平台数据服务 REST。"""
        if not self._config.api_base or not self._config.api_key:
            raise RuntimeError("api_base/api_key 未配置,无法使用 REST 通道")
        url = self._config.api_base + path
        request = urllib.request.Request(
            url,
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json", "X-API-Key": self._config.api_key},
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=timeout) as resp:
                body = resp.read().decode("utf-8")
        except urllib.error.HTTPError as exc:
            raise RuntimeError(f"platform api error: HTTP {exc.code}") from exc
        return json.loads(body) if body else {}
