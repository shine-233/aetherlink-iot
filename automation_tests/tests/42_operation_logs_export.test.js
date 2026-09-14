/**
 * 文件用途：P3 审计导出端点（POST /operation_logs/export）的 API 契约测试。
 * 核心逻辑：时间窗必填且有界（≤1 年）、空窗口显式报错、导出结果为 CSV 信封，
 *          并尽可能就地校验 CSV 表头**不含** request/response 载荷列。
 * 关键注意事项：
 *   - 载荷列（request_message / response_message）不导出是审计最小化约定：
 *     导出文件会脱离数据库的访问控制成为第二份无保护副本。
 *     因此表头断言是本用例的核心，不是可选项。
 *   - 表头校验是 best-effort：导出文件写在后端进程 cwd 下的 ./files/export/，
 *     测试进程与后端同机时才能读到。读不到时明确记为"未验证"，不冒充通过。
 *   - 租户作用域刻意保守（仅本租户），跨租户导出不先于列表开放——
 *     所以不测"能不能导出别人的日志"，只测"导出的都是自己的"。
 * 重构建议：导出落盘改为对象存储或签名下载链接后，应改为校验下载内容与有效期，
 *          当前只能校验服务端相对路径。
 */

const fs = require('fs');
const path = require('path');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Operation log audit export [42_operation_logs_export]';
const ACCOUNT = 'tenant_admin';

const CODE_PARAM_ERROR = 100002;
const DAY_MS = 24 * 60 * 60 * 1000;

// 后端 dal.ListOperationLogsForExport 的 CSV 表头（service/audit_export.go 里的 header 常量）。
// 这两列一旦出现即为审计泄漏，属于必须拦住的回归。
const FORBIDDEN_HEADER_COLUMNS = ['request_message', 'response_message'];

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  expect(resp.data, 'export result').to.be.an('object');
  return resp.data;
}

function expectRejected(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected ${CODE_PARAM_ERROR} but got ${resp.code}: ${resp.message || ''}`).to.equal(CODE_PARAM_ERROR);
}

function isoTs(ms) {
  return new Date(ms).toISOString();
}

function locateExportFile(filePath) {
  if (!filePath || typeof filePath !== 'string') return null;
  if (fs.existsSync(filePath)) return filePath;
  // 后端进程 cwd 通常是仓库 backend/ 目录；测试进程在 automation_tests/。
  const candidates = [
    path.resolve(process.cwd(), filePath),
    path.resolve(process.cwd(), '..', 'backend', filePath.replace(/^\.\//, '')),
    path.resolve(process.cwd(), '..', filePath.replace(/^\.\//, ''))
  ];
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) return candidate;
  }
  return null;
}

describe(SUITE, function () {
  this.timeout(60000);

  const end = Date.now();
  const start = end - 30 * DAY_MS;
  let lastExportPath = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 42_operation_logs_export.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(ACCOUNT);
  });

  it('rejects an export without a time window', async function () {
    expectRejected(await apiClient.post('/operation_logs/export', {}, ACCOUNT));
    expectRejected(await apiClient.post('/operation_logs/export', { start_time: isoTs(start) }, ACCOUNT));
    expectRejected(await apiClient.post('/operation_logs/export', { end_time: isoTs(end) }, ACCOUNT));
  });

  it('rejects a window whose end is not after its start', async function () {
    expectRejected(await apiClient.post('/operation_logs/export', {
      start_time: isoTs(end),
      end_time: isoTs(start)
    }, ACCOUNT));
  });

  it('rejects a window longer than one year', async function () {
    expectRejected(await apiClient.post('/operation_logs/export', {
      start_time: isoTs(end - 400 * DAY_MS),
      end_time: isoTs(end)
    }, ACCOUNT));
  });

  it('rejects an empty window instead of producing an empty CSV', async function () {
    // 1970 年前后不可能有审计行；正确行为是显式报错，不是"导出 0 行成功"。
    expectRejected(await apiClient.post('/operation_logs/export', {
      start_time: isoTs(DAY_MS),
      end_time: isoTs(2 * DAY_MS)
    }, ACCOUNT));
  });

  it('exports the current tenant audit logs as a CSV envelope', async function () {
    const data = expectOk(await apiClient.post('/operation_logs/export', {
      start_time: isoTs(start),
      end_time: isoTs(end)
    }, ACCOUNT));
    expect(data.format, 'format').to.equal('csv');
    expect(data.rows, 'rows').to.be.a('number');
    expect(data.rows, 'an accepted export must contain at least one row').to.be.above(0);
    expect(data.file_path, 'file_path').to.be.a('string').and.not.equal('');
    expect(path.basename(data.file_path)).to.match(/^audit-log-[^/\\]+-\d+\.csv$/);
    lastExportPath = data.file_path;
  });

  it('never writes request/response payload columns into the CSV header', async function () {
    if (!lastExportPath) {
      this.skip('no export was produced in this run, so the CSV header could not be inspected');
      return;
    }
    const resolved = locateExportFile(lastExportPath);
    if (!resolved) {
      // 读不到文件不能等价于"没有泄漏"——如实记为未验证。
      this.skip(`export file was written outside the test process reach (${lastExportPath}); header not inspected`);
      return;
    }
    const headerLine = fs.readFileSync(resolved, 'utf8').split(/\r?\n/)[0];
    const columns = headerLine.split(',').map(value => value.trim().replace(/^"|"$/g, ''));
    expect(columns, 'csv header').to.include('id');
    expect(columns, 'csv header').to.include('tenant_id');
    for (const forbidden of FORBIDDEN_HEADER_COLUMNS) {
      expect(columns, `csv header must not contain ${forbidden}`).to.not.include(forbidden);
    }
  });
});
