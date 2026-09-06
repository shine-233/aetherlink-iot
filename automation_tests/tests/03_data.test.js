/**
 * 文件用途：用于验证遥测数据 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const testData = require('../lib/test_data');
const seedData = require('../lib/seed_data');
const { skipIfBlocked } = require('../lib/integration_blocked');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectDeviceTelemetryRow(row, deviceId) {
  expect(row).to.be.an('object');
  expect(row.device_id).to.equal(deviceId);
  expect(row.key).to.be.a('string');
  expect(row.tenant_id).to.be.a('string');
  expect(row.value).to.not.equal(undefined);
}

function expectTelemetryRowsForKey(rows, deviceId, key) {
  expect(rows).to.be.an('array');
  expect(rows.length, `seeded telemetry rows for ${key}`).to.be.greaterThan(0);
  rows.forEach(row => {
    expectDeviceTelemetryRow(row, deviceId);
    expect(row.key).to.equal(key);
    expect(row.ts).to.be.a('number');
  });
}

function expectHistoryPageWithSeededRows(data, deviceId, key) {
  expect(data).to.be.an('object');
  expect(data).to.have.property('total').that.is.a('number').and.is.greaterThan(0);
  expect(data).to.have.property('list').that.is.an('array');
  expect(data.list.length, `seeded paged telemetry rows for ${key}`).to.be.greaterThan(0);
  expect(data.list.length).to.be.at.most(data.total);
  // The paged history endpoints already filter by device_id and intentionally
  // return the compact { key, ts, value } row contract.  They do not echo the
  // request's device_id or tenant_id on every row.
  data.list.forEach(row => {
    expect(row).to.be.an('object');
    expect(row.key).to.equal(key);
    expect(row.ts).to.be.a('number');
    expect(row.value).to.not.equal(undefined);
  });
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

describe('Data query API module [03_data]', function () {
  this.timeout(30000);

  let deviceId = null;
  let seededDevice = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 03_data.test.js; unified verification requires a healthy API service');
    }

    const mqttAvailable = await seedData.isMqttBrokerAvailable();
    if (!mqttAvailable) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is not available on ' + seedData.mqttEndpointDescription() + '; telemetry-dependent tests require a live broker',
        category: 'runtime-external',
        seedable: false
      });
    }

    await apiClient.login('tenant_admin');
    seededDevice = await seedData.ensureDeviceWithTelemetry('tenant_admin');
    deviceId = seededDevice.id;
    expect(deviceId).to.be.a('string').and.not.equal('');
    expect(seededDevice.telemetrySeeded, JSON.stringify(seededDevice.telemetrySeedResponse)).to.equal(true);
  });

  after(async function () {
    try {
      if (seededDevice && seededDevice.cleanup) {
        await seededDevice.cleanup();
      }
    } finally {
      apiClient.clearAllTokens();
    }
  });

  it('returns current telemetry rows for the selected device', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/telemetry/datas/current/' + deviceId);

    expectOk(resp);
    expect(resp.data).to.be.an('array');
    expect(resp.data.length).to.be.greaterThan(0);
    expectCurrentRowsForKeys(resp.data, deviceId, ['temperature_1']);
  });

  it('returns history rows for a concrete telemetry key', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const range = testData.getHistoryTimeRange();
    const resp = await apiClient.get('/telemetry/datas/history', {
      device_id: deviceId,
      key: 'temperature_1',
      start_time: range.startTime,
      end_time: range.endTime
    });

    expectOk(resp);
    expectTelemetryRowsForKey(resp.data, deviceId, 'temperature_1');
  });

  it('returns the current local paged history shape even when no rows are present', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const range = testData.getHistoryTimeRange();
    const resp = await apiClient.get('/telemetry/datas/history/page', {
      device_id: deviceId,
      key: 'temperature_1',
      start_time: range.startTime,
      end_time: range.endTime,
      page: 1,
      page_size: 10
    });

    expectOk(resp);
    expectHistoryPageWithSeededRows(resp.data, deviceId, 'temperature_1');
  });

  it('returns the current local RDI history page shape', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const range = testData.getHistoryTimeRange();
    const resp = await apiClient.get('/rdi/devices/' + deviceId + '/history', {
      key: 'temperature_1',
      start_time: range.startTime,
      end_time: range.endTime
    });

    expectOk(resp);
    expectHistoryPageWithSeededRows(resp.data, deviceId, 'temperature_1');
  });

  it('creates a CSV export task for recent RDI history', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const range = testData.getRecentTimeRange();
    const resp = await apiClient.get('/rdi/devices/' + deviceId + '/history', {
      key: 'temperature_1',
      start_time: range.startTime,
      end_time: range.endTime,
      export_excel: true,
      export_format: 'csv'
    });

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.fileType).to.equal('csv');
    expect(resp.data.fileName).to.be.a('string').and.include('.csv');
    expect(resp.data.filePath).to.be.a('string').and.include('files/excel/');

    // 真实下载验证：导出产物必须可通过 /files 静态通道取回且非空。
    const fileResp = await apiClient.getRootNoAuth('/' + String(resp.data.filePath).replace(/^\/+/, ''));
    expect(fileResp.httpStatus, 'exported csv must be downloadable').to.equal(200);
    const csvText = String(fileResp.data);
    expect(csvText.length).to.be.greaterThan(0);
  });

  it('creates an Excel export task for recent RDI history', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const range = testData.getRecentTimeRange();
    const resp = await apiClient.get('/rdi/devices/' + deviceId + '/history', {
      key: 'temperature_1',
      start_time: range.startTime,
      end_time: range.endTime,
      export_excel: true,
      export_format: 'excel'
    });

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.fileType).to.equal('excel');
    expect(resp.data.fileName).to.be.a('string').and.include('.xlsx');
    expect(resp.data.filePath).to.be.a('string').and.include('files/excel/');

    // 真实下载验证：xlsx 是 zip 容器，产物必须可取回、非空且带 PK 魔数。
    const fileResp = await apiClient.getRootNoAuth('/' + String(resp.data.filePath).replace(/^\/+/, ''));
    expect(fileResp.httpStatus, 'exported xlsx must be downloadable').to.equal(200);
    expect(String(fileResp.data).length).to.be.greaterThan(0);
    expect(String(fileResp.data).substring(0, 2), 'xlsx must start with the zip PK magic').to.equal('PK');
  });

  it('returns statistic points for the selected telemetry key', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/telemetry/datas/statistic', {
      device_id: deviceId,
      key: 'temperature_1',
      time_range: 'last_24h',
      aggregate_window: 'no_aggregate'
    });

    expectOk(resp);
    expect(resp.data).to.be.an('array');
    expect(resp.data.length, 'seeded statistic points').to.be.greaterThan(0);
    resp.data.forEach(point => {
      expect(point).to.include.keys('x', 'y');
    });
  });

  it('returns device status history with list and total fields', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/device/status/history', {
      device_id: deviceId,
      page: 1,
      page_size: 10
    });

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.list.length).to.be.at.most(resp.data.total);
    resp.data.list.forEach(row => {
      expect(row.device_id).to.equal(deviceId);
      expect(row).to.include.keys('status', 'change_time');
    });
  });

  it('returns the current key-filtered telemetry payload as an array, even when empty', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/telemetry/datas/current/keys', {
      device_id: deviceId,
      // Gin's query binder maps repeated `keys` parameters to []string.  A
      // comma-delimited scalar would be treated as one literal key and can
      // legitimately return an empty result.
      keys: ['temperature_1', 'temperature_2']
    });

    expectOk(resp);
    expectCurrentRowsForKeys(resp.data, deviceId, ['temperature_1', 'temperature_2']);
  });
});

// 负向校验测试不依赖 MQTT seeded telemetry，独立 describe 块确保 MQTT 不可用时仍能运行。
// 缺失 DataType 字段的校验在设备查询之前发生，可用任意合法 UUID 格式触发。
describe('Data query API negative validation [03_data]', function () {
  this.timeout(30000);

  const fakeDeviceId = '00000000-0000-0000-0000-000000000000';

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 03_data.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('rejects metrics chart queries that omit the current required DataType field', async function () {
    const range = testData.getHistoryTimeRange();
    const resp = await apiClient.get('/device/metrics/chart', {
      device_id: fakeDeviceId,
      key: 'temperature_1',
      start_time: range.startTime,
      end_time: range.endTime
    });

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100002);
    expect(resp.message).to.be.a('string').and.include('DataType');
  });
});
