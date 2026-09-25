/**
 * 文件用途：租户 API 日配额（TB-17：计量落库 + 按套餐执法 + 配额查询端点）API 契约测试。
 *
 * 覆盖：
 *   1. GET /api/v1/billing/api-quota 契约——今日调用数 / 套餐限额 / 剩余量 / 百分比 / 状态；
 *   2. 计量同日累加——连续调用后 api_calls_today 单调不减（每笔经 TenantRateLimit 的请求都计数）；
 *   3. 数值自洽——remaining === max - used、usage_pct 与 used/max 一致（执法判定纯函数同口径）；
 *   4. 租户隔离——TENANT_ADMIN 不能越权查看其他租户配额；SYS_ADMIN 可指定 tenant_id 查询；
 *   5. 429 执法契约——低阈值租户（/ratelimit/override 覆盖 api 限额为 1:60）触发 429，
 *      响应体断言 code(200006)/message/retry_after 字段存在（与 per-tenant 限流同构）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Tenant API daily quota [79_billing_api_quota]';
const SYS_ACCOUNT = 'super_admin';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_TOO_MANY_ATTEMPTS = 200006; // backend/pkg/errcode CodeTooManyAttempts
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

describe(SUITE, function () {
  this.timeout(120000);

  let tenantAId = null;
  let tenantBId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 79_billing_api_quota.test.js');
    }
    await apiClient.login(SYS_ACCOUNT);
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);

    const usageA = await apiClient.get('/billing/usage', {}, TENANT_A);
    expect(usageA.code, 'get tenant A usage for tenant id').to.equal(200);
    tenantAId = usageA.data.tenant_id;

    const usageB = await apiClient.get('/billing/usage', {}, TENANT_B);
    expect(usageB.code, 'get tenant B usage for tenant id').to.equal(200);
    tenantBId = usageB.data.tenant_id;
  });

  after(async function () {
    // 清理低阈值限流覆盖，避免拖累其他套件中的 tenant B（覆盖删除即时生效）。
    if (tenantBId) {
      try {
        await apiClient.delete('/ratelimit/override/tenant/' + tenantBId, {}, TENANT_B);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. returns the daily API quota contract for the current tenant', async function () {
    const resp = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(resp.code, JSON.stringify(resp)).to.equal(200);
    expect(resp.data).to.be.an('object');

    expect(resp.data).to.have.property('tenant_id', tenantAId);
    expect(resp.data).to.have.property('date').that.is.a('string').and.match(DATE_RE);
    expect(resp.data).to.have.property('plan_code').that.is.a('string').and.not.equal('');
    expect(resp.data).to.have.property('api_calls_today').that.is.a('number').at.least(0);
    expect(resp.data).to.have.property('max_api_calls_per_day').that.is.a('number');
    expect(resp.data).to.have.property('remaining').that.is.a('number');
    expect(resp.data).to.have.property('usage_pct').that.is.a('number').at.least(0);
    expect(resp.data).to.have.property('quota_status').that.is.oneOf(['normal', 'warning', 'exceeded', 'unlimited']);
  });

  it('2. accumulates same-day metering across middleware-passed calls', async function () {
    const before = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(before.code).to.equal(200);
    const usedBefore = before.data.api_calls_today;

    // 本端点自身也经过 per-tenant 限流中间件：再调两次即计入当日用量。
    await apiClient.get('/billing/api-quota', {}, TENANT_A);
    await apiClient.get('/billing/api-quota', {}, TENANT_A);

    const after = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(after.code).to.equal(200);
    expect(
      after.data.api_calls_today,
      'api_calls_today must be monotonically non-decreasing within the same day'
    ).to.be.at.least(usedBefore + 1);
  });

  it('3. keeps quota numbers consistent with the enforcement semantics', async function () {
    const resp = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(resp.code).to.equal(200);
    const { api_calls_today: used, max_api_calls_per_day: max, remaining, usage_pct: pct } = resp.data;

    if (max > 0) {
      expect(remaining, 'remaining must equal limit minus used').to.equal(Math.max(max - used, 0));
      expect(pct).to.be.at.most(100);
      if (used > 0) {
        const expectedPct = Math.round((used / max) * 1000) / 10;
        expect(pct, 'usage_pct must match used/max at one decimal').to.equal(expectedPct);
      }
    } else {
      // 限额未配置（<=0）：剩余 -1 哨兵 + unlimited 状态（不执法）。
      expect(remaining).to.equal(-1);
      expect(resp.data.quota_status).to.equal('unlimited');
    }
  });

  it('4. rejects cross-tenant quota inspection by non-sysadmin', async function () {
    const leakResp = await apiClient.get('/billing/api-quota?tenant_id=' + tenantAId, {}, TENANT_B);
    expect(leakResp.code, 'cross-tenant quota inspection must fail').to.not.equal(200);
  });

  it('5. allows sysadmin to inspect any tenant quota by tenant_id', async function () {
    const resp = await apiClient.get('/billing/api-quota?tenant_id=' + tenantAId, {}, SYS_ACCOUNT);
    expect(resp.code, JSON.stringify(resp)).to.equal(200);
    expect(resp.data).to.have.property('tenant_id', tenantAId);
  });

  it('6. returns the 429 contract shape when tenant api limit is exceeded', async function () {
    // 低阈值租户模拟：把 tenant B 的 api 限流覆盖为 1:60（1 次/分钟）。
    // 日配额与 per-tenant 限流共用同一中间件与同一 429 响应契约（code/message/retry_after）。
    const setResp = await apiClient.post(
      '/ratelimit/override',
      {
        target_type: 'tenant',
        limit_type: 'api',
        rate_limits: '1:60',
        description: '79_billing_api_quota low-threshold simulation'
      },
      TENANT_B
    );
    expect(setResp.code, JSON.stringify(setResp)).to.equal(200);

    try {
      // 第 1 笔放行，随后的调用应命中 429；apiClient 对 429 有退避重试（≤4 次），
      // 持续超限时如实返回标准化错误对象：{code: 429, data: <429 响应体>}。
      // 最多探测 3 笔：覆盖"重试期间恰好跨过 60s 窗口、新窗口首笔放行"的边界。
      let saw429 = null;
      for (let i = 0; i < 3 && !saw429; i += 1) {
        const resp = await apiClient.get('/ratelimit/config', {}, TENANT_B);
        if (resp._requestError && Number(resp.code) === 429) {
          saw429 = resp;
        }
      }

      expect(saw429, 'low-threshold tenant must trigger a 429').to.not.equal(null);
      expect(saw429.data, '429 response body must be present').to.be.an('object');
      expect(Number(saw429.data.code), '429 body code must be errcode.CodeTooManyAttempts')
        .to.equal(CODE_TOO_MANY_ATTEMPTS);
      expect(saw429.data).to.have.property('message').that.is.a('string').and.not.equal('');
      expect(saw429.data).to.have.property('retry_after').that.is.a('number').at.least(0);
    } finally {
      await apiClient.delete('/ratelimit/override/tenant/' + tenantBId, {}, TENANT_B);
    }
  });
});
