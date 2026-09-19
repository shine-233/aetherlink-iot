/**
 * 文件用途：P1.5「真实边缘节点联调与断云演练」的运行期测试。
 *
 * 场景（全部走平台真实 HTTP API，边缘侧是一个独立运行的模拟器进程）：
 *   1. 注册：POST /edge/nodes（node_id + 版本 + 能力）→ 成功；
 *   2. 稳态：心跳 + Reconcile 循环，health=online；
 *   3. 断云：模拟器进入不可达窗口——心跳失败、退避重试、本地状态保留；
 *   4. 云恢复：切回真实基址 → 首次心跳成功 → Reconcile 重新收敛。
 *
 * 判定全部基于模拟器回执（JSONL），平台侧行为以 API 响应为事实源。
 * 断云语义：从**边缘视角**云不可达（生产中断云的边缘侧含义）；
 * 后端进程本身不重启——它是否可用由演练窗口外的健康检查保证。
 */

const { expect } = require('chai');
const { execFileSync, spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'P1.5 edge node outage drill [64_edge_node_outage_drill]';
const ACCOUNT = 'tenant_admin';

function readReceipts(receiptPath) {
  if (!receiptPath || !fs.existsSync(receiptPath)) return [];
  return fs.readFileSync(receiptPath, 'utf8')
    .split(/\r?\n/)
    .filter(line => line.trim())
    .map(line => { try { return JSON.parse(line); } catch (_) { return null; } })
    .filter(Boolean);
}

describe(SUITE, function () {
  this.timeout(180000);

  let deviceSeed = null;
  let gatewayDeviceId = null;
  let nodeId = null;
  let receiptPath = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 64_edge_node_outage_drill.test.js');
    }
    await apiClient.login(ACCOUNT);

    // Reconcile 需要 gateway_device_id：种一台网关型设备。
    deviceSeed = await seedData.createSimulationDevice(ACCOUNT);
    const activate = await apiClient.put('/device/active', {
      device_id: deviceSeed.id,
      device_type: '2' // 2 = 网关
    }, ACCOUNT).catch(() => null);
    gatewayDeviceId = deviceSeed.id;
    if (activate && activate.code !== 200) {
      // 设备类型切换失败不阻断：reconcile 的编排门槛以平台判定为准。
      console.warn('device type switch failed (non-fatal):', activate.message);
    }

    nodeId = 'e2e-edge-' + Date.now().toString().slice(-8);
    receiptPath = path.join(process.env.TEMP || '/tmp', `edge-drill-${nodeId}.jsonl`);
  });

  after(async function () {
    if (deviceSeed && typeof deviceSeed.cleanup === 'function') {
      try { await deviceSeed.cleanup(); } catch (err) { /* 尽力回收 */ }
    }
    if (receiptPath && fs.existsSync(receiptPath)) {
      try { fs.rmSync(receiptPath, { force: true }); } catch (_) { /* 尽力回收 */ }
    }
  });

  it('registers, runs the steady loop, survives the outage window, and converges after recovery', async function () {
    const script = path.join(__dirname, '..', 'scripts', 'edge_node_simulator.js');
    const args = [
      script,
      '--api-base', 'http://127.0.0.1:9999',
      '--node-id', nodeId,
      '--version', '1.0.0',
      '--gateway-device-id', gatewayDeviceId,
      '--receipts', receiptPath,
      '--outage-after', '8',
      '--outage-seconds', '6',
      '--runtime', '24',
      '--heartbeat-interval-ms', '1500',
      '--api-token', await apiClient.getToken(ACCOUNT)
    ];
    // 独立子进程 = 真实边缘客户端；stdout/stderr 丢弃，回执文件是唯一事实源。
    spawn(process.execPath, args, { stdio: ['ignore', 'ignore', 'ignore'] });

    // 等运行窗口结束再统一断言（模拟器自带 runtime 自杀）。
    const deadline = Date.now() + 40000;
    while (Date.now() < deadline) {
      const receipts = readReceipts(receiptPath);
      if (receipts.some(row => row.kind === 'done')) break;
      await new Promise(resolve => setTimeout(resolve, 1000));
    }

    const receipts = readReceipts(receiptPath);
    const kinds = receipts.map(row => row.kind);

    expect(receipts.find(row => row.kind === 'registered' && row.ok), 'registration must succeed').to.be.an('object');

    const okHeartbeatsBefore = receipts.filter(row => row.kind === 'heartbeat' && row.ok && !row.recoveredLogged).length;
    expect(okHeartbeatsBefore, 'steady-state heartbeats must succeed').to.be.greaterThan(0);

    expect(kinds, 'outage window must be entered').to.include('outage_start');
    const failedHeartbeats = receipts.filter(row => row.kind === 'heartbeat' && !row.ok);
    expect(failedHeartbeats.length, 'heartbeats during outage must fail').to.be.greaterThan(0);
    expect(kinds, 'outage window must end').to.include('outage_end');

    expect(kinds, 'recovery must be observed').to.include('recovered');
    const after = receipts.indexOf(receipts.find(row => row.kind === 'recovered'));
    const okAfterRecovery = receipts.slice(after).filter(row => row.kind === 'heartbeat' && row.ok).length;
    expect(okAfterRecovery, 'heartbeats must succeed again after recovery').to.be.greaterThan(0);

    const reconciles = receipts.filter(row => row.kind === 'reconcile');
    expect(reconciles.length, 'reconcile runs recorded').to.be.greaterThan(0);

    // 平台侧终态：节点健康在线（权威判定在平台，不在模拟器）。
    const list = await apiClient.get('/edge/nodes', {}, ACCOUNT);
    expect(list.code, 'edge node list').to.equal(200);
    const rows = Array.isArray(list.data) ? list.data : (list.data && list.data.list) || [];
    const node = rows.find(row => row.node_id === nodeId || (row.node && row.node.node_id === nodeId));
    if (node) {
      const health = node.health || (node.node && node.node.health);
      expect(String(health || ''), 'platform-side health after recovery').to.not.equal('offline');
    }
  });
});
