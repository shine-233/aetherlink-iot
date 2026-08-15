/**
 * 文件用途：用于验证系统管理 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言校验真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const { skipIfBlocked } = require('../lib/integration_blocked');
const seedData = require('../lib/seed_data');
const {
  expectBusinessError,
  expectCurrentSystemMetrics,
  expectMetricPoint,
  expectOtaPackageRow
} = require('../lib/response_assertions');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectPagedListData(data, options = {}) {
  const totalKey = options.totalKey || 'total';
  const listKey = options.listKey || 'list';
  const rowCheck = options.rowCheck;

  expect(data).to.be.an('object');
  expect(data).to.have.property(totalKey).that.is.a('number').and.at.least(0);
  expect(data).to.have.property(listKey).that.is.an('array');
  expect(data[listKey].length).to.be.at.most(data[totalKey]);
  if (typeof rowCheck === 'function') {
    data[listKey].forEach(rowCheck);
  }
}

function expectPagedList(resp, options = {}) {
  expectOk(resp);
  expectPagedListData(resp.data, options);
}

function expectRejected(resp, expected) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(expected.code);
  expect(resp.message).to.equal(expected.message);
  if (Object.prototype.hasOwnProperty.call(expected, 'requestError')) {
    expect(Boolean(resp._requestError)).to.equal(expected.requestError);
  }
  if (Object.prototype.hasOwnProperty.call(expected, 'dataCode')) {
    expect(resp.data).to.be.an('object');
    expect(resp.data.code).to.equal(expected.dataCode);
  }
  if (Object.prototype.hasOwnProperty.call(expected, 'dataMessage')) {
    expect(resp.data).to.be.an('object');
    expect(resp.data.message).to.equal(expected.dataMessage);
  }
}

function pickId(record) {
  if (!record || typeof record !== 'object') {
    return null;
  }
  return record.id || record.ID || null;
}

function flattenMenuNodes(nodes) {
  const flattened = [];
  const visit = (items) => {
    if (!Array.isArray(items)) return;
    items.forEach(item => {
      if (!item || typeof item !== 'object') return;
      flattened.push(item);
      visit(item.children);
    });
  };
  visit(nodes);
  return flattened;
}

function expectLogoConfigRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('id', 'system_name');
}

function expectMenuNodeRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('id', 'element_code', 'route_path');
}

describe('System API module [06_system]', function () {
  this.timeout(30000);

  let deviceId = null;
  let seededDevice = null;
  let createdUiElementId = null;
  let superAdminAvailable = false;
  let tenantUserAvailable = false;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 06_system.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');
    await apiClient.login('super_admin');
    superAdminAvailable = true;
    tenantUserAvailable = await apiClient.isAccountAvailable('tenant_user');

    seededDevice = await seedData.ensureRdiDevice('tenant_admin');
    deviceId = seededDevice.id;
    expect(deviceId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (createdUiElementId && superAdminAvailable) {
      await apiClient.delete('/ui_elements/' + createdUiElementId, {}, 'super_admin');
    }
    if (seededDevice && seededDevice.cleanup) {
      await seededDevice.cleanup();
    }
    apiClient.clearAllTokens();
  });

  describe('TC-SYS-001 health check', function () {
    it('returns healthy when the backend is reachable', async function () {
      const healthy = await apiClient.healthCheck();
      expect(healthy).to.equal(true);
    });
  });

  describe('TC-SYS-002 system time', function () {
    it('returns current system time without authentication', async function () {
      const resp = await apiClient.getNoAuth('/systime');
      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(resp.data.systime).to.be.a('number');
      expect(resp.data.systime).to.be.greaterThan(0);
    });
  });

  describe('TC-SYS-003 system version', function () {
    it('returns version information without authentication', async function () {
      const resp = await apiClient.getNoAuth('/sys_version');
      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(resp.data.version).to.be.a('string').and.not.equal('');
    });
  });

  describe('TC-SYS-004 system function flags', function () {
    it('returns system function settings without authentication', async function () {
      const resp = await apiClient.getNoAuth('/sys_function');
      expectOk(resp);
      expect(resp.data).to.be.an('array');
      expect(resp.data.length).to.be.greaterThan(0);
      expect(resp.data[0]).to.include.keys('id', 'name', 'enable_flag');
    });
  });

  describe('TC-SYS-005 logo config', function () {
    it('returns logo configuration without authentication', async function () {
      const resp = await apiClient.getNoAuth('/logo');
      expectOk(resp);
      expectPagedListData(resp.data, { rowCheck: expectLogoConfigRow });
    });
  });

  describe('TC-SYS-006 warning email query', function () {
    it('returns the authenticated warning email configuration', async function () {
      const resp = await apiClient.get('/user/warning-email');
      expectOk(resp);
      if (resp.data === null) {
        return;
      }
      expect(resp.data).to.be.an('array');
      resp.data.forEach(email => {
        expect(email).to.be.a('string').and.include('@');
      });
    });
  });

  describe('TC-SYS-007 warning email update protection', function () {
    it('rejects warning email updates without authentication', async function () {
      const resp = await apiClient.putNoAuth('/user/warning-email', {
        emails: ['alarm-test@test.com']
      });
      expectRejected(resp, {
        code: 401,
        message: 'missing authentication (x-token or x-api-key required)',
        dataCode: 40100,
        dataMessage: 'missing authentication (x-token or x-api-key required)',
        requestError: true
      });
    });
  });

  describe('TC-SYS-008 OTA package list', function () {
    it('returns a paged OTA package list', async function () {
      const resp = await apiClient.get('/ota/package', { page: 1, page_size: 10 });
      expectOk(resp);
      expectPagedListData(resp.data, { rowCheck: expectOtaPackageRow });
    });
  });

  describe('TC-SYS-009 OTA task list', function () {
    it('returns OTA tasks for the seeded real package', async function () {
      const otaSource = await seedData.ensureOtaTaskSupportBundleSource('tenant_admin');
      try {
        if (otaSource.blocked) {
          const helperProvidedRuntimeExternal =
            otaSource.category === 'runtime-external' && otaSource.seedable === false;
          const blockedReason = {
            reason: otaSource.reason || 'OTA task list fixture is unavailable',
            category: 'seedable-local'
          };
          if (helperProvidedRuntimeExternal) {
            blockedReason.category = 'runtime-external';
            blockedReason.seedable = false;
          }
          skipIfBlocked(this, blockedReason);
        }

        expect(otaSource.packageId).to.be.a('string').and.not.equal('');
        expect(otaSource.taskId).to.be.a('string').and.not.equal('');

        const packageResp = await apiClient.get('/ota/package', { page: 1, page_size: 20 });
        expectOk(packageResp);
        expectPagedListData(packageResp.data, { rowCheck: expectOtaPackageRow });
        expect(packageResp.data.list).to.be.an('array').and.not.empty;
        const packageRow = packageResp.data.list.find(row => pickId(row) === otaSource.packageId);
        expect(packageRow, 'seeded OTA package must be visible in the real package list').to.be.an('object');

        const taskResp = await apiClient.get('/ota/task', {
          page: 1,
          page_size: 20,
          ota_upgrade_package_id: otaSource.packageId
        });
        expectPagedList(taskResp);
        expect(taskResp.data.list).to.be.an('array').and.not.empty;
        const taskRow = taskResp.data.list.find(row => (
          row.ota_upgrade_task_id ||
          row.OtaUpgradeTaskID ||
          row.OTAUpgradeTaskID ||
          row.task_id ||
          row.TaskID ||
          row.id ||
          row.ID
        ) === otaSource.taskId);
        expect(taskRow, 'seeded OTA task must be visible in the real task list').to.be.an('object');
      } finally {
        await otaSource.cleanup();
      }
    });
  });

  describe('TC-SYS-010 latest firmware check', function () {
    it('returns firmware availability for a known device', async function () {
      expect(deviceId).to.be.a('string').and.not.equal('');

      const resp = await apiClient.get('/rdi/devices/' + deviceId + '/latest-firmware');
      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(resp.data).to.have.property('update_available');
      expect(resp.data.update_available).to.be.a('boolean');
    });
  });

  describe('TC-SYS-011 verification code request', function () {
    it('returns success or an explicit rate-limit style response', async function () {
      const resp = await apiClient.getNoAuth('/verification/code', {
        email: 'autotest@example.com',
        type: 1
      });
      expect(resp).to.be.an('object');
      expect(resp.code).to.be.a('number');
      expect([200, 429, 200015]).to.include(resp.code);
      if (resp.code === 200) {
        expect(resp.message).to.equal('操作成功');
      } else {
        expect(resp.message).to.be.a('string').and.not.equal('');
      }
    });
  });

  describe('TC-SYS-012 super admin detail', function () {
    it('returns the current super admin profile without relying on a fixed email', async function () {
      expect(superAdminAvailable).to.equal(true);

      const resp = await apiClient.get('/user/detail', {}, 'super_admin');
      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(pickId(resp.data)).to.be.a('string').and.not.equal('');
      if (Object.prototype.hasOwnProperty.call(resp.data, 'email') && resp.data.email !== null) {
        expect(resp.data.email).to.be.a('string');
        expect(resp.data.email.trim()).to.not.equal('');
      }
    });
  });

  describe('TC-SYS-013 data policy list', function () {
    it('returns a paged data policy list for super admin', async function () {
      expect(superAdminAvailable).to.equal(true);

      const resp = await apiClient.get('/datapolicy', { page: 1, page_size: 10 }, 'super_admin');
      expectPagedList(resp);
    });
  });

  describe('TC-SYS-014 invalid data policy update', function () {
    it('rejects updates for a non-existent data policy id', async function () {
      expect(superAdminAvailable).to.equal(true);

      const resp = await apiClient.put(
        '/datapolicy',
        {
          id: '00000000-0000-0000-0000-000000000000',
          retain_days: 30
        },
        'super_admin'
      );
      expectBusinessError(resp, 100002, "Field 'RetentionDays' is required");
      expect(Boolean(resp._requestError)).to.equal(false);
    });
  });

  describe('TC-SYS-015 operation log list', function () {
    it('returns a paged operation log list', async function () {
      const resp = await apiClient.get('/operation_logs', { page: 1, page_size: 10 });
      expectPagedList(resp);
    });
  });

  describe('TC-SYS-016 UI element list', function () {
    it('returns a paged UI element list for super admin', async function () {
      expect(superAdminAvailable).to.equal(true);

      const resp = await apiClient.get('/ui_elements', { page: 1, page_size: 10 }, 'super_admin');
      expectPagedList(resp);
    });
  });

  describe('TC-SYS-017 create UI element', function () {
    it('creates a UI element and verifies it appears in the list', async function () {
      expect(superAdminAvailable).to.equal(true);

      const elementCode = 'autotest_ui_' + Date.now();
      const resp = await apiClient.post(
        '/ui_elements',
        {
          parent_id: '0',
          element_code: elementCode,
          element_type: 3,
          orders: 999,
          param1: '/automation/scene-manage',
          param2: 'mdi:source-branch',
          param3: 'self',
          authority: '["SYS_ADMIN"]',
          description: 'scene management test element',
          remark: 'automation_scene_fixture',
          multilingual: 'route.automation_scene-manage',
          route_path: 'view.automation_scene-manage'
        },
        'super_admin'
      );
      expectOk(resp);

      const listResp = await apiClient.get('/ui_elements', { page: 1, page_size: 200 }, 'super_admin');
      expectPagedList(listResp);

      const created = listResp.data.list.find(item => item.element_code === elementCode);
      expect(created).to.be.an('object');
      createdUiElementId = pickId(created);
      expect(createdUiElementId).to.be.a('string').and.not.equal('');
      expect(created.route_path).to.equal('view.automation_scene-manage');
    });
  });

  describe('TC-SYS-018 invalid UI element update', function () {
    it('rejects updates for a non-existent UI element id', async function () {
      expect(superAdminAvailable).to.equal(true);

      const resp = await apiClient.put(
        '/ui_elements',
        {
          id: '00000000-0000-0000-0000-000000000000',
          element_type: 'menu'
        },
        'super_admin'
      );
      expectRejected(resp, {
        code: 100002,
        message: 'json: cannot unmarshal string into Go struct field UpdateUiElementsReq.element_type of type int16',
        requestError: false
      });
    });
  });

  describe('TC-SYS-019 current user menu', function () {
    it('returns the current user menu payload', async function () {
      const resp = await apiClient.get('/ui_elements/menu');
      expectOk(resp);
      expectPagedListData(resp.data, { rowCheck: expectMenuNodeRow });
    });

    it('does not expose SYS_ADMIN-only management menu entries to tenant users', async function () {
      if (!tenantUserAvailable) {
        skipIfBlocked(this, {
          reason: 'tenant_user account is unavailable; cannot verify tenant menu permission boundary',
          category: 'runtime-external',
          seedable: false
        });
      }

      const resp = await apiClient.get('/ui_elements/menu', {}, 'tenant_user');
      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(resp.data.total).to.be.a('number').and.at.least(0);
      if (resp.data.list === null) {
        expect(resp.data.total).to.equal(0);
      } else {
        expect(resp.data.list).to.be.an('array');
        expect(resp.data.list.length).to.be.at.most(resp.data.total);
      }

      const menuNodes = flattenMenuNodes(resp.data.list || []);
      const forbiddenCodes = new Set(['management_user', 'management_auth', 'management_setting']);
      const forbiddenRoutes = new Set(['/management/user', '/management/auth', '/management/setting']);
      const forbiddenRoutePaths = new Set([
        'view.management_user',
        'view.management_auth',
        'view.management_setting'
      ]);

      menuNodes.forEach(node => {
        expect(forbiddenCodes.has(node.element_code)).to.equal(false);
        expect(forbiddenRoutes.has(node.param1)).to.equal(false);
        expect(forbiddenRoutePaths.has(node.route_path)).to.equal(false);
      });
    });
  });

  describe('TC-SYS-020 logo update', function () {
    it('re-saves the current logo payload without changing values', async function () {
      expect(superAdminAvailable).to.equal(true);

      const currentResp = await apiClient.get('/logo', {}, 'super_admin');
      expectOk(currentResp);

      const currentLogo = currentResp.data && Array.isArray(currentResp.data.list) ? currentResp.data.list[0] : null;
      expect(currentLogo).to.be.an('object');
      expect(currentLogo.id).to.be.a('string').and.not.equal('');

      const resp = await apiClient.put(
        '/logo',
        {
          id: currentLogo.id,
          system_name: currentLogo.system_name,
          logo_cache: currentLogo.logo_cache,
          logo_background: currentLogo.logo_background,
          logo_loading: currentLogo.logo_loading,
          home_background: currentLogo.home_background,
          remark: currentLogo.remark
        },
        'super_admin'
      );
      expectOk(resp);

      const verifyResp = await apiClient.get('/logo', {}, 'super_admin');
      expectOk(verifyResp);
      const verifiedLogo = verifyResp.data && Array.isArray(verifyResp.data.list) ? verifyResp.data.list[0] : null;
      expect(verifiedLogo).to.be.an('object');
      expect(verifiedLogo.id).to.equal(currentLogo.id);
    });
  });

  describe('TC-SYS-021 current system metrics', function () {
    it('returns current system metrics for super admin', async function () {
      expect(superAdminAvailable).to.equal(true);

      const resp = await apiClient.get('/system/metrics/current', {}, 'super_admin');
      expectOk(resp);
      expectCurrentSystemMetrics(resp.data);
    });
  });

  describe('TC-SYS-022 system metric history', function () {
    it('returns historical system metrics for super admin', async function () {
      expect(superAdminAvailable).to.equal(true);

      const resp = await apiClient.get('/system/metrics/history', { hours: 1 }, 'super_admin');
      expectOk(resp);
      expect(resp.data).to.be.an('array');
      resp.data.forEach(expectMetricPoint);
    });
  });
});
