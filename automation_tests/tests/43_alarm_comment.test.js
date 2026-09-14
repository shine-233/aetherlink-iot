/**
 * 文件用途：告警评论（ROADMAP TB-1 第一片）的 API 契约测试。
 *
 * 覆盖的是"告警生命周期里的协作面"这一条链路的四面：
 *   1. 入参校验：空内容、纯空白内容必须被拒（空白评论在 UI 上就是一条空气泡）。
 *   2. 存在性：对不存在的告警评论必须 404，不能 200 落一条孤儿评论。
 *   3. 正常链路：建评论 → 列表读回 → 删除 → 再读已消失。
 *   4. 租户边界：别的租户管理员既看不到也评不了本租户的告警。
 *
 * 关键注意事项：
 *   - 评论挂在 **alarm_history**（现代告警记录），不是已废弃的 alarm_info。
 *     夹具因此走 seedData.ensureSceneAlarmHistory：激活场景 → 产生一条真实告警历史。
 *   - 该夹具在环境不满足时会返回 blocked 而不是抛错。这里把它当成**硬失败**：
 *     没有真实告警历史就无法验证评论链路，跳过会让"0 覆盖"看起来像通过。
 *   - 跨租户用 tenant_admin_b（第二租户管理员），不是 TENANT_USER ——
 *     TENANT_USER 对告警历史本身就是 fail-closed，测出来的拒绝分不清是
 *     "跨租户被拦"还是"角色本身没权限"。
 *
 * 重构建议：评论支持编辑/软删后，补"编辑后列表时间序不变"与"软删不出现在列表"两条。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'Alarm comment lifecycle [43_alarm_comment]';
const ACCOUNT = 'tenant_admin';
const OTHER_TENANT_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NOT_FOUND = 100404;

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  return resp.data;
}

function expectCode(resp, code, label) {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: expected ${code} but got ${resp.code} (${resp.message || ''})`).to.equal(code);
}

function commentList(data) {
  expect(data, 'list payload').to.be.an('object');
  const list = data.list;
  expect(list, 'data.list').to.be.an('array');
  return list;
}

describe(SUITE, function () {
  this.timeout(120000);

  let historyId = null;
  let seed = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 43_alarm_comment.test.js');
    }
    await apiClient.login(ACCOUNT);

    seed = await seedData.ensureSceneAlarmHistory(ACCOUNT);
    if (seed.blocked) {
      // 不跳过：没有真实告警历史，评论链路一条都验证不了。
      throw new Error('alarm history fixture blocked: ' + seed.reason);
    }
    historyId = seed.id;
    expect(historyId, 'fixture alarm history id').to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (seed && seed.cleanup) {
      await seed.cleanup();
    }
  });

  it('rejects a comment whose content is empty', async function () {
    const resp = await apiClient.post(`/alarm/info/history/${historyId}/comment`, { content: '' }, ACCOUNT);
    expectCode(resp, CODE_PARAM_ERROR, 'empty content');
  });

  it('rejects a comment whose content is only whitespace', async function () {
    const resp = await apiClient.post(`/alarm/info/history/${historyId}/comment`, { content: '   \n\t ' }, ACCOUNT);
    expectCode(resp, CODE_PARAM_ERROR, 'whitespace-only content');
  });

  it('rejects a comment on an alarm history that does not exist', async function () {
    const resp = await apiClient.post(
      '/alarm/info/history/00000000-0000-0000-0000-000000000000/comment',
      { content: 'ghost' },
      ACCOUNT
    );
    expectCode(resp, CODE_NOT_FOUND, 'unknown alarm history');
  });

  it('creates a comment and lists it back', async function () {
    const content = 'checked the gateway logs, sensor is flapping';
    const created = expectOk(await apiClient.post(`/alarm/info/history/${historyId}/comment`, { content }, ACCOUNT));
    expect(created, 'created comment').to.be.an('object');
    expect(created.id, 'comment id').to.be.a('string').and.not.equal('');
    expect(created.alarm_history_id, 'comment is attached to the alarm history').to.equal(historyId);
    expect(created.content, 'content is stored trimmed').to.equal(content);

    const listed = commentList(expectOk(await apiClient.get(`/alarm/info/history/${historyId}/comment`, {}, ACCOUNT)));
    const found = listed.find(item => item.id === created.id);
    expect(found, 'created comment must appear in the list').to.be.an('object');
    expect(found.content).to.equal(content);
  });

  it('trims surrounding whitespace instead of storing it', async function () {
    const created = expectOk(
      await apiClient.post(`/alarm/info/history/${historyId}/comment`, { content: '   padded note   ' }, ACCOUNT)
    );
    expect(created.content, 'content must be trimmed on write').to.equal('padded note');
  });

  it('lists comments in chronological order', async function () {
    const listed = commentList(expectOk(await apiClient.get(`/alarm/info/history/${historyId}/comment`, {}, ACCOUNT)));
    expect(listed.length, 'at least the comments created above').to.be.greaterThan(1);
    for (let i = 1; i < listed.length; i += 1) {
      const prev = Date.parse(listed[i - 1].created_at);
      const curr = Date.parse(listed[i].created_at);
      expect(curr, 'comments must be ascending by created_at').to.be.greaterThanOrEqual(prev);
    }
  });

  it('deletes a comment and it disappears from the list', async function () {
    const created = expectOk(
      await apiClient.post(`/alarm/info/history/${historyId}/comment`, { content: 'to be removed' }, ACCOUNT)
    );
    expectOk(await apiClient.delete(`/alarm/info/history/${historyId}/comment/${created.id}`, {}, ACCOUNT));

    const listed = commentList(expectOk(await apiClient.get(`/alarm/info/history/${historyId}/comment`, {}, ACCOUNT)));
    expect(listed.find(item => item.id === created.id), 'deleted comment must be gone').to.equal(undefined);
  });

  it('rejects deleting a comment that does not exist', async function () {
    const resp = await apiClient.delete(
      `/alarm/info/history/${historyId}/comment/00000000-0000-0000-0000-000000000000`,
      {},
      ACCOUNT
    );
    expectCode(resp, CODE_NOT_FOUND, 'unknown comment id');
  });

  it('rejects reading comments from another tenant', async function () {
    // tenant_admin_b 看不到本租户的告警历史，因此连评论列表都进不去。
    const resp = await apiClient.get(`/alarm/info/history/${historyId}/comment`, {}, OTHER_TENANT_ACCOUNT);
    expect(resp.code, 'cross-tenant read must not succeed').to.not.equal(200);
  });

  it('rejects commenting on another tenant alarm history', async function () {
    const resp = await apiClient.post(
      `/alarm/info/history/${historyId}/comment`,
      { content: 'cross tenant write' },
      OTHER_TENANT_ACCOUNT
    );
    expect(resp.code, 'cross-tenant write must not succeed').to.not.equal(200);
  });
});
