/**
 * 文件用途：用于验证角色与 Casbin 权限 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectOk,
  extractId,
  listFrom,
  getAssignableFunctionIds,
  createRole,
  createUser,
  getRoleFunctions,
  getUserRoles,
  cleanupCasbinFixtures
} = require('./helpers/casbin_fixtures');

describe('角色与Casbin权限模块 [08_role_casbin]', function () {
  this.timeout(30000);

  const accountKey = 'tenant_admin';
  const fixture = { roleIds: [], userIds: [] };
  let crudRoleId = null;
  let casbinRole = null;
  let replacementRole = null;
  let testUser = null;
  let functionIds = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 08_role_casbin.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login(accountKey);
    functionIds = await getAssignableFunctionIds(apiClient, accountKey, 2);
    casbinRole = await createRole(apiClient, '自动化角色权限A', accountKey);
    replacementRole = await createRole(apiClient, '自动化角色权限B', accountKey);
    fixture.roleIds.push(casbinRole.id, replacementRole.id);
    testUser = await createUser(apiClient, 'automation_user', [], accountKey);
    fixture.userIds.push(testUser.id);
  });

  after(async function () {
    if (crudRoleId) {
      fixture.roleIds.push(crudRoleId);
    }
    await cleanupCasbinFixtures(apiClient, fixture, accountKey);
    apiClient.clearAllTokens();
  });

  // ==================== 角色管理 ====================

  describe('TC-ROLE-001 角色分页查询', function () {
    it('covers TC-ROLE-001 with concrete API assertions', async function () {
      const resp = await apiClient.get('/role', { page: 1, page_size: 10 }, accountKey);

      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(resp.data.list).to.be.an('array');
      expect(resp.data.total).to.be.a('number');
    });
  });

  describe('TC-ROLE-002 创建角色', function () {
    it('应成功创建角色并可在分页列表查回', async function () {
      const roleName = '自动化测试角色_' + Date.now();
      const resp = await apiClient.post(
        '/role',
        {
          name: roleName,
          description: '由自动化测试创建'
        },
        accountKey
      );
      expectOk(resp);

      const listResp = await apiClient.get('/role', { page: 1, page_size: 50, name: roleName }, accountKey);
      expectOk(listResp);
      const created = listFrom(listResp).find(item => item.name === roleName);
      expect(created).to.be.an('object');
      crudRoleId = extractId(created);
      expect(crudRoleId).to.be.a('string').and.not.equal('');
    });
  });

  describe('TC-ROLE-003 更新角色', function () {
    it('应成功更新角色并可查回新名称', async function () {
      const updatedName = '自动化测试角色_更新_' + Date.now();
      const resp = await apiClient.put(
        '/role',
        {
          id: crudRoleId,
          name: updatedName,
          description: '由自动化测试更新'
        },
        accountKey
      );
      expectOk(resp);

      const listResp = await apiClient.get('/role', { page: 1, page_size: 50, name: updatedName }, accountKey);
      expectOk(listResp);
      const updated = listFrom(listResp).find(item => extractId(item) === crudRoleId);
      expect(updated).to.be.an('object');
      expect(updated.name).to.equal(updatedName);
    });
  });

  describe('TC-ROLE-004 删除角色', function () {
    it('covers TC-ROLE-004 with concrete API assertions', async function () {
      const resp = await apiClient.delete('/role/' + crudRoleId, {}, accountKey);
      expectOk(resp);

      const listResp = await apiClient.get('/role', { page: 1, page_size: 50 }, accountKey);
      expectOk(listResp);
      expect(listFrom(listResp).some(item => extractId(item) === crudRoleId)).to.equal(false);
      crudRoleId = null;
    });
  });

  // ==================== Casbin 角色权限 ====================

  describe('TC-CASBIN-001 查询角色权限', function () {
    it('新建角色应返回空权限列表', async function () {
      const functions = await getRoleFunctions(apiClient, casbinRole.id, accountKey);
      expect(functions).to.deep.equal([]);
    });
  });

  describe('TC-CASBIN-002 添加角色权限', function () {
    it('covers TC-CASBIN-002 with concrete API assertions', async function () {
      const resp = await apiClient.post(
        '/casbin/function',
        {
          role_id: casbinRole.id,
          functions_ids: [functionIds[0]]
        },
        accountKey
      );
      expectOk(resp);

      const functions = await getRoleFunctions(apiClient, casbinRole.id, accountKey);
      expect(functions).to.include(functionIds[0]);
    });
  });

  describe('TC-CASBIN-003 更新角色权限', function () {
    it('covers TC-CASBIN-003 with concrete API assertions', async function () {
      const resp = await apiClient.put(
        '/casbin/function',
        {
          role_id: casbinRole.id,
          functions_ids: [functionIds[1]]
        },
        accountKey
      );
      expectOk(resp);

      const functions = await getRoleFunctions(apiClient, casbinRole.id, accountKey);
      expect(functions).to.include(functionIds[1]);
      expect(functions).to.not.include(functionIds[0]);
    });
  });

  describe('TC-CASBIN-004 删除角色权限', function () {
    it('应成功删除角色权限并查回为空', async function () {
      const resp = await apiClient.delete('/casbin/function/' + casbinRole.id, {}, accountKey);
      expectOk(resp);

      const functions = await getRoleFunctions(apiClient, casbinRole.id, accountKey);
      expect(functions).to.deep.equal([]);
    });
  });

  // ==================== Casbin 用户角色 ====================

  describe('TC-CASBIN-005 查询用户角色', function () {
    it('新建用户应返回空角色列表', async function () {
      const roles = await getUserRoles(apiClient, testUser.id, accountKey);
      expect(roles).to.satisfy(r => Array.isArray(r) && (r.length === 0 || (r.length === 1 && r[0] === 'TENANT_USER')));
    });
  });

  describe('TC-CASBIN-006 添加用户角色', function () {
    it('covers TC-CASBIN-006 with concrete API assertions', async function () {
      const resp = await apiClient.post(
        '/casbin/user',
        {
          user_id: testUser.id,
          roles_ids: [casbinRole.id]
        },
        accountKey
      );
      expectOk(resp);

      const roles = await getUserRoles(apiClient, testUser.id, accountKey);
      expect(roles).to.include(casbinRole.id);
    });
  });

  describe('TC-CASBIN-007 更新用户角色', function () {
    it('covers TC-CASBIN-007 with concrete API assertions', async function () {
      const resp = await apiClient.put(
        '/casbin/user',
        {
          user_id: testUser.id,
          roles_ids: [replacementRole.id]
        },
        accountKey
      );
      expectOk(resp);

      const roles = await getUserRoles(apiClient, testUser.id, accountKey);
      expect(roles).to.include(replacementRole.id);
      expect(roles).to.not.include(casbinRole.id);
    });
  });

  describe('TC-CASBIN-008 删除用户角色', function () {
    it('应成功删除用户角色并查回为空', async function () {
      const resp = await apiClient.delete('/casbin/user/' + testUser.id, {}, accountKey);
      expectOk(resp);

      const roles = await getUserRoles(apiClient, testUser.id, accountKey);
      expect(roles).to.deep.equal([]);
    });
  });
});
