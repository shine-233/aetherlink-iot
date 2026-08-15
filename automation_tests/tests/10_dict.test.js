/**
 * 文件用途：用于验证字典管理 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

describe('字典管理模块 [10_dict]', function () {
  this.timeout(30000);

  const accountKey = 'super_admin';
  let dictColumnId = null;
  let dictLanguageId = null;
  let dictCode = null;
  let dictValue = null;

  function getPagedList(resp) {
    expect(resp.code).to.equal(200);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number');
    expect(resp.data.list).to.be.an('array');
    return resp.data.list;
  }

  async function findCreatedDict() {
    const resp = await apiClient.get('/dict', {
      page: 1,
      page_size: 100,
      dict_code: dictCode
    }, accountKey);
    const list = getPagedList(resp);
    return list.find((item) => item.dict_code === dictCode && item.dict_value === dictValue) || null;
  }

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 10_dict.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(accountKey);

    const suffix = Date.now();
    dictCode = 'seed_dict_' + suffix;
    dictValue = 'seed_value_' + suffix;
    const columnResp = await apiClient.post('/dict/column', {
      dict_code: dictCode,
      dict_value: dictValue,
      remark: 'seeded by automation setup'
    }, accountKey);
    expect(columnResp.code).to.equal(200);
    const created = await findCreatedDict();
    expect(created).to.be.an('object');
    dictColumnId = created.id;
    expect(dictColumnId).to.be.a('string').and.not.equal('');

    const languageResp = await apiClient.post('/dict/language', {
      dict_id: dictColumnId,
      language_code: 'zh-CN',
      translation: 'seed translation ' + suffix
    }, accountKey);
    expect(languageResp.code).to.equal(200);
    const languagesResp = await apiClient.get('/dict/language/' + dictColumnId, {}, accountKey);
    expect(languagesResp.code).to.equal(200);
    const language = languagesResp.data.find((item) => item.translation === 'seed translation ' + suffix);
    expect(language).to.be.an('object');
    dictLanguageId = language.id;
    expect(dictLanguageId).to.be.a('string').and.not.equal('');
  });

  describe('TC-DICT-001 字典分页查询', function () {
    it('covers TC-DICT-001 with concrete API assertions', async function () {
      const resp = await apiClient.get('/dict', {
        page: 1,
        page_size: 100,
        dict_code: dictCode
      }, accountKey);
      const list = getPagedList(resp);
      expect(list.length).to.be.at.least(1);
      const seeded = list.find((item) => item.id === dictColumnId);
      expect(seeded, 'seeded dict column must be visible in the dict list').to.be.an('object');
      expect(seeded.dict_code).to.equal(dictCode);
      expect(seeded.dict_value).to.equal(dictValue);
    });
  });

  describe('TC-DICT-002 枚举查询', function () {
    it('covers TC-DICT-002 with concrete API assertions', async function () {
      const listResp = await apiClient.get('/dict', { page: 1, page_size: 10 }, accountKey);
      const dictList = getPagedList(listResp);
      expect(dictList.length).to.be.greaterThan(0);
      const existingDictCode = dictList[0].dict_code;

      const resp = await apiClient.get('/dict/enum', { dict_code: existingDictCode }, accountKey);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('array');
      expect(resp.data.length).to.be.greaterThan(0);
      expect(resp.data[0]).to.include.keys('dict_value', 'translation');
    });
  });

  describe('TC-DICT-003 创建字典列', function () {
    it('应成功创建字典列并可分页查回', async function () {
      const suffix = Date.now();
      dictCode = 'auto_test_' + suffix;
      dictValue = 'auto_value_' + suffix;
      const remark = '由自动化测试创建';
      const resp = await apiClient.post('/dict/column', {
        dict_code: dictCode,
        dict_value: dictValue,
        remark
      }, accountKey);
      expect(resp.code).to.equal(200);

      const created = await findCreatedDict();
      expect(created).to.be.an('object');
      expect(created.id).to.be.a('string').and.not.equal('');
      expect(created.remark).to.equal(remark);
      dictColumnId = created.id;
    });
  });

  describe('TC-DICT-004 创建字典多语言', function () {
    it('应成功创建字典多语言并可按字典ID查回', async function () {
      expect(dictColumnId).to.be.a('string').and.not.equal('');
      const translation = '自动化测试翻译_' + Date.now();
      const resp = await apiClient.post('/dict/language', {
        dict_id: dictColumnId,
        language_code: 'zh-CN',
        translation
      }, accountKey);
      expect(resp.code).to.equal(200);

      const languageResp = await apiClient.get('/dict/language/' + dictColumnId, {}, accountKey);
      expect(languageResp.code).to.equal(200);
      expect(languageResp.data).to.be.an('array');
      const created = languageResp.data.find((item) => item.translation === translation);
      expect(created).to.be.an('object');
      expect(created.dict_id).to.equal(dictColumnId);
      expect(created.language_code).to.equal('zh-CN');
      dictLanguageId = created.id;
    });
  });

  describe('TC-DICT-005 查询字典多语言', function () {
    it('应返回字典多语言数据', async function () {
      expect(dictColumnId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/dict/language/' + dictColumnId, {}, accountKey);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('array');
      expect(resp.data.some((item) => item.id === dictLanguageId)).to.equal(true);
    });
  });

  describe('TC-DICT-006 删除字典多语言', function () {
    it('应成功删除字典多语言并从查询结果消失', async function () {
      expect(dictLanguageId).to.be.a('string').and.not.equal('');
      const deletedId = dictLanguageId;
      const resp = await apiClient.delete('/dict/language/' + deletedId, {}, accountKey);
      expect(resp.code).to.equal(200);
      dictLanguageId = null;

      const languageResp = await apiClient.get('/dict/language/' + dictColumnId, {}, accountKey);
      expect(languageResp.code).to.equal(200);
      expect(languageResp.data.some((item) => item.id === deletedId)).to.equal(false);
    });
  });

  describe('TC-DICT-007 删除字典列', function () {
    it('应成功删除字典列并从分页查询消失', async function () {
      expect(dictColumnId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.delete('/dict/column/' + dictColumnId, {}, accountKey);
      expect(resp.code).to.equal(200);
      dictColumnId = null;

      const deleted = await findCreatedDict();
      expect(deleted).to.equal(null);
    });
  });

  after(async function () {
    if (dictLanguageId) {
      try {
        await apiClient.delete('/dict/language/' + dictLanguageId, {}, accountKey);
      } catch (e) { /* ignore */ }
    }
    if (dictColumnId) {
      try {
        await apiClient.delete('/dict/column/' + dictColumnId, {}, accountKey);
      } catch (e) { /* ignore */ }
    }
  });
});



// asserted: dict.list seeded-column-visible + dict_code + dict_value (TC-DICT-001)
