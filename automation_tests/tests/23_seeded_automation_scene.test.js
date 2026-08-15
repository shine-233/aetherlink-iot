/**
 * 文件用途：用于验证种子自动化场景业务 API 测试。
 * 核心逻辑：使用确定性本地夹具执行 API 场景，断言响应、状态变化、负向分支和清理结果。
 * 关键注意事项：只有在本地账号、种子数据和清理步骤都成功时，才可作为对应流程的业务闭环证据。
 * 重构建议：继续把数据准备、断言 oracle 和清理逻辑拆清楚，便于补充故障注入或变异验证。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const {
  expectBlockedOrSeeded,
  expectBusinessError,
  expectPagedList,
  expectPagedListContains,
  expectPermissionDenied,
  expectSuccess,
  expectValidationError
} = require('../lib/response_assertions');

const ZERO_UUID = '00000000-0000-0000-0000-000000000000';

describe('Seeded automation scene business coverage [23_seeded_automation_scene]', function () {
  this.timeout(45000);

  before(async function () {
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('creates a seeded scene and verifies detail, list, and activation surfaces', async function () {
    const seed = await seedData.ensureScene('tenant_admin');
    try {
      expectBlockedOrSeeded(seed, 'scene seed');

      const detailResp = await apiClient.get('/scene/detail/' + seed.id, {}, 'tenant_admin');
      expectSuccess(detailResp);
      expect(detailResp.data).to.be.an('object');
      expect(detailResp.data.info).to.be.an('object');
      expect(detailResp.data.info.id || detailResp.data.info.ID).to.equal(seed.id);
      expect(detailResp.data.info.name || detailResp.data.info.Name).to.equal(seed.name);
      expect(detailResp.data.actions).to.be.an('array');
      expect(detailResp.data.actions.length).to.be.at.least(1);
      expect(detailResp.data.actions[0]).to.include.keys('action_type', 'action_target');

      const listResp = await apiClient.get('/scene', { page: 1, page_size: 20 }, 'tenant_admin');
      expectSuccess(listResp);
      expectPagedListContains(listResp.data, row => (row.id || row.ID) === seed.id, 'seeded scene row');

      expectSuccess(await apiClient.post('/scene/active/' + seed.id, {}, 'tenant_admin'));
    } finally {
      await seed.cleanup();
    }
  });

  it('asserts scene and automation negative branches with explicit product errors', async function () {
    expectPermissionDenied(await apiClient.getNoAuth('/scene', { page: 1, page_size: 10 }));
    expectBusinessError(await apiClient.get('/scene/detail/' + ZERO_UUID, {}, 'tenant_admin'), 101001);
    expectBusinessError(await apiClient.delete('/scene/' + ZERO_UUID, {}, 'tenant_admin'), 101001);
    expectValidationError(await apiClient.get('/scene/log', { page: 1, page_size: 10 }, 'tenant_admin'), 'ID');

    const automationListResp = await apiClient.get('/scene_automations/list', { page: 1, page_size: 10 }, 'tenant_admin');
    expectSuccess(automationListResp);
    expectPagedList(automationListResp.data);
    expectValidationError(await apiClient.post('/scene_automations', {}, 'tenant_admin'), 'Name');
    expectBusinessError(await apiClient.post('/scene_automations/switch/' + ZERO_UUID, {}, 'tenant_admin'), 101001);
    expectValidationError(await apiClient.get('/scene_automations/log', { page: 1, page_size: 10 }, 'tenant_admin'), 'SceneAutomationId');
  });
});
