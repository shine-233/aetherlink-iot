/**
 * 文件用途：用户组与组权限（TB-46 GPE v1，131.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 用户组完整生命周期（POST /api/v1/user_group 创建/更新、GET /api/v1/user_groups 分页、
 *      GET /api/v1/user_group/:id 详情、DELETE /api/v1/user_group/:id 删除）；
 *   2. 参数校验（名称必填、同租户重名拒绝）与组权限元素绑定校验
 *      （非法命名空间 menu:、不存在/跨租户资源 board:/asset: 全部 fail-closed 拒绝）；
 *   3. 成员管理全量替换（不存在的用户被拒绝；customer 客户不接入组授权——v1 边界）；
 *   4. 组共享可见性（绑定看板→组外 TENANT_USER 不可见 fail-closed；入组后可见；
 *      清空绑定后恢复租户内可见不回归）；
 *   5. 严格多租户隔离（租户 B 无法查询/更新/删除租户 A 的用户组，列表不串租户）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'User Group Management [88_user_group_management]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';
const TENANT_USER = 'tenant_user';

const GROUP_NAME = '契约测试用户组_88';
const BOARD_NAME = '契约测试看板_88';
const CUSTOMER_NAME = '契约测试客户_88';
// 不存在的资源/非法元素码统一用固定占位 UUID，断言后端 fail-closed 拒绝。
const MISSING_ID = '00000000-0000-0000-0000-00000000aa88';

describe(SUITE, function () {
  // 看板可见性用例前提：TENANT_USER 需拥有 /board 路由权限（全新库迁移不预设该授权），
  // 未授予时组共享的看板可见性用例 skip，避免误报为组共享缺陷。
  let boardVisibilityTestable = false;
  this.timeout(120000);

  let groupId = null;
  let boardId = null;
  let customerId = null;
  let tenantUserId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 88_user_group_management.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);

  // 看板可见性用例前提：TENANT_USER 需拥有 /board 路由权限（全新库迁移不预设该授权，
  // 需先经角色-权限 API 授予；未授予时这 5 个用例 skip，避免误报为组共享缺陷）。
  try {
    const probe = await apiClient.get('/board', { page: 1, page_size: 10 }, TENANT_USER);
    boardVisibilityTestable = probe.code === 200;
  } catch (e) {
    boardVisibilityTestable = false;
  }

    await apiClient.login(TENANT_USER);

    // 前置：租户 A 种一块看板（组共享可见性的被绑定资源）。
    const boardRes = await apiClient.post(
      '/board',
      { name: BOARD_NAME, home_flag: 'N' },
      TENANT_A
    );
    expect(boardRes.code, JSON.stringify(boardRes)).to.equal(200);
    boardId = boardRes.data.id;

    // 前置：解析租户 A 的 TENANT_USER 账号 ID（成员绑定与共享可见性断言的主角）。
    const selectorRes = await apiClient.get('/user/selector', { page: 1, page_size: 100 }, TENANT_A);
    expect(selectorRes.code, JSON.stringify(selectorRes)).to.equal(200);
    const users = (selectorRes.data && selectorRes.data.list) || [];
    const hit = users.find((item) => item.user_type === 'TENANT_USER');
    expect(hit, 'tenant A must have a TENANT_USER account for sharing visibility cases').to.be.an('object');
    tenantUserId = hit.user_id;
  });


  after(async function () {
    if (groupId) {
      try {
        await apiClient.delete('/user_group/' + groupId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (boardId) {
      try {
        await apiClient.delete('/board/' + boardId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (customerId) {
      try {
        await apiClient.delete('/customer/' + customerId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. 组共享基线：未绑定任何组时看板对租户内 TENANT_USER 可见（不回归）', async function () {
    if (!boardVisibilityTestable) {
      this.skip();
    }

    const res = await apiClient.get('/board', { page: 1, page_size: 200 }, TENANT_USER);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const hit = (res.data.list || []).find((item) => item.id === boardId);
    expect(hit, 'unbound board should be tenant-visible before group binding').to.be.an('object');
  });

  it('2. POST /user_group 创建用户组并返回完整档案', async function () {
    const res = await apiClient.post(
      '/user_group',
      { name: GROUP_NAME, description: 'TB-46 契约测试用户组' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.id).to.be.a('string').and.not.equal('');
    expect(res.data.name).to.equal(GROUP_NAME);
    expect(res.data.tenant_id).to.be.a('string').and.not.equal('');
    groupId = res.data.id;
  });

  it('3. POST /user_group 同租户重名被拒绝', async function () {
    const res = await apiClient.post('/user_group', { name: GROUP_NAME }, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('4. POST /user_group 名称缺失返回参数错误', async function () {
    const res = await apiClient.post('/user_group', { description: 'no name' }, TENANT_A);
    expect(res.code).to.not.equal(200);
  });

  it('5. GET /user_groups 分页与名称过滤命中新组', async function () {
    const res = await apiClient.get(
      '/user_groups',
      { page: 1, page_size: 10, name: GROUP_NAME },
      TENANT_A
    );
    expect(res.code).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.total).to.be.a('number').and.at.least(1);
    const hit = (res.data.list || []).find((item) => item.id === groupId);
    expect(hit, 'created group should appear in list').to.be.an('object');
    expect(hit.name).to.equal(GROUP_NAME);
  });

  it('6. PUT /user_group 更新描述并回读一致', async function () {
    const res = await apiClient.put(
      '/user_group',
      { id: groupId, description: 'TB-46 契约测试用户组_改' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const detail = await apiClient.get('/user_group/' + groupId, {}, TENANT_A);
    expect(detail.code).to.equal(200);
    expect(detail.data.description).to.equal('TB-46 契约测试用户组_改');
  });

  it('7. POST /user_group/:id/permissions 非法命名空间元素码被拒绝', async function () {
    const res = await apiClient.post(
      '/user_group/' + groupId + '/permissions',
      { element_codes: ['menu:' + MISSING_ID] },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('8. POST /user_group/:id/permissions 绑定不存在的看板被拒绝（fail-closed）', async function () {
    const res = await apiClient.post(
      '/user_group/' + groupId + '/permissions',
      { element_codes: ['board:' + MISSING_ID] },
      TENANT_A
    );
    expect(res.code).to.not.equal(200);
  });

  it('9. POST /user_group/:id/permissions 绑定不存在的资产被拒绝（fail-closed）', async function () {
    const res = await apiClient.post(
      '/user_group/' + groupId + '/permissions',
      { element_codes: ['asset:' + MISSING_ID] },
      TENANT_A
    );
    expect(res.code).to.not.equal(200);
  });

  it('10. POST /user_group/:id/permissions 绑定真实看板成功且回读元素快照', async function () {
    const res = await apiClient.post(
      '/user_group/' + groupId + '/permissions',
      { element_codes: ['board:' + boardId] },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const list = await apiClient.get('/user_group/' + groupId + '/permissions', {}, TENANT_A);
    expect(list.code).to.equal(200);
    expect(list.data.elements).to.be.an('array').with.lengthOf(1);
    expect(list.data.elements[0].code).to.equal('board:' + boardId);
    expect(list.data.elements[0].kind).to.equal('board');
    expect(list.data.elements[0].id).to.equal(boardId);
    expect(list.data.elements[0].name).to.be.a('string').and.not.equal('');
  });

  it('11. 组共享 fail-closed：组绑定看板后组外 TENANT_USER 不可见', async function () {
    if (!boardVisibilityTestable) {
      this.skip();
    }

    const res = await apiClient.get('/board', { page: 1, page_size: 200 }, TENANT_USER);
    expect(res.code).to.equal(200);
    const hit = (res.data.list || []).find((item) => item.id === boardId);
    expect(hit, 'group-bound board must be hidden from non-member tenant user').to.equal(undefined);
  });

  it('12. POST /user_group/:id/users 绑定不存在的用户被拒绝（fail-closed）', async function () {
    const res = await apiClient.post('/user_group/' + groupId + '/users', { user_ids: [MISSING_ID] }, TENANT_A);
    expect(res.code).to.not.equal(200);
  });

  it('13. POST /user_group/:id/users 拒绝 customer 客户（v1 边界：customer 不接入组授权）', async function () {
    const created = await apiClient.post(
      '/customer',
      { name: CUSTOMER_NAME, email: 'customer88@example.com' },
      TENANT_A
    );
    expect(created.code, JSON.stringify(created)).to.equal(200);
    customerId = created.data.id;

    const res = await apiClient.post('/user_group/' + groupId + '/users', { user_ids: [customerId] }, TENANT_A);
    expect(res.code, 'customer id must not be accepted as group member').to.not.equal(200);
    const members = await apiClient.get('/user_group/' + groupId + '/users', {}, TENANT_A);
    const found = (members.data.users || []).find((item) => item.id === customerId);
    expect(found, 'customer must not appear in group members').to.equal(undefined);
  });

  it('14. POST /user_group/:id/users 绑定租户内 TENANT_USER 成功且回读', async function () {
    const res = await apiClient.post('/user_group/' + groupId + '/users', { user_ids: [tenantUserId] }, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const members = await apiClient.get('/user_group/' + groupId + '/users', {}, TENANT_A);
    expect(members.code).to.equal(200);
    const found = (members.data.users || []).find((item) => item.id === tenantUserId);
    expect(found, 'tenant user should appear in group members').to.be.an('object');
  });

  it('15. 组共享生效：组内 TENANT_USER 重新可见绑定看板', async function () {
    if (!boardVisibilityTestable) {
      this.skip();
    }

    const res = await apiClient.get('/board', { page: 1, page_size: 200 }, TENANT_USER);
    expect(res.code).to.equal(200);
    const hit = (res.data.list || []).find((item) => item.id === boardId);
    expect(hit, 'group member should see the group-bound board').to.be.an('object');
  });

  it('16. 全量替换语义：清空成员后组外成员重新不可见（fail-closed 恢复）', async function () {
    if (!boardVisibilityTestable) {
      this.skip();
    }

    const res = await apiClient.post('/user_group/' + groupId + '/users', { user_ids: [] }, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const list = await apiClient.get('/board', { page: 1, page_size: 200 }, TENANT_USER);
    const hit = (list.data.list || []).find((item) => item.id === boardId);
    expect(hit, 'board must be hidden again after member removal').to.equal(undefined);
  });

  it('17. 全量替换语义：清空组权限绑定后看板恢复租户内可见（不回归）', async function () {
    if (!boardVisibilityTestable) {
      this.skip();
    }

    const res = await apiClient.post('/user_group/' + groupId + '/permissions', { element_codes: [] }, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const list = await apiClient.get('/board', { page: 1, page_size: 200 }, TENANT_USER);
    const hit = (list.data.list || []).find((item) => item.id === boardId);
    expect(hit, 'unbound board should be tenant-visible again after unbinding').to.be.an('object');
  });

  it('18. 租户隔离：租户 B 无法查询/更新/删除租户 A 的用户组', async function () {
    const detail = await apiClient.get('/user_group/' + groupId, {}, TENANT_B);
    expect(detail.code).to.not.equal(200);

    const update = await apiClient.put(
      '/user_group',
      { id: groupId, description: 'cross-tenant attack' },
      TENANT_B
    );
    expect(update.code).to.not.equal(200);

    const members = await apiClient.post('/user_group/' + groupId + '/users', { user_ids: [] }, TENANT_B);
    expect(members.code).to.not.equal(200);

    const permissions = await apiClient.post(
      '/user_group/' + groupId + '/permissions',
      { element_codes: [] },
      TENANT_B
    );
    expect(permissions.code).to.not.equal(200);

    const del = await apiClient.delete('/user_group/' + groupId, {}, TENANT_B);
    expect(del.code).to.not.equal(200);

    const check = await apiClient.get('/user_group/' + groupId, {}, TENANT_A);
    expect(check.code, 'group must survive tenant B attacks').to.equal(200);
  });

  it('19. 列表隔离：租户 B 的用户组列表不含租户 A 的组', async function () {
    const res = await apiClient.get(
      '/user_groups',
      { page: 1, page_size: 50, name: GROUP_NAME },
      TENANT_B
    );
    expect(res.code).to.equal(200);
    expect(res.data.total).to.equal(0);
    expect(res.data.list || []).to.have.lengthOf(0);
  });

  it('20. DELETE /user_group/:id 删除后组与成员/权限接口均不可再访问', async function () {
    const res = await apiClient.delete('/user_group/' + groupId, {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);

    const detail = await apiClient.get('/user_group/' + groupId, {}, TENANT_A);
    expect(detail.code).to.not.equal(200);

    const members = await apiClient.get('/user_group/' + groupId + '/users', {}, TENANT_A);
    expect(members.code).to.not.equal(200);

    groupId = null;
  });
});
