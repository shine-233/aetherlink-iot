/**
 * 文件用途：TP-22 大屏固定分辨率与轮播投屏活栈契约测试。
 *
 * 覆盖：
 *   1. 画布保存契约（新字段兼容，锚点 backend/internal/scadadoc）：
 *      displayMode/display_mode oneof（responsive|fixed1080）+ fixed1080 必须 1920×1080；
 *      旧画布（无该字段）零改动通过（向后兼容）；未知模式/坏尺寸/双键冲突 fail-closed 拒绝；
 *      显示模式跨发布/回滚保留（发布快照不可变，坏模式绝不能冻进历史）。
 *   2. 公开轮播端点 GET /api/v1/board/shared-carousel（无认证面）：
 *      空 tokens 参数错误；按 share token 批量解析已发布原生看板、顺序保持、
 *      missing_tokens 回填；公开面不认内部 board id（凭证语义）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'TP-22 Canvas Display Mode & Board Carousel [99_tp22_canvas_display_mode]';

// fixed1080 合法画布（camelCase canonical 键，与前端 serializeScadaCanvas 输出一致）。
const FIXED1080_CANVAS = JSON.stringify({
  schemaVersion: 1,
  displayMode: 'fixed1080',
  width: 1920,
  height: 1080,
  nodes: []
});
// 旧画布（无 display_mode 字段）：向后兼容契约的基线样例。
const LEGACY_CANVAS = JSON.stringify({
  schemaVersion: 1,
  width: 1280,
  height: 720,
  nodes: [{ id: 'n1', kind: 'shape', ref: 'rect', x: 0, y: 0, width: 10, height: 10 }]
});

describe(SUITE, function () {
  this.timeout(120000);

  let projectId = null;
  let documentId = null;
  let boardId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 99_tp22_canvas_display_mode.test.js');
    }
    await apiClient.login('tenant_admin');
  });

  after(async function () {
    if (documentId) {
      try {
        await apiClient.post('/scada/documents/' + documentId + '/archive', {}, 'tenant_admin');
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (projectId) {
      try {
        await apiClient.delete('/scada/projects/' + projectId, {}, 'tenant_admin');
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (boardId) {
      try {
        await apiClient.delete('/board/' + boardId, {}, 'tenant_admin');
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. POST /scada/projects 创建画布项目', async function () {
    const res = await apiClient.post('/scada/projects', { name: '契约测试大屏项目_99' }, 'tenant_admin');
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.id).to.be.a('string').and.not.equal('');
    projectId = res.data.id;
  });

  it('2. fixed1080 1920x1080 画布可创建且 displayMode 原样保留', async function () {
    const created = await apiClient.post(
      '/scada/projects/' + projectId + '/documents',
      { name: '契约测试大屏_99', json_data: FIXED1080_CANVAS },
      'tenant_admin'
    );
    expect(created.code, JSON.stringify(created)).to.equal(200);
    expect(created.data).to.be.an('object');
    documentId = created.data.id;
    expect(documentId).to.be.a('string').and.not.equal('');

    const fetched = await apiClient.get('/scada/documents/' + documentId, {}, 'tenant_admin');
    expect(fetched.code, JSON.stringify(fetched)).to.equal(200);
    // 新字段必须原样往返：保存后读回仍是 fixed1080（这就是"保存契约"本身）。
    expect(String(fetched.data.json_data)).to.contain('"displayMode":"fixed1080"');
    expect(String(fetched.data.json_data)).to.contain('1920');
  });

  it('3. fixed1080 尺寸不符保存被拒绝（1920x1080 尺寸契约）', async function () {
    const bad = JSON.stringify({ schemaVersion: 1, displayMode: 'fixed1080', width: 1280, height: 720, nodes: [] });
    const res = await apiClient.put(
      '/scada/documents/' + documentId,
      { expected_version: 1, json_data: bad },
      'tenant_admin'
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
    expect(JSON.stringify(res)).to.contain('1920');
  });

  it('4. 未知 display_mode 值 fail-closed 拒绝', async function () {
    const bad = JSON.stringify({ schemaVersion: 1, displayMode: 'fixed4k', width: 3840, height: 2160, nodes: [] });
    const res = await apiClient.put(
      '/scada/documents/' + documentId,
      { expected_version: 1, json_data: bad },
      'tenant_admin'
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
    expect(JSON.stringify(res)).to.contain('display_mode');
  });

  it('5. displayMode 与 display_mode 双键冲突拒绝', async function () {
    const bad = JSON.stringify({
      schemaVersion: 1,
      displayMode: 'responsive',
      display_mode: 'fixed1080',
      width: 1920,
      height: 1080,
      nodes: []
    });
    const res = await apiClient.put(
      '/scada/documents/' + documentId,
      { expected_version: 1, json_data: bad },
      'tenant_admin'
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
    expect(JSON.stringify(res)).to.contain('conflict');
  });

  it('6. 旧画布（无 display_mode 字段）向后兼容保存', async function () {
    const fetched = await apiClient.get('/scada/documents/' + documentId, {}, 'tenant_admin');
    expect(fetched.code, JSON.stringify(fetched)).to.equal(200);
    const version = fetched.data.current_version;

    const res = await apiClient.put(
      '/scada/documents/' + documentId,
      { expected_version: version, json_data: LEGACY_CANVAS },
      'tenant_admin'
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    // 兼容不是"悄悄补字段"：旧画布保存后仍不携带 display_mode。
    expect(String(res.data.json_data)).to.not.contain('displayMode');
    expect(String(res.data.json_data)).to.not.contain('display_mode');
  });

  it('7. 显示模式跨 publish/rollback 保留', async function () {
    // 先把画布改回 fixed1080，再发布 + 回滚验证快照往返。
    const fetched = await apiClient.get('/scada/documents/' + documentId, {}, 'tenant_admin');
    const version = fetched.data.current_version;
    const saved = await apiClient.put(
      '/scada/documents/' + documentId,
      { expected_version: version, json_data: FIXED1080_CANVAS },
      'tenant_admin'
    );
    expect(saved.code, JSON.stringify(saved)).to.equal(200);

    const published = await apiClient.post('/scada/documents/' + documentId + '/publish', {}, 'tenant_admin');
    expect(published.code, JSON.stringify(published)).to.equal(200);

    // 草稿改成 responsive 后回滚到 v1：快照（fixed1080）写回新草稿必须原样。
    const publishedVersion = published.data.current_version;
    await apiClient.put(
      '/scada/documents/' + documentId,
      {
        expected_version: publishedVersion,
        json_data: JSON.stringify({ schemaVersion: 1, displayMode: 'responsive', width: 1280, height: 720, nodes: [] })
      },
      'tenant_admin'
    );
    const rolled = await apiClient.post(
      '/scada/documents/' + documentId + '/rollback',
      { version: 1 },
      'tenant_admin'
    );
    expect(rolled.code, JSON.stringify(rolled)).to.equal(200);
    expect(String(rolled.data.json_data)).to.contain('"displayMode":"fixed1080"');
  });

  it('8. 公开轮播端点：空 tokens 返回参数错误（无认证访问可达）', async function () {
    const body = await apiClient.getRootNoAuth('/api/v1/board/shared-carousel', {});
    expect(body.httpStatus).to.equal(200);
    expect(body.data).to.be.an('object');
    expect(body.data.code, JSON.stringify(body.data)).to.not.equal(200);
  });

  it('9. 发布原生看板获得 share token（轮播凭证来源）', async function () {
    const created = await apiClient.post(
      '/board',
      {
        name: '契约测试轮播看板_99',
        config: '{}',
        home_flag: 'N',
        vis_type: 'native'
      },
      'tenant_admin'
    );
    expect(created.code, JSON.stringify(created)).to.equal(200);
    expect(created.data.id).to.be.a('string').and.not.equal('');
    boardId = created.data.id;

    const published = await apiClient.post('/board/' + boardId + '/publish', {}, 'tenant_admin');
    expect(published.code, JSON.stringify(published)).to.equal(200);
    expect(String(published.data.share_token || '')).to.not.equal('');
  });

  it('10. 无认证批量轮播：顺序保持 + missing_tokens 回填 + 未知 token 不可见', async function () {
    const shareToken = await (async () => {
      const detail = await apiClient.get('/board/' + boardId, {}, 'tenant_admin');
      expect(detail.code, JSON.stringify(detail)).to.equal(200);
      return detail.data.share_token;
    })();

    // 顺序：未知 token 在前、有效 token 在后 → items 必须仍按请求顺序给出有效项，
    // 未知 token 全量回填 missing_tokens；顺带验证公开面不认内部 board id。
    const body = await apiClient.getRootNoAuth('/api/v1/board/shared-carousel', {
      tokens: 'contract-test-unknown-token,' + shareToken
    });
    expect(body.httpStatus).to.equal(200);
    expect(body.data.code, JSON.stringify(body.data)).to.equal(200);
    expect(body.data.data).to.be.an('object');
    const items = body.data.data.items || [];
    const missing = body.data.data.missing_tokens || [];
    expect(items).to.have.lengthOf(1);
    expect(items[0]).to.be.an('object');
    expect(items[0].published).to.equal(true);
    expect(items[0].share_token).to.equal(shareToken);
    expect(missing).to.deep.equal(['contract-test-unknown-token']);
  });
});
