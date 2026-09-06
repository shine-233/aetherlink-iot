/**
 * 文件用途：用于验证仪表盘与看板 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
// Keep live API assertions on the environment-only client configuration;
// runtime_config is reserved for file-backed offline/preflight fixtures.
const runtimeConfig = apiClient.getConfig();

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectBoardSummary(resp) {
  expectOk(resp);
  expect(resp.data).to.be.an('object');
  expect(resp.data).to.include.keys('device_total', 'device_on', 'device_offline');
  expect(resp.data.device_total).to.be.a('number');
  expect(resp.data.device_on).to.be.a('number');
  expect(resp.data.device_offline).to.be.a('number');
}

function expectBoardListRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('id', 'name', 'home_flag');
  expect(row.id).to.be.a('string').and.not.equal('');
  expect(row.name).to.be.a('string').and.not.equal('');
  expect(row.home_flag).to.be.oneOf(['Y', 'N']);
}

function expectBoardTrendPoint(point) {
  expect(point).to.be.an('object');
  expect(point).to.include.keys('timestamp', 'device_total', 'device_online', 'device_offline');
  expect(point.timestamp).to.be.a('string').and.not.empty;
  expect(Number.isNaN(Date.parse(point.timestamp))).to.equal(false);
  expect(point.device_total).to.be.a('number');
  expect(point.device_online).to.be.a('number');
  expect(point.device_offline).to.be.a('number');
}

describe('Board API module [07_board]', function () {
  this.timeout(30000);

  let boardId = null;
  let createdBoardName = '';
  let updatedBoardName = '';

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 07_board.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');
    await apiClient.login('super_admin');

    createdBoardName = 'codex-board-seed-' + Date.now();
    const seedResp = await apiClient.post(
      '/board',
      {
        name: createdBoardName,
        home_flag: 'N'
      },
      'tenant_admin'
    );
    expectOk(seedResp);
    boardId = seedResp.data.id;
    expect(boardId).to.be.a('string').and.not.empty;
  });

  after(async function () {
    if (boardId) {
      try {
        await apiClient.delete('/board/' + boardId, {}, 'tenant_admin');
      } catch (error) {
        // Cleanup failures should not hide the real assertion result.
      }
    }
    apiClient.clearAllTokens();
  });

  it('returns the tenant board list with list and total fields', async function () {
    const resp = await apiClient.get('/board', { page: 1, page_size: 10 }, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.total).to.be.a('number').and.at.least(1);
    expect(resp.data.list.length).to.be.at.least(1);
    expect(resp.data.list.length).to.be.at.most(resp.data.total);
    resp.data.list.forEach(expectBoardListRow);

    const createdRow = resp.data.list.find(item => item.id === boardId);
    expect(createdRow, 'seeded tenant board must be visible in the board list').to.be.an('object');
    expectBoardListRow(createdRow);
    expect(createdRow.name).to.equal(createdBoardName);
  });

  it('creates a tenant board with the current local payload shape', async function () {
    createdBoardName = 'codex-board-' + Date.now();

    const resp = await apiClient.post(
      '/board',
      {
        name: createdBoardName,
        home_flag: 'N'
      },
      'tenant_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.be.a('string').and.not.empty;
    expect(resp.data.name).to.equal(createdBoardName);
    expect(resp.data.home_flag).to.equal('N');
    boardId = resp.data.id;
  });

  it('returns the created board detail', async function () {
    expect(boardId).to.be.a('string').and.not.empty;

    const resp = await apiClient.get('/board/' + boardId, {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.equal(boardId);
    expect(resp.data.name).to.equal(createdBoardName);
    expect(resp.data.home_flag).to.equal('N');
  });

  it('updates the created board', async function () {
    expect(boardId).to.be.a('string').and.not.empty;

    updatedBoardName = 'codex-board-updated-' + Date.now();
    const resp = await apiClient.put(
      '/board',
      {
        id: boardId,
        name: updatedBoardName
      },
      'tenant_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.equal(boardId);
    expect(resp.data.name).to.equal(updatedBoardName);
  });

  it('deletes a dedicated board and verifies it disappears from detail and list', async function () {
    const deleteBoardName = 'codex-board-delete-' + Date.now();
    const createResp = await apiClient.post(
      '/board',
      { name: deleteBoardName, home_flag: 'N' },
      'tenant_admin'
    );
    expectOk(createResp);
    const doomedId = createResp.data.id;
    expect(doomedId).to.be.a('string').and.not.empty;

    const deleteResp = await apiClient.delete('/board/' + doomedId, {}, 'tenant_admin');
    expectOk(deleteResp);

    // 删除后单读必须不可用。当前后端对已删除板返回 101001（DB 错误）而非
    // 100404（资源不存在），语义不一致已作为程序缺陷记录（审计 2026-09-04）；
    // 这里断言“不再可用”这一业务事实，两个非 200 码都接受。
    const goneResp = await apiClient.get('/board/' + doomedId, {}, 'tenant_admin');
    expect(goneResp).to.be.an('object');
    expect([100404, 101001]).to.include(goneResp.code);

    const listResp = await apiClient.get('/board', { page: 1, page_size: 100 }, 'tenant_admin');
    expectOk(listResp);
    expect(listResp.data.list.map(item => item.id)).to.not.include(doomedId);
  });

  it('returns the current home board object with serialized config', async function () {
    const resp = await apiClient.get('/board/home', {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.be.a('string').and.not.empty;
    expect(resp.data.home_flag).to.equal('Y');
    expect(resp.data.config).to.be.a('string');
  });

  it('returns device trend points for the last 24 hours', async function () {
    const now = Math.floor(Date.now() / 1000);
    const resp = await apiClient.get(
      '/board/trend',
      {
        start_time: now - 24 * 60 * 60,
        end_time: now
      },
      'tenant_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.points).to.be.an('array');
    resp.data.points.forEach(expectBoardTrendPoint);
  });

  it('returns the current device total consistently with the device summary', async function () {
    const [totalResp, summaryResp] = await Promise.all([
      apiClient.get('/board/device/total', {}, 'tenant_admin'),
      apiClient.get('/board/device', {}, 'tenant_admin')
    ]);

    expectOk(totalResp);
    expectBoardSummary(summaryResp);
    expect(totalResp.data).to.be.a('number');
    expect(totalResp.data).to.equal(summaryResp.data.device_total);
  });

  it('returns the current tenant device summary', async function () {
    const resp = await apiClient.get('/board/device', {}, 'tenant_admin');

    expectBoardSummary(resp);
  });

  it('publishes a native board and reads the published payload without authentication', async function () {
    const publishName = 'codex-public-native-board-' + Date.now();
    const createResp = await apiClient.post(
      '/board',
      {
        name: publishName,
        home_flag: 'N',
        menu_flag: 'N',
        vis_type: 'native',
        config: JSON.stringify({ version: 1, columns: 24, rowHeight: 60, widgets: [] })
      },
      'tenant_admin'
    );
    expectOk(createResp);
    const publicBoardId = createResp.data.id;
    expect(publicBoardId).to.be.a('string').and.not.empty;

    try {
      const publishResp = await apiClient.post(
        '/board/' + publicBoardId + '/publish',
        {},
        'tenant_admin'
      );
      expectOk(publishResp);
      expect(publishResp.data).to.be.an('object');
      expect(publishResp.data.id).to.equal(publicBoardId);
      expect(publishResp.data.vis_type).to.equal('native');
      expect(publishResp.data.published).to.equal(true);
      expect(publishResp.data.share_token).to.be.a('string').and.not.empty;

      const sharedResp = await apiClient.getNoAuth(
        '/board/shared/' + encodeURIComponent(publishResp.data.share_token)
      );
      expectOk(sharedResp);
      expect(sharedResp.data).to.be.an('object');
      expect(sharedResp.data.id).to.equal(publicBoardId);
      expect(sharedResp.data.name).to.equal(publishName);
      expect(sharedResp.data.vis_type).to.equal('native');
      expect(sharedResp.data.published).to.equal(true);
      expect(sharedResp.data.share_token).to.equal(publishResp.data.share_token);
    } finally {
      await apiClient.delete('/board/' + publicBoardId, {}, 'tenant_admin');
    }
  });

  it('returns the tenant-scoped device summary endpoint', async function () {
    const resp = await apiClient.get('/board/tenant/device/info', {}, 'tenant_admin');

    expectBoardSummary(resp);
  });

  it('returns the tenant-scoped user summary for tenant_admin', async function () {
    const resp = await apiClient.get('/board/tenant/user/info', {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data).to.include.keys('user_total', 'user_added_yesterday', 'user_added_month', 'user_list_month');
    expect(resp.data.user_total).to.be.a('number');
  });

  it('rejects tenant overview for tenant_admin in the current local deployment', async function () {
    const resp = await apiClient.get('/board/tenant', {}, 'tenant_admin');

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(201001);
    expect(resp.message).to.equal('no permission to query tenant overview');
  });

  it('returns tenant overview for super_admin', async function () {
    const resp = await apiClient.get('/board/tenant', {}, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data).to.include.keys('user_total', 'user_added_yesterday', 'user_added_month', 'user_list_month');
  });

  it('returns the current tenant_admin profile when queried with the tenant token', async function () {
    const resp = await apiClient.get('/board/user/info', {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.authority).to.equal('TENANT_ADMIN');
    expect(resp.data.email).to.equal(runtimeConfig.accounts.tenant_admin.email);
    expect(resp.data.name).to.be.a('string').and.not.equal('');
  });

  it('returns the current super_admin profile when queried with the super token', async function () {
    const resp = await apiClient.get('/board/user/info', {}, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.authority).to.equal('SYS_ADMIN');
    expect(resp.data.email).to.match(/^[^@\s]+@[^@\s]+$/);
    expect(resp.data.name).to.be.a('string').and.not.equal('');
  });

  it('updates tenant_admin additional_info and persists the new note', async function () {
    const note = 'codex-board-note-' + Date.now();
    const updateResp = await apiClient.post(
      '/board/user/update',
      {
        additional_info: JSON.stringify({ test_note: note })
      },
      'tenant_admin'
    );

    expectOk(updateResp);

    const profileResp = await apiClient.get('/board/user/info', {}, 'tenant_admin');
    expectOk(profileResp);
    expect(profileResp.data.additional_info).to.be.a('string').and.include(note);
  });

  it('rejects password change when the old password is wrong using the current frontend payload shape', async function () {
    const resp = await apiClient.post(
      '/board/user/update/password',
      {
        old_password: 'definitely-wrong-' + Date.now(),
        password: 'Test@2026',
        salt: null
      },
      'tenant_admin'
    );

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(200045);
    expect(resp.message).to.be.a('string').and.not.equal('');
  });

  it('deletes the created board', async function () {
    expect(boardId).to.be.a('string').and.not.empty;

    const resp = await apiClient.delete('/board/' + boardId, {}, 'tenant_admin');

    expectOk(resp);
    boardId = null;
  });
});
