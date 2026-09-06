/**
 * 文件用途：2FA（TOTP）生命周期业务自动化测试（ROADMAP C7）。
 * 核心逻辑：以动态创建的专用账号走完 2FA 全生命周期——setup 下发材料、
 *           非法码拒绝、合法码激活并发放恢复码、启用后密码登录降级为挑战、
 *           票据+验证码完成第二因子登录、重放拒绝、禁用后恢复密码直登。
 *           测试内以 Node crypto 复刻 RFC 6238（SHA1/30s/6 位，与后端
 *           internal/totp 同参数）生成验证码。
 * 关键注意事项：使用动态账号而非共享夹具，避免 2FA 状态影响其他套件登录。
 * 重构建议：若后端开放恢复码登录端点，应补充恢复码路径用例。
 */

const { expect } = require('chai');
const crypto = require('crypto');
const apiClient = require('../lib/api_client');
const {
  expectSuccess: expectOk,
  expectBusinessError
} = require('../lib/response_assertions');

const CODE_TOTP_INVALID = 200061;
const CODE_TOTP_ALREADY_ENABLED = 200062;

const B32_ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';

function base32NoPadDecode(input) {
  const clean = input.toUpperCase().replace(/=+$/, '');
  let bits = 0;
  let value = 0;
  const out = [];
  for (const ch of clean) {
    const idx = B32_ALPHABET.indexOf(ch);
    if (idx < 0) throw new Error('invalid base32 character: ' + ch);
    value = (value << 5) | idx;
    bits += 5;
    if (bits >= 8) {
      out.push((value >>> (bits - 8)) & 0xff);
      bits -= 8;
    }
  }
  return Buffer.from(out);
}

// RFC 6238：HMAC-SHA1 + 30s 步进 + 6 位 + 动态截断（与后端 internal/totp 同参数）。
function totpCode(secretB32, offsetSteps = 0) {
  const key = base32NoPadDecode(secretB32);
  const counter = Math.floor(Date.now() / 30000) + offsetSteps;
  const msg = Buffer.alloc(8);
  msg.writeBigUInt64BE(BigInt(counter));
  const hmac = crypto.createHmac('sha1', key).update(msg).digest();
  const offset = hmac[hmac.length - 1] & 0x0f;
  const bin = ((hmac[offset] & 0x7f) << 24) | (hmac[offset + 1] << 16) |
    (hmac[offset + 2] << 8) | hmac[offset + 3];
  return String(bin % 1000000).padStart(6, '0');
}

