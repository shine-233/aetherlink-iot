/**
 * 文件用途：用于验证设备告警共享 API 自动化测试。
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
  cleanupDynamicAccounts
} = require('./helpers/dynamic_accounts');

const ZERO_UUID = '00000000-0000-0000-0000-000000000000';

function nestedCode(resp) {
  return resp && resp.data && typeof resp.data === 'object' ? resp.data.code : undefined;
}

function responseText(resp) {
  return JSON.stringify(resp && resp.data !== undefined ? resp.data : resp);
}

function expectValidation(resp, messagePart) {
  expect(resp.code).to.equal(100002);
  expect(resp.message).to.include(messagePart);
}

function expectRecordNotFound(resp) {
  expect(resp.code).to.equal(101001);
  expect(responseText(resp)).to.include('record not found');
}

function expectNoAuth(resp) {
  expect(resp.code).to.equal(401);
  expect(resp.message).to.include('missing authentication');
  expect(nestedCode(resp)).to.equal(40100);
}

function expectInvalidShareToken(resp) {
  expect(resp.code).to.equal(201001);
  expect(resp.message).to.equal('share token is invalid or expired');
}

function extractId(entity) {
  return entity && (entity.id || entity.ID);
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

describe('device update/delete, alarm delete, and share-token lifecycle [16_device_alarm_share]', function () {
  this.timeout(30000);

  let deviceId = null;
  let seededDevice = null;
  let originalDeviceName = null;
  let recipientAccountKey = null;
  const dynamicRecipientAccounts = [];
  const alarmConfigIdsToCleanup = new Set();
  const deviceIdsToCleanup = new Set();

  async function requireDevice() {
    expect(deviceId).to.be.a('string').and.not.equal('');
    return deviceId;
  }

  async function createAlarmConfig() {
    const createResp = await apiClient.post('/alarm/config', testData.getCreateAlarmConfigReq());
    expect(createResp.code).to.equal(200);
    const alarmConfigId = extractId(createResp.data);
    expect(alarmConfigId).to.be.a('string').and.not.equal('');
    alarmConfigIdsToCleanup.add(alarmConfigId);
    return alarmConfigId;
  }

  async function deleteAlarmConfig(alarmConfigId) {
    const deleteResp = await apiClient.delete('/alarm/config/' + alarmConfigId);
    if (deleteResp.code === 200) {
      alarmConfigIdsToCleanup.delete(alarmConfigId);
    }
    return deleteResp;
  }

  async function createShareToken(expiresIn) {
    if (!deviceId) {
      return null;
    }
    const resp = await apiClient.post(
      '/rdi/devices/' + deviceId + '/share-token',
      testData.getShareTokenReq(expiresIn)
    );
    expect(resp.code).to.equal(200);
    expect(resp.data).to.be.an('object');
    expect(resp.data.token).to.be.a('string').and.not.equal('');
    expect(resp.data.share_path).to.be.a('string').and.include(resp.data.token);
    expect(resp.data.accept_path).to.be.a('string').and.include(resp.data.token);
    expect(resp.data.expires_at).to.be.a('number').and.greaterThan(0);
    return resp.data.token;
  }

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 16_device_alarm_share.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');
    if (await apiClient.isAccountAvailable('tenant_admin_b')) {
      recipientAccountKey = 'tenant_admin_b';
    } else {
      const account = await createTenantAdminAccount(apiClient);
      dynamicRecipientAccounts.push(account);
      recipientAccountKey = account.accountKey;
    }

    seededDevice = await seedData.ensureRdiDevice('tenant_admin');
    deviceId = seededDevice.id;
    originalDeviceName = seededDevice.row.name || seededDevice.row.device_name || null;
    expect(deviceId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    try {
      await apiClient.login('tenant_admin');

      if (deviceId && originalDeviceName) {
        await apiClient.put('/device', {
          id: deviceId,
          name: originalDeviceName
        });
      }

      for (const alarmConfigId of alarmConfigIdsToCleanup) {
        await apiClient.delete('/alarm/config/' + alarmConfigId);
      }

      for (const cleanupDeviceId of deviceIdsToCleanup) {
        await apiClient.delete('/device/' + cleanupDeviceId);
      }

      if (seededDevice && seededDevice.cleanup) {
        await seededDevice.cleanup();
      }

      await cleanupDynamicAccounts(apiClient, dynamicRecipientAccounts);
    } finally {
      apiClient.clearToken('tenant_admin_b');
      if (recipientAccountKey) {
        apiClient.clearToken(recipientAccountKey);
      }
      apiClient.clearAllTokens();
    }
  });

  describe('device update', function () {
    it('updates an existing device name and verifies the detail response', async function () {
      await requireDevice.call(this);
      const newName = 'AutoApiUpdate-' + Date.now();

      const updateResp = await apiClient.put('/device', {
        id: deviceId,
        name: newName
      });
      expect(updateResp.code).to.equal(200);

      const detailResp = await apiClient.get('/device/detail/' + deviceId);
      expect(detailResp.code).to.equal(200);
      expect(detailResp.data).to.be.an('object');
      expect(detailResp.data.name || detailResp.data.device_name).to.equal(newName);
    });

    it('accepts optional device metadata update fields for the existing fixture device', async function () {
      await requireDevice.call(this);
      const updatedDescription = 'auto-description-' + Date.now();
      const updatedLabel = 'auto-test-label';
      const updatedLocation = '121.47,31.23';
      const updateResp = await apiClient.put('/device', {
        id: deviceId,
        description: updatedDescription,
        label: updatedLabel,
        location: updatedLocation
      });
      expect(updateResp.code).to.equal(200);

      // 回读验证：确认 description/label/location 变更已持久化
      const detailResp = await apiClient.get('/device/detail/' + deviceId);
      expect(detailResp.code).to.equal(200);
      expect(detailResp.data).to.be.an('object');
      expect(detailResp.data.id || detailResp.data.ID, '回读必须命中同一设备').to.equal(deviceId);
      expect(detailResp.data, '回读载荷必须包含 description/label/location 字段').to.include.keys(
        'description', 'label', 'location'
      );
      expect(detailResp.data.description, '更新后的 description 必须持久化').to.equal(updatedDescription);
      expect(detailResp.data.label, '更新后的 label 必须持久化').to.equal(updatedLabel);
      expect(detailResp.data.location, '更新后的 location 必须持久化').to.equal(updatedLocation);
    });

    it('rejects update without device id with validation code 100002', async function () {
      const resp = await apiClient.put('/device', {
        name: 'MissingIdUpdate'
      });
      expectValidation(resp, 'Id');
    });

    it('rejects update for a missing UUID with database record-not-found code', async function () {
      const resp = await apiClient.put('/device', {
        id: ZERO_UUID,
        name: 'MissingDeviceUpdate'
      });
      expectRecordNotFound(resp);
    });

    it('rejects overlong device name with max length validation', async function () {
      await requireDevice.call(this);
      const resp = await apiClient.put('/device', {
        id: deviceId,
        name: 'A'.repeat(256)
      });
      expectValidation(resp, "Field 'Name' failed validation (At most 255 characters)");
    });

    it('rejects unauthenticated device update with HTTP 401 and nested 40100', async function () {
      await requireDevice.call(this);
      const resp = await apiClient.putNoAuth('/device', {
        id: deviceId,
        name: 'NoAuthUpdate'
      });
      expectNoAuth(resp);
    });
  });

  describe('device delete', function () {
    it('deletes a disposable activated device when available, otherwise asserts consumed PID boundary', async function () {
      const pid = testData.getDevicePID('inactive_pid');
      const activateResp = await apiClient.post('/rdi/devices/activate', {
        pid_number: pid,
        name: testData.generateDeviceName('DeleteProbe')
      });

      if (activateResp.code === 204001) {
        expect(activateResp.message).to.include(pid);
        return;
      }

      expect(activateResp.code).to.equal(200);
      const disposableDeviceId = extractId(activateResp.data);
      expect(disposableDeviceId).to.be.a('string').and.not.equal('');
      deviceIdsToCleanup.add(disposableDeviceId);

      const deleteResp = await apiClient.delete('/device/' + disposableDeviceId);
      expect(deleteResp.code).to.equal(200);
      deviceIdsToCleanup.delete(disposableDeviceId);

      const secondDeleteResp = await apiClient.delete('/device/' + disposableDeviceId);
      expectRecordNotFound(secondDeleteResp);
    });

    it('rejects deleting a missing UUID with database record-not-found code', async function () {
      const resp = await apiClient.delete('/device/' + ZERO_UUID);
      expectRecordNotFound(resp);
    });

    it('rejects deleting an invalid id string with database record-not-found code', async function () {
      const resp = await apiClient.delete('/device/invalid-id');
      expectRecordNotFound(resp);
    });

    it('rejects unauthenticated device delete with HTTP 401 and nested 40100', async function () {
      const resp = await apiClient.deleteNoAuth('/device/' + ZERO_UUID);
      expectNoAuth(resp);
    });
  });

  describe('alarm config delete', function () {
    it('deletes a created alarm config and verifies it is absent from the list', async function () {
      const alarmConfigId = await createAlarmConfig();

      const deleteResp = await deleteAlarmConfig(alarmConfigId);
      expect(deleteResp.code).to.equal(200);

      const listResp = await apiClient.get('/alarm/config', { page: 1, page_size: 100 });
      expect(listResp.code).to.equal(200);
      expect(listResp.data).to.be.an('object');
      expect(listResp.data.list).to.be.an('array');
      expect(listResp.data.list.some(item => extractId(item) === alarmConfigId)).to.equal(false);
    });

    it('rejects deleting a missing alarm config UUID with record-not-found code', async function () {
      const resp = await apiClient.delete('/alarm/config/' + ZERO_UUID);
      expectRecordNotFound(resp);
    });

    it('rejects deleting an invalid alarm config id string with record-not-found code', async function () {
      const resp = await apiClient.delete('/alarm/config/invalid-id');
      expectRecordNotFound(resp);
    });

    it('rejects unauthenticated alarm config delete with HTTP 401 and nested 40100', async function () {
      const resp = await apiClient.deleteNoAuth('/alarm/config/' + ZERO_UUID);
      expectNoAuth(resp);
    });

    it('rejects deleting the same alarm config twice with record-not-found code', async function () {
      const alarmConfigId = await createAlarmConfig();

      const firstDeleteResp = await deleteAlarmConfig(alarmConfigId);
      expect(firstDeleteResp.code).to.equal(200);

      const secondDeleteResp = await apiClient.delete('/alarm/config/' + alarmConfigId);
      expectRecordNotFound(secondDeleteResp);
    });
  });

  describe('share-token lifecycle', function () {
    it('creates a share token with token, share path, accept path, and future expiry', async function () {
      await requireDevice.call(this);
      const token = await createShareToken();
      expect(token).to.be.a('string').and.not.equal('');
    });

    it('allows public unauthenticated access to a valid shared RDI device snapshot', async function () {
      await requireDevice.call(this);
      const token = await createShareToken();

      const resp = await apiClient.getNoAuth('/rdi/shared/' + token);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data).to.have.property('config');
      expect(resp.data).to.have.property('system_info');
      expect(resp.data).to.have.property('thing_model');
    });

    it('marks owner acceptance of its own share token as already accepted', async function () {
      await requireDevice.call(this);
      const token = await createShareToken();

      const resp = await apiClient.post('/rdi/share-tokens/' + token + '/accept', {}, 'tenant_admin');
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.already_accepted).to.equal(true);
    });

    it('allows recipient tenant to accept and then list the shared device when tenant_admin_b exists', async function () {
      await requireDevice.call(this);
      expect(recipientAccountKey).to.be.a('string').and.not.equal('');

      const token = await createShareToken();
      const acceptResp = await apiClient.post('/rdi/share-tokens/' + token + '/accept', {}, recipientAccountKey);
      expect(acceptResp.code).to.equal(200);
      expect(acceptResp.data).to.be.an('object');
      expect(acceptResp.data.device).to.be.an('object');
      expect(acceptResp.data.accepted_at).to.be.a('number');
      expect(acceptResp.data.already_accepted).to.be.a('boolean');

      const listResp = await apiClient.get('/rdi/shared-with-me/devices', { page: 1, page_size: 20 }, recipientAccountKey);
      expect(listResp.code).to.equal(200);
      expect(listResp.data).to.be.an('object');
      expect(listResp.data.total).to.be.a('number').and.greaterThan(0);
      expect(listResp.data.list).to.be.an('array');
    });

    it('rejects public access with an invalid token using share-token business code 201001', async function () {
      const resp = await apiClient.getNoAuth('/rdi/shared/invalid-share-token-xyz');
      expectInvalidShareToken(resp);
    });

    it('rejects empty public share route with HTTP 404 route miss', async function () {
      const resp = await apiClient.getNoAuth('/rdi/shared/');
      expect(resp.code).to.equal(404);
      expect(String(resp.data)).to.include('404 page not found');
    });

    it('rejects accepting an invalid token using share-token business code 201001', async function () {
      const resp = await apiClient.post('/rdi/share-tokens/invalid-token-value/accept', {}, 'tenant_admin');
      expectInvalidShareToken(resp);
    });

    it('rejects unauthenticated accept of a valid share token with HTTP 401 and nested 40100', async function () {
      await requireDevice.call(this);
      const token = await createShareToken();

      const resp = await apiClient.postNoAuth('/rdi/share-tokens/' + token + '/accept', {});
      expectNoAuth(resp);
    });

    it('rejects creating a share token for a missing device with business code 100404', async function () {
      const resp = await apiClient.post(
        '/rdi/devices/' + ZERO_UUID + '/share-token',
        testData.getShareTokenReq()
      );
      expect(resp.code).to.equal(100404);
      expect(resp.message).to.equal('device not found');
    });

    it('expires a short-lived share token and rejects public access after expiry', async function () {
      await requireDevice.call(this);
      const token = await createShareToken(3);

      const immediateResp = await apiClient.getNoAuth('/rdi/shared/' + token);
      expect(immediateResp.code).to.equal(200);

      await sleep(4100);
      const expiredResp = await apiClient.getNoAuth('/rdi/shared/' + token);
      expectInvalidShareToken(expiredResp);
    });

    it('keeps multiple share tokens for the same device independently valid', async function () {
      await requireDevice.call(this);
      const firstToken = await createShareToken();
      const secondToken = await createShareToken();
      expect(secondToken).to.not.equal(firstToken);

      const firstResp = await apiClient.getNoAuth('/rdi/shared/' + firstToken);
      expect(firstResp.code).to.equal(200);
      expect(firstResp.data).to.be.an('object');
      expect(firstResp.data.device_id).to.equal(deviceId);
      expect(firstResp.data).to.have.property('config');
      expect(firstResp.data).to.have.property('system_info');
      expect(firstResp.data).to.have.property('thing_model');

      const secondResp = await apiClient.getNoAuth('/rdi/shared/' + secondToken);
      expect(secondResp.code).to.equal(200);
      expect(secondResp.data).to.be.an('object');
      expect(secondResp.data.device_id).to.equal(deviceId);
      expect(secondResp.data).to.have.property('config');
      expect(secondResp.data).to.have.property('system_info');
      expect(secondResp.data).to.have.property('thing_model');
    });
  });
});
