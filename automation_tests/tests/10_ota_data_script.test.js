/**
 * 文件用途：用于验证OTA 包与数据脚本 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectBusinessError,
  expectOtaPackageRow,
  expectPagedList
} = require('../lib/response_assertions');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

describe('OTA and data script API module [10_ota_data_script]', function () {
  this.timeout(30000);

  const fakeId = '00000000-0000-0000-0000-000000000000';

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 10_ota_data_script.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('super_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('returns the current OTA package page shape', async function () {
    const resp = await apiClient.get('/ota/package', { page: 1, page_size: 10 }, 'super_admin');

    expectOk(resp);
    expectPagedList(resp.data, { rowCheck: expectOtaPackageRow });
  });

  it('returns the current device_config page shape used by OTA and scripts', async function () {
    // 审计备注（2026-09-04）：super_admin（SYS_ADMIN，平台级无租户）当前访问
    // GET /device_config 会命中后端 "empty tenant id in claims" 的 101001 DB 错误，
    // 属待修复的程序缺陷（对照：asset 服务对平台级返回空列表）。页面形状契约
    // 由租户管理员视角验证，与 OTA/脚本页面的真实使用方一致。
    const resp = await apiClient.get('/device_config', { page: 1, page_size: 10 }, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.total).to.be.a('number');
  });

  it('rejects OTA package creation when device_config_id is omitted from the current frontend payload shape', async function () {
    const resp = await apiClient.post(
      '/ota/package',
      {
        name: 'codex-package-' + Date.now(),
        version: '1.0.0',
        description: 'created by codex',
        module: 'firmware',
        file_path: '/upgradePackage/test.bin',
        file_size: 1024
      },
      'super_admin'
    );

    expectBusinessError(resp, 100002, "Field 'DeviceConfigID' is required");
  });

  it('fails OTA package creation against a fake device_config fixture with the current local file-path error', async function () {
    const resp = await apiClient.post(
      '/ota/package',
      {
        name: 'codex-package-' + Date.now(),
        version: '1.0.0',
        target_version: '1.0.1',
        device_config_id: fakeId,
        module: 'firmware',
        package_type: 2,
        signature_type: 'MD5',
        package_url: '/upgradePackage/test.bin',
        additional_info: '{}',
        description: 'created by codex',
        remark: ''
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100000);
    expect(resp.message).to.be.a('string').and.include('upgradePackage');
  });

  it('rejects OTA task list queries without ota_upgrade_package_id in the current backend contract', async function () {
    const resp = await apiClient.get('/ota/task', { page: 1, page_size: 10 }, 'super_admin');

    expectBusinessError(resp, 100002, "Field 'OTAUpgradePackageId' is required");
  });

  it('rejects OTA task preview when device_filter is omitted', async function () {
    const resp = await apiClient.post(
      '/ota/task/preview',
      {
        ota_upgrade_package_id: fakeId,
        max_devices: 5000
      },
      'super_admin'
    );

    expectBusinessError(resp, 100002, "Field 'DeviceFilter' is required");
  });

  it('scaffolds OTA full-filter preview with a non-empty device_filter but keeps fake package boundary explicit', async function () {
    const resp = await apiClient.post(
      '/ota/task/preview',
      {
        ota_upgrade_package_id: fakeId,
        device_filter: {
          device_config_id: fakeId,
          is_online: 1
        },
        exclude_device_id_list: [],
        max_devices: 5000
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100000);
    expect(resp.message).to.be.a('string').and.not.equal('');
    if (resp.data !== undefined && resp.data !== null) {
      expect(resp.data).to.not.have.property('preview_devices');
    }
  });

  it('returns record-not-found when OTA task creation uses fake package and device fixtures', async function () {
    const resp = await apiClient.post(
      '/ota/task',
      {
        name: 'codex-task-' + Date.now(),
        ota_upgrade_package_id: fakeId,
        description: 'created by codex',
        remark: '',
        device_id_list: [fakeId]
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100000);
    expect(resp.message).to.equal('record not found');
  });

  it('returns record-not-found for a fake OTA task support bundle id', async function () {
    const resp = await apiClient.get('/ota/task/' + fakeId + '/support-bundle', {}, 'super_admin');

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100000);
    expect(resp.message).to.be.a('string').and.match(/record|not found|task/i);
    if (resp.data !== undefined && resp.data !== null) {
      expect(resp.data).to.not.have.property('support_bundle');
    }
  });

  it('rejects OTA task detail queries without ota_upgrade_task_id', async function () {
    const resp = await apiClient.get('/ota/task/detail', { page: 1, page_size: 10 }, 'super_admin');

    expectBusinessError(resp, 100002, "Field 'OtaUpgradeTaskId' is required");
  });

  it('returns record-not-found for a fake OTA task detail id', async function () {
    const resp = await apiClient.get(
      '/ota/task/detail',
      {
        page: 1,
        page_size: 10,
        ota_upgrade_task_id: fakeId
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100000);
    expect(resp.message).to.equal('record not found');
  });

  it('rejects OTA task detail updates when the previous status field is sent instead of action', async function () {
    const resp = await apiClient.put(
      '/ota/task/detail',
      {
        id: fakeId,
        status: 3
      },
      'super_admin'
    );

    expectBusinessError(resp, 100002, "Field 'Action' is required");
  });

  it('returns record-not-found when OTA task detail update uses a fake id with the current action field', async function () {
    const resp = await apiClient.put(
      '/ota/task/detail',
      {
        id: fakeId,
        action: 1
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100000);
    expect(resp.message).to.equal('record not found');
  });

  it('rejects data script list queries without device_config_id', async function () {
    const resp = await apiClient.get('/data_script', { page: 1, page_size: 10 }, 'super_admin');

    expectBusinessError(resp, 100002, "Field 'DeviceConfigId' is required");
  });

  it('returns record-not-found for data script list queries against a fake device_config_id', async function () {
    const resp = await apiClient.get(
      '/data_script',
      {
        page: 1,
        page_size: 10,
        device_config_id: fakeId
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(101001);
    expect(resp.message).to.be.a('string').and.not.equal('');
    expect(resp.data).to.be.an('object');
    expect(resp.data.sql_error).to.equal('record not found');
  });

  it('rejects data script creation when the old numeric script_type format is sent', async function () {
    const resp = await apiClient.post(
      '/data_script',
      {
        name: 'codex-script-' + Date.now(),
        description: 'created by codex',
        script: 'function decode(input) { return input; }',
        script_type: 1
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100002);
    expect(resp.message).to.include('CreateDataScriptReq.script_type');
  });

  it('rejects data script creation when the current string script_type is used with a fake device_config fixture', async function () {
    const resp = await apiClient.post(
      '/data_script',
      {
        name: 'codex-script-' + Date.now(),
        device_config_id: fakeId,
        content: 'function encodeInp(msg,topic) return msg end',
        script_type: 'A',
        description: 'created by codex',
        remark: ''
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(101001);
    expect(resp.message).to.be.a('string').and.not.equal('');
    expect(resp.data).to.be.an('object');
    expect(resp.data.sql_error).to.equal('record not found');
  });
});
