/**
 * 文件用途：P3 商业化计费套餐、租户用量计量与配额消耗（Billing & Usage Metering）API 契约测试。
 *
 * 覆盖：
 *   1. 套餐目录查询（GET /api/v1/billing/plans）——SYS_ADMIN 与 TENANT_ADMIN 可见 free/pro/enterprise 阶梯；
 *   2. 租户用量计量（GET /api/v1/billing/usage）——租户查看自身实时设备/用户/子租户与遥测消耗；
 *   3. 租户多租户作用域防线——TENANT_ADMIN 无法越权探查其他租户用量（403/权限拒绝）；
 *   4. 套餐变更与订购（POST /api/v1/billing/subscriptions）——TENANT_ADMIN 升级至 pro，用量报告即时呈现新配额；
 *   5. 非法套餐代码拒绝——输入不存在的 plan_code 抛出明确参数错误。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'P3 billing & tenant usage metering [70_p3_billing_and_usage_metering]';
const SYS_ACCOUNT = 'super_admin';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NO_PERMISSION = 100003;

describe(SUITE, function () {
  this.timeout(120000);

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 70_p3_billing_and_usage_metering.test.js');
    }
    await apiClient.login(SYS_ACCOUNT);
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  it('1. lists available subscription plans with pricing and quota tiers', async function () {
    const resp = await apiClient.get('/billing/plans', {}, TENANT_A);
    expect(resp.code, 'get billing plans').to.equal(200);
    expect(resp.data).to.be.an('array').with.lengthOf.at.least(3);

    const codes = resp.data.map(p => p.code);
    expect(codes).to.include('free');
    expect(codes).to.include('pro');
    expect(codes).to.include('enterprise');

    const proPlan = resp.data.find(p => p.code === 'pro');
    expect(proPlan).to.have.property('max_devices').that.is.at.least(500);
    expect(proPlan).to.have.property('price_monthly').that.is.above(0);
    expect(proPlan).to.have.property('features').that.is.a('string');
  });

  it('2. retrieves tenant usage report with quota consumption percentages', async function () {
    const resp = await apiClient.get('/billing/usage', {}, TENANT_A);
    expect(resp.code, 'get tenant usage').to.equal(200);
    expect(resp.data).to.be.an('object');

    expect(resp.data).to.have.property('tenant_id').that.is.a('string');
    expect(resp.data).to.have.property('plan_code').that.is.a('string');
    expect(resp.data).to.have.property('status').that.is.a('string');
    expect(resp.data).to.have.property('device_count').that.is.a('number');
    expect(resp.data).to.have.property('max_devices').that.is.a('number');
    expect(resp.data).to.have.property('device_usage_pct').that.is.a('number');
    expect(resp.data).to.have.property('quota_status').that.be.oneOf(['normal', 'warning', 'exceeded']);
    expect(resp.data).to.have.property('features').that.is.an('array');
  });

  it('3. rejects cross-tenant usage inspection by non-sysadmin', async function () {
    // 租户 B 试图查询租户 A 的用量
    const usageA = await apiClient.get('/billing/usage', {}, TENANT_A);
    expect(usageA.code).to.equal(200);
    const tenantAId = usageA.data.tenant_id;
    expect(tenantAId).to.be.a('string');

    const leakResp = await apiClient.get(`/billing/usage?tenant_id=${tenantAId}`, {}, TENANT_B);
    expect(leakResp.code, 'cross-tenant inspection must fail').to.not.equal(200);
  });

  it('4. subscribes tenant to pro plan and reflects new quotas in usage report', async function () {
    const subResp = await apiClient.post('/billing/subscriptions', {
      plan_code: 'pro'
    }, TENANT_A);
    expect(subResp.code, 'subscribe to pro').to.equal(200);
    expect(subResp.data).to.have.property('plan_code').that.equals('pro');
    expect(subResp.data).to.have.property('status').that.equals('active');

    // 读回用量，验证 plan_code 与 max_devices 对应提升
    const usageResp = await apiClient.get('/billing/usage', {}, TENANT_A);
    expect(usageResp.code, 'read updated usage').to.equal(200);
    expect(usageResp.data.plan_code).to.equal('pro');
    expect(usageResp.data.max_devices).to.equal(500);
  });

  it('5. rejects subscribing to a non-existent plan code', async function () {
    const badResp = await apiClient.post('/billing/subscriptions', {
      plan_code: 'non_existent_plan_xyz'
    }, TENANT_A);
    expect(badResp.code, 'invalid plan rejection').to.equal(CODE_PARAM_ERROR);
  });
});
