const fs = require('fs');
const os = require('os');
const path = require('path');
const { expect } = require('chai');

const runner = require('../run_tests');
const reporter = require('../lib/reporter');
const provenance = require('../lib/coverage_provenance');
const resultSummary = require('../lib/runner/result-summary');
const runArtifacts = require('../lib/runner/run-artifacts');
const coverageContract = require('../lib/coverage_contract');
const coverageInspector = require('../lib/coverage_inspector');

function mochaAllSkippedResult(stdout = '') {
  return {
    code: 0,
    stdout,
    stderr: ''
  };
}

function mochaAllSkippedReport() {
  return {
    stats: {
      tests: 2,
      passes: 0,
      pending: 2,
      failures: 0
    }
  };
}

function completeOperationCaseOutcomes(outcome = 'passed') {
  return coverageContract.BUSINESS_OPERATIONS.flatMap(operation => (
    operation.cases.map(item => ({ file: item.file, title: item.title, outcome }))
  ));
}

function writeJson(filePath, value) {
  fs.writeFileSync(filePath, JSON.stringify(value, null, 2), 'utf8');
}

function makeArtifactFixture(options = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-producer-'));
  const stagingDir = path.join(root, '.staging', 'run-fixture');
  fs.mkdirSync(stagingDir, { recursive: true });
  const startedAt = new Date(Date.now() - 1000).toISOString();
  const source = options.source || {
    gitCommit: 'a'.repeat(40),
    gitRef: 'main',
    dirty: false,
    diffHash: null
  };
  const run = {
    runName: 'run-fixture',
    stagingDir,
    reportDir: stagingDir,
    verificationRoot: root,
    startedAt,
    command: ['node', 'run_tests.js', '--include-e2e'],
    scope: options.scope || 'diagnostic',
    publicationRequested: options.publicationRequested === true,
    source,
    environment: {
      evidenceKind: 'local-real',
      lane: 'producer-contract',
      frontendUrl: 'http://127.0.0.1:9725',
      backendUrl: 'http://127.0.0.1:9999',
      mqttAddress: '127.0.0.1:1883',
      database: 'fixture',
      accountSource: 'contract fixture'
    }
  };
  const summary = {
    summary: {
      moduleOutcomes: [{ module: 'fixture', outcome: 'passed' }],
      caseOutcomes: completeOperationCaseOutcomes()
    }
  };
  writeJson(path.join(stagingDir, 'summary.json'), summary);
  if (options.endpoint !== false) {
    writeJson(path.join(stagingDir, 'endpoint-coverage.json'), {
      startedAt, finishedAt: new Date().toISOString(), total: 1, covered: 1, uncovered: 0, rate: '100.00'
    });
  }
  if (options.page !== false) {
    writeJson(path.join(stagingDir, 'page-coverage.json'), {
      startedAt,
      finishedAt: new Date().toISOString(),
      pages: { total: 1, covered: 1, uncovered: 0, rate: '100.00' },
      businessFlows: { total: 1, covered: 1, uncovered: 0, rate: '100.00' }
    });
  }
  if (options.go !== false) {
    writeJson(path.join(stagingDir, 'go-test-evidence.json'), {
      schema: 'aetherlink.go-test.evidence.v1',
      revision: source.gitCommit,
      startedAt,
      finishedAt: new Date().toISOString(),
      source,
      commandPolicy: { json: true, count: 1, cacheDisabled: true },
      packages: [],
      outcomes: coverageContract.ALL_GO_EVIDENCE.map(item => ({
        evidenceId: item.evidenceId,
        package: item.package,
        testFunction: item.testFunction,
        outcome: 'passed'
      })),
      errors: [],
      passed: true
    });
  }
  return { root, run };
}

