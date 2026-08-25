/**
 * 数据转发规则端点的真实 HTTP 覆盖（对标 26_uncovered_endpoints 的边界口径）。
 * 本模块当前定位为 boundary：验证鉴权门禁、参数校验与记录级错误契约，
 * 不声称第三方投递闭环（投递链路由 forward 引擎单测 + 后续专项覆盖）。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const INVALID_UUID = '00000000-0000-0000-0000-000000000001';

function expectOkPage(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
  expect(resp.data).to.be.an('object');
  expect(resp.data.total).to.be.a('number');
  expect(resp.data.list).to.be.an('array');
}

function expectRecordNotFound(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(101001);
}

describe('Forward rules API module', function () {
  it('returns the current forward rule page shape', async function () {
    const resp = await apiClient.get('/forward_rules', { page: 1, page_size: 20 });
    expectOkPage(resp);
  });

  it('rejects creation with an invalid source type (validation contract)', async function () {
    const resp = await apiClient.post('/forward_rules', {
      name: 'e2e_forward_bad_source',
      source_type: 'not-a-source',
      target_type: 'http',
      http_url: 'http://127.0.0.1:9/ingest'
    });
    // 绑定校验失败走统一参数错误码，而非 200。
    expect(resp.code).to.not.equal(200);
  });

  it('returns record-not-found when updating a fake forward rule id', async function () {
    const resp = await apiClient.put(`/forward_rules/${INVALID_UUID}`, {
      name: 'e2e_forward_missing',
      source_type: 'telemetry',
      target_type: 'http',
      http_url: 'http://127.0.0.1:9/ingest'
    });
    expectRecordNotFound(resp);
  });

  it('returns record-not-found when toggling a fake forward rule id', async function () {
    const resp = await apiClient.put(`/forward_rules/${INVALID_UUID}/toggle`, { enabled: true });
    expectRecordNotFound(resp);
  });

  it('returns record-not-found when deleting a fake forward rule id', async function () {
    const resp = await apiClient.delete(`/forward_rules/${INVALID_UUID}`);
    expectRecordNotFound(resp);
  });
});
