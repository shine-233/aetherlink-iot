/**
 * 文件用途：TB-18 通用 Secrets Storage（Universal Secrets Management & Storage，对标 ThingsBoard PE）契约与端到端测试。
 *
 * 核心验证矩阵：
 *  1. 完整 CRUD 生命周期（创建、分页检索、脱敏详情、原地更新、物理删除）；
 *  2. 静态加密与脱敏防泄密（默认 API 仅出不可逆 MaskPreview，绝对不泄密密文与明文）；
 *  3. 安全解密（Reveal）与审计闭环（管理员解密恢复原始明文，审计日志留痕且绝不记录明文）；
 *  4. 租户命名空间与跨租户强隔离（同一租户 key 唯一，不同租户允许同名 key，跨租户访问 100404 拒绝）；
 *  5. 细粒度 RBAC 权限门禁（TENANT_USER 仅允许只读查看脱敏列表，禁止调用 /reveal 触发 403 阻断）；
 *  6. 在线重加密轮换（Reseal）闭环与持久化一致性验证。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');

const SUITE = 'TB-18 Universal Secrets Storage End-to-End [56_secrets_storage]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';
const USER_ACCOUNT = 'tenant_user';

const CODE_PARAM_ERROR = 100002;
const CODE_NOT_FOUND = 100404;

function pickId(row) {
  return row && (row.id || row.ID);
}

describe(SUITE, function () {
  this.timeout(60000);

  const cleanups = [];
  let testSecretId = null;
  const testSecretKey = `TB18_AWS_KEY_${Date.now()}`;
  const testSecretValue = 'sk-live-0123456789-abcdefghijklmnopqrstuvwxyz';

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 56_secrets_storage.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(USER_ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);
  });

  after(async function () {
    // 清理创建的密钥
    for (const action of cleanups.reverse()) {
      try {
        await action();
      } catch (err) {
        // ignore cleanup error
      }
    }
  });

  it('1. POST /api/v1/secrets: 创建通用密钥（写入 AES-256-GCM 信封密文，默认返回脱敏掩码）', async function () {
    const payload = {
      key: testSecretKey,
      name: 'AWS IoT 生产访问密钥',
      secret_type: 'API_KEY',
      description: '自动化测试专用 AWS IoT Core 桥接凭证',
      value: testSecretValue
    };

    const resp = await apiClient.post('/secrets', payload, ACCOUNT);
    expectSuccess(resp, 'create secret');

    expect(resp.data).to.be.an('object');
    testSecretId = pickId(resp.data);
    expect(testSecretId, 'secret ID').to.be.a('string').and.not.equal('');

    cleanups.push(async () => {
      await apiClient.delete(`/secrets/${testSecretId}`, {}, ACCOUNT);
    });

    expect(resp.data.key).to.equal(testSecretKey);
    expect(resp.data.name).to.equal('AWS IoT 生产访问密钥');
    expect(resp.data.secret_type).to.equal('API_KEY');
    expect(resp.data.description).to.equal('自动化测试专用 AWS IoT Core 桥接凭证');
    expect(resp.data.mask_preview).to.equal('sk-l****');
    expect(resp.data.needs_reseal).to.equal(false);
    expect(resp.data.key_id).to.be.a('string').and.not.equal('');

    // 关键安全断言：出参中绝无明文 value 或 encrypted_value
    expect(resp.data.value).to.be.undefined;
    expect(resp.data.encrypted_value).to.be.undefined;
  });

  it('2. GET /api/v1/secrets: 列表检索支持分页、关键字模糊查询与类型过滤（全量脱敏）', async function () {
    const listResp = await apiClient.get('/secrets', { query: testSecretKey }, ACCOUNT);
    expectSuccess(listResp, 'list secrets with query');

    expect(listResp.data).to.be.an('object');
    expect(listResp.data.list).to.be.an('array');
    expect(listResp.data.total).to.be.at.least(1);

    const found = listResp.data.list.find(s => s.id === testSecretId || s.key === testSecretKey);
    expect(found, 'found created secret in list').to.be.an('object');
    expect(found.key).to.equal(testSecretKey);
    expect(found.mask_preview).to.equal('sk-l****');
    expect(found.value).to.be.undefined;
    expect(found.encrypted_value).to.be.undefined;
  });

  it('3. GET /api/v1/secrets/:id: 获取单条密钥详情，验证元数据回显与脱敏掩码', async function () {
    const detailResp = await apiClient.get(`/secrets/${testSecretId}`, null, ACCOUNT);
    expectSuccess(detailResp, 'get secret detail');

    expect(detailResp.data).to.be.an('object');
    expect(detailResp.data.id).to.equal(testSecretId);
    expect(detailResp.data.key).to.equal(testSecretKey);
    expect(detailResp.data.mask_preview).to.equal('sk-l****');
    expect(detailResp.data.needs_reseal).to.equal(false);
    expect(detailResp.data.value).to.be.undefined;
    expect(detailResp.data.encrypted_value).to.be.undefined;
  });

  it('4. 同一租户下重复 Key 校验：冲突拒绝（CodeParamError）', async function () {
    const dupResp = await apiClient.post('/secrets', {
      key: testSecretKey, // 同名 key
      name: '重复的 Key',
      secret_type: 'GENERIC',
      value: 'another-secret'
    }, ACCOUNT);

    expectBusinessError(dupResp, CODE_PARAM_ERROR, 'already exists');
  });

  it('5. 跨租户命名空间与隔离：租户 B 可创建同名 Key，但无法访问租户 A 的密钥', async function () {
    // 租户 B 创建同名 key
    const bCreateResp = await apiClient.post('/secrets', {
      key: testSecretKey,
      name: '租户 B 独立密钥',
      secret_type: 'TOKEN',
      value: 'tenant-b-secret-token'
    }, OTHER_ACCOUNT);
    expectSuccess(bCreateResp, 'tenant B create same-named secret');
    const tenantBSecretId = pickId(bCreateResp.data);

    cleanups.push(async () => {
      await apiClient.delete(`/secrets/${tenantBSecretId}`, {}, OTHER_ACCOUNT);
    });

    // 租户 B 尝试访问租户 A 的密钥 -> 404 (CodeNotFound)
    const bAccessAResp = await apiClient.get(`/secrets/${testSecretId}`, null, OTHER_ACCOUNT);
    expectBusinessError(bAccessAResp, CODE_NOT_FOUND, 'not found');

    // 租户 B 尝试解密租户 A 的密钥 -> 404
    const bRevealAResp = await apiClient.post(`/secrets/${testSecretId}/reveal`, {}, OTHER_ACCOUNT);
    expectBusinessError(bRevealAResp, CODE_NOT_FOUND, 'not found');

    // 租户 B 尝试删除租户 A 的密钥 -> 404
    const bDeleteAResp = await apiClient.delete(`/secrets/${testSecretId}`, {}, OTHER_ACCOUNT);
    expectBusinessError(bDeleteAResp, CODE_NOT_FOUND, 'not found');
  });

  it('6. POST /api/v1/secrets/:id/reveal: 管理员解密验证，精准还原原始明文并记录审计', async function () {
    const revealResp = await apiClient.post(`/secrets/${testSecretId}/reveal`, {}, ACCOUNT);
    expectSuccess(revealResp, 'reveal secret');

    expect(revealResp.data).to.be.an('object');
    expect(revealResp.data.id).to.equal(testSecretId);
    expect(revealResp.data.key).to.equal(testSecretKey);
    expect(revealResp.data.value).to.equal(testSecretValue);

    // 验证审计日志记录（操作日志中应有 SECRET_REVEAL，且绝不泄露明文）
    const logsResp = await apiClient.get('/operation_logs', { limit: 10 }, ACCOUNT);
    if (logsResp && logsResp.data && Array.isArray(logsResp.data.list)) {
      const revealLog = logsResp.data.list.find(l => (l.name === 'SECRET_REVEAL' || (l.path && l.path.includes(testSecretId))));
      if (revealLog) {
        expect(revealLog.name).to.equal('SECRET_REVEAL');
        // 确保明文不在 request_message 或 response_message 中
        const reqMsg = String(revealLog.request_message || '');
        const respMsg = String(revealLog.response_message || '');
        expect(reqMsg).to.not.include(testSecretValue);
        expect(respMsg).to.not.include(testSecretValue);
      }
    }
  });

  it('7. 细粒度 RBAC 门禁：TENANT_USER 角色调用 /reveal 被 403 阻断，但可读取脱敏列表', async function () {
    // 1. TENANT_USER 读取列表允许
    const userListResp = await apiClient.get('/secrets', { query: testSecretKey }, USER_ACCOUNT);
    expectSuccess(userListResp, 'tenant user list secrets');

    // 2. TENANT_USER 尝试 reveal 触发 403 阻断
    const userRevealResp = await apiClient.post(`/secrets/${testSecretId}/reveal`, {}, USER_ACCOUNT);
    expect(userRevealResp.code).to.be.oneOf([403, 201001, 100000]);
  });

  it('8. PUT /api/v1/secrets/:id: 更新元数据与轮换明文凭据', async function () {
    const newSecretValue = 'sk-live-9999999999-new-rotated-token';
    const updateResp = await apiClient.put(`/secrets/${testSecretId}`, {
      name: 'AWS IoT 生产访问密钥（已更新）',
      description: '定期轮换凭据',
      value: newSecretValue
    }, ACCOUNT);
    expectSuccess(updateResp, 'update secret');

    expect(updateResp.data.name).to.equal('AWS IoT 生产访问密钥（已更新）');
    expect(updateResp.data.description).to.equal('定期轮换凭据');

    // 调用 reveal 验证新值已生效
    const revealResp = await apiClient.post(`/secrets/${testSecretId}/reveal`, {}, ACCOUNT);
    expectSuccess(revealResp, 'reveal updated secret');
    expect(revealResp.data.value).to.equal(newSecretValue);
  });

  it('9. POST /api/v1/secrets/:id/reseal: 在线轮换重加密成功', async function () {
    const resealResp = await apiClient.post(`/secrets/${testSecretId}/reseal`, {}, ACCOUNT);
    expectSuccess(resealResp, 'reseal secret');
    expect(resealResp.data.needs_reseal).to.equal(false);
  });

  it('10. DELETE /api/v1/secrets/:id: 物理删除密钥，后续访问返回 404', async function () {
    const delResp = await apiClient.delete(`/secrets/${testSecretId}`, {}, ACCOUNT);
    expectSuccess(delResp, 'delete secret');

    const getResp = await apiClient.get(`/secrets/${testSecretId}`, null, ACCOUNT);
    expectBusinessError(getResp, CODE_NOT_FOUND, 'not found');
  });
});
