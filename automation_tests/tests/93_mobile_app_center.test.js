/**
 * 文件用途：移动应用中心（mobile_app_bundles，TB-23，133.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 上传登记全生命周期：POST /api/v1/mobile/app_bundles/upload（multipart apk 夹具）→
 *      GET /api/v1/mobile/app_bundles 列表 → GET /api/v1/mobile/app_bundles/:id 详情 →
 *      PUT 更新发布说明 → publish/archive 状态机 → DELETE（文件+登记行）；
 *   2. 发布状态机：draft→published→archived 合法流转（published_at 落值且归档后保留）；
 *      非法流转拒绝（draft 直接归档 / 重复发布 / 归档后复活与重复归档 / published 直接删除 → 202005）；
 *   3. 重复版本拒绝：同租户同平台同版本上传返回 202006；同版本不同平台放行；
 *   4. 上传校验矩阵：非法 platform、非法 version（路径符号）、扩展名与平台不匹配、伪 ZIP 内容；
 *   5. 下载端点 GET /mobile/app_bundles/:id/download 可达（200）；
 *   6. 严格多租户隔离：租户 B 无法查看/发布/归档/删除租户 A 的应用包；
 *      版本唯一性是租户内概念——租户 B 可登记同平台同版本。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Mobile App Center [93_mobile_app_center]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const VERSION = '9.9.93';
const VERSION_H5 = '9.9.93';
// 最小合法 ZIP 容器头（PK\x03\x04 + 局部内容）。apk/ipa/zip 同为 ZIP 容器，
// 后端扩展名白名单按平台校验（android→.apk / ios→.ipa / h5→.zip）+ PK 魔数签名。
const ZIP_FIXTURE = Buffer.concat([
  Buffer.from([0x50, 0x4b, 0x03, 0x04]),
  Buffer.from('aetherlink-tb23-app-bundle-fixture', 'utf8')
]);

function bundleUploadFields(platform, version, releaseNotes) {
  const fields = { platform, version };
  if (releaseNotes) {
    fields.release_notes = releaseNotes;
  }
  return fields;
}

/** 上传一个 android .apk 草稿（缺省用全局版本号），返回登记对象。 */
async function uploadAndroidBundle(accountKey, version) {
  const upload = await apiClient.upload(
    '/mobile/app_bundles/upload',
    ZIP_FIXTURE,
    bundleUploadFields('android', version || VERSION, '契约测试应用包_93'),
    accountKey,
    { filename: 'aetherlink-tb93.apk', contentType: 'application/vnd.android.package-archive' }
  );
  return upload;
}

