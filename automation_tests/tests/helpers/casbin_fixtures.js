/**
 * 文件用途：用于支撑Casbin 权限测试夹具模块。
 * 核心逻辑：封装 API 边界、Casbin 权限或动态账号夹具，减少测试文件中的重复准备步骤。
 * 关键注意事项：夹具可能修改本地测试状态；调用方仍需显式断言业务结果并处理清理。
 * 重构建议：新增夹具时保持幂等和可诊断，避免把核心断言藏进准备函数。
 */

const { expect } = require('chai');
const crypto = require('crypto');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code, JSON.stringify(resp)).to.equal(200);
}

function expectParamError(resp, expectedData) {
  expect(resp).to.be.an('object');
  expect(resp.code, JSON.stringify(resp)).to.equal(100002);
  expect(resp.message).to.equal('请求参数验证失败');
  expect(resp.data).to.deep.include(expectedData);
}

function extractId(entity) {
  return entity && (entity.id || entity.ID || entity.user_id || entity.UserID);
}

function listFrom(resp) {
  if (Array.isArray(resp?.data)) return resp.data;
  if (Array.isArray(resp?.data?.list)) return resp.data.list;
  return [];
}

function uniqueSuffix() {
  return Date.now() + '_' + crypto.randomBytes(8).toString('hex');
}

function dynamicPassword() {
  return `Test@${crypto.randomBytes(9).toString('base64url') + '1'}`;
}

function collectUiElementIds(items, result = []) {
  if (!Array.isArray(items)) return result;
  for (const item of items) {
    const id = extractId(item);
    if (typeof id === 'string' && id) {
      result.push(id);
    }
    collectUiElementIds(item.children, result);
  }
  return result;
}

async function getAssignableFunctionIds(apiClient, accountKey = 'tenant_admin', count = 2) {
  const resp = await apiClient.get('/ui_elements/select/form', {}, accountKey);
  expectOk(resp);

  const ids = collectUiElementIds(resp.data?.list);
  expect(ids.length, 'ui_elements/select/form should expose assignable menu/function ids').to.be.at.least(count);
  return ids.slice(0, count);
}

async function createRole(apiClient, prefix = '自动化权限角色', accountKey = 'tenant_admin') {
  const name = prefix + '_' + uniqueSuffix();
  const resp = await apiClient.post(
    '/role',
    {
      name,
      description: '由自动化权限测试创建'
    },
    accountKey
  );
  expectOk(resp);

  const listResp = await apiClient.get('/role', { page: 1, page_size: 50, name }, accountKey);
  expectOk(listResp);
  const created = listFrom(listResp).find(item => item.name === name);
  expect(created, 'created role should be visible in /role list').to.be.an('object');

  const id = extractId(created);
  expect(id).to.be.a('string').and.not.equal('');
  return { id, name };
}

async function createUser(apiClient, prefix = '自动化权限用户', roleIds = [], accountKey = 'tenant_admin') {
  const suffix = uniqueSuffix();
  const password = dynamicPassword();
  const digits = suffix.replace(/\D/g, '').slice(-8).padStart(8, '0');
  const email = 'codex_casbin_' + suffix + '@test.com';
  const resp = await apiClient.post(
    '/user',
    {
      email,
      password,
      name: prefix,
      phone_number: '+86 139' + digits,
      userRoles: roleIds
    },
    accountKey
  );
  expectOk(resp);

  const listResp = await apiClient.get('/user', { page: 1, page_size: 10, email }, accountKey);
  expectOk(listResp);
  const created = listFrom(listResp).find(item => item.email === email);
  expect(created, 'created user should be visible in /user list').to.be.an('object');

  const id = extractId(created);
  expect(id).to.be.a('string').and.not.equal('');
  return { id, email };
}

async function getRoleFunctions(apiClient, roleId, accountKey = 'tenant_admin') {
  const resp = await apiClient.get('/casbin/function', { role_id: roleId }, accountKey);
  expectOk(resp);
  if (resp.data === null || resp.data === undefined) {
    return [];
  }
  expect(resp.data).to.be.an('array');
  return resp.data;
}

async function getUserRoles(apiClient, userId, accountKey = 'tenant_admin') {
  const resp = await apiClient.get('/casbin/user', { user_id: userId }, accountKey);
  expectOk(resp);
  if (resp.data === null || resp.data === undefined) {
    return [];
  }
  expect(resp.data).to.be.an('array');
  return resp.data;
}

async function cleanupCasbinFixtures(apiClient, fixture, accountKey = 'tenant_admin') {
  for (const userId of fixture.userIds || []) {
    try {
      await apiClient.delete('/casbin/user/' + userId, {}, accountKey);
    } catch (error) {
      // Best-effort cleanup only.
    }
  }

  for (const userId of fixture.userIds || []) {
    try {
      await apiClient.delete('/user/' + userId, {}, accountKey);
    } catch (error) {
      // Best-effort cleanup only.
    }
  }

  for (const roleId of fixture.roleIds || []) {
    try {
      await apiClient.delete('/casbin/function/' + roleId, {}, accountKey);
    } catch (error) {
      // Best-effort cleanup only.
    }
  }

  for (const roleId of fixture.roleIds || []) {
    try {
      await apiClient.delete('/role/' + roleId, {}, accountKey);
    } catch (error) {
      // Best-effort cleanup only.
    }
  }
}

module.exports = {
  expectOk,
  expectParamError,
  extractId,
  listFrom,
  getAssignableFunctionIds,
  createRole,
  createUser,
  getRoleFunctions,
  getUserRoles,
  cleanupCasbinFixtures
};

