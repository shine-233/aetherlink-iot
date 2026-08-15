/**
 * 文件用途：用于验证MQTT 设备管线 API 侧证据测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const { skipIfBlocked } = require('../lib/integration_blocked');
const {
  expectArray,
  expectBusinessError,
  expectSuccess,
  expectValidationError
} = require('../lib/response_assertions');

function expectDeviceTelemetryRow(row, deviceId) {
  expect(row).to.be.an('object');
  expect(row.device_id).to.equal(deviceId);
  expect(row.key).to.be.a('string');
  expect(row.tenant_id).to.be.a('string');
  expect(row.value).to.not.equal(undefined);
}

function expectCurrentRowsForKeys(rows, deviceId, keys) {
  expect(rows).to.be.an('array');
  expect(rows.length, 'seeded current telemetry rows').to.be.at.least(keys.length);
  keys.forEach(key => {
    const row = rows.find(item => item && item.key === key);
    expect(row, `current telemetry row for ${key}`).to.be.an('object');
    expectDeviceTelemetryRow(row, deviceId);
    expect(row.key).to.equal(key);
  });
}

function expectConnectInfoPayload(payload) {
  expect(payload).to.be.an('object');
  const values = Object.values(payload).filter(value => value !== undefined && value !== null && value !== '');
  expect(values, 'connect info should expose endpoint, username, topic, or control-topic values').to.not.be.empty;
  expect(
    values.some(value => /mqtt_|telemetry|1883|control/i.test(String(value))),
    'connect info should include MQTT username, topic, control topic, or broker endpoint evidence'
  ).to.equal(true);
}

describe('MQTT device pipeline API coverage [22_mqtt_device_pipeline]', function () {
  this.timeout(45000);

  before(async function () {
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('validates device topic mapping list and mutation boundaries', async function () {
    const listResp = await apiClient.get('/device/topic-mappings', { page: 1, page_size: 10 }, 'tenant_admin');
    if (listResp.code === 200) {
      expectSuccess(listResp);
      expectArray(listResp.data && (listResp.data.list || listResp.data), {
        rowCheck: row => {
          expect(row).to.be.an('object');
          expect(row).to.include.keys(['name', 'direction', 'source_topic', 'target_topic']);
          expect(row.name).to.be.a('string').and.not.equal('');
          expect(row.direction).to.be.oneOf(['up', 'down']);
          expect(row.source_topic).to.be.a('string').and.not.equal('');
          expect(row.target_topic).to.be.a('string').and.not.equal('');
        }
      });
    } else {
      expectValidationError(listResp, 'DeviceConfigID');
    }

    expectValidationError(await apiClient.post('/device/topic-mappings', {}, 'tenant_admin'));
  });

  it('validates backend publish/simulation API request shape for MQTT uplink and downlink', async function () {
    expectValidationError(await apiClient.post('/telemetry/datas/simulation', {}, 'tenant_admin'), 'Command');
    expectValidationError(await apiClient.get('/telemetry/datas/simulation', {}, 'tenant_admin'), 'DeviceId');
    expectValidationError(await apiClient.post('/telemetry/datas/pub', {}, 'tenant_admin'), 'DeviceID');
    expectValidationError(await apiClient.post('/attribute/datas/pub', {}, 'tenant_admin'), 'DeviceID');
    expectValidationError(await apiClient.post('/command/datas/pub', {}, 'tenant_admin'), 'DeviceID');
    expectValidationError(await apiClient.post('/command/datas/direct-method', {}, 'tenant_admin'), 'DeviceID');
  });

  it('ties seeded device identity to connect info and debug endpoints', async function () {
    const mqttAvailable = await seedData.isMqttBrokerAvailable();
    if (!mqttAvailable) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is not available on ' + seedData.mqttEndpointDescription() + '; telemetry-dependent test requires a live broker',
        category: 'runtime-external',
        seedable: false
      });
    }
    const seededDevice = await seedData.createSimulationDevice('tenant_admin');
    try {
      expect(seededDevice.id).to.be.a('string').and.not.equal('');
      const telemetryResult = await seedData.publishSimulatedTelemetryAndReadCurrent(
        seededDevice.id,
        { temperature_1: 25.5 },
        'tenant_admin'
      );
      expect(telemetryResult.rows).to.have.length(1);

      const connectResp = await apiClient.get('/device/connect/info', { device_id: seededDevice.id }, 'tenant_admin');
      // This is a seeded, valid device path; a validation error would mask a
      // broken connect-info implementation and must fail the test.
      expectSuccess(connectResp);
      expectConnectInfoPayload(connectResp.data);

      const currentTelemetryResp = await apiClient.get('/telemetry/datas/current/' + seededDevice.id, {}, 'tenant_admin');
      expectSuccess(currentTelemetryResp);
      expectCurrentRowsForKeys(currentTelemetryResp.data, seededDevice.id, ['temperature_1']);

      const debugResp = await apiClient.post('/device/' + seededDevice.id + '/debug', {}, 'tenant_admin');
      expectSuccess(debugResp);
    } finally {
      await seededDevice.cleanup();
    }
  });

  it('publishes unique simulated telemetry and reads the same current value back', async function () {
    const mqttAvailable = await seedData.isMqttBrokerAvailable();
    if (!mqttAvailable) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is not available on ' + seedData.mqttEndpointDescription() + '; MQTT-dependent test requires a live broker',
        category: 'runtime-external',
        seedable: false
      });
    }
    const seededDevice = await seedData.createSimulationDevice('tenant_admin');
    const telemetryKey = 'codex_mqtt_' + Date.now();
    const telemetryValue = 'mqtt-pipeline-' + Date.now();

    try {
      const result = await seedData.publishSimulatedTelemetryAndReadCurrent(
        seededDevice.id,
        { [telemetryKey]: telemetryValue },
        'tenant_admin'
      );

      expect(result.publishResp.code).to.equal(200);
      expect(result.rows).to.have.length(1);
      const row = result.rows[0];
      expectDeviceTelemetryRow(row, seededDevice.id);
      expect(row.key).to.equal(telemetryKey);
      expect(String(row.value)).to.equal(telemetryValue);
    } finally {
      await seededDevice.cleanup();
    }
  });
});
