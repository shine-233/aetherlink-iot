# PHASE-D-D8B 交付说明(设备端 Python SDK)

> 分支 `phase-d/d8b`(基线 main@ac72114),主会话实现。全新目录 `sdk/python/`,零平台代码改动。

## 完成清单

1. **核心客户端** `aetherlink/client.py`:
   - MQTT 主通道(paho-mqtt 可选依赖,缺失时显式报错并指引)——遥测 `devices/telemetry`、
     属性 `devices/attributes/{message_id}`、命令订阅 `devices/command/{device_number}/+`、
     命令响应 `devices/command/response/{message_id}`;
   - 设备凭证 = MQTT username/password(设备接入凭证 voucher,契约 hooks_auth.go buildMQTTVoucher);
   - 离线缓冲(容量环形+丢最旧+计数),重连自动补发;
   - REST 辅通道:标准库 urllib + `X-API-Key` 头(OpenAPI Key)。
2. **离线缓冲** `aetherlink/buffer.py`:drop-oldest 语义 + dropped_count。
3. **测试** `tests/test_aetherlink_sdk.py`:13 例全绿(契约常量对齐 broker ACL/adapter 模板、
   上行 JSON 形状、离线缓冲与补发、命令订阅主题含 device_number、命令回调 JSON/坏载荷包装、
   命令响应主题、REST X-API-Key 头与 4xx 处理;MQTT 以桩客户端覆盖,hermetic)。
4. **示例**:examples/temperature_sensor.py(MQTT 传感器模拟+命令响应)、
   examples/http_uplink.py(纯标准库 REST)。
5. **文档** `sdk/python/README.md`:快速开始 + 契约表(逐条标注仓库出处)。

## 证据
- `python -m unittest discover -s tests` **13 tests OK**;
- `go build ./...`(backend)通过——确认平台代码零改动(仅新增 sdk/ 与文档)。

## 预期冲突点
- 几乎为零(全新顶层目录 sdk/);docs/sdk-python.md 未建(README 已含契约表,集成时决定是否入 docs/ 链接)。

## 未尽事项(留集成)
- 运行期真机联调:隔离栈 broker 实连(devices/telemetry 落库验证);
- MicroPython/Arduino 版本(ROADMAP D8 后续);
- paho-mqtt 2.x API 兼容核查(当前按 1.x Callback API 编写)。
