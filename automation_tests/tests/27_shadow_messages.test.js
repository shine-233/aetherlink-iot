/**
 * 文件用途：设备影子（离线命令缓存）API 证据测试（ROADMAP A3）。
 * 核心逻辑：覆盖离线入队、列表/计数、取消语义、参数边界，以及
 *   「设备离线 → 入队 → MQTT 上行使设备上线 → 后端自动投递 pending」的完整闭环。
 * 关键注意事项：
 *   - 投递闭环依赖真实 broker 与后端上线钩子，broker 不可用时按 runtime-external 跳过；
 *   - TTL 过期由 cron 周期触发，分钟级时序不适合 API 证据层，过期语义由 DAL 单测覆盖；
 *   - 投递成功以影子记录 status=delivered 且 delivered_at 非空为准，不假设设备侧业务已执行。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const { skipIfBlocked } = require('../lib/integration_blocked');
const {
  expectBusinessError,
  expectSuccess,
  expectValidationError
} = require('../lib/response_assertions');

function shadowSetPath(deviceId) {
  return '/device/shadow/' + deviceId;
}

function shadowCancelPath(deviceId, msgId) {
  return '/device/shadow/' + deviceId + '/' + msgId;
}

async function queueShadow(deviceId, overrides = {}, accountKey = 'tenant_admin') {
  const payload = Object.assign({
    message_type: 'command',
    payload: { method: 'set', params: { power: 1 } },
    ttl_seconds: 3600
  }, overrides);
  return apiClient.post(shadowSetPath(deviceId), payload, accountKey);
}

async function listShadows(deviceId, status, accountKey = 'tenant_admin') {
  const query = status ? { status } : {};
  return apiClient.get(shadowSetPath(deviceId), query, accountKey);
}

describe('Device shadow offline command cache [27_shadow_messages]', function () {
  this.timeout(90000);

  before(async function () {
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('rejects invalid shadow message payloads at the binding layer', async function () {
    const seededDevice = await seedData.createSimulationDevice('tenant_admin');
    try {
      expectValidationError(await apiClient.post(shadowSetPath(seededDevice.id), {}, 'tenant_admin'), 'MessageType');

      expectValidationError(
        await queueShadow(seededDevice.id, { ttl_seconds: 5 }),
        'TTLSeconds'
      );

      expectValidationError(
        await queueShadow(seededDevice.id, { message_type: 'broadcast' }),
        'MessageType'
      );
    } finally {
      await seededDevice.cleanup();
    }
  });

  it('queues an offline device command as a pending shadow message with counts', async function () {
    const seededDevice = await seedData.createSimulationDevice('tenant_admin');
    try {
      const setResp = await queueShadow(seededDevice.id);
      expectSuccess(setResp);
      expect(setResp.data).to.be.an('object');
      // 新建设备从未上行过，必须走队列而不是直发。
      expect(setResp.data.direct).to.equal(false);
      expect(setResp.data.message).to.be.an('object');
      expect(setResp.data.message.status).to.equal('pending');
      expect(setResp.data.message.device_id).to.equal(seededDevice.id);

      const pendingResp = await listShadows(seededDevice.id, 'pending');
      expectSuccess(pendingResp);
      const pendingList = pendingResp.data.list || [];
      const matched = pendingList.find(row => row && row.id === setResp.data.message.id);
      expect(matched, 'queued message visible under status=pending').to.be.an('object');
      expect(pendingResp.data.counts).to.be.an('object');
      expect(Number(pendingResp.data.counts.pending || 0)).to.be.at.least(1);
      expect(Number(pendingResp.data.total || 0)).to.be.at.least(1);
    } finally {
      await seededDevice.cleanup();
    }
  });

  it('cancels a pending shadow exactly once', async function () {
    const seededDevice = await seedData.createSimulationDevice('tenant_admin');
    try {
      const setResp = await queueShadow(seededDevice.id);
      expectSuccess(setResp);
      const msgId = setResp.data.message.id;

      expectSuccess(await apiClient.delete(shadowCancelPath(seededDevice.id, msgId), {}, 'tenant_admin'));

      const pendingAfter = await listShadows(seededDevice.id, 'pending');
      expectSuccess(pendingAfter);
      const stillPending = (pendingAfter.data.list || []).find(row => row && row.id === msgId);
      expect(stillPending, 'canceled message must leave pending list').to.equal(undefined);

      // 二次取消：目标不再是 pending，应报业务错误而不是静默成功。
      expectBusinessError(
        await apiClient.delete(shadowCancelPath(seededDevice.id, msgId), {}, 'tenant_admin'),
        100002
      );
    } finally {
      await seededDevice.cleanup();
    }
  });

  it('delivers pending shadows automatically after the device comes online', async function () {
    const mqttAvailable = await seedData.isMqttBrokerAvailable();
    if (!mqttAvailable) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is not available on ' + seedData.mqttEndpointDescription() +
          '; shadow delivery requires a live uplink to trigger the online hook',
        category: 'runtime-external',
        seedable: false
      });
    }

    const seededDevice = await seedData.createSimulationDevice('tenant_admin');
    try {
      const first = await queueShadow(seededDevice.id, {
        payload: { method: 'shadow-e2e', params: { seq: 1 } }
      });
      const second = await queueShadow(seededDevice.id, {
        payload: { method: 'shadow-e2e', params: { seq: 2 } }
      });
      expectSuccess(first);
      expectSuccess(second);
      const queuedIds = [first.data.message.id, second.data.message.id];

      // 设备上线：模拟遥测经 broker 上行，后端 uplink 首条消息路径会把设备置为在线，
      // 并在约 3 秒延迟后投递 pending 影子（与 expected data 相同的窗口）。
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        seededDevice.id,
        { shadow_e2e_online: 1 },
        'tenant_admin'
      );

      const deadline = Date.now() + 45000;
      let statuses = {};
      while (Date.now() < deadline) {
        const allResp = await listShadows(seededDevice.id, '');
        expectSuccess(allResp);
        const rows = allResp.data.list || [];
        statuses = Object.fromEntries(rows.map(row => [row.id, row]));
        const allDelivered = queuedIds.every(id => statuses[id] && statuses[id].status === 'delivered');
        if (allDelivered) {
          queuedIds.forEach(id => {
            expect(statuses[id].delivered_at, 'delivered_at must be recorded').to.be.a('string').and.not.equal('');
          });
          return;
        }
        await new Promise(resolve => setTimeout(resolve, 2000));
      }

      const snapshot = queuedIds.map(id => (statuses[id] ? statuses[id].status : 'missing')).join(',');
      throw new Error('pending shadows were not delivered within 45s after online; final statuses=' + snapshot);
    } finally {
      await seededDevice.cleanup();
    }
  });
});
