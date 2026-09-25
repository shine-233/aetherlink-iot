/**
 * 文件用途：数据策略（data_policy）读写契约活栈测试（TB-15，ROADMAP §4.1）。
 *
 * 覆盖：
 *   1. GET /api/v1/datapolicy 分页读取契约（sys_admin 专属，含设备数据 data_type=1 行的
 *      retention_days 字段口径——它是 TimescaleDB retention policy 与冷层清理的配置源）；
 *   2. PUT /api/v1/datapolicy 写入契约（更新 retention_days 后读回，结束后恢复原值）；
 *   3. 参数校验负例（retention_days 非法、enabled 非法、remark 缺失）；
 *   4. 权限/租户隔离：非 SYS_ADMIN（tenant_admin / tenant_admin_b）读写均被拒绝。
 *
 * 边界：retention policy 在 TimescaleDB 的实际生效（drop_chunks 真删数据）需要活库的
 *       timescaledb 扩展与后台作业，不在 HTTP 契约可观察范围内——见交付 residual。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Data Policy Contract [84_data_policy]';
const SYS_ADMIN = 'super_admin';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

function expectOk(resp) {
  expect(resp.code, JSON.stringify(resp)).to.equal(200);
}

function expectDenied(resp) {
  expect(resp.code, JSON.stringify(resp)).to.not.equal(200);
}

describe(SUITE, function () {
  this.timeout(120000);

  let devicePolicy = null; // 设备数据（data_type=1）策略行快照
  let originalSnapshot = null; // {retention_days, enabled, remark} 用于 after 恢复

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 84_data_policy.test.js');
    }
    await apiClient.login(SYS_ADMIN);
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    // 恢复设备数据策略原值：data_policy 是全局清理行为配置，测试不得留下漂移。
    // remark 带 required 校验（空串会被拒），原值为空时写入恢复标识。
    if (originalSnapshot && devicePolicy) {
      try {
        await apiClient.put(
          '/datapolicy',
          {
            id: devicePolicy.id,
            retention_days: originalSnapshot.retention_days,
            enabled: originalSnapshot.enabled,
            remark: originalSnapshot.remark || 'TB-84 恢复原值（原 remark 为空）'
          },
          SYS_ADMIN
        );
      } catch (e) {
        /* 恢复失败不掩盖用例结果 */
      }
    }
    apiClient.clearAllTokens();
  });

  it('1. GET /datapolicy super_admin 读取分页列表，含设备数据策略行', async function () {
    const res = await apiClient.get('/datapolicy', { page: 1, page_size: 10 }, SYS_ADMIN);
    expectOk(res);
    expect(res.data).to.be.an('object');
    expect(res.data.total).to.be.a('number').and.at.least(1);
    expect(res.data.list).to.be.an('array');
    devicePolicy = res.data.list.find((row) => row.data_type === '1');
    expect(devicePolicy, '种子数据应含 data_type=1（设备数据）策略行').to.be.an('object');
    expect(devicePolicy.id).to.be.a('string').and.not.equal('');
    expect(devicePolicy.retention_days, 'retention_days 是 retention/冷层清理的配置源，必须为正整数')
      .to.be.a('number').and.at.least(1);
    expect(devicePolicy.enabled).to.be.oneOf(['1', '2']);
    originalSnapshot = {
      retention_days: devicePolicy.retention_days,
      enabled: devicePolicy.enabled,
      remark: devicePolicy.remark === null || devicePolicy.remark === undefined ? '' : devicePolicy.remark
    };
  });

  it('2. PUT /datapolicy 更新设备数据保留天数并读回生效', async function () {
    expect(devicePolicy, '依赖用例 1 的策略行快照').to.be.an('object');
    // 选一个与当前值不同的合法天数（1~3650），保证"写入→读回"能观察到位移。
    const targetDays = originalSnapshot.retention_days === 90 ? 60 : 90;
    const res = await apiClient.put(
      '/datapolicy',
      {
        id: devicePolicy.id,
        retention_days: targetDays,
        enabled: originalSnapshot.enabled,
        remark: 'TB-15 契约测试临时值'
      },
      SYS_ADMIN
    );
    expectOk(res);

    const read = await apiClient.get('/datapolicy', { page: 1, page_size: 50 }, SYS_ADMIN);
    expectOk(read);
    const hit = read.data.list.find((row) => row.id === devicePolicy.id);
    expect(hit, '更新后的策略行应在列表中').to.be.an('object');
    expect(hit.retention_days).to.equal(targetDays);
  });

  it('3. PUT /datapolicy retention_days=0 被校验拒绝（口径：1~3650）', async function () {
    const res = await apiClient.put(
      '/datapolicy',
      {
        id: devicePolicy.id,
        retention_days: 0,
        enabled: '1',
        remark: 'negative'
      },
      SYS_ADMIN
    );
    expectDenied(res);
    const read = await apiClient.get('/datapolicy', { page: 1, page_size: 50 }, SYS_ADMIN);
    const hit = read.data.list.find((row) => row.id === devicePolicy.id);
    expect(hit.retention_days).to.not.equal(0);
  });

  it('4. PUT /datapolicy enabled 非法取值被校验拒绝', async function () {
    const res = await apiClient.put(
      '/datapolicy',
      {
        id: devicePolicy.id,
        retention_days: originalSnapshot.retention_days,
        enabled: '9',
        remark: 'negative'
      },
      SYS_ADMIN
    );
    expectDenied(res);
  });

  it('5. PUT /datapolicy remark 缺失被校验拒绝', async function () {
    const res = await apiClient.put(
      '/datapolicy',
      {
        id: devicePolicy.id,
        retention_days: originalSnapshot.retention_days,
        enabled: originalSnapshot.enabled
      },
      SYS_ADMIN
    );
    expectDenied(res);
  });

  it('6. 租户管理员（tenant_admin）无权读写数据策略', async function () {
    const read = await apiClient.get('/datapolicy', { page: 1, page_size: 10 }, TENANT_A);
    expectDenied(read);
    const write = await apiClient.put(
      '/datapolicy',
      {
        id: devicePolicy.id,
        retention_days: 1,
        enabled: '1',
        remark: '越权尝试'
      },
      TENANT_A
    );
    expectDenied(write);
  });

  it('7. 租户 B（tenant_admin_b）同样无权读写，策略值未被越权修改', async function () {
    const read = await apiClient.get('/datapolicy', { page: 1, page_size: 10 }, TENANT_B);
    expectDenied(read);
    const write = await apiClient.put(
      '/datapolicy',
      {
        id: devicePolicy.id,
        retention_days: 1,
        enabled: '1',
        remark: '越权尝试'
      },
      TENANT_B
    );
    expectDenied(write);
    const check = await apiClient.get('/datapolicy', { page: 1, page_size: 50 }, SYS_ADMIN);
    const hit = check.data.list.find((row) => row.id === devicePolicy.id);
    expect(hit.retention_days).to.not.equal(1);
  });
});
