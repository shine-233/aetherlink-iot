'use strict';

const crypto = require('crypto');
const { expect } = require('chai');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const inspector = require('../lib/coverage_inspector');
const coverageContract = require('../lib/coverage_contract');

function completeOperationCaseOutcomes(operations = coverageContract.BUSINESS_OPERATIONS) {
  return operations.flatMap(operation => (
    operation.cases.map(item => ({ file: item.file, title: item.title, outcome: 'passed' }))
  ));
}

function completeReadyOperations() {
  return coverageContract.BUSINESS_OPERATIONS.map(operation => ({
    ...operation,
    requiredDimensions: [...new Set(operation.cases.flatMap(item => item.dimensions || []))]
  }));
}

function operationCheck(result) {
  return result.checks.find(item => item.id === 'case-outcomes');
}

const projectRoot = path.resolve(__dirname, '..', '..');

function git(args) {
  const result = spawnSync('git', ['-C', projectRoot, ...args], { encoding: 'utf8' });
  expect(result.status, result.stderr).to.equal(0);
  return String(result.stdout || '').trim();
}

function sha256(filePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
}

function writeJson(filePath, value) {
  fs.writeFileSync(filePath, JSON.stringify(value, null, 2), 'utf8');
}

function makeVerification(options = {}) {
  const verificationRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-inspector-'));
  const archiveDir = path.join(verificationRoot, options.name || 'run-current');
  fs.mkdirSync(archiveDir);

  const finishedAt = new Date().toISOString();
  const startedAt = new Date(Date.parse(finishedAt) - 60_000).toISOString();
  const summary = {
    revision: git(['rev-parse', 'HEAD']),
    verdict: 'passed',
    startedAt,
    finishedAt,
    moduleOutcomes: [{ module: 'api', outcome: 'passed' }],
    caseOutcomes: completeOperationCaseOutcomes(),
    ...(options.summary || {})
  };
  const endpointCoverage = {
    startedAt,
    finishedAt,
    total: 2,
    covered: 2,
    uncovered: 0,
    rate: '100.00',
    ...(options.endpointCoverage || {})
  };
  const pageCoverage = {
    startedAt,
    finishedAt,
    pages: { total: 2, covered: 2, uncovered: 0, rate: '100.00' },
    businessFlows: { total: 1, covered: 1, uncovered: 0, rate: '100.00' },
    ...(options.pageCoverage || {})
  };
  const goTestEvidence = {
    schema: 'aetherlink.go-test.evidence.v1',
    revision: summary.revision,
    startedAt,
    finishedAt,
    source: {
      gitCommit: summary.revision,
      gitRef: git(['branch', '--show-current']) || null,
      dirty: false,
      diffHash: null
    },
    commandPolicy: { json: true, count: 1, cacheDisabled: true },
    packages: [],
    outcomes: coverageContract.ALL_GO_EVIDENCE.map(item => ({
      evidenceId: item.evidenceId,
      package: item.package,
      testFunction: item.testFunction,
      outcome: 'passed'
    })),
    errors: [],
    passed: true,
    ...(options.goTestEvidence || {})
  };

  const reports = { summary, endpointCoverage, pageCoverage, goTestEvidence };
  const reportIndex = {};
  for (const [key, value] of Object.entries(reports)) {
    const name = inspectorKeyToName(key);
    const reportPath = path.join(archiveDir, name);
    writeJson(reportPath, value);
    reportIndex[key] = { path: name, sha256: sha256(reportPath) };
  }

  const manifest = {
    schema: inspector.ARCHIVE_SCHEMA,
    kind: 'automation-coverage',
    archivedAt: finishedAt,
    scope: 'full',
    run: {
      startedAt,
      finishedAt,
      command: ['node', 'run_tests.js', '--include-e2e'],
      exitCode: 0,
      strictIntegration: true
    },
    source: {
      gitCommit: git(['rev-parse', 'HEAD']),
      gitRef: git(['branch', '--show-current']) || null,
      dirty: false,
      diffHash: null
    },
    environment: {
      evidenceKind: 'local-real',
      lane: 'contract-fixture',
      frontendUrl: 'http://127.0.0.1:9725',
      backendUrl: 'http://127.0.0.1:9999',
      mqttAddress: '127.0.0.1:1883',
      database: 'fixture',
      accountSource: 'contract fixture'
    },
    reports: reportIndex,
    cleanup: { status: 'passed', notes: [] },
    redaction: { secretsRemoved: true, notes: '' },
    verdict: 'passed',
    blockingGaps: [],
    ...(options.manifest || {})
  };
  writeJson(path.join(archiveDir, 'archive-manifest.json'), manifest);
  return { verificationRoot, archiveDir };
}

