/**
 * 文件用途：用于验证Casbin 权限 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectOk,
  expectParamError,
  getAssignableFunctionIds,
  createRole,
  createUser,
  getRoleFunctions,
  getUserRoles,
  cleanupCasbinFixtures
} = require('./helpers/casbin_fixtures');

describe('权限/Casbin模块 [09_casbin]', function () {
  this.timeout(30000);

  const accountKey = 'tenant_admin';
  const fixture = { roleIds: [], userIds: [] };
  let functionIds = [];
  let roleA = null;
  let roleB = null;
  let user = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 09_casbin.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login(accountKey);
    functionIds = await getAssignableFunctionIds(apiClient, accountKey, 2);
    roleA = await createRole(apiClient, '自动化Casbin角色A', accountKey);
    roleB = await createRole(apiClient, '自动化Casbin角色B', accountKey);
    fixture.roleIds.push(roleA.id, roleB.id);
    user = await createUser(apiClient, 'automation_user', [], accountKey);
    fixture.userIds.push(user.id);
  });

  after(async function () {
    await cleanupCasbinFixtures(apiClient, fixture, accountKey);
    apiClient.clearAllTokens();
  });

  describe('TC-CASBIN-001 查询角色权限', function () {
    it('新建角色应返回空权限列表', async function () {
      const functions = await getRoleFunctions(apiClient, roleA.id, accountKey);
      expect(functions).to.deep.equal([]);
    });
  });

  describe('TC-CASBIN-002 添加角色权限', function () {
    it('空权限列表应拒绝添加角色权限', async function () {
      const resp = await apiClient.post(
        '/casbin/function',
        { role_id: roleA.id, functions_ids: [] },
        accountKey
      );

      expectParamError(resp, {
        role_id: roleA.id,
        function_ids: [],
        error: 'AddFunctionToRole failed'
      });
    });

    it('covers TC-CASBIN-002 with concrete API assertions', async function () {
      const resp = await apiClient.post(
        '/casbin/function',
        { role_id: roleA.id, functions_ids: [functionIds[0]] },
        accountKey
      );
      expectOk(resp);

      const functions = await getRoleFunctions(apiClient, roleA.id, accountKey);
      expect(functions).to.include(functionIds[0]);
    });
  });

  describe('TC-CASBIN-003 更新角色权限', function () {
    it('空权限列表应拒绝更新角色权限', async function () {
      const resp = await apiClient.put(
        '/casbin/function',
        { role_id: roleB.id, functions_ids: [] },
        accountKey
      );

      expectParamError(resp, {
        role_id: roleB.id,
        function_ids: [],
        error: 'AddFunctionToRole failed'
      });
    });

    it('应替换角色权限并可查回新权限', async function () {
      const resp = await apiClient.put(
        '/casbin/function',
        { role_id: roleA.id, functions_ids: [functionIds[1]] },
        accountKey
      );
      expectOk(resp);

      const functions = await getRoleFunctions(apiClient, roleA.id, accountKey);
      expect(functions).to.include(functionIds[1]);
      expect(functions).to.not.include(functionIds[0]);
    });
  });

  describe('TC-CASBIN-004 删除角色权限', function () {
    it('应删除角色权限并查回为空', async function () {
      const resp = await apiClient.delete('/casbin/function/' + roleA.id, {}, accountKey);
      expectOk(resp);

      const functions = await getRoleFunctions(apiClient, roleA.id, accountKey);
      expect(functions).to.deep.equal([]);
    });
  });

  describe('TC-CASBIN-005 查询用户角色', function () {
    it('新建用户应返回空角色列表', async function () {
      const roles = await getUserRoles(apiClient, user.id, accountKey);
      expect(roles).to.satisfy(r => Array.isArray(r) && (r.length === 0 || (r.length === 1 && r[0] === 'TENANT_USER')));
    });
  });

  describe('TC-CASBIN-006 添加用户角色', function () {
    it('空角色列表应拒绝添加用户角色', async function () {
      const resp = await apiClient.post('/casbin/user', { user_id: user.id, roles_ids: [] }, accountKey);

      expectParamError(resp, {
        user_id: user.id,
        role_id: [],
        error: 'AddRolesToUser failed'
      });
    });

    it('covers TC-CASBIN-006 with concrete API assertions', async function () {
      const resp = await apiClient.post('/casbin/user', { user_id: user.id, roles_ids: [roleA.id] }, accountKey);
      expectOk(resp);

      const roles = await getUserRoles(apiClient, user.id, accountKey);
      expect(roles).to.include(roleA.id);
    });
  });

  describe('TC-CASBIN-007 更新用户角色', function () {
    it('空角色列表应拒绝更新用户角色', async function () {
      const resp = await apiClient.put('/casbin/user', { user_id: user.id, roles_ids: [] }, accountKey);

      expectParamError(resp, {
        user_id: user.id,
        role_id: [],
        error: 'AddRolesToUser failed'
      });
    });

    it('应替换用户角色并可查回新角色', async function () {
      const resp = await apiClient.put('/casbin/user', { user_id: user.id, roles_ids: [roleB.id] }, accountKey);
      expectOk(resp);

      const roles = await getUserRoles(apiClient, user.id, accountKey);
      expect(roles).to.include(roleB.id);
      expect(roles).to.not.include(roleA.id);
    });
  });

  describe('TC-CASBIN-008 删除用户角色', function () {
    it('应删除用户角色并查回为空', async function () {
      const resp = await apiClient.delete('/casbin/user/' + user.id, {}, accountKey);
      expectOk(resp);

      const roles = await getUserRoles(apiClient, user.id, accountKey);
      expect(roles).to.deep.equal([]);
    });
  });
});
