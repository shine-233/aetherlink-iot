/**
 * 文件用途：用于提供API 边界冒烟测试。
 * 核心逻辑：扫过代表性 API 边界和验证分支，记录端点分类、响应契约和已知闭环缺口。
 * 关键注意事项：边界冒烟用于定位覆盖缺口，本身不应被计为完整业务闭环。
 * 重构建议：高价值边界用例应逐步升级为带种子数据、状态校验和负向断言的业务套件。
 */

// @file-boundary-evidence-only: validates API boundary contracts, not seeded business closure.

const { expect } = require('chai');
const {
  ZERO_UUID,
  apiClient,
  rawGet,
  bootstrapApiCoverageContext,
  cleanupApiCoverageContext,
  expectOk,
  expectCode,
  expectRecordNotFound,
  expectSqlRecordNotFound,
  expectDbSqlError,
  expectDbField,
  expectObjectPayload,
  expectNullableObjectPayload,
  expectOkOrCode,
  expectArrayPayloadOrCode,
  expectObjectPayloadOrCode,
  expectPagedPayloadOrCode,
  expectDiagnosticsPayload
} = require('./helpers/api_closure_helpers');

describe('API boundary smoke module [17_api_boundary_smoke]', function () {
  this.timeout(45000);

  let deviceId = null;

  before(async function () {
    ({ deviceId } = await bootstrapApiCoverageContext());
  });

  after(function () {
    cleanupApiCoverageContext();
  });

  describe('root and public system endpoints', function () {
    it('serves the root health endpoint', async function () {
      const resp = await rawGet('/health');
      expect(resp.status).to.equal(200);
      const body = typeof resp.data === 'string' ? JSON.parse(resp.data) : resp.data;
      expect(body).to.be.an('object');
      expect(body.code).to.equal(200);
      expect(body).to.have.property('message').that.is.a('string').and.not.equal('');
    });

    it('serves prometheus metrics and the metrics viewer', async function () {
      const metricsResp = await rawGet('/metrics');
      expect(metricsResp.status).to.equal(200);
      expect(String(metricsResp.data)).to.include('# HELP');
      expect(String(metricsResp.data)).to.match(/go_goroutines|process_cpu_seconds_total|http_/);

      const viewerResp = await rawGet('/metrics-viewer');
      expect(viewerResp.status).to.equal(200);
      expect(String(viewerResp.data)).to.include('<html');
      expect(String(viewerResp.data)).to.include('AetherLink IoT Metrics Dashboard');
    });

    it('routes swagger and static file requests through the configured wildcard handlers', async function () {
      const swaggerResp = await rawGet('/swagger/index.html');
      expect(swaggerResp.status).to.equal(200);
      expect(String(swaggerResp.data)).to.include('Swagger');

      const fileResp = await rawGet('/files/upgradePackage/not-present.bin');
      expect(fileResp.status).to.equal(404);
    });

    it('exercises public websocket-style endpoints and OTA download validation edges', async function () {
      const telemetryWsResp = await rawGet('/api/v1/telemetry/datas/current/ws');
      expect(telemetryWsResp.status).to.equal(400);

      const onlineWsResp = await rawGet('/api/v1/device/online/status/ws');
      expect(onlineWsResp.status).to.equal(400);

      const onlineBatchWsResp = await rawGet('/api/v1/device/online/status/ws/batch');
      expect(onlineBatchWsResp.status).to.equal(400);

      const telemetryKeysWsResp = await rawGet('/api/v1/telemetry/datas/current/keys/ws');
      expect(telemetryKeysWsResp.status).to.equal(400);

      const downloadResp = await rawGet('/api/v1/ota/download/files/upgradePackage/test/missing.bin');
      expect(downloadResp.status).to.equal(200);
      const downloadBody = typeof downloadResp.data === 'string'
        ? JSON.parse(downloadResp.data)
        : downloadResp.data;
      expect(downloadBody).to.be.an('object');
      expect(downloadBody.code).to.equal(100002);
      expect(downloadBody.message).to.equal('请求参数验证失败');
    });
  });

  describe('public auth and plugin validation boundaries', function () {
    it('exposes setup-state probes without authentication', async function () {
      expectOk(await apiClient.getNoAuth('/tenant/has-admin'));
      expectOk(await apiClient.getNoAuth('/tenant/setup-state'));
    });

    it('rejects incomplete public auth mutation payloads with validation errors', async function () {
      expectCode(await apiClient.postNoAuth('/reset/password', { email: 'nobody@example.com' }), 100002, 'Password');
      expectCode(await apiClient.postNoAuth('/tenant/email/register', { email: 'nobody@example.com' }), 100002, 'VerifyCode');
      expectCode(await apiClient.postNoAuth('/tenant/super-admin/init', { email: 'root@example.com' }), 100002, 'Password');
      expectCode(await apiClient.postNoAuth('/tenant/market-register', { email: 'tenant@example.com' }), 100002, 'Password');
    });

    it('rejects incomplete plugin requests at the public plugin boundary', async function () {
      expectCode(await apiClient.postNoAuth('/plugin/heartbeat', {}), 100002, 'ServiceIdentifier');
      expectCode(await apiClient.postNoAuth('/plugin/devices', {}), 100002, 'ServiceIdentifier');
      expectCode(await apiClient.postNoAuth('/plugin/service/access/list', {}), 100002, 'ServiceIdentifier');
      expectCode(await apiClient.postNoAuth('/plugin/service/access', {}), 100002, 'ServiceAccessID');

      const configResp = await apiClient.postNoAuth('/plugin/device/config', {});
      expectCode(configResp, 401, 'missing authentication');
    });

    it('rejects incomplete public gateway registration payloads', async function () {
      expectCode(await apiClient.postNoAuth('/device/gateway-register', {}), 100002, 'GatewayId');
      expectCode(await apiClient.postNoAuth('/device/gateway-sub-register', {}), 100002, 'DeviceId');
    });
  });

  describe('cross-domain invalid-id and validation smoke', function () {
    it('only classifies casbin user list and mutation boundaries', async function () {
      const selectorResp = await apiClient.get('/user/selector', { page: 1, page_size: 10 }, 'super_admin');
      const selectorData = selectorResp.data && (selectorResp.data.list || selectorResp.data);
      const userId = Array.isArray(selectorData) && selectorData.length > 0
        ? (selectorData[0].id || selectorData[0].ID || ZERO_UUID)
        : ZERO_UUID;

      const listResp = await apiClient.get('/casbin/user', { user_id: userId }, 'super_admin');
      expectOk(listResp);

      const createResp = await apiClient.post('/casbin/user', { user_id: userId, roles_ids: [] }, 'super_admin');
      expect(createResp).to.be.an('object');
      expect(createResp.code).to.equal(100002);
      expect(createResp.data).to.be.an('object');
      expect(createResp.data.error).to.include('AddRolesToUser');

      const updateResp = await apiClient.put('/casbin/user', { user_id: userId, roles_ids: [] }, 'super_admin');
      expect(updateResp).to.be.an('object');
      expect(updateResp.code).to.equal(100002);
      expect(updateResp.data).to.be.an('object');
      expect(updateResp.data.error).to.include('AddRolesToUser');

      const deleteResp = await apiClient.delete('/casbin/user/' + userId, {}, 'super_admin');
      expect(deleteResp).to.be.an('object');
      expect(deleteResp.code).to.equal(100002);
      expect(deleteResp.data).to.be.an('object');
      expect(deleteResp.data.error).to.include('RemoveUserAndRole');
    });

    it('only classifies OTA delete and update invalid-id boundaries', async function () {
      expectRecordNotFound(await apiClient.delete('/ota/package/' + ZERO_UUID, {}, 'super_admin'));
      expectRecordNotFound(await apiClient.put('/ota/package', { id: ZERO_UUID }, 'super_admin'));
      expectRecordNotFound(await apiClient.delete('/ota/task/' + ZERO_UUID, {}, 'super_admin'));
    });

    it('only classifies expected-data validation endpoints', async function () {
      expectCode(await apiClient.get('/expected/data/list', { page: 1, page_size: 10 }, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.post('/expected/data', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectSqlRecordNotFound(await apiClient.delete('/expected/data/' + ZERO_UUID, {}, 'tenant_admin'));
    });

    it('only classifies message push config and session endpoints', async function () {
      expectCode(await apiClient.post('/message_push', {}, 'tenant_admin'), 100002, 'PushId');
      expectCode(await apiClient.post('/message_push/logout', {}, 'tenant_admin'), 100002, 'PushId');

      expectCode(
        await apiClient.get('/message_push/config', {}, 'tenant_admin'),
        201001,
        'message push config'
      );

      const superConfigResp = await apiClient.get('/message_push/config', {}, 'super_admin');
      expectObjectPayload(superConfigResp, ['url']);

      expectCode(await apiClient.post('/message_push/config', {}, 'tenant_admin'), 100002, 'Url');
    });

    it('only classifies dashboard-menu invalid id boundaries', async function () {
      const getResp = await apiClient.get('/dashboard-menu/' + ZERO_UUID, {}, 'tenant_admin');
      expectOk(getResp);
      expect(getResp.data).to.equal(null);
      expectCode(await apiClient.put('/dashboard-menu/' + ZERO_UUID, { name: 'coverage' }, 'tenant_admin'), 100002, 'MenuName');
      expectOk(await apiClient.delete('/dashboard-menu/' + ZERO_UUID, {}, 'tenant_admin'));
    });

    it('only classifies openapi invalid update and delete boundaries', async function () {
      expectDbField(await apiClient.put('/open/keys', { id: ZERO_UUID, name: 'coverage' }, 'super_admin'), 'error', 'record not found');
      expectDbField(await apiClient.delete('/open/keys/' + ZERO_UUID, {}, 'super_admin'), 'error', 'record not found');
    });

    it('only classifies authenticated SSE and system-function update boundaries', async function () {
      const headers = await apiClient.authHeaders('super_admin');
      const eventResp = await rawGet('/api/v1/events', {
        headers,
        timeout: 2000,
        responseType: 'stream'
      });
      expect([200, 204]).to.include(eventResp.status);
      if (eventResp.status === 200) {
        expect(String(eventResp.headers['content-type'] || '')).to.include('text/event-stream');
      }

      expectSqlRecordNotFound(await apiClient.put('/sys_function/' + ZERO_UUID, { enable_flag: 'N' }, 'super_admin'));
    });

    it('only classifies selected device read, debug, and onboarding guide boundaries', async function () {
      expect(deviceId, 'API boundary read coverage requires a seeded device id').to.be.a('string').and.not.equal('');
      if (deviceId) {
        expectDiagnosticsPayload(await apiClient.get('/devices/' + deviceId + '/diagnostics', {}, 'tenant_admin'), deviceId);
        const connectionDiagnosticsResp = await apiClient.get(
          '/device/' + deviceId + '/connection/diagnostics',
          { debug_log_limit: 5 },
          'tenant_admin'
        );
        expectOk(connectionDiagnosticsResp);
        expect(connectionDiagnosticsResp.data).to.include({
          device_id: deviceId
        });
        expect(connectionDiagnosticsResp.data).to.have.property('evaluated_at');
        expect(connectionDiagnosticsResp.data).to.have.property('conclusion').that.is.an('object');
        expect(connectionDiagnosticsResp.data.conclusion).to.include.keys(['level', 'code', 'summary', 'next_actions']);
        expect(connectionDiagnosticsResp.data.conclusion.next_actions).to.be.an('array');
        expect(connectionDiagnosticsResp.data).to.have.property('online').that.is.an('object');
        expect(connectionDiagnosticsResp.data.online).to.include.keys(['device_status', 'is_online']);
        expect(connectionDiagnosticsResp.data).to.have.property('debug').that.is.an('object');
        expect(connectionDiagnosticsResp.data.debug).to.include.keys([
          'enabled',
          'remaining_seconds',
          'total',
          'offset',
          'limit',
          'recent_logs'
        ]);
        expect(connectionDiagnosticsResp.data.debug.limit).to.equal(5);
        expect(connectionDiagnosticsResp.data.debug.recent_logs).to.be.an('array');
        expect(connectionDiagnosticsResp.data).to.have.property('diagnostics').that.is.an('object');
        expect(connectionDiagnosticsResp.data.diagnostics.recent_failures).to.be.an('array');
        if (connectionDiagnosticsResp.data.partial_results !== undefined) {
          expect(connectionDiagnosticsResp.data.partial_results).to.be.an('array');
        }

        const connectionGuideResp = await apiClient.get(
          '/device/' + deviceId + '/onboarding/connection-guide',
          { debug_log_limit: 5, command_log_limit: 3 },
          'tenant_admin'
        );
        expectOk(connectionGuideResp);
        expect(connectionGuideResp.data).to.include({
          device_id: deviceId
        });
        expect(connectionGuideResp.data).to.have.property('evaluated_at');
        expect(connectionGuideResp.data).to.have.property('access').that.is.an('object');
        expect(connectionGuideResp.data.access).to.have.property('protocol').that.is.a('string');
        expect(connectionGuideResp.data.access).to.have.property('tls').that.is.an('object');
        expect(connectionGuideResp.data.access.tls).to.include.keys([
          'enabled',
          'certificate_hint',
          'advertised_to_device'
        ]);
        expect(connectionGuideResp.data).to.have.property('readiness').that.is.an('object');
        expect(connectionGuideResp.data.readiness).to.include.keys([
          'level',
          'code',
          'summary',
          'online',
          'ready',
          'next_actions'
        ]);
        expect(connectionGuideResp.data.readiness).to.have.property('ready').that.is.a('boolean');
        expect(connectionGuideResp.data.readiness.next_actions).to.be.an('array');
        expect(connectionGuideResp.data).to.have.property('next_steps').that.is.an('array');
        expect(connectionGuideResp.data.next_steps.map((step) => step.key)).to.include.members([
          'credentials',
          'publish_telemetry',
          'ready_check',
          'control_loop'
        ]);
        connectionGuideResp.data.next_steps.forEach((step) => {
          expect(step).to.have.property('status').that.is.oneOf(['done', 'warning', 'todo']);
        });
        if (connectionGuideResp.data.partial_results !== undefined) {
          expect(connectionGuideResp.data.partial_results).to.be.an('array');
        }

        const invalidDebugLimitResp = await apiClient.get(
          '/device/' + deviceId + '/onboarding/connection-guide',
          { debug_log_limit: 'abc' },
          'tenant_admin'
        );
        expectCode(invalidDebugLimitResp, 100002);
        expect(invalidDebugLimitResp.data).to.equal('debug_log_limit must be an integer');

        const invalidCommandLimitResp = await apiClient.get(
          '/device/' + deviceId + '/onboarding/connection-guide',
          { command_log_limit: 'abc' },
          'tenant_admin'
        );
        expectCode(invalidCommandLimitResp, 100002);
        expect(invalidCommandLimitResp.data).to.equal('command_log_limit must be an integer');

        expectObjectPayload(
          await apiClient.get('/device/map/telemetry/' + deviceId, {}, 'tenant_admin'),
          ['device_id', 'telemetry_data']
        );
        expectObjectPayload(
          await apiClient.post('/device/' + deviceId + '/debug', {}, 'tenant_admin'),
          ['enabled', 'remaining_seconds', 'config']
        );
        expectObjectPayload(
          await apiClient.get('/device/' + deviceId + '/debug/status', {}, 'tenant_admin'),
          ['enabled', 'remaining_seconds', 'config']
        );
        expectObjectPayload(
          await apiClient.get('/device/' + deviceId + '/debug/logs', { page: 1, page_size: 10 }, 'tenant_admin'),
          ['total', 'list']
        );
      }
    });

    it('only classifies selected device permission-sensitive list and metrics boundaries', async function () {
      expect(deviceId, 'API boundary permission coverage requires a seeded device id').to.be.a('string').and.not.equal('');
      if (deviceId) {
        expectPagedPayloadOrCode(await apiClient.get('/device/sub-list/' + deviceId, { page: 1, page_size: 10 }, 'tenant_admin'), 201001, 'permission');
        expectArrayPayloadOrCode(await apiClient.get('/device/metrics/' + deviceId, {}, 'tenant_admin'), 201001, 'permission');
      }

      expectArrayPayloadOrCode(
        await apiClient.get('/device/tenant/list', { page: 1, page_size: 10 }, 'tenant_admin'),
        201001,
        'permission'
      );
      expectOkOrCode(await apiClient.get('/device/list', { page: 1, page_size: 10 }, 'tenant_admin'), 201001, 'permission');
    });

    it('only classifies device mutation and connect validation boundaries', async function () {
      expectCode(await apiClient.get('/device/connect/form', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.get('/device/connect/info', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.post('/device/son/add', {}, 'tenant_admin'), 100002, 'ID');
      expectCode(await apiClient.put('/device/sub-remove', {}, 'tenant_admin'), 100002, 'SubDeviceId');
      expectCode(await apiClient.post('/device/service/access/batch', {}, 'tenant_admin'), 100002, 'ServiceAccessId');
      expectCode(await apiClient.post('/device/update/voucher', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.get('/device/metrics/menu', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.get('/device/metrics/condition/menu', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectArrayPayloadOrCode(await apiClient.get('/device/telemetry/latest', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.put('/device/update/config', { id: deviceId || ZERO_UUID }, 'tenant_admin'), 100002, 'DeviceID');
    });

    it('only classifies telemetry, attribute, and command validation boundaries', async function () {
      expectCode(await apiClient.get('/telemetry/datas/current/keys', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectObjectPayloadOrCode(await apiClient.get('/telemetry/datas/msg/count', {}, 'tenant_admin'), ['msg'], 100002, 'DeviceID');
      expectCode(await apiClient.get('/telemetry/datas/simulation', {}, 'tenant_admin'), 100002, 'DeviceId');
      expectCode(await apiClient.post('/telemetry/datas/simulation', {}, 'tenant_admin'), 100002, 'Command');
      expectCode(await apiClient.post('/telemetry/datas/pub', {}, 'tenant_admin'), 100002, 'DeviceID');

      expectCode(await apiClient.post('/attribute/datas/pub', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.get('/attribute/datas/get', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.get('/attribute/datas/key', {}, 'tenant_admin'), 100002, 'DeviceId');
      expectCode(await apiClient.post('/command/datas/pub', {}, 'tenant_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.post('/command/datas/direct-method', {}, 'tenant_admin'), 100002, 'DeviceID');
    });

    it('only classifies device-config, UI-form, upload, and shared-RDI read boundaries', async function () {
      expectCode(await apiClient.get('/device_config/connect', {}, 'super_admin'), 100002, 'DeviceID');
      expectCode(await apiClient.get('/device_config/voucher_type', {}, 'super_admin'), 100002, 'DeviceType');
      expectCode(await apiClient.get('/device_config/metrics/menu', {}, 'super_admin'), 100002, 'DeviceConfigID');
      expectCode(await apiClient.get('/device_config/metrics/condition/menu', {}, 'super_admin'), 100002, 'DeviceConfigID');
      expectObjectPayload(await apiClient.get('/ui_elements/select/form', {}, 'super_admin'), ['list']);
      expectOk(await apiClient.get('/dict/protocol/service', {}, 'super_admin'));
      expectPagedPayloadOrCode(
        await apiClient.get('/rdi/shared-with-me/devices', { page: 1, page_size: 10 }, 'tenant_admin'),
        201001,
        'permission'
      );

      const uploadResp = await apiClient.post('/file/up', {}, 'super_admin');
      expectCode(uploadResp, 202001);
    });

    it('only classifies template, group, data-script, and alarm invalid-id boundaries', async function () {
      const templateResp = await apiClient.get('/device/template', { page: 1, page_size: 10 }, 'super_admin');
      const templateList = templateResp.data && Array.isArray(templateResp.data.list) ? templateResp.data.list : [];
      const templateId = templateList.length > 0 ? (templateList[0].id || templateList[0].ID || ZERO_UUID) : ZERO_UUID;

      const templateDetailResp = await apiClient.get('/device/template/detail/' + templateId, {}, 'super_admin');
      if (templateId === ZERO_UUID) {
        expectSqlRecordNotFound(templateDetailResp);
      } else {
        expectObjectPayload(templateDetailResp, ['id', 'name', 'tenant_id']);
        expect(templateDetailResp.data.id || templateDetailResp.data.ID).to.equal(templateId);
        expect(templateDetailResp.data.name).to.be.a('string').and.not.equal('');
      }
      expectArrayPayloadOrCode(await apiClient.get('/device/template/chart/select', {}, 'super_admin'), 201001, 'permission');
      expectCode(await apiClient.get('/device/group/relation/list', { page: 1, page_size: 10 }, 'tenant_admin'), 100002, 'GroupId');

      expectDbSqlError(await apiClient.delete('/data_script/' + ZERO_UUID, {}, 'super_admin'), 'data script not found');
      expectCode(await apiClient.put('/data_script', { id: ZERO_UUID }, 'super_admin'), 100002, 'Name');
      expectCode(await apiClient.put('/data_script/enable', { id: ZERO_UUID, enabled: true }, 'super_admin'), 100002, 'EnableFlag');

      expectCode(await apiClient.put('/alarm/info', {}, 'tenant_admin'), 100002, 'Id');
      expectCode(await apiClient.put('/alarm/info/batch', {}, 'tenant_admin'), 100002, 'Id');
      expectCode(await apiClient.put('/alarm/info/history', {}, 'tenant_admin'), 100002, 'AlarmHistoryId');
      const alarmDeviceConfigResp = await apiClient.get('/alarm/info/config/device', { device_id: deviceId || ZERO_UUID }, 'tenant_admin');
      expectOk(alarmDeviceConfigResp);
      expectNullableObjectPayload(alarmDeviceConfigResp.data);
      expectSqlRecordNotFound(await apiClient.put('/alarm/info/history/' + ZERO_UUID + '/acknowledge', {}, 'tenant_admin'));
      expectSqlRecordNotFound(await apiClient.put('/alarm/info/history/' + ZERO_UUID + '/reset', {}, 'tenant_admin'));
      expectSqlRecordNotFound(await apiClient.get('/alarm/info/history/' + ZERO_UUID, {}, 'tenant_admin'));
      expectSqlRecordNotFound(await apiClient.delete('/alarm/info/history/' + ZERO_UUID, {}, 'tenant_admin'));
    });

    it('only classifies user transform and logout boundaries without poisoning later tests', async function () {
      expectCode(await apiClient.post('/user/transform', {}, 'tenant_admin'), 100002, 'BecomeUserID');

      const logoutResp = await apiClient.get('/user/logout', {}, 'tenant_admin');
      expectOk(logoutResp);
      apiClient.clearToken('tenant_admin');
    });
  });
});
