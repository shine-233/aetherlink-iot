/**
 * 文件用途：TB-10 实体级审计日志（127.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 操作后能查询到带 action/entity_type/entity_id/status_code 的审计行
 *      （create → update → delete 全生命周期）；
 *   2. 列表 API 的新筛选维度（action / entity_type / entity_id）正负用例；
 *   3. 严格租户隔离（租户 B 查不到租户 A 实体的审计行）；
 *   4. CSV 审计导出含新列（action/entity_type/entity_id/status_code），
 *      且仍不含 request/response 载荷列（与 42 号用例同一审计最小化口径）。
 *
 * 关键注意事项：
 *   - 审计行由 middleware/operations_log.go 在响应返回前同步落库，理论上查得到；
 *     但为容忍并发/刷写抖动，查询侧做短轮询（最多 ~6s），超时如实失败。
 *   - 导出 CSV 表头校验是 best-effort：文件写在后端进程 cwd 下 ./files/export/，
 *     测试进程读不到时明确 skip 记为"未验证"，不冒充通过（与 42 号用例一致）。
 */

require('../lib/runtime_config');
const fs = require('fs');
const path = require('path');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Operation entity-level audit [82_operation_entity_audit]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const DAY_MS = 24 * 60 * 60 * 1000;
const AUDIT_POLL_MAX_MS = 6000;
const AUDIT_POLL_STEP_MS = 500;

// TB-10（127.sql）导出新列；载荷列出现即为审计泄漏回归（与 42 号用例同口径）。
const EXPORT_HEADER_MUST_INCLUDE = ['action', 'entity_type', 'entity_id', 'status_code'];
const EXPORT_HEADER_FORBIDDEN = ['request_message', 'response_message'];

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// 短轮询等待审计行出现；返回命中的行或 null（调用方自行断言，避免把轮询时长伪装成通过）。
async function waitForAuditRow(filters) {
  const deadline = Date.now() + AUDIT_POLL_MAX_MS;
  let lastList = [];
  while (Date.now() <= deadline) {
    const res = await apiClient.get('/operation_logs', { page: 1, page_size: 50, ...filters }, TENANT_A);
    if (res.code === 200 && res.data && Array.isArray(res.data.list)) {
      lastList = res.data.list;
      if (lastList.length > 0) {
        return lastList;
      }
    }
    await sleep(AUDIT_POLL_STEP_MS);
  }
  return lastList;
}

