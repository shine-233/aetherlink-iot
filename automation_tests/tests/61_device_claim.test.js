/**
 * 文件用途：TB-12 设备认领与自动注册（Device Claiming）的 API 契约测试。
 *
 * 对标 ThingsBoard CE 的 Claiming Devices：
 *   1. 签发——持有租户为设备签发一次性认领令牌，明文 claim_key 只在签发响应出现一次；
 *   2. 赎回——接收方租户凭 device_number + claim_key 认领，设备租户随之转移；
 *   3. 安全边界——错 key / 重放 / 过期 / 撤销 / 认领自己租户 / 跨租户探测，全部拒绝；
 *   4. 列表接口永不出明文与哈希（SHA-256 hex 只落库）。
 *
 * 关键注意事项：
 *   - 存在性错误统一为 not-claimable 文案：认领方不能靠错误差异探测他租户设备清单；
 *   - 认领会把设备转移到 tenant_admin_b 名下，测试结束后设备归 B 租户，
 *     cleanup 由 seed 对象的 cleanup（删除设备）兜底。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'TB-12 device claiming [61_device_claim]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NOT_FOUND = 100404;
const CODE_OP_DENIED = 201002;

describe(SUITE, function () {
  this.timeout(120000);

  let seed = null;
  let deviceId = null;
  let deviceNumber = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 61_device_claim.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);
    seed = await seedData.createSimulationDevice(ACCOUNT);
    deviceId = seed.id;
    // 取设备详情拿 device_number（认领凭据由 device_number + claim_key 组成）。
    const detail = await apiClient.get('/device/detail/' + deviceId, {}, ACCOUNT);
    expect(detail.code, 'device detail').to.equal(200);
    deviceNumber = detail.data && (detail.data.device_number || (detail.data.device && detail.data.device.device_number));
    expect(deviceNumber, 'device_number').to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (seed && typeof seed.cleanup === 'function') {
      // 认领成功后设备已归 tenant_admin_b：按当前归属依次尝试，清理失败不掩盖主结果。
      try {
        await seed.cleanup();
      } catch (err) {
        const resp = await apiClient.delete('/device/' + deviceId, {}, OTHER_ACCOUNT);
        if (resp.code !== 200) throw err;
      }
    }
  });

  it('issues a one-time claim token and never echoes the plaintext again', async function () {
    const resp = await apiClient.post('/device/claim-tokens', {
      device_id: deviceId,
      ttl_seconds: 3600
    }, ACCOUNT);
    expect(resp.code, 'issue').to.equal(200);
    expect(resp.data.claim_key, 'claim key shape').to.be.a('string')
      .and.have.lengthOf(4 + 48);
    expect(resp.data.claim_key).to.match(/^ack_[0-9a-f]{48}$/);
    expect(resp.data.token_id, 'token id').to.be.a('string').and.not.equal('');

    // 管理视图：只有元数据与生效状态，明文与哈希都不出现。
    const list = await apiClient.get('/device/claim-tokens', { device_id: deviceId }, ACCOUNT);
    expect(list.code, 'list').to.equal(200);
    const text = JSON.stringify(list.data);
    expect(text, 'plaintext must never appear in list responses').to.not.include(resp.data.claim_key);
    const rows = Array.isArray(list.data) ? list.data : (list.data && list.data.list) || [];
    const row = rows.find(r => r.token_id === resp.data.token_id);
    expect(row, 'issued token visible to issuer').to.be.an('object');
    expect(row.effective, 'fresh token is effectively active').to.equal('active');
    expect(row, 'no hash leak').to.not.have.property('claim_key_hash');

    // 保存给后续用例使用（mocha 按声明顺序执行）。
    this.token = resp.data;
  });

  it('rejects claiming your own tenant device', async function () {
    const resp = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: deviceNumber,
      claim_key: this.token.claim_key
    }, ACCOUNT);
    expect(resp.code, 'own-tenant redeem').to.equal(CODE_OP_DENIED);
  });

  it('rejects a wrong key without leaking whether the device exists', async function () {
    const wrong = 'ack_' + '0'.repeat(48);
    const resp = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: deviceNumber,
      claim_key: wrong
    }, OTHER_ACCOUNT);
    expect(resp.code, 'wrong key').to.equal(CODE_NOT_FOUND);
    // 不存在的编号与错 key 必须**同码同文案**：任何差异都是"该设备存在且有活跃令牌"
    // 的存在性泄露，认领方能据此探测他租户设备清单。
    const ghost = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: 'no-such-device-' + Date.now(),
      claim_key: wrong
    }, OTHER_ACCOUNT);
    expect(ghost.code, 'ghost device same code').to.equal(CODE_NOT_FOUND);
    expect(ghost.message, 'uniform wording').to.equal(resp.message);
  });

  it('transfers the device to the claiming tenant and consumes the token', async function () {
    const resp = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: deviceNumber,
      claim_key: this.token.claim_key
    }, OTHER_ACCOUNT);
    expect(resp.code, 'redeem').to.equal(200);
    expect(resp.data.device_id, 'redeemed device').to.equal(deviceId);
    expect(resp.data.previous_tenant_id, 'previous tenant recorded').to.be.a('string').and.not.equal('');

    // 认领方立即可见；签发方（原租户）不再可见——跨租户边界在转移那一刻生效。
    const detailB = await apiClient.get('/device/detail/' + deviceId, {}, OTHER_ACCOUNT);
    expect(detailB.code, 'claimer sees the device').to.equal(200);
    const listA = await apiClient.get('/device/detail/' + deviceId, {}, ACCOUNT);
    expect([100404, 201001, 100002], 'original tenant loses visibility').to.include(listA.code);
  });

  it('rejects replaying the same claim key', async function () {
    const resp = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: deviceNumber,
      claim_key: this.token.claim_key
    }, OTHER_ACCOUNT);
    expect(resp.code, 'replay').to.equal(CODE_OP_DENIED);
  });

  it('allows re-issue after consumption and rejects a revoked token', async function () {
    const issue = await apiClient.post('/device/claim-tokens', {
      device_id: deviceId,
      ttl_seconds: 3600
    }, OTHER_ACCOUNT);
    expect(issue.code, 're-issue by new owner').to.equal(200);

    const revoke = await apiClient.delete('/device/claim-tokens/' + issue.data.token_id, {}, OTHER_ACCOUNT);
    expect(revoke.code, 'revoke').to.equal(200);

    const redeem = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: deviceNumber,
      claim_key: issue.data.claim_key
    }, ACCOUNT);
    expect(redeem.code, 'redeem after revoke (uniform rejection)').to.equal(CODE_NOT_FOUND);

    const revokeAgain = await apiClient.delete('/device/claim-tokens/' + issue.data.token_id, {}, OTHER_ACCOUNT);
    expect(revokeAgain.code, 'double revoke hits terminal-state guard').to.equal(CODE_OP_DENIED);
  });

  it('rejects an expired claim key', async function () {
    const issue = await apiClient.post('/device/claim-tokens', {
      device_id: deviceId,
      ttl_seconds: 1
    }, OTHER_ACCOUNT);
    expect(issue.code, 'short-ttl issue').to.equal(200);
    await new Promise(resolve => setTimeout(resolve, 2000));

    const redeem = await apiClient.post('/device/claim-tokens/redeem', {
      device_number: deviceNumber,
      claim_key: issue.data.claim_key
    }, ACCOUNT);
    expect(redeem.code, 'expired key (uniform rejection)').to.equal(CODE_NOT_FOUND);

    const list = await apiClient.get('/device/claim-tokens', { device_id: deviceId }, OTHER_ACCOUNT);
    const rows = Array.isArray(list.data) ? list.data : (list.data && list.data.list) || [];
    const row = rows.find(r => r.token_id === issue.data.token_id);
    expect(row && row.effective, 'expired rows report effective=expired').to.equal('expired');
  });

  it('validates parameters (empty body, unknown device)', async function () {
    expectBusinessCode(await apiClient.post('/device/claim-tokens', {}, ACCOUNT), CODE_PARAM_ERROR);
    expectBusinessCode(await apiClient.post('/device/claim-tokens/redeem', {}, ACCOUNT), CODE_PARAM_ERROR);
    expectBusinessCode(await apiClient.post('/device/claim-tokens', {
      device_id: '00000000-0000-0000-0000-00000000nope'
    }, ACCOUNT), CODE_NOT_FOUND);
  });
});

function expectBusinessCode(resp, code) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected ${code} but got ${resp.code}: ${resp.message || ''}`).to.equal(code);
}
