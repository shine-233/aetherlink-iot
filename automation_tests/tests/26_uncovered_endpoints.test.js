/**
 * Real HTTP coverage for endpoints that previously only appeared in the
 * endpoint catalog.  Every case below makes a request to the live backend and
 * asserts either a business payload or an explicit validation/runtime result.
 * Boundary results are intentionally classified as non-business evidence in
 * test_metadata.js; an HTTP response alone must not be promoted to closure.
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const {
  expectBusinessError,
  expectPagedList,
  expectSuccess
} = require('../lib/response_assertions');

const INVALID_UUID = '00000000-0000-0000-0000-000000000000';

function expectRecordNotFound(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(100000);
  expect(resp.message).to.match(/record|not found/i);
}

function expectDeadLetterStatusResult(resp, expected) {
  expect(resp).to.be.an('object');
  expect(resp.message).to.be.a('string').and.not.empty;
  if (expected === 'telemetry-record-not-found') {
    // The legacy telemetry dead-letter lookup currently surfaces GORM's
    // missing-row result as a structured database error.  Keep that existing
    // API contract explicit, including the requested id, instead of accepting
    // an arbitrary object payload.
    expect(resp.code).to.equal(101001);
    expect(resp.data).to.include.keys('error', 'id');
    expect(resp.data.error).to.match(/record not found/i);
    expect(resp.data.id).to.equal(INVALID_UUID);
    return;
  }

  if (expected === 'uplink-status-conflict') {
    expect(resp.code).to.equal(201002);
    expect(resp.message).to.match(/status conflict/i);
    expect(resp).to.not.have.property('data');
    return;
  }

  throw new Error('unknown dead-letter status expectation: ' + expected);
}

describe('Previously uncovered API endpoints [26_uncovered_endpoints]', function () {
  this.timeout(30000);

  let deviceSeed = null;
  let deviceId = '';
  let deviceConfigSeed = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 26_uncovered_endpoints.test.js');
    }

    await apiClient.login('tenant_admin');
    deviceSeed = await seedData.ensureRdiDevice('tenant_admin');
    deviceId = deviceSeed.id;
    expect(deviceId).to.be.a('string').and.not.empty;
    deviceConfigSeed = await seedData.ensureDeviceConfig('tenant_admin');
    expect(deviceConfigSeed.id).to.be.a('string').and.not.empty;
  });

  after(async function () {
    try {
      if (deviceConfigSeed && deviceConfigSeed.cleanup) {
        await deviceConfigSeed.cleanup();
      }
      if (deviceSeed && deviceSeed.cleanup) {
        await deviceSeed.cleanup();
      }
    } finally {
      apiClient.clearAllTokens();
    }
  });

  it('returns structured root and API deployment health reports', async function () {
    const readyResp = await apiClient.getRootNoAuth('/ready');
    expect([200, 503]).to.include(readyResp.httpStatus);
    expect(readyResp.data).to.be.an('object');
    expect(readyResp.data).to.include.keys('service', 'status', 'version', 'checks', 'guidance');
    expect(['ok', 'down']).to.include(readyResp.data.status);
    expect(readyResp.data.checks).to.be.an('object');
    expect(readyResp.data.checks.database).to.include.keys('ok', 'required');
    expect(readyResp.data.checks.redis).to.include.keys('ok', 'required');
    expect(readyResp.data.checks.mqtt).to.include.keys('ok', 'required');

    const rootResp = await apiClient.getRootNoAuth('/deployment/health');
    expect([200, 503]).to.include(rootResp.httpStatus);
    expect(rootResp.data).to.be.an('object');
    expect(rootResp.data).to.include.keys('service', 'status', 'version', 'checks', 'guidance');
    expect(rootResp.data.checks).to.be.an('object');
    expect(rootResp.data.checks.database).to.include.keys('ok', 'required');
    expect(rootResp.data.checks.mqtt).to.include.keys('ok', 'required');

    const apiResp = await apiClient.getNoAuth('/deployment/health');
    expect(apiResp).to.be.an('object');
    // Deployment health intentionally returns the raw report from both the
    // root and /api/v1 route; it is not wrapped in the normal {code,data}
    // envelope used by authenticated business APIs.
    expect(apiResp).to.include.keys(
      'service',
      'status',
      'version',
      'timestamp',
      'checks',
      'guidance'
    );
    expect(['ok', 'down']).to.include(apiResp.status);
    expect(apiResp.checks).to.be.an('object');
    expect(apiResp.checks.database).to.include.keys('ok', 'required');
    expect(apiResp.checks.mqtt).to.include.keys('ok', 'required');
  });

  it('rejects a password reset-link request without an email', async function () {
    const resp = await apiClient.postNoAuth('/reset/password/link', {});
    expectBusinessError(resp, 100002, "Field 'Email' is required");
  });

  it('keeps OTA governance preview record-not-found behavior explicit', async function () {
    const resp = await apiClient.get(
      '/ota/task/' + INVALID_UUID + '/governance-preview',
      {},
      'tenant_admin'
    );
    expectRecordNotFound(resp);
  });

  it('returns a tenant-scoped twin drift index with stable counters', async function () {
    const resp = await apiClient.get('/device/twin-drift', { page: 1, page_size: 20 }, 'tenant_admin');
    expectSuccess(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data).to.include.keys(
      'entries',
      'total_devices',
      'drift_devices',
      'waiting_devices',
      'expired_devices',
      'ready_devices',
      'no_desired_devices',
      'evidence_boundary'
    );
    expect(resp.data.entries).to.be.an('array');
    expect(resp.data.total_devices).to.be.a('number');
    expect(resp.data.evidence_boundary).to.equal('platform_visible_evidence_only');
    resp.data.entries.forEach(entry => {
      expect(entry).to.include.keys(
        'device_id',
        'convergence_status',
        'next_action',
        'delta_count',
        'severity',
        'summary'
      );
    });
  });

  it('exercises the real MQTT debug session lifecycle or reports its explicit runtime boundary', async function () {
    const createResp = await apiClient.post('/device/' + deviceId + '/mqtt-debug/session', {}, 'tenant_admin');
    const runtimeUnavailable = () => {
      expect(createResp.code).to.equal(100000);
      expect(createResp.message).to.match(/mqtt debug runtime is unavailable/i);
    };

    if (createResp.code === 100000 && /mqtt debug runtime is unavailable/i.test(createResp.message || '')) {
      runtimeUnavailable();
      for (const operation of [
        () => apiClient.get('/device/' + deviceId + '/mqtt-debug/session/missing-session', {}, 'tenant_admin'),
        () => apiClient.post(
          '/device/' + deviceId + '/mqtt-debug/session/missing-session/command',
          { action: 'subscribe', topic: 'devices/status/' + deviceId },
          'tenant_admin'
        ),
        () => apiClient.delete('/device/' + deviceId + '/mqtt-debug/session/missing-session', {}, 'tenant_admin')
      ]) {
        const response = await operation();
        expect(response.code).to.equal(100000);
        expect(response.message).to.match(/mqtt debug runtime is unavailable/i);
      }
      return;
    }

    expectSuccess(createResp);
    expect(createResp.data).to.be.an('object');
    expect(createResp.data).to.include.keys(
      'session_id',
      'device_id',
      'connected',
      'created_at',
      'expires_at',
      'subscriptions',
      'messages',
      'last_sequence'
    );
    expect(createResp.data.device_id).to.equal(deviceId);
    expect(createResp.data.session_id).to.be.a('string').and.not.empty;
    expect(createResp.data.connected).to.equal(true);
    expect(Date.parse(createResp.data.created_at)).to.be.finite;
    expect(Date.parse(createResp.data.expires_at)).to.be.finite;

    const sessionId = createResp.data.session_id;
    let closed = false;
    try {
      const readResp = await apiClient.get(
        '/device/' + deviceId + '/mqtt-debug/session/' + sessionId,
        { limit: 20 },
        'tenant_admin'
      );
      expectSuccess(readResp);
      expect(readResp.data).to.include.keys('session_id', 'device_id', 'subscriptions', 'messages');
      expect(readResp.data.session_id).to.equal(sessionId);
      expect(readResp.data.device_id).to.equal(deviceId);

      const topic = 'devices/status/' + deviceId;
      const commandResp = await apiClient.post(
        '/device/' + deviceId + '/mqtt-debug/session/' + sessionId + '/command',
        { action: 'subscribe', topic },
        'tenant_admin'
      );
      expectSuccess(commandResp);
      expect(commandResp.data.session_id).to.equal(sessionId);
      expect(commandResp.data.subscriptions).to.include(topic);
      expect(commandResp.data.subscription_details).to.be.an('array');

      const deleteResp = await apiClient.delete(
        '/device/' + deviceId + '/mqtt-debug/session/' + sessionId,
        {},
        'tenant_admin'
      );
      expectSuccess(deleteResp);
      closed = true;
    } finally {
      if (!closed) {
        await apiClient.delete(
          '/device/' + deviceId + '/mqtt-debug/session/' + sessionId,
          {},
          'tenant_admin'
        );
      }
    }
  });

  it('returns device status history and a real topic-mapping dry-run result', async function () {
    const historyResp = await apiClient.get(
      '/device/status/history',
      { device_id: deviceId, page: 1, page_size: 20 },
      'tenant_admin'
    );
    expectSuccess(historyResp);
    expectPagedList(historyResp.data, {
      rowCheck: row => {
        expect(row).to.include.keys('device_id', 'status', 'change_time');
        expect(Date.parse(row.change_time)).to.be.finite;
      }
    });

    const dryRunResp = await apiClient.post(
      '/device/topic-mappings/dry-run',
      {
        device_config_id: deviceConfigSeed.id,
        direction: 'up',
        source_topic: 'devices/+/telemetry',
        target_topic: 'tenant/{device_id}/telemetry',
        test_topic: 'devices/example-device/telemetry'
      },
      'tenant_admin'
    );
    expectSuccess(dryRunResp);
    expect(dryRunResp.data).to.include.keys(
      'matched',
      'direction',
      'source_topic',
      'target_topic',
      'test_topic',
      'resolved_topic',
      'diagnostics',
      'next_steps'
    );
    expect(dryRunResp.data.matched).to.equal(true);
    expect(dryRunResp.data.diagnostics).to.be.an('array').and.not.empty;
  });

  it('rejects dashboard-menu batch requests without dashboard ids', async function () {
    const resp = await apiClient.post('/dashboard-menu/batch', {}, 'tenant_admin');
    expectBusinessError(resp, 100002, "Field 'DashboardIDs' is required");
  });

  it('returns telemetry detail/statistic contracts and stable dead-letter operator responses', async function () {
    const detailResp = await apiClient.get(
      '/telemetry/datas/current/detail/' + deviceId,
      {},
      'tenant_admin'
    );
    expect(detailResp).to.be.an('object');
    if (detailResp.code === 200) {
      expect(detailResp.data).to.include.keys('device_id', 'key', 'ts', 'value');
      expect(detailResp.data.device_id).to.equal(deviceId);
    } else {
      expectRecordNotFound(detailResp);
    }

    const now = Date.now();
    const statisticResp = await apiClient.get(
      '/telemetry/datas/statistic',
      {
        device_id: deviceId,
        key: 'temperature_1',
        time_range: 'last_1h',
        aggregate_window: '1m',
        aggregate_function: 'avg',
        start_time: now - 3600000,
        end_time: now
      },
      'tenant_admin'
    );
    expectSuccess(statisticResp);
    expect(statisticResp.data).to.be.an('array');
    // The live statistic endpoint returns chart points, not the legacy
    // {key,time,value} DTO.  Keep the actual contract observable: x is the
    // millisecond timestamp, y is the numeric aggregate, and x2 is optional
    // for bucket end times.
    statisticResp.data.forEach(point => {
      expect(point).to.be.an('object');
      expect(point).to.include.keys('x', 'y');
      expect(point.x).to.be.a('number');
      expect(point.y).to.be.a('number');
      if (Object.prototype.hasOwnProperty.call(point, 'x2')) {
        expect(point.x2).to.be.a('number');
      }
    });

    const deadListResp = await apiClient.get(
      '/telemetry/datas/dead-letters',
      { page: 1, page_size: 20 },
      'tenant_admin'
    );
    expectSuccess(deadListResp);
    expectPagedList(deadListResp.data);

    const deadDrainResp = await apiClient.post(
      '/telemetry/datas/dead-letters/drain',
      { limit: 10 },
      'tenant_admin'
    );
    expectSuccess(deadDrainResp);
    expect(deadDrainResp.data).to.include.keys('total_ready', 'attempted', 'replayed', 'failed', 'items');
    expect(deadDrainResp.data.items).to.be.an('array');

    const deadStatusResp = await apiClient.patch(
      '/telemetry/datas/dead-letters/' + INVALID_UUID + '/status',
      { action: 'resolve' },
      'tenant_admin'
    );
    expectDeadLetterStatusResult(deadStatusResp, 'telemetry-record-not-found');
  });

  it('returns uplink dead-letter list/drain and validates a missing status row', async function () {
    const listResp = await apiClient.get(
      '/telemetry/datas/uplink-dead-letters',
      { page: 1, page_size: 20 },
      'tenant_admin'
    );
    expectSuccess(listResp);
    expectPagedList(listResp.data);

    const drainResp = await apiClient.post(
      '/telemetry/datas/uplink-dead-letters/drain',
      { limit: 10 },
      'tenant_admin'
    );
    expectSuccess(drainResp);
    expect(drainResp.data).to.include.keys('total_ready', 'attempted', 'replayed', 'failed', 'items');
    expect(drainResp.data.items).to.be.an('array');

    const statusResp = await apiClient.patch(
      '/telemetry/datas/uplink-dead-letters/' + INVALID_UUID + '/status',
      { action: 'resolve', expected_status: 'pending' },
      'tenant_admin'
    );
    expectDeadLetterStatusResult(statusResp, 'uplink-status-conflict');
  });

  it('keeps simulation-send and command-job-retry validation/record boundaries explicit', async function () {
    const simulationResp = await apiClient.post('/telemetry/datas/simulation/send', {}, 'tenant_admin');
    expectBusinessError(simulationResp, 100002, "Field 'DeviceID' is required");

    const retryResp = await apiClient.post(
      '/command/datas/jobs/' + INVALID_UUID + '/retry',
      {},
      'tenant_admin'
    );
    expectRecordNotFound(retryResp);
  });

  it('creates, validates, updates, lists, and deletes a payload schema through HTTP', async function () {
    const schemaName = 'automation-schema-' + Date.now();
    const fields = [{ name: 'temperature', type: 'number', required: true, min: -20, max: 80 }];
    const createResp = await apiClient.post(
      '/payload-schema',
      { name: schemaName, description: 'automation endpoint coverage', strict: true, fields },
      'tenant_admin'
    );
    expectSuccess(createResp);
    expect(createResp.data).to.include.keys('id', 'name', 'strict', 'fields');
    expect(createResp.data.name).to.equal(schemaName);
    const schemaId = createResp.data.id;
    expect(schemaId).to.be.a('string').and.not.empty;

    try {
      const listResp = await apiClient.get('/payload-schema', {}, 'tenant_admin');
      expectSuccess(listResp);
      expect(listResp.data.list).to.be.an('array');
      expect(listResp.data.list.some(row => row.id === schemaId && row.name === schemaName)).to.equal(true);

      const validResp = await apiClient.post(
        '/payload-schema/validate',
        { schema_name: schemaName, strict: true, fields, sample_payload: JSON.stringify({ temperature: 22 }) },
        'tenant_admin'
      );
      expectSuccess(validResp);
      expect(validResp.data.valid).to.equal(true);
      expect(validResp.data.is_simulation).to.equal(true);
      expect(validResp.data.errors).to.deep.equal([]);

      const invalidResp = await apiClient.post(
        '/payload-schema/validate',
        { schema_name: schemaName, strict: true, fields, sample_payload: JSON.stringify({ temperature: 99 }) },
        'tenant_admin'
      );
      expectSuccess(invalidResp);
      expect(invalidResp.data.valid).to.equal(false);
      expect(invalidResp.data.errors).to.be.an('array').and.not.empty;

      const updateResp = await apiClient.put(
        '/payload-schema/' + schemaId,
        { name: schemaName + '-updated', description: 'updated', strict: false, fields },
        'tenant_admin'
      );
      expectSuccess(updateResp);
      expect(updateResp.data.id).to.equal(schemaId);
      expect(updateResp.data.name).to.equal(schemaName + '-updated');
      expect(updateResp.data.strict).to.equal(false);
    } finally {
      const deleteResp = await apiClient.delete('/payload-schema/' + schemaId, {}, 'tenant_admin');
      expectSuccess(deleteResp);
    }
  });

  it('returns a structured scene dry-run result without executing actions', async function () {
    const resp = await apiClient.post('/scene/dry-run', {}, 'tenant_admin');
    expectSuccess(resp);
    expect(resp.data).to.include.keys(
      'supported',
      'valid',
      'can_save',
      'dry_run',
      'warnings',
      'errors',
      'blocking_errors',
      'execution_trace'
    );
    expect(resp.data.supported).to.equal(true);
    expect(resp.data.valid).to.equal(false);
    expect(resp.data.execution_trace).to.include.keys('steps', 'step_count', 'is_simulation');
    expect(resp.data.execution_trace.is_simulation).to.equal(false);
  });

  it('returns an empty RDI history page and explicit share-revocation not-found results', async function () {
    const now = Date.now();
    const historyResp = await apiClient.get(
      '/rdi/devices/' + deviceId + '/history',
      {
        key: 'temperature_1',
        start_time: now - 3600000,
        end_time: now,
        page: 1,
        page_size: 20
      },
      'tenant_admin'
    );
    expectSuccess(historyResp);
    expectPagedList(historyResp.data);

    const tokenResp = await apiClient.delete(
      '/rdi/devices/' + deviceId + '/share-tokens/not-a-real-token',
      {},
      'tenant_admin'
    );
    expect(tokenResp.code).to.equal(100404);
    expect(tokenResp.message).to.match(/share token not found/i);

    const recipientResp = await apiClient.delete(
      '/rdi/devices/' + deviceId + '/share-recipients/' + INVALID_UUID,
      {},
      'tenant_admin'
    );
    expect(recipientResp.code).to.equal(100404);
    expect(recipientResp.message).to.match(/share recipient not found/i);
  });

  it('publishes a native board and serves it through the unauthenticated share endpoint', async function () {
    const name = 'automation-native-share-' + Date.now();
    let boardId = '';
    try {
      const createResp = await apiClient.post(
        '/board',
        {
          name,
          home_flag: 'N',
          menu_flag: 'N',
          vis_type: 'native'
        },
        'tenant_admin'
      );
      expectSuccess(createResp);
      expect(createResp.data).to.include.keys('id', 'name', 'vis_type', 'published');
      expect(createResp.data.name).to.equal(name);
      expect(createResp.data.vis_type).to.equal('native');
      boardId = createResp.data.id;

      const publishResp = await apiClient.post('/board/' + boardId + '/publish', {}, 'tenant_admin');
      expectSuccess(publishResp);
      expect(publishResp.data).to.include.keys('id', 'published', 'share_token', 'vis_type');
      expect(publishResp.data.id).to.equal(boardId);
      expect(publishResp.data.published).to.equal(true);
      expect(publishResp.data.vis_type).to.equal('native');
      expect(publishResp.data.share_token).to.be.a('string').and.not.empty;

      const sharedResp = await apiClient.getNoAuth(
        '/board/shared/' + encodeURIComponent(publishResp.data.share_token)
      );
      expectSuccess(sharedResp);
      expect(sharedResp.data).to.include.keys('id', 'name', 'vis_type', 'published', 'share_token');
      expect(sharedResp.data.id).to.equal(boardId);
      expect(sharedResp.data.name).to.equal(name);
      expect(sharedResp.data.vis_type).to.equal('native');
      expect(sharedResp.data.published).to.equal(true);
      expect(sharedResp.data.share_token).to.equal(publishResp.data.share_token);

      const invalidTokenResp = await apiClient.getNoAuth('/board/shared/not-a-real-share-token');
      expect(invalidTokenResp.code).to.equal(100404);
      expect(invalidTokenResp.message).to.match(/dashboard|not found/i);
    } finally {
      if (boardId) {
        await apiClient.delete('/board/' + boardId, {}, 'tenant_admin');
      }
    }
  });

  it('requires and honors an explicit tenant context for super-admin native boards', async function () {
    await apiClient.login('super_admin');
    const usersResp = await apiClient.get('/user', { page: 1, page_size: 1000 }, 'super_admin');
    expectSuccess(usersResp);
    const tenantAdmin = (usersResp.data.list || []).find(row => row.authority === 'TENANT_ADMIN' && row.tenant_id);
    expect(tenantAdmin, 'super-admin must be able to select a real tenant admin').to.be.an('object');
    const tenantId = tenantAdmin.tenant_id;

    const missingContextResp = await apiClient.post(
      '/board',
      { name: 'automation-super-admin-missing-context-' + Date.now(), home_flag: 'N', vis_type: 'native' },
      'super_admin'
    );
    expect(missingContextResp.code).to.equal(100002);
    expect(missingContextResp.message).to.match(/tenant context is required/i);

    let boardId = '';
    try {
      const createResp = await apiClient.post(
        '/board',
        {
          name: 'automation-super-admin-board-' + Date.now(),
          home_flag: 'N',
          vis_type: 'native',
          tenant_id: tenantId
        },
        'super_admin'
      );
      expectSuccess(createResp);
      expect(createResp.data.tenant_id).to.equal(tenantId);
      boardId = createResp.data.id;

      const listResp = await apiClient.get(
        '/board',
        { page: 1, page_size: 1000, tenant_id: tenantId, vis_type: 'native' },
        'super_admin'
      );
      expectSuccess(listResp);
      expect(listResp.data.list.some(row => row.id === boardId && row.tenant_id === tenantId)).to.equal(true);
    } finally {
      if (boardId) await apiClient.delete('/board/' + boardId, {}, 'super_admin');
    }
  });
});