describe('Runner result summary contract', function() {
  it('keeps the run_tests facade functions on the extracted module references', function() {
    expect(runner.summarizeMochaResult).to.equal(resultSummary.summarizeMochaResult);
    expect(runner.summarizePlaywrightResult).to.equal(resultSummary.summarizePlaywrightResult);
  });

  it('passes an all-skipped Mocha run with only structured runtime-external blocks', function() {
    const stdout = 'integration-blocked-meta: ' + JSON.stringify({
      reason: 'external identity provider is unavailable',
      category: 'runtime-external',
      seedable: false
    });
    const summary = resultSummary.summarizeMochaResult(
      mochaAllSkippedResult(stdout),
      mochaAllSkippedReport()
    );

    expect(summary).to.include({
      passed: true,
      outcome: 'partial-skip',
      skipped: 2
    });
    expect(summary.blockedReasons).to.deep.equal([{
      reason: 'external identity provider is unavailable',
      category: 'runtime-external',
      seedable: false
    }]);
  });

  it('fails an unblocked all-skipped Mocha run', function() {
    const summary = resultSummary.summarizeMochaResult(
      mochaAllSkippedResult(),
      mochaAllSkippedReport()
    );

    expect(summary).to.include({
      passed: false,
      outcome: 'all-skipped',
      skipped: 2
    });
    expect(summary.blockedReasons).to.deep.equal([]);
  });

  it('does not pass an all-skipped Mocha run blocked by seedable-local data', function() {
    const stdout = 'integration-blocked-meta: ' + JSON.stringify({
      reason: 'local device fixture is missing',
      category: 'seedable-local',
      seedable: true
    });
    const summary = resultSummary.summarizeMochaResult(
      mochaAllSkippedResult(stdout),
      mochaAllSkippedReport()
    );

    expect(summary).to.include({
      passed: false,
      outcome: 'all-skipped'
    });
    expect(summary.blockedReasons[0]).to.include({
      category: 'seedable-local',
      seedable: true
    });
  });

  it('fails a Playwright result when the report contains errors', function() {
    const summary = resultSummary.summarizePlaywrightResult(
      { code: 0, stdout: '', stderr: 'reporter error' },
      {
        stats: { expected: 1, skipped: 0, unexpected: 0 },
        errors: [{ message: 'report failed' }]
      }
    );

    expect(summary).to.include({
      passed: false,
      outcome: 'failed',
      skipped: 0,
      reason: 'reporter error'
    });
  });

  it('falls back to legacy blocked text after malformed structured metadata', function() {
    const summary = resultSummary.summarizeMochaResult(
      mochaAllSkippedResult([
        'integration-blocked-meta: {not-json}',
        'integration-blocked: requires runtime fixture or external dependency: remote broker unavailable'
      ].join('\n')),
      mochaAllSkippedReport()
    );

    expect(summary).to.include({
      passed: true,
      outcome: 'partial-skip'
    });
    expect(summary.blockedReasons).to.have.length(1);
    expect(summary.blockedReasons[0]).to.include({
      reason: 'requires runtime fixture or external dependency: remote broker unavailable',
      category: 'runtime-external',
      seedable: false
    });
  });

  it('extracts exact nested Mochawesome case outcomes', function() {
    const summary = resultSummary.summarizeMochaResult(
      { code: 0, stdout: '', stderr: '' },
      {
        stats: { tests: 1, passes: 1, pending: 0, failures: 0 },
        results: [{
          suites: [{
            fullFile: 'C:\\repo\\automation_tests\\tests\\example.test.js',
            tests: [{
              title: 'works',
              fullTitle: 'outer nested works',
              state: 'passed',
              pass: true,
              duration: 3
            }],
            suites: []
          }]
        }]
      }
    );

    expect(summary.caseResults).to.deep.equal([{
      file: 'tests/example.test.js',
      title: 'works',
      fullTitle: 'outer nested works',
      outcome: 'passed',
      duration: 3
    }]);
  });

  it('collects canonical Playwright case identity and retry outcome', function() {
    const [testCase] = resultSummary.collectPlaywrightCases({
      suites: [{
        title: 'device suite',
        file: 'e2e/02_device.spec.js',
        specs: [{
          id: 'spec-id',
          title: 'renders device',
          ok: true,
          tests: [{
            status: 'expected',
            results: [{ status: 'passed', retry: 1, workerIndex: 3, parallelIndex: 0 }]
          }]
        }]
      }]
    });

    expect(testCase).to.include({
      caseId: 'spec-id',
      file: 'e2e/02_device.spec.js',
      title: 'renders device',
      outcome: 'flaky'
    });
    expect(testCase.titlePath).to.deep.equal(['device suite', 'renders device']);
    expect(testCase.attempts[0]).to.include({ retry: 1, workerIndex: 3 });
  });

  it('qualifies Mocha events by exact fullTitle when no stable case ID exists', function() {
    const [qualified] = resultSummary.qualifyCoverageProvenance([
      provenance.createEvent({
        eventId: 'mocha-full-title-event',
        runId: 'runner-run',
        module: 'rule-chain-business',
        kind: 'endpoint',
        target: 'POST /api/v1/rule-chain',
        statusCode: 200,
        case: {
          file: 'C:\\repo\\automation_tests\\tests\\29_rule_chain_business.test.js',
          title: 'supports branch execution',
          titlePath: ['Rule chain business supports branch execution']
        }
      })
    ], resultSummary.createRunnerSummary({
      passed: true,
      outcome: 'passed',
      caseResults: [{
        file: 'tests/29_rule_chain_business.test.js',
        title: 'supports branch execution',
        fullTitle: 'Rule chain business supports branch execution',
        outcome: 'passed'
      }]
    }), 'rule-chain-business', 'runner-run');

    expect(qualified.disposition).to.equal('effective');
  });

  it('finalizes only passing same-module provenance as effective', function() {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-finalize-'));
    const ledgerPath = path.join(outputDir, 'module-provenance.json');
    const event = provenance.createEvent({
      eventId: 'runner-event',
      runId: 'runner-run',
      module: 'device',
      kind: 'page',
      target: '/device/manage',
      case: {
        file: 'e2e/02_device.spec.js',
        title: 'renders device',
        titlePath: ['device suite', 'renders device'],
        caseId: 'spec-id'
      }
    });
    provenance.writeLedger(ledgerPath, [event]);

    try {
      const summary = resultSummary.createRunnerSummary({
        passed: true,
        outcome: 'passed',
        caseResults: [{ caseId: 'spec-id', outcome: 'passed' }]
      });
      const qualified = runner.finalizeCoverageProvenance(
        { key: 'device' },
        { provenanceFile: ledgerPath, coverageRunId: 'runner-run' },
        summary
      );

      expect(qualified[0].disposition).to.equal('effective');
      expect(summary.coverageProvenance).to.include({ total: 1, effective: 1, diagnostic: 0 });
      expect(provenance.readLedger(ledgerPath).events[0].disposition).to.equal('effective');
    } finally {
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('demotes authoritative cases when their module is partial', function() {
    const [qualified] = resultSummary.qualifyCoverageProvenance([
      provenance.createEvent({
        eventId: 'partial-event',
        runId: 'runner-run',
        module: 'device',
        kind: 'page',
        target: '/device/manage',
        case: { file: 'e2e/02_device.spec.js', title: 'renders device', caseId: 'spec-id' }
      })
    ], resultSummary.createRunnerSummary({
      passed: true,
      outcome: 'partial-skip',
      caseResults: [{ caseId: 'spec-id', outcome: 'passed' }]
    }), 'device', 'runner-run');

    expect(qualified.disposition).to.equal('diagnostic');
  });

  it('records qualified provenance in the persisted reporter result', async function() {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-record-order-'));
    const ledgerPath = path.join(outputDir, 'module-provenance.json');
    provenance.writeLedger(ledgerPath, [provenance.createEvent({
      eventId: 'record-order-event',
      runId: 'record-order-run',
      module: 'contract-order',
      kind: 'endpoint',
      target: 'GET /api/v1/health',
      statusCode: 200,
      case: {
        file: 'tests/contract-order.test.js',
        title: 'reports health',
        titlePath: ['contract order reports health']
      }
    })]);

    reporter.results = [];
    try {
      const record = await runner.executeModuleRun({
        mod: {
          key: 'contract-order',
          name: 'contract order',
          file: 'tests/contract-order.test.js',
          metadataFile: 'tests/contract-order.test.js',
          evidenceLabel: 'contract'
        },
        kind: 'API',
        type: 'api',
        execute: async () => ({
          code: 0,
          stdout: '',
          stderr: '',
          coverageFile: null,
          provenanceFile: ledgerPath,
          coverageRunId: 'record-order-run'
        }),
        summarize: () => resultSummary.createRunnerSummary({
          passed: true,
          outcome: 'passed',
          caseResults: [{
            file: 'tests/contract-order.test.js',
            title: 'reports health',
            fullTitle: 'contract order reports health',
            outcome: 'passed'
          }]
        }),
        mergeCoverage: () => {}
      });

      expect(record.summary.coverageProvenance).to.include({
        total: 1,
        effective: 1,
        diagnostic: 0
      });
      expect(reporter.results).to.have.length(1);
      expect(reporter.results[0].coverageProvenance).to.include({
        total: 1,
        effective: 1,
        diagnostic: 0
      });
    } finally {
      reporter.results = [];
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('rewrites module coverage provenance before aggregate merge', async function() {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-qualified-merge-'));
    const coveragePath = path.join(outputDir, 'module-coverage.json');
    const ledgerPath = path.join(outputDir, 'module-provenance.json');
    const event = provenance.createEvent({
      eventId: 'qualified-merge-event',
      runId: 'qualified-merge-run',
      module: 'contract-order',
      kind: 'endpoint',
      target: 'GET /api/v1/health',
      statusCode: 200,
      case: {
        file: 'tests/contract-order.test.js',
        title: 'reports health',
        titlePath: ['contract order reports health']
      }
    });
    fs.writeFileSync(coveragePath, JSON.stringify({
      hits: [],
      provenance: { schema: provenance.SCHEMA, events: [event] }
    }), 'utf8');
    provenance.writeLedger(ledgerPath, [event]);

    reporter.results = [];
    let mergedPayload = null;
    try {
      const record = await runner.executeModuleRun({
        mod: {
          key: 'contract-order',
          name: 'contract order',
          file: 'tests/contract-order.test.js',
          metadataFile: 'tests/contract-order.test.js',
          evidenceLabel: 'contract'
        },
        kind: 'API',
        type: 'api',
        execute: async () => ({
          code: 0,
          stdout: '',
          stderr: '',
          coverageFile: coveragePath,
          provenanceFile: ledgerPath,
          coverageRunId: 'qualified-merge-run'
        }),
        summarize: () => resultSummary.createRunnerSummary({
          passed: true,
          outcome: 'passed',
          caseResults: [{
            file: 'tests/contract-order.test.js',
            title: 'reports health',
            fullTitle: 'contract order reports health',
            outcome: 'passed'
          }]
        }),
        mergeCoverage: filePath => {
          mergedPayload = JSON.parse(fs.readFileSync(filePath, 'utf8'));
        }
      });

      expect(record.result.coverageArtifactQualified).to.equal(true);
      expect(mergedPayload.provenance.events[0].disposition).to.equal('effective');
      expect(JSON.parse(fs.readFileSync(coveragePath, 'utf8')).provenance.events[0].disposition)
        .to.equal('effective');
      expect(reporter.results[0].coverageProvenance).to.include({
        total: 1,
        effective: 1,
        diagnostic: 0
      });
    } finally {
      reporter.results = [];
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('does not merge malformed module coverage after provenance qualification', async function() {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-malformed-merge-'));
    const coveragePath = path.join(outputDir, 'module-coverage.json');
    const ledgerPath = path.join(outputDir, 'module-provenance.json');
    const event = provenance.createEvent({
      eventId: 'malformed-merge-event',
      runId: 'malformed-merge-run',
      module: 'contract-order',
      kind: 'endpoint',
      target: 'GET /api/v1/health',
      statusCode: 200,
      case: {
        file: 'tests/contract-order.test.js',
        title: 'reports health',
        titlePath: ['contract order reports health']
      }
    });
    fs.writeFileSync(coveragePath, '{not-json}', 'utf8');
    provenance.writeLedger(ledgerPath, [event]);

    reporter.results = [];
    let mergeCalls = 0;
    try {
      const record = await runner.executeModuleRun({
        mod: {
          key: 'contract-order',
          name: 'contract order',
          file: 'tests/contract-order.test.js',
          metadataFile: 'tests/contract-order.test.js',
          evidenceLabel: 'contract'
        },
        kind: 'API',
        type: 'api',
        execute: async () => ({
          code: 0,
          stdout: '',
          stderr: '',
          coverageFile: coveragePath,
          provenanceFile: ledgerPath,
          coverageRunId: 'malformed-merge-run'
        }),
        summarize: () => resultSummary.createRunnerSummary({
          passed: true,
          outcome: 'passed',
          caseResults: [{
            file: 'tests/contract-order.test.js',
            title: 'reports health',
            fullTitle: 'contract order reports health',
            outcome: 'passed'
          }]
        }),
        mergeCoverage: () => { mergeCalls++; }
      });

      expect(record.result.coverageArtifactQualified).to.equal(false);
      expect(mergeCalls).to.equal(0);
      expect(record.summary.coverageProvenance).to.include({
        total: 1,
        effective: 0,
        diagnostic: 1
      });
      expect(provenance.readLedger(ledgerPath).events[0]).to.include({
        disposition: 'diagnostic'
      });
      expect(provenance.readLedger(ledgerPath).events[0].diagnostics)
        .to.have.property('coverageArtifactRewriteFailed');
    } finally {
      reporter.results = [];
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('redacts credential-like run metadata before any manifest is written', function() {
    const metadata = runArtifacts.sanitizeRunMetadata(
      ['node', 'run_tests.js', '--target=https://alice:password@example.com', '--token=secret-token'],
      {
        frontendUrl: 'https://bob:credential@frontend.example.com',
        backendUrl: 'https://api.example.com?api_key=secret-key',
        lane: 'producer-contract'
      }
    );
    expect(metadata.command.join(' ')).to.not.include('password');
    expect(metadata.command.join(' ')).to.not.include('secret-token');
    expect(metadata.environment.frontendUrl).to.equal('https://frontend.example.com');
    expect(metadata.environment.backendUrl).to.include('[redacted-credential]');
    expect(metadata.environment.backendUrl).to.not.include('api_key=');
    expect(metadata.redaction.secretsRemoved).to.equal(true);
    expect(metadata.redaction.notes).to.match(/^4 credential-like/);
  });

  it('finalizes API-only and E2E-only runs as honest diagnostic artifacts', function() {
    for (const missing of ['endpoint', 'page']) {
      const fixture = makeArtifactFixture({
        endpoint: missing !== 'endpoint',
        page: missing !== 'page'
      });
      try {
        const result = runArtifacts.finalizeRunArtifacts(fixture.run, {
          finishedAt: new Date(),
          exitCode: 0,
          strictIntegration: true,
          cleanup: { status: 'passed', notes: [] },
          blockingGaps: []
        });
        expect(result.archiveDir).to.equal(null);
        expect(result.diagnosticDir).to.equal(fixture.run.stagingDir);
        expect(Object.keys(result.manifest.reports).sort()).to.deep.equal(
          missing === 'endpoint'
            ? ['goTestEvidence', 'pageCoverage', 'summary']
            : ['endpointCoverage', 'goTestEvidence', 'summary']
        );
        expect(result.manifest.verdict).to.equal('blocked');
        expect(result.manifest.blockingGaps).to.include('filtered-or-partial-run');
      } finally {
        fs.rmSync(fixture.root, { recursive: true, force: true });
      }
    }
  });

  it('preserves failed precedence when source is also dirty', function() {
    const fixture = makeArtifactFixture({
      source: { gitCommit: 'a'.repeat(40), gitRef: 'main', dirty: true, diffHash: 'b'.repeat(64) }
    });
    try {
      const result = runArtifacts.finalizeRunArtifacts(fixture.run, {
        finishedAt: new Date(),
        exitCode: 1,
        strictIntegration: true,
        cleanup: { status: 'failed', notes: ['fixture cleanup failed'] },
        blockingGaps: []
      });
      expect(result.manifest.verdict).to.equal('failed');
      expect(result.manifest.blockingGaps).to.include('dirty-source');
    } finally {
      fs.rmSync(fixture.root, { recursive: true, force: true });
    }
  });

  it('derives cleanup truth only from passing cleanup-dimension cases', function() {
    const complete = completeOperationCaseOutcomes();
    expect(runArtifacts.deriveCleanupEvidence({ caseOutcomes: complete }).status).to.equal('passed');
    const cleanupCase = coverageContract.BUSINESS_OPERATIONS
      .flatMap(operation => operation.cases)
      .find(item => item.dimensions.includes('cleanup'));
    const incomplete = complete.filter(item => (
      item.file !== cleanupCase.file || item.title !== cleanupCase.title
    ));
    expect(runArtifacts.deriveCleanupEvidence({ caseOutcomes: incomplete }).status).to.equal('not-run');
    expect(runArtifacts.deriveCleanupEvidence({
      caseOutcomes: complete.map(item => (
        item.file === cleanupCase.file && item.title === cleanupCase.title
          ? { ...item, outcome: 'failed' }
          : item
      ))
    }).status).to.equal('failed');
  });

  it('writes an explicit failed manifest for interrupted staging', function() {
    const fixture = makeArtifactFixture({ endpoint: false, page: false });
    try {
      const result = runArtifacts.writeInterruptedManifest(fixture.run, {
        exitCode: 2,
        strictIntegration: true,
        blockingGaps: ['service-unavailable']
      });
      expect(result.manifest).to.include({ verdict: 'failed', scope: 'diagnostic' });
      expect(result.manifest.run.exitCode).to.equal(2);
      expect(result.manifest.cleanup.status).to.equal('not-run');
      expect(fs.existsSync(path.join(fixture.run.stagingDir, 'archive-manifest.json'))).to.equal(true);
    } finally {
      fs.rmSync(fixture.root, { recursive: true, force: true });
    }
  });

  it('publishes only after inspector-backed validation and remaps report paths', function() {
    const fixture = makeArtifactFixture({ scope: 'full', publicationRequested: true });
    const nestedArtifact = path.join(fixture.run.stagingDir, 'playwright', 'traces', 'trace.zip');
    fs.mkdirSync(path.dirname(nestedArtifact), { recursive: true });
    fs.writeFileSync(nestedArtifact, 'nested-playwright-artifact', 'utf8');
    try {
      const result = runArtifacts.finalizeRunArtifacts(fixture.run, {
        finishedAt: new Date(),
        exitCode: 0,
        strictIntegration: true,
        cleanup: { status: 'passed', notes: ['verified cleanup'] },
        blockingGaps: []
      });
      expect(result.archiveDir).to.equal(path.join(fixture.root, 'run-fixture'));
      expect(result.reportDir).to.equal(result.archiveDir);
      expect(result.reports.summary).to.equal(path.join(result.archiveDir, 'summary.json'));
      expect(result.reports.goTestEvidence).to.equal(path.join(result.archiveDir, 'go-test-evidence.json'));
      expect(fs.existsSync(result.reports.summary)).to.equal(true);
      expect(fs.existsSync(result.reports.goTestEvidence)).to.equal(true);
      expect(fs.readFileSync(path.join(result.archiveDir, 'playwright', 'traces', 'trace.zip'), 'utf8'))
        .to.equal('nested-playwright-artifact');
      expect(fs.existsSync(fixture.run.stagingDir)).to.equal(false);
    } finally {
      fs.rmSync(fixture.root, { recursive: true, force: true });
    }
  });

  it('requires exact Go evidence before a full run can publish', function() {
    const fixture = makeArtifactFixture({ scope: 'full', publicationRequested: true, go: false });
    try {
      expect(() => runArtifacts.finalizeRunArtifacts(fixture.run, {
        finishedAt: new Date(),
        exitCode: 0,
        strictIntegration: true,
        cleanup: { status: 'passed', notes: [] },
        blockingGaps: []
      })).to.throw('Required canonical report is missing: go-test-evidence.json');
      expect(fs.existsSync(fixture.run.stagingDir)).to.equal(true);
    } finally {
      fs.rmSync(fixture.root, { recursive: true, force: true });
    }
  });

  it('keeps a full candidate in staging when inspector validation fails', function() {
    const fixture = makeArtifactFixture({ scope: 'full', publicationRequested: true });
    const originalClassify = coverageInspector.classifyArchive;
    coverageInspector.classifyArchive = () => ({ eligible: false, reasonCodes: ['report-hash-mismatch:summary'] });
    try {
      const result = runArtifacts.finalizeRunArtifacts(fixture.run, {
        finishedAt: new Date(),
        exitCode: 0,
        strictIntegration: true,
        cleanup: { status: 'passed', notes: [] },
        blockingGaps: []
      });
      expect(result.archiveDir).to.equal(null);
      expect(result.publicationErrors).to.include('inspector: report-hash-mismatch:summary');
      expect(fs.existsSync(fixture.run.stagingDir)).to.equal(true);
    } finally {
      coverageInspector.classifyArchive = originalClassify;
      fs.rmSync(fixture.root, { recursive: true, force: true });
    }
  });

  it('prefers explicitReport without reading result.reportJson', function() {
    const originalReadFileSync = fs.readFileSync;
    let readAttempted = false;
    fs.readFileSync = function() {
      readAttempted = true;
      throw new Error('unexpected report read');
    };

    try {
      const summary = resultSummary.summarizeMochaResult(
        {
          code: 0,
          stdout: '',
          stderr: '',
          reportJson: 'must-not-be-read.json'
        },
        {
          stats: { tests: 1, passes: 1, pending: 0, failures: 0 }
        }
      );

      expect(summary).to.include({ passed: true, outcome: 'passed' });
      expect(readAttempted).to.equal(false);
    } finally {
      fs.readFileSync = originalReadFileSync;
    }
  });
});
