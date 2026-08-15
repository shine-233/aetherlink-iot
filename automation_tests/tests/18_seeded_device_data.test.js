/**
 * 文件用途：用于验证种子设备数据业务 API 测试。
 * 核心逻辑：使用确定性本地夹具执行 API 场景，断言响应、状态变化、负向分支和清理结果。
 * 关键注意事项：只有在本地账号、种子数据和清理步骤都成功时，才可作为对应流程的业务闭环证据。
 * 重构建议：继续把数据准备、断言 oracle 和清理逻辑拆清楚，便于补充故障注入或变异验证。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const {
  expectArray,
  expectBusinessError,
  expectNullableObject,
  expectPagedList,
  expectSuccess,
  expectValidationError
} = require('../lib/response_assertions');

describe('Seeded device and telemetry business coverage [18_seeded_device_data]', function () {
  this.timeout(45000);

  before(async function () {
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  function expectOnboardingGuideEvidence(data, deviceId) {
    expect(data).to.include({ device_id: deviceId });
    expect(data).to.have.property('evaluated_at').that.is.a('string').and.not.equal('');
    expect(Date.parse(data.evaluated_at)).to.satisfy(Number.isFinite);
    expect(data).to.have.property('access').that.is.an('object');
    expect(data.access).to.have.property('protocol').that.is.a('string').and.not.equal('');
    expect(data.access).to.have.property('tls').that.is.an('object');
    expect(data.access.tls).to.include.keys([
      'enabled',
      'certificate_hint',
      'advertised_to_device'
    ]);
    expect(data).to.have.property('readiness').that.is.an('object');
    expect(data.readiness).to.include.keys([
      'level',
      'code',
      'summary',
      'online',
      'ready',
      'next_actions'
    ]);
    expect(data.readiness.next_actions).to.be.an('array');
    expect(data).to.have.property('next_steps').that.is.an('array');
    expect(data.next_steps.map(step => step.key)).to.include.members([
      'credentials',
      'publish_telemetry',
      'ready_check',
      'control_loop'
    ]);
    data.next_steps.forEach(step => {
      expect(step).to.include.keys(['key', 'title', 'description', 'status']);
      expect(step.status).to.be.oneOf(['done', 'warning', 'todo']);
    });
    if (data.partial_results !== undefined) {
      expect(data.partial_results).to.be.an('array');
    }
  }

  function expectConnectionDiagnosticsEvidence(data, deviceId, debugLimit) {
    expect(data).to.include({ device_id: deviceId });
    expect(data).to.have.property('evaluated_at').that.is.a('string').and.not.equal('');
    expect(Date.parse(data.evaluated_at)).to.satisfy(Number.isFinite);
    expect(data).to.have.property('conclusion').that.is.an('object');
    expect(data.conclusion).to.include.keys(['level', 'code', 'summary', 'next_actions']);
    expect(data.conclusion.next_actions).to.be.an('array');
    expect(data).to.have.property('online').that.is.an('object');
    expect(data.online).to.include.keys(['device_status', 'is_online']);
    expect(data).to.have.property('debug').that.is.an('object');
    expect(data.debug).to.include.keys([
      'enabled',
      'remaining_seconds',
      'total',
      'offset',
      'limit',
      'recent_logs'
    ]);
    expect(data.debug.limit).to.equal(debugLimit);
    expect(data.debug.recent_logs).to.be.an('array');
    expect(data).to.have.property('diagnostics').that.is.an('object');
    expect(data.diagnostics).to.have.property('recent_failures').that.is.an('array');
    if (data.partial_results !== undefined) {
      expect(data.partial_results).to.be.an('array');
    }
  }

  it('creates or locates a deterministic device and validates device read surfaces', async function () {
    await seedData.withSeededDevice(async seed => {
      expect(seed.id).to.be.a('string').and.not.equal('');

      const detailResp = await apiClient.get('/device/detail/' + seed.id, {}, 'tenant_admin');
      expectSuccess(detailResp);
      expect(detailResp.data).to.be.an('object');
      expect(detailResp.data.id || detailResp.data.ID || detailResp.data.device_id).to.equal(seed.id);

      const currentResp = await apiClient.get('/device/telemetry/latest', { device_id: seed.id }, 'tenant_admin');
      if (currentResp.code === 200) {
        if (Array.isArray(currentResp.data)) {
          currentResp.data.forEach(row => expect(row).to.be.an('object'));
        } else {
          expectNullableObject(currentResp.data);
        }
      } else {
        expectValidationError(currentResp, 'DeviceID');
      }
    });
  });

  it('sets desired twin state and reads platform-visible convergence evidence', async function () {
    await seedData.withSeededDevice(async seed => {
      const twinKey = 'automation_twin_' + Date.now();
      let desiredId = null;

      try {
        const desiredResp = await apiClient.put('/device/twin/' + seed.id + '/desired', {
          source: 'telemetry',
          key: twinKey,
          desired: { target: 'ready-check', run: twinKey }
        }, 'tenant_admin');
        expectSuccess(desiredResp);
        expect(desiredResp.data).to.be.an('object');
        desiredId = desiredResp.data.id || desiredResp.data.ID || '';
        expect(desiredId).to.be.a('string').and.not.equal('');
        expect(desiredResp.data).to.include({
          device_id: seed.id,
          send_type: 'telemetry',
          status: 'pending'
        });

        const twinResp = await apiClient.get('/device/twin/' + seed.id, {}, 'tenant_admin');
        expectSuccess(twinResp);
        expect(twinResp.data).to.be.an('object');
        expect(twinResp.data).to.have.property('summary').that.is.an('object');
        expect(twinResp.data.summary).to.include.keys([
          'desiredCount',
          'reportedCount',
          'matchedCount',
          'deltaCount',
          'unavailableCount',
          'staleDesiredCount',
          'convergenceStatus',
          'nextAction',
          'evidenceBoundary'
        ]);
        expect(twinResp.data.summary.evidenceBoundary).to.equal('platform_visible_evidence_only');
        expect(twinResp.data.summary.convergenceStatus).to.be.oneOf([
          'ready',
          'waiting_reported',
          'needs_review',
          'expired_desired',
          'no_desired'
        ]);
        expect(twinResp.data.summary.nextAction).to.be.oneOf([
          'safe_to_continue_after_review',
          'wait_for_reported_state',
          'compare_delta_before_device_action',
          'review_expired_desired_state',
          'create_desired_state'
        ]);
        expect(twinResp.data).to.have.property('rows').that.is.an('array');
        const desiredRow = twinResp.data.rows.find(row => row.key === twinKey);
        expect(desiredRow).to.be.an('object');
        expect(desiredRow).to.include({
          key: twinKey,
          label: twinKey,
          source: 'telemetry',
          comparable: true,
          status: 'pending'
        });
        expect(desiredRow).to.have.property('desired').that.deep.equals({
          target: 'ready-check',
          run: twinKey
        });
        expect(desiredRow).to.have.property('matched').that.is.a('boolean');
      } finally {
        if (desiredId) {
          await apiClient.delete('/expected/data/' + desiredId, {}, 'tenant_admin');
        }
      }
    });
  });

  it('walks seeded device onboarding guide through access, readiness, diagnostics, and next-step evidence', async function () {
    await seedData.withSeededDevice(async seed => {
      expect(seed.id).to.be.a('string').and.not.equal('');

      const guideResp = await apiClient.get(
        '/device/' + seed.id + '/onboarding/connection-guide',
        { debug_log_limit: 5, command_log_limit: 3 },
        'tenant_admin'
      );
      expectSuccess(guideResp);
      expectOnboardingGuideEvidence(guideResp.data, seed.id);

      const diagnosticsResp = await apiClient.get(
        '/device/' + seed.id + '/connection/diagnostics',
        { debug_log_limit: 5 },
        'tenant_admin'
      );
      expectSuccess(diagnosticsResp);
      expectConnectionDiagnosticsEvidence(diagnosticsResp.data, seed.id, 5);
    });
  });

  it('asserts telemetry, attribute, event, and command failure branches with explicit product errors', async function () {
    expectValidationError(await apiClient.get('/telemetry/datas/current/keys', {}, 'tenant_admin'), 'DeviceID');
    expectValidationError(await apiClient.get('/telemetry/datas/history', {}, 'tenant_admin'), 'DeviceID');
    expectValidationError(await apiClient.post('/telemetry/datas/pub', {}, 'tenant_admin'), 'DeviceID');
    expectValidationError(await apiClient.post('/attribute/datas/pub', {}, 'tenant_admin'), 'DeviceID');
    expectValidationError(await apiClient.get('/attribute/datas/get', {}, 'tenant_admin'), 'DeviceID');
    // The event data controller is mounted at the catalogued collection route.
    expectValidationError(
      await apiClient.get('/event/datas', { page: 1, page_size: 10 }, 'tenant_admin'),
      'DeviceId'
    );
    expectValidationError(await apiClient.post('/command/datas/pub', {}, 'tenant_admin'), 'DeviceID');
  });

  it('returns paged device and model lists with stable shapes', async function () {
    await seedData.withSeededDevice(async seed => {
      const deviceResp = await apiClient.get('/device', { page: 1, page_size: 10 }, 'tenant_admin');
      expectSuccess(deviceResp);
      expectPagedList(deviceResp.data);
      const seededDeviceRow = deviceResp.data.list.find(row => (row.id || row.ID || row.device_id) === seed.id);
      expect(seededDeviceRow, 'seeded device must appear in the paged device list').to.be.an('object');

      const selectorResp = await apiClient.get('/device/selector', { page: 1, page_size: 10 }, 'tenant_admin');
      expectSuccess(selectorResp);
      expectPagedList(selectorResp.data);
      const seededSelectorRow = selectorResp.data.list.find(row => row.device_id === seed.id);
      expect(seededSelectorRow, 'seeded device must appear in the device selector list').to.be.an('object');
    });

    const metricMenuResp = await apiClient.get('/device/metrics/menu', {}, 'tenant_admin');
    expectBusinessError(metricMenuResp, 100002, 'DeviceID');

    const conditionMenuResp = await apiClient.get('/device/metrics/condition/menu', {}, 'tenant_admin');
    expectBusinessError(conditionMenuResp, 100002, 'DeviceID');
  });

  it('validates topic mapping list/create/update boundaries for MQTT pipeline setup', async function () {
    expectValidationError(await apiClient.post('/device/topic-mappings', {}, 'tenant_admin'));

    const listResp = await apiClient.get('/device/topic-mappings', { page: 1, page_size: 10 }, 'tenant_admin');
    if (listResp.code === 200) {
      expectArray(listResp.data && (listResp.data.list || listResp.data));
    } else {
      expectValidationError(listResp, 'DeviceConfigID');
    }

    const updateResp = await apiClient.put('/device/topic-mappings/00000000-0000-0000-0000-000000000000', {}, 'tenant_admin');
    expectBusinessError(updateResp, 100002, 'invalid id');
  });
});
