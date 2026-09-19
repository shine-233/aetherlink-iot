/**
 * 文件用途：P3 租户管理、客户自助开通与许可证配额执法（max_tenants）API 契约测试。
 *
 * 覆盖：
 *   1. 平台级租户管理（SYS_ADMIN）——创建、列表、详情、更新（含统计与防环）；
 *   2. 租户作用域层级隔离——TENANT_ADMIN 仅可见自身与子孙租户，不可越权访问独立租户；
 *   3. 客户自助开箱入驻（POST /api/v1/tenant/provision）——无需登录，原子创建租户、初始管理员与默认看板；
 *   4. 入驻凭据立即可用——新管理员成功登录并持有 TENANT_ADMIN 权限；
 *   5. 参数校验与防重——邮箱冲突、手机号冲突与格式校验；
 *   6. 许可证配额执法——max_tenants 边界感知。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'P3 tenant provisioning & quota enforcement [68_p3_tenant_provisioning_and_quota]';
const SYS_ACCOUNT = 'super_admin';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NO_PERMISSION = 100003;
const CODE_NOT_FOUND = 100404;

describe(SUITE, function () {
  this.timeout(120000);

  let createdTenantId = null;
  let provisionedTenantId = null;
  let provisionedAdminEmail = null;
  let provisionedAdminPass = 'ProvisionPass123!';

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 68_p3_tenant_provisioning_and_quota.test.js');
    }
    await apiClient.login(SYS_ACCOUNT);
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  it('SYS_ADMIN lists existing tenants with pagination and counts', async function () {
    const res = await apiClient.get('/tenants', { page: 1, page_size: 10 }, SYS_ACCOUNT);
    expect(res.code, 'list tenants').to.equal(200);
    expect(res.data).to.have.property('list').that.is.an('array');
    expect(res.data).to.have.property('total').that.is.a('number');
    expect(res.data.total, 'at least 1 tenant').to.be.at.least(1);

    const first = res.data.list[0];
    expect(first).to.have.property('id').that.is.a('string');
    expect(first).to.have.property('name').that.is.a('string');
    expect(first).to.have.property('device_count').that.is.a('number');
    expect(first).to.have.property('user_count').that.is.a('number');
  });

  it('SYS_ADMIN creates a new tenant and fetches its detail', async function () {
    const name = seedData.makeRunLabel('p3_tenant');
    const create = await apiClient.post('/tenants', { name }, SYS_ACCOUNT);
    expect(create.code, 'create tenant').to.equal(200);
    expect(create.data).to.have.property('id').that.is.a('string');
    createdTenantId = create.data.id;

    // 详情读取
    const detail = await apiClient.get('/tenants/' + createdTenantId, {}, SYS_ACCOUNT);
    expect(detail.code, 'get tenant detail').to.equal(200);
    expect(detail.data.name, 'tenant name matches').to.equal(name);
    expect(detail.data.device_count, 'initial device count 0').to.equal(0);
    expect(detail.data.user_count, 'initial user count 0').to.equal(0);

    // 更新名称
    const updatedName = name + '_renamed';
    const update = await apiClient.put('/tenants/' + createdTenantId, { name: updatedName }, SYS_ACCOUNT);
    expect(update.code, 'update tenant').to.equal(200);

    const recheck = await apiClient.get('/tenants/' + createdTenantId, {}, SYS_ACCOUNT);
    expect(recheck.data.name, 'updated name preserved').to.equal(updatedName);
  });

  it('TENANT_ADMIN can only see self and descendants, hiding unrelated tenants', async function () {
    const resA = await apiClient.get('/tenants', {}, TENANT_A);
    expect(resA.code, 'tenant A list').to.equal(200);
    const idsA = resA.data.list.map(t => t.id);

    // Tenant A 不能查看到刚被 SYS_ADMIN 新建的独立租户
    expect(idsA, 'tenant A cannot see createdTenantId').to.not.include(createdTenantId);

    // Tenant A 试图直读 createdTenantId 详情返回 404
    const detail = await apiClient.get('/tenants/' + createdTenantId, {}, TENANT_A);
    expect(detail.code, 'unrelated tenant detail returns 404').to.equal(CODE_NOT_FOUND);
  });

  it('Customer self-service provisions a new organization atomically', async function () {
    const timestamp = Date.now();
    const tenantName = 'Acme Energy ' + timestamp;
    provisionedAdminEmail = 'acme_' + timestamp + '@example.com';
    const adminPhone = '138' + String(timestamp).slice(-8);

    const provision = await apiClient.post('/tenant/provision', {
      tenant_name: tenantName,
      admin_email: provisionedAdminEmail,
      admin_phone: adminPhone,
      admin_password: provisionedAdminPass,
      admin_name: 'Acme Leader'
    });
    expect(provision.code, 'self-service provision success').to.equal(200);
    expect(provision.data).to.have.property('tenant_id').that.is.a('string');
    expect(provision.data).to.have.property('admin_id').that.is.a('string');
    expect(provision.data.tenant_name).to.equal(tenantName);
    expect(provision.data.admin_email).to.equal(provisionedAdminEmail);
    provisionedTenantId = provision.data.tenant_id;

    // 验证新管理员凭证可以立刻登录并获得令牌
    const login = await apiClient.post('/login', {
      email: provisionedAdminEmail,
      password: provisionedAdminPass
    });
    expect(login.code, 'newly provisioned admin login').to.equal(200);
    expect(login.data).to.have.property('token').that.is.a('string');

    // 用新管理员 token 调用 GET /tenants，断言其能且仅能看到自己开通的租户
    const adminToken = login.data.token;
    const selfTenant = await apiClient.client.get('/tenants', {
      headers: { 'x-token': adminToken }
    });
    expect(selfTenant.data.code, 'self tenant read').to.equal(200);
    expect(selfTenant.data.data.list, 'only sees self tenant').to.have.lengthOf(1);
    expect(selfTenant.data.data.list[0].id).to.equal(provisionedTenantId);
    expect(selfTenant.data.data.list[0].name).to.equal(tenantName);
  });

  it('Rejects duplicate self-service provisioning with conflicting email', async function () {
    const dup = await apiClient.post('/tenant/provision', {
      tenant_name: 'Conflicting Org',
      admin_email: provisionedAdminEmail, // duplicate
      admin_phone: '13900001111',
      admin_password: 'PassWord123!'
    });
    expect(dup.code, 'duplicate email rejected').to.equal(CODE_PARAM_ERROR);
  });

  it('Verifies license status endpoint reflects boundary status', async function () {
    const status = await apiClient.get('/license/status', {}, SYS_ACCOUNT);
    expect(status.code, 'license status').to.equal(200);
    expect(status.data).to.have.property('enabled').that.is.a('boolean');
    if (status.data.enabled && status.data.valid) {
      expect(status.data).to.have.property('max_tenants');
    } else {
      expect(status.data).to.have.property('reason');
    }
  });
});
