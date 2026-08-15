/**
 * 文件用途：用于验证用户与权限扩展 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const { expectBusinessError } = require('../lib/response_assertions');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectUserListRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('id', 'email', 'authority', 'tenant_id');
  expect(row.id).to.be.a('string').and.not.empty;
  expect(row.email).to.match(/^[^@\s]+@[^@\s]+$/);
  expect(row.authority).to.be.a('string').and.not.empty;
}

describe('User extra API module [13_user_extra]', function () {
  this.timeout(30000);

  const fakeId = '00000000-0000-0000-0000-000000000000';
  let firstUserId = null;
  let createdEmail = '';
  let createdUserId = null;
  const createdUserIds = new Set();

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 13_user_extra.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('super_admin');
    const detailResp = await apiClient.get('/user/detail', {}, 'super_admin');
    expectOk(detailResp);
    firstUserId = detailResp.data && detailResp.data.id;
    expect(firstUserId).to.be.a('string').and.not.empty;

    const suffix = Date.now();
    createdEmail = 'codex_user_seed_' + suffix + '@test.com';
    const createResp = await apiClient.post(
      '/user',
      {
        email: createdEmail,
        password: 'Test@2026',
        name: 'codex seed user',
        phone_number: '+86 138' + String(suffix).slice(-8)
      },
      'super_admin'
    );
    expectOk(createResp);

    const seedListResp = await apiClient.get('/user', { page: 1, page_size: 10, email: createdEmail }, 'super_admin');
    expectOk(seedListResp);
    const seeded = seedListResp.data.list[0];
    expect(seeded).to.be.an('object');
    createdUserId = seeded.id;
    expect(createdUserId).to.be.a('string').and.not.empty;
    createdUserIds.add(createdUserId);
    firstUserId = createdUserId;
  });

  after(async function () {
    if (createdUserId) {
      createdUserIds.add(createdUserId);
    }
    for (const id of createdUserIds) {
      try {
        await apiClient.delete('/user/' + id, {}, 'super_admin');
      } catch (error) {
        // Cleanup failures should not hide the real assertion result.
      }
    }
    apiClient.clearAllTokens();
  });

  it('returns the current user page shape', async function () {
    const resp = await apiClient.get('/user', { page: 1, page_size: 10 }, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.total).to.be.a('number').and.at.least(1);
    expect(resp.data.list.length).to.be.at.least(1);
    expect(resp.data.list.length).to.be.at.most(resp.data.total);
    resp.data.list.forEach(expectUserListRow);

    const seededRow = resp.data.list.find(item => item.id === createdUserId || item.email === createdEmail);
    expect(seededRow, 'seeded user must be visible in the user page').to.be.an('object');
    expectUserListRow(seededRow);
  });

  it('returns detail for the first visible user', async function () {
    expect(firstUserId).to.be.a('string').and.not.empty;

    const resp = await apiClient.get('/user/' + firstUserId, {}, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.equal(firstUserId);
    expect(resp.data.email).to.be.a('string').and.not.empty;
  });

  it('rejects user creation when phone_number is omitted', async function () {
    const resp = await apiClient.post(
      '/user',
      {
        email: 'codex_missing_phone_' + Date.now() + '@test.com',
        password: 'Test@2026',
        name: 'codex user'
      },
      'super_admin'
    );

    expectBusinessError(resp, 100002, "Field 'PhoneNumber' is required");
  });

  it('creates a user when the current required fields are supplied and finds it by email', async function () {
    const suffix = Date.now();
    createdEmail = 'codex_user_' + suffix + '@test.com';

    const createResp = await apiClient.post(
      '/user',
      {
        email: createdEmail,
        password: 'Test@2026',
        name: 'codex user',
        phone_number: '+86 139' + String(suffix).slice(-8)
      },
      'super_admin'
    );

    expectOk(createResp);

    const listResp = await apiClient.get('/user', { page: 1, page_size: 10, email: createdEmail }, 'super_admin');
    expectOk(listResp);
    expect(listResp.data.list).to.be.an('array').and.have.length.greaterThan(0);
    const created = listResp.data.list[0];
    expect(created.email).to.equal(createdEmail);
    expect(created.phone_number).to.include(String(suffix).slice(-8));
    createdUserId = created.id;
    createdUserIds.add(createdUserId);
  });

  it('returns record-not-found for invalid user update', async function () {
    const resp = await apiClient.put(
      '/user',
      {
        id: fakeId,
        name: 'invalid update'
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(101001);
    expect(resp.message).to.be.a('string').and.not.equal('');
    expect(resp.data).to.include({ error: 'record not found', user_id: fakeId });
  });

  it('returns record-not-found for invalid user deletion', async function () {
    const resp = await apiClient.delete('/user/' + fakeId, {}, 'super_admin');

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(101001);
    expect(resp.message).to.be.a('string').and.not.equal('');
    expect(resp.data).to.include({ error: 'record not found', user_id: fakeId });
  });

  it('updates the current super_admin profile remark', async function () {
    const currentResp = await apiClient.get('/user/detail', {}, 'super_admin');
    expectOk(currentResp);
    const currentUserId = currentResp.data && currentResp.data.id;
    expect(currentUserId).to.be.a('string').and.not.empty;

    const updatedRemark = 'automation_remark_' + Date.now();
    const resp = await apiClient.put(
      '/user/update',
      {
        id: currentUserId,
        // The isolated seed can contain a legacy name copied from a long
        // automation email address.  Do not echo that invalid historical
        // value back into the max=50 name validator when this case only
        // verifies remark persistence.
        remark: updatedRemark
      },
      'super_admin'
    );

    expectOk(resp);

    // 回读验证：确认 remark 变更已持久化
    const readbackResp = await apiClient.get('/user/detail', {}, 'super_admin');
    expectOk(readbackResp);
    expect(readbackResp.data).to.be.an('object');
    expect(readbackResp.data.id, '回读必须命中同一用户').to.equal(currentUserId);
    expect(readbackResp.data, '回读载荷必须包含 remark 字段').to.include.keys('remark');
    expect(
      readbackResp.data.remark,
      '更新后的 remark 必须持久化, 实际得到: ' + JSON.stringify(readbackResp.data.remark)
    ).to.equal(updatedRemark);
  });

  it('rejects change-email requests that use the previous verification_code field', async function () {
    const resp = await apiClient.post(
      '/user/change-email',
      {
        new_email: 'newemail@test.com',
        verification_code: '000000'
      },
      'super_admin'
    );

    expectBusinessError(resp, 100002, "Field 'VerifyCode' is required");
  });

  it('sets and restores the current preferred language', async function () {
    const enResp = await apiClient.post('/user/prefer-lang', { default_language: 'en-US' }, 'super_admin');
    expectOk(enResp);
    expect(enResp.data).to.include({ default_language: 'en-US', prefer_lang: 'en-US' });

    const zhResp = await apiClient.post('/user/prefer-lang', { default_language: 'zh-CN' }, 'super_admin');
    expectOk(zhResp);
    expect(zhResp.data).to.include({ default_language: 'zh-CN', prefer_lang: 'zh-CN' });
  });

  it('returns record-not-found for invalid user address update', async function () {
    const resp = await apiClient.put(
      '/user/address/' + fakeId,
      {
        address: 'test address',
        city: 'test city'
      },
      'super_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(101001);
    expect(resp.message).to.be.a('string').and.not.equal('');
    expect(resp.data).to.include({ error: 'record not found', user_id: fakeId });
  });

  it('deletes the created user', async function () {
    expect(createdUserId).to.be.a('string').and.not.empty;

    const resp = await apiClient.delete('/user/' + createdUserId, {}, 'super_admin');

    expectOk(resp);
    createdUserIds.delete(createdUserId);
    createdUserId = null;
  });
});
