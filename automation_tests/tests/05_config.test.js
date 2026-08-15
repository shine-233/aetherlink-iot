/**
 * 文件用途：用于验证配置 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const testData = require('../lib/test_data');
const seedData = require('../lib/seed_data');
const acquireProcessLock = require('../lib/process_lock');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectRejected(resp, expectedCode, expectedMessage) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(expectedCode);
  expect(resp.message).to.equal(expectedMessage);
}

describe('Device config API module [05_config]', function () {
  this.timeout(30000);

  let deviceId = null;
  let originalConfig = null;
  let releaseConfigLock = null;
  let seededDevice = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 05_config.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');
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
      if (seededDevice && seededDevice.cleanup) {
        await seededDevice.cleanup();
      }
    } finally {
      if (releaseConfigLock) {
        await releaseConfigLock();
        releaseConfigLock = null;
      }
    }
  });

  async function updateConfigAndFetch(config, extra = {}) {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const updateResp = await apiClient.put('/rdi/devices/' + deviceId + '/config', {
      config,
      apply_to_device: false,
      ...extra
    });
    expectOk(updateResp);

    const queryResp = await apiClient.get('/rdi/devices/' + deviceId + '/config');
    expectOk(queryResp);
    expect(queryResp.data).to.be.an('object');
    expect(queryResp.data.config).to.be.an('object');
    return queryResp.data;
  }

  it('updates and persists sensor alarm thresholds', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const cfg = testData.getSensorAlarmConfig();
    cfg.sensor_1_upper = 85;
    cfg.sensor_1_lower = -15;
    cfg.sensor_1_duration = 60;

    const data = await updateConfigAndFetch.call(this, cfg);
    expect(data.config.sensor_1_upper).to.equal(85);
    expect(data.config.sensor_1_lower).to.equal(-15);
    expect(data.config.sensor_1_duration).to.equal(60);
  });

  it('updates and persists switch alarm modes', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const cfg = testData.getSensorAlarmConfig();
    cfg.switch_1_alarm_mode = 'powered_off';
    cfg.switch_2_alarm_mode = 'powered_on';

    const data = await updateConfigAndFetch.call(this, cfg);
    expect(data.config.switch_1_alarm_mode).to.equal('powered_off');
    expect(data.config.switch_2_alarm_mode).to.equal('powered_on');
  });

  it('updates and persists dry contact levels and delays', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const cfg = testData.getSensorAlarmConfig();
    cfg.dry_contact_alarm_level = 'low';
    cfg.dry_contact_normal_level = 'high';
    cfg.dry_contact_alarm_delay = 20;
    cfg.dry_contact_normal_delay = 10;

    const data = await updateConfigAndFetch.call(this, cfg);
    expect(data.config.dry_contact_alarm_level).to.equal('low');
    expect(data.config.dry_contact_normal_level).to.equal('high');
    expect(data.config.dry_contact_alarm_delay).to.equal(20);
    expect(data.config.dry_contact_normal_delay).to.equal(10);
  });

  it('updates and persists data collection interval', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const cfg = testData.getSensorAlarmConfig();
    cfg.data_collection_interval = 60;

    const data = await updateConfigAndFetch.call(this, cfg);
    expect(data.config.data_collection_interval).to.equal(60);
  });

  it('updates and persists notification and email fields', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const cfg = testData.getSensorAlarmConfig();
    cfg.notification_enabled = true;
    cfg.notification_temperature_alarm = true;
    cfg.notification_switch_alarm = false;
    cfg.notification_warranty_alarm = true;
    cfg.sensor_alarm_emails = 'sensor1@test.com,sensor2@test.com';
    cfg.switch_alarm_emails = 'switch1@test.com,switch2@test.com';

    const data = await updateConfigAndFetch.call(this, cfg);
    expect(data.config.notification_enabled).to.equal(true);
    expect(data.config.notification_temperature_alarm).to.equal(true);
    expect(data.config.notification_switch_alarm).to.equal(false);
    expect(data.config.notification_warranty_alarm).to.equal(true);
    expect(data.config.sensor_alarm_emails).to.equal('sensor1@test.com,sensor2@test.com');
    expect(data.config.switch_alarm_emails).to.equal('switch1@test.com,switch2@test.com');
  });

  it('accepts system information together with config updates', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const cfg = testData.getSensorAlarmConfig();
    const sysInfo = testData.getSystemInfo();
    const data = await updateConfigAndFetch.call(this, cfg, { system_info: sysInfo });
    expect(data.config.data_collection_interval).to.equal(cfg.data_collection_interval);
    expect(data.system_info.address).to.equal(sysInfo.address);
    expect(data.system_info.installation_date).to.equal(sysInfo.installation_date);
    expect(data.system_info.installer_name).to.equal(sysInfo.installer_name);
    expect(data.system_info.installer_phone).to.equal(sysInfo.installer_phone);
    expect(data.system_info.installer_email).to.equal(sysInfo.installer_email);
    expect(data.system_info.controller_serial_number).to.equal(sysInfo.controller_serial_number);
  });

  it('rejects temperature upper limit above 125C', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.put('/rdi/devices/' + deviceId + '/config', {
      config: testData.getInvalidTempUpperConfig(),
      apply_to_device: false
    });
    expectRejected(resp, 100002, 'sensor_1 limits must be between -40 and 125 C');
  });

  it('rejects temperature lower limit below -40C', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.put('/rdi/devices/' + deviceId + '/config', {
      config: testData.getInvalidTempLowerConfig(),
      apply_to_device: false
    });
    expectRejected(resp, 100002, 'sensor_1 limits must be between -40 and 125 C');
  });

  it('rejects data collection interval below the 45-second minimum', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.put('/rdi/devices/' + deviceId + '/config', {
      config: testData.getInvalidIntervalLowConfig(),
      apply_to_device: false
    });
    // Customer/product contract: RDI sampling interval is constrained to 45-60 seconds.
    expectRejected(resp, 100002, 'data_collection_interval must be between 45 and 60 seconds');
  });

  it('sends the dry contact test command', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.post('/rdi/devices/' + deviceId + '/commands', {
      identifier: 'test_dry_contact',
      params: testData.getTestDryContactParams()
    });
    expectOk(resp);
    expect(resp.data).to.be.an('object');
    // 后端 SendCommand 返回 message_id（非 command_id），并附带 command_tracking 状态对象
    expect(resp.data).to.have.property('message_id').that.is.a('string').and.not.equal('');
    expect(resp.data).to.have.property('identifier').that.equals('test_dry_contact');
    expect(resp.data).to.have.property('device_id').that.equals(deviceId);
    expect(resp.data).to.have.property('status').that.is.a('string').and.not.equal('');
    expect(resp.data).to.have.property('command_tracking').that.is.an('object');
    expect(resp.data.command_tracking).to.have.property('message_id').that.equals(resp.data.message_id);
  });

  it('sends the field setting command', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.post('/rdi/devices/' + deviceId + '/commands', {
      identifier: 'set_field_setting',
      params: testData.getFieldSettingParams()
    });
    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data).to.have.property('message_id').that.is.a('string').and.not.equal('');
    expect(resp.data).to.have.property('identifier').that.equals('set_field_setting');
    expect(resp.data).to.have.property('device_id').that.equals(deviceId);
    expect(resp.data).to.have.property('status').that.is.a('string').and.not.equal('');
    expect(resp.data).to.have.property('command_tracking').that.is.an('object');
    expect(resp.data.command_tracking).to.have.property('message_id').that.equals(resp.data.message_id);
  });

  it('rejects an unknown command identifier', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.post('/rdi/devices/' + deviceId + '/commands', {
      identifier: 'invalid_command',
      params: {}
    });
    expectRejected(resp, 100002, 'unsupported RDI command identifier');
  });
});
