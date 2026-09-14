/**
 * 文件用途：P2.2 基础异常检测端点（POST /telemetry/analysis/anomaly）的 API 契约测试。
 * 核心逻辑：覆盖 bounds / deviation 两种规则的正常路径，以及六条参数与规则校验的拒绝路径。
 * 关键注意事项：
 *   - **"窗口内无数据"必须返回 error='no data in window'，不能返回空 anomalies**。
 *     把"没数据"报成"没异常"是本功能最容易出事的地方，所以这条单独成用例。
 *   - 越权/不可读设备返回 device error='device not readable'，且**不中断多设备检测**——
 *     单设备失败不得让整次请求失败，所以断言写成逐设备 error，而不是 resp.code != 200。
 *   - 时间窗与 window_ms 的包含关系、bounds 的 min/max 完整性、deviation 的 k 正负
 *     都是 service 层显式判定（CodeParamError），不是数据库错误。
 * 重构建议：拿到确定性遥测种子后应补"已知序列 → 已知命中"的数值断言，
 *          当前只能断言结构与语义，无法断言检测数学（纯数学部分由 Go 单测覆盖）。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'Telemetry anomaly detection [40_telemetry_anomaly]';
const ACCOUNT = 'tenant_admin';

const CODE_PARAM_ERROR = 100002;

const HOUR_MS = 60 * 60 * 1000;
const NO_DATA_ERROR = 'no data in window';
const NOT_READABLE_ERROR = 'device not readable';

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  expect(resp.data, 'anomaly result').to.be.an('object');
  return resp.data;
}

function expectRejected(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected ${CODE_PARAM_ERROR} but got ${resp.code}`).to.equal(CODE_PARAM_ERROR);
}

function expectResultShape(data, expected) {
  expect(data.key, 'result.key').to.equal(expected.key);
  expect(data.aggregate, 'result.aggregate').to.be.a('string').and.not.equal('');
  expect(data.window_ms, 'result.window_ms').to.equal(expected.windowMs);
  expect(data.rule, 'result.rule').to.be.an('object');
  expect(data.rule.type, 'result.rule.type').to.equal(expected.ruleType);
  expect(data.devices, 'result.devices').to.be.an('array');
  expect(data.devices.length, 'one result row per requested device').to.equal(expected.deviceCount);
  for (const row of data.devices) {
    expect(row.device_id, 'device row id').to.be.a('string').and.not.equal('');
    expect(row.anomalies, 'device row anomalies').to.be.an('array');
    expect(typeof row.rate, 'device row rate').to.equal('number');
    for (const hit of row.anomalies) {
      expect(hit.index, 'hit.index').to.be.a('number');
      expect(hit.value, 'hit.value').to.be.a('number');
      expect(hit.reason, 'hit.reason').to.be.a('string').and.not.equal('');
    }
  }
}

describe(SUITE, function () {
  this.timeout(60000);

  let deviceId = null;
  const end = Date.now();
  const start = end - 24 * HOUR_MS;
  const windowMs = HOUR_MS;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 40_telemetry_anomaly.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(ACCOUNT);
    // uplinkEnvelope: 本地 broker 没有 gmqtt 的 aetherlink 插件，
    // 不会把扁平上行载荷转成 { device_id, values } 信封，因此这里直接按
    // adapter.verifyPayload 声明的契约发送。在带插件的 gmqtt 上可去掉该选项。
    const seeded = await seedData.ensureDeviceWithTelemetry(ACCOUNT, { uplinkEnvelope: true });
    expect(seeded, 'seeded device').to.be.an('object');
    deviceId = seeded.id;
    expect(deviceId, 'seeded device id').to.be.a('string').and.not.equal('');
  });

  it('accepts a bounds rule and returns one row per device', async function () {
    const data = expectOk(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      aggregate: 'avg',
      rule: { type: 'bounds', min: -1000, max: 1000 }
    }, ACCOUNT));
    expectResultShape(data, {
      key: 'temperature',
      windowMs,
      ruleType: 'bounds',
      deviceCount: 1
    });
  });

  it('accepts a deviation rule and defaults k to 3', async function () {
    const data = expectOk(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      aggregate: 'avg',
      rule: { type: 'deviation' }
    }, ACCOUNT));
    expectResultShape(data, {
      key: 'temperature',
      windowMs,
      ruleType: 'deviation',
      deviceCount: 1
    });
    expect(data.rule.k, 'k must be defaulted to 3').to.equal(3);
  });

  it('reports an empty window as "no data in window" instead of "no anomalies"', async function () {
    // 一个不可能有数据的时间窗：1970 年前后。
    const data = expectOk(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: 1,
      end_time: HOUR_MS,
      window_ms: 60 * 1000,
      aggregate: 'avg',
      rule: { type: 'bounds', min: 0, max: 1 }
    }, ACCOUNT));
    expectResultShape(data, {
      key: 'temperature',
      windowMs: 60 * 1000,
      ruleType: 'bounds',
      deviceCount: 1
    });
    expect(data.devices[0].error, 'an empty window must be an explicit error').to.equal(NO_DATA_ERROR);
    expect(data.devices[0].anomalies.length, 'no anomaly may be fabricated from no data').to.equal(0);
  });

  it('rejects an end_time that is not after start_time', async function () {
    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: end,
      end_time: start,
      window_ms: windowMs,
      rule: { type: 'bounds', min: 0, max: 1 }
    }, ACCOUNT));
  });

  it('rejects a window_ms larger than the query window', async function () {
    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: (end - start) * 2,
      rule: { type: 'bounds', min: 0, max: 1 }
    }, ACCOUNT));
  });

  it('rejects a bounds rule without min and max', async function () {
    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      rule: { type: 'bounds' }
    }, ACCOUNT));
  });

  it('rejects a bounds rule whose min is greater than max', async function () {
    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      rule: { type: 'bounds', min: 10, max: 1 }
    }, ACCOUNT));
  });

  it('rejects a deviation rule with a non-positive k', async function () {
    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      rule: { type: 'deviation', k: 0 }
    }, ACCOUNT));
  });

  it('rejects an unsupported rule type and an unsupported aggregate', async function () {
    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      rule: { type: 'magic' }
    }, ACCOUNT));

    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      aggregate: 'median',
      rule: { type: 'deviation' }
    }, ACCOUNT));
  });

  it('marks unreadable devices per-row without failing the whole request', async function () {
    const data = expectOk(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId, '00000000-0000-0000-0000-000000000000'],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      aggregate: 'avg',
      rule: { type: 'bounds', min: -1000, max: 1000 }
    }, ACCOUNT));
    expect(data.devices.length, 'a single unreadable device must not abort the batch').to.equal(2);
    const foreign = data.devices.find(row => row.device_id === '00000000-0000-0000-0000-000000000000');
    expect(foreign, 'the foreign device row').to.be.an('object');
    expect(foreign.error, 'a foreign device must be reported as not readable').to.equal(NOT_READABLE_ERROR);
  });

  it('rejects an empty device list and a missing key', async function () {
    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [],
      key: 'temperature',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      rule: { type: 'deviation' }
    }, ACCOUNT));

    expectRejected(await apiClient.post('/telemetry/analysis/anomaly', {
      device_ids: [deviceId],
      key: '',
      start_time: start,
      end_time: end,
      window_ms: windowMs,
      rule: { type: 'deviation' }
    }, ACCOUNT));
  });
});
