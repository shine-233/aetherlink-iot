/**
 * 文件用途：用于验证遥测边界 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const testData = require('../lib/test_data');
const seedData = require('../lib/seed_data');
const { skipIfBlocked } = require('../lib/integration_blocked');
const { expectArray, expectPagedList } = require('../lib/response_assertions');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

describe('Telemetry extra API module [12_telemetry_extra]', function () {
  this.timeout(30000);

  let deviceId = null;
  let seededDevice = null;
  let mqttAvailable = false;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 12_telemetry_extra.test.js; unified verification requires a healthy API service');
    }

    mqttAvailable = await seedData.isMqttBrokerAvailable();
    await apiClient.login('tenant_admin');
    seededDevice = await seedData.ensureDevice('tenant_admin');
    deviceId = seededDevice.id;
    expect(deviceId).to.be.a('string').and.not.equal('');
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

  it('returns the current telemetry detail row for the tenant device', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');
    if (!mqttAvailable) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is not available on ' + seedData.mqttEndpointDescription() + '; current telemetry detail requires a real telemetry sample',
        category: 'runtime-external',
        seedable: false
      });
    }
    const telemetrySeed = await seedData.ensureDeviceWithTelemetry('tenant_admin');
    expect(telemetrySeed.telemetrySeeded, JSON.stringify(telemetrySeed.telemetrySeedResponse)).to.equal(true);

    const resp = await apiClient.get('/telemetry/datas/current/detail/' + deviceId, {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.device_id).to.equal(deviceId);
    expect(resp.data.key).to.be.a('string').and.not.empty;
    expect(resp.data).to.include.keys('tenant_id', 'ts', 'value');
  });

  it('returns the current local history pagination shape', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    // current detail is a latest-row diagnostic endpoint without a key
    // selector; verify the sent key/value through the keyed current list.
    const currentResp = await apiClient.get(
      '/telemetry/datas/current/' + deviceId,
      {},
      'tenant_admin'
    );
    expectOk(currentResp);
    expect(currentResp.data).to.be.an('array');
    currentResp.data.forEach(row => {
      expect(row).to.be.an('object');
      expect(row.device_id).to.equal(deviceId);
      expect(row.key).to.be.a('string').and.not.empty;
    });

    const range = testData.getHistoryTimeRange();
    const request = {
      device_id: deviceId,
      key: 'temperature_1',
      start_time: range.startTime,
      end_time: range.endTime,
      page: 1,
      page_size: 10
    };
    const resp = await apiClient.get(
      '/telemetry/datas/history/pagination',
      request,
      'tenant_admin'
    );

    expectOk(resp);
    expectPagedList(resp.data, {
      rowCheck: row => {
        expect(row.key).to.equal('temperature_1');
        expect(row.ts).to.be.a('number');
        expect(row.value).to.not.equal(undefined);
      }
    });

    const pageResp = await apiClient.get(
      '/telemetry/datas/history/page',
      request,
      'tenant_admin'
    );
    expectOk(pageResp);
    expectPagedList(pageResp.data, {
      rowCheck: row => {
        expect(row.key).to.equal('temperature_1');
        expect(row.ts).to.be.a('number');
        expect(row.value).to.not.equal(undefined);
      }
    });
  });

  it('returns telemetry set logs with count and list fields', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get(
      '/telemetry/datas/set/logs',
      {
        device_id: deviceId,
        page: 1,
        page_size: 10
      },
      'tenant_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.count).to.be.a('number');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.list.length).to.be.at.most(resp.data.count);
    resp.data.list.forEach(item => {
      expect(item).to.be.an('object');
      expect(item.device_id).to.equal(deviceId);
    });
  });

  it('returns batch statistic points for the tenant device', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get(
      '/telemetry/datas/statistic/batch',
      {
        device_ids: [deviceId],
        keys: ['temperature_1'],
        time_type: 'hour',
        aggregate_method: 'avg',
        limit: 24
      },
      'tenant_admin'
    );

    expectOk(resp);
    expectArray(resp.data, {
      rowCheck: row => {
        expect(row).to.include.keys('key', 'time', 'value');
        expect(row.key).to.equal('temperature_1');
      }
    });
  });

  it('returns simulation connection bootstrap data', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/telemetry/datas/simulation/init', { device_id: deviceId }, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data).to.include.keys('username', 'password', 'client_id', 'server', 'port', 'topic');
    expect(resp.data.topic_options).to.be.an('array');
  });

  it('accepts simulated telemetry send for the tenant device', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');
    if (!mqttAvailable) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is not available on ' + seedData.mqttEndpointDescription() + '; simulation send requires a real broker',
        category: 'runtime-external',
        seedable: false
      });
    }
    const telemetrySeed = await seedData.ensureDeviceWithTelemetry('tenant_admin');
    expect(telemetrySeed.telemetrySeeded, JSON.stringify(telemetrySeed.telemetrySeedResponse)).to.equal(true);

    const payload = { temperature_1: 25.5 };
    const resp = await apiClient.post(
      '/telemetry/datas/simulation/send',
      {
        device_id: deviceId,
        data: JSON.stringify(payload)
      },
      'tenant_admin'
    );

    expectOk(resp);

    // 回读验证：确认刚发送的模拟遥测载荷已持久化到 current detail
    const readbackResp = await apiClient.get(
      '/telemetry/datas/current/detail/' + deviceId,
      {},
      'tenant_admin'
    );
    expectOk(readbackResp);
    expect(readbackResp.data).to.be.an('object');
    expect(readbackResp.data.device_id).to.equal(deviceId);
    // current detail 暴露 {key, value} 结构；先宽松校验键存在，再做具体值断言
    expect(readbackResp.data, '回读载荷必须包含 key 与 value 字段').to.include.keys('key', 'value');
    expect(
      readbackResp.data.key,
      '回读的 current detail key 必须反映刚发送的模拟键'
    ).to.be.a('string').and.not.equal('');
    const currentResp = await apiClient.get(
      '/telemetry/datas/current/' + deviceId,
      {},
      'tenant_admin'
    );
    expectOk(currentResp);
    expect(currentResp.data).to.be.an('array');
    const currentRow = currentResp.data.find(row => row && row.key === 'temperature_1');
    expect(currentRow, 'keyed current data must expose the sent telemetry key').to.be.an('object');
    const sentValue = currentRow.value;
    expect(sentValue, '回读必须暴露发送的载荷值').to.not.equal(undefined);
    // value 可能是裸数字或 JSON 字符串；两种形态都接受
    const parsed = typeof sentValue === 'string' ? Number(JSON.parse(sentValue)) : Number(sentValue);
    expect(
      parsed,
      '发送的模拟值必须持久化在 current detail 中, 实际得到: ' + JSON.stringify(sentValue)
    ).to.equal(25.5);
  });
});

// 负向校验测试不依赖 MQTT seeded telemetry，独立 describe 块确保 MQTT 不可用时仍能运行
describe('Telemetry extra API negative validation [12_telemetry_extra]', function () {
  this.timeout(30000);

  const fakeId = '00000000-0000-0000-0000-000000000000';

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 12_telemetry_extra.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('rejects telemetry delete when the previous keys field is sent instead of key', async function () {
    const resp = await apiClient.delete(
      '/telemetry/datas',
      {
        device_id: fakeId,
        keys: 'nonexistent_key'
      },
      'tenant_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100002);
    expect(resp.message).to.equal("Field 'Key' is required");
  });
});
