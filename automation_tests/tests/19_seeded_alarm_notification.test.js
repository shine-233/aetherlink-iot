/**
 * 文件用途：用于验证种子告警通知业务 API 测试。
 * 核心逻辑：使用确定性本地夹具执行 API 场景，断言响应、状态变化、负向分支和清理结果。
 * 关键注意事项：只有在本地账号、种子数据和清理步骤都成功时，才可作为对应流程的业务闭环证据。
 * 重构建议：继续把数据准备、断言 oracle 和清理逻辑拆清楚，便于补充故障注入或变异验证。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const {
  expectBusinessError,
  expectBlockedOrSeeded,
  expectNullableObject,
  expectPagedList,
  expectSuccess,
  expectValidationError
} = require('../lib/response_assertions');

const ZERO_UUID = '00000000-0000-0000-0000-000000000000';

function expectBatchActionFailure(resp, action) {
  expectSuccess(resp);
  expect(resp.data).to.be.an('object');
  expect(resp.data).to.include({
    action,
    success_count: 0,
    failure_count: 1
  });
  expect(resp.data.results).to.be.an('array').with.lengthOf(1);

  const [result] = resp.data.results;
  expect(result).to.include({
    id: ZERO_UUID,
    ok: false
  });
  expect(result.error).to.be.a('string').and.not.equal('');
  expect(result).to.not.have.property('history');
}

function expectBatchActionSuccess(resp, action, id, status, note) {
  expectSuccess(resp);
  expect(resp.data).to.be.an('object');
  expect(resp.data).to.include({
    action,
    success_count: 1,
    failure_count: 0
  });
  expect(resp.data.results).to.be.an('array').with.lengthOf(1);

  const [result] = resp.data.results;
  expect(result).to.include({
    id,
    ok: true
  });
  expect(result.history).to.be.an('object');
  expect(result.history).to.include({
    id,
    alarm_status: status,
    action_note: note
  });
  return result.history;
}

function parseRemark(raw) {
  if (!raw) return {};
  if (typeof raw === 'object') return raw;
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function expectHistoryIdentity(row, seed) {
  expect(row).to.be.an('object');
  expect(seedData.pickAlarmHistoryId(row)).to.equal(seed.id);
  expect(row.alarm_config_id || row.AlarmConfigID).to.equal(seed.alarmConfigId);
  expect(row.scene_automation_id || row.SceneAutomationID || row.scene_id).to.equal(seed.sceneId);
}

describe('Seeded alarm and notification business coverage [19_seeded_alarm_notification]', function () {
  this.timeout(45000);

  before(async function () {
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('validates alarm history acknowledgement and reset negative branches', async function () {
    expectBusinessError(
      await apiClient.put('/alarm/info/history/' + ZERO_UUID + '/acknowledge', {}, 'tenant_admin'),
      101001
    );
    expectBusinessError(
      await apiClient.put('/alarm/info/history/' + ZERO_UUID + '/reset', {}, 'tenant_admin'),
      101001
    );
    expectBusinessError(await apiClient.get('/alarm/info/history/' + ZERO_UUID, {}, 'tenant_admin'), 101001);
    expectBusinessError(await apiClient.delete('/alarm/info/history/' + ZERO_UUID, {}, 'tenant_admin'), 101001);
  });

  it('reports per-row failures for alarm history batch-action boundaries', async function () {
    const note = 'automation batch boundary probe';
    for (const action of ['acknowledge', 'reset']) {
      const resp = await apiClient.put(
        '/alarm/info/history/batch-action',
        {
          ids: [ZERO_UUID],
          action,
          note
        },
        'tenant_admin'
      );

      expectBatchActionFailure(resp, action);
    }
  });

  it('batch-acknowledges and batch-resets a seeded alarm history with detail readback', async function () {
    const seed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    try {
      expectBlockedOrSeeded(seed, 'scene alarm history seed');
      expectHistoryIdentity(seed.row, seed);

      const acknowledgeNote = 'batch acknowledge seeded alarm closure';
      const ackResp = await apiClient.put(
        '/alarm/info/history/batch-action',
        {
          ids: [seed.id],
          action: 'acknowledge',
          note: acknowledgeNote
        },
        'tenant_admin'
      );
      const ackHistory = expectBatchActionSuccess(ackResp, 'acknowledge', seed.id, 'H', acknowledgeNote);
      expect(ackHistory.acknowledged_by).to.be.a('string').and.not.equal('');
      expect(ackHistory.acknowledged_at).to.be.a('string').and.not.equal('');

      const ackDetailResp = await apiClient.get('/alarm/info/history/' + seed.id, {}, 'tenant_admin');
      expectSuccess(ackDetailResp);
      expectHistoryIdentity(ackDetailResp.data, seed);
      const ackRemark = parseRemark(ackDetailResp.data.remark);
      expect(ackRemark.acknowledged).to.equal(true);
      expect(ackRemark.acknowledge_note).to.equal(acknowledgeNote);
      expect(ackRemark.acknowledged_by).to.equal(ackHistory.acknowledged_by);
      expect(ackRemark.acknowledged_at).to.equal(ackHistory.acknowledged_at);

      const resetNote = 'batch reset seeded alarm closure';
      const resetResp = await apiClient.put(
        '/alarm/info/history/batch-action',
        {
          ids: [seed.id],
          action: 'reset',
          note: resetNote
        },
        'tenant_admin'
      );
      const resetHistory = expectBatchActionSuccess(resetResp, 'reset', seed.id, 'N', resetNote);
      expect(resetHistory.reset_by).to.be.a('string').and.not.equal('');
      expect(resetHistory.reset_at).to.be.a('string').and.not.equal('');

      const resetDetailResp = await apiClient.get('/alarm/info/history/' + seed.id, {}, 'tenant_admin');
      expectSuccess(resetDetailResp);
      expectHistoryIdentity(resetDetailResp.data, seed);
      expect(resetDetailResp.data.alarm_status).to.equal('N');
      const resetRemark = parseRemark(resetDetailResp.data.remark);
      expect(resetRemark.acknowledged).to.equal(true);
      expect(resetRemark.reset).to.equal(true);
      expect(resetRemark.reset_note).to.equal(resetNote);
      expect(resetRemark.reset_by).to.equal(resetHistory.reset_by);
      expect(resetRemark.reset_at).to.equal(resetHistory.reset_at);
    } finally {
      await seed.cleanup();
    }
  });

  it('activates a seeded alarm scene and verifies acknowledge/reset history readback', async function () {
    const seed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    try {
      expectBlockedOrSeeded(seed, 'scene alarm history seed');
      expectHistoryIdentity(seed.row, seed);
      expect(seed.row.alarm_status).to.equal('H');

      const ackResp = await apiClient.put('/alarm/info/history/' + seed.id + '/acknowledge', {}, 'tenant_admin');
      expectSuccess(ackResp);
      expect(ackResp.data).to.include({
        id: seed.id,
        alarm_status: 'H'
      });
      expect(ackResp.data.acknowledged_by).to.be.a('string').and.not.equal('');
      expect(ackResp.data.acknowledged_at).to.be.a('string').and.not.equal('');

      const ackDetailResp = await apiClient.get('/alarm/info/history/' + seed.id, {}, 'tenant_admin');
      expectSuccess(ackDetailResp);
      expectHistoryIdentity(ackDetailResp.data, seed);
      const ackRemark = parseRemark(ackDetailResp.data.remark);
      expect(ackRemark.acknowledged).to.equal(true);
      expect(ackRemark.acknowledged_by).to.equal(ackResp.data.acknowledged_by);
      expect(ackRemark.acknowledged_at).to.equal(ackResp.data.acknowledged_at);

      const resetResp = await apiClient.put('/alarm/info/history/' + seed.id + '/reset', {}, 'tenant_admin');
      expectSuccess(resetResp);
      expect(resetResp.data).to.include({
        id: seed.id,
        alarm_status: 'N'
      });
      expect(resetResp.data.reset_by).to.be.a('string').and.not.equal('');
      expect(resetResp.data.reset_at).to.be.a('string').and.not.equal('');

      const resetDetailResp = await apiClient.get('/alarm/info/history/' + seed.id, {}, 'tenant_admin');
      expectSuccess(resetDetailResp);
      expectHistoryIdentity(resetDetailResp.data, seed);
      expect(resetDetailResp.data.alarm_status).to.equal('N');
      const resetRemark = parseRemark(resetDetailResp.data.remark);
      expect(resetRemark.acknowledged).to.equal(true);
      expect(resetRemark.reset).to.equal(true);
      expect(resetRemark.reset_by).to.equal(resetResp.data.reset_by);
      expect(resetRemark.reset_at).to.equal(resetResp.data.reset_at);
    } finally {
      await seed.cleanup();
    }
  });

  it('asserts alarm list and device alarm config response shapes', async function () {
    const listResp = await apiClient.get('/alarm/info', { page: 1, page_size: 10 }, 'tenant_admin');
    expectSuccess(listResp);
    expectPagedList(listResp.data);

    const deviceConfigResp = await apiClient.get('/alarm/info/config/device', { device_id: ZERO_UUID }, 'tenant_admin');
    if (deviceConfigResp.code === 200) {
      expectNullableObject(deviceConfigResp.data);
    } else {
      expectBusinessError(deviceConfigResp, 100000);
    }
  });

  it('covers notification group and record list/read plus create validation boundaries', async function () {
    const groupListResp = await apiClient.get('/notification_group/list', { page: 1, page_size: 10 }, 'tenant_admin');
    if (groupListResp.code === 200) {
      expectPagedList(groupListResp.data);
    } else {
      expectBusinessError(groupListResp, 201001);
    }

    const historyResp = await apiClient.get('/notification_history/list', { page: 1, page_size: 10 }, 'tenant_admin');
    if (historyResp.code === 200) {
      expectPagedList(historyResp.data);
    } else {
      expectBusinessError(historyResp, 201001);
    }

    expectValidationError(await apiClient.post('/notification_group', {}, 'tenant_admin'));
  });

  it('uses seed helper for notification groups and verifies the created row', async function () {
    const seed = await seedData.ensureNotificationGroup('tenant_admin');
    try {
      expectBlockedOrSeeded(seed, 'notification group seed');
      const detailResp = await apiClient.get('/notification_group/' + seed.id, {}, 'tenant_admin');
      expectSuccess(detailResp);
      expect(detailResp.data).to.be.an('object');
      expect(detailResp.data.id).to.equal(seed.id);
      expect(detailResp.data.name).to.equal(seed.row.name);
    } finally {
      await seed.cleanup();
    }
  });
});
