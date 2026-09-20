/**
 * 文件用途：P2.2 轻量分析核心端点（POST /telemetry/analysis 与 /export）的 API 契约测试。
 *
 * 背景：anomaly 端点已有 40 号用例，但分析查询与 CSV/Excel 导出此前只有 Go 单测、
 * 没有任何活栈运行期证据——P2.2 在路线图里长期停在 partial 的真实原因。
 *
 * 关键注意事项：
 *   - **线契约为 snake_case**：`current` 内层是 `{value, ok, aggregate}`。
 *     这三个 tag 是本次随测试一起补的（此前 Go 字段名 Value/OK/Aggregate 会原样
 *     序列化）；55 号用例里 `devResult.current.avg` 的软断言因此从未生效过，
 *     本文件用严格断言锁死真实形状，防止再出现"看着像断言过"的假绿。
 *   - **"没有可计算的值"与"值为 0"是两种事实**：基线窗口无数据时
 *     `percent_change` 必须缺位且带 reason，绝不允许出现 0%。
 *   - 种子遥测是单点数据（25.5），聚合值可精确断言；窗口在播种**之后**计算，
 *     否则数据点会落在窗口之外——这是时序敏感点，不要把 end 提前到播种前。
 *   - 导出端点返回服务端文件路径（`files/export/`，已 gitignore），不是文件字节流；
 *     断言 file_path 与 format 即可，文件本体由 ExportTelemetryAnalysis 单测覆盖。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const { expectSuccess, expectBusinessError } = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');

const SUITE = 'P2.2 telemetry analysis query & export [60_telemetry_analysis]';
const ACCOUNT = 'tenant_admin';

const CODE_PARAM_ERROR = 100002;
const HOUR_MS = 60 * 60 * 1000;

describe(SUITE, function () {
  this.timeout(120000);

  let deviceId = null;
  let seed = null;
  let start = 0;
  let end = 0;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 60_telemetry_analysis.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(ACCOUNT);
    // uplinkEnvelope：本地 stub broker 没有 aetherlink 插件补信封，按 adapter 原生契约发送。
    seed = await seedData.ensureDeviceWithTelemetry(ACCOUNT, { uplinkEnvelope: true });
    deviceId = seed.id;
    expect(deviceId, 'seeded device id').to.be.a('string').and.not.equal('');
    // 窗口必须在播种之后计算：数据点时间戳 ≈ 播种完成时刻。
    end = Date.now();
    start = end - 2 * HOUR_MS;
  });

  after(async function () {
    if (seed && typeof seed.cleanup === 'function') {
      await seed.cleanup();
    }
  });

  it('returns one numeric aggregate per device with the snake_case wire contract', async function () {
    const resp = await apiClient.post('/telemetry/analysis', {
      device_ids: [deviceId],
      key: 'temperature_1',
      start_time: start,
      end_time: end,
      aggregate: 'avg'
    }, ACCOUNT);
    expectSuccess(resp, 'analysis query avg');
    const data = resp.data;

    expect(data.key, 'result.key').to.equal('temperature_1');
    expect(data.aggregate, 'result.aggregate').to.equal('avg');
    expect(data.compare, 'result.compare defaults to none').to.equal('none');
    expect(data.devices, 'one row per device').to.be.an('array').with.lengthOf(1);

    const row = data.devices[0];
    expect(row.device_id, 'row device id').to.equal(deviceId);
    expect(row.error, 'readable device must not carry an error').to.equal(undefined);
    // 严格锁线契约：{value, ok, aggregate}——不允许大驼峰、不允许 avg 之类别名。
    expect(row.current, 'row.current keys').to.have.all.keys('value', 'ok', 'aggregate');
    expect(row.current.ok, 'row.current.ok').to.equal(true);
    expect(row.current.aggregate, 'row.current.aggregate').to.equal('avg');
    // 种子是单点 25.5，均值必然精确等于 25.5；宽松断言只遮真回归。
    expect(row.current.value, 'avg of a single 25.5 sample').to.be.closeTo(25.5, 0.001);
  });

  it('supports the aggregate=last contract that the hot path previously rejected', async function () {
    // API 校验一直允许 last，但 DAL 此前没有 last 分支——运行期报「不支持的聚合函数」。
    // 本用例是该承诺的运行期证据：last 必须返回桶内最新一个样本（= 25.5）。
    const resp = await apiClient.post('/telemetry/analysis', {
      device_ids: [deviceId],
      key: 'temperature_1',
      start_time: start,
      end_time: end,
      aggregate: 'last'
    }, ACCOUNT);
    expectSuccess(resp, 'analysis query last');
    const row = resp.data.devices[0];
    expect(row.current.ok, 'row.current.ok').to.equal(true);
    expect(row.current.aggregate, 'row.current.aggregate').to.equal('last');
    expect(row.current.value, 'last sample value').to.be.closeTo(25.5, 0.001);
  });

  it('returns undefined percent change with a reason when the baseline window has no data', async function () {
    const resp = await apiClient.post('/telemetry/analysis', {
      device_ids: [deviceId],
      key: 'temperature_1',
      start_time: start,
      end_time: end,
      aggregate: 'avg',
      compare: 'previous_period'
    }, ACCOUNT);
    expectSuccess(resp, 'analysis query previous_period');
    expect(resp.data.compare, 'result.compare').to.equal('previous_period');

    const row = resp.data.devices[0];
    expect(row.current.ok, 'current period has data').to.equal(true);
    // 基线窗口 [end-4h, end-2h) 早于播种，必然无数据。
    expect(row.baseline, 'row.baseline keys').to.have.property('ok');
    expect(row.baseline.ok, 'baseline without data is not ok').to.equal(false);
    expect(row, 'undefined percent must not be a number').to.not.have.property('percent_change');
    expect(row.percent_change_reason, 'undefined percent carries a reason')
      .to.equal('insufficient data in one or both periods');
    expect(row, 'delta without baseline must be absent').to.not.have.property('delta');
  });

  it('reports an unreadable device per-row without failing the whole comparison', async function () {
    const ghost = '00000000-0000-0000-0000-00000000dead';
    const resp = await apiClient.post('/telemetry/analysis', {
      device_ids: [deviceId, ghost],
      key: 'temperature_1',
      start_time: start,
      end_time: end,
      aggregate: 'avg'
    }, ACCOUNT);
    expectSuccess(resp, 'analysis with one unreadable device');
    const rows = resp.data.devices;
    expect(rows, 'one row per requested device').to.have.lengthOf(2);

    const byId = new Map(rows.map(r => [r.device_id, r]));
    expect(byId.get(deviceId).error, 'readable device has no error').to.equal(undefined);
    expect(byId.get(ghost).error, 'unreadable device reports per-row error')
      .to.equal('device not readable');
  });

  it('rejects an unsupported aggregate, an empty device list and a reversed window', async function () {
    expectBusinessError(await apiClient.post('/telemetry/analysis', {
      device_ids: [deviceId],
      key: 'temperature_1',
      start_time: start,
      end_time: end,
      aggregate: 'median'
    }, ACCOUNT), CODE_PARAM_ERROR);

    expectBusinessError(await apiClient.post('/telemetry/analysis', {
      device_ids: [],
      key: 'temperature_1',
      start_time: start,
      end_time: end
    }, ACCOUNT), CODE_PARAM_ERROR);

    expectBusinessError(await apiClient.post('/telemetry/analysis', {
      device_ids: [deviceId],
      key: 'temperature_1',
      start_time: end,
      end_time: start
    }, ACCOUNT), CODE_PARAM_ERROR);
  });

  it('exports the analysis as CSV and reports the server-side file path', async function () {
    const resp = await apiClient.post('/telemetry/analysis/export', {
      device_ids: [deviceId],
      key: 'temperature_1',
      start_time: start,
      end_time: end,
      aggregate: 'avg',
      format: 'csv'
    }, ACCOUNT);
    expectSuccess(resp, 'export csv');
    expect(resp.data.format, 'echoed format').to.equal('csv');
    expect(resp.data.file_path, 'server-side export path').to.be.a('string')
      .and.match(/\.csv$/);
  });

  it('defaults the export format to xlsx when format is omitted', async function () {
    const resp = await apiClient.post('/telemetry/analysis/export', {
      device_ids: [deviceId],
      key: 'temperature_1',
      start_time: start,
      end_time: end,
      aggregate: 'avg'
    }, ACCOUNT);
    expectSuccess(resp, 'export default xlsx');
    expect(resp.data.format, 'default format').to.equal('xlsx');
    expect(resp.data.file_path, 'server-side export path').to.be.a('string')
      .and.match(/\.xlsx$/);
  });

  it('rejects an unsupported export format', async function () {
    expectBusinessError(await apiClient.post('/telemetry/analysis/export', {
      device_ids: [deviceId],
      key: 'temperature_1',
      start_time: start,
      end_time: end,
      format: 'pdf'
    }, ACCOUNT), CODE_PARAM_ERROR);
  });
});