function inspectorKeyToName(key) {
  return {
    summary: 'summary.json',
    endpointCoverage: 'endpoint-coverage.json',
    pageCoverage: 'page-coverage.json',
    goTestEvidence: 'go-test-evidence.json'
  }[key];
}

function removeDirectory(directory) {
  fs.rmSync(directory, { recursive: true, force: true });
}

describe('Coverage evidence inspector contract [00_coverage_inspector_contract]', function () {
  it('fails closed for a wrong project root without synthetic coverage metrics', function () {
    const wrongRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-wrong-root-'));
    try {
      const result = inspector.inspect({ projectRoot: wrongRoot });
      expect(result.project.identity).to.equal('unknown');
      expect(result.metrics).to.equal(null);
      expect(result.readiness.status).to.equal('unknown');
      expect(result.readiness.blockingReasons).to.include('invalid-project-root');
    } finally {
      removeDirectory(wrongRoot);
    }
  });

  it('accepts reporter-shaped nested summaries without losing canonical outcomes', function () {
    const fixture = makeVerification({
      summary: {
        revision: undefined,
        verdict: undefined,
        moduleOutcomes: undefined,
        caseOutcomes: undefined,
        summary: {
          revision: git(['rev-parse', 'HEAD']),
          verdict: 'passed',
          moduleOutcomes: [{ module: 'api', outcome: 'passed' }],
          caseOutcomes: completeOperationCaseOutcomes()
        }
      }
    });
    try {
      const result = inspector.inspect({
        projectRoot,
        verificationRoot: fixture.verificationRoot
      });
      expect(operationCheck(result).status).to.equal('pass');
      expect(result.checks.find(item => item.id === 'summary-consistency').status)
        .to.equal('pass');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('retains legacy manifest history but does not select it as current evidence', function () {
    const verificationRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-inspector-'));
    const archiveDir = path.join(verificationRoot, 'legacy-run');
    fs.mkdirSync(archiveDir);
    writeJson(path.join(archiveDir, 'archive-manifest.json'), {
      archivedAt: '2026-08-23T03:44:09.808Z',
      command: ['node', 'run_tests.js', '--archive']
    });

    try {
      const result = inspector.inspect({ projectRoot, verificationRoot });
      expect(result.archives).to.have.length(1);
      expect(result.archives[0]).to.include({
        compatibility: 'legacy',
        validity: 'legacy-manifest-only',
        eligible: false
      });
      expect(result.archiveSelection.status).to.equal('none');
      expect(result.metrics).to.equal(null);
      expect(result.readiness.status).to.equal('unknown');
    } finally {
      removeDirectory(verificationRoot);
    }
  });

  it('accepts a complete current archive only when every canonical operation is statically ready', function () {
    const fixture = makeVerification();
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.archiveSelection.status).to.equal('selected');
      expect(result.metrics.businessFlows).to.deep.equal({ total: 1, covered: 1, uncovered: 0, rate: 100 });
      expect(operationCheck(result)).to.include({ status: 'pass', reasonCode: null });
      expect(result.readiness).to.deep.equal({ status: 'pass', blockingReasons: [] });
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('continues to accept a complete operation inventory override used by negative controls', function () {
    const fixture = makeVerification();
    const originalOperations = coverageContract.BUSINESS_OPERATIONS;
    coverageContract.BUSINESS_OPERATIONS = completeReadyOperations();
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(operationCheck(result)).to.include({ status: 'pass', reasonCode: null });
      expect(result.readiness).to.deep.equal({ status: 'pass', blockingReasons: [] });
    } finally {
      coverageContract.BUSINESS_OPERATIONS = originalOperations;
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('blocks a manifest for another commit', function () {
    const fixture = makeVerification({
      manifest: {
        source: {
          gitCommit: '0'.repeat(40),
          gitRef: 'main',
          dirty: false,
          diffHash: null
        }
      }
    });
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.archiveSelection.status).to.equal('none');
      expect(result.archives[0].reasonCodes).to.include('revision-mismatch');
      expect(result.readiness.status).to.equal('unknown');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('fails closed for missing or tampered Go evidence artifacts', function () {
    const variants = [
      fixture => fs.unlinkSync(path.join(fixture.archiveDir, 'go-test-evidence.json')),
      fixture => fs.appendFileSync(path.join(fixture.archiveDir, 'go-test-evidence.json'), '\n', 'utf8')
    ];
    const expectedReasons = ['missing-report:goTestEvidence', 'report-hash-mismatch:goTestEvidence'];
    variants.forEach((mutate, index) => {
      const fixture = makeVerification();
      try {
        mutate(fixture);
        const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
        expect(result.archiveSelection.status).to.equal('none');
        expect(result.archives[0].reasonCodes).to.include(expectedReasons[index]);
      } finally {
        removeDirectory(fixture.verificationRoot);
      }
    });
  });

  it('fails closed for malformed or non-passing exact Go outcomes', function () {
    const first = coverageContract.ALL_GO_EVIDENCE[0];
    const variants = [
      [{ schema: 'wrong' }, 'incomplete-go-test-evidence'],
      [{ outcomes: [] }, 'incomplete-go-test-evidence'],
      [{
        outcomes: coverageContract.ALL_GO_EVIDENCE.map(item => ({
          evidenceId: item.evidenceId,
          package: item.package,
          testFunction: item.testFunction,
          outcome: item.evidenceId === first.evidenceId ? 'failed' : 'passed'
        })),
        passed: false
      }, 'incomplete-go-test-evidence'],
      [{ errors: [{ reason: 'go-test-build-failed' }], passed: false }, 'go-runtime-errors']
    ];
    for (const [goTestEvidence, expectedReason] of variants) {
      const fixture = makeVerification({ goTestEvidence });
      try {
        const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
        expect(result.readiness.status).to.equal('fail');
        expect(result.readiness.blockingReasons).to.include(expectedReason);
      } finally {
        removeDirectory(fixture.verificationRoot);
      }
    }
  });

  it('binds exact Go evidence to revision, command policy, source state, and run interval', function () {
    const variants = [
      [{ revision: '0'.repeat(40) }, 'go-evidence-revision-mismatch'],
      [{ commandPolicy: { json: true, count: 0, cacheDisabled: false } }, 'invalid-go-command-policy'],
      [{ source: { gitCommit: git(['rev-parse', 'HEAD']), gitRef: null, dirty: true, diffHash: 'a'.repeat(64) } }, 'go-evidence-source-mismatch'],
      [{ startedAt: '2020-01-01T00:00:00.000Z' }, 'report-interval-mismatch']
    ];
    for (const [goTestEvidence, expectedReason] of variants) {
      const fixture = makeVerification({ goTestEvidence });
      try {
        const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
        expect(result.readiness.status).to.equal('fail');
        expect(result.readiness.blockingReasons).to.include(expectedReason);
      } finally {
        removeDirectory(fixture.verificationRoot);
      }
    }
  });

  it('rejects report path traversal', function () {
    const fixture = makeVerification();
    const manifestPath = path.join(fixture.archiveDir, 'archive-manifest.json');
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    manifest.reports.summary.path = '../summary.json';
    writeJson(manifestPath, manifest);

    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.archiveSelection.status).to.equal('none');
      expect(result.archives[0].reasonCodes).to.include('invalid-report-path:summary');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('does not convert complete reachability into business readiness', function () {
    const fixture = makeVerification({
      pageCoverage: {
        pages: { total: 2, covered: 2, uncovered: 0, rate: '100.00' },
        businessFlows: { total: 1, covered: 0, uncovered: 1, rate: '0.00' }
      }
    });
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.metrics.endpointReachability.rate).to.equal(100);
      expect(result.metrics.routeRender.rate).to.equal(100);
      expect(result.metrics.businessFlows.rate).to.equal(0);
      expect(result.readiness.status).to.equal('fail');
      expect(result.readiness.blockingReasons).to.include('incomplete-business-flow-evidence');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('blocks contradictory summary and manifest verdicts', function () {
    const fixture = makeVerification({ summary: { verdict: 'failed' } });
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.readiness.status).to.equal('fail');
      expect(result.readiness.blockingReasons).to.include('summary-verdict-mismatch');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('rejects malformed, unrelated, and duplicate operation identities', function () {
    const variants = [
      [{ file: '', title: '', outcome: 'passed' }],
      [{ file: 'tests/unrelated.test.js', title: 'unrelated case', outcome: 'passed' }],
      (() => {
        const outcomes = completeOperationCaseOutcomes();
        return [...outcomes, { ...outcomes[0] }];
      })()
    ];

    for (const caseOutcomes of variants) {
      const fixture = makeVerification({ summary: { caseOutcomes } });
      try {
        const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
        expect(operationCheck(result)).to.include({
          status: 'fail',
          reasonCode: 'invalid-operation-case-outcomes'
        });
      } finally {
        removeDirectory(fixture.verificationRoot);
      }
    }
  });

  it('keeps missing and skipped operation outcomes incomplete', function () {
    const complete = completeOperationCaseOutcomes();
    const variants = [
      complete.slice(1),
      complete.map((item, index) => index === 0 ? { ...item, outcome: 'skipped' } : item)
    ];

    for (const caseOutcomes of variants) {
      const fixture = makeVerification({ summary: { caseOutcomes } });
      try {
        const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
        expect(operationCheck(result)).to.include({
          status: 'unknown',
          reasonCode: 'incomplete-operation-case-outcomes'
        });
      } finally {
        removeDirectory(fixture.verificationRoot);
      }
    }
  });

  it('fails when an exact canonical operation case fails', function () {
    const caseOutcomes = completeOperationCaseOutcomes();
    caseOutcomes[0] = { ...caseOutcomes[0], outcome: 'failed' };
    const fixture = makeVerification({ summary: { caseOutcomes } });
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(operationCheck(result)).to.include({
        status: 'fail',
        reasonCode: 'failed-operation-case-outcomes'
      });
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('blocks partial skips even when coverage reports are complete', function () {
    const fixture = makeVerification({
      summary: { moduleOutcomes: [{ module: 'e2e', outcome: 'partial-skip' }] }
    });
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.readiness.status).to.equal('fail');
      expect(result.readiness.blockingReasons).to.include('incomplete-module-outcomes');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('rejects reports without a manifest and malformed manifests', function () {
    const variants = [
      manifestPath => fs.unlinkSync(manifestPath),
      manifestPath => fs.writeFileSync(manifestPath, '{not-json', 'utf8')
    ];
    const expectedReasons = ['reports-without-manifest', 'malformed-manifest'];

    variants.forEach((mutate, index) => {
      const fixture = makeVerification();
      try {
        const manifestPath = path.join(fixture.archiveDir, 'archive-manifest.json');
        mutate(manifestPath);
        const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
        expect(result.archiveSelection.status).to.equal('none');
        expect(result.archives[0].reasonCodes).to.include(expectedReasons[index]);
        expect(result.metrics).to.equal(null);
      } finally {
        removeDirectory(fixture.verificationRoot);
      }
    });
  });

  it('rejects incomplete reports and dirty archived source state', function () {
    const variants = [
      fixture => fs.unlinkSync(path.join(fixture.archiveDir, 'page-coverage.json')),
      fixture => {
        const manifestPath = path.join(fixture.archiveDir, 'archive-manifest.json');
        const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
        manifest.source.dirty = true;
        writeJson(manifestPath, manifest);
      }
    ];
    const expectedReasons = ['missing-report:pageCoverage', 'dirty-archive-source'];

    variants.forEach((mutate, index) => {
      const fixture = makeVerification();
      try {
        mutate(fixture);
        const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
        expect(result.archiveSelection.status).to.equal('none');
        expect(result.archives[0].reasonCodes).to.include(expectedReasons[index]);
      } finally {
        removeDirectory(fixture.verificationRoot);
      }
    });
  });

  it('rejects stale canonical evidence', function () {
    const finishedAt = '2026-01-01T00:00:00.000Z';
    const fixture = makeVerification({
      manifest: {
        archivedAt: finishedAt,
        run: {
          startedAt: '2025-12-31T23:59:00.000Z',
          finishedAt,
          command: ['node', 'run_tests.js', '--include-e2e'],
          exitCode: 0,
          strictIntegration: true
        }
      }
    });
    try {
      const result = inspector.inspect({
        projectRoot,
        verificationRoot: fixture.verificationRoot,
        now: new Date('2026-09-08T00:00:00.000Z')
      });
      expect(result.archiveSelection.status).to.equal('none');
      expect(result.archives[0].reasonCodes).to.include('stale-archive');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('does not let synthetic evidence satisfy a real-device gate', function () {
    const fixture = makeVerification({
      manifest: {
        environment: {
          evidenceKind: 'synthetic',
          lane: 'contract-fixture',
          frontendUrl: 'http://127.0.0.1:9725',
          backendUrl: 'http://127.0.0.1:9999',
          mqttAddress: '127.0.0.1:1883',
          database: 'fixture',
          accountSource: 'contract fixture'
        }
      }
    });
    try {
      const result = inspector.inspect({
        projectRoot,
        verificationRoot: fixture.verificationRoot,
        requiredEvidenceKind: 'real-device'
      });
      expect(result.archiveSelection.status).to.equal('selected');
      expect(result.readiness.status).to.equal('fail');
      expect(result.readiness.blockingReasons).to.include('insufficient-evidence-kind');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('rejects credentials embedded in manifest URLs', function () {
    const fixture = makeVerification({
      manifest: {
        environment: {
          evidenceKind: 'local-real',
          lane: 'contract-fixture',
          frontendUrl: 'http://user:password@127.0.0.1:9725',
          backendUrl: 'http://127.0.0.1:9999',
          mqttAddress: '127.0.0.1:1883',
          database: 'fixture',
          accountSource: 'contract fixture'
        }
      }
    });
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.archiveSelection.status).to.equal('none');
      expect(result.archives[0].reasonCodes).to.include('manifest-contains-credentials');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('can require a clean current worktree for release comparison', function () {
    const fixture = makeVerification();
    try {
      const result = inspector.inspect({
        projectRoot,
        verificationRoot: fixture.verificationRoot,
        requireCleanCurrentWorktree: true
      });
      const expectedStatus = git(['status', '--porcelain=v1']).length > 0 ? 'fail' : 'pass';
      expect(result.readiness.status).to.equal(expectedStatus);
      if (expectedStatus === 'fail') {
        expect(result.readiness.blockingReasons).to.include('current-worktree-dirty');
      }
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('selects the newest current run even when it failed', function () {
    const older = makeVerification({ name: 'older-pass' });
    const newerDir = path.join(older.verificationRoot, 'newer-failed');
    fs.cpSync(older.archiveDir, newerDir, { recursive: true });
    const manifestPath = path.join(newerDir, 'archive-manifest.json');
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    const finishedAt = new Date(Date.parse(manifest.run.finishedAt) + 60_000).toISOString();
    manifest.archivedAt = finishedAt;
    manifest.run.finishedAt = finishedAt;
    manifest.run.exitCode = 1;
    manifest.verdict = 'failed';
    const summaryPath = path.join(newerDir, 'summary.json');
    const summary = JSON.parse(fs.readFileSync(summaryPath, 'utf8'));
    summary.finishedAt = finishedAt;
    summary.verdict = 'failed';
    writeJson(summaryPath, summary);
    manifest.reports.summary.sha256 = sha256(summaryPath);
    writeJson(manifestPath, manifest);

    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: older.verificationRoot });
      expect(result.archiveSelection.selectedPath).to.equal(newerDir);
      expect(result.readiness.status).to.equal('fail');
      expect(result.readiness.blockingReasons).to.include('run-failed');
    } finally {
      removeDirectory(older.verificationRoot);
    }
  });

  it('rejects the non-canonical plural command field', function () {
    const fixture = makeVerification();
    const manifestPath = path.join(fixture.archiveDir, 'archive-manifest.json');
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    manifest.run.commands = manifest.run.command;
    delete manifest.run.command;
    writeJson(manifestPath, manifest);

    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.archiveSelection.status).to.equal('none');
      expect(result.archives[0].reasonCodes).to.include('missing-command');
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('blocks fallback when a newer current-revision canonical archive is invalid', function () {
    const older = makeVerification({ name: 'older-pass' });
    const newerDir = path.join(older.verificationRoot, 'newer-invalid');
    fs.cpSync(older.archiveDir, newerDir, { recursive: true });
    const manifestPath = path.join(newerDir, 'archive-manifest.json');
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    const finishedAt = new Date(Date.parse(manifest.run.finishedAt) + 60_000).toISOString();
    manifest.archivedAt = finishedAt;
    manifest.run.finishedAt = finishedAt;
    manifest.reports.summary.sha256 = '0'.repeat(64);
    writeJson(manifestPath, manifest);

    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: older.verificationRoot });
      expect(result.archiveSelection).to.include({
        status: 'invalid-latest',
        reasonCode: 'latest-canonical-archive-invalid',
        selectedPath: null,
        rejectedPath: newerDir
      });
      expect(result.readiness.status).to.equal('unknown');
      expect(result.readiness.blockingReasons).to.include('latest-canonical-archive-invalid');
      expect(result.archives.find(item => item.directory === newerDir).reasonCodes)
        .to.include('report-hash-mismatch:summary');
    } finally {
      removeDirectory(older.verificationRoot);
    }
  });

  it('ignores interrupted staging directories during canonical archive selection', function () {
    const fixture = makeVerification();
    const stagedDir = path.join(fixture.verificationRoot, '.staging', 'interrupted-run');
    fs.mkdirSync(stagedDir, { recursive: true });
    writeJson(path.join(stagedDir, 'archive-manifest.json'), {
      schema: inspector.ARCHIVE_SCHEMA,
      kind: 'automation-coverage',
      verdict: 'failed'
    });
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: fixture.verificationRoot });
      expect(result.inputs.archivesScanned).to.equal(1);
      expect(result.archiveSelection.selectedPath).to.equal(fixture.archiveDir);
      expect(result.archives.some(item => item.directory === stagedDir)).to.equal(false);
    } finally {
      removeDirectory(fixture.verificationRoot);
    }
  });

  it('fails closed when two latest canonical archives have the same timestamp', function () {
    const timestamp = new Date().toISOString();
    const first = makeVerification({
      name: 'run-a',
      manifest: {
        archivedAt: timestamp,
        run: {
          startedAt: new Date(Date.parse(timestamp) - 60_000).toISOString(),
          finishedAt: timestamp,
          command: ['node', 'run_tests.js', '--include-e2e'],
          exitCode: 0,
          strictIntegration: true
        }
      }
    });
    const secondDir = path.join(first.verificationRoot, 'run-b');
    fs.cpSync(first.archiveDir, secondDir, { recursive: true });
    try {
      const result = inspector.inspect({ projectRoot, verificationRoot: first.verificationRoot });
      expect(result.archiveSelection.status).to.equal('ambiguous');
      expect(result.readiness.status).to.equal('unknown');
      expect(result.readiness.blockingReasons).to.include('ambiguous-latest-archive');
    } finally {
      removeDirectory(first.verificationRoot);
    }
  });
});