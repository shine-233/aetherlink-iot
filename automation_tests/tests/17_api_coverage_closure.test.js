/**
 * 文件用途：用于提供API 覆盖率闭环边界测试。
 * 核心逻辑：扫过代表性 API 边界和验证分支，记录端点分类、响应契约和已知闭环缺口。
 * 关键注意事项：边界冒烟用于定位覆盖缺口，本身不应被计为完整业务闭环。
 * 重构建议：高价值边界用例应逐步升级为带种子数据、状态校验和负向断言的业务套件。
 */

const { expect } = require('chai');
const {
  ZERO_UUID,
  apiClient,
  bootstrapApiCoverageContext,
  cleanupApiCoverageContext,
  expectCode,
  expectSqlRecordNotFound,
  expectValidationFieldOneOf,
  expectPagedPayload
} = require('./helpers/api_closure_helpers');

// @file-boundary-evidence-only: validates API boundary contracts, not seeded business closure.
describe('API domain boundary evidence [17_api_coverage_closure]', function () {
  this.timeout(45000);

  before(async function () {
    await bootstrapApiCoverageContext();
  });

  after(function () {
    cleanupApiCoverageContext();
  });

  describe('device model validation boundaries', function () {
    const deviceModelKinds = [
      'telemetry',
      'attributes',
      'events',
      'commands',
      'custom/commands',
      'custom/control'
    ];

    it('rejects model list requests without a device template id', async function () {
      expectCode(await apiClient.get('/device/model/source/at/list', {}, 'super_admin'), 100002, 'ID');

      for (const kind of deviceModelKinds) {
        const resp = await apiClient.get('/device/model/' + kind, { page: 1, page_size: 10 }, 'super_admin');
        expectCode(resp, 100002, 'DeviceTemplateId');
      }
    });

    it('rejects incomplete model create and update payloads', async function () {
      for (const kind of deviceModelKinds) {
        const createResp = await apiClient.post('/device/model/' + kind, { name: 'coverage_probe' }, 'super_admin');
        expectCode(createResp, 100002, 'DeviceTemplateId');

        const updateResp = await apiClient.put('/device/model/' + kind, { id: ZERO_UUID, name: 'coverage_probe' }, 'super_admin');
        if (kind === 'custom/control') {
          expectSqlRecordNotFound(updateResp);
        } else if (kind === 'custom/commands') {
          expectValidationFieldOneOf(updateResp, ['ButtomName']);
        } else {
          expectValidationFieldOneOf(updateResp, ['DataIdentifier']);
        }
      }
    });

    it('rejects delete and custom command detail requests for non-existent model ids', async function () {
      for (const kind of deviceModelKinds) {
        const resp = await apiClient.delete('/device/model/' + kind + '/' + ZERO_UUID, {}, 'super_admin');
        expectSqlRecordNotFound(resp);
      }

      const detailResp = await apiClient.get('/device/model/custom/commands/' + ZERO_UUID, {}, 'super_admin');
      expectCode(detailResp, 100000, 'record not found');
    });
  });

  describe('service plugin and access endpoints', function () {
    it('returns service and access lists', async function () {
      expectPagedPayload(await apiClient.get('/service/list', { page: 1, page_size: 10 }, 'super_admin'));
      expectPagedPayload(await apiClient.get('/service/access/list', { page: 1, page_size: 10 }, 'super_admin'));

      const selectResp = await apiClient.get('/service/plugin/select', {}, 'super_admin');
      expectCode(selectResp, 200);
      expect(selectResp.data).to.be.an('object');
      expect(Array.isArray(selectResp.data), 'service plugin select payload must not be an array').to.equal(false);
      expect(selectResp.data.protocol).to.be.an('array');
      expect(selectResp.data.protocol).to.deep.include({ service_identifier: 'MQTT', name: 'MQTT' });
      expect(selectResp.data.service).to.be.an('array');
      selectResp.data.service.forEach(item => {
        expect(item).to.include.keys('service_identifier', 'name', 'service_plugin_id');
      });
    });

    it('rejects incomplete service and service access mutations', async function () {
      expectCode(await apiClient.post('/service', {}, 'super_admin'), 100002, 'Name');
      expectCode(await apiClient.put('/service', { id: ZERO_UUID }, 'super_admin'), 101001);
      expectSqlRecordNotFound(await apiClient.get('/service/detail/' + ZERO_UUID, {}, 'super_admin'));
      expectCode(await apiClient.delete('/service/' + ZERO_UUID, {}, 'super_admin'), 200);

      expectCode(await apiClient.get('/service/plugin/info', {}, 'super_admin'), 100002, 'ServiceIdentifier');
      expectCode(await apiClient.post('/service/access', {}, 'super_admin'), 100002, 'Name');
      expectSqlRecordNotFound(await apiClient.put('/service/access', { id: ZERO_UUID }, 'super_admin'));
      expectSqlRecordNotFound(await apiClient.delete('/service/access/' + ZERO_UUID, {}, 'super_admin'));
      expectCode(await apiClient.get('/service/access/voucher/form', {}, 'super_admin'), 100002, 'ServicePluginID');
      expectCode(await apiClient.get('/service/access/device/list', {}, 'super_admin'), 100002, 'Voucher');
    });
  });

  describe('scene and automation boundary endpoints', function () {
    it('returns scene and automation lists', async function () {
      expectPagedPayload(await apiClient.get('/scene', { page: 1, page_size: 10 }, 'tenant_admin'));
      expectPagedPayload(await apiClient.get('/scene_automations/list', { page: 1, page_size: 10 }, 'tenant_admin'));
    });

    it('rejects incomplete scene mutations and invalid scene ids', async function () {
      expectCode(await apiClient.post('/scene', {}, 'tenant_admin'), 100002, 'Name');
      expectCode(await apiClient.put('/scene', { id: ZERO_UUID }, 'tenant_admin'), 100002, 'Name');
      expectSqlRecordNotFound(await apiClient.get('/scene/detail/' + ZERO_UUID, {}, 'tenant_admin'));
      expectSqlRecordNotFound(await apiClient.delete('/scene/' + ZERO_UUID, {}, 'tenant_admin'));
      expectSqlRecordNotFound(await apiClient.post('/scene/active/' + ZERO_UUID, {}, 'tenant_admin'));
      expectCode(await apiClient.get('/scene/log', { page: 1, page_size: 10 }, 'tenant_admin'), 100002, 'ID');
    });

    it('rejects incomplete automation mutations and invalid automation ids', async function () {
      expectCode(await apiClient.post('/scene_automations', {}, 'tenant_admin'), 100002, 'Name');
      expectCode(await apiClient.put('/scene_automations', { id: ZERO_UUID }, 'tenant_admin'), 100002, 'Name');
      expectSqlRecordNotFound(await apiClient.get('/scene_automations/detail/' + ZERO_UUID, {}, 'tenant_admin'));
      expectSqlRecordNotFound(await apiClient.delete('/scene_automations/' + ZERO_UUID, {}, 'tenant_admin'));
      expectSqlRecordNotFound(await apiClient.post('/scene_automations/switch/' + ZERO_UUID, {}, 'tenant_admin'));
      expectCode(await apiClient.get('/scene_automations/log', { page: 1, page_size: 10 }, 'tenant_admin'), 100002, 'SceneAutomationId');

      const alarmResp = await apiClient.get('/scene_automations/alarm', {}, 'tenant_admin');
      expectCode(alarmResp, 100002, 'Page');
    });
  });
});
