/**
 * 文件用途：媒体库（media_files，TB-41，129.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 上传落登记全生命周期：POST /file/up（type=media）→ GET /media/files 列表 →
 *      GET /media/files/:id 详情 → DELETE /media/files/:id 删除（文件+登记行）；
 *   2. 列表分页与文件名搜索；
 *   3. 引用拒绝语义：看板 config 引用该文件后删除被拒（202004 + 引用方列表），
 *      解除引用后删除成功；
 *   4. 严格多租户隔离（租户 B 无法检索/查看/删除租户 A 的媒体登记）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Media Library [85_media_library]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const FIXTURE_FILE_NAME = 'aetherlink-media-85.png';
// 最小合法 1x1 PNG（通过后端 .png 内容签名校验：PNG magic + IEND）。
const PNG_FIXTURE = Buffer.from(
  '89504e470d0a1a0a0000000d494844520000000100000001080600000'
    + '01f15c4890000000d4944415478da6364f8cf500f00038601805a347d6b0000000049454e44ae426082',
  'hex'
);
const BOARD_NAME = '契约测试媒体引用看板_85';

describe(SUITE, function () {
  this.timeout(120000);

  let mediaId = null;
  let mediaPath = null;
  let boardId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 85_media_library.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (boardId) {
      try {
        await apiClient.delete('/board/' + boardId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (mediaId) {
      try {
        await apiClient.delete('/media/files/' + mediaId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  /** 上传夹具并在媒体列表中定位登记行，返回登记对象。 */
  async function uploadAndLocate() {
    const upload = await apiClient.upload(
      '/file/up',
      PNG_FIXTURE,
      { type: 'media' },
      TENANT_A,
      { filename: FIXTURE_FILE_NAME, contentType: 'image/png' }
    );
    expect(upload.code, JSON.stringify(upload)).to.equal(200);
    expect(upload.data).to.be.an('object');
    expect(upload.data.path).to.be.a('string')
      .and.match(/^\.\/files\/media\/\d{4}-\d{2}-\d{2}\/[0-9a-f]{32}\.png$/);

    const list = await apiClient.get(
      '/media/files', { page: 1, page_size: 50, search: FIXTURE_FILE_NAME }, TENANT_A
    );
    expect(list.code, JSON.stringify(list)).to.equal(200);
    const hit = (list.data.list || []).find((item) => item.file_path === upload.data.path);
    expect(hit, 'uploaded media should be registered immediately').to.be.an('object');
    return hit;
  }

  it('1. POST /file/up 上传媒体并落登记，列表立即可检索且字段完整', async function () {
    const hit = await uploadAndLocate();
    mediaId = hit.id;
    mediaPath = hit.file_path;
    expect(hit.file_name).to.equal(FIXTURE_FILE_NAME);
    expect(hit.mime).to.equal('image/png');
    expect(Number(hit.file_size)).to.equal(PNG_FIXTURE.length);
    expect(Number(hit.referenced_count)).to.equal(0);
  });

  it('2. GET /media/files 分页列表按文件名搜索命中登记', async function () {
    const res = await apiClient.get(
      '/media/files', { page: 1, page_size: 10, search: FIXTURE_FILE_NAME }, TENANT_A
    );
    expect(res.code).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.total).to.be.a('number').and.at.least(1);
    const hit = (res.data.list || []).find((item) => item.id === mediaId);
    expect(hit, 'created media should appear in list').to.be.an('object');
  });

  it('3. GET /media/files/:id 详情返回登记与空引用方', async function () {
    const res = await apiClient.get('/media/files/' + mediaId, {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.file).to.be.an('object');
    expect(res.data.file.id).to.equal(mediaId);
    expect(res.data.file.file_path).to.equal(mediaPath);
    expect(res.data.referencers).to.be.an('array').with.lengthOf(0);
  });

  it('4. 租户 B 无法检索/查看/删除租户 A 的媒体登记', async function () {
    const list = await apiClient.get(
      '/media/files', { page: 1, page_size: 50, search: FIXTURE_FILE_NAME }, TENANT_B
    );
    expect(list.code).to.equal(200);
    const hit = (list.data.list || []).find((item) => item.id === mediaId);
    expect(hit, 'tenant B must not see tenant A media').to.equal(undefined);

    const detail = await apiClient.get('/media/files/' + mediaId, {}, TENANT_B);
    expect(detail.code).to.not.equal(200);

    const del = await apiClient.delete('/media/files/' + mediaId, {}, TENANT_B);
    expect(del.code).to.not.equal(200);

    const check = await apiClient.get('/media/files/' + mediaId, {}, TENANT_A);
    expect(check.code).to.equal(200, 'cross-tenant delete must not remove tenant A media');
  });

  it('5. 看板引用该媒体后 DELETE 被拒绝（202004）并返回引用方', async function () {
    const board = await apiClient.post(
      '/board',
      {
        name: BOARD_NAME,
        home_flag: 'N',
        menu_flag: 'N',
        config: JSON.stringify({ widgets: [{ img: mediaPath }] })
      },
      TENANT_A
    );
    expect(board.code, JSON.stringify(board)).to.equal(200);
    expect(board.data.id).to.be.a('string').and.not.equal('');
    boardId = board.data.id;

    const res = await apiClient.delete('/media/files/' + mediaId, {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
    expect(res.code).to.equal(202004);
    expect(res.data).to.be.an('object');
    const referencers = res.data.referencers || [];
    const hit = referencers.find((ref) => ref.kind === 'board' && ref.id === boardId);
    expect(hit, 'referencer list should contain the board').to.be.an('object');

    // 详情的引用计数已实时刷新。
    const detail = await apiClient.get('/media/files/' + mediaId, {}, TENANT_A);
    expect(detail.code).to.equal(200);
    expect(Number(detail.data.file.referenced_count)).to.be.at.least(1);
  });

  it('6. 解除引用后 DELETE 成功（删文件+删登记），登记不可再查询', async function () {
    const delBoard = await apiClient.delete('/board/' + boardId, {}, TENANT_A);
    expect(delBoard.code, JSON.stringify(delBoard)).to.equal(200);
    boardId = null;

    const res = await apiClient.delete('/media/files/' + mediaId, {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.deleted).to.equal(true);

    const check = await apiClient.get('/media/files/' + mediaId, {}, TENANT_A);
    expect(check.code).to.not.equal(200);

    const list = await apiClient.get(
      '/media/files', { page: 1, page_size: 50, search: FIXTURE_FILE_NAME }, TENANT_A
    );
    const gone = (list.data.list || []).find((item) => item.id === mediaId);
    expect(gone, 'deleted media must disappear from list').to.equal(undefined);
    mediaId = null;
  });

  it('7. 重复删除同一媒体返回资源不存在', async function () {
    const res = await apiClient.delete('/media/files/' + mediaId, {}, TENANT_A);
    expect(res.code).to.not.equal(200);
  });

  it('8. 缺少文件的 POST /file/up 返回参数错误', async function () {
    const res = await apiClient.post('/file/up', {}, TENANT_A);
    expect(res.code).to.not.equal(200);
  });
});
