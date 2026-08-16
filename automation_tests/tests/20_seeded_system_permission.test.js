/**
 * 文件用途：用于验证种子系统权限业务 API 测试。
 * 核心逻辑：使用确定性本地夹具执行 API 场景，断言响应、状态变化、负向分支和清理结果。
 * 关键注意事项：只有在本地账号、种子数据和清理步骤都成功时，才可作为对应流程的业务闭环证据。
 * 重构建议：继续把数据准备、断言 oracle 和清理逻辑拆清楚，便于补充故障注入或变异验证。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
// The seeded permission suite is a live API suite, so its role assertions must
// use the same environment-only configuration as the client itself.
const config = apiClient.getConfig();
const {
  expectArray,
  expectBusinessError,
  expectPagedList,
  expectPermissionDenied,
  expectSuccess,
  expectValidationError
} = require('../lib/response_assertions');
const {
  cleanupCasbinFixtures,
  createRole,
  extractId
} = require('./helpers/casbin_fixtures');

describe('Seeded system, permission, and tenancy coverage [20_seeded_system_permission]', function () {
  this.timeout(45000);
  const fixture = { roleIds: [], userIds: [] };

  before(async function () {
    await apiClient.login('tenant_admin');
    await apiClient.login('super_admin');
  });

  after(async function () {
    await cleanupCasbinFixtures(apiClient, fixture, 'tenant_admin');
    apiClient.clearAllTokens();
  });

  function expectUserRow(row) {
    expect(row).to.be.an('object');
    expect(row.id || row.ID).to.be.a('string').and.not.equal('');
    expect(row.email).to.be.a('string').and.include('@');
    expect(row).to.have.property('authority');
    expect(row).to.have.property('tenant_id');
  }

  function expectRoleRow(row) {
    expect(row).to.be.an('object');
    expect(row.id || row.ID).to.be.a('string').and.not.equal('');
    expect(row.name).to.be.a('string').and.not.equal('');
  }

  function expectFunctionRow(row) {
    expect(row).to.be.an('object');
    const functionId = row.id || row.ID || row.value;
    expect(functionId).to.satisfy(value =>
      (typeof value === 'string' && value.trim() !== '') ||
      (typeof value === 'number' && Number.isFinite(value))
    );
    expect(row.name || row.label).to.be.a('string').and.not.equal('');
  }

  it('refreshes token and rejects protected endpoints without auth', async function () {
    const refreshResp = await apiClient.get('/user/refresh', {}, 'tenant_admin');
    expectSuccess(refreshResp);
    expect(refreshResp.data).to.have.property('token').that.is.a('string').and.not.equal('');

    expectPermissionDenied(await apiClient.getNoAuth('/user/detail'));
    expectPermissionDenied(await apiClient.putNoAuth('/user/update', {}));
  });

  it('asserts user, role, and casbin list surfaces', async function () {
    const userResp = await apiClient.get('/user', { page: 1, page_size: 10 }, 'super_admin');
    expectSuccess(userResp);
    expectPagedList(userResp.data, { rowCheck: expectUserRow });

    const detailResp = await apiClient.get('/user/detail', {}, 'super_admin');
    expectSuccess(detailResp);
    expectUserRow(detailResp.data);
    expect(detailResp.data.authority).to.equal(config.accounts.super_admin.role);

    const tenantAdminUser = userResp.data.list.find(item => item.authority === 'TENANT_ADMIN');
    expect(
      tenantAdminUser,
      'super_admin user management list should expose at least one TENANT_ADMIN row',
    ).to.be.an('object');
    expectUserRow(tenantAdminUser);
    expect(userResp.data.list.some(item => item.authority === config.accounts.super_admin.role)).to.equal(false);

    const createdRole = await createRole(apiClient, 'seeded_system_permission_role', 'tenant_admin');
    fixture.roleIds.push(createdRole.id);

    const roleResp = await apiClient.get(
      '/role',
      { page: 1, page_size: 10, name: createdRole.name },
      'tenant_admin'
    );
    expectSuccess(roleResp);
    expectPagedList(roleResp.data, { rowCheck: expectRoleRow });
    const persistedRole = roleResp.data.list.find(item => extractId(item) === createdRole.id);
    expect(persistedRole, 'created role should be persisted and visible to its tenant').to.be.an('object');

    const functionResp = await apiClient.get(
      '/casbin/function',
      { role_id: createdRole.id },
      'tenant_admin'
    );
    expectSuccess(functionResp);
    expectArray(functionResp.data, { rowCheck: expectFunctionRow });
  });

  it('asserts role and casbin mutation validation failures', async function () {
    expectValidationError(await apiClient.post('/role', {}, 'super_admin'));
    expectValidationError(await apiClient.put('/role', {}, 'super_admin'));

    const casbinResp = await apiClient.post('/casbin/function', {}, 'super_admin');
    expectBusinessError(casbinResp, 100002);
    expect(casbinResp.data).to.be.an('object');
    expect(casbinResp.data.role_id).to.equal('');
    expect(casbinResp.data.error).to.equal('AddFunctionToRole failed');
    expect(casbinResp.data).to.have.property('function_ids');
    if (casbinResp.data.function_ids !== null) {
      expect(casbinResp.data.function_ids).to.deep.equal([]);
    }
  });

  it('covers system identity, function flags, logo, and version read APIs', async function () {
    expectSuccess(await apiClient.getNoAuth('/logo'));
    expectSuccess(await apiClient.getNoAuth('/systime'));
    expectSuccess(await apiClient.getNoAuth('/sys_version'));
    expectSuccess(await apiClient.getNoAuth('/sys_function'));

    const updateResp = await apiClient.put('/sys_function/00000000-0000-0000-0000-000000000000', { enable_flag: 'N' }, 'super_admin');
    expectBusinessError(updateResp, 101001);
  });
});
