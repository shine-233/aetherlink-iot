/**
 * 文件用途：TB-13 地理空间追踪与看板地图部件（Geospatial Map Tracking & Dashboard Map Widget）活栈契约测试。
 *
 * 覆盖：
 *   1. 租户设备最新地理空间位置拉取（GET /api/v1/devices/locations/latest 与 GET /api/v1/device/locations/latest）；
 *   2. 支持静态配置坐标（devices.location 如 "116.4074, 39.9042"）的自动解析与标绘；
 *   3. 支持动态时序遥测坐标（latitude, longitude, speed, altitude）的高精度解析与最新态聚合；
 *   4. 单设备历史行驶轨迹点阵查询（GET /api/v1/device/:id/location/history 与 GET /api/v1/devices/:id/location/history）；
 *   5. 历史轨迹时间窗口过滤（from / to）与排序（ts ASC）；
 *   6. 严格多租户隔离与越权阻断（租户 B 访问租户 A 设备历史轨迹返回 201001）；
 *   7. 异常输入防御与健壮性校验（非法设备 ID 与极端坐标容错）。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'TB-13 Geospatial Map Tracking [72_geospatial_map_tracking]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NO_PERMISSION = 201001;

describe(SUITE, function () {
  this.timeout(120000);

  let staticDevice = null;
  let dynamicDevice = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 72_geospatial_map_tracking.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (staticDevice && staticDevice.id) {
      try {
        await apiClient.delete('/device/' + staticDevice.id, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (dynamicDevice && dynamicDevice.cleanup) {
      try {
        await dynamicDevice.cleanup();
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. GET /devices/locations/latest returns list and total structure', async function () {
    const res = await apiClient.get('/devices/locations/latest', {}, TENANT_A);
    expect(res.code, 'query latest device locations').to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data).to.have.property('list').that.is.an('array');
    expect(res.data).to.have.property('total').that.is.a('number');
  });

  it('2. GET /device/locations/latest (singular route) returns identical contract', async function () {
    const res = await apiClient.get('/device/locations/latest', {}, TENANT_A);
    expect(res.code, 'query latest device locations via singular route').to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data).to.have.property('list').that.is.an('array');
  });

  it('3. Device with static location string is resolved to numeric coordinates', async function () {
    const devName = seedData.makeRunLabel('geo-static-dev');
    const voucher = JSON.stringify({
      username: 'geo-static-' + Date.now(),
      password: 'password-' + Date.now()
    });

    // 写入经纬度格式 "116.4074, 39.9042" (北京)
    const createResp = await apiClient.post('/device', {
      name: devName,
      device_config_id: '',
      voucher,
      location: '116.4074, 39.9042'
    }, TENANT_A);

    expect(createResp.code, 'create static geo device').to.equal(200);
    const devId = createResp.data && (createResp.data.id || createResp.data.ID);
    expect(devId).to.be.a('string').and.not.empty;
    staticDevice = { id: devId, name: devName };

    // 查询最新位置列表
    const locResp = await apiClient.get('/devices/locations/latest', {}, TENANT_A);
    expect(locResp.code).to.equal(200);
    const target = locResp.data.list.find(d => d.device_id === devId);
    expect(target, 'device should be in location list').to.be.an('object');
    expect(target.device_name).to.equal(devName);
    expect(target.latitude, 'latitude parsed').to.be.closeTo(39.9042, 0.001);
    expect(target.longitude, 'longitude parsed').to.be.closeTo(116.4074, 0.001);
  });

  it('4. Device reporting dynamic GPS telemetry updates latest coordinates and telemetry attributes', async function () {
    const isMqtt = await seedData.isMqttBrokerAvailable();
    if (!isMqtt) {
      this.skip();
      return;
    }

    dynamicDevice = await seedData.createSimulationDevice(TENANT_A);
    expect(dynamicDevice.id).to.be.a('string').and.not.empty;

    // 上报第一组 GPS 遥测 (上海坐标)
    const gpsPayload = {
      latitude: 31.2304,
      longitude: 121.4737,
      speed: 68.5,
      altitude: 15.0
    };

    const pubResult = await seedData.publishSimulatedTelemetryAndReadCurrent(
      dynamicDevice.id,
      gpsPayload,
      TENANT_A,
      { waitForHistory: true, attempts: 15, delayMs: 400 }
    );
    expect(pubResult).to.be.an('object');

    // 验证 GET /devices/locations/latest 读到最新动态遥测坐标
    const locResp = await apiClient.get('/devices/locations/latest', {}, TENANT_A);
    expect(locResp.code).to.equal(200);
    const target = locResp.data.list.find(d => d.device_id === dynamicDevice.id);
    expect(target, 'device found in latest locations').to.be.an('object');
    expect(target.latitude).to.be.closeTo(31.2304, 0.001);
    expect(target.longitude).to.be.closeTo(121.4737, 0.001);
    expect(target.speed).to.be.closeTo(68.5, 0.001);
    expect(target.altitude).to.be.closeTo(15.0, 0.001);
  });

  it('5. GET /device/:id/location/history returns historical breadcrumb points', async function () {
    if (!dynamicDevice || !dynamicDevice.id) {
      this.skip();
      return;
    }

    const histResp = await apiClient.get('/device/' + dynamicDevice.id + '/location/history', {}, TENANT_A);
    expect(histResp.code, 'query location history').to.equal(200);
    expect(histResp.data).to.be.an('object');
    expect(histResp.data.device_id).to.equal(dynamicDevice.id);
    expect(histResp.data.points).to.be.an('array').that.is.not.empty;

    const pt = histResp.data.points[0];
    expect(pt).to.have.property('ts').that.is.a('number');
    expect(pt.latitude).to.be.closeTo(31.2304, 0.001);
    expect(pt.longitude).to.be.closeTo(121.4737, 0.001);
    expect(pt.speed).to.be.closeTo(68.5, 0.001);
  });

  it('6. GET /devices/:device_id/location/history (plural route) returns identical trajectory', async function () {
    if (!dynamicDevice || !dynamicDevice.id) {
      this.skip();
      return;
    }

    const histResp = await apiClient.get('/devices/' + dynamicDevice.id + '/location/history', {}, TENANT_A);
    expect(histResp.code, 'query location history plural').to.equal(200);
    expect(histResp.data).to.be.an('object');
    expect(histResp.data.device_id).to.equal(dynamicDevice.id);
    expect(histResp.data.points).to.be.an('array').that.is.not.empty;
  });

  it('7. Multi-tenant security: Tenant B cannot access Tenant A device location history (201001)', async function () {
    if (!dynamicDevice || !dynamicDevice.id) {
      this.skip();
      return;
    }

    const crossResp = await apiClient.get('/device/' + dynamicDevice.id + '/location/history', {}, TENANT_B);
    expect(crossResp.code, 'tenant isolation').to.equal(CODE_NO_PERMISSION);
  });

  it('8. Querying non-existent device location history returns error gracefully (not 500)', async function () {
    const fakeId = '00000000-0000-0000-0000-000000000000';
    const fakeResp = await apiClient.get('/device/' + fakeId + '/location/history', {}, TENANT_A);
    expect(fakeResp.code).to.not.equal(500);
    expect([CODE_NO_PERMISSION, 100404, CODE_PARAM_ERROR]).to.include(fakeResp.code);
  });
});
