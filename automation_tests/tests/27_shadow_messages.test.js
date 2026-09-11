/**
 * 文件用途：设备影子（离线命令缓存）API 证据测试（ROADMAP A3）。
 * 核心逻辑：覆盖离线入队、列表/计数、取消语义、参数边界，以及
 *   「设备离线 → 入队 → MQTT 上行使设备上线 → 后端自动投递 pending」的完整闭环。
 * 关键注意事项：
 *   - 投递闭环依赖真实 broker 与后端上线钩子，broker 不可用时按 runtime-external 跳过；
 *   - TTL 过期由 cron 周期触发，分钟级时序不适合 API 证据层，过期语义由 DAL 单测覆盖；
 *   - P0.2 ACK 闭环：投递成功只等于 status=sent（已下发待设备确认），绝不等于 delivered；
 *     delivered 必须由设备 ACK 触发并写入 ack_at，不得把"发出去"当成"设备收到了"。
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

function shadowAckPath(deviceId, msgId) {
  return '/device/shadow/' + deviceId + '/' + msgId + '/ack';
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

  it('rejects acknowledging a shadow message that is not ackable', async function () {
    const seededDevice = await seedData.createSimulationDevice('tenant_admin');
    try {
      const setResp = await queueShadow(seededDevice.id);
      expectSuccess(setResp);
      const msgId = setResp.data.message.id;

      // 先取消使其进入终态 canceled。
      expectSuccess(await apiClient.delete(shadowCancelPath(seededDevice.id, msgId), {}, 'tenant_admin'));

      // 终态行不可被确认：必须报错，不得静默成功或把历史改写成已送达。
      expectBusinessError(
        await apiClient.post(shadowAckPath(seededDevice.id, msgId), {}, 'tenant_admin'),
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

      // 第一步：上线应使消息变为 sent（已下发待 ACK）。
      // 旧断言在这里直接要求 delivered，等于把"已下发"当成"设备已确认"，是虚假成功。
      const deadline = Date.now() + 45000;
      let statuses = {};
      while (Date.now() < deadline) {
        const allResp = await listShadows(seededDevice.id, '');
        expectSuccess(allResp);
        const rows = allResp.data.list || [];
        statuses = Object.fromEntries(rows.map(row => [row.id, row]));
        if (queuedIds.every(id => statuses[id] && statuses[id].status === 'sent')) break;
        await new Promise(resolve => setTimeout(resolve, 2000));
      }
      queuedIds.forEach(id => {
        expect(statuses[id] && statuses[id].status, 'shadow must be sent after device online')
          .to.equal('sent');
      });

      // 第二步：设备确认。只有 ACK 才允许变成 delivered 并写入 ack_at。
      for (const id of queuedIds) {
        expectSuccess(await apiClient.post(shadowAckPath(seededDevice.id, id), {}, 'tenant_admin'));
      }

      const ackDeadline = Date.now() + 15000;
      let acked = {};
      while (Date.now() < ackDeadline) {
        const allResp = await listShadows(seededDevice.id, '');
        expectSuccess(allResp);
        acked = Object.fromEntries((allResp.data.list || []).map(row => [row.id, row]));
        if (queuedIds.every(id => acked[id] && acked[id].status === 'delivered')) break;
        await new Promise(resolve => setTimeout(resolve, 1000));
      }
      queuedIds.forEach(id => {
        expect(acked[id] && acked[id].status, 'ACK must move sent -> delivered').to.equal('delivered');
        expect(acked[id].ack_at, 'ack_at must be recorded on ACK').to.be.a('string').and.not.equal('');
      });
    } finally {
      await seededDevice.cleanup();
    }
  });
});
