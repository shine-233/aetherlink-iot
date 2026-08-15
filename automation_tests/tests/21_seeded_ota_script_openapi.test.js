/**
 * 文件用途：用于验证种子 OTA、脚本与 OpenAPI 业务 API 测试。
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
  expectSuccess,
  expectValidationError
} = require('../lib/response_assertions');

const ZERO_UUID = '00000000-0000-0000-0000-000000000000';

function expectOtaSupportBundleShape(bundle, taskId, packageId) {
  expect(bundle).to.be.an('object');
  expect(bundle.task_id).to.equal(taskId);
  expect(bundle.package_id).to.equal(packageId);
  expect(bundle.generated_at).to.be.a('string').and.not.equal('');
  expect(Number.isFinite(Date.parse(bundle.generated_at))).to.equal(true);
  expect(bundle.statistics).to.be.an('array');
  expect(bundle.total_rows).to.be.a('number').and.satisfy(Number.isInteger).and.at.least(0);
  expect(bundle.failed_count).to.be.a('number').and.satisfy(Number.isInteger).and.at.least(0);
  expect(bundle.total_rows).to.be.at.least(bundle.failed_count);
  for (const row of bundle.statistics) {
    expect(row).to.be.an('object');
    expect(row).to.have.property('status');
    expect(row).to.have.property('count');
    expect(Number.isInteger(Number(row.status))).to.equal(true);
    expect(Number.isInteger(Number(row.count))).to.equal(true);
    expect(Number(row.count)).to.be.greaterThan(0);
  }
  const statisticTotal = bundle.statistics.reduce((sum, row) => sum + Number(row.count), 0);
  const failedStatisticTotal = bundle.statistics
    .filter(row => String(row.status) === '5' || String(row.status).toLowerCase() === 'failed')
    .reduce((sum, row) => sum + Number(row.count), 0);
  expect(statisticTotal).to.equal(bundle.total_rows);
  expect(failedStatisticTotal).to.equal(bundle.failed_count);
  expect(bundle.failed_devices).to.be.an('array');
  expect(bundle.failed_devices.length).to.be.at.most(Math.min(bundle.failed_count, 50));
  expect(bundle.failure_groups).to.be.an('array');
  expect(bundle.next_actions).to.be.an('array').and.not.empty;
  expect(bundle.evidence_boundary).to.be.an('array').and.not.empty;
  expect(bundle.evidence_boundary.some(item => String(item).includes('persisted OTA task-detail rows'))).to.equal(true);
  expect(bundle.share_hint).to.be.a('string').and.not.equal('');

  for (const group of bundle.failure_groups) {
    expect(group.reason).to.be.a('string').and.not.equal('');
    expect(group.count).to.be.a('number').and.satisfy(Number.isInteger).and.greaterThan(0);
  }
  expect(bundle.failure_groups.reduce((sum, group) => sum + group.count, 0)).to.equal(bundle.failed_count);

  for (const device of bundle.failed_devices) {
    expect(device.detail_id).to.be.a('string').and.not.equal('');
    expect(device.device_id).to.be.a('string').and.not.equal('');
    expect(device.failure_reason).to.be.a('string');
    if (device.ready_check_url) {
      expect(device.ready_check_url).to.include('tab=ready-check');
      expect(device.ready_check_url).to.include('source=ota');
      expect(device.ready_check_url).to.include('ota_task_id=');
      expect(device.ready_check_url).to.include('ota_detail_id=');
    }
  }
}

describe('Seeded OTA, data script, OpenAPI, and service coverage [21_seeded_ota_script_openapi]', function () {
  this.timeout(45000);
  let tenantId = '';

  before(async function () {
    await apiClient.login('super_admin');
    await apiClient.login('tenant_admin');
    const tenantResp = await apiClient.get('/user/tenant/id', {}, 'tenant_admin');
    if (tenantResp.code === 200) {
      tenantId = typeof tenantResp.data === 'string'
        ? tenantResp.data
        : (tenantResp.data && (tenantResp.data.tenant_id || tenantResp.data.tenantId || tenantResp.data.id || tenantResp.data.ID)) || '';
    }
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('asserts OTA package and task read plus invalid-id boundaries', async function () {
    const packageResp = await apiClient.get('/ota/package', { page: 1, page_size: 10 }, 'super_admin');
    expectSuccess(packageResp);
    expectPagedList(packageResp.data);

    const taskResp = await apiClient.get('/ota/task', { page: 1, page_size: 10 }, 'super_admin');
    expectValidationError(taskResp, 'OTAUpgradePackageId');

    expectBusinessError(await apiClient.delete('/ota/package/' + ZERO_UUID, {}, 'super_admin'), 100000);
    expectBusinessError(await apiClient.delete('/ota/task/' + ZERO_UUID, {}, 'super_admin'), 100000);
  });

  it('returns OTA task support archive with task-level rollout counts and conditional Ready Check handoff fields', async function () {
    const source = await seedData.ensureOtaTaskSupportBundleSource('super_admin');
    expectBlockedOrSeeded(source, 'OTA task support-bundle source');

    const supportResp = await apiClient.get('/ota/task/' + source.taskId + '/support-bundle', {}, 'super_admin');
    expectSuccess(supportResp);
    expectOtaSupportBundleShape(supportResp.data, source.taskId, source.packageId);
  });

  it('asserts data script list and mutation validation branches', async function () {
    const listResp = await apiClient.get('/data_script', { page: 1, page_size: 10 }, 'super_admin');
    expectValidationError(listResp, 'DeviceConfigId');

    expectValidationError(await apiClient.post('/data_script', {}, 'super_admin'), 'Name');
    expectValidationError(await apiClient.put('/data_script', { id: ZERO_UUID }, 'super_admin'), 'Name');
    expectValidationError(await apiClient.put('/data_script/enable', { id: ZERO_UUID }, 'super_admin'), 'EnableFlag');
  });

  it('uses seeded OpenAPI key helper and verifies the created key appears in list results', async function () {
    const seed = await seedData.ensureOpenApiKey('tenant_admin', tenantId);
    try {
      expectBlockedOrSeeded(seed, 'openapi seed');
      const listResp = await apiClient.get('/open/keys', { page: 1, page_size: 10 }, 'tenant_admin');
      expectSuccess(listResp);
      expectPagedList(listResp.data);
      const created = listResp.data.list.find(row => row.id === seed.id || row.ID === seed.id);
      expect(created).to.be.an('object');
    } finally {
      await seed.cleanup();
    }
  });

  it('asserts service plugin and access validation boundaries', async function () {
    expectSuccess(await apiClient.get('/service/list', { page: 1, page_size: 10 }, 'super_admin'));
    expectValidationError(await apiClient.post('/service', {}, 'super_admin'), 'Name');
    expectValidationError(await apiClient.post('/service/access', {}, 'super_admin'), 'Name');
    expectValidationError(await apiClient.get('/service/access/voucher/form', {}, 'super_admin'), 'ServicePluginID');
  });
});
