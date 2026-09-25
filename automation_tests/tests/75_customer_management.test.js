/**
 * 文件用途：Customer 客户管理体系（ThingsBoard 核心实体对标，122.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 客户完整 CRUD 生命周期（POST /api/v1/customer 创建/更新、GET /api/v1/customers 分页、
 *      GET /api/v1/customer/:id 详情、DELETE /api/v1/customer/:id 删除）；
 *   2. 参数校验（名称必填）；
 *   3. 设备分配校验（不存在/不属于本租户的设备被拒绝）；
 *   4. 严格多租户隔离（租户 B 无法查询/删除租户 A 的客户）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Customer Management [75_customer_management]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

describe(SUITE, function () {
  this.timeout(120000);

  let createdCustomerId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 75_customer_management.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (createdCustomerId) {
      try {
        await apiClient.delete('/customer/' + createdCustomerId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. POST /customer 创建客户并返回完整档案', async function () {
    const res = await apiClient.post(
      '/customer',
      {
        name: '契约测试客户_75',
        country: '中国',
        state: '浙江',
        city: '杭州',
        address: '西湖区文一西路 969 号',
        phone: '13800000000',
        email: 'customer75@example.com'
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.id).to.be.a('string').and.not.equal('');
    expect(res.data.name).to.equal('契约测试客户_75');
    expect(res.data.tenant_id).to.be.a('string').and.not.equal('');
    createdCustomerId = res.data.id;
  });

  it('2. POST /customer 名称缺失返回参数错误', async function () {
    const res = await apiClient.post('/customer', { phone: '13800000000' }, TENANT_A);
    expect(res.code).to.not.equal(200);
  });

  it('3. GET /customers 分页检索命中新客户', async function () {
    const res = await apiClient.get('/customers', { page: 1, page_size: 10, search: '契约测试客户_75' }, TENANT_A);
    expect(res.code).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.total).to.be.a('number').and.at.least(1);
    const hit = (res.data.list || []).find((item) => item.id === createdCustomerId);
    expect(hit, 'created customer should appear in list').to.be.an('object');
    expect(hit.email).to.equal('customer75@example.com');
  });

  it('4. POST /customer 携带 id 更新客户档案', async function () {
    const res = await apiClient.post(
      '/customer',
      { id: createdCustomerId, name: '契约测试客户_75_改', city: '宁波' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.name).to.equal('契约测试客户_75_改');
    expect(res.data.city).to.equal('宁波');
  });

  it('5. GET /customer/:id 查询客户详情', async function () {
    const res = await apiClient.get('/customer/' + createdCustomerId, {}, TENANT_A);
    expect(res.code).to.equal(200);
    expect(res.data.id).to.equal(createdCustomerId);
    expect(res.data.name).to.equal('契约测试客户_75_改');
  });

  it('6. POST /customer/:id/devices 拒绝不存在或跨租户的设备', async function () {
    const res = await apiClient.post(
      '/customer/' + createdCustomerId + '/devices',
      { device_ids: ['00000000-0000-0000-0000-00000000aa75'] },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('7. 租户 B 无法查询租户 A 的客户详情', async function () {
    const res = await apiClient.get('/customer/' + createdCustomerId, {}, TENANT_B);
    expect(res.code).to.not.equal(200);
  });

  it('8. 租户 B 无法删除租户 A 的客户', async function () {
    const res = await apiClient.delete('/customer/' + createdCustomerId, {}, TENANT_B);
    expect(res.code).to.not.equal(200);
    const check = await apiClient.get('/customer/' + createdCustomerId, {}, TENANT_A);
    expect(check.code).to.equal(200);
  });

  it('9. DELETE /customer/:id 删除后不可再查询', async function () {
    const res = await apiClient.delete('/customer/' + createdCustomerId, {}, TENANT_A);
    expect(res.code).to.equal(200);
    const check = await apiClient.get('/customer/' + createdCustomerId, {}, TENANT_A);
    expect(check.code).to.not.equal(200);
    createdCustomerId = null;
  });
});
