/**
 * 待执行 E2E（ROADMAP A3）：设备影子「离线→上线投递 / TTL 过期 / 取消」真实浏览器链路。
 * 运行前提：隔离栈全活（PG5433/Redis6379/Broker1883/Backend9999/Preview9725）+ 一个可下线的 MQTT 设备桩。
 * 本文件暂放 pending-e2e 目录，避免被 run_tests/playwright 的 e2e/*.spec.js 契约计数误收录；
 * 接入正式套件时移入 automation_tests/e2e/ 并同步 00_e2e_metadata_contract 注册表。
 *
 * 场景：
 *  1) 准备：登录租户管理员；创建/选择一台离线设备（broker 侧踢下线）。
 *  2) 离线投递：POST /api/v1/device/shadow/{deviceId}（type=command,ttl=60）→ 状态应为 pending；
 *     通过 broker 使设备上线并回一条 MQTT 消息 → 3s 内钩子投递 → 断言 GET 列表该消息 delivered。
 *  3) TTL 过期：创建 ttl=5 的影子消息，等待 cron/到期逻辑 → 断言 expired（可调用内部到期接口或等待≤60s 轮询）。
 *  4) 取消：对 pending 消息 DELETE /api/v1/device/shadow/{deviceId}/{msgId} → 断言 canceled，且后续上线不再投递。
 *
 * 标识：AETHERLINK_E2E_A3_SHADOW
 */
const { test, expect } = require('@playwright/test')

test.describe('A3 device shadow offline delivery chain', () => {
  test('shadow message delivered on device online, expires on ttl, canceled stays canceled', async ({
    page,
    request
  }) => {
    // 占位：接入隔离栈后用 prepare_local_accounts 的种子账号替换。
    const apiBase = process.env.API_TARGET || 'http://127.0.0.1:9999/api/v1'
    const login = await request.post(`${apiBase}/login`, { data: { email: 'admin@local.dev', password: 'Aa123456!' } })
    expect(login.ok()).toBeTruthy()
    const { token } = await login.json()
    const headers = { 'x-token': token }

    const device = 'e2e-shadow-device-a3'
    const resp = await request.post(`${apiBase}/device/shadow/${device}`, {
      headers,
      data: { message_type: 'command', payload: { method: 'reboot' }, ttl_seconds: 30 }
    })
    expect(resp.ok()).toBeTruthy()

    const list = await (await request.get(`${apiBase}/device/shadow/${device}?status=pending`, { headers })).json()
    expect(Array.isArray(list.data)).toBeTruthy()

    // 以下 MQTT 上线/ACK 与 TTL 等待依赖真实栈，详细断言见 PR 描述与 VALIDATION.md A3。
    test.info().annotations.push({
      type: 'integration-blocked',
      description: 'needs live broker device ack path (see WORKPLAN A3 E2E)'
    })
  })
})
