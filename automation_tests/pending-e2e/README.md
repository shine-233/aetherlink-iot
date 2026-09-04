# pending-e2e — 真实栈 E2E 挂起区（Runbook）

> 本目录存放**依赖完整生产形态栈**才能执行的 E2E。它们已编写但未接入正式套件，
> 目的是不被 `run_tests`/playwright 的 `e2e/*.spec.js` 契约计数误收录。
> 接入时：①移入 `automation_tests/e2e/`；②同步 `00_e2e_metadata_contract` 注册表。

## 现存清单

| 规格 | 标识 | 阻塞原因 |
|---|---|---|
| shadow-offline-delivery.spec.js | `AETHERLINK_E2E_A3_SHADOW` | 需 Broker1883 + Backend9999 + Preview9725 全活 + Playwright 浏览器 |

## A3 影子 E2E 解除挂起 Runbook（按序执行）

### 前提（五件套全活）
1. **Broker 1883**（已实测可构建，2026-09-04）：在仓库根 `go build ./mqtt-broker/cmd/gmqttd/` →
   `gmqttd(.exe)`。运行配置：拷贝 `mqtt-broker/cmd/gmqttd/aetherlink.example.yml` 为
   `aetherlink.yml`，替换全部 CHANGE_ME——其中 `db.psql.*` 与 `redis.*` 必须指向平台**同一**数据库
   与 Redis（aetherlink 插件直连共享库做设备凭证校验，即 devices.voucher_hash 体系；
   dev-local 对应 127.0.0.1:5433/aetherlink_iot_local，Redis db 号与后端 conf 对齐），
   `mqtt_session_revocations.broker_id` 手工部署必须显式设置，随后
   `./gmqttd -c aetherlink.yml` 启动（监听 `mqtt.broker` 指定的 tcp://127.0.0.1:1883）。
2. **Backend 9999**（完整端口，非 9099 验证栈）：`uplink.enable: true`（影子投递钩子挂在
   status_flow/上行首消息路径，uplink 关闭则投递不发生）。
3. **Preview 9725**：前端预览服务（规格走浏览器链路）。
4. **种子账号**：`prepare_local_accounts` 产出的租户管理员（规格内占位邮箱
   `admin@local.dev` / `Aa123456!` 需与种子一致）。
5. **Playwright**：`npm i -D @playwright/test && npx playwright install chromium`。

### 规格补全（当前为占位，接入前必须补）
- ① MQTT 上线触发：broker 踢下线/重连设备桩（`e2e-shadow-device-a3`），投递后回一条
  MQTT 消息驱动 ACK；断言 GET 列表 `delivered`（3s 钩子窗口）。
- ② TTL 过期：创建 ttl=5 的消息，轮询（≤60s）断言 `expired`（或调用内部到期接口）。
- ③ 取消：DELETE pending 消息 → 断言 `canceled` 且重新上线不再投递。
- 详细语义参考 PR 描述与 `VALIDATION.md` A3 段。

### 接入
1. 移动规格：`git mv automation_tests/pending-e2e/shadow-offline-delivery.spec.js automation_tests/e2e/`。
2. 在 `00_e2e_metadata_contract` 注册表登记标识 `AETHERLINK_E2E_A3_SHADOW`。
3. `npx playwright test` 全绿后，ROADMAP A3 的 E2E 行可去掉"pending"注记。

### 验收标准（ROADMAP A3 最后一格）
三个场景断言全绿：离线→上线投递 delivered、TTL 过期 expired、取消 canceled 且不再投递。
