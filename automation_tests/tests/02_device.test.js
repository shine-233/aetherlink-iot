/**
 * 文件用途：用于验证设备管理 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const testData = require('../lib/test_data');
const seedData = require('../lib/seed_data');
const {
  createTenantAdminAccount,
  createTenantUserAccount,
  cleanupDynamicAccounts
} = require('./helpers/dynamic_accounts');
const {
  expectSuccess: expectOk,
  expectBusinessError,
  expectPermissionDenied
} = require('../lib/response_assertions');
const { skipWhenBlocked } = require('../lib/integration_blocked');

function pickId(record) {
  return record && (record.id || record.ID) ? (record.id || record.ID) : null;
}

function expectDeviceListRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('id', 'name', 'pid_number');
  expect(row.id).to.be.a('string').and.not.equal('');
  expect(row.name).to.be.a('string').and.not.equal('');
  expect(row.pid_number).to.be.a('string').and.not.equal('');
}

function expectDeviceSelectorRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('device_id', 'device_name', 'device_type');
  expect(row.device_id).to.be.a('string').and.not.equal('');
  expect(row.device_name).to.be.a('string').and.not.equal('');
  expect(row.device_type).to.be.a('string');
}

function expectDeviceGroupTreeNode(node) {
  expect(node).to.be.an('object');
  expect(node).to.have.property('group').that.is.an('object');
  if (Object.prototype.hasOwnProperty.call(node.group, 'id')) {
    expect(node.group.id).to.be.a('string').and.not.equal('');
  }
  if (Object.prototype.hasOwnProperty.call(node.group, 'name')) {
    expect(node.group.name).to.be.a('string');
  }
  if (Object.prototype.hasOwnProperty.call(node, 'children')) {
    expect(node.children).to.be.an('array');
    node.children.forEach(expectDeviceGroupTreeNode);
  }
}

function flattenDeviceGroupTree(nodes) {
  const flattened = [];
  const visit = (items) => {
    if (!Array.isArray(items)) return;
    items.forEach(item => {
      expectDeviceGroupTreeNode(item);
      flattened.push(item);
      visit(item.children);
    });
  };
  visit(nodes);
  return flattened;
}

describe('Device API module [02_device]', function () {
  this.timeout(30000);

  let activatedDeviceId = null;
  let shareToken = null;
  let sharePath = null;
  let sharedDeviceConfig = null;
  let recipientAccountKey = null;
  let ownerTenantId = null;
  let recipientTenantId = null;
  let seededDevice = null;
  const dynamicRecipientAccounts = [];
  const createdOwnerDevices = [];
  const createdGroupIds = [];
  const createdTemplateIds = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 02_device.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');
    // Always use a fresh recipient.  The shared tenant_admin_b fixture can
    // retain a prior share relation after a rerun, which changes the first
    // accept oracle from already_accepted=false to true.  A dynamic account
    // keeps the cross-tenant boundary while making the test repeatable.
    const recipientPrefix = process.env.AETHERLINK_RDI_FIXTURE_MODE === 'synthetic-rdi'
      ? 'codex_synthetic_cross_tenant'
      : 'codex_device_recipient';
    const account = await createTenantAdminAccount(apiClient, recipientPrefix);
    dynamicRecipientAccounts.push(account);
    recipientAccountKey = account.accountKey;

    const ownerProfileResp = await apiClient.get('/board/user/info', {}, 'tenant_admin');
    expectOk(ownerProfileResp);
    ownerTenantId = ownerProfileResp.data && (ownerProfileResp.data.tenant_id || ownerProfileResp.data.tenantId);
    expect(ownerTenantId, 'owner tenant id').to.be.a('string').and.not.equal('');

    const recipientProfileResp = await apiClient.get('/board/user/info', {}, recipientAccountKey);
    expectOk(recipientProfileResp);
    recipientTenantId = recipientProfileResp.data &&
      (recipientProfileResp.data.tenant_id || recipientProfileResp.data.tenantId);
    expect(recipientTenantId, 'recipient tenant id').to.be.a('string').and.not.equal('');
    expect(recipientTenantId, 'synthetic share recipient must be cross-tenant')
      .to.not.equal(ownerTenantId);

    seededDevice = await seedData.ensureRdiDevice('tenant_admin');
    activatedDeviceId = seededDevice.id;
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const seedShareResp = await apiClient.post(
      '/rdi/devices/' + activatedDeviceId + '/share-token',
      testData.getShareTokenReq()
    );
    expectOk(seedShareResp);
    shareToken = seedShareResp.data.token;
    sharePath = seedShareResp.data.share_path;
    expect(shareToken).to.be.a('string').and.not.equal('');

    const seedSharedResp = await apiClient.getNoAuth('/rdi/shared/' + shareToken);
    expectOk(seedSharedResp);
    sharedDeviceConfig = seedSharedResp.data;
    expect(sharedDeviceConfig.device_id).to.equal(activatedDeviceId);
  });

  after(async function () {
    try {
      for (const id of createdTemplateIds) {
        try {
          await apiClient.delete('/device/template/' + id);
        } catch (error) {
          // Cleanup failure should not mask the test verdict.
        }
      }

      for (const id of createdGroupIds) {
        try {
          await apiClient.delete('/device/group/' + id);
        } catch (error) {
          // Cleanup failure should not mask the test verdict.
        }
      }

      for (const device of createdOwnerDevices) {
        try {
          await apiClient.delete('/device/' + device.id, {}, device.accountKey);
        } catch (error) {
          try {
            await apiClient.delete('/device/' + device.id, {}, 'tenant_admin');
          } catch (fallbackError) {
            // Cleanup failure should not mask the test verdict.
          }
        }
      }

      await cleanupDynamicAccounts(apiClient, dynamicRecipientAccounts);
      if (seededDevice && seededDevice.cleanup) {
        await seededDevice.cleanup();
      }
    } finally {
      apiClient.clearAllTokens();
    }
  });

  async function createGroup(namePrefix) {
    const name = namePrefix + '_' + Date.now();
    const resp = await apiClient.post('/device/group', { name });
    expectOk(resp);

    const pageResp = await apiClient.get('/device/group', { page: 1, page_size: 20 });
    expectOk(pageResp);
    const list = pageResp.data && Array.isArray(pageResp.data.list) ? pageResp.data.list : [];
    const found = list.find(item => item.name === name);
    expect(found).to.be.an('object');

    const id = pickId(found);
    expect(id).to.be.a('string').and.not.equal('');
    createdGroupIds.push(id);
    return { id, name };
  }

  async function createTemplate(namePrefix) {
    const name = namePrefix + '_' + Date.now();
    const resp = await apiClient.post('/device/template', {
      name,
      description: 'created by hardened device automation'
    });
    expectOk(resp);
    expect(resp.data).to.be.an('object');

    const id = pickId(resp.data);
    expect(id).to.be.a('string').and.not.equal('');
    createdTemplateIds.push(id);
    return { id, name, resp };
  }

  async function createOwnedDevice(accountKey, namePrefix) {
    const suffix = Date.now().toString(36) + Math.floor(Math.random() * 100000).toString(36);
    const name = namePrefix + '_' + suffix;
    const deviceNumber = ('own' + suffix).slice(0, 36);
    const resp = await apiClient.post('/device', {
      name,
      device_number: deviceNumber,
      voucher: JSON.stringify({
        username: deviceNumber,
        password: 'owner-api'
      })
    }, accountKey);
    expectOk(resp);

    const id = pickId(resp.data);
    expect(id).to.be.a('string').and.not.equal('');
    createdOwnerDevices.push({ id, accountKey });
    return { id, name, deviceNumber };
  }

  it('returns the current RDI thing model definition', async function () {
    const resp = await apiClient.get('/rdi/thing-model');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.telemetry).to.be.an('array');
    expect(resp.data.properties).to.be.an('array');
    expect(resp.data.events).to.be.an('array');
    expect(resp.data.services).to.be.an('array');

    const telemetryIds = resp.data.telemetry.map(item => item.identifier);
    expect(telemetryIds).to.include('temperature_1');
    expect(telemetryIds).to.include('temperature_2');
    expect(telemetryIds).to.include('switch_1');
    expect(telemetryIds).to.include('switch_2');
  });

  it('returns the current tenant device list with pagination', async function () {
    const resp = await apiClient.get('/device', { page: 1, page_size: 10 });

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number').and.at.least(1);
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.list.length).to.be.at.least(1);
    expect(resp.data.list.length).to.be.at.most(resp.data.total);
    resp.data.list.forEach(expectDeviceListRow);

    const seededRow = resp.data.list.find(item => pickId(item) === activatedDeviceId);
    expect(seededRow, 'seeded device must be visible in the tenant device list').to.be.an('object');
    expectDeviceListRow(seededRow);
  });

  it('returns the opt-in RDI installation summary without exposing private device metadata', async function () {
    const defaultResp = await apiClient.get('/device', { page: 1, page_size: 100 });
    expectOk(defaultResp);
    const defaultRow = defaultResp.data.list.find(item => pickId(item) === activatedDeviceId);
    expect(defaultRow, 'seeded device must be visible in the default device list').to.be.an('object');
    expect(defaultRow).to.not.have.property('rdi_system_info_summary');

    const summaryResp = await apiClient.get('/device', {
      page: 1,
      page_size: 100,
      include_rdi_system_info_summary: true
    });
    expectOk(summaryResp);
    const summaryRow = summaryResp.data.list.find(item => pickId(item) === activatedDeviceId);
    expect(summaryRow, 'seeded device must be visible in the opt-in device list').to.be.an('object');
    expect(summaryRow).to.have.property('rdi_system_info_summary').that.is.an('object');

    const summary = summaryRow.rdi_system_info_summary;
    [
      'customer_name',
      'contact_email',
      'contact_phone',
      'warranty_status',
      'extra_fields',
      'rdi_config',
      'rdi_share_tokens',
      'rdi_share_recipients'
    ].forEach(key => {
      expect(summary, 'private field must not be exposed: ' + key).to.not.have.property(key);
    });
  });

  it('keeps tenant_user device list and selector scoped to owned devices', async function () {
    const ownerAccount = await createTenantUserAccount(apiClient, 'codex_owner_scope');
    dynamicRecipientAccounts.push(ownerAccount);
    const ownedDevice = await createOwnedDevice(ownerAccount.accountKey, 'codex_owner_device');
    expect(ownedDevice.id).to.not.equal(activatedDeviceId);

    const listResp = await apiClient.get('/device', { page: 1, page_size: 100 }, ownerAccount.accountKey);
    expectOk(listResp);
    expect(listResp.data).to.be.an('object');
    expect(listResp.data.list).to.be.an('array');

    const visibleDeviceIds = listResp.data.list.map(pickId).filter(Boolean);
    expect(visibleDeviceIds, 'tenant_user should see the device it created').to.include(ownedDevice.id);
    expect(visibleDeviceIds, 'tenant_user must not see a tenant-admin-owned device').to.not.include(activatedDeviceId);

    const selectorResp = await apiClient.get('/device/selector', { page: 1, page_size: 100 }, ownerAccount.accountKey);
    expectOk(selectorResp);
    expect(selectorResp.data).to.be.an('object');
    expect(selectorResp.data.list).to.be.an('array');

    const selectorDeviceIds = selectorResp.data.list.map(row => row && row.device_id).filter(Boolean);
    expect(selectorDeviceIds, 'tenant_user selector should include the owned device').to.include(ownedDevice.id);
    expect(selectorDeviceIds, 'tenant_user selector must not include a tenant-admin-owned device').to.not.include(activatedDeviceId);
  });

  it('allows only super_admin to opt into the all-tenant RDI overview scope', async function () {
    const otherTenantDevice = await seedData.ensureDevice(recipientAccountKey);
    expect(otherTenantDevice.id).to.be.a('string').and.not.equal('');
    expect(otherTenantDevice.id).to.not.equal(activatedDeviceId);

    try {
      const listResp = await apiClient.get(
        '/device',
        { page: 1, page_size: 1000, all_tenants: true, include_rdi_system_info_summary: true },
        'super_admin'
      );
      expectOk(listResp);
      expect(listResp.data).to.be.an('object');
      expect(listResp.data.list).to.be.an('array');
      const deviceRows = new Map(listResp.data.list.map(row => [pickId(row), row]));
      const primaryRow = deviceRows.get(activatedDeviceId);
      const otherTenantRow = deviceRows.get(otherTenantDevice.id);
      expect(primaryRow, 'system administrator should see the primary tenant device').to.be.an('object');
      expect(otherTenantRow, 'system administrator should see the other tenant device').to.be.an('object');
      expect(primaryRow.scope_tenant_id).to.be.a('string').and.not.equal('');
      expect(otherTenantRow.scope_tenant_id).to.be.a('string').and.not.equal('');
      expect(primaryRow.scope_tenant_id).to.not.equal(otherTenantRow.scope_tenant_id);

      const overviewResp = await apiClient.get(
        '/board/tenant/device/info',
        { all_tenants: true },
        'super_admin'
      );
      expectOk(overviewResp);
      expect(overviewResp.data.device_total).to.be.a('number').and.at.least(2);
      expect(overviewResp.data.device_on).to.be.a('number');
      expect(overviewResp.data.device_offline).to.be.a('number');

      const countsResp = await apiClient.get('/alarm/device/counts', { all_tenants: true }, 'super_admin');
      expectOk(countsResp);
      expect(countsResp.data.alarm_device_total).to.be.a('number');
      expect(countsResp.data.active_alarm_total).to.be.a('number');

      const historyResp = await apiClient.get(
        '/alarm/info/history',
        { page: 1, page_size: 100, all_tenants: true },
        'super_admin'
      );
      expectOk(historyResp);
      expect(historyResp.data.total).to.be.a('number');
      expect(historyResp.data.list).to.be.an('array');

      const trendResp = await apiClient.get(
        '/alarm/info/history/monthly',
        { year: new Date().getUTCFullYear(), timezone: 'UTC', all_tenants: true },
        'super_admin'
      );
      expectOk(trendResp);
      expect(trendResp.data.months).to.be.an('array').and.have.lengthOf(12);

      const tenantAdminDeniedResponses = await Promise.all([
        apiClient.get('/device', { page: 1, page_size: 10, all_tenants: true }, 'tenant_admin'),
        apiClient.get('/board/tenant/device/info', { all_tenants: true }, 'tenant_admin'),
        apiClient.get('/alarm/device/counts', { all_tenants: true }, 'tenant_admin'),
        apiClient.get('/alarm/info/history', { page: 1, page_size: 10, all_tenants: true }, 'tenant_admin'),
        apiClient.get(
          '/alarm/info/history/monthly',
          { year: new Date().getUTCFullYear(), timezone: 'UTC', all_tenants: true },
          'tenant_admin'
        )
      ]);
      tenantAdminDeniedResponses.forEach(expectPermissionDenied);
    } finally {
      await otherTenantDevice.cleanup();
    }
  });

  it('rejects tenant_user direct and batch actions on tenant-admin alarm history', async function () {
    const historySeed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    if (historySeed.blocked) {
      throw new Error('Alarm owner boundary seed is blocked: ' + historySeed.reason);
    }

    try {
      const tenantUser = await createTenantUserAccount(apiClient, 'codex_alarm_owner_boundary');
      dynamicRecipientAccounts.push(tenantUser);

      const listResp = await apiClient.get(
        '/alarm/info/history',
        { page: 1, page_size: 100 },
        tenantUser.accountKey
      );
      expectOk(listResp);
      expect(listResp.data.list).to.be.an('array');
      const visibleHistoryIds = listResp.data.list.map(pickId).filter(Boolean);
      expect(visibleHistoryIds, 'tenant_user list must hide tenant-admin alarm history').to.not.include(historySeed.id);

      const detailResp = await apiClient.get(
        '/alarm/info/history/' + historySeed.id,
        {},
        tenantUser.accountKey
      );
      expectPermissionDenied(detailResp);

      const descriptionResp = await apiClient.put(
        '/alarm/info/history',
        { id: historySeed.id, description: 'unauthorized update' },
        tenantUser.accountKey
      );
      expectPermissionDenied(descriptionResp);

      const acknowledgeResp = await apiClient.put(
        '/alarm/info/history/' + historySeed.id + '/acknowledge',
        {},
        tenantUser.accountKey
      );
      expectPermissionDenied(acknowledgeResp);

      const resetResp = await apiClient.put(
        '/alarm/info/history/' + historySeed.id + '/reset',
        {},
        tenantUser.accountKey
      );
      expectPermissionDenied(resetResp);

      const batchResp = await apiClient.put(
        '/alarm/info/history/batch-action',
        { ids: [historySeed.id], action: 'acknowledge' },
        tenantUser.accountKey
      );
      expectOk(batchResp);
      expect(batchResp.data.success_count).to.equal(0);
      expect(batchResp.data.failure_count).to.equal(1);
      expect(batchResp.data.results).to.be.an('array').with.length(1);
      expect(batchResp.data.results[0].ok).to.equal(false);
      expect(batchResp.data.results[0].error).to.match(/no permission/i);

      const deleteResp = await apiClient.delete(
        '/alarm/info/history/' + historySeed.id,
        {},
        tenantUser.accountKey
      );
      expectPermissionDenied(deleteResp);
    } finally {
      await historySeed.cleanup();
    }
  });

  it('rejects a PID that is too short', async function () {
    const resp = await apiClient.post('/rdi/devices/activate', {
      pid_number: testData.getDevicePID('invalid_pid_short'),
      name: 'TestShort'
    });

    expectBusinessError(resp, 100002, 'exactly 12 alphanumeric characters');
  });

  it('rejects a PID that is too long', async function () {
    const resp = await apiClient.post('/rdi/devices/activate', {
      pid_number: testData.getDevicePID('invalid_pid_long'),
      name: 'TestLong'
    });

    expectBusinessError(resp, 100002, 'exactly 12 alphanumeric characters');
  });

  it('rejects a PID with special characters', async function () {
    const resp = await apiClient.post('/rdi/devices/activate', {
      pid_number: testData.getDevicePID('invalid_pid_special'),
      name: 'TestSpecial'
    });

    expectBusinessError(resp, 100002, 'only letters and numbers');
  });

  it('keeps the inactive PID activation case honest when the fixture is already consumed', async function () {
    const pid = testData.getDevicePID('inactive_pid');
    const activate = () => apiClient.post('/rdi/devices/activate', {
      pid_number: pid,
      name: testData.generateDeviceName('AutoActivate')
    });
    const resp = await activate();

    if (resp.code === 200) {
      // 激活成功分支：回读一致 + 立即重复激活必须被 204002 拒绝（激活状态机闭环）。
      expect(resp.data).to.be.an('object');
      expect(resp.data.pid_number).to.equal(pid);
      const duplicateResp = await activate();
      expectBusinessError(duplicateResp, 204002);
      return;
    }

    // 未走成功分支时，本栈夹具只有两种合法状态：
    //   204001 = PID 未预置（rdi.go: GetDeviceByDeviceNumber ErrRecordNotFound）
    //   204002 = 已被先前运行激活（ActivateFlag == "active"）
    // （2026-09-04 审计修正：原实现把"已激活"误断言为 204001，与后端状态机不符。）
    expect(resp.code).to.be.an('number');
    expect([204001, 204002]).to.include(resp.code);
    console.warn('  inactive test PID fixture not activatable in this run (code=' + resp.code + ')');
  });

  it('rejects duplicate activation for an already activated PID', async function () {
    const pid = testData.getDevicePID('activated_pid');
    const listResp = await apiClient.get('/device', {
      page: 1,
      page_size: 100,
      device_number: pid
    });
    expectOk(listResp);
    const matchingRows = Array.isArray(listResp.data && listResp.data.list)
      ? listResp.data.list.filter(row => String(row && (row.pid_number || row.device_number || '')).trim() === pid)
      : [];
    const activeRow = matchingRows.find(row => String(row.activate_flag || '').trim().toLowerCase() === 'active');

    if (!activeRow) {
      skipWhenBlocked(this, true, {
        reason: `activated PID ${pid} is not visible as an active pre-registered fixture in the current database`,
        category: 'runtime-external',
        seedable: false
      });
      return;
    }

    const resp = await apiClient.post('/rdi/devices/activate', {
      pid_number: pid,
      name: 'DuplicateActivate'
    });

    // Missing pre-registration is handled as an explicit blocked fixture
    // above.  Once an active row is visible, the backend must expose the
    // duplicate-activation business contract rather than a permissive
    // fallback.
    expectBusinessError(resp, 204002);
  });

  it('returns config, system info, and thing model for the activated device', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/rdi/devices/' + activatedDeviceId + '/config');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.device_id).to.equal(activatedDeviceId);
    expect(resp.data.config).to.be.an('object');
    expect(resp.data.system_info).to.be.an('object');
    expect(resp.data.thing_model).to.be.an('object');
  });

  it('returns device detail payload for the activated device', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/device/detail/' + activatedDeviceId);

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.equal(activatedDeviceId);
    expect(resp.data.device_number).to.be.a('string');
    expect(resp.data.name).to.be.a('string');
  });

  it('returns the current device online status payload', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/device/online/status/' + activatedDeviceId);

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data).to.include.keys('device_status', 'is_online');
  });

  it('creates a share token and exposes the public share path', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.post(
      '/rdi/devices/' + activatedDeviceId + '/share-token',
      testData.getShareTokenReq()
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.token).to.be.a('string').and.not.equal('');
    expect(resp.data.share_path).to.be.a('string').and.include('/device/share?share_token=');
    expect(resp.data.expires_at).to.be.a('number');

    shareToken = resp.data.token;
    sharePath = resp.data.share_path;
  });

  it('returns the public shared device payload for a valid token', async function () {
    expect(shareToken).to.be.a('string').and.not.equal('');

    const resp = await apiClient.getNoAuth('/rdi/shared/' + shareToken);

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.device_id).to.equal(activatedDeviceId);
    expect(resp.data.config).to.be.an('object');
    expect(resp.data.system_info).to.be.an('object');
    expect(resp.data.thing_model).to.be.an('object');
    expect(sharePath).to.include(shareToken);

    sharedDeviceConfig = resp.data;
  });

  it('allows a fresh cross-tenant recipient to accept the shared device', async function () {
    expect(shareToken).to.be.a('string').and.not.equal('');
    expect(recipientAccountKey).to.be.a('string').and.not.equal('');

    const resp = await apiClient.post(
      '/rdi/share-tokens/' + shareToken + '/accept',
      {},
      recipientAccountKey
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.device).to.be.an('object');
    expect(resp.data.device.device_id).to.equal(activatedDeviceId);
    expect(resp.data.accepted_at).to.be.a('number');
    expect(resp.data.already_accepted).to.equal(false);
    expect(resp.data.shared_with_me).to.equal(true);
  });

  it('marks repeated share acceptance as already accepted', async function () {
    expect(shareToken).to.be.a('string').and.not.equal('');
    expect(recipientAccountKey).to.be.a('string').and.not.equal('');

    const resp = await apiClient.post(
      '/rdi/share-tokens/' + shareToken + '/accept',
      {},
      recipientAccountKey
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.device.device_id).to.equal(activatedDeviceId);
    expect(resp.data.already_accepted).to.equal(true);
  });

  it('allows the cross-tenant recipient to read but not write or reshare the accepted RDI device', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');
    expect(recipientAccountKey).to.be.a('string').and.not.equal('');

    const readResp = await apiClient.get(
      '/rdi/devices/' + activatedDeviceId + '/config',
      {},
      recipientAccountKey
    );
    expectOk(readResp);
    expect(readResp.data.device_id).to.equal(activatedDeviceId);
    expect(readResp.data.config).to.be.an('object');

    const attemptedConfig = {
      ...readResp.data.config,
      dry_contact_alarm_delay: Number(readResp.data.config.dry_contact_alarm_delay || 0) + 1
    };
    const updateResp = await apiClient.put(
      '/rdi/devices/' + activatedDeviceId + '/config',
      { config: attemptedConfig, apply_to_device: false },
      recipientAccountKey
    );
    expectBusinessError(updateResp, 201001);

    const reshareResp = await apiClient.post(
      '/rdi/devices/' + activatedDeviceId + '/share-token',
      testData.getShareTokenReq(),
      recipientAccountKey
    );
    expectBusinessError(reshareResp, 201001);
  });

  it('allows a same-tenant tenant_user to accept and read but not manage a shared device', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const recipient = await createTenantUserAccount(apiClient, 'codex_same_tenant_share');
    dynamicRecipientAccounts.push(recipient);

    const tokenResp = await apiClient.post(
      '/rdi/devices/' + activatedDeviceId + '/share-token',
      testData.getShareTokenReq(),
      'tenant_admin'
    );
    expectOk(tokenResp);
    const token = tokenResp.data.token;
    expect(token).to.be.a('string').and.not.equal('');

    const firstAcceptResp = await apiClient.post(
      '/rdi/share-tokens/' + token + '/accept',
      {},
      recipient.accountKey
    );
    expectOk(firstAcceptResp);
    expect(firstAcceptResp.data.device.device_id).to.equal(activatedDeviceId);
    expect(firstAcceptResp.data.already_accepted).to.equal(false);
    expect(firstAcceptResp.data.shared_with_me).to.equal(true);

    const repeatedAcceptResp = await apiClient.post(
      '/rdi/share-tokens/' + token + '/accept',
      {},
      recipient.accountKey
    );
    expectOk(repeatedAcceptResp);
    expect(repeatedAcceptResp.data.already_accepted).to.equal(true);
    expect(repeatedAcceptResp.data.shared_with_me).to.equal(true);

    const listResp = await apiClient.get(
      '/rdi/shared-with-me/devices',
      { page: 1, page_size: 20, device_id: activatedDeviceId },
      recipient.accountKey
    );
    expectOk(listResp);
    expect(listResp.data.list).to.be.an('array');
    const sharedRow = listResp.data.list.find(item => item.device && item.device.device_id === activatedDeviceId);
    expect(sharedRow, 'same-tenant accepted device must appear in shared-with-me').to.be.an('object');

    const readResp = await apiClient.get(
      '/rdi/devices/' + activatedDeviceId + '/config',
      {},
      recipient.accountKey
    );
    expectOk(readResp);
    expect(readResp.data.device_id).to.equal(activatedDeviceId);
    expect(readResp.data.config).to.be.an('object');

    const updateResp = await apiClient.put(
      '/rdi/devices/' + activatedDeviceId + '/config',
      { config: readResp.data.config, apply_to_device: false },
      recipient.accountKey
    );
    expectBusinessError(updateResp, 201001);

    const reshareResp = await apiClient.post(
      '/rdi/devices/' + activatedDeviceId + '/share-token',
      testData.getShareTokenReq(),
      recipient.accountKey
    );
    expectBusinessError(reshareResp, 201001);
  });

  it('rejects an invalid public share token with the current business error', async function () {
    const resp = await apiClient.getNoAuth('/rdi/shared/not-a-real-share-token');

    expectBusinessError(resp, 201001, 'share token is invalid or expired');
  });

  it('returns the accepted shared device in the recipient tenant list', async function () {
    expect(shareToken).to.be.a('string').and.not.equal('');
    expect(sharedDeviceConfig).to.be.an('object');
    expect(recipientAccountKey).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get(
      '/rdi/shared-with-me/devices',
      {
        page: 1,
        page_size: 20,
        device_id: sharedDeviceConfig.device_id
      },
      recipientAccountKey
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number');
    expect(resp.data.list).to.be.an('array');

    const matched = resp.data.list.find(item => item.device && item.device.device_id === sharedDeviceConfig.device_id);
    expect(matched).to.be.an('object');
    expect(matched.device.device_name).to.equal(sharedDeviceConfig.device_name);
  });

  it('returns the current device group tree payload', async function () {
    const group = await createGroup('codex_tree');
    const resp = await apiClient.get('/device/group/tree');

    expectOk(resp);
    expect(resp.data).to.be.an('array');
    expect(resp.data.length).to.be.at.least(1);

    const treeNodes = flattenDeviceGroupTree(resp.data);
    const matched = treeNodes.find(node => pickId(node.group) === group.id);
    expect(matched, 'created group must be visible in the group tree').to.be.an('object');
    expect(matched.group.name).to.equal(group.name);
  });

  it('returns the device selector payload', async function () {
    const resp = await apiClient.get('/device/selector', { page: 1, page_size: 20 });

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number').and.at.least(1);
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.list.length).to.be.at.least(1);
    expect(resp.data.list.length).to.be.at.most(resp.data.total);
    resp.data.list.forEach(expectDeviceSelectorRow);

    const seededRow = resp.data.list.find(item => item.device_id === activatedDeviceId);
    expect(seededRow, 'activated seeded device must be visible in the device selector').to.be.an('object');
    expectDeviceSelectorRow(seededRow);
  });

  it('returns the current device group page', async function () {
    const resp = await apiClient.get('/device/group', { page: 1, page_size: 10 });

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number');
    expect(resp.data.list).to.be.an('array');
  });

  it('creates, details, and updates a device group cleanly', async function () {
    const group = await createGroup('codex_group');

    const detailResp = await apiClient.get('/device/group/detail/' + group.id);
    expectOk(detailResp);
    expect(detailResp.data).to.be.an('object');
    expect(detailResp.data.detail).to.be.an('object');
    expect(detailResp.data.detail.id).to.equal(group.id);
    expect(detailResp.data.statistics).to.be.an('object');

    const updatedName = 'codex_group_updated_' + Date.now();
    const updateResp = await apiClient.put('/device/group', {
      id: group.id,
      parent_id: '0',
      name: updatedName
    });
    expectOk(updateResp);

    const pageResp = await apiClient.get('/device/group', { page: 1, page_size: 20 });
    expectOk(pageResp);
    const list = pageResp.data && Array.isArray(pageResp.data.list) ? pageResp.data.list : [];
    const updated = list.find(item => pickId(item) === group.id);
    expect(updated).to.be.an('object');
    expect(updated.name).to.equal(updatedName);
  });

  it('creates and lists a device-group relation for the activated device', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const group = await createGroup('codex_relation');
    const relationResp = await apiClient.post('/device/group/relation', {
      group_id: group.id,
      device_id_list: [activatedDeviceId]
    });

    expectOk(relationResp);

    const listResp = await apiClient.get('/device/group/relation', { device_id: activatedDeviceId });
    expectOk(listResp);
    expect(listResp.data).to.be.an('array');

    const matched = listResp.data.find(item => item.group_id === group.id);
    expect(matched).to.be.an('object');
  });

  it('rejects duplicate device-group relations with the current uniqueness error', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const group = await createGroup('codex_relation_dup');
    const firstResp = await apiClient.post('/device/group/relation', {
      group_id: group.id,
      device_id_list: [activatedDeviceId]
    });
    expectOk(firstResp);

    const secondResp = await apiClient.post('/device/group/relation', {
      group_id: group.id,
      device_id_list: [activatedDeviceId]
    });

    expectBusinessError(secondResp, 100000, '重复键违反唯一约束');
  });

  it('deletes a device-group relation and stays idempotent on a repeated delete', async function () {
    expect(activatedDeviceId).to.be.a('string').and.not.equal('');

    const group = await createGroup('codex_relation_delete');
    const createResp = await apiClient.post('/device/group/relation', {
      group_id: group.id,
      device_id_list: [activatedDeviceId]
    });
    expectOk(createResp);

    const firstDeleteResp = await apiClient.delete('/device/group/relation', {
      group_id: group.id,
      device_id: activatedDeviceId
    });
    expectOk(firstDeleteResp);

    // Readback via the list endpoint to verify the relation was actually removed
    // after the first delete (matches the readback pattern used in the create test).
    const listResp = await apiClient.get('/device/group/relation', { device_id: activatedDeviceId });
    expectOk(listResp);
    expect(listResp.data).to.be.an('array');

    const stillMatched = listResp.data.find(item => item.group_id === group.id);
    expect(stillMatched).to.equal(undefined);

    const secondDeleteResp = await apiClient.delete('/device/group/relation', {
      group_id: group.id,
      device_id: activatedDeviceId
    });
    expectOk(secondDeleteResp);
  });

  it('returns the current device template page', async function () {
    const resp = await apiClient.get('/device/template', { page: 1, page_size: 10 });

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number');
    expect(resp.data.list).to.be.an('array');
  });

  it('creates a device template and exposes it in the stats endpoint', async function () {
    const template = await createTemplate('codex_template');

    const statsResp = await apiClient.get('/device/template/stats', {
      device_template_id: template.id
    });

    expectOk(statsResp);
    expect(statsResp.data).to.be.an('object');
    expect(statsResp.data.device_template_id).to.equal(template.id);
    expect(statsResp.data.name).to.equal(template.name);
    expect(statsResp.data.total_devices).to.be.a('number');
    expect(statsResp.data.online_devices).to.be.a('number');
  });

  it('returns the current device template menu and selector payloads', async function () {
    const menuResp = await apiClient.get('/device/template/menu');
    expectOk(menuResp);
    expect(menuResp.data).to.be.an('array');

    const selectorResp = await apiClient.get('/device/template/selector');
    expectOk(selectorResp);
    expect(selectorResp.data).to.be.an('array');
  });

  it('returns the current database-not-found error for an invalid template id', async function () {
    const resp = await apiClient.get('/device/template/stats', {
      device_template_id: '00000000-0000-0000-0000-000000000000'
    });

    expectBusinessError(resp, 101001);
    expect(resp.data).to.be.an('object');
    expect(resp.data.sql_error).to.equal('record not found');
  });

  it('returns the current device template chart payload for the activated device', async function () {
    // seedData.ensureDevice 创建的设备 device_config_id 为空，/device/template/chart 在
    // device → device_config → device_template LEFT JOIN 找不到模板时会返回空对象 {}。
    // 要断言真实 chart payload，需要先打通 template → device_config → device 链路。
    const template = await createTemplate('codex_chart_payload');
    const configResp = await apiClient.post('/device_config', {
      name: 'chart_payload_config_' + Date.now(),
      device_type: '1',
      protocol_type: 'MQTT',
      voucher_type: 'ACCESSTOKEN',
      device_conn_type: 'A',
      protocol_config: '{}',
      device_template_id: template.id
    });
    expectOk(configResp);
    const configId = pickId(configResp.data);
    expect(configId).to.be.a('string').and.not.equal('');

    const deviceResp = await apiClient.post('/device', {
      name: 'chart_payload_device_' + Date.now(),
      device_config_id: configId,
      voucher: JSON.stringify({ username: 'chart_' + Date.now(), password: 'chart_pwd' })
    });
    expectOk(deviceResp);
    const deviceId = pickId(deviceResp.data);
    expect(deviceId).to.be.a('string').and.not.equal('');

    try {
      const resp = await apiClient.get('/device/template/chart', { device_id: deviceId });

      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(Array.isArray(resp.data), 'device template chart payload must not be an array').to.equal(false);
      expect(resp.data.id || resp.data.ID, 'device template chart id').to.be.a('string').and.not.equal('');
      expect(resp.data.name, 'device template chart name').to.be.a('string').and.not.equal('');
      expect(resp.data.tenant_id, 'device template chart tenant_id').to.be.a('string').and.not.equal('');
    } finally {
      await apiClient.delete('/device/' + deviceId);
      await apiClient.delete('/device_config/' + configId);
    }
  });
});
