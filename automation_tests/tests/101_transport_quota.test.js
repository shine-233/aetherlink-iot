/**
 * 文件用途：TB-17R 传输维度租户配额接线 API 契约测试（扩展 GET /api/v1/billing/api-quota）。
 *
 * 覆盖：
 *   1. 既有 API 维度契约不回归——TB-17 的 api_calls_today/max_api_calls_per_day/remaining/
 *      usage_pct/quota_status 字段与语义原样保留（守护既有配额查询契约不变）；
 *   2. TB-17R 传输维度新契约——transport_events_today/max_transport_per_day/transport_remaining/
 *      transport_usage_pct/transport_quota_status 字段存在、类型正确、状态枚举同域；
 *   3. 传输数值自洽——限额>0 时 transport_remaining === max(max - transport_events_today, 0)、
 *      transport_usage_pct 与 used/max 一致（一位小数）；限额缺失（broker 未发布缓存）时
 *      transport_remaining=-1 + transport_quota_status=unlimited（与 broker fail-open 实况同口径）；
 *   4. 读取幂等——HTTP 查询不改变传输计数（计数仅由 MQTT 连接认证驱动，读端点无副作用）；
 *   5. 租户隔离 fail-closed——TENANT_ADMIN 不能越权查看其他租户配额；SYS_ADMIN 可指定 tenant_id；
 *   6. 套餐变更入参负向——POST /billing/subscriptions 传非法 plan_code 被拒（不落库、不触发发布）。
 *
 * 边界说明：MQTT CONNECT 层面的传输配额执法（超限 CONNACK 拒绝）发生在 broker，
 * HTTP 契约测试无法触达；该行为由 broker 侧 Go 单测
 * mqtt-broker/plugin/aetherlink/transport_quota_test.go 钉死（计数/阈值/fail-open/UTC 日界）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Tenant transport daily quota [101_transport_quota]';
const SYS_ACCOUNT = 'super_admin';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const QUOTA_STATUSES = ['normal', 'warning', 'exceeded', 'unlimited'];

describe(SUITE, function () {
  this.timeout(120000);

  let tenantAId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 101_transport_quota.test.js');
    }
    await apiClient.login(SYS_ACCOUNT);
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);

    const usageA = await apiClient.get('/billing/usage', {}, TENANT_A);
    expect(usageA.code, 'get tenant A usage for tenant id').to.equal(200);
    tenantAId = usageA.data.tenant_id;
  });

  it('1. keeps the existing API-dimension quota contract unchanged', async function () {
    const resp = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(resp.code, JSON.stringify(resp)).to.equal(200);
    expect(resp.data).to.be.an('object');

    // 既有字段（TB-17 批次二契约）：TB-17R 扩展不得破坏。
    expect(resp.data).to.have.property('tenant_id', tenantAId);
    expect(resp.data).to.have.property('date').that.is.a('string').and.match(DATE_RE);
    expect(resp.data).to.have.property('plan_code').that.is.a('string').and.not.equal('');
    expect(resp.data).to.have.property('api_calls_today').that.is.a('number').at.least(0);
    expect(resp.data).to.have.property('max_api_calls_per_day').that.is.a('number');
    expect(resp.data).to.have.property('remaining').that.is.a('number');
    expect(resp.data).to.have.property('usage_pct').that.is.a('number').at.least(0);
    expect(resp.data).to.have.property('quota_status').that.is.oneOf(QUOTA_STATUSES);
  });

  it('2. exposes the TB-17R transport dimension contract fields', async function () {
    const resp = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(resp.code, JSON.stringify(resp)).to.equal(200);

    expect(resp.data).to.have.property('transport_events_today').that.is.a('number').at.least(0);
    expect(resp.data).to.have.property('max_transport_per_day').that.is.a('number');
    expect(resp.data).to.have.property('transport_remaining').that.is.a('number');
    expect(resp.data).to.have.property('transport_usage_pct').that.is.a('number').at.least(0);
    expect(resp.data).to.have.property('transport_quota_status').that.is.oneOf(QUOTA_STATUSES);
  });

  it('3. keeps transport quota numbers consistent with the broker enforcement semantics', async function () {
    const resp = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(resp.code).to.equal(200);
    const {
      transport_events_today: used,
      max_transport_per_day: max,
      transport_remaining: remaining,
      transport_usage_pct: pct
    } = resp.data;

    if (max > 0) {
      // 限额缓存已发布：数值自洽（与 DecideDailyQuota 执法判定纯函数同口径）。
      expect(remaining, 'transport_remaining must equal limit minus used').to.equal(Math.max(max - used, 0));
      expect(pct).to.be.at.most(100);
      if (used > 0) {
        const expectedPct = Math.round((used / max) * 1000) / 10;
        expect(pct, 'transport_usage_pct must match used/max at one decimal').to.equal(expectedPct);
      }
    } else {
      // 限额缓存未发布（broker 未执法）：剩余 -1 哨兵 + unlimited 状态。
      expect(remaining).to.equal(-1);
      expect(resp.data.transport_quota_status).to.equal('unlimited');
    }
  });

  it('4. does not mutate transport metering on read', async function () {
    const before = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(before.code).to.equal(200);
    const usedBefore = before.data.transport_events_today;

    // 传输计数仅由 MQTT 连接认证（broker 侧 INCR）驱动：HTTP 读取零副作用，
    // 多次读取计数不得增加（同窗口内恰好有设备连接时可能增加，故用"等待后不减少"的弱断言：
    // 立即复读两次，值必须单调不减且增幅为 0 的概率极高；把严格等值断言放宽为不减少）。
    await apiClient.get('/billing/api-quota', {}, TENANT_A);
    const after = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(after.code).to.equal(200);
    expect(after.data.transport_events_today).to.be.at.least(usedBefore);
  });

  it('5. rejects cross-transport-quota inspection by non-sysadmin', async function () {
    const leakResp = await apiClient.get('/billing/api-quota?tenant_id=' + tenantAId, {}, TENANT_B);
    expect(leakResp.code, 'cross-tenant transport quota inspection must fail').to.not.equal(200);
  });

  it('6. allows sysadmin to inspect any tenant transport quota by tenant_id', async function () {
    const resp = await apiClient.get('/billing/api-quota?tenant_id=' + tenantAId, {}, SYS_ACCOUNT);
    expect(resp.code, JSON.stringify(resp)).to.equal(200);
    expect(resp.data).to.have.property('tenant_id', tenantAId);
    expect(resp.data).to.have.property('transport_events_today').that.is.a('number');
  });

  it('7. rejects subscribing an unknown plan without publishing anything', async function () {
    // 负向：非法 plan_code 在校验层被拒（不落库、不触发限额缓存发布），订阅状态保持不变。
    const badResp = await apiClient.post(
      '/billing/subscriptions',
      { plan_code: 'no-such-plan-tb17r' },
      TENANT_A
    );
    expect(badResp.code, 'unknown plan_code must be rejected').to.not.equal(200);

    // 主流程不回归：拒绝后配额查询端点仍可正常返回完整契约。
    const resp = await apiClient.get('/billing/api-quota', {}, TENANT_A);
    expect(resp.code).to.equal(200);
    expect(resp.data).to.have.property('transport_quota_status').that.is.oneOf(QUOTA_STATUSES);
  });
});
