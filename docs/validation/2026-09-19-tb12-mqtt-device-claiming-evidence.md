# TB-12 设备端 MQTT 自助认领（v1/devices/me/claim & devices/claim）端到端闭环证据（2026-09-19）

> 对应路线图：§7.1 TB-12 设备认领与自动注册（Device Claiming）——设备侧 MQTT 话题接收通道补齐
> 对应竞品：ThingsBoard CE 原生设备开箱认领通道 `v1/devices/me/claim` 与 AetherLink 原生 `devices/claim`
> 自动化用例：`automation_tests/tests/66_tb12_mqtt_device_claiming.test.js`（4/4 全绿）
> 回归用例：`automation_tests/tests/61_device_claim.test.js`（8/8 全绿）
> 运行结果：**VERDICT=PASS**

---

## 一、背景与缺口消除

此前 TB-12 在 2026-09-19 完成了：
1. 112.sql 数据库迁移（`device_claim_tokens`，partial unique 索引）；
2. 租户管理台与 REST API 侧的令牌签发与赎回；
3. 设备列表 UI 生成令牌与顶栏认领设备弹窗（Playwright 31 号用例全绿）。

但遗留缺口明确记载于路线图：
> **"剩余仅 MQTT 设备侧自助认领话题（v1/devices/me/claim）未做"**

ThingsBoard 官方规范支持设备连接 MQTT 之后，主动向 `v1/devices/me/claim` 发送包含 `secretKey` 与有效时长的 JSON 报文，平台自动登记该 Claim Key，下游租户即可直接凭此 SecretKey 赎回设备。

本次工作完成设备侧 MQTT 认领链路开发并彻底闭环该缺口。

---

## 二、交付物与实现细节

1. **主题定义（`backend/internal/adapter/mqttadapter/topics.go`）**：
   - 增加 `TopicPatternDeviceClaim = "v1/devices/me/claim"`（ThingsBoard 标准）；
   - 增加 `TopicPatternNativeDeviceClaim = "devices/claim"`（原生规范）。
2. **主题订阅与分发（`backend/internal/adapter/mqttadapter/subscriber.go`）**：
   - 在 `SubscribeDeviceTopics` 中以 QoS 1 注册上述两条主题，并接入 `handleDeviceClaimMessage`。
3. **认领消息解析与登记（`backend/internal/adapter/mqttadapter/claim.go`）**：
   - 支持解析信封与纯 JSON 载荷（`DeviceClaimMQTTPayload`）；
   - 兼容 `secretKey` 与 `claimKey`，支持 `durationMs` 与 `ttl_seconds`；
   - 提取认证设备 ID 并调用服务层。
4. **服务层设备自主认领（`backend/internal/service/device_claim.go`）**：
   - 新增 `RegisterDeviceClaimFromDevice`：在数据库事务中将设备存量 active 令牌原子替换（`replaced`）并写入新令牌（`active`），SHA-256 哈希安全落库。
5. **Broker ACL 白名单（`mqtt-broker/plugin/aetherlink/util/check_pub_topic.go`）**：
   - 在 `pubList` 增加 `v1/devices/me/claim`、`devices/claim`、`gateway/claim`，保证真实 GMQTT Broker 放行设备认领报文。
6. **单元防御契约（`backend/internal/adapter/mqttadapter/adapter_claim_test.go`）**：
   - 4 项单测全过：非法 JSON 拦截、缺少设备 ID 拦截、缺少密钥拦截、密钥长度（4~128）越界拦截。
7. **活栈契约测试（`automation_tests/tests/66_tb12_mqtt_device_claiming.test.js`）**：
   - 真实 Node.js Socket MQTT 客户端连接本地活栈 Broker；
   - 4/4 全部通过，涵盖 TB 规范发布、原生规范覆盖、错误 Key 404 防探测、接收租户凭设备自报 SecretKey 赎回并完成跨租户所有权转移。

---

## 三、实测结果

```
  TB-12 Device-Side MQTT Claiming [66_tb12_mqtt_device_claiming]
    √ 1. 设备向 v1/devices/me/claim 发布带 secretKey 的认领上报，平台正确持久化 active 令牌 (219ms)
    √ 2. 原生话题 devices/claim 覆盖签发旧令牌（同一设备至多保留 1 条 active） (217ms)
    √ 3. 接收租户使用旧 key 赎回被拒绝（404 not-claimable 防探测）
    √ 4. 接收租户凭最新 secretKey 成功赎回认领，完成设备所有权跨租户转移

  4 passing (608ms)
```

联合回归测试（`61_device_claim.test.js`）：
```
  TB-12 device claiming [61_device_claim]
    √ issues a one-time claim token and never echoes the plaintext again
    √ rejects claiming your own tenant device
    √ rejects a wrong key without leaking whether the device exists
    √ transfers the device to the claiming tenant and consumes the token
    √ rejects replaying the same claim key
    √ allows re-issue after consumption and rejects a revoked token
    √ rejects an expired claim key (2024ms)
    √ validates parameters (empty body, unknown device)

  8 passing (2s)
```

---

## 四、结论

TB-12「设备认领与自动注册（Device Claiming）」此前遗留的唯一缺口——设备端 MQTT 自助认领通道，现已在代码、Broker ACL、单元测试、活栈端到端测试四面彻底闭环，至此 TB-12 达到 100% 全面闭环状态（覆盖 HTTP API、Web UI 控制台、设备端 MQTT 话题通道）。
