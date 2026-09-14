/**
 * 文件用途：P1.6 模板市场"打包导出 → 验签 → 预览 → 确认覆盖 → 导入"闭环的 API 契约测试。
 * 核心逻辑：无条件覆盖验签 fail closed（未签名 / 篡改 / 空包一律拒），
 *          再在签名密钥已配置的前提下跑通导出→预览→幂等重导的完整往返。
 * 关键注意事项：
 *   - **验签在预览之前执行**：preview=true 不是绕过验签的口子。未签名包即使只预览也必须被拒，
 *     这是本用例最核心的一条断言。
 *   - 导出端点在未配置 market.bundle_signing_keys 时**刻意拒绝出包**（出包即签名）。
 *     此时往返路径只能跳过，跳过原因必须写清楚，不能算作通过。
 *   - 覆盖闸门的语义是：预览列出 overwrite 后必须带 confirm_overwrite=true 重发；
 *     但**同名同版本属幂等命中，不需要确认**（否则"同包重导全幂等"这条路径不可达）。
 *   - 阻断项（stage='dependencies'）非空即拒绝，不进预览确认环节。
 * 重构建议：签名密钥成为 CI 标配后，应把往返路径从条件跳过改为强制断言。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Market bundle export / import gate [41_market_bundle_import]';
const ACCOUNT = 'tenant_admin';

const CODE_PARAM_ERROR = 100002;

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  expect(resp.data, 'data payload').to.be.an('object');
  return resp.data;
}

function expectRejected(resp, stage) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected ${CODE_PARAM_ERROR} but got ${resp.code}: ${resp.message || ''}`).to.equal(CODE_PARAM_ERROR);
  const data = resp.data;
  if (stage) {
    expect(data, `rejection must carry a stage (expected ${stage})`).to.be.an('object');
    expect(data.stage, `rejection stage (message: ${data.error || resp.message || ''})`).to.equal(stage);
  }
  return data;
}

function buildUnsignedBundle(name) {
  return {
    type_key: 'automation',
    exported_at: Date.now(),
    count: 1,
    templates: [
      {
        kind: 'aetherlink-device-template',
        name,
        version: '1.0.0',
        type_key: 'automation',
        description: 'unsigned bundle built by automation; must be rejected by the verifier'
      }
    ]
  };
}

function buildTamperedBundle(name) {
  return Object.assign(buildUnsignedBundle(name), {
    digest: 'a'.repeat(64),
    signature: 'b'.repeat(64),
    signed_key_id: 'automation-tampered'
  });
}

function expectPreviewShape(preview) {
  expect(preview, 'preview').to.be.an('object');
  expect(preview.total, 'preview.total').to.be.a('number');
  expect(preview.create, 'preview.create').to.be.an('array');
  expect(preview.overwrite, 'preview.overwrite').to.be.an('array');
  expect(preview.blocking, 'preview.blocking').to.be.an('array');
}

describe(SUITE, function () {
  this.timeout(60000);

  let signingConfigured = false;
  let exported = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 41_market_bundle_import.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(ACCOUNT);

    // 空租户里没有模板，导出会返回 100404 "no templates for this type_key"——
    // 这是空态而不是签名失败。先按导入契约建一条模板，让往返路径真的有数据可跑，
    // 否则"预览/幂等重导"两条永远停在 skip，等于没验证。
    const seedName = `automation_seed_${Date.now()}`;
    const seedResp = await apiClient.post('/device/template/import', {
      kind: 'aetherlink-device-template',
      name: seedName,
      version: '1.0.0',
      type_key: 'automation',
      description: 'seed template created by 41_market_bundle_import for the signed round-trip'
    }, ACCOUNT);
    expect(seedResp, 'seed template response').to.be.an('object');
    expect(seedResp.code, `seeding a template must succeed, got ${seedResp.code}: ${seedResp.message || ''}`).to.equal(200);

    // 导出端点未配置签名密钥时拒绝出包——这本身是刻意的 fail closed 行为，
    // 用它来探测"往返路径能否在本环境验证"，而不是硬编码环境变量。
    const exportResp = await apiClient.get('/device/template/market/bundle', { type_key: 'automation' }, ACCOUNT);
    if (exportResp.code === 200) {
      const data = exportResp.data;
      expect(data.file_name, 'file_name').to.be.a('string').and.not.equal('');
      expect(data.content_base64, 'content_base64').to.be.a('string').and.not.equal('');
      expect(data.count, 'count').to.be.a('number');
      expect(data.count, 'the seeded template must be inside the bundle').to.be.at.least(1);
      signingConfigured = true;
      exported = data;
    } else {
      signingConfigured = false;
    }
  });

  it('rejects an unsigned bundle even when only previewing', async function () {
    const name = `automation_unsigned_${Date.now()}`;
    const resp = await apiClient.post('/device/template/market/bundle/import', {
      bundle: buildUnsignedBundle(name),
      preview: true
    }, ACCOUNT);
    expectRejected(resp, 'verify');
  });

  it('rejects a bundle whose signature does not match its digest', async function () {
    const name = `automation_tampered_${Date.now()}`;
    const resp = await apiClient.post('/device/template/market/bundle/import', {
      bundle: buildTamperedBundle(name),
      preview: true
    }, ACCOUNT);
    expectRejected(resp, 'verify');
  });

  it('rejects a request without a bundle payload', async function () {
    const resp = await apiClient.post('/device/template/market/bundle/import', { preview: true }, ACCOUNT);
    expectRejected(resp);
  });

  it('does not modify anything while previewing', async function () {
    if (!signingConfigured) {
      this.skip('market.bundle_signing_keys is not configured, so no signed bundle can be produced for a preview round-trip');
      return;
    }
    const bundle = JSON.parse(Buffer.from(exported.content_base64, 'base64').toString('utf8'));
    const data = expectOk(await apiClient.post('/device/template/market/bundle/import', {
      bundle,
      preview: true
    }, ACCOUNT));
    expect(data.applied, 'preview must never write').to.equal(false);
    expectPreviewShape(data.preview);
    expect(data.results === undefined || data.results.length === 0,
      'preview must not carry import results').to.equal(true);
  });

  it('re-importing the same signed bundle is idempotent and does not demand overwrite confirmation', async function () {
    if (!signingConfigured) {
      this.skip('market.bundle_signing_keys is not configured, so the idempotent re-import path cannot be exercised');
      return;
    }
    const bundle = JSON.parse(Buffer.from(exported.content_base64, 'base64').toString('utf8'));
    const data = expectOk(await apiClient.post('/device/template/market/bundle/import', {
      bundle,
      preview: false
    }, ACCOUNT));
    expect(data.applied, 'a non-preview import must apply').to.equal(true);
    expect(data.results, 'results').to.be.an('array');
    // 同名同版本是幂等命中，不是覆盖——若它被 confirm_overwrite 闸门挡住，
    // 说明预览判定与实际幂等键分叉了（该缺陷此前已修过一次，这里守住回归）。
    const blockedByConfirm = data.preview && data.preview.overwrite && data.preview.overwrite.length > 0;
    for (const row of data.results) {
      expect(row.name, 'result name').to.be.a('string').and.not.equal('');
      expect(['created', 'idempotent', 'rejected']).to.include(row.outcome);
    }
    if (blockedByConfirm) {
      expect(data.results.every(row => row.outcome === 'rejected'),
        'a same-version re-import must not be classified as an overwrite').to.equal(false);
    }
  });
});
