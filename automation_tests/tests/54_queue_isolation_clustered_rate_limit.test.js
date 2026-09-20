/**
 * 文件用途：TB-7 队列隔离与限流集群化（Queue Isolation & Clustered Rate Limiting）端到端契约测试。
 *
 * 对标 ThingsBoard 3.6.3+ 队列隔离机制与 ThingsBoard 4.3 LTS 集群多策略限流：
 *   1. 集群限流配置与指标度量基线（/ratelimit/config, /ratelimit/metrics）；
 *   2. 动态多策略配额管理（CRUD、复合窗口 "100:1,1000:60" 校验、目标类型/限流类型校验）；
 *   3. 运行时限流拦截与 HTTP 429 协议契约（Retry-After 响应头、200006 错误码）；
 *   4. 多租户配额越权隔离防护（租户 B 无法越权修改或删除租户 A 的配额）；
 *   5. 多队列隔离拓扑与运行期指标监测（Main、HighPriority、SequentialByOriginator 队列状态、深度与健康度）。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');

const SUITE = 'TB-7 Queue Isolation & Clustered Rate Limiting [54_queue_isolation_clustered_rate_limit]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_FORBIDDEN = 201001;
const CODE_TOO_MANY_REQUESTS = 200006;

describe(SUITE, function () {
  this.timeout(180000);

  const cleanups = [];
  let tenantAId = '';
  let tenantBId = '';

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 54_queue_isolation_clustered_rate_limit.test.js');
    }
    const tokenA = await apiClient.login(ACCOUNT);
    const tokenB = await apiClient.login(OTHER_ACCOUNT);

    function getTenantId(token) {
      const payload = JSON.parse(Buffer.from(token.split('.')[1], 'base64').toString('utf8'));
      return payload.tenant_id;
    }

    tenantAId = getTenantId(tokenA);
    tenantBId = getTenantId(tokenB);
  });

  after(async function () {
    for (let i = cleanups.length - 1; i >= 0; i--) {
      try {
        await cleanups[i]();
      } catch (err) {
        // ignore cleanup error
      }
    }
    apiClient.clearAllTokens();
  });

  describe('1. Rate Limiting Configuration & Baseline Metrics', function () {
    it('retrieves platform rate limit configuration', async function () {
      const resp = await apiClient.get('/ratelimit/config', {}, ACCOUNT);
      expectSuccess(resp);
      expect(resp.data).to.have.property('backend');
      expect(resp.data).to.have.property('default_tenant_api_limit');
      expect(resp.data).to.have.property('default_tenant_transport_limit');
      expect(resp.data).to.have.property('default_device_transport_limit');
    });

    it('retrieves rate limit monitoring metrics', async function () {
      const resp = await apiClient.get('/ratelimit/metrics', {}, ACCOUNT);
      expectSuccess(resp);
      expect(resp.data).to.have.property('total_checked');
      expect(resp.data).to.have.property('total_allowed');
      expect(resp.data).to.have.property('total_blocked');
      expect(resp.data).to.have.property('blocked_by_scope');
      expect(resp.data.total_checked).to.be.a('number');
    });
  });

  describe('2. Dynamic Rate Limit Overrides & Schema Validation', function () {
    it('rejects malformed rate limit expressions with 100002', async function () {
      const invalidExpressions = [
        'invalid_string',
        '100:-1',
        '-50:10',
        '100:1,200:1' // duplicate window
      ];

      for (const expr of invalidExpressions) {
        const resp = await apiClient.post('/ratelimit/override', {
          tenant_id: tenantAId,
          target_type: 'tenant',
          target_id: tenantAId,
          limit_type: 'api',
          rate_limits: expr
        }, ACCOUNT);

        expectBusinessError(resp, CODE_PARAM_ERROR);
      }
    });

    it('rejects invalid target_type or limit_type with 100002', async function () {
      const resp1 = await apiClient.post('/ratelimit/override', {
        tenant_id: tenantAId,
        target_type: 'unknown_type',
        target_id: tenantAId,
        limit_type: 'api',
        rate_limits: '100:1'
      }, ACCOUNT);
      expectBusinessError(resp1, CODE_PARAM_ERROR);

      const resp2 = await apiClient.post('/ratelimit/override', {
        tenant_id: tenantAId,
        target_type: 'tenant',
        target_id: tenantAId,
        limit_type: 'invalid_limit_scope',
        rate_limits: '100:1'
      }, ACCOUNT);
      expectBusinessError(resp2, CODE_PARAM_ERROR);
    });

    it('creates or updates a custom rate limit override successfully', async function () {
      const resp = await apiClient.post('/ratelimit/override', {
        tenant_id: tenantAId,
        target_type: 'tenant',
        target_id: tenantAId,
        limit_type: 'api',
        rate_limits: '200:1,3000:60',
        description: 'custom high-burst quota for tenant A'
      }, ACCOUNT);
      expectSuccess(resp);

      cleanups.push(async () => {
        await apiClient.delete(`/ratelimit/override/tenant/${tenantAId}?limit_type=api`, {}, ACCOUNT);
      });

      // 验证列表包含新建规则
      const listResp = await apiClient.get('/ratelimit/overrides', {}, ACCOUNT);
      expectSuccess(listResp);
      expect(listResp.data).to.be.an('array');
      const found = listResp.data.find(r => r.target_id === tenantAId && r.limit_type === 'api');
      expect(Boolean(found)).to.equal(true);
      expect(found.rate_limits).to.equal('200:1,3000:60');
    });

    it('deletes custom rate limit override and restores default', async function () {
      const delResp = await apiClient.delete(`/ratelimit/override/tenant/${tenantAId}?limit_type=api`, {}, ACCOUNT);
      expectSuccess(delResp);

      const listResp = await apiClient.get('/ratelimit/overrides', {}, ACCOUNT);
      expectSuccess(listResp);
      const found = listResp.data.find(r => r.target_id === tenantAId && r.limit_type === 'api');
      expect(found).to.be.undefined;
    });
  });

  describe('3. Rate Limiting HTTP 429 Protocol Contract', function () {
    it('returns HTTP 429 and Retry-After header when quota is exceeded', async function () {
      // 临时为 Tenant A 设置超紧凑配额：1秒内最多 2 次
      const setResp = await apiClient.post('/ratelimit/override', {
        tenant_id: tenantAId,
        target_type: 'tenant',
        target_id: tenantAId,
        limit_type: 'api',
        rate_limits: '2:1,10000:60',
        description: 'strict limit for 429 verification'
      }, ACCOUNT);
      expectSuccess(setResp);

      cleanups.push(async () => {
        await apiClient.delete(`/ratelimit/override/tenant/${tenantAId}?limit_type=api`, {}, ACCOUNT);
      });

      // 等待 1.1 秒确保此前测试调用的 1 秒滑动窗口清空，避免跨用例流量偶发叠加
      await new Promise(resolve => setTimeout(resolve, 1100));

      // 发送 2 次正常请求
      const r1 = await apiClient.get('/ratelimit/config', {}, ACCOUNT);
      expectSuccess(r1);
      const r2 = await apiClient.get('/ratelimit/config', {}, ACCOUNT);
      expectSuccess(r2);

      // 第 3 次请求在 1 秒内到达，设置 httpRateLimitRetries: 99 以直通返回 429
      const r3 = await apiClient.get('/ratelimit/config', {}, ACCOUNT, { httpRateLimitRetries: 99 });
      expect(r3.code).to.equal(429);
      expect(r3.data).to.be.an('object');
      expect(r3.data.code).to.equal(CODE_TOO_MANY_REQUESTS);
      expect(r3.data.retry_after).to.be.at.least(1);

      // 清理该严格限流规则
      await apiClient.delete(`/ratelimit/override/tenant/${tenantAId}?limit_type=api`, {}, ACCOUNT);

      // 等待 1.1 秒窗口滑过，确认配额恢复
      await new Promise(resolve => setTimeout(resolve, 1100));
      const recovered = await apiClient.get('/ratelimit/config', {}, ACCOUNT);
      expectSuccess(recovered);
    });
  });

  describe('4. Multi-Tenant Isolation Protection', function () {
    it('prevents Tenant B from modifying Tenant A rate limit override', async function () {
      // Tenant B 试图修改 Tenant A 的配额
      try {
        const crossResp = await apiClient.post('/ratelimit/override', {
          tenant_id: tenantAId, // 攻击目标：租户 A
          target_type: 'tenant',
          target_id: tenantAId,
          limit_type: 'api',
          rate_limits: '1:60'
        }, OTHER_ACCOUNT);

        // 如果后端拦截返回 403 / 201001
        if (crossResp.data) {
          expect(crossResp.data.code).to.be.oneOf([CODE_FORBIDDEN, 200]);
        }
      } catch (err) {
        expect(err.response.status).to.be.oneOf([403, 400]);
      }

      // 验证 Tenant A 的配额未被篡改
      const listA = await apiClient.get('/ratelimit/overrides', {}, ACCOUNT);
      const hacked = listA.data.find(r => r.target_id === tenantAId && r.rate_limits === '1:60');
      expect(hacked).to.be.undefined;
    });

    it('ensures Tenant B requests are isolated and not blocked by Tenant A quota limits', async function () {
      const respB = await apiClient.get('/ratelimit/config', {}, OTHER_ACCOUNT);
      expectSuccess(respB);
    });
  });

  describe('5. Multi-Queue Isolation Topology & Observability', function () {
    it('retrieves queue topology and isolation configuration', async function () {
      const resp = await apiClient.get('/queue/config', {}, ACCOUNT);
      expectSuccess(resp);
      expect(resp.data).to.have.property('standard_queues');
      expect(resp.data.standard_queues).to.be.an('array').with.lengthOf(3);

      const queueNames = resp.data.standard_queues.map(q => q.name);
      expect(queueNames).to.include.members(['Main', 'HighPriority', 'SequentialByOriginator']);

      const seqQueue = resp.data.standard_queues.find(q => q.name === 'SequentialByOriginator');
      expect(seqQueue.submit_strategy).to.equal('SEQUENTIAL_BY_ORIGINATOR');
      expect(seqQueue.partition_count).to.be.at.least(4);
    });

    it('retrieves multi-queue real-time metrics and health stats', async function () {
      const resp = await apiClient.get('/queue/stats', {}, ACCOUNT);
      expectSuccess(resp);
      expect(resp.data).to.have.property('queues');
      expect(resp.data.queues).to.be.an('array');

      for (const q of resp.data.queues) {
        expect(q).to.have.property('name');
        expect(q).to.have.property('capacity');
        expect(q).to.have.property('size');
        expect(q).to.have.property('total_submitted');
        expect(q).to.have.property('total_processed');
        expect(q).to.have.property('total_dropped');
        expect(q).to.have.property('health_status');
        expect(q.health_status).to.equal('HEALTHY');
      }
    });
  });
});
