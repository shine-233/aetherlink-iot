"""AetherLink 设备端 SDK(PHASE-D-D8b)。

契约出处(仓库内引用):
- 上行主题白名单: mqtt-broker/plugin/aetherlink/util/check_pub_topic.go:48-56
- 命令下发主题:   backend/internal/adapter/mqttadapter/topics.go:53 (devices/command/{device_number}/{message_id})
- 命令响应主题:   mqtt-broker/plugin/aetherlink/util/check_pub_topic.go:56 (devices/command/response/+)
- 设备凭证:       MQTT username/password 即设备接入凭证 voucher 的 username/password
                  (mqtt-broker/plugin/aetherlink/hooks_auth.go buildMQTTVoucher)
"""

from .client import AetherLinkClient, ClientConfig
from .buffer import OfflineBuffer

__all__ = ["AetherLinkClient", "ClientConfig", "OfflineBuffer"]
__version__ = "0.1.0"
