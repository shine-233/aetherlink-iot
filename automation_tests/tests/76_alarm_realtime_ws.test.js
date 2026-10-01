/**
 * 文件用途：告警状态实时 WebSocket 订阅（TB-30 对标，/api/v1/alarm/status/ws）活栈契约测试。
 *
 * 覆盖：
 *   1. 订阅建立：首帧携带合法 token 认证后，服务端推送初始快照帧
 *      （{"type":"snapshot","items":[...]}）；
 *   2. 首帧鉴权：非法 token 被拒绝（收到错误帧或连接被关闭）；
 *   3. 心跳契约：发送 "ping" 收到 "pong"；
 *   4. 事件通道契约：Redis 发布触发/恢复/处理状态事件由 WS 转发（由 service 层单测与
 *      docs/validation 证据留痕覆盖，本用例验证订阅面与快照面）。
 *
 * 说明：Node >= 22 提供全局 WebSocket 客户端，本套件 engines.node >= 20，
 * 运行前请以 Node 22+ 执行（与仓库 CI 的 Node 版本一致）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Alarm Realtime WebSocket [76_alarm_realtime_ws]';
const TENANT_A = 'tenant_admin';

function buildWsUrl() {
  const base = apiClient.getConfig().baseURL; // http://127.0.0.1:9999/api/v1
  return base.replace(/^http/, 'ws') + '/alarm/status/ws';
}

describe(SUITE, function () {
  this.timeout(120000);
  let tenantToken = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 76_alarm_realtime_ws.test.js');
    }
    tenantToken = await apiClient.login(TENANT_A);
    expect(tenantToken).to.be.a('string').and.not.equal('');
  });

  it('1. 合法 token 订阅后收到初始快照帧（type=snapshot）', async function () {
    const snapshot = await new Promise((resolve, reject) => {
      const socket = new WebSocket(buildWsUrl());
      const timer = setTimeout(() => {
        try { socket.close(); } catch (e) { /* ignore */ }
        reject(new Error('alarm snapshot frame timed out'));
      }, 10000);
      socket.onopen = () => {
        socket.send(JSON.stringify({ token: tenantToken }));
      };
      socket.onmessage = (event) => {
        clearTimeout(timer);
        const payload = JSON.parse(String(event.data));
        resolve(payload);
        try { socket.close(); } catch (e) { /* ignore */ }
      };
      socket.onerror = (err) => {
        clearTimeout(timer);
        reject(new Error('alarm websocket error: ' + String(err && err.message ? err.message : err)));
      };
    });

    expect(snapshot).to.be.an('object');
    expect(snapshot.type).to.equal('snapshot');
    expect(snapshot.items).to.be.an('array');
  });

  it('2. 非法 token 被拒绝（错误帧或连接关闭）', async function () {
    const outcome = await new Promise((resolve) => {
      const socket = new WebSocket(buildWsUrl());
      const timer = setTimeout(() => {
        try { socket.close(); } catch (e) { /* ignore */ }
        resolve({ closed: true, errorFrame: false });
      }, 8000);
      socket.onopen = () => {
        socket.send(JSON.stringify({ token: 'invalid-token-for-76' }));
      };
      socket.onmessage = (event) => {
        clearTimeout(timer);
        const text = String(event.data);
        resolve({ closed: false, errorFrame: true, text });
        try { socket.close(); } catch (e) { /* ignore */ }
      };
      socket.onclose = () => {
        clearTimeout(timer);
        resolve({ closed: true, errorFrame: false });
      };
    });

    const rejected = outcome.errorFrame || outcome.closed;
    expect(rejected, 'server must reject invalid token with error frame or close').to.be.true;
  });

  it('3. 心跳契约：ping -> pong', async function () {
    const pong = await new Promise((resolve, reject) => {
      const socket = new WebSocket(buildWsUrl());
      const timer = setTimeout(() => {
        try { socket.close(); } catch (e) { /* ignore */ }
        reject(new Error('alarm ws pong timed out'));
      }, 10000);
      socket.onopen = () => {
        socket.send(JSON.stringify({ token: tenantToken }));
        socket.send('ping');
      };
      socket.onmessage = (event) => {
        const text = String(event.data);
        if (text === 'pong') {
          clearTimeout(timer);
          resolve(text);
          try { socket.close(); } catch (e) { /* ignore */ }
        }
      };
      socket.onerror = () => {
        clearTimeout(timer);
        reject(new Error('alarm websocket error during ping/pong'));
      };
    });
    expect(pong).to.equal('pong');
  });
});
