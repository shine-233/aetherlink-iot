/**
 * 文件用途：用于验证聚焦 API 写入流程测试。
 * 核心逻辑：使用确定性本地夹具执行 API 场景，断言响应、状态变化、负向分支和清理结果。
 * 关键注意事项：只有在本地账号、种子数据和清理步骤都成功时，才可作为对应流程的业务闭环证据。
 * 重构建议：继续把数据准备、断言 oracle 和清理逻辑拆清楚，便于补充故障注入或变异验证。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const testData = require('../lib/test_data');
const seedData = require('../lib/seed_data');
const acquireProcessLock = require('../lib/process_lock');
const {
  createTenantAdminAccount,
  cleanupDynamicAccounts
} = require('./helpers/dynamic_accounts');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectRejected(resp, expectedCode, expectedMessage) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(expectedCode);
  expect(resp.message).to.equal(expectedMessage);
}

function pickId(record) {
  return record && (record.id || record.ID) ? (record.id || record.ID) : null;
}

function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

describe('Write-flow API module [16_write_flows]', function () {
  this.timeout(30000);

  let deviceId = null;
  let originalConfig = null;
  let releaseConfigLock = null;
  let seededDevice = null;
  const alarmConfigIds = [];
  const dynamicRecipientAccounts = [];
  let recipientAccountKey = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 16_write_flows.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');
    if (await apiClient.isAccountAvailable('tenant_admin_b')) {
      recipientAccountKey = 'tenant_admin_b';
    } else {
      const account = await createTenantAdminAccount(apiClient);
      dynamicRecipientAccounts.push(account);
      recipientAccountKey = account.accountKey;
    }
    releaseConfigLock = await acquireProcessLock('rdi-device-config-write');

    seededDevice = await seedData.ensureRdiDevice('tenant_admin');
    deviceId = seededDevice.id;
    expect(deviceId).to.be.a('string').and.not.equal('');

    if (deviceId) {
      const cfgResp = await apiClient.get('/rdi/devices/' + deviceId + '/config');
      if (cfgResp.code === 200 && cfgResp.data && cfgResp.data.config) {
        originalConfig = cfgResp.data.config;
      }
    }
  });

  after(async function () {
    try {
      if (deviceId && originalConfig) {
        await apiClient.put('/rdi/devices/' + deviceId + '/config', {
          config: originalConfig,
          apply_to_device: false
        });
      }

      for (const id of alarmConfigIds) {
        try {
          await apiClient.delete('/alarm/config/' + id);
        } catch (error) {
          // Cleanup failure should not mask the test verdict.
        }
      }
      if (seededDevice && seededDevice.cleanup) {
        await seededDevice.cleanup();
      }
    } finally {
      if (releaseConfigLock) {
        await releaseConfigLock();
        releaseConfigLock = null;
      }
      await cleanupDynamicAccounts(apiClient, dynamicRecipientAccounts);
      apiClient.clearToken('tenant_admin_b');
      if (recipientAccountKey) {
        apiClient.clearToken(recipientAccountKey);
      }
    }
  });

  async function updateConfigAndFetch(config) {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const updateResp = await apiClient.put('/rdi/devices/' + deviceId + '/config', {
      config,
      apply_to_device: false
    });
    expectOk(updateResp);

    const queryResp = await apiClient.get('/rdi/devices/' + deviceId + '/config');
    expectOk(queryResp);
    expect(queryResp.data).to.be.an('object');
    expect(queryResp.data.config).to.be.an('object');
    return queryResp.data.config;
  }

  async function waitForSharedDevice(accountKey, targetDeviceId) {
    let lastResp = null;

    for (let attempt = 0; attempt < 8; attempt += 1) {
      const resp = await apiClient.get(
        '/rdi/shared-with-me/devices',
        { page: 1, page_size: 20, device_id: targetDeviceId },
        accountKey
      );
      expectOk(resp);
      lastResp = resp;

      const data = resp.data || {};
      const list = Array.isArray(data.list) ? data.list : [];
      const matched = list.find(item => item && item.device && item.device.device_id === targetDeviceId);
      if (matched) {
        return data;
      }

      await delay(500);
    }

    expect(lastResp && lastResp.data).to.be.an('object');
    expect(lastResp.data.total).to.be.greaterThan(0);
    expect(lastResp.data.list).to.be.an('array').and.not.empty;
    const matched = lastResp.data.list.find(item => item && item.device && item.device.device_id === targetDeviceId);
    expect(matched, 'shared-with-me list must include the accepted target device').to.be.an('object');
    return lastResp.data;
  }

  it('persists a combined RDI config write', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const cfg = testData.getSensorAlarmConfig();
    cfg.data_collection_interval = 45;
    cfg.sensor_1_upper = 95;
    cfg.sensor_1_lower = -20;
    cfg.sensor_alarm_emails = 'write-flow-test@example.com';

    const savedConfig = await updateConfigAndFetch.call(this, cfg);
    expect(savedConfig.data_collection_interval).to.equal(45);
    expect(savedConfig.sensor_1_upper).to.equal(95);
    expect(savedConfig.sensor_1_lower).to.equal(-20);
    expect(savedConfig.sensor_alarm_emails).to.equal('write-flow-test@example.com');
  });

  it('creates an alarm config and returns a durable id', async function () {
    const resp = await apiClient.post('/alarm/config', testData.getCreateAlarmConfigReq());
    expectOk(resp);
    expect(resp.data).to.be.an('object');

    const id = pickId(resp.data);
    expect(id).to.be.a('string').and.not.equal('');
    alarmConfigIds.push(id);
  });

  it('finds a created alarm config in the paged list', async function () {
    const alarmName = 'write_flow_alarm_' + Date.now();
    const createResp = await apiClient.post('/alarm/config', {
      name: alarmName,
      description: 'write flow list verification',
      alarm_level: 'M',
      enabled: 'Y',
      remark: 'write-flow-test'
    });
    expectOk(createResp);

    const id = pickId(createResp.data);
    expect(id).to.be.a('string').and.not.equal('');
    alarmConfigIds.push(id);

    const listResp = await apiClient.get('/alarm/config', { page: 1, page_size: 100 });
    expectOk(listResp);
    const list = Array.isArray(listResp.data && listResp.data.list) ? listResp.data.list : [];
    const found = list.find(item => pickId(item) === id);
    expect(found).to.be.an('object');
    expect(found.name).to.equal(alarmName);
    expect(found.alarm_level).to.equal('M');
    expect(found.enabled).to.equal('Y');
  });

  it('updates an alarm config and verifies the updated list value', async function () {
    const createResp = await apiClient.post('/alarm/config', {
      name: 'write_flow_update_' + Date.now(),
      description: 'before update',
      alarm_level: 'L',
      enabled: 'Y',
      remark: 'write-flow-update'
    });
    expectOk(createResp);

    const id = pickId(createResp.data);
    expect(id).to.be.a('string').and.not.equal('');
    alarmConfigIds.push(id);

    const updatedName = 'write_flow_updated_' + Date.now();
    const updateResp = await apiClient.put('/alarm/config', {
      id,
      name: updatedName,
      enabled: 'N'
    });
    expectOk(updateResp);

    const listResp = await apiClient.get('/alarm/config', { page: 1, page_size: 100 });
    expectOk(listResp);
    const list = Array.isArray(listResp.data && listResp.data.list) ? listResp.data.list : [];
    const found = list.find(item => pickId(item) === id);
    expect(found).to.be.an('object');
    expect(found.name).to.equal(updatedName);
    expect(found.enabled).to.equal('N');
  });

  it('deletes an alarm config and verifies it is absent from the list', async function () {
    const createResp = await apiClient.post('/alarm/config', {
      name: 'write_flow_delete_' + Date.now(),
      description: 'to delete',
      alarm_level: 'H',
      enabled: 'Y',
      remark: 'write-flow-delete'
    });
    expectOk(createResp);

    const id = pickId(createResp.data);
    expect(id).to.be.a('string').and.not.equal('');

    const deleteResp = await apiClient.delete('/alarm/config/' + id);
    expectOk(deleteResp);

    const listResp = await apiClient.get('/alarm/config', { page: 1, page_size: 100 });
    expectOk(listResp);
    const list = Array.isArray(listResp.data && listResp.data.list) ? listResp.data.list : [];
    expect(list.find(item => pickId(item) === id)).to.equal(undefined);
  });

  it('creates a share token and exposes the public shared payload', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const shareResp = await apiClient.post('/rdi/devices/' + deviceId + '/share-token', {
      expires_in: 7 * 24 * 60 * 60
    });
    expectOk(shareResp);
    expect(shareResp.data).to.be.an('object');
    expect(shareResp.data.token).to.be.a('string').and.not.equal('');
    expect(shareResp.data.share_path).to.be.a('string').and.include('/device/share?share_token=');
    expect(shareResp.data.expires_at).to.be.a('number');
    expect(shareResp.data.expires_at).to.be.greaterThan(Math.floor(Date.now() / 1000));

    const publicResp = await apiClient.getNoAuth('/rdi/shared/' + shareResp.data.token);
    expectOk(publicResp);
    expect(publicResp.data).to.be.an('object');
    expect(publicResp.data.device_id).to.equal(deviceId);
    expect(publicResp.data.config).to.be.an('object');
    expect(publicResp.data.system_info).to.be.an('object');
    expect(publicResp.data.thing_model).to.be.an('object');
  });

  it('rejects an invalid share token', async function () {
    const resp = await apiClient.getNoAuth('/rdi/shared/not-a-real-share-token');
    expectRejected(resp, 201001, 'share token is invalid or expired');
  });

  it('accepts a share token into shared-with-me when the recipient tenant exists', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');
    expect(recipientAccountKey).to.be.a('string').and.not.equal('');

    const shareResp = await apiClient.post('/rdi/devices/' + deviceId + '/share-token', {
      expires_in: 7 * 24 * 60 * 60
    });
    expectOk(shareResp);

    const acceptResp = await apiClient.post('/rdi/share-tokens/' + shareResp.data.token + '/accept', {}, recipientAccountKey);
    expectOk(acceptResp);
    expect(acceptResp.data).to.be.an('object');
    expect(acceptResp.data.device).to.be.an('object');
    expect(acceptResp.data.device.device_id).to.equal(deviceId);
    expect(acceptResp.data.accepted_at).to.be.a('number');
    expect(acceptResp.data.already_accepted).to.be.a('boolean');

    const sharedListData = await waitForSharedDevice(recipientAccountKey, deviceId);
    expect(sharedListData.total).to.be.greaterThan(0);
    expect(sharedListData.list).to.be.an('array').and.not.empty;
  });

  it('marks repeated share-token acceptance as already accepted', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');
    expect(recipientAccountKey).to.be.a('string').and.not.equal('');

    const shareResp = await apiClient.post('/rdi/devices/' + deviceId + '/share-token', {
      expires_in: 7 * 24 * 60 * 60
    });
    expectOk(shareResp);

    await apiClient.post('/rdi/share-tokens/' + shareResp.data.token + '/accept', {}, recipientAccountKey);
    const secondAcceptResp = await apiClient.post(
      '/rdi/share-tokens/' + shareResp.data.token + '/accept',
      {},
      recipientAccountKey
    );
    expectOk(secondAcceptResp);
    expect(secondAcceptResp.data.already_accepted).to.equal(true);
  });
});
