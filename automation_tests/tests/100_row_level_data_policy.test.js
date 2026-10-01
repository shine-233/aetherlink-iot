/**
 * 文件用途：行级数据保留 TTL（TB-15R，138.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 全局默认行基线：GET /api/v1/datapolicy 返回的两条全局行（tenant_id 为 null）
 *      语义不变（1.sql 种子：设备数据 30 天 / 操作日志 15 天），这是"全局回落"的基线；
 *   2. 行级 CRUD：POST /api/v1/datapolicy 创建租户级（device_config_id=null）与档案级
 *      （device_config_id 非空）行 → GET 列表回读身份列 → PUT 修改保留天数 →
 *      DELETE /api/v1/datapolicy/:id 删除；
 *   3. 行级唯一性（138.sql uq_data_policy_row_level 部分唯一索引，device_config_id 经
 *      COALESCE 归一）：同 (租户, 档案) 重复创建被拒；同 (租户, NULL) 租户级重复创建被拒；
 *   4. 覆盖语义的 API 契约：行级行的身份列（tenant_id/device_config_id）与保留天数
 *      独立落库、独立更新、独立删除，全局行不受行级 CRUD 影响（列表仍含两条全局行）；
 *      注：删除层面的设备级覆盖语义（全局清理不越权删除被行级覆盖设备的数据）由
 *      backend/internal/service/datapolicy_test.go 的
 *      TestCleanSystemDataByCronRowLevelOverride 在 sqlite 夹具上锁定，本活栈测试不重复搭建
 *      设备+遥测夹具；
 *   5. 输入契约：行级仅支持设备数据（data_type=2 行级创建被拒）；租户必填；
 *   6. 保护契约：全局默认行删除被拒（服务层 CodeOpDenied）；
 *   7. 鉴权契约：TENANT_ADMIN 走 casbin 放行的路径仍被服务层 SYS_ADMIN 校验拒绝。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Row-Level Data Policy TTL [100_row_level_data_policy]';
const ADMIN = 'super_admin';
const TENANT_A = 'tenant_admin';

// 合法形态的档案 id（后端不校验档案存在性，只校验格式 ≤36 字符）。
const SYNTHETIC_PROFILE_ID = 'c0ffee00-0000-4000-8000-00000000c0de';

describe(SUITE, function () {
  this.timeout(120000);

  let tenantId = null;
  let tenantLevelPolicyId = null;
  let profileLevelPolicyId = null;
  const createdPolicyIds = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 100_row_level_data_policy.test.js');
    }
    await apiClient.login(ADMIN);
    await apiClient.login(TENANT_A);

    // 取 tenant_admin 所属租户 id（行级策略的身份锚点）。
    const info = await apiClient.get('/board/user/info', {}, TENANT_A);
    expect(info.code, JSON.stringify(info)).to.equal(200);
    tenantId = info.data && (info.data.tenant_id || info.data.tenantId);
    expect(tenantId, 'board/user/info should carry tenant_id').to.be.a('string').and.not.equal('');
  });

  after(async function () {
    for (const id of createdPolicyIds) {
      try {
        await apiClient.delete('/datapolicy/' + id, {}, ADMIN);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. GET /datapolicy 返回两条全局默认行（tenant_id 为空，语义不变）', async function () {
    const res = await apiClient.get('/datapolicy', { page: 1, page_size: 50 }, ADMIN);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.total).to.be.a('number').and.at.least(2);
    const list = res.data.list || [];
    const globalDeviceRow = list.find((item) => item.data_type === '1' && !item.tenant_id);
    const globalOplogRow = list.find((item) => item.data_type === '2' && !item.tenant_id);
    expect(globalDeviceRow, 'global device-data policy row should exist').to.be.an('object');
    expect(globalOplogRow, 'global operation-log policy row should exist').to.be.an('object');
    expect(globalDeviceRow.device_config_id).to.equal(null);
    expect(globalDeviceRow.retention_days).to.be.a('number').and.above(0);
  });

  it('2. POST /datapolicy 创建租户级行级策略（device_config_id 为空）', async function () {
    const res = await apiClient.post(
      '/datapolicy',
      {
        data_type: '1',
        tenant_id: tenantId,
        device_config_id: null,
        retention_days: 90,
        enabled: '1',
        remark: '契约测试行级策略_100_租户级'
      },
      ADMIN
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    tenantLevelPolicyId = res.data && res.data.id;
    createdPolicyIds.push(tenantLevelPolicyId);
  });

  it('3. POST /datapolicy 创建档案级行级策略（device_config_id 非空）', async function () {
    const res = await apiClient.post(
      '/datapolicy',
      {
        data_type: '1',
        tenant_id: tenantId,
        device_config_id: SYNTHETIC_PROFILE_ID,
        retention_days: 7,
        enabled: '1',
        remark: '契约测试行级策略_100_档案级'
      },
      ADMIN
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    profileLevelPolicyId = res.data && res.data.id;
    createdPolicyIds.push(profileLevelPolicyId);
  });

  it('4. GET /datapolicy 回读行级行：身份列与保留天数独立落库', async function () {
    const res = await apiClient.get('/datapolicy', { page: 1, page_size: 50 }, ADMIN);
    expect(res.code).to.equal(200);
    const list = res.data.list || [];

    const tenantRow = list.find((item) => item.id === tenantLevelPolicyId);
    expect(tenantRow, 'tenant-level row should appear in list').to.be.an('object');
    expect(tenantRow.tenant_id).to.equal(tenantId);
    expect(tenantRow.device_config_id).to.equal(null);
    expect(tenantRow.retention_days).to.equal(90);

    const profileRow = list.find((item) => item.id === profileLevelPolicyId);
    expect(profileRow, 'profile-level row should appear in list').to.be.an('object');
    expect(profileRow.tenant_id).to.equal(tenantId);
    expect(profileRow.device_config_id).to.equal(SYNTHETIC_PROFILE_ID);
    expect(profileRow.retention_days).to.equal(7);

    // 覆盖语义基线：行级 CRUD 之后全局行仍在且身份列仍为空。
    const globalRows = list.filter((item) => !item.tenant_id);
    expect(globalRows.length).to.be.at.least(2);
  });

  it('5. PUT /datapolicy 更新行级行的保留天数与启停', async function () {
    const res = await apiClient.put(
      '/datapolicy',
      {
        id: tenantLevelPolicyId,
        retention_days: 120,
        enabled: '1',
        remark: '契约测试行级策略_100_租户级_更新'
      },
      ADMIN
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);

    const list = await apiClient.get('/datapolicy', { page: 1, page_size: 50 }, ADMIN);
    const row = (list.data.list || []).find((item) => item.id === tenantLevelPolicyId);
    expect(row, 'updated row should appear in list').to.be.an('object');
    expect(row.retention_days).to.equal(120);
    // 更新不得改写行级身份列（GORM 零值跳过语义）。
    expect(row.tenant_id).to.equal(tenantId);
    expect(row.device_config_id).to.equal(null);
  });

  it('6. POST /datapolicy 同 (租户, 档案) 重复创建被唯一索引拒绝', async function () {
    const res = await apiClient.post(
      '/datapolicy',
      {
        data_type: '1',
        tenant_id: tenantId,
        device_config_id: SYNTHETIC_PROFILE_ID,
        retention_days: 30,
        enabled: '1',
        remark: '契约测试行级策略_100_重复档案级'
      },
      ADMIN
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('7. POST /datapolicy 同 (租户, NULL) 租户级重复创建被拒（COALESCE 归一唯一）', async function () {
    const res = await apiClient.post(
      '/datapolicy',
      {
        data_type: '1',
        tenant_id: tenantId,
        device_config_id: null,
        retention_days: 30,
        enabled: '1',
        remark: '契约测试行级策略_100_重复租户级'
      },
      ADMIN
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('8. POST /datapolicy data_type=2 的行级行被拒（行级仅支持设备数据）', async function () {
    const res = await apiClient.post(
      '/datapolicy',
      {
        data_type: '2',
        tenant_id: tenantId,
        device_config_id: null,
        retention_days: 15,
        enabled: '1',
        remark: '契约测试行级策略_100_操作日志'
      },
      ADMIN
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('9. DELETE 全局默认行被拒（全局回落基线不可删除）', async function () {
    const res = await apiClient.get('/datapolicy', { page: 1, page_size: 50 }, ADMIN);
    const globalRow = (res.data.list || []).find((item) => !item.tenant_id);
    expect(globalRow, 'global row should exist').to.be.an('object');
    const del = await apiClient.delete('/datapolicy/' + globalRow.id, {}, ADMIN);
    expect(del.code, JSON.stringify(del)).to.not.equal(200);
  });

  it('10. TENANT_ADMIN 调用被服务层 SYS_ADMIN 校验拒绝（casbin 放行不越权）', async function () {
    const res = await apiClient.post(
      '/datapolicy',
      {
        data_type: '1',
        tenant_id: tenantId,
        device_config_id: null,
        retention_days: 30,
        enabled: '1',
        remark: '契约测试行级策略_100_越权'
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('11. DELETE /datapolicy/:id 删除行级行后列表不再返回', async function () {
    const delTenant = await apiClient.delete('/datapolicy/' + tenantLevelPolicyId, {}, ADMIN);
    expect(delTenant.code, JSON.stringify(delTenant)).to.equal(200);
    const delProfile = await apiClient.delete('/datapolicy/' + profileLevelPolicyId, {}, ADMIN);
    expect(delProfile.code, JSON.stringify(delProfile)).to.equal(200);

    const res = await apiClient.get('/datapolicy', { page: 1, page_size: 50 }, ADMIN);
    const list = res.data.list || [];
    expect(list.find((item) => item.id === tenantLevelPolicyId), 'tenant-level row should be gone').to.equal(undefined);
    expect(list.find((item) => item.id === profileLevelPolicyId), 'profile-level row should be gone').to.equal(undefined);
    // 全局行不受影响。
    expect(list.filter((item) => !item.tenant_id).length).to.be.at.least(2);

    // 已删除，避免 after 钩子重复清理。
    createdPolicyIds.length = 0;
  });
});
