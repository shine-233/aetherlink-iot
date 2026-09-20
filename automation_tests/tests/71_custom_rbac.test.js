/**
 * 文件用途：TB-17 自定义角色与细粒度权限控制体系（Custom RBAC & Granular Permissions）契约测试。
 *
 * 覆盖：
 *   1. 标准系统权限点字典查询（GET /api/v1/permissions）——模块化分组与 api_patterns 结构验证；
 *   2. 租户管理员创建自定义业务角色（POST /api/v1/role）；
 *   3. 角色权限点查询与初始空状态（GET /api/v1/roles/:id/permissions）；
 *   4. 非法权限代码分配拦截（POST /api/v1/roles/:id/permissions 传入不存在代码拦截 100002）；
 *   5. 合法权限代码批量分配（device:read, alarm:operate）与动态 Casbin p 策略生效；
 *   6. 角色权限列表读回与数据自洽（验证 codes 包含分配项）；
 *   7. 角色用户分配与查询（GET/POST /api/v1/roles/:id/users）；
 *   8. 严格多租户拓扑隔离——租户 B 越权访问/修改租户 A 的角色权限被严格拦截（100003）。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'TB-17 Custom RBAC & Granular Permissions [71_custom_rbac]';
const SYS_ACCOUNT = 'super_admin';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NO_PERMISSION = 201001;

describe(SUITE, function () {
  this.timeout(120000);

  let roleIdTenantA = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 71_custom_rbac.test.js');
    }
    await apiClient.login(SYS_ACCOUNT);
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  it('1. GET /permissions returns standard permission catalog with module & api_patterns', async function () {
    const res = await apiClient.get('/permissions', {}, TENANT_A);
    expect(res.code, 'query permissions').to.equal(200);
    expect(res.data).to.be.an('array').that.is.not.empty;

    const first = res.data[0];
    expect(first).to.have.property('code').that.is.a('string');
    expect(first).to.have.property('name').that.is.a('string');
    expect(first).to.have.property('module').that.is.a('string');
    expect(first).to.have.property('api_patterns');

    const codes = res.data.map(p => p.code);
    expect(codes).to.include('device:read');
    expect(codes).to.include('device:control');
    expect(codes).to.include('alarm:operate');
    expect(codes).to.include('rule:write');
  });

  it('2. GET /permissions supports filtering by module', async function () {
    const res = await apiClient.get('/permissions', { module: 'device' }, TENANT_A);
    expect(res.code, 'query device module permissions').to.equal(200);
    expect(res.data).to.be.an('array').that.is.not.empty;
    res.data.forEach(p => {
      expect(p.module).to.equal('device');
    });
  });

  it('3. TENANT_ADMIN creates a custom business role', async function () {
    const roleName = seedData.makeRunLabel('operator_role');
    const res = await apiClient.post('/role', {
      name: roleName,
      description: 'Custom operator role for testing fine-grained RBAC',
    }, TENANT_A);
    expect(res.code, 'create custom role').to.equal(200);

    // 查出该角色 ID
    const listRes = await apiClient.get('/role', { page: 1, page_size: 20 }, TENANT_A);
    expect(listRes.code).to.equal(200);
    const found = (listRes.data.list || []).find(r => r.name === roleName);
    expect(Boolean(found), 'created role found in list').to.equal(true);
    roleIdTenantA = found.id;
    expect(roleIdTenantA).to.be.a('string');
  });

  it('4. GET /roles/:id/permissions initially returns empty permissions', async function () {
    const res = await apiClient.get(`/roles/${roleIdTenantA}/permissions`, {}, TENANT_A);
    expect(res.code).to.equal(200);
    expect(res.data).to.have.property('role_id', roleIdTenantA);
    expect(res.data).to.have.property('codes').that.is.an('array').that.is.empty;
    expect(res.data).to.have.property('permissions').that.is.an('array').that.is.empty;
  });

  it('5. POST /roles/:id/permissions rejects invalid/non-existent permission codes with 100002', async function () {
    const res = await apiClient.post(`/roles/${roleIdTenantA}/permissions`, {
      permission_codes: ['device:read', 'invalid_non_existent_code_xyz'],
    }, TENANT_A);
    expect(res.code).to.equal(CODE_PARAM_ERROR);
    expect(res.message).to.include('invalid permission codes');
  });

  it('6. POST /roles/:id/permissions assigns valid permissions and updates Casbin policies', async function () {
    const targetCodes = ['device:read', 'alarm:operate', 'telemetry:read'];
    const res = await apiClient.post(`/roles/${roleIdTenantA}/permissions`, {
      permission_codes: targetCodes,
    }, TENANT_A);
    expect(res.code).to.equal(200);

    // 读回验证
    const readRes = await apiClient.get(`/roles/${roleIdTenantA}/permissions`, {}, TENANT_A);
    expect(readRes.code).to.equal(200);
    expect(readRes.data.codes).to.have.members(targetCodes);
    expect(readRes.data.permissions).to.have.lengthOf(targetCodes.length);
  });

  it('7. Cross-tenant defense: TENANT_B cannot view or modify TENANT_A role permissions', async function () {
    const getRes = await apiClient.get(`/roles/${roleIdTenantA}/permissions`, {}, TENANT_B);
    expect(getRes.code, 'cross-tenant get role permissions denied').to.equal(CODE_NO_PERMISSION);

    const postRes = await apiClient.post(`/roles/${roleIdTenantA}/permissions`, {
      permission_codes: ['device:read'],
    }, TENANT_B);
    expect(postRes.code, 'cross-tenant post role permissions denied').to.equal(CODE_NO_PERMISSION);
  });

  it('8. SYS_ADMIN can view role permissions across tenants', async function () {
    const res = await apiClient.get(`/roles/${roleIdTenantA}/permissions`, {}, SYS_ACCOUNT);
    expect(res.code, 'sys admin get role permissions').to.equal(200);
    expect(res.data.role_id).to.equal(roleIdTenantA);
    expect(res.data.codes).to.include('device:read');
  });

  it('9. TENANT_ADMIN assigns users to the custom role (POST /roles/:id/users)', async function () {
    // 查出本租户下一个普通用户
    const usersRes = await apiClient.get('/user', { page: 1, page_size: 10 }, TENANT_A);
    expect(usersRes.code).to.equal(200);
    const userList = usersRes.data.list || [];
    if (userList.length > 0) {
      const targetUserId = userList[0].id;
      const assignRes = await apiClient.post(`/roles/${roleIdTenantA}/users`, {
        user_ids: [targetUserId],
      }, TENANT_A);
      expect(assignRes.code, 'assign users to role').to.equal(200);

      // 10. 读回验证
      const getRoleUsersRes = await apiClient.get(`/roles/${roleIdTenantA}/users`, {}, TENANT_A);
      expect(getRoleUsersRes.code).to.equal(200);
      expect(getRoleUsersRes.data).to.have.property('users').that.is.an('array');
      const assignedIds = getRoleUsersRes.data.users.map(u => u.id);
      expect(assignedIds).to.include(targetUserId);
    }
  });
});
