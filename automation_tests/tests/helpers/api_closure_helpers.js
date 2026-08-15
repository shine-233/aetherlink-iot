/**
 * 文件用途：用于支撑API 闭环与边界测试辅助模块。
 * 核心逻辑：封装 API 边界、Casbin 权限或动态账号夹具，减少测试文件中的重复准备步骤。
 * 关键注意事项：夹具可能修改本地测试状态；调用方仍需显式断言业务结果并处理清理。
 * 重构建议：新增夹具时保持幂等和可诊断，避免把核心断言藏进准备函数。
 */

const { expect } = require('chai');
const axios = require('axios');
const apiClient = require('../../lib/api_client');
const endpointCoverage = require('../../lib/endpoint_coverage');
const runtimeConfig = require('../../lib/runtime_config');

const ZERO_UUID = '00000000-0000-0000-0000-000000000000';
const ROOT_URL = process.env.API_TARGET || new URL(runtimeConfig.healthURL).origin;

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectCode(resp, code, text) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(code);
  expect(resp.message).to.be.a('string').and.not.equal('');
  if (text) {
    expect(resp.message).to.include(text);
  }
}

function expectRecordNotFound(resp) {
  expectCode(resp, 100000, 'record not found');
}

function expectSqlRecordNotFound(resp) {
  expectCode(resp, 101001);
  expect(resp.data).to.be.an('object');
  expect(resp.data.sql_error).to.equal('record not found');
}

function expectDbSqlError(resp, text) {
  expectCode(resp, 101001);
  expect(resp.data).to.be.an('object');
  expect(resp.data.sql_error).to.be.a('string').and.include(text);
}

function expectDbField(resp, field, text) {
  expectCode(resp, 101001);
  expect(resp.data).to.be.an('object');
  expect(resp.data[field]).to.be.a('string').and.include(text);
}

function expectValidationFieldOneOf(resp, fields) {
  expectCode(resp, 100002);
  expect(fields.some(field => resp.message.includes(field))).to.equal(
    true,
    'validation message should include one of: ' + fields.join(', ')
  );
}

function expectPagedPayload(resp) {
  expectOk(resp);
  expect(resp.data).to.be.an('object');
  expect(resp.data).to.have.property('list');
  expect(resp.data).to.have.property('total');
  expect(resp.data.total).to.be.a('number');
  if (resp.data.list !== null) {
    expect(resp.data.list).to.be.an('array');
  }
}

function expectArrayPayload(resp) {
  expectOk(resp);
  expect(resp.data).to.be.an('array');
}

function expectObjectPayload(resp, keys) {
  expect(keys, 'expectObjectPayload requires explicit business keys').to.be.an('array').and.not.be.empty;
  expectOk(resp);
  expect(resp.data).to.be.an('object');
  expect(Array.isArray(resp.data), 'object payload must not be an array').to.equal(false);
  for (const key of keys) {
    expect(resp.data).to.have.property(key);
  }
}

function expectNullableObjectPayload(data, keys = []) {
  if (data === null) {
    return;
  }
  expect(data).to.be.an('object');
  expect(Array.isArray(data), 'nullable object payload must not be an array').to.equal(false);
  for (const key of keys) {
    expect(data).to.have.property(key);
  }
}

function expectOkOrCode(resp, code, text) {
  if (resp.code === 200) {
    expect(resp.message).to.be.a('string').and.not.equal('');
    return;
  }
  expectCode(resp, code, text);
}

function expectArrayPayloadOrCode(resp, code, text) {
  if (resp.code === 200) {
    expectArrayPayload(resp);
    return;
  }
  expectCode(resp, code, text);
}

function expectObjectPayloadOrCode(resp, keys, code, text) {
  if (resp.code === 200) {
    expectObjectPayload(resp, keys);
    return;
  }
  expectCode(resp, code, text);
}

function expectPagedPayloadOrCode(resp, code, text) {
  if (resp.code === 200) {
    expectPagedPayload(resp);
    return;
  }
  expectCode(resp, code, text);
}

function expectDiagnosticsPayload(resp, deviceId) {
  expectObjectPayload(resp, ['device_id', 'stats', 'recent_failures']);
  expect(resp.data.device_id).to.equal(deviceId);
  expect(resp.data.recent_failures).to.be.an('array');
  expectNullableObjectPayload(resp.data.stats);
}

async function rawGet(pathname, options = {}) {
  const url = ROOT_URL + pathname;
  endpointCoverage.hit('GET', url);
  const resp = await axios.get(url, {
    timeout: options.timeout || 5000,
    responseType: options.responseType || 'text',
    headers: options.headers,
    validateStatus: () => true
  });
  if (resp.data && typeof resp.data.destroy === 'function') {
    resp.data.destroy();
  }
  return resp;
}

async function bootstrapApiCoverageContext() {
  const healthy = await apiClient.healthCheck();
  if (!healthy) {
    throw new Error('Backend service is not running locally for API coverage closure tests; unified verification requires a healthy API service');
  }

  await apiClient.login('tenant_admin');
  await apiClient.login('super_admin');

  let deviceId = null;
  const deviceResp = await apiClient.get('/device', { page: 1, page_size: 10 }, 'tenant_admin');
  const devices = deviceResp.data && Array.isArray(deviceResp.data.list) ? deviceResp.data.list : [];
  const firstDevice = devices[0] || null;
  deviceId = firstDevice ? (firstDevice.id || firstDevice.ID || null) : null;

  return { deviceId };
}

function cleanupApiCoverageContext() {
  apiClient.clearAllTokens();
}

module.exports = {
  ZERO_UUID,
  apiClient,
  rawGet,
  bootstrapApiCoverageContext,
  cleanupApiCoverageContext,
  expectOk,
  expectCode,
  expectRecordNotFound,
  expectSqlRecordNotFound,
  expectDbSqlError,
  expectDbField,
  expectValidationFieldOneOf,
  expectPagedPayload,
  expectArrayPayload,
  expectObjectPayload,
  expectNullableObjectPayload,
  expectOkOrCode,
  expectArrayPayloadOrCode,
  expectObjectPayloadOrCode,
  expectPagedPayloadOrCode,
  expectDiagnosticsPayload
};
