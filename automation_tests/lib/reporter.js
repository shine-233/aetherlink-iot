/**
 * 文件用途：用于支撑 automation_tests 的自动化运行报告写入模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const fs = require('fs');
const path = require('path');

class Reporter {
  constructor() {
    this.results = [];
    this.startTime = null;
    this.endTime = null;
    this.parallel = false;
  }

  setParallel(flag) {
    this.parallel = !!flag;
  }

  start() {
    this.startTime = new Date();
    console.log('\n' + '='.repeat(70));
    console.log('  AetherLink IoT automation tests started');
    console.log('  Mode: ' + (this.parallel ? 'parallel' : 'sequential'));
    console.log('  Start time: ' + this.startTime.toLocaleString('zh-CN'));
    console.log('='.repeat(70) + '\n');
  }

  end() {
    this.endTime = new Date();
    const duration = (this.endTime - this.startTime) / 1000;
    const passed = this.results.filter(r => r.passed).length;
    const failed = this.results.filter(r => !r.passed).length;
    const total = this.results.length;
    const passRate = total > 0 ? ((passed / total) * 100).toFixed(2) : '0.00';

    console.log('\n' + '='.repeat(70));
    console.log('  Test execution completed');
    console.log('  Mode: ' + (this.parallel ? 'parallel' : 'sequential'));
    console.log('  End time: ' + this.endTime.toLocaleString('zh-CN'));
    console.log('  Duration: ' + duration.toFixed(2) + ' seconds');
    console.log('  ' + '-'.repeat(50));

    const byType = this.groupByType();
    Object.keys(byType).forEach(type => {
      const t = byType[type];
      const tRate = t.total > 0 ? ((t.passed / t.total) * 100).toFixed(2) : '0.00';
      const typeLabel = type === 'e2e' ? 'E2E' : 'API';
      console.log('  [' + typeLabel + '] total: ' + t.total + ' passed: ' + t.passed + ' failed: ' + t.failed + ' pass rate: ' + tRate + '%');
    });

    console.log('  ' + '-'.repeat(50));
    console.log('  Total cases: ' + total);
    console.log('  Passed: ' + passed + ' Failed: ' + failed);
    console.log('  Pass rate: ' + passRate + '%');
    console.log('='.repeat(70));

    if (failed > 0) {
      console.log('\nFailed case details:');
      this.results.filter(r => !r.passed).forEach(r => {
        const typeTag = r.type === 'e2e' ? '[E2E]' : '[API]';
        console.log('  FAIL ' + typeTag + ' [' + r.module + '] ' + r.name);
        console.log('    Reason: ' + r.reason);
      });
    }
    console.log('');
    return { total, passed, failed, passRate, duration, parallel: this.parallel };
  }

  record(module, name, passed, reason = '', type = 'api', evidenceKind = type, summary = {}) {
    const businessClosureEvidence = this.getBusinessClosureEvidence(evidenceKind, summary);
    const outcome = summary.outcome || (passed ? 'passed' : 'failed');
    const skipped = Number(summary.skipped || 0);
    const blockedReasons = Array.isArray(summary.blockedReasons) ? summary.blockedReasons : [];
    this.results.push({
      module,
      name,
      passed,
      reason,
      type,
      evidenceKind,
      outcome,
      skipped,
      blockedReasons,
      businessClosureEvidence,
      timestamp: new Date()
    });
    const mark = passed ? 'PASS' : 'FAIL';
    const typeTag = type === 'e2e' ? '[E2E]' : '[API]';
    console.log('  ' + mark + ' ' + typeTag + ' [' + module + '] ' + name + (reason && !passed ? ' (' + reason + ')' : ''));
  }

  generateJsonReport(outputDir = './reports') {
    if (!fs.existsSync(outputDir)) {
      fs.mkdirSync(outputDir, { recursive: true });
    }
    const passed = this.results.filter(r => r.passed).length;
    const failed = this.results.filter(r => !r.passed).length;
    const total = this.results.length;
    const passRate = total > 0 ? ((passed / total) * 100).toFixed(2) : '0.00';
    const duration = this.startTime && this.endTime ? (this.endTime - this.startTime) / 1000 : 0;

    const byType = this.groupByType();
    const apiSummary = byType.api || { total: 0, passed: 0, failed: 0, passRate: 0 };
    const e2eSummary = byType.e2e || { total: 0, passed: 0, failed: 0, passRate: 0 };
    const byEvidenceKind = this.groupByEvidenceKind();
    const businessClosureEvidence = this.groupBusinessClosureEvidence();

    const report = {
      summary: {
        total,
        passed,
        failed,
        passRate: parseFloat(passRate),
        duration: parseFloat(duration.toFixed(2)),
        parallel: this.parallel,
        startTime: this.startTime ? this.startTime.toISOString() : null,
        endTime: this.endTime ? this.endTime.toISOString() : null
      },
      byType: {
        api: this.toReportGroup(apiSummary),
        e2e: this.toReportGroup(e2eSummary)
      },
      byEvidenceKind,
      businessClosureEvidence,
      evidenceContract: {
        businessClosureRequiresEvidenceKind: 'business',
        nonBusinessEvidenceKinds: ['boundary', 'catalog', 'contract', 'preflight', 'config', 'page-coverage-only'],
        note: 'Passing boundary/catalog/preflight/page checks does not prove business closure.'
      },
      modules: this.groupByModule(),
      details: this.results
    };

    const reportPath = path.join(outputDir, 'summary.json');
    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2), 'utf8');
    console.log('  JSON summary generated: ' + path.resolve(reportPath));

    this.generateHtmlReport(outputDir, report);

    return reportPath;
  }

  toReportGroup(group) {
    return {
      total: group.total,
      passed: group.passed,
      failed: group.failed,
      passRate: group.total > 0 ? parseFloat(((group.passed / group.total) * 100).toFixed(2)) : 0
    };
  }

  groupByModule() {
    const modules = {};
    this.results.forEach(r => {
      if (!modules[r.module]) {
        modules[r.module] = {
          total: 0,
          passed: 0,
          failed: 0,
          type: r.type,
          evidenceKind: r.evidenceKind || r.type,
          businessClosureEvidence: false
        };
      }
      modules[r.module].total++;
      if (r.passed) {
        modules[r.module].passed++;
      } else {
        modules[r.module].failed++;
      }
      modules[r.module].outcome = r.outcome || (r.passed ? 'passed' : 'failed');
      modules[r.module].skipped = Number(r.skipped || 0);
      modules[r.module].blockedReasons = Array.isArray(r.blockedReasons) ? r.blockedReasons : [];
      modules[r.module].businessClosureEvidence =
        modules[r.module].businessClosureEvidence || r.businessClosureEvidence === true;
    });
    return modules;
  }

  getBusinessClosureEvidence(evidenceKind, summary = {}) {
    if (evidenceKind !== 'business') {
      return false;
    }
    if (typeof summary.businessClosureEvidence === 'boolean') {
      return summary.businessClosureEvidence;
    }
    if (Array.isArray(summary.cases)) {
      return summary.cases.some(item => item && item.businessClosureEvidence === true);
    }
    if (Array.isArray(summary.oracleCases)) {
      return summary.oracleCases.some(item => item && item.businessClosureEvidence === true);
    }
    return evidenceKind === 'business' && summary.caseLevelBusinessClosureEvidence === true;
  }

  groupByType() {
    const groups = {};
    this.results.forEach(r => {
      const type = r.type || 'api';
      if (!groups[type]) {
        groups[type] = { total: 0, passed: 0, failed: 0 };
      }
      groups[type].total++;
      if (r.passed) {
        groups[type].passed++;
      } else {
        groups[type].failed++;
      }
    });
    return groups;
  }

  groupByEvidenceKind() {
    return this.groupByField('evidenceKind');
  }

  groupBusinessClosureEvidence() {
    const groups = this.results.reduce((acc, result) => {
      const key = result.businessClosureEvidence ? 'business' : 'nonBusiness';
      acc[key].total++;
      if (result.passed) {
        acc[key].passed++;
      } else {
        acc[key].failed++;
      }
      return acc;
    }, {
      business: { total: 0, passed: 0, failed: 0 },
      nonBusiness: { total: 0, passed: 0, failed: 0 }
    });

    Object.keys(groups).forEach(key => {
      const item = groups[key];
      item.passRate = item.total > 0 ? parseFloat(((item.passed / item.total) * 100).toFixed(2)) : 0;
    });

    return groups;
  }

  groupByField(fieldName) {
    const groups = {};
    this.results.forEach(r => {
      const key = r[fieldName] || 'unknown';
      if (!groups[key]) {
        groups[key] = { total: 0, passed: 0, failed: 0, passRate: 0 };
      }
      groups[key].total++;
      if (r.passed) {
        groups[key].passed++;
      } else {
        groups[key].failed++;
      }
    });
    Object.keys(groups).forEach(key => {
      const item = groups[key];
      item.passRate = item.total > 0 ? parseFloat(((item.passed / item.total) * 100).toFixed(2)) : 0;
    });
    return groups;
  }

  generateHtmlReport(outputDir, report) {
    const html = this.buildHtml(report);
    const htmlPath = path.join(outputDir, 'summary.html');
    fs.writeFileSync(htmlPath, html, 'utf8');
    console.log('  HTML summary generated: ' + path.resolve(htmlPath));
  }

  buildHtml(report) {
    const {
      summary,
      byType,
      byEvidenceKind = {},
      businessClosureEvidence = {},
      details
    } = report;
    const apiRows = details.filter(d => d.type !== 'e2e');
    const e2eRows = details.filter(d => d.type === 'e2e');

    const renderRows = rows => {
      if (rows.length === 0) {
        return '<tr><td colspan="6" style="text-align:center;color:#999;">No test records</td></tr>';
      }
      return rows.map(r => {
        const status = r.passed
          ? '<span class="pass">PASS</span>'
          : '<span class="fail">FAIL</span>';
        const reasonParts = [];
        if (r.reason) {
          reasonParts.push(escapeHtml(r.reason));
        }
        if (r.outcome && r.outcome !== (r.passed ? 'passed' : 'failed')) {
          reasonParts.push('outcome: ' + escapeHtml(r.outcome));
        }
        if (Number(r.skipped || 0) > 0) {
          reasonParts.push('skipped: ' + Number(r.skipped));
        }
        if (Array.isArray(r.blockedReasons) && r.blockedReasons.length > 0) {
          reasonParts.push('blocked: ' + escapeHtml(
            r.blockedReasons.map(item => item.reason || item.category || String(item)).join(' | ')
          ));
        }
        const reason = reasonParts.length > 0
          ? '<div class="reason">' + reasonParts.join('<br>') + '</div>'
          : '';
        return '<tr>' +
          '<td>' + escapeHtml(r.module) + '</td>' +
          '<td>' + escapeHtml(r.name) + '</td>' +
          '<td>' + escapeHtml(r.evidenceKind || r.type || 'unknown') + '</td>' +
          '<td>' + (r.businessClosureEvidence ? 'business' : 'non-business') + '</td>' +
          '<td>' + status + '</td>' +
          '<td>' + reason + '</td>' +
          '</tr>';
      }).join('');
    };

    const renderCard = (label, item = { total: 0, passed: 0, failed: 0 }) => {
      const rate = item.total > 0 ? ((item.passed / item.total) * 100).toFixed(2) : '0.00';
      return '<div class="card">' +
        '<div class="card-title">' + escapeHtml(label) + '</div>' +
        '<div class="card-stat">Total: ' + item.total + '</div>' +
        '<div class="card-stat pass-text">Passed: ' + item.passed + '</div>' +
        '<div class="card-stat fail-text">Failed: ' + item.failed + '</div>' +
        '<div class="card-stat">Pass rate: ' + rate + '%</div>' +
        '</div>';
    };

    const renderEvidenceCards = (groups, labelPrefix) => Object.keys(groups).sort().map(kind => {
      return renderCard(labelPrefix + ': ' + kind, groups[kind]);
    }).join('');

    return '<!DOCTYPE html>' +
      '<html lang="zh-CN"><head><meta charset="UTF-8">' +
      '<title>AetherLink IoT automation test summary</title>' +
      '<style>' +
      'body{font-family:"Microsoft YaHei",Arial,sans-serif;margin:20px;background:#f5f7fa;color:#333;}' +
      'h1{color:#1a73e8;}h2{border-left:4px solid #1a73e8;padding-left:10px;margin-top:30px;}' +
      '.summary{display:flex;gap:15px;flex-wrap:wrap;margin:20px 0;}' +
      '.card{background:#fff;border-radius:8px;padding:15px 20px;box-shadow:0 2px 6px rgba(0,0,0,.08);min-width:180px;}' +
      '.card-title{font-size:16px;font-weight:bold;margin-bottom:8px;color:#1a73e8;}' +
      '.card-stat{font-size:14px;margin:4px 0;}' +
      '.pass-text{color:#34a853;}.fail-text{color:#ea4335;}' +
      'table{width:100%;border-collapse:collapse;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 6px rgba(0,0,0,.08);}' +
      'th,td{padding:10px 12px;border-bottom:1px solid #eee;text-align:left;font-size:14px;}' +
      'th{background:#1a73e8;color:#fff;}' +
      '.pass{color:#34a853;font-weight:bold;}.fail{color:#ea4335;font-weight:bold;}' +
      '.reason{color:#ea4335;font-size:12px;margin-top:4px;word-break:break-all;}' +
      '.evidence-note{background:#fff8e1;border-left:4px solid #fbbc04;padding:12px 16px;margin:12px 0 24px;color:#5f4b00;}' +
      '.badge{display:inline-block;padding:2px 8px;border-radius:10px;font-size:12px;margin-left:8px;}' +
      '.badge-parallel{background:#fff3cd;color:#856404;}.badge-sequential{background:#e2e3e5;color:#383d41;}' +
      '</style></head><body>' +
      '<h1>AetherLink IoT automation test summary' +
      (summary.parallel ? '<span class="badge badge-parallel">Parallel</span>' : '<span class="badge badge-sequential">Sequential</span>') +
      '</h1>' +
      '<div class="summary">' +
      renderCard('Overall', summary) +
      renderCard('API tests', byType.api) +
      renderCard('E2E tests', byType.e2e) +
      renderEvidenceCards(businessClosureEvidence, 'Business closure evidence') +
      renderEvidenceCards(byEvidenceKind, 'Evidence kind') +
      '</div>' +
      '<div class="evidence-note">Evidence note: boundary, catalog, contract, preflight, config, and page-coverage-only passes are not business closure. Fresh runtime release evidence still requires archived API automation and Playwright E2E.</div>' +
      '<h2>API test details</h2>' +
      '<table><thead><tr><th>Module</th><th>Case/File</th><th>Evidence kind</th><th>Business closure</th><th>Status</th><th>Reason</th></tr></thead>' +
      '<tbody>' + renderRows(apiRows) + '</tbody></table>' +
      '<h2>E2E test details</h2>' +
      '<table><thead><tr><th>Module</th><th>Case/File</th><th>Evidence kind</th><th>Business closure</th><th>Status</th><th>Reason</th></tr></thead>' +
      '<tbody>' + renderRows(e2eRows) + '</tbody></table>' +
      '</body></html>';
  }
}

function escapeHtml(str) {
  if (str == null) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

const reporter = new Reporter();
module.exports = reporter;
