/**
 * 文件用途：计算字段（遥测派生指标）API 边界证据测试。
 * 核心逻辑：覆盖列表分页形状、非法 expression 的 100002 参数错误，
 *   以及不存在 id 的更新/删除/详情 100404 与 message 契约。
 * 关键注意事项：
 *   - 本套件只做边界/契约验证，不构造持久化业务状态，不证明派生遥测的业务闭环；
 *   - @file-boundary-evidence-only：证据分类为 boundary，不计入 business closure；
 *   - message 断言锚定后端契约文案 "calculated field not found"。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');

const CALCULATED_FIELDS_PATH = '/calculated_fields';

function calculatedFieldPath(id) {
  return CALCULATED_FIELDS_PATH + '/' + id;
}

function validCreatePayload(overrides = {}) {
  return Object.assign({
    name: 'boundary-field',
    device_template_id: '00000000-0000-0000-0000-000000000001',
    output_key: 'power_w',
    expression: '(voltage * current) / 1000'
  }, overrides);
}

describe('Calculated fields API boundary [28_calculated_fields]', function () {
  this.timeout(60000);

  before(async function () {
    await apiClient.login('tenant_admin');
  });

  after(function () {
    apiClient.clearAllTokens();
  });

  it('returns the paged list shape with code 200', async function () {
    const resp = await apiClient.get(CALCULATED_FIELDS_PATH, { page: 1, page_size: 10 }, 'tenant_admin');
    expectSuccess(resp);
    expect(resp.code).to.equal(200);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number');
    expect(Number(resp.data.total)).to.be.at.least(0);
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.list.length).to.be.at.most(Number(resp.data.total));
    for (const row of resp.data.list.slice(0, 3)) {
      expect(row).to.be.an('object');
      expect(row).to.have.property('id').that.is.a('string').and.not.equal('');
      expect(row).to.have.property('device_template_id').that.is.a('string');
      expect(row).to.have.property('output_key').that.is.a('string').and.not.equal('');
      expect(row).to.have.property('expression').that.is.a('string');
      expect(row).to.have.property('enabled');
      expect(row.enabled).to.be.a('boolean');
    }
  });

  it('rejects an invalid expression with 100002 and a parse hint', async function () {
    const resp = await apiClient.post(
      CALCULATED_FIELDS_PATH,
      validCreatePayload({ expression: 'voltage * * current' }),
      'tenant_admin'
    );
    expectBusinessError(resp, 100002);
    expect(String(resp.message)).to.include('expression');
    expect(String(resp.message)).to.include('voltage');

    const badKeyResp = await apiClient.post(
      CALCULATED_FIELDS_PATH,
      validCreatePayload({ output_key: '1bad-key' }),
      'tenant_admin'
    );
    expectBusinessError(badKeyResp, 100002);
    expect(String(badKeyResp.message)).to.include('output_key');
  });

  it('reports calculated field not found when updating a fake id', async function () {
    const fakeId = '00000000-0000-0000-0000-0000000000aa';
    const resp = await apiClient.put(calculatedFieldPath(fakeId), {
      name: 'ghost',
      device_template_id: '00000000-0000-0000-0000-000000000001',
      output_key: 'power_w',
      expression: 'voltage + 1'
    }, 'tenant_admin');
    expectBusinessError(resp, 100404, 'calculated field not found');
    expect(resp.code).to.equal(100404);
    expect(String(resp.message)).to.match(/calculated field not found/iu);
  });

  it('reports calculated field not found when deleting a fake id twice', async function () {
    const fakeId = '00000000-0000-0000-0000-0000000000bb';
    const firstDelete = await apiClient.delete(calculatedFieldPath(fakeId), {}, 'tenant_admin');
    expectBusinessError(firstDelete, 100404, 'calculated field not found');

    const detail = await apiClient.get(calculatedFieldPath(fakeId), {}, 'tenant_admin');
    expectBusinessError(detail, 100404, 'calculated field not found');

    const toggle = await apiClient.put(calculatedFieldPath(fakeId) + '/toggle', {}, 'tenant_admin');
    expectBusinessError(toggle, 100404, 'calculated field not found');
  });
});
