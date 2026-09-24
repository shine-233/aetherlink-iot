/**
 * 文件用途：TB 数据转换器引擎（ThingsBoard Data Converter Engine）活栈契约测试。
 *
 * 覆盖：
 *   1. HEX_BINARY 十六进制二进制解析引擎测试（带 Modbus 偏移量、大端字节序与 scale 缩放因子）；
 *   2. JSON_PATH 嵌套 JSON 提取引擎测试（dot-notation 多层抽取与 telemetry/attributes 分类映射）；
 *   3. SCRIPT Lua 沙箱脚本解析引擎测试（解析原始 payload 并动态生成 ThingsBoard 格式遥测）；
 *   4. 数据转换器完整 CRUD 生命周期（POST /api/v1/converters, GET, PUT, DELETE）；
 *   5. 基于已存储转换器 ID 的 dry-run 仿真执行；
 *   6. 严格多租户数据隔离（租户 B 无法查询、修改或删除租户 A 的转换器）；
 *   7. 路由别名（/api/v1/data-converters）契约一致性验证。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'TB Data Converter Engine [73_data_converters]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NO_PERMISSION = 201001;

describe(SUITE, function () {
  this.timeout(120000);

  let createdConverterId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 73_data_converters.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (createdConverterId) {
      try {
        await apiClient.delete('/converters/' + createdConverterId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. POST /converters/test with HEX_BINARY mode decodes byte offsets and applies scale', async function () {
    const hexPayload = '01030400FA01F4E853'; // offset 3: 00FA (250), offset 5: 01F4 (500)
    const testReq = {
      converter_mode: 'HEX_BINARY',
      payload: hexPayload,
      configuration: JSON.stringify({
        mappings: [
          {
            key: 'temperature',
            offset: 3,
            length: 2,
            type: 'int16',
            endian: 'big',
            scale: 0.1
          },
          {
            key: 'humidity',
            offset: 5,
            length: 2,
            type: 'uint16',
            endian: 'big',
            scale: 0.1
          }
        ]
      })
    };

    const res = await apiClient.post('/converters/test', testReq, TENANT_A);
    expect(res.code, 'hex converter test').to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.success).to.be.true;
    expect(res.data.telemetry).to.be.an('object');
    expect(res.data.telemetry.temperature).to.equal(25.0);
    expect(res.data.telemetry.humidity).to.equal(50.0);
  });

  it('2. POST /converters/test with JSON_PATH mode extracts nested fields', async function () {
    const jsonPayload = JSON.stringify({
      device_sn: 'SENSOR-X-101',
      metrics: {
        climate: {
          temp_c: 23.4,
          humidity_pct: 58.2
        },
        battery_pct: 95
      },
      firmware: 'v2.1.0'
    });

    const testReq = {
      converter_mode: 'JSON_PATH',
      payload: jsonPayload,
      configuration: JSON.stringify({
        device_name: 'device_sn',
        telemetry: {
          temperature: 'metrics.climate.temp_c',
          humidity: 'metrics.climate.humidity_pct',
          battery: 'metrics.battery_pct'
        },
        attributes: {
          version: 'firmware'
        }
      })
    };

    const res = await apiClient.post('/converters/test', testReq, TENANT_A);
    expect(res.code, 'json path test').to.equal(200);
    expect(res.data.success).to.be.true;
    expect(res.data.device_name).to.equal('SENSOR-X-101');
    expect(res.data.telemetry).to.deep.equal({
      temperature: 23.4,
      humidity: 58.2,
      battery: 95
    });
    expect(res.data.attributes).to.deep.equal({
      version: 'v2.1.0'
    });
  });

  it('3. POST /converters/test with SCRIPT mode executes Lua transformation', async function () {
    const rawPayload = JSON.stringify({
      dId: 'EDGE-GW-001',
      raw_temp: 245,
      alarm: 1
    });

    const luaScript = `
      local d = json.decode(payload)
      local res = {
        deviceName = d.dId,
        telemetry = {
          temperature = d.raw_temp / 10.0,
          alarm_flag = d.alarm == 1
        }
      }
      return json.encode(res)
    `;

    const testReq = {
      converter_mode: 'SCRIPT',
      payload: rawPayload,
      script: luaScript
    };

    const res = await apiClient.post('/converters/test', testReq, TENANT_A);
    expect(res.code, 'script converter test').to.equal(200);
    expect(res.data.success).to.be.true;
    expect(res.data.device_name).to.equal('EDGE-GW-001');
    expect(res.data.telemetry.temperature).to.equal(24.5);
    expect(res.data.telemetry.alarm_flag).to.be.true;
  });

  it('4. POST /converters creates a new data converter', async function () {
    const createReq = {
      name: 'Modbus Telemetry Converter ' + Date.now(),
      type: 'UPLINK',
      converter_mode: 'HEX_BINARY',
      debug_mode: true,
      description: 'Automated test uplink converter',
      configuration: JSON.stringify({
        mappings: [
          { key: 'co2', offset: 0, length: 2, type: 'uint16', endian: 'big', scale: 1.0 }
        ]
      })
    };

    const res = await apiClient.post('/converters', createReq, TENANT_A);
    expect(res.code, 'create converter').to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.id).to.be.a('string');
    expect(res.data.name).to.equal(createReq.name);
    createdConverterId = res.data.id;
  });

  it('5. GET /converters/:id fetches created converter', async function () {
    const res = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_A);
    expect(res.code, 'get converter by id').to.equal(200);
    expect(res.data.id).to.equal(createdConverterId);
    expect(res.data.converter_mode).to.equal('HEX_BINARY');
    expect(res.data.debug_mode).to.be.true;
  });

  it('6. GET /converters lists tenant converters with pagination', async function () {
    const res = await apiClient.get('/converters', { page: 1, page_size: 10 }, TENANT_A);
    expect(res.code, 'list converters').to.equal(200);
    expect(res.data.list).to.be.an('array');
    const found = res.data.list.find(c => c.id === createdConverterId);
    expect(found, 'created converter in list').to.not.be.undefined;
    expect(res.data.total).to.be.at.least(1);
  });

  it('7. PUT /converters updates converter metadata and script/configuration', async function () {
    const updateReq = {
      id: createdConverterId,
      name: 'Updated Modbus Converter',
      type: 'UPLINK',
      converter_mode: 'HEX_BINARY',
      debug_mode: false,
      description: 'Updated description by automated test'
    };

    const res = await apiClient.put('/converters', updateReq, TENANT_A);
    expect(res.code, 'update converter').to.equal(200);

    const check = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_A);
    expect(check.data.name).to.equal('Updated Modbus Converter');
    expect(check.data.debug_mode).to.be.false;
  });

  it('8. POST /converters/test using saved converter_id executes successfully', async function () {
    const testReq = {
      converter_id: createdConverterId,
      payload: '03E8' // 1000 in uint16
    };

    const res = await apiClient.post('/converters/test', testReq, TENANT_A);
    expect(res.code, 'test with saved converter_id').to.equal(200);
    expect(res.data.success).to.be.true;
    expect(res.data.telemetry).to.deep.equal({ co2: 1000 });
  });

  it('9. Multi-tenant isolation: Tenant B cannot read or modify Tenant A converter', async function () {
    // Tenant B try get
    const getRes = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_B);
    expect(getRes.code).to.satisfy(c => c === CODE_NO_PERMISSION || c === 404 || c === 100000 || c === 100002 || c === 100404);

    // Tenant B try delete
    const delRes = await apiClient.delete('/converters/' + createdConverterId, {}, TENANT_B);
    expect(delRes.code).to.satisfy(c => c === CODE_NO_PERMISSION || c === 404 || c === 100000 || c === 100002 || c === 100404);
  });

  it('10. DELETE /converters/:id deletes converter and confirms cleanup', async function () {
    const delRes = await apiClient.delete('/converters/' + createdConverterId, {}, TENANT_A);
    expect(delRes.code, 'delete converter').to.equal(200);

    const checkRes = await apiClient.get('/converters/' + createdConverterId, {}, TENANT_A);
    expect(checkRes.code).to.not.equal(200);
    createdConverterId = null;
  });

  it('11. Alias /api/v1/data-converters route operates identically', async function () {
    const res = await apiClient.get('/data-converters', { page: 1, page_size: 5 }, TENANT_A);
    expect(res.code, 'alias get /data-converters').to.equal(200);
    expect(res.data.list).to.be.an('array');
  });
});