describe('TOTP second-factor lifecycle [34_totp_lifecycle]', function () {
  this.timeout(30000);

  let account = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 34_totp_lifecycle.test.js; 2FA coverage requires a healthy API service');
    }
    await apiClient.login('super_admin');

    // 专用动态账号：2FA 状态与共享夹具完全隔离。
    const suffix = String(Date.now()) + Math.floor(Math.random() * 1000);
    const email = `codex_totp_owner_${suffix}@test.com`;
    const password = 'Ae@' + crypto.randomBytes(5).toString('hex') + '1';
    const createResp = await apiClient.post('/user', {
      email,
      password,
      name: '自动化 2FA 生命周期账号',
      phone_number: '+86 137' + String(Date.now()).slice(-8)
    }, 'super_admin');
    expectOk(createResp);

    const listResp = await apiClient.get('/user', { page: 1, page_size: 10, email }, 'super_admin');
    expectOk(listResp);
    const created = (listResp.data.list || []).find(item => item.email === email);
    expect(created, 'created 2FA account must be visible').to.be.an('object');

    account = { email, password, userId: created.id };
  });

  after(async function () {
    try {
      if (account && account.userId) {
        await apiClient.delete('/user/' + account.userId, {}, 'super_admin');
      }
    } catch (error) {
      // cleanup failure must not mask the test verdict
    } finally {
      apiClient.clearAllTokens();
    }
  });

  function accountKeyForToken(token) {
    const key = 'totp_owner_session';
    apiClient.tokens[key] = token;
    return key;
  }

  it('issues pending 2FA setup material with an otpauth provisioning uri', async function () {
    const loginResp = await apiClient.postNoAuth('/login', {
      email: account.email,
      password: account.password
    });
    expectOk(loginResp);
    accountKeyForToken(loginResp.data.token);

    const setupResp = await apiClient.get('/user/totp/setup', {}, 'totp_owner_session');
    expectOk(setupResp);
    expect(setupResp.data).to.be.an('object');
    expect(setupResp.data.secret, 'secret must be base32 material').to.match(/^[A-Z2-7]+$/);
    expect(setupResp.data.secret.length).to.be.at.least(16);
    expect(setupResp.data.uri).to.be.a('string').and.include('otpauth://totp/');
    expect(setupResp.data.uri).to.include('secret=' + setupResp.data.secret);
    expect(setupResp.data.account).to.equal(account.email);
    expect(setupResp.data.issuer).to.equal('AetherLink');
    expect(setupResp.data.enabled).to.equal(false);
    account.secret = setupResp.data.secret;
  });

  it('rejects activation with an invalid code before enabling 2FA', async function () {
    const resp = await apiClient.post('/user/totp/activate', { code: '000000' }, 'totp_owner_session');
    expectBusinessError(resp, CODE_TOTP_INVALID);

    const statusResp = await apiClient.get('/user/totp/status', {}, 'totp_owner_session');
    expectOk(statusResp);
    expect(statusResp.data.enabled, 'failed activation must not enable 2FA').to.equal(false);
  });

  it('activates 2FA with a valid TOTP code, issues recovery codes, and reports enabled status', async function () {
    const activateResp = await apiClient.post('/user/totp/activate', {
      code: totpCode(account.secret, 0)
    }, 'totp_owner_session');
    expectOk(activateResp);
    expect(activateResp.data.codes).to.be.an('array').with.length.of.at.least(1);
    activateResp.data.codes.forEach((code) => {
      expect(code).to.be.a('string').and.not.equal('');
    });

    const statusResp = await apiClient.get('/user/totp/status', {}, 'totp_owner_session');
    expectOk(statusResp);
    expect(statusResp.data.enabled).to.equal(true);
  });

  it('rejects duplicate setup while 2FA is enabled', async function () {
    const resp = await apiClient.get('/user/totp/setup', {}, 'totp_owner_session');
    expectBusinessError(resp, CODE_TOTP_ALREADY_ENABLED);
  });

  it('challenges the second factor at login and completes it with a valid code', async function () {
    // 密码正确但 2FA 已启用：不再直接发 token，而是下发 step=totp 挑战票据。
    const challengeResp = await apiClient.postNoAuth('/login', {
      email: account.email,
      password: account.password
    });
    expectOk(challengeResp);
    expect(challengeResp.data).to.be.an('object');
    expect(challengeResp.data.step).to.equal('totp');
    expect(challengeResp.data.ticket).to.be.a('string').and.not.equal('');
    expect(challengeResp.data.token, 'no token must be issued before the second factor').to.not.exist;

    const consumedCode = totpCode(account.secret, 1);
    account.consumedLoginCode = consumedCode;
    const secondFactorResp = await apiClient.postNoAuth('/login/totp', {
      ticket: challengeResp.data.ticket,
      code: consumedCode
    });
    expectOk(secondFactorResp);
    expect(secondFactorResp.data.token).to.be.a('string').and.not.equal('');

    // 第二因子会话必须可用：携带其 token 访问受保护端点。
    accountKeyForToken(secondFactorResp.data.token);
    const profileResp = await apiClient.get('/user/detail', {}, 'totp_owner_session');
    expectOk(profileResp);
    expect(profileResp.data.email).to.equal(account.email);
  });

  it('rejects replaying a consumed TOTP code for a second second-factor login', async function () {
    // 重放 case 中已消费的同一码（同一时间步），防重放逻辑必须拒绝。
    const consumedCode = account.consumedLoginCode;
    expect(consumedCode, 'consumed code captured by the previous case').to.be.a('string').and.not.equal('');

    const challengeResp = await apiClient.postNoAuth('/login', {
      email: account.email,
      password: account.password
    });
    expectOk(challengeResp);
    expect(challengeResp.data.ticket).to.be.a('string').and.not.equal('');

    const replayResp = await apiClient.postNoAuth('/login/totp', {
      ticket: challengeResp.data.ticket,
      code: consumedCode
    });
    expectBusinessError(replayResp, CODE_TOTP_INVALID);
  });

  it('disables 2FA with a valid code and restores plain password login', async function () {
    const disableResp = await apiClient.post('/user/totp/disable', {
      code: totpCode(account.secret, 0)
    }, 'totp_owner_session');
    expectOk(disableResp);

    const statusResp = await apiClient.get('/user/totp/status', {}, 'totp_owner_session');
    expectOk(statusResp);
    expect(statusResp.data.enabled).to.equal(false);

    // 禁用后密码直登必须恢复，且直接拿到 token。
    const loginResp = await apiClient.postNoAuth('/login', {
      email: account.email,
      password: account.password
    });
    expectOk(loginResp);
    expect(loginResp.data.token).to.be.a('string').and.not.equal('');
    expect(loginResp.data.step, 'no challenge should remain after disable').to.not.exist;
  });
});
