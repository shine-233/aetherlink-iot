# TP-4 剩余项取证：调试会话内 Topic 映射的订阅/发布交互（2026-09-19）

> 对应路线图：§7.3-1 TP-4「仍缺：Topic 映射的订阅/发布交互未取证
> （需先开启调试会话，会真在 broker 上开会话）」
> 演练脚本：`automation_tests/scripts/tp4_mqtt_debug_topic_drill.js`（入库，
> 可重复执行；回执 JSONL `automation_tests/scratch/tp4_mqtt_debug_drill.jsonl`）
> 运行结果：**VERDICT=PASS**

---

## 一、背景

TP-4（对标 ThingsPanel 1.1.11 设备诊断/GMQTT 管理面）此前已完成四面接线：
`e2e/25_tp4_device_diagnostics.spec.js` 5/5 全绿，四个组件在真实环境逐个取证，
并修掉 `DeviceMqttDebugWorkbench` 永不可达的死代码（b570eb2）。唯一保留注记是
**调试会话里主题映射的订阅/发布交互从未在真实 broker 会话上取证**。本轮补齐。

## 二、演练内容（全部走真实 HTTP API + 真实 broker 会话）

1. 读设备详情取 `device_number`（主题映射身份段）；
2. `POST /device/:id/mqtt-debug/session` 开启调试会话——隔离 debug 客户端
   **真实连接 broker**（`connected=true`）；
3. 按主题映射白名单订阅两条映射主题：
   `devices/telemetry/control/{number}`、`devices/command/{number}/+`；
4. 向 `devices/telemetry/control/{number}` 发布 47 字节载荷；
5. 快照断言：订阅列表含两条主题、会话消息含 `outbound/outcome=published`
   与 `inbound`（broker 真实往返被会话捕获，message_count=6）；
6. **主题策略防线取证**：跨设备主题订阅（`devices/command/other-device-number/+`）
   被拒（201001）、只读主题发布（`devices/status/{number}`）被拒（201001）——
   状态与回执主题对平台凭据 debug 客户端刻意只读，防伪造在线状态/协议回执；
7. `DELETE` 关闭会话（code=200）。

## 三、实测输出（JSONL 关键行）

```
{"event":"session_opened","session_id":"f3f91121-...","connected":true}
{"event":"subscribed","topics":["devices/telemetry/control/1e59c341-...","devices/command/1e59c341-.../+"]}
{"event":"published","topic":"devices/telemetry/control/1e59c341-...","payload_bytes":47}
{"event":"snapshot","connected":true,"subscriptions":[2 topics],"message_count":6,"published_seen":true,"received_seen":true}
{"event":"policy_guards","cross_device_subscribe_rejected":true,"cross_device_code":201001,"readonly_publish_rejected":true,"readonly_publish_code":201001}
{"event":"verdict","verdict":"PASS"}
{"event":"session_closed","code":200}
```

## 四、结论

- §7.3-1 TP-4 行的「仍缺」注记**已消除**：订阅/发布交互在真实 broker 会话上
  取证通过，且顺带验证了主题策略的跨设备隔离与只读主题防线；
- 设备诊断页 / 调试工作台 / MQTT 调试会话三面此前已取证，TP-4 全项闭环；
- 演练脚本入库可复跑（--device-id / --device-file 两种入口）。
