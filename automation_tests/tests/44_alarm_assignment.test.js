/**
 * 文件用途：告警指派 + 指派历史审计（ROADMAP TB-1 第二片）的 API 契约测试。
 *
 * 覆盖的是"告警生命周期里的责任归属"这一条链路的四面：
 *   1. 入参校验：assignee_user_id 为空串必须被拒 —— NULL 才表示"取消指派"，
 *      空串会让 `IS NULL` 判定当前处理人失效。
 *   2. 存在性：对不存在的告警指派必须 404，不能 200 落一条孤儿流水。
 *   3. 正常链路：指派 → 列表读回 → 重复指派同一人（幂等追加）→ 取消指派（追加 NULL 行）。
 *   4. 租户边界：别的租户管理员既读不到也改不了本租户告警的指派；
 *      且**不能把本租户的告警指派给别的租户的人**——那是把告警内容暴露给租户外的人。
 *   5. 语义约定：流水 append-only。取消指派后历史行仍在，只是最新一行变成 NULL。
 *
 * 关键注意事项：
 *   - 指派挂在 **alarm_history**（现代告警记录），不是已废弃的 alarm_info.processor。
 *     夹具因此走 seedData.ensureSceneAlarmHistory：激活场景 → 产生一条真实告警历史。
 *   - 该夹具在环境不满足时会返回 blocked 而不是抛错。这里把它当成**硬失败**：
 *     没有真实告警历史就无法验证指派链路，跳过会让"0 覆盖"看起来像通过。
 *   - 跨租户用 tenant_admin_b（第二租户管理员），不是 TENANT_USER ——
 *     TENANT_USER 对告警历史本身就是 fail-closed，测出来的拒绝分不清是
 *     "跨租户被拦"还是"角色本身没权限"。
 *   - 列表是时间倒序：**首行即当前处理人**。这是本片对消费方的核心约定，
 *     用例里锁死（每写一条新流水，它必须出现在 index 0）。
 *
 * 重构建议：若后续支持"指派给团队/角色"或 SLA 计时，补"非用户指派"与
 * "取消指派后 current assignee 为空"的用例。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'Alarm assignment audit trail [44_alarm_assignment]';
const ACCOUNT = 'tenant_admin';
const OTHER_TENANT_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NOT_FOUND = 100404;

const MISSING_ID = '00000000-0000-0000-0000-000000000000';

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  return resp.data;
}

function expectCode(resp, code, label) {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: expected ${code} but got ${resp.code} (${resp.message || ''})`).to.equal(code);
}

function assignmentList(data) {
  expect(data, 'list payload').to.be.an('object');
  const list = data.list;
  expect(list, 'data.list').to.be.an('array');
  return list;
}

async function listAssignments(historyId, accountKey = ACCOUNT) {
  return assignmentList(expectOk(await apiClient.get(`/alarm/info/history/${historyId}/assignment`, {}, accountKey)));
}

async function currentUserId(accountKey) {
  const detail = expectOk(await apiClient.get('/user/detail', {}, accountKey));
  expect(detail.id, `${accountKey} user id`).to.be.a('string').and.not.equal('');
  return detail.id;
}

function assertDescending(listed, label) {
  for (let i = 1; i < listed.length; i += 1) {
    const prev = Date.parse(listed[i - 1].created_at);
    const curr = Date.parse(listed[i].created_at);
    expect(curr, `${label}: assignments must be descending by created_at`).to.be.at.most(prev);
  }
}

describe(SUITE, function () {
  this.timeout(120000);

  let historyId = null;
  let seed = null;
  let selfUserId = null;
  let otherTenantUserId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 44_alarm_assignment.test.js');
    }
    await apiClient.login(ACCOUNT);

    seed = await seedData.ensureSceneAlarmHistory(ACCOUNT);
    if (seed.blocked) {
      // 不跳过：没有真实告警历史，指派链路一条都验证不了。
      throw new Error('alarm history fixture blocked: ' + seed.reason);
    }
    historyId = seed.id;
    expect(historyId, 'fixture alarm history id').to.be.a('string').and.not.equal('');

    selfUserId = await currentUserId(ACCOUNT);
    otherTenantUserId = await currentUserId(OTHER_TENANT_ACCOUNT);
    expect(otherTenantUserId, 'the two accounts must be different users').to.not.equal(selfUserId);
  });

  after(async function () {
    if (seed && seed.cleanup) {
      await seed.cleanup();
    }
  });

  it('rejects an empty-string assignee (only null means unassign)', async function () {
    const resp = await apiClient.post(
      `/alarm/info/history/${historyId}/assignment`,
      { assignee_user_id: '', remark: 'empty assignee' },
      ACCOUNT
    );
    expectCode(resp, CODE_PARAM_ERROR, 'empty-string assignee');
  });

  it('rejects an assignment on an alarm history that does not exist', async function () {
    const resp = await apiClient.post(
      `/alarm/info/history/${MISSING_ID}/assignment`,
      { assignee_user_id: selfUserId },
      ACCOUNT
    );
    expectCode(resp, CODE_NOT_FOUND, 'unknown alarm history');
  });

  it('rejects listing assignments for an alarm history that does not exist', async function () {
    const resp = await apiClient.get(`/alarm/info/history/${MISSING_ID}/assignment`, {}, ACCOUNT);
    expectCode(resp, CODE_NOT_FOUND, 'unknown alarm history list');
  });

  it('assigns the alarm to a tenant user and records the operator', async function () {
    const created = expectOk(
      await apiClient.post(
        `/alarm/info/history/${historyId}/assignment`,
        { assignee_user_id: selfUserId, remark: '   take a look at this one   ' },
        ACCOUNT
      )
    );
    expect(created, 'created assignment').to.be.an('object');
    expect(created.id, 'assignment id').to.be.a('string').and.not.equal('');
    expect(created.alarm_history_id, 'assignment is attached to the alarm history').to.equal(historyId);
    expect(created.assignee_user_id, 'assignee is stored').to.equal(selfUserId);
    expect(created.operator_user_id, 'operator is the caller').to.equal(selfUserId);
    expect(created.remark, 'remark is stored trimmed').to.equal('take a look at this one');

    // 首行即当前处理人：倒序列表的第一条必须就是刚写的这条。
    const listed = await listAssignments(historyId);
    expect(listed[0].id, 'the newest assignment must be first (current assignee)').to.equal(created.id);
    expect(listed[0].assignee_user_id).to.equal(selfUserId);
  });

  it('rejects assigning to a user that belongs to another tenant', async function () {
    const resp = await apiClient.post(
      `/alarm/info/history/${historyId}/assignment`,
      { assignee_user_id: otherTenantUserId },
      ACCOUNT
    );
    expect(resp.code, 'cross-tenant assignee must not succeed').to.not.equal(200);
  });

  it('rejects assigning to a user that does not exist', async function () {
    const resp = await apiClient.post(
      `/alarm/info/history/${historyId}/assignment`,
      { assignee_user_id: MISSING_ID },
      ACCOUNT
    );
    expectCode(resp, CODE_NOT_FOUND, 'unknown assignee user');
  });

  it('accepts re-assigning the same user as an idempotent audit entry', async function () {
    const before = await listAssignments(historyId);
    const second = expectOk(
      await apiClient.post(
        `/alarm/info/history/${historyId}/assignment`,
        { assignee_user_id: selfUserId, remark: 're-confirmed' },
        ACCOUNT
      )
    );
    expect(second.assignee_user_id, 're-assignment keeps the same assignee').to.equal(selfUserId);

    const after = await listAssignments(historyId);
    expect(after.length, 're-assignment appends a new entry instead of overwriting').to.equal(before.length + 1);
    expect(after[0].id).to.equal(second.id);
    expect(after[1].assignee_user_id, 'the previous entry is not rewritten').to.equal(selfUserId);
  });

  it('lists assignments in reverse chronological order', async function () {
    const listed = await listAssignments(historyId);
    expect(listed.length, 'at least the assignments created above').to.be.greaterThan(1);
    assertDescending(listed, 'assignments');
  });

  it('cancels the assignment by appending a null row and keeps history', async function () {
    const before = await listAssignments(historyId);

    const cancelled = expectOk(
      await apiClient.post(
        `/alarm/info/history/${historyId}/assignment`,
        { assignee_user_id: null, remark: 'handing back to the queue' },
        ACCOUNT
      )
    );
    expect(cancelled.assignee_user_id, 'cancel writes a null assignee').to.equal(null);
    expect(cancelled.operator_user_id, 'cancel still records who did it').to.equal(selfUserId);

    const after = await listAssignments(historyId);
    expect(after.length, 'cancel appends a row, it does not delete history').to.equal(before.length + 1);
    expect(after[0].id).to.equal(cancelled.id);
    expect(after[0].assignee_user_id, 'current assignee is nobody').to.equal(null);
    // 历史行仍在：取消之前的指派记录没有被改写。
    expect(after[1].assignee_user_id, 'the previous assignment is still in the trail').to.equal(selfUserId);
    assertDescending(after, 'after cancel');
  });

  it('accepts an omitted assignee as a cancel', async function () {
    const created = expectOk(
      await apiClient.post(`/alarm/info/history/${historyId}/assignment`, {}, ACCOUNT)
    );
    expect(created.assignee_user_id, 'omitted assignee is treated as null').to.equal(null);
  });

  it('rejects reading assignments from another tenant', async function () {
    const resp = await apiClient.get(`/alarm/info/history/${historyId}/assignment`, {}, OTHER_TENANT_ACCOUNT);
    expect(resp.code, 'cross-tenant read must not succeed').to.not.equal(200);
  });

  it('rejects assigning another tenant alarm history', async function () {
    const resp = await apiClient.post(
      `/alarm/info/history/${historyId}/assignment`,
      { assignee_user_id: otherTenantUserId },
      OTHER_TENANT_ACCOUNT
    );
    expect(resp.code, 'cross-tenant write must not succeed').to.not.equal(200);
  });
});
