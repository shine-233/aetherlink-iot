/**
 * 文件用途：模板市场（导出/导入/分类目录）API 自动化测试 [36_template_market]。
 * 核心逻辑：创建 run 唯一的工业模板源 → 分类目录过滤断言（正断言，无条件跳过）→
 *   导出可移植描述符（无 id/tenant 字段）→ 改名导入（created=true）→ 重复导入
 *   （幂等 created=false 同 id）→ 非法 kind 拒绝（100002，走 expectBusinessError）。
 * 关键注意事项：源模板与导入副本均用 run 唯一后缀命名，重复运行不积累同名脏数据；
 *   endpoint catalog 的 export/:id 与 import 两条路由由本文件提供运行期覆盖。
 */

const { expect } = require('chai');
const { expectBusinessError } = require('../lib/response_assertions');
const apiClient = require('../lib/api_client');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function runSuffix() {
  return String(Date.now()) + '_' + Math.floor(Math.random() * 100000);
}

describe('Template market API module [36_template_market]', function () {
  this.timeout(30000);

  const suffix = runSuffix();
  const sourceName = 'automation_market_source_' + suffix;
  const importedName = 'automation_imported_template_' + suffix;
  const IMPORTED_VERSION = '1.0.0';

  let sourceId = null;
  let exportedPayload = null;
  let importedTemplateId = null;

  it('creates an industrial source template and filters the directory by type_key', async function () {
    const createResp = await apiClient.post(
      '/device/template',
      {
        name: sourceName,
        description: 'template market e2e source',
        version: '1.0.0',
        type_key: 'industrial',
      },
      'tenant_admin',
    );
    expectOk(createResp);
    sourceId = (createResp.data || {}).id;
    expect(sourceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get(
      '/device/template',
      { page: 1, page_size: 50, type_key: 'industrial' },
      'tenant_admin',
    );
    expectOk(resp);
    const rows = (resp.data && resp.data.list) || [];
    expect(rows.length).to.be.at.least(1);
    for (const row of rows) {
      expect(row.type_key).to.equal('industrial');
    }
    const found = rows.find(row => row.id === sourceId);
    expect(found, 'created source must appear in the industrial directory').to.be.an('object');
  });

  it('exports a portable template descriptor without id/tenant fields', async function () {
    const resp = await apiClient.get('/device/template/export/' + sourceId, {}, 'tenant_admin');
    expectOk(resp);
    const payload = resp.data;
    expect(payload.kind).to.equal('aetherlink-device-template');
    expect(payload.name).to.equal(sourceName);
    expect(payload).to.not.have.property('id');
    expect(payload).to.not.have.property('tenant_id');
    expect(payload.exported_at).to.be.a('string').and.not.equal('');
    exportedPayload = payload;
  });

  it('imports the payload as a new tenant template (created=true)', async function () {
    const payload = Object.assign({}, exportedPayload, {
      name: importedName,
      version: IMPORTED_VERSION,
    });
    const resp = await apiClient.post('/device/template/import', payload, 'tenant_admin');
    expectOk(resp);
    expect(resp.data.created).to.equal(true);
    expect(resp.data.template.name).to.equal(importedName);
    expect(resp.data.template.id).to.be.a('string').and.not.equal('');
    importedTemplateId = resp.data.template.id;
  });

  it('re-imports the same name+version idempotently (created=false, same id)', async function () {
    const payload = Object.assign({}, exportedPayload, {
      name: importedName,
      version: IMPORTED_VERSION,
    });
    const resp = await apiClient.post('/device/template/import', payload, 'tenant_admin');
    expectOk(resp);
    expect(resp.data.created).to.equal(false);
    expect(resp.data.template.id).to.equal(importedTemplateId);
  });

  it('rejects unsupported template kinds via expectBusinessError(100002)', async function () {
    const payload = Object.assign({}, exportedPayload, {
      name: 'automation_bad_kind_' + suffix,
      kind: 'other-vendor-template',
    });
    const resp = await apiClient.post('/device/template/import', payload, 'tenant_admin');
    expectBusinessError(resp, 100002);
  });

  it('keeps other industry directories isolated from the industrial entries', async function () {
    const resp = await apiClient.get(
      '/device/template',
      { page: 1, page_size: 50, type_key: 'power' },
      'tenant_admin',
    );
    expectOk(resp);
    const rows = (resp.data && resp.data.list) || [];
    for (const row of rows) {
      expect(row.type_key).to.equal('power');
      expect(row.name).to.not.equal(sourceName);
      expect(row.name).to.not.equal(importedName);
    }
  });
});
