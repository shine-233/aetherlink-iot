/**
 * 文件用途：报表格式契约测试（TB-49，format 放开为 csv/html/pdf 并以附件投递）活栈契约测试。
 *
 * 覆盖：
 *   1. POST /report/schedules 携带 format=html / format=pdf 创建成功且回读持久化格式一致；
 *   2. 缺省 format 回落 csv（与后端默认口径一致）；
 *   3. 非法格式（xlsx）被参数校验拒绝（errcode.CodeParamError）；
 *   4. PUT 更新 format（html→pdf）持久化新格式并推进 revision；
 *   5. 严格多租户隔离（租户 B 查询租户 A 的报表任务按 not-found 关闭）。
 *
 * 注意：本套件只做 CRUD 层格式契约，不触发 /run（不需要 SMTP/设备真实遥测，互不依赖 37 号套件）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Report schedule format contract [96_report_schedule_format]';
const ACCOUNT = 'tenant_admin';
const OTHER_TENANT = 'tenant_admin_b';
// errcode.CodeNotFound：租户越权读取按资源不存在关闭（与 37 号套件口径一致）。
const CODE_NOT_FOUND = 100404;
// errcode.CodeParamError：oneof 校验失败在绑定层拒绝。
const CODE_PARAM_ERROR = 100002;
const RECIPIENTS = 'report96@example.com';

function uniqueValue(prefix) {
  return `${prefix}_${Date.now()}_${Math.floor(Math.random() * 100000)}`;
}

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectProductError(resp, code) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(code);
}

function scheduleData(resp) {
  expectOk(resp);
  expect(resp.data).to.satisfy(
    value =>
      value !== null &&
      typeof value === 'object' &&
      !Array.isArray(value) &&
      Object.keys(value).length > 0,
    'report API response must carry a non-empty object payload'
  );
  return resp.data;
}

function buildPayload(format) {
  const payload = {
    name: uniqueValue('automation_report_format'),
    cron_expr: '0 4 * * *',
    timezone: 'UTC',
    recipients: RECIPIENTS,
    device_ids: ['00000000-0000-0000-0000-000000000096'],
    keys: ['temperature'],
    lookback_hours: 24
  };
  if (format !== undefined) {
    payload.format = format;
  }
  return payload;
}

describe(SUITE, function () {
  this.timeout(120000);

  const createdIds = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 96_report_schedule_format.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_TENANT);
  });

  after(async function () {
    for (const id of createdIds) {
      try {
        const current = await apiClient.get(`/report/schedules/${encodeURIComponent(id)}`, {}, ACCOUNT);
        if (current && current.code === 200 && current.data && current.data.revision) {
          await apiClient.delete(
            `/report/schedules/${encodeURIComponent(id)}?revision=${current.data.revision}`,
            {},
            ACCOUNT
          );
        }
      } catch (e) {
        /* 清理失败不影响套件结果，残留数据由名称前缀可辨识 */
      }
    }
  });

  it('1. POST 携带 format=html 创建并回读持久化格式', async function () {
    const created = scheduleData(await apiClient.post('/report/schedules', buildPayload('html'), ACCOUNT));
    expect(created.format).to.equal('html');
    expect(created.revision).to.equal(1);
    createdIds.push(created.id);

    const detail = scheduleData(await apiClient.get(`/report/schedules/${encodeURIComponent(created.id)}`, {}, ACCOUNT));
    expect(detail.id).to.equal(created.id);
    expect(detail.format).to.equal('html');
  });

  it('2. POST 携带 format=pdf 创建并回读持久化格式', async function () {
    const created = scheduleData(await apiClient.post('/report/schedules', buildPayload('pdf'), ACCOUNT));
    expect(created.format).to.equal('pdf');
    createdIds.push(created.id);

    const detail = scheduleData(await apiClient.get(`/report/schedules/${encodeURIComponent(created.id)}`, {}, ACCOUNT));
    expect(detail.format).to.equal('pdf');
  });

  it('3. POST 缺省 format 回落 csv', async function () {
    const created = scheduleData(await apiClient.post('/report/schedules', buildPayload(undefined), ACCOUNT));
    expect(created.format).to.equal('csv');
    createdIds.push(created.id);
  });

  it('4. POST 非法格式 xlsx 被参数校验拒绝', async function () {
    const resp = await apiClient.post('/report/schedules', buildPayload('xlsx'), ACCOUNT);
    expectProductError(resp, CODE_PARAM_ERROR);
  });

  it('5. PUT 更新 format html→pdf 持久化新格式并推进 revision', async function () {
    const created = scheduleData(await apiClient.post('/report/schedules', buildPayload('html'), ACCOUNT));
    createdIds.push(created.id);

    const updated = scheduleData(await apiClient.put(`/report/schedules/${encodeURIComponent(created.id)}`, {
      revision: created.revision,
      format: 'pdf'
    }, ACCOUNT));
    expect(updated.format).to.equal('pdf');
    expect(updated.revision).to.equal(created.revision + 1);

    const detail = scheduleData(await apiClient.get(`/report/schedules/${encodeURIComponent(created.id)}`, {}, ACCOUNT));
    expect(detail.format).to.equal('pdf');
  });

  it('6. 租户 B 无法查询租户 A 的报表任务', async function () {
    const created = scheduleData(await apiClient.post('/report/schedules', buildPayload('html'), ACCOUNT));
    createdIds.push(created.id);

    expectProductError(await apiClient.get(`/report/schedules/${encodeURIComponent(created.id)}`, {}, OTHER_TENANT), CODE_NOT_FOUND);
    // 归属租户仍可读取，确认拒绝来自隔离而非资源缺失。
    expectOk(await apiClient.get(`/report/schedules/${encodeURIComponent(created.id)}`, {}, ACCOUNT));
  });
});
