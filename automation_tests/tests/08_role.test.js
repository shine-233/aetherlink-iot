/**
 * 文件用途：用于验证角色管理 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

describe('角色管理模块 [08_role]', function () {
  this.timeout(30000);

  let roleId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 08_role.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login('super_admin');

    const seedRoleName = 'seed_role_' + Date.now();
    const seedResp = await apiClient.post('/role', {
      name: seedRoleName,
      description: 'seeded by automation setup'
    }, 'super_admin');
    expect(seedResp.code).to.equal(200);
    const listResp = await apiClient.get('/role', { page: 1, page_size: 100 }, 'super_admin');
    expect(listResp.code).to.equal(200);
    const created = (listResp.data && listResp.data.list || []).find(item => item.name === seedRoleName);
    expect(created).to.be.an('object');
    roleId = created.id || (created.ID ? String(created.ID) : null);
    expect(roleId).to.be.a('string').and.not.equal('');
  });

  describe('TC-ROLE-001 角色分页查询', function () {
    it('covers TC-ROLE-001 with concrete API assertions', async function () {
      const resp = await apiClient.get('/role', { page: 1, page_size: 100 }, 'super_admin');
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.total).to.be.a('number').and.at.least(1);
      expect(resp.data.list).to.be.an('array');
      expect(resp.data.list.length).to.be.at.least(1);
      expect(resp.data.list.length).to.be.at.most(resp.data.total);
      const seededRow = resp.data.list.find(item => (item.id || (item.ID ? String(item.ID) : null)) === roleId);
      expect(seededRow, 'seeded role must be visible in the role page').to.be.an('object');
      expect(seededRow.name).to.be.a('string').and.not.equal('');
    });
  });

  describe('TC-ROLE-002 创建角色', function () {
    it('covers TC-ROLE-002 with concrete API assertions', async function () {
      const roleName = '自动化测试角色_' + Date.now();
      const resp = await apiClient.post('/role', {
        name: roleName,
        description: '由自动化测试创建'
      }, 'super_admin');
      expect(resp.code).to.equal(200);
      const listResp = await apiClient.get('/role', { page: 1, page_size: 50, name: roleName }, 'super_admin');
      expect(listResp.code).to.equal(200);
      const list = listResp.data?.list || [];
      const created = list.find(item => item.name === roleName);
      expect(created).to.be.an('object');
      roleId = created.id || (created.ID ? String(created.ID) : null);
      expect(roleId).to.be.a('string').and.not.equal('');
    });
  });

  describe('TC-ROLE-003 更新角色', function () {
    it('covers TC-ROLE-003 with concrete API assertions', async function () {
      expect(roleId).to.be.a('string').and.not.equal('');
      const updatedName = '自动化测试角色_更新_' + Date.now();
      const resp = await apiClient.put('/role', {
        id: roleId,
        name: updatedName,
        description: '由自动化测试更新'
      }, 'super_admin');
      expect(resp.code).to.equal(200);
      // 回读验证 name 已持久化（name-readback）
      const readbackResp = await apiClient.get('/role', { page: 1, page_size: 100 }, 'super_admin');
      expect(readbackResp.code).to.equal(200);
      const readbackRow = (readbackResp.data?.list || []).find(item => (item.id || (item.ID ? String(item.ID) : null)) === roleId);
      expect(readbackRow, 'updated role must still be visible after update').to.be.an('object');
      expect(readbackRow.name).to.equal(updatedName);
    });
  });

  describe('TC-ROLE-004 删除角色', function () {
    it('covers TC-ROLE-004 with concrete API assertions', async function () {
      expect(roleId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.delete('/role/' + roleId, {}, 'super_admin');
      expect(resp.code).to.equal(200);
      // 回读验证角色已从列表消失（absence-readback）
      const readbackResp = await apiClient.get('/role', { page: 1, page_size: 100 }, 'super_admin');
      expect(readbackResp.code).to.equal(200);
      const stillPresent = (readbackResp.data?.list || []).find(item => (item.id || (item.ID ? String(item.ID) : null)) === roleId);
      expect(stillPresent, 'deleted role must no longer appear in the role list').to.be.undefined;
      roleId = null;
    });
  });

  after(async function () {
    if (roleId) {
      try {
        await apiClient.delete('/role/' + roleId, {}, 'super_admin');
      } catch (e) {
        // Cleanup failure should not mask the test verdict.
      }
    }
  });
});



// asserted: role.list seeded-row-visible + name (TC-ROLE-001), role.update name-readback (TC-ROLE-003), role.delete absence-readback (TC-ROLE-004)
