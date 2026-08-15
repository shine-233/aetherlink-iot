/**
 * 文件用途：用于验证字典与通知 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const { expectBusinessError } = require('../lib/response_assertions');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectDictRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('id', 'dict_code', 'dict_value');
  expect(row.id).to.be.a('string').and.not.equal('');
  expect(row.dict_code).to.be.a('string').and.not.equal('');
  expect(row.dict_value).to.be.a('string').and.not.equal('');
}

describe('Dict and notification API module [09_dict_notification]', function () {
  this.timeout(30000);

  let dictCode = '';
  let dictId = null;
  let dictLanguageId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 09_dict_notification.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('super_admin');
    dictCode = 'codex_dict_seed_' + Date.now();

    const columnResp = await apiClient.post(
      '/dict/column',
      {
        dict_code: dictCode,
        dict_value: 'EMAIL',
        remark: 'seeded by codex'
      },
      'super_admin'
    );
    expectOk(columnResp);

    const findResp = await apiClient.get('/dict', { page: 1, page_size: 10, dict_code: dictCode }, 'super_admin');
    expectOk(findResp);
    expect(findResp.data.list).to.be.an('array').and.have.length.greaterThan(0);
    dictId = findResp.data.list[0].id;
    expect(dictId).to.be.a('string').and.not.equal('');

    const languageResp = await apiClient.post(
      '/dict/language',
      {
        dict_id: dictId,
        language_code: 'zh-CN',
        translation: '编码测试'
      },
      'super_admin'
    );
    expectOk(languageResp);

    const languagesResp = await apiClient.get('/dict/language/' + dictId, {}, 'super_admin');
    expectOk(languagesResp);
    const language = languagesResp.data.find(item => item.translation === '编码测试');
    expect(language).to.be.an('object');
    dictLanguageId = language.id;
    expect(dictLanguageId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (dictLanguageId) {
      try {
        await apiClient.delete('/dict/language/' + dictLanguageId, {}, 'super_admin');
      } catch (error) {
        // Cleanup failures should not hide the real assertion result.
      }
    }

    if (dictId) {
      try {
        await apiClient.delete('/dict/column/' + dictId, {}, 'super_admin');
      } catch (error) {
        // Cleanup failures should not hide the real assertion result.
      }
    }

    apiClient.clearAllTokens();
  });

  it('returns the current dictionary page shape', async function () {
    const resp = await apiClient.get('/dict', { page: 1, page_size: 10 }, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.total).to.be.a('number').and.at.least(1);
    expect(resp.data.list.length).to.be.at.least(1);
    expect(resp.data.list.length).to.be.at.most(resp.data.total);
    resp.data.list.forEach(expectDictRow);

    const seededRow = resp.data.list.find(item => item.id === dictId || item.dict_code === dictCode);
    expect(seededRow, 'seeded dictionary column must be visible in the dictionary page').to.be.an('object');
    expectDictRow(seededRow);
  });

  it('returns null for the current device_type enum lookup', async function () {
    const resp = await apiClient.get('/dict/enum', { dict_code: 'device_type' }, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.equal(null);
  });

  it('returns an empty array for an invalid dict language lookup', async function () {
    const resp = await apiClient.get('/dict/language/00000000-0000-0000-0000-000000000000', {}, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('array').and.have.lengthOf(0);
  });

  it('creates a dictionary column with the current required fields', async function () {
    dictCode = 'codex_dict_' + Date.now();

    const resp = await apiClient.post(
      '/dict/column',
      {
        dict_code: dictCode,
        dict_value: 'EMAIL',
        remark: 'created by codex'
      },
      'super_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.be.a('string').and.not.be.empty;
    expect(resp.data.dict_code).to.equal(dictCode);
  });

  it('finds the created dictionary row by dict_code', async function () {
    const resp = await apiClient.get('/dict', { page: 1, page_size: 10, dict_code: dictCode }, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array').and.have.length.greaterThan(0);
    const row = resp.data.list[0];
    expect(row.dict_code).to.equal(dictCode);
    expect(row.dict_value).to.equal('EMAIL');
    dictId = row.id;
  });

  it('creates a language translation for the created dictionary row', async function () {
    expect(dictId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.post(
      '/dict/language',
      {
        dict_id: dictId,
        language_code: 'zh-CN',
        translation: '编码测试'
      },
      'super_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.id).to.be.a('string').and.not.be.empty;
    expect(resp.data.language_code).to.equal('zh-CN');
  });

  it('returns the created language translation rows', async function () {
    expect(dictId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/dict/language/' + dictId, {}, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('array').and.have.length.greaterThan(0);
    expect(resp.data[0]).to.include({
      dict_id: dictId,
      language_code: 'zh-CN',
      translation: '编码测试'
    });
    dictLanguageId = resp.data[0].id;
  });

  it('deletes the created dictionary language row', async function () {
    expect(dictLanguageId).to.be.a('string').and.not.equal('');
    const deletedLanguageId = dictLanguageId;

    const resp = await apiClient.delete('/dict/language/' + dictLanguageId, {}, 'super_admin');

    expectOk(resp);

    const readback = await apiClient.get('/dict/language/' + dictId, {}, 'super_admin');
    expectOk(readback);
    expect(readback.data).to.be.an('array');
    expect(readback.data.some(item => item.id === deletedLanguageId)).to.equal(false);
    dictLanguageId = null;
  });

  it('deletes the created dictionary column', async function () {
    expect(dictId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.delete('/dict/column/' + dictId, {}, 'super_admin');

    expectOk(resp);

    const readback = await apiClient.get('/dict', { page: 1, page_size: 10, dict_code: dictCode }, 'super_admin');
    expectOk(readback);
    expect(readback.data).to.be.an('object');
    expect(readback.data.list).to.be.an('array');
    expect(readback.data.list.some(item => item.dict_code === dictCode)).to.equal(false);
    dictId = null;
  });

  it('returns the current notification group page shape', async function () {
    const resp = await apiClient.get('/notification_group/list', { page: 1, page_size: 10 }, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.total).to.be.a('number');
  });

  it('returns record-not-found for an invalid notification group id', async function () {
    const resp = await apiClient.get('/notification_group/00000000-0000-0000-0000-000000000000', {}, 'super_admin');

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(101001);
    expect(resp.message).to.be.a('string').and.not.equal('');
    expect(resp.data).to.be.an('object');
    expect(resp.data.sql_error).to.equal('record not found');
  });

  it('returns the current email notification config', async function () {
    const resp = await apiClient.get('/notification/services/config/EMAIL', {}, 'super_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.notice_type).to.equal('EMAIL');
    expect(resp.data.status).to.be.oneOf(['OPEN', 'CLOSE']);
  });

  it('persists a minimal email notification config save request', async function () {
    const resp = await apiClient.post(
      '/notification/services/config',
      {
        notice_type: 'EMAIL',
        status: 'CLOSE'
      },
      'super_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.notice_type).to.equal('EMAIL');
    expect(resp.data.status).to.equal('CLOSE');
  });

  it('rejects email test send when the Email field is missing in the current request shape', async function () {
    const resp = await apiClient.post(
      '/notification/services/config/e-mail/test',
      {
        recipient: 'test@example.com'
      },
      'super_admin'
    );

    expectBusinessError(resp, 100002, "Field 'Email' is required");
  });
});
