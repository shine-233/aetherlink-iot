/**
 * 文件用途：P3 商业许可证状态端点（GET /license/status）的 API 契约测试。
 * 核心逻辑：SYS_ADMIN 可读状态视图；非 SYS_ADMIN 一律被拒；状态视图不得回传许可证材料本身。
 * 关键注意事项：
 *   - 本端点是**只读状态视图**，验证与执法在 service 层。测试不能假定许可证已配置——
 *     未配置公钥时的正确行为是 enabled=false 且带 reason，不是报错。
 *   - 材料泄漏检查是本用例的核心价值：响应里出现 material / license 明文即视为契约破坏。
 *     只断言"能读到字段"而不断言"没有泄漏字段"，等于给不了任何安全保证。
 *   - 非 SYS_ADMIN 的拒绝码来自 service 层显式判定（CodeNoPermission），不是 Casbin 中间件。
 * 重构建议：拿到签发工具后应补"签发→配置→状态 valid=true→配额执法"的全链用例，
 *          当前只覆盖验证侧与权限侧。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'License status view [39_license_status]';
const ADMIN = 'super_admin';
const NON_ADMIN = 'tenant_admin';

const CODE_NO_PERMISSION = 201001;

// 任何出现在响应里的这些键名都意味着许可证材料被回传了。
const FORBIDDEN_KEYS = ['material', 'license_material', 'signature', 'signed', 'token'];

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  expect(resp.data, 'data payload').to.satisfy(
    value => value !== null && typeof value === 'object' && !Array.isArray(value),
    'license status must be an object payload'
  );
  return resp.data;
}

function assertNoMaterialLeak(payload, path = 'data') {
  if (payload === null || typeof payload !== 'object') return;
  if (Array.isArray(payload)) {
    payload.forEach((item, index) => assertNoMaterialLeak(item, `${path}[${index}]`));
    return;
  }
  for (const key of Object.keys(payload)) {
    const normalized = key.toLowerCase();
    const leaked = FORBIDDEN_KEYS.some(forbidden => normalized.includes(forbidden));
    expect(leaked, `${path}.${key} looks like license material and must not be returned`).to.equal(false);
    assertNoMaterialLeak(payload[key], `${path}.${key}`);
  }
}

describe(SUITE, function () {
  this.timeout(30000);

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 39_license_status.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(ADMIN);
  });

  it('returns a status view for platform admins', async function () {
    const data = expectOk(await apiClient.get('/license/status', {}, ADMIN));
    expect(data.enabled, 'enabled').to.be.a('boolean');
    expect(data.required, 'required').to.be.a('boolean');
    expect(data.valid, 'valid').to.be.a('boolean');
  });

  it('explains itself when the license boundary is not enabled', async function () {
    const data = expectOk(await apiClient.get('/license/status', {}, ADMIN));
    if (data.enabled === false) {
      // 边界未启用是既有部署的默认行为，但必须给出原因，不能返回空对象。
      expect(data.reason, 'reason is required when enabled=false').to.be.a('string').and.not.equal('');
      expect(data.valid, 'a disabled boundary must never report valid=true').to.equal(false);
    }
  });

  it('never returns the license material itself', async function () {
    const data = expectOk(await apiClient.get('/license/status', {}, ADMIN));
    assertNoMaterialLeak(data);
  });

  it('reports a fingerprint only when the license is valid', async function () {
    const data = expectOk(await apiClient.get('/license/status', {}, ADMIN));
    if (data.valid) {
      expect(data.fingerprint, 'fingerprint must accompany a valid license').to.be.a('string').and.not.equal('');
      if (data.max_devices !== undefined) expect(data.max_devices).to.be.a('number');
      if (data.max_tenants !== undefined) expect(data.max_tenants).to.be.a('number');
    } else {
      expect(data.fingerprint === undefined || data.fingerprint === '',
        'fingerprint must not be present for an invalid license').to.equal(true);
    }
  });

  it('rejects non-platform-admin roles', async function () {
    if (!await apiClient.isAccountAvailable(NON_ADMIN)) {
      this.skip('tenant admin account is not configured');
      return;
    }
    const resp = await apiClient.get('/license/status', {}, NON_ADMIN);
    expect(resp, 'response envelope').to.be.an('object');
    // 实测口径：越权由 Casbin 中间件在统一响应封装**之前**短路，返回裸 HTTP 403
    // （body 为 {"error":"非法访问"}），因此这里拿到的是 403 而不是业务码 201001。
    // 这是全局中间件行为，不是本端点的特例——若日后统一为业务码，本断言需同步改。
    // 关键事实是"非平台管理员拿不到状态"，不纠结用哪种码表达拒绝。
    expect(resp.code, `non-admin must be denied, got ${resp.code}`).to.not.equal(200);
    expect([403, CODE_NO_PERMISSION], `unexpected denial code ${resp.code}`).to.include(resp.code);
  });

  it('rejects unauthenticated access', async function () {
    const resp = await apiClient.getNoAuth('/license/status');
    expect(resp, 'response envelope').to.be.an('object');
    expect(resp.code, 'unauthenticated access must not be 200').to.not.equal(200);
  });
});