function locateExportFile(filePath) {
  if (!filePath || typeof filePath !== 'string') return null;
  if (fs.existsSync(filePath)) return filePath;
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
  this.timeout(120000);

  const now = Date.now();
  const exportStart = new Date(now - DAY_MS).toISOString();
  const exportEnd = new Date(now + DAY_MS).toISOString();
  let createdCustomerId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 82_operation_entity_audit.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (createdCustomerId) {
      try {
        await apiClient.delete('/customer/' + createdCustomerId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. POST /customer 后能查到 action=create 的实体级审计行', async function () {
    const res = await apiClient.post(
      '/customer',
      { name: '实体审计契约客户_82', email: 'audit82@example.com' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.id).to.be.a('string').and.not.equal('');
    createdCustomerId = res.data.id;

    const rows = await waitForAuditRow({ action: 'create', entity_type: 'customer', entity_id: createdCustomerId });
    expect(rows, 'audit rows for the created customer must appear').to.have.length.of.at.least(1);
    const hit = rows.find((row) => row.entity_id === createdCustomerId && row.action === 'create');
    expect(hit, 'create audit row').to.be.an('object');
    expect(hit.entity_type).to.equal('customer');
    expect(hit.status_code).to.equal(200);
    expect(String(hit.path)).to.include('/api/v1/customer');
    expect(hit.tenant_id, 'audit row stays in creator tenant').to.be.a('string').and.not.equal('');
  });

  it('2. 更新客户后能查到 action=update 的审计行', async function () {
    const res = await apiClient.post(
      '/customer',
      { id: createdCustomerId, name: '实体审计契约客户_82_改' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);

    const rows = await waitForAuditRow({ action: 'update', entity_type: 'customer', entity_id: createdCustomerId });
    const hit = rows.find((row) => row.entity_id === createdCustomerId && row.action === 'update');
    expect(hit, 'update audit row').to.be.an('object');
    expect(hit.entity_type).to.equal('customer');
    expect(hit.status_code).to.equal(200);
  });

  it('3. action/delete 筛选在删除前查不到该实体（负向）', async function () {
    const res = await apiClient.get(
      '/operation_logs',
      { page: 1, page_size: 50, action: 'delete', entity_type: 'customer', entity_id: createdCustomerId },
      TENANT_A
    );
    expect(res.code).to.equal(200);
    const rows = (res.data && res.data.list) || [];
    expect(rows.find((row) => row.entity_id === createdCustomerId), 'no delete audit row before deletion').to.equal(undefined);
  });

  it('4. entity_type 筛选命中已知实体、未知识别返回空列表', async function () {
    const known = await apiClient.get(
      '/operation_logs',
      { page: 1, page_size: 50, entity_type: 'customer', entity_id: createdCustomerId },
      TENANT_A
    );
    expect(known.code).to.equal(200);
    const knownRows = (known.data && known.data.list) || [];
    expect(knownRows.length, 'entity filter must keep the audit rows').to.be.at.least(1);
    for (const row of knownRows) {
      expect(row.entity_type).to.equal('customer');
    }

    const unknown = await apiClient.get(
      '/operation_logs',
      { page: 1, page_size: 50, entity_type: 'no_such_entity_tb10' },
      TENANT_A
    );
    expect(unknown.code).to.equal(200);
    expect((unknown.data && unknown.data.list) || []).to.have.length.of(0);
  });

  it('5. 租户 B 查不到租户 A 实体的审计行（租户隔离 fail-closed）', async function () {
    const res = await apiClient.get(
      '/operation_logs',
      { page: 1, page_size: 50, entity_type: 'customer', entity_id: createdCustomerId },
      TENANT_B
    );
    expect(res.code).to.equal(200);
    const rows = (res.data && res.data.list) || [];
    expect(rows.find((row) => row.entity_id === createdCustomerId), 'tenant B must not see tenant A audit rows').to.equal(undefined);
  });

  it('6. DELETE /customer/:id 后能查到 action=delete 的审计行', async function () {
    const customerId = createdCustomerId;
    const res = await apiClient.delete('/customer/' + customerId, {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const check = await apiClient.get('/customer/' + customerId, {}, TENANT_A);
    expect(check.code).to.not.equal(200);
    createdCustomerId = null;

    const rows = await waitForAuditRow({ action: 'delete', entity_type: 'customer', entity_id: customerId });
    const hit = rows.find((row) => row.entity_id === customerId && row.action === 'delete');
    expect(hit, 'delete audit row').to.be.an('object');
    expect(hit.entity_type).to.equal('customer');
    expect(hit.status_code).to.equal(200);
    expect(String(hit.path)).to.include('/api/v1/customer/' + customerId);
  });

  it('7. 审计导出 CSV 含实体级新列且不含载荷列', async function () {
    const res = await apiClient.post(
      '/operation_logs/export',
      { start_time: exportStart, end_time: exportEnd, entity_type: 'customer' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.format).to.equal('csv');
    expect(res.data.rows, 'filtered export must contain the audit rows produced above').to.be.at.least(1);

    const resolved = locateExportFile(res.data.file_path);
    if (!resolved) {
      this.skip(`export file was written outside the test process reach (${res.data.file_path}); header not inspected`);
      return;
    }
    const headerLine = fs.readFileSync(resolved, 'utf8').split(/\r?\n/)[0];
    const columns = headerLine.split(',').map((value) => value.trim().replace(/^"|"$/g, ''));
    for (const required of EXPORT_HEADER_MUST_INCLUDE) {
      expect(columns, `csv header must contain ${required}`).to.include(required);
    }
    for (const forbidden of EXPORT_HEADER_FORBIDDEN) {
      expect(columns, `csv header must not contain ${forbidden}`).to.not.include(forbidden);
    }
  });
});
