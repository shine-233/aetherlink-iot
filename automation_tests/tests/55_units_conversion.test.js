/**
 * 文件用途：TB-9 单位换算全链路闭环（Units Conversion End-to-End）契约与端到端测试。
 *
 * 对标 ThingsBoard 4.1.0 LTS 头条特性 Units Conversion：
 *   1. 单位字典查询与规范映射契约（GET /units/registry）；
 *   2. 原子单位换算服务（POST /units/convert：单值换算、序列换算、制式自动换算、Fail-Closed 异常拒绝）；
 *   3. 遥测分析与物模型两跳自动解析集成（POST /telemetry/analysis：显式源单位、未传单位时物模型两跳解析）；
 *   4. 物理不变量防御（Fail-Closed：count 聚合跳过换算、sum 聚合对温度等带 Offset 单位物理安全拒绝）；
 *   5. 鉴权体系与多租户权限隔离（401 未登录拦截、普通租户成员只读与换算权限、跨租户隔离）。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');

const SUITE = 'TB-9 Units Conversion End-to-End [55_units_conversion]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';
const USER_ACCOUNT = 'tenant_user';

const CODE_PARAM_ERROR = 100002;

function pickId(row) {
  return row && (
    row.id ||
    row.ID ||
    row.device_id ||
    row.deviceId ||
    row.template_id ||
    row.device_template_id
  );
}

describe(SUITE, function () {
  this.timeout(180000);

  const cleanups = [];
  let deviceSeed = null;
  let templateId = null;
  let modelTelemetryId = null;
  let configId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 55_units_conversion.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(USER_ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);

    // 创建测试设备并播种遥测数据
    deviceSeed = await seedData.ensureDeviceWithTelemetry(ACCOUNT, { uplinkEnvelope: true });
    expect(deviceSeed, 'deviceSeed').to.be.an('object');
    expect(deviceSeed.id, 'deviceSeed.id').to.be.a('string').and.not.equal('');

    // 为两跳物模型测试构建物模型与设备配置链条：
    // device -> device_config -> device_template -> device_model_telemetry (unit: °C)
    const templateName = `tb9_template_${Date.now()}`;
    const tResp = await apiClient.post('/device/template', {
      name: templateName,
      author: 'automation',
      version: '1.0.0',
      description: 'TB-9 units conversion fixture template'
    }, ACCOUNT);
    expectSuccess(tResp, 'create fixture template');
    templateId = pickId(tResp.data);

    const mResp = await apiClient.post('/device/model/telemetry', {
      device_template_id: templateId,
      data_name: 'Temperature Sensor 1',
      data_identifier: 'temperature_1',
      data_type: 'Number',
      unit: '°C'
    }, ACCOUNT);
    expectSuccess(mResp, 'create fixture telemetry model');
    modelTelemetryId = pickId(mResp.data);

    const cfgName = `tb9_config_${Date.now()}`;
    const cResp = await apiClient.post('/device_config', {
      name: cfgName,
      device_template_id: templateId,
      device_type: '1',
      protocol_type: 'MQTT',
      voucher_type: 'ACCESSTOKEN',
      device_conn_type: 'A',
      protocol_config: '{}'
    }, ACCOUNT);
    expectSuccess(cResp, 'create fixture device config');
    configId = pickId(cResp.data);

    const bindResp = await apiClient.put('/device/update/config', {
      device_id: deviceSeed.id,
      device_config_id: configId
    }, ACCOUNT);
    expectSuccess(bindResp, 'bind device to fixture config');
  });

  after(async function () {
    // 逆序执行清理
    if (deviceSeed && configId) {
      try {
        await apiClient.put('/device/update/config', {
          device_id: deviceSeed.id,
          device_config_id: ''
        }, ACCOUNT);
      } catch (_) {}
    }
    if (configId) {
      try {
        await apiClient.delete('/device_config/' + configId, {}, ACCOUNT);
      } catch (_) {}
    }
    if (modelTelemetryId) {
      try {
        await apiClient.delete('/device/model/telemetry/' + modelTelemetryId, {}, ACCOUNT);
      } catch (_) {}
    }
    if (templateId) {
      try {
        await apiClient.delete('/device/template/' + templateId, {}, ACCOUNT);
      } catch (_) {}
    }
    if (deviceSeed) {
      try {
        await deviceSeed.cleanup();
      } catch (_) {}
    }

    for (let i = cleanups.length - 1; i >= 0; i--) {
      try {
        await cleanups[i]();
      } catch (_) {}
    }
    apiClient.clearAllTokens();
  });

  describe('1. Units Registry & Dimensional Catalog Contract (GET /units/registry)', function () {
    it('retrieves full units catalog with 12 dimensions and canonical mappings as tenant_admin', async function () {
      const resp = await apiClient.get('/units/registry', {}, ACCOUNT);
      expectSuccess(resp, 'GET /units/registry');
      const data = resp.data;

      expect(data).to.be.an('object');
      expect(data.dimensions).to.be.an('array');
      expect(data.dimensions).to.include.members([
        'temperature',
        'length',
        'mass',
        'volume',
        'area',
        'speed',
        'pressure',
        'energy',
        'power',
        'flow',
        'time',
        'ratio'
      ]);

      expect(data.units).to.be.an('array');
      expect(data.units.length).to.be.at.least(50);

      // 验证代表单位映射
      expect(data.canonical).to.be.an('object');
      expect(data.canonical.temperature).to.deep.equal({ metric: '°C', imperial: '°F' });
      expect(data.canonical.pressure).to.deep.equal({ metric: 'kPa', imperial: 'psi' });
      expect(data.canonical.speed).to.deep.equal({ metric: 'm/s', imperial: 'mph' });

      // 验证别名映射
      expect(data.aliases).to.be.an('object');
      expect(data.aliases.celsius).to.equal('°C');
      expect(data.aliases.degF).to.equal('°F');
      expect(data.aliases.mps).to.equal('m/s');
    });

    it('allows tenant_user (ordinary member) to access units registry', async function () {
      const resp = await apiClient.get('/units/registry', {}, USER_ACCOUNT);
      expectSuccess(resp, 'GET /units/registry as tenant_user');
      expect(resp.data.dimensions.length).to.equal(12);
    });
  });

  describe('2. Atomic Unit Conversion API (POST /units/convert)', function () {
    it('converts single scalar temperature value from °C to °F', async function () {
      const resp = await apiClient.post('/units/convert', {
        from: '°C',
        to: '°F',
        value: 100
      }, ACCOUNT);
      expectSuccess(resp, 'convert 100 °C to °F');
      expect(resp.data.from).to.equal('°C');
      expect(resp.data.to).to.equal('°F');
      expect(resp.data.dimension).to.equal('temperature');
      expect(resp.data.value).to.be.closeTo(212, 0.001);
    });

    it('converts single scalar value to target unit system canonically', async function () {
      const resp = await apiClient.post('/units/convert', {
        from: 'kPa',
        system: 'imperial',
        value: 100
      }, ACCOUNT);
      expectSuccess(resp, 'convert 100 kPa to imperial');
      expect(resp.data.from).to.equal('kPa');
      expect(resp.data.to).to.equal('psi');
      expect(resp.data.dimension).to.equal('pressure');
      expect(resp.data.value).to.be.closeTo(14.5038, 0.01);
    });

    it('converts numerical series in batch preserving precision', async function () {
      const resp = await apiClient.post('/units/convert', {
        from: '°C',
        to: '°F',
        series: [0, 50, 100]
      }, ACCOUNT);
      expectSuccess(resp, 'convert series from °C to °F');
      expect(resp.data.series).to.be.an('array').with.lengthOf(3);
      expect(resp.data.series[0]).to.be.closeTo(32, 0.001);
      expect(resp.data.series[1]).to.be.closeTo(122, 0.001);
      expect(resp.data.series[2]).to.be.closeTo(212, 0.001);
    });

    it('rejects cross-dimension conversion fail-closed (e.g. temperature -> pressure)', async function () {
      const resp = await apiClient.post('/units/convert', {
        from: '°C',
        to: 'psi',
        value: 100
      }, ACCOUNT);
      expectBusinessError(resp, CODE_PARAM_ERROR, 'dimension mismatch');
      expect(resp.message).to.match(/dimension mismatch/i);
    });

    it('rejects unknown unit symbol fail-closed', async function () {
      const resp = await apiClient.post('/units/convert', {
        from: 'non_existent_unit_xyz',
        to: '°F',
        value: 10
      }, ACCOUNT);
      expectBusinessError(resp, CODE_PARAM_ERROR, 'unknown from unit');
      expect(resp.message).to.match(/unknown.*unit/i);
    });
  });

  describe('3. Telemetry Analysis Units Conversion & Invariant Protection (POST /telemetry/analysis)', function () {
    const end = Date.now();
    const start = end - 24 * 60 * 60 * 1000;

    it('performs telemetry analysis with explicit unit and unit_system', async function () {
      const resp = await apiClient.post('/telemetry/analysis', {
        device_ids: [deviceSeed.id],
        key: 'temperature_1',
        start_time: start,
        end_time: end,
        aggregate: 'avg',
        unit: '°C',
        unit_system: 'imperial'
      }, ACCOUNT);
      expectSuccess(resp, 'telemetry analysis explicit unit conversion');
      const data = resp.data;

      expect(data.unit_system).to.equal('imperial');
      expect(data.source_unit).to.equal('°C');
      expect(data.target_unit).to.equal('°F');
      expect(data.devices).to.be.an('array').with.lengthOf(1);

      const devResult = data.devices[0];
      expect(devResult.device_id).to.equal(deviceSeed.id);
      // 原值 25.5 °C -> 换算至 77.9 °F
      if (devResult.current && typeof devResult.current.avg === 'number') {
        expect(devResult.current.avg).to.be.closeTo(77.9, 0.2);
      }
    });

    it('automatically resolves source unit from 2-hop device model when unit is omitted', async function () {
      const resp = await apiClient.post('/telemetry/analysis', {
        device_ids: [deviceSeed.id],
        key: 'temperature_1',
        start_time: start,
        end_time: end,
        aggregate: 'avg',
        // unit 故意省略为空，依赖服务端 2-hop 物模型自动解析
        unit_system: 'imperial'
      }, ACCOUNT);
      expectSuccess(resp, 'telemetry analysis auto resolved unit conversion');
      const data = resp.data;

      expect(data.unit_system).to.equal('imperial');
      expect(data.source_unit, 'source unit resolved from device model').to.equal('°C');
      expect(data.target_unit, 'target canonical unit').to.equal('°F');
      expect(data.devices).to.be.an('array').with.lengthOf(1);

      const devResult = data.devices[0];
      expect(devResult.device_id).to.equal(deviceSeed.id);
      if (devResult.current && typeof devResult.current.avg === 'number') {
        expect(devResult.current.avg).to.be.closeTo(77.9, 0.2);
      }
    });

    it('protects physical invariant: count aggregation skips conversion fail-closed', async function () {
      const resp = await apiClient.post('/telemetry/analysis', {
        device_ids: [deviceSeed.id],
        key: 'temperature_1',
        start_time: start,
        end_time: end,
        aggregate: 'count',
        unit: '°C',
        unit_system: 'imperial'
      }, ACCOUNT);
      expectSuccess(resp, 'count aggregation unit conversion');
      const data = resp.data;

      expect(data.devices).to.be.an('array').with.lengthOf(1);
      const devResult = data.devices[0];
      expect(devResult.unit_reason).to.include('count is a tally');
      // 目标单位不应伪造为 °F
      expect(data.target_unit || '').to.equal('');
    });

    it('protects physical invariant: sum aggregation rejects units with non-zero offset', async function () {
      const resp = await apiClient.post('/telemetry/analysis', {
        device_ids: [deviceSeed.id],
        key: 'temperature_1',
        start_time: start,
        end_time: end,
        aggregate: 'sum',
        unit: '°C',
        unit_system: 'imperial'
      }, ACCOUNT);
      expectSuccess(resp, 'sum aggregation with offset unit');
      const data = resp.data;

      expect(data.devices).to.be.an('array').with.lengthOf(1);
      const devResult = data.devices[0];
      expect(devResult.unit_reason).to.include('sum of an offset-based unit');
      expect(data.target_unit || '').to.equal('');
    });
  });

  describe('4. Cross-Tenant Isolation & Role Boundaries', function () {
    it('prevents Tenant B from analyzing Tenant A device telemetry fail-closed', async function () {
      const end = Date.now();
      const start = end - 24 * 60 * 60 * 1000;
      const resp = await apiClient.post('/telemetry/analysis', {
        device_ids: [deviceSeed.id],
        key: 'temperature_1',
        start_time: start,
        end_time: end,
        unit: '°C',
        unit_system: 'imperial'
      }, OTHER_ACCOUNT);

      // 服务端安全隔离：单设备越权时返回 device error 而非暴露数据
      expectSuccess(resp, 'cross tenant telemetry analysis response envelope');
      const devResult = resp.data.devices && resp.data.devices[0];
      expect(devResult).to.be.an('object');
      expect(devResult.error).to.match(/device not found|not readable|no permission/i);
    });
  });
});
