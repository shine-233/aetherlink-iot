/**
 * 文件用途：用于支撑动态账号测试夹具模块。
 * 核心逻辑：封装 API 边界、Casbin 权限或动态账号夹具，减少测试文件中的重复准备步骤。
 * 关键注意事项：夹具可能修改本地测试状态；调用方仍需显式断言业务结果并处理清理。
 * 重构建议：新增夹具时保持幂等和可诊断，避免把核心断言藏进准备函数。
 */

const { expect } = require('chai');
const crypto = require('crypto');

function dynamicPassword() {
  return `Test@${crypto.randomBytes(18).toString('base64url')}`;
}

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code, JSON.stringify(resp)).to.equal(200);
}

function uniqueSuffix() {
  return Date.now() + '_' + crypto.randomBytes(8).toString('hex');
}

function extractId(entity) {
  return entity && (entity.id || entity.ID || entity.user_id || entity.UserID);
}

function listFrom(resp) {
  if (Array.isArray(resp?.data)) return resp.data;
  if (Array.isArray(resp?.data?.list)) return resp.data.list;
  return [];
}

async function createTenantAdminAccount(apiClient, prefix = 'codex_share_recipient') {
  const suffix = uniqueSuffix();
  const password = dynamicPassword();
  const digits = suffix.replace(/\D/g, '').slice(-8).padStart(8, '0');
  const email = prefix + '_' + suffix + '@test.com';

  await apiClient.login('super_admin');
  const createResp = await apiClient.post(
    '/user',
    {
      email,
      password,
      name: '动态收件租户',
      phone_number: '+86 138' + digits
    },
    'super_admin'
  );
  expectOk(createResp);

  const listResp = await apiClient.get('/user', { page: 1, page_size: 10, email }, 'super_admin');
  expectOk(listResp);
  const created = listFrom(listResp).find(item => item.email === email);
  expect(created, 'created tenant admin should be visible to super_admin').to.be.an('object');
  const userId = extractId(created);
  expect(userId).to.be.a('string').and.not.equal('');

  const loginResp = await apiClient.postNoAuth('/login', { email, password });
  expectOk(loginResp);
  expect(loginResp.data).to.be.an('object');
  expect(loginResp.data.token).to.be.a('string').and.not.equal('');

  const accountKey = 'dynamic_recipient_' + suffix;
  apiClient.tokens[accountKey] = loginResp.data.token;

  return { accountKey, userId, email };
}

async function createTenantUserAccount(apiClient, prefix = 'codex_tenant_user') {
  const suffix = uniqueSuffix();
  const password = dynamicPassword();
  const digits = suffix.replace(/\D/g, '').slice(-8).padStart(8, '0');
  const email = prefix + '_' + suffix + '@test.com';

  await apiClient.login('tenant_admin');
  const createResp = await apiClient.post(
    '/user',
    {
      email,
      password,
      name: '动态租户用户',
      phone_number: '+86 137' + digits
    },
    'tenant_admin'
  );
  expectOk(createResp);

  const listResp = await apiClient.get('/user', { page: 1, page_size: 10, email }, 'tenant_admin');
  expectOk(listResp);
  const created = listFrom(listResp).find(item => item.email === email);
  expect(created, 'created tenant user should be visible to tenant_admin').to.be.an('object');
  const userId = extractId(created);
  expect(userId).to.be.a('string').and.not.equal('');

  const loginResp = await apiClient.postNoAuth('/login', { email, password });
  expectOk(loginResp);
  expect(loginResp.data).to.be.an('object');
  expect(loginResp.data.token).to.be.a('string').and.not.equal('');

  const accountKey = 'dynamic_tenant_user_' + suffix;
  apiClient.tokens[accountKey] = loginResp.data.token;

  return { accountKey, userId, email, ownerAccountKey: 'tenant_admin' };
}

async function cleanupDynamicAccounts(apiClient, accounts) {
  if (!accounts || accounts.length === 0) return;

  for (const account of accounts) {
    if (account.accountKey) {
      apiClient.clearToken(account.accountKey);
    }
    if (!account.userId) continue;
    try {
      const ownerAccountKey = account.ownerAccountKey || 'super_admin';
      await apiClient.login(ownerAccountKey);
      await apiClient.delete('/user/' + account.userId, {}, ownerAccountKey);
    } catch (error) {
      // Best-effort cleanup; test assertions have already run.
    }
  }
}

module.exports = {
  createTenantAdminAccount,
  createTenantUserAccount,
  cleanupDynamicAccounts
};
