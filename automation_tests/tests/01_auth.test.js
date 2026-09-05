/**
 * 文件用途：用于验证认证与用户 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
// Use the same environment-only account source as the live API client. The
// file-backed runtime_config module is for offline/preflight fixtures and must
// not feed credentials from config.json into an outbound request.
const config = apiClient.getConfig();
const {
  createTenantAdminAccount,
  createTenantUserAccount,
  cleanupDynamicAccounts
} = require('./helpers/dynamic_accounts');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectRejected(resp, expectedCode) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(expectedCode);
  expect(resp.message).to.be.a('string').and.not.equal('');
}

function expectUserSelectorRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('user_id', 'email');
  expect(row.user_id).to.be.a('string').and.not.equal('');
  expect(row.email).to.match(/^[^@\s]+@[^@\s]+$/);
}

describe('Auth API module [01_auth]', function () {
  this.timeout(30000);

  let tenantUserAccountKey = null;
  let tenantAdminBAccountKey = null;
  const dynamicAccounts = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 01_auth.test.js; unified verification requires a healthy API service');
    }

    if (await apiClient.isAccountAvailable('tenant_user')) {
      tenantUserAccountKey = 'tenant_user';
    } else {
      const account = await createTenantUserAccount(apiClient);
      dynamicAccounts.push(account);
      tenantUserAccountKey = account.accountKey;
    }

    if (await apiClient.isAccountAvailable('tenant_admin_b')) {
      tenantAdminBAccountKey = 'tenant_admin_b';
    } else {
      const account = await createTenantAdminAccount(apiClient, 'codex_auth_tenant_b');
      dynamicAccounts.push(account);
      tenantAdminBAccountKey = account.accountKey;
    }
  });

  after(async function () {
    await cleanupDynamicAccounts(apiClient, dynamicAccounts);
    apiClient.clearAllTokens();
  });

  it('logs in as super admin and returns a durable token', async function () {
    const token = await apiClient.login('super_admin');
    expect(token).to.be.a('string');
    expect(token.length).to.be.greaterThan(10);
  });

  it('logs in as tenant admin and returns a durable token', async function () {
    const token = await apiClient.login('tenant_admin');
    expect(token).to.be.a('string');
    expect(token.length).to.be.greaterThan(10);
  });

  it('logs in as tenant user when the optional local account exists', async function () {
    const token = await apiClient.getToken(tenantUserAccountKey);
    expect(token).to.be.a('string');
    expect(token.length).to.be.greaterThan(10);
  });

  it('rejects login with a wrong password using the current business error', async function () {
    const resp = await apiClient.postNoAuth('/login', {
      email: config.accounts.tenant_admin.email,
      password: 'WrongPassword@2026'
    });

    expectRejected(resp, 200002);
    expect(resp.message).to.equal('用户名或密码错误');
  });

  it('rejects login for a nonexistent user with the same current error contract', async function () {
    const resp = await apiClient.postNoAuth('/login', {
      email: 'nonexistent@test.com',
      password: 'Test@2026'
    });

    expectRejected(resp, 200002);
    expect(resp.message).to.equal('用户名或密码错误');
  });

  it('returns the current tenant admin profile', async function () {
    const resp = await apiClient.get('/user/detail', {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.email).to.equal(config.accounts.tenant_admin.email);
    expect(resp.data.authority).to.equal(config.accounts.tenant_admin.role);
  });

  it('rejects protected profile access without auth using the current 401 contract', async function () {
    const resp = await apiClient.getNoAuth('/user/detail');

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(401);
    expect(resp._requestError).to.equal(true);
    expect(resp.message).to.equal('missing authentication (x-token or x-api-key required)');
    expect(resp.data).to.be.an('object');
    expect(resp.data.code).to.equal(40100);
    expect(resp.data.message).to.equal('missing authentication (x-token or x-api-key required)');
  });

  it('returns the current tenant id for tenant admin', async function () {
    const resp = await apiClient.get('/user/tenant/id', {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.a('string');
    expect(resp.data).to.not.equal('');
  });

  it('returns the user selector list with paging metadata', async function () {
    const resp = await apiClient.get('/user/selector', { page: 1, page_size: 10 }, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number').and.at.least(1);
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.list.length).to.be.at.least(1);
    expect(resp.data.list.length).to.be.at.most(resp.data.total);
    resp.data.list.forEach(expectUserSelectorRow);

    const tenantAdminRow = resp.data.list.find(item => item.email === config.accounts.tenant_admin.email);
    expect(tenantAdminRow, 'tenant_admin account must be visible in the user selector').to.be.an('object');
    expectUserSelectorRow(tenantAdminRow);
  });

  it('updates prefer language and restores the original local default', async function () {
    const updateResp = await apiClient.put('/user/prefer-lang', {
      default_language: 'en-US'
    }, 'tenant_admin');

    expectOk(updateResp);
    expect(updateResp.data).to.be.an('object');
    expect(updateResp.data.default_language).to.equal('en-US');
    expect(updateResp.data.prefer_lang).to.equal('en-US');

    // 持久化重读：偏好语言必须真实落库，而不只是回显请求值。
    const persistedResp = await apiClient.get('/user/detail', {}, 'tenant_admin');
    expectOk(persistedResp);
    expect(persistedResp.data.default_language).to.equal('en-US');

    const restoreResp = await apiClient.put('/user/prefer-lang', {
      default_language: 'zh-CN'
    }, 'tenant_admin');

    expectOk(restoreResp);
    expect(restoreResp.data.default_language).to.equal('zh-CN');
    expect(restoreResp.data.prefer_lang).to.equal('zh-CN');

    const restoredDetailResp = await apiClient.get('/user/detail', {}, 'tenant_admin');
    expectOk(restoredDetailResp);
    expect(restoredDetailResp.data.default_language).to.equal('zh-CN');
  });

  it('refreshes the tenant admin token and returns expiry metadata', async function () {
    const resp = await apiClient.get('/user/refresh', {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.token).to.be.a('string');
    expect(resp.data.token.length).to.be.greaterThan(10);
    expect(resp.data.expires_in).to.be.a('number');
    expect(resp.data.expires_in).to.be.greaterThan(0);
  });

  it('keeps tenant isolation checks honest when the second tenant exists', async function () {
    const respA = await apiClient.get('/device', { page: 1, page_size: 10 }, 'tenant_admin');
    const respB = await apiClient.get('/device', { page: 1, page_size: 10 }, tenantAdminBAccountKey);

    expectOk(respA);
    expectOk(respB);

    const listA = respA.data && Array.isArray(respA.data.list) ? respA.data.list : [];
    const listB = respB.data && Array.isArray(respB.data.list) ? respB.data.list : [];
    const idsA = new Set(listA.map(item => item.id || item.ID).filter(Boolean));
    const overlap = listB.filter(item => idsA.has(item.id || item.ID));

    expect(overlap.length).to.equal(0);
  });
});
