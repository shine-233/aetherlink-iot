/**
 * 文件用途：用于支撑 automation_tests 的API 响应断言工具模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const { expect } = require('chai');

function expectApiEnvelope(resp) {
  expect(resp).to.be.an('object');
  expect(resp).to.have.property('code');
  expect(resp).to.have.property('message');
}

function expectSuccess(resp, options = {}) {
  expectApiEnvelope(resp);
  expect(resp.code).to.equal(200);
  if (options.dataType) {
    expect(resp.data).to.be.a(options.dataType);
  }
  if (Array.isArray(options.keys)) {
    expect(resp.data).to.be.an('object');
    for (const key of options.keys) {
      expect(resp.data).to.have.property(key);
    }
  }
  return resp;
}

function expectCreatedId(resp, key) {
  expectSuccess(resp);
  expect(resp.data).to.be.an('object');
  expect(resp.data).to.have.property(key).that.is.a('string').and.not.equal('');
  return resp.data[key];
}

function expectBusinessError(resp, code, messageText) {
  expectApiEnvelope(resp);
  expect(resp.code).to.equal(code);
  expect(resp.message).to.be.a('string').and.not.equal('');
  if (messageText) {
    expectMessage(resp.message, messageText);
  }
  return resp;
}

function expectMessage(actual, expected) {
  const message = String(actual || '');
  const wanted = String(expected || '');
  if (message.includes(wanted)) return true;

  // The API intentionally localizes validation and governance messages. Keep
  // assertions semantic: a required-field contract must still name the same
  // field, while allowing the configured locale to choose the wording.
  const required = wanted.match(/^Field '([^']+)' is required$/i);
  if (required) {
    expect(message).to.include(required[1]);
    expect(message).to.match(/required|必填|不能为空|不得为空|必须/iu);
    return true;
  }
  // 同理处理 "Field 'X' failed validation (...)" 格式——后端默认中文返回
  // "字段 "X" 未通过校验（...）"，只要字段名匹配且语义一致即放行。
  const failedValidation = wanted.match(/^Field '([^']+)' failed validation/i);
  if (failedValidation) {
    expect(message).to.include(failedValidation[1]);
    expect(message).to.match(/failed validation|未通过校验/iu);
    return true;
  }
  if (/preview token/i.test(wanted)) {
    expect(message).to.match(/preview\s*token|预览.*令牌/iu);
    return true;
  }
  if (/preview expired/i.test(wanted)) {
    expect(message).to.match(/preview.*expired|预览.*过期/iu);
    return true;
  }
  expect(message).to.include(wanted);
  return true;
}

function expectPermissionDenied(resp) {
  expectApiEnvelope(resp);
  expect([401, 403, 201001]).to.include(resp.code);
  expect(resp.message).to.be.a('string').and.not.equal('');
  return resp;
}

function expectValidationError(resp, fieldName) {
  expectBusinessError(resp, 100002);
  if (fieldName) {
    expect(resp.message).to.include(fieldName);
  }
  return resp;
}

function expectBlockedOrSeeded(seed, label) {
  expect(seed).to.be.an('object');
  if (seed.blocked) {
    throw new Error((label || 'seed') + ' is blocked: ' + (seed.reason || 'missing deterministic seed prerequisite'));
  }
  expect(seed.id, label || 'seed id').to.be.a('string').and.not.equal('');
  return true;
}

function expectNullablePagedList(data, options = {}) {
  const totalKey = options.totalKey || 'total';
  const listKey = options.listKey || 'list';
  const rowCheck = options.rowCheck;

  expect(data).to.be.an('object');
  expect(data).to.have.property(totalKey);
  expect(data[totalKey]).to.be.a('number');
  expect(data[totalKey]).to.be.at.least(0);
  expect(data).to.have.property(listKey);

  if (data[listKey] === null) {
    expect(data[totalKey]).to.equal(0);
    return;
  }

  expect(data[listKey]).to.be.an('array');
  expect(data[listKey].length).to.be.at.most(data[totalKey]);

  if (data[listKey].length > 0 && typeof rowCheck === 'function') {
    rowCheck(data[listKey][0]);
  }
}

function expectPagedList(data, options = {}) {
  const totalKey = options.totalKey || 'total';
  const listKey = options.listKey || 'list';
  const rowCheck = options.rowCheck;

  expect(data).to.be.an('object');
  expect(data).to.have.property(totalKey);
  expect(data[totalKey]).to.be.a('number');
  expect(data[totalKey]).to.be.at.least(0);
  expect(data).to.have.property(listKey);
  expect(data[listKey]).to.be.an('array');
  expect(data[listKey].length).to.be.at.most(data[totalKey]);

  if (typeof rowCheck === 'function') {
    data[listKey].forEach(rowCheck);
  }
}

function expectPagedListContains(data, predicate, label = 'expected row') {
  expectNullablePagedList(data);
  expect(data.list, 'paged list').to.be.an('array');
  const row = data.list.find(predicate);
  expect(row, label).to.be.an('object');
  return row;
}

function expectNullableCountList(data, options = {}) {
  return expectNullablePagedList(data, {
    totalKey: 'count',
    listKey: 'list',
    rowCheck: options.rowCheck
  });
}

function expectNullableArray(data, options = {}) {
  const rowCheck = options.rowCheck;

  if (data === null) {
    return;
  }

  expect(data).to.be.an('array');

  if (data.length > 0 && typeof rowCheck === 'function') {
    rowCheck(data[0]);
  }
}

function expectArray(data, options = {}) {
  const rowCheck = options.rowCheck;

  expect(data).to.be.an('array');

  if (typeof rowCheck === 'function') {
    data.forEach(rowCheck);
  }
}

function expectNullableObject(data, options = {}) {
  const keys = options.keys || [];

  if (data === null) {
    return;
  }

  expect(data).to.be.an('object');
  expect(Array.isArray(data), 'nullable object payload must not be an array').to.equal(false);

  for (const key of keys) {
    expect(data).to.have.property(key);
  }
}

function expectMetricPoint(point) {
  expect(point).to.be.an('object');
  expect(point.timestamp).to.be.a('string').and.not.equal('');
  expect(point.cpu).to.be.a('number');
  expect(point.memory).to.be.a('number');
  expect(point.disk).to.be.a('number');
}

function expectCurrentSystemMetrics(data) {
  expect(data).to.be.an('object');
  expect(data.cpu_usage).to.be.a('number');
  expect(data.memory_usage).to.be.a('number');
  expect(data.disk_usage).to.be.a('number');
  expect(data.timestamp).to.be.a('string').and.not.equal('');
}

function expectOtaPackageRow(row) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys('id', 'version', 'target_version', 'package_url');
}

module.exports = {
  expectApiEnvelope,
  expectArray,
  expectBlockedOrSeeded,
  expectBusinessError,
  expectMessage,
  expectCreatedId,
  expectCurrentSystemMetrics,
  expectMetricPoint,
  expectNullableArray,
  expectNullableCountList,
  expectNullableObject,
  expectNullablePagedList,
  expectPagedList,
  expectOtaPackageRow,
  expectPagedListContains,
  expectPermissionDenied,
  expectSuccess,
  expectValidationError
};
