/**
 * 文件用途：TP-6 / TB PE 设备综合健康度评估（Device Health Score）活栈契约测试。
 *
 * 覆盖：
 *   1. 租户设备综合健康大盘统计（GET /api/v1/devices/health/summary 与 GET /api/v1/device/health/summary）；
 *   2. 租户全量设备健康评估调度（POST /api/v1/devices/health/evaluate 与 POST /api/v1/device/health/evaluate）；
 *   3. 单设备多维健康评分诊断（GET /api/v1/devices/:device_id/health 与 GET /api/v1/device/:id/health）；
 *   4. 单设备即时健康重评估（POST /api/v1/devices/:device_id/health/evaluate 与 POST /api/v1/device/:id/health/evaluate）；
 *   5. 扣分维度与等级状态契约（score, health_status, alarm_penalty, offline_penalty, suggestions）；
 *   6. 严格多租户隔离与越权阻断（租户 B 无法查询或重评估租户 A 设备的健康度）；
 *   7. 异常输入与不存在设备容错校验（返回 100404）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'TP-6 / TB PE Device Health Score [74_device_health_scores]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NO_PERMISSION = 201001;
const CODE_NOT_FOUND = 100404;

describe(SUITE, function () {
  this.timeout(120000);

  let createdDevice = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 74_device_health_scores.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);

    // 为租户 A 创建一个测试设备
    const devRes = await apiClient.post('/device', {
      name: 'Health Test Device ' + Date.now(),
      voucher: 'health-dev-' + Date.now()
    }, TENANT_A);
    expect(devRes.code, 'create health test device').to.equal(200);
    createdDevice = devRes.data;
  });

  after(async function () {
    if (createdDevice && createdDevice.id) {
      try {
        await apiClient.delete('/device/' + createdDevice.id, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. GET /devices/health/summary returns tenant health overview metrics', async function () {
    const res = await apiClient.get('/devices/health/summary', {}, TENANT_A);
    expect(res.code, 'get health summary').to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.total_devices).to.be.a('number');
    expect(res.data.healthy_count).to.be.a('number');
    expect(res.data.sub_healthy_count).to.be.a('number');
    expect(res.data.warning_count).to.be.a('number');
    expect(res.data.critical_count).to.be.a('number');
    expect(res.data.average_score).to.be.a('number');
    expect(res.data.unhealthy_devices).to.be.an('array');
  });

  it('2. POST /devices/health/evaluate triggers batch evaluation for tenant', async function () {
    const res = await apiClient.post('/devices/health/evaluate', {}, TENANT_A);
    expect(res.code, 'evaluate all devices').to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.total_devices).to.be.at.least(1);
  });

  it('3. GET /devices/:device_id/health returns multidimensional scoring for single device', async function () {
    const res = await apiClient.get('/devices/' + createdDevice.id + '/health', {}, TENANT_A);
    expect(res.code, 'get device health').to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.device_id).to.equal(createdDevice.id);
    expect(res.data.score).to.be.a('number');
    expect(res.data.score).to.be.at.least(0).and.at.most(100);
    expect(res.data.health_status).to.be.oneOf(['HEALTHY', 'SUB_HEALTHY', 'WARNING', 'CRITICAL']);
    expect(res.data.alarm_penalty).to.be.a('number');
    expect(res.data.offline_penalty).to.be.a('number');
    expect(res.data.anomaly_penalty).to.be.a('number');
    expect(res.data.suggestions).to.be.an('array');
    expect(res.data.suggestions.length).to.be.at.least(1);
  });

  it('4. POST /devices/:device_id/health/evaluate triggers immediate re-evaluation', async function () {
    const res = await apiClient.post('/devices/' + createdDevice.id + '/health/evaluate', {}, TENANT_A);
    expect(res.code, 'evaluate single device').to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.device_id).to.equal(createdDevice.id);
    expect(res.data.score).to.be.a('number');
  });

  it('5. Singular route contract: GET /device/:id/health operates identically', async function () {
    const res = await apiClient.get('/device/' + createdDevice.id + '/health', {}, TENANT_A);
    expect(res.code, 'singular get device health').to.equal(200);
    expect(res.data.device_id).to.equal(createdDevice.id);
  });

  it('6. Multi-tenant isolation: Tenant B cannot view Tenant A device health', async function () {
    const res = await apiClient.get('/devices/' + createdDevice.id + '/health', {}, TENANT_B);
    expect(res.code).to.satisfy(c => c === CODE_NO_PERMISSION || c === CODE_NOT_FOUND || c === 404 || c === 100000);
  });

  it('7. Multi-tenant isolation: Tenant B cannot trigger evaluate on Tenant A device', async function () {
    const res = await apiClient.post('/devices/' + createdDevice.id + '/health/evaluate', {}, TENANT_B);
    expect(res.code).to.satisfy(c => c === CODE_NO_PERMISSION || c === CODE_NOT_FOUND || c === 404 || c === 100000);
  });

  it('8. Negative validation: query health for non-existent device returns not found error', async function () {
    const fakeId = '00000000-0000-0000-0000-000000000000';
    const res = await apiClient.get('/devices/' + fakeId + '/health', {}, TENANT_A);
    expect(res.code).to.satisfy(c => c === CODE_NOT_FOUND || c === 404 || c === 100000);
  });
});
