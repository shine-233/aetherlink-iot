const { expect } = require('chai');

const runner = require('../run_tests');
const reporter = require('../lib/reporter');
const testMetadata = require('../lib/test_metadata');
const {
  PLAYWRIGHT_CASE_SCHEMA,
  extractPlaywrightJsonCases,
  reconcilePlaywrightCases,
} = require('../lib/runner/playwright-case-reconciliation');

const VISUALIZATION_FILE = 'e2e/11_visualization.spec.js';
const SUPER_ADMIN_FILE = 'e2e/23_native_board_super_admin.spec.js';

function reportCase(metadata, item, overrides = {}) {
  const attempt = {
    retry: 0,
    status: 'passed',
    duration: 12,
    startTime: '2026-09-08T00:00:00.000Z',
    annotations: [],
    ...overrides.attempt,
  };
  return {
    title: metadata.suiteTitle,
    file: metadata.file.replace(/^e2e\//, ''),
    line: 1,
    column: 1,
    specs: [{
      title: item.title,
      id: `playwright-${item.caseId}`,
      file: metadata.file.replace(/^e2e\//, ''),
      line: 2,
      column: 1,
      tests: [{
        projectId: 'contract-browser',
        projectName: 'contract-browser',
        expectedStatus: 'passed',
        status: 'expected',
        annotations: [],
        results: [attempt],
        ...overrides.test,
      }],
      ...overrides.spec,
    }],
  };
}

function reportFor(metadata, overridesByCase = new Map(), cases = metadata.cases) {
  return {
    config: { rootDir: 'C:/repo/automation_tests', forbidOnly: true },
    errors: [],
    suites: cases.map(item => reportCase(metadata, item, overridesByCase.get(item.title) || {})),
  };
}

function managedMetadata(file) {
  const metadata = testMetadata.getTestMetadata(file);
  return {
    ...metadata,
    cases: metadata.cases.filter(item => item.caseId),
  };
}

describe('Managed Playwright case reconciliation contract', function () {
  for (const file of [VISUALIZATION_FILE, SUPER_ADMIN_FILE]) {
    it(`extracts exact normalized file and full title for ${file}`, function () {
      const metadata = managedMetadata(file);
      const extracted = extractPlaywrightJsonCases(reportFor(metadata));
      expect(extracted).to.have.length(metadata.cases.length);
      expect(extracted.map(item => ({ file: item.file, title: item.fullTitle }))).to.deep.equal(
        metadata.cases.map(item => ({ file, title: item.fullTitle }))
      );
    });

    it(`promotes only passed reconciled v2 oracle cases for ${file}`, function () {
      const metadata = managedMetadata(file);
      const report = reportFor(metadata);
      report.suites[0].specs[0].tests[0].results.unshift({
        retry: 0,
        status: 'failed',
        duration: 8,
        startTime: '2026-09-07T23:59:00.000Z',
        annotations: [],
      });
      report.suites[0].specs[0].tests[0].results[1].retry = 1;

      const reconciled = reconcilePlaywrightCases(report, metadata);
      expect(reconciled.errors).to.deep.equal([]);
      expect(reconciled.oracleCases).to.have.length(metadata.cases.length);
      expect(reconciled.oracleCases[0]).to.include({
        schema: PLAYWRIGHT_CASE_SCHEMA,
        caseId: metadata.cases[0].caseId,
        file,
        fullTitle: metadata.cases[0].fullTitle,
        playwrightSpecId: `playwright-${metadata.cases[0].caseId}`,
        status: 'expected',
      });
      expect(reconciled.oracleCases[0].attempts.map(item => item.status)).to.deep.equal(['failed', 'passed']);
      expect(reconciled.oracleCases[0].operationDimensions).to.deep.equal(metadata.cases[0].operationDimensions);

      reporter.results = [];
      const moduleSummary = {
        passed: true,
        outcome: 'passed',
        skipped: 0,
        blockedReasons: [],
        reason: '',
      };
      runner.recordModuleSummary(
        { key: 'managed-case', file, evidenceLabel: 'business' },
        'e2e',
        moduleSummary,
        {},
        report
      );
      expect(moduleSummary.oracleCases).to.deep.equal(reconciled.oracleCases);
      expect(reporter.results[0].oracleCases).to.deep.equal(reconciled.oracleCases);
      reporter.results = [];
    });
  }

  it('accepts unmanaged cases from the same exact metadata file without promoting them', function () {
    const metadata = testMetadata.getTestMetadata(VISUALIZATION_FILE);
    const report = reportFor(metadata);
    const reconciled = reconcilePlaywrightCases(report, metadata);
    const managedCases = metadata.cases.filter(item => item.caseId && item.fullTitle);

    expect(reconciled.errors).to.deep.equal([]);
    expect(reconciled.extractedCases).to.have.length(metadata.cases.length);
    expect(reconciled.oracleCases.map(item => item.caseId)).to.deep.equal(
      managedCases.map(item => item.caseId)
    );
  });

  it('fails closed for duplicate, stale, missing, wrong-file, focused, and skipped identities', function () {
    const metadata = managedMetadata(SUPER_ADMIN_FILE);
    const scenarios = [];

    const duplicate = reportFor(metadata);
    duplicate.suites.push(JSON.parse(JSON.stringify(duplicate.suites[0])));
    scenarios.push([duplicate, 'duplicate Playwright identity']);

    const stale = reportFor(metadata);
    stale.suites[0].specs[0].title += ' stale';
    scenarios.push([stale, 'missing or stale Playwright identity']);

    const missing = reportFor(metadata);
    missing.suites = [];
    scenarios.push([missing, 'missing or stale Playwright identity']);

    const wrongFile = reportFor(metadata);
    wrongFile.suites[0].file = '11_visualization.spec.js';
    wrongFile.suites[0].specs[0].file = '11_visualization.spec.js';
    scenarios.push([wrongFile, 'wrong-file Playwright identity']);

    const focused = reportFor(metadata);
    focused.config.forbidOnly = false;
    scenarios.push([focused, 'focused Playwright selection']);

    const skipped = reportFor(metadata);
    skipped.suites[0].specs[0].tests[0].expectedStatus = 'skipped';
    skipped.suites[0].specs[0].tests[0].status = 'skipped';
    skipped.suites[0].specs[0].tests[0].annotations = [{ type: 'skip' }];
    skipped.suites[0].specs[0].tests[0].results[0].status = 'skipped';
    scenarios.push([skipped, 'skipped or expected-failure Playwright identity']);

    for (const [report, expectedError] of scenarios) {
      const reconciled = reconcilePlaywrightCases(report, metadata);
      expect(reconciled.errors.some(error => error.includes(expectedError)), expectedError).to.equal(true);
      expect(reconciled.oracleCases, expectedError).to.deep.equal([]);
    }
  });

  it('makes managed reconciliation failures fail the module summary', function () {
    const metadata = managedMetadata(SUPER_ADMIN_FILE);
    const summary = {
      passed: true,
      outcome: 'passed',
      skipped: 0,
      blockedReasons: [],
      reason: '',
    };
    runner.recordModuleSummary(
      { key: 'native-board-super-admin', file: metadata.file, evidenceLabel: 'business' },
      'e2e',
      summary,
      { reportJson: 'missing-managed-playwright-report.json' }
    );
    expect(summary.passed).to.equal(false);
    expect(summary.oracleCases).to.equal(undefined);
    expect(summary.caseReconciliationErrors).to.deep.equal([
      'Playwright JSON report is missing for managed case reconciliation',
    ]);
    reporter.results = [];
  });
});