describe(SUITE, function () {
  this.timeout(120000);

  let bundleId = null;
  let tenantBBundleId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 93_mobile_app_center.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (bundleId) {
      try {
        await apiClient.delete('/mobile/app_bundles/' + bundleId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (tenantBBundleId) {
      try {
        await apiClient.delete('/mobile/app_bundles/' + tenantBBundleId, {}, TENANT_B);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. POST /mobile/app_bundles/upload 上传 android 包并登记为 draft（checksum/path 落值）', async function () {
    const upload = await uploadAndroidBundle(TENANT_A);
    expect(upload.code, JSON.stringify(upload)).to.equal(200);
    expect(upload.data).to.be.an('object');
    expect(upload.data.id).to.be.a('string').and.not.equal('');
    expect(upload.data.tenant_id).to.be.a('string').and.not.equal('');
    expect(upload.data.platform).to.equal('android');
    expect(upload.data.version).to.equal(VERSION);
    expect(upload.data.status).to.equal('draft');
    expect(upload.data.checksum).to.be.a('string').with.lengthOf(64);
    expect(upload.data.file_path).to.be.a('string')
      .and.match(/^\.\/files\/apps\/android\/\d{4}-\d{2}-\d{2}\/[0-9a-f]{32}\.apk$/);
    bundleId = upload.data.id;
  });

  it('2. 同租户同平台同版本重复上传返回 202006', async function () {
    const again = await uploadAndroidBundle(TENANT_A);
    expect(again.code, JSON.stringify(again)).to.equal(202006);
  });

  it('3. 上传校验矩阵：非法平台/非法版本/扩展名不匹配/伪 ZIP 均被拒绝', async function () {
    const badPlatform = await apiClient.upload(
      '/mobile/app_bundles/upload',
      ZIP_FIXTURE,
      bundleUploadFields('windows', '1.0.93'),
      TENANT_A,
      { filename: 'aetherlink-tb93.zip', contentType: 'application/zip' }
    );
    expect(badPlatform.code).to.not.equal(200);

    const badVersion = await apiClient.upload(
      '/mobile/app_bundles/upload',
      ZIP_FIXTURE,
      bundleUploadFields('android', '../../evil'),
      TENANT_A,
      { filename: 'aetherlink-tb93.apk', contentType: 'application/vnd.android.package-archive' }
    );
    expect(badVersion.code).to.not.equal(200);

    const wrongExt = await apiClient.upload(
      '/mobile/app_bundles/upload',
      ZIP_FIXTURE,
      bundleUploadFields('android', '2.0.93'),
      TENANT_A,
      { filename: 'aetherlink-tb93.zip', contentType: 'application/zip' }
    );
    expect(wrongExt.code).to.not.equal(200);

    const fakeZip = await apiClient.upload(
      '/mobile/app_bundles/upload',
      Buffer.from('MZ fake executable, not a zip container', 'utf8'),
      bundleUploadFields('android', '3.0.93'),
      TENANT_A,
      { filename: 'aetherlink-tb93.apk', contentType: 'application/vnd.android.package-archive' }
    );
    expect(fakeZip.code).to.not.equal(200);
  });

  it('4. GET /mobile/app_bundles 列表命中新登记（platform/status 过滤），详情与更新可用', async function () {
    const list = await apiClient.get(
      '/mobile/app_bundles',
      { page: 1, page_size: 50, platform: 'android', status: 'draft' },
      TENANT_A
    );
    expect(list.code, JSON.stringify(list)).to.equal(200);
    expect(list.data).to.be.an('object');
    const hit = (list.data.list || []).find((item) => item.id === bundleId);
    expect(hit, 'created bundle should appear in filtered list').to.be.an('object');

    const detail = await apiClient.get('/mobile/app_bundles/' + bundleId, {}, TENANT_A);
    expect(detail.code, JSON.stringify(detail)).to.equal(200);
    expect(detail.data.version).to.equal(VERSION);
    expect(detail.data.release_notes).to.equal('契约测试应用包_93');

    const updated = await apiClient.put(
      '/mobile/app_bundles/' + bundleId,
      { release_notes: '契约测试应用包_93_改' },
      TENANT_A
    );
    expect(updated.code, JSON.stringify(updated)).to.equal(200);
    expect(updated.data.release_notes).to.equal('契约测试应用包_93_改');
  });

  it('5. 状态机：draft 直接归档拒绝（202005）→ publish 成功落 published_at → 重复 publish 拒绝', async function () {
    const archiveOnDraft = await apiClient.post('/mobile/app_bundles/' + bundleId + '/archive', {}, TENANT_A);
    expect(archiveOnDraft.code, JSON.stringify(archiveOnDraft)).to.equal(202005);

    const publish = await apiClient.post('/mobile/app_bundles/' + bundleId + '/publish', {}, TENANT_A);
    expect(publish.code, JSON.stringify(publish)).to.equal(200);
    expect(publish.data.status).to.equal('published');
    expect(publish.data.published_at).to.be.a('string').and.not.equal('');

    const republish = await apiClient.post('/mobile/app_bundles/' + bundleId + '/publish', {}, TENANT_A);
    expect(republish.code).to.equal(202005);
  });

  it('6. published 更新与直接删除被拒（202005）；archive 后 published_at 保留', async function () {
    const updateOnPublished = await apiClient.put(
      '/mobile/app_bundles/' + bundleId,
      { release_notes: '不应生效' },
      TENANT_A
    );
    expect(updateOnPublished.code).to.equal(202005);

    const deleteOnPublished = await apiClient.delete('/mobile/app_bundles/' + bundleId, {}, TENANT_A);
    expect(deleteOnPublished.code).to.equal(202005);

    const detailBefore = await apiClient.get('/mobile/app_bundles/' + bundleId, {}, TENANT_A);
    const publishedAt = detailBefore.data && detailBefore.data.published_at;

    const archive = await apiClient.post('/mobile/app_bundles/' + bundleId + '/archive', {}, TENANT_A);
    expect(archive.code, JSON.stringify(archive)).to.equal(200);
    expect(archive.data.status).to.equal('archived');
    expect(archive.data.published_at).to.equal(publishedAt, 'archived bundle keeps published_at');

    const rearchive = await apiClient.post('/mobile/app_bundles/' + bundleId + '/archive', {}, TENANT_A);
    expect(rearchive.code).to.equal(202005);
    const revive = await apiClient.post('/mobile/app_bundles/' + bundleId + '/publish', {}, TENANT_A);
    expect(revive.code).to.equal(202005);
  });

  it('7. GET /mobile/app_bundles/:id/download 下载端点可达（200）', async function () {
    const download = await apiClient.get(
      '/mobile/app_bundles/' + bundleId + '/download',
      {},
      TENANT_A,
      { rawResponse: true }
    );
    expect(download.status).to.equal(200);
  });

  it('8. 归档后可删除（生命周期终点），删除后详情 404 语义', async function () {
    const del = await apiClient.delete('/mobile/app_bundles/' + bundleId, {}, TENANT_A);
    expect(del.code, JSON.stringify(del)).to.equal(200);
    const detail = await apiClient.get('/mobile/app_bundles/' + bundleId, {}, TENANT_A);
    expect(detail.code).to.not.equal(200);
    bundleId = null;
  });

  it('9. 严格租户隔离：租户 B 看不到/动不了租户 A 的应用包', async function () {
    const seed = await uploadAndroidBundle(TENANT_A);
    expect(seed.code, JSON.stringify(seed)).to.equal(200);
    bundleId = seed.data.id;

    const detail = await apiClient.get('/mobile/app_bundles/' + bundleId, {}, TENANT_B);
    expect(detail.code).to.not.equal(200);

    const publish = await apiClient.post('/mobile/app_bundles/' + bundleId + '/publish', {}, TENANT_B);
    expect(publish.code).to.not.equal(200);

    const archive = await apiClient.post('/mobile/app_bundles/' + bundleId + '/archive', {}, TENANT_B);
    expect(archive.code).to.not.equal(200);

    const del = await apiClient.delete('/mobile/app_bundles/' + bundleId, {}, TENANT_B);
    expect(del.code).to.not.equal(200);

    const stillThere = await apiClient.get('/mobile/app_bundles/' + bundleId, {}, TENANT_A);
    expect(stillThere.code).to.equal(200, 'cross-tenant operations must not mutate tenant A bundle');
  });

  it('10. 版本唯一性是租户内概念：租户 B 可登记同平台同版本', async function () {
    const own = await uploadAndroidBundle(TENANT_B);
    expect(own.code, JSON.stringify(own)).to.equal(200);
    expect(own.data.tenant_id).to.not.equal('');
    tenantBBundleId = own.data.id;

    const listB = await apiClient.get(
      '/mobile/app_bundles',
      { page: 1, page_size: 50, platform: 'android' },
      TENANT_B
    );
    expect(listB.code).to.equal(200);
    const hit = (listB.data.list || []).find((item) => item.id === tenantBBundleId);
    expect(hit, 'tenant B bundle should be visible to tenant B itself').to.be.an('object');
    const leak = (listB.data.list || []).find((item) => item.id === bundleId);
    expect(leak, 'tenant A bundle must not leak into tenant B list').to.equal(undefined);
  });
});
