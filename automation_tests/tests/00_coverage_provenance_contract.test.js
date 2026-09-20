const { expect } = require('chai');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const provenance = require('../lib/coverage_provenance');

function candidate(overrides = {}) {
  return provenance.createEvent({
    eventId: overrides.eventId || '11111111-1111-4111-8111-111111111111',
    runId: 'run-1',
    module: 'device',
    kind: 'endpoint',
    target: 'GET /api/v1/device',
    case: {
      file: 'tests/02_device.test.js',
      title: 'lists the seeded device',
      caseId: 'device-list'
    },
    attempt: { retry: 0, responseReceived: true },
    outcome: 'pending',
    statusCode: 200,
    observedAt: '2026-09-08T00:00:00.000Z',
    disposition: 'candidate',
    ...overrides
  });
}

describe('Coverage provenance contract [00_coverage_provenance_contract]', function () {
  it('creates the versioned endpoint/page hit event contract', function () {
    const event = candidate();
    expect(event).to.include({
      schema: 'aetherlink.coverage.hit.v1',
      eventId: '11111111-1111-4111-8111-111111111111',
      runId: 'run-1',
      module: 'device',
      kind: 'endpoint',
      target: 'GET /api/v1/device',
      outcome: 'pending',
      statusCode: 200,
      disposition: 'candidate'
    });
    expect(event.case).to.include({
      file: 'tests/02_device.test.js',
      title: 'lists the seeded device',
      caseId: 'device-list',
      attributed: true
    });
    expect(event.attempt).to.include({
      retry: 0,
      responseReceived: true,
      transportFailure: false
    });
    expect(provenance.validateEvent(event)).to.equal(true);
  });

  it('keeps suite hooks and unattributed setup hits diagnostic', function () {
    for (const event of [
      candidate({ case: { file: 'tests/02_device.test.js', hook: 'before all' } }),
      candidate({ case: {} })
    ]) {
      expect(event.case.attributed).to.equal(false);
      expect(event.disposition).to.equal('diagnostic');
    }
  });

  it('never qualifies transport failures or responses without an HTTP status', function () {
    const events = [
      candidate({
        eventId: 'transport-failure',
        statusCode: null,
        attempt: { transportFailure: true, responseReceived: false, errorCode: 'ECONNREFUSED' }
      }),
      candidate({ eventId: 'missing-status', statusCode: null })
    ];
    const qualified = provenance.qualifyEvents(events, {
      runId: 'run-1',
      module: 'device',
      modulePassed: true,
      moduleOutcome: 'passed',
      cases: [{ caseId: 'device-list', outcome: 'passed' }]
    });

    expect(qualified.every(event => event.disposition === 'diagnostic')).to.equal(true);
  });

  it('qualifies only an authoritative passed case in a passed module', function () {
    const events = [
      candidate({ eventId: 'passed-case' }),
      candidate({
        eventId: 'failed-case',
        case: { file: 'tests/02_device.test.js', title: 'fails', caseId: 'failed' }
      }),
      candidate({
        eventId: 'skipped-case',
        case: { file: 'tests/02_device.test.js', title: 'skips', caseId: 'skipped' }
      }),
      candidate({
        eventId: 'flaky-case',
        case: { file: 'tests/02_device.test.js', title: 'flakes', caseId: 'flaky' }
      }),
      candidate({
        eventId: 'mismatched-case',
        case: { file: 'tests/02_device.test.js', title: 'mismatch', caseId: 'mismatch' }
      })
    ];
    const qualified = provenance.qualifyEvents(events, {
      runId: 'run-1',
      module: 'device',
      modulePassed: true,
      moduleOutcome: 'passed',
      cases: [
        { caseId: 'device-list', outcome: 'passed' },
        { caseId: 'failed', outcome: 'failed' },
        { caseId: 'skipped', outcome: 'skipped' },
        { caseId: 'flaky', outcome: 'flaky' },
        { caseId: 'mismatch', outcome: 'passed', mismatched: true }
      ]
    });

    expect(qualified.find(event => event.eventId === 'passed-case').disposition).to.equal('effective');
    expect(qualified.filter(event => event.eventId !== 'passed-case').every(event => (
      event.disposition === 'diagnostic'
    ))).to.equal(true);
  });

  it('uses exact file and title path and rejects ambiguous title fallback', function () {
    const events = [
      candidate({
        eventId: 'exact-path',
        case: {
          file: 'tests\\duplicate.test.js',
          title: 'same title',
          titlePath: ['suite B', 'same title']
        }
      }),
      candidate({
        eventId: 'ambiguous-title',
        case: { file: 'tests/duplicate.test.js', title: 'same title' }
      })
    ];
    const qualified = provenance.qualifyEvents(events, {
      runId: 'run-1',
      module: 'device',
      modulePassed: true,
      moduleOutcome: 'passed',
      cases: [
        {
          file: 'tests/duplicate.test.js',
          title: 'same title',
          titlePath: ['suite A', 'same title'],
          outcome: 'failed'
        },
        {
          file: 'tests/duplicate.test.js',
          title: 'same title',
          titlePath: ['suite B', 'same title'],
          outcome: 'passed'
        }
      ]
    });

    expect(qualified.find(event => event.eventId === 'exact-path').disposition).to.equal('effective');
    expect(qualified.find(event => event.eventId === 'ambiguous-title').disposition).to.equal('diagnostic');
  });

  it('does not fall back when an authoritative caseId is ambiguous', function () {
    const [qualified] = provenance.qualifyEvents([
      candidate({
        eventId: 'ambiguous-id',
        case: {
          file: 'tests/duplicate.test.js',
          title: 'same title',
          titlePath: ['suite B', 'same title'],
          caseId: 'duplicate-id'
        }
      })
    ], {
      runId: 'run-1',
      module: 'device',
      modulePassed: true,
      moduleOutcome: 'passed',
      cases: [
        {
          caseId: 'duplicate-id',
          file: 'tests/duplicate.test.js',
          title: 'same title',
          titlePath: ['suite A', 'same title'],
          outcome: 'failed'
        },
        {
          caseId: 'duplicate-id',
          file: 'tests/duplicate.test.js',
          title: 'same title',
          titlePath: ['suite B', 'same title'],
          outcome: 'passed'
        }
      ]
    });

    expect(qualified.disposition).to.equal('diagnostic');
  });

  it('demotes events outside the authoritative run or module scope', function () {
    const events = [
      candidate({ eventId: 'wrong-run', runId: 'other-run' }),
      candidate({ eventId: 'wrong-module', module: 'alarm' })
    ];
    const qualified = provenance.qualifyEvents(events, {
      runId: 'run-1',
      module: 'device',
      modulePassed: true,
      moduleOutcome: 'passed',
      cases: [{ caseId: 'device-list', outcome: 'passed' }]
    });

    expect(qualified.every(event => event.disposition === 'diagnostic')).to.equal(true);
    expect(qualified.every(event => event.diagnostics.qualificationScopeMatched === false)).to.equal(true);
  });

  it('revokes all candidates when the authoritative module is partial or failed', function () {
    for (const moduleOutcome of ['failed', 'partial-skip', 'all-skipped']) {
      const [event] = provenance.qualifyEvents([candidate()], {
        runId: 'run-1',
        module: 'device',
        modulePassed: moduleOutcome === 'partial-skip',
        moduleOutcome,
        cases: [{ caseId: 'device-list', outcome: 'passed' }]
      });
      expect(event.disposition, moduleOutcome).to.equal('diagnostic');
    }
  });

  it('deduplicates replacement-worker events by stable eventId', function () {
    const initial = candidate({ eventId: 'stable-id' });
    const replacement = { ...initial, disposition: 'effective', outcome: 'passed' };
    const merged = provenance.mergeEvents([initial], [replacement]);

    expect(merged).to.have.length(1);
    expect(merged[0]).to.include({
      eventId: 'stable-id',
      disposition: 'effective',
      outcome: 'passed'
    });
  });

  it('attributes API hits to the exact active Mocha case', function () {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-provenance-'));
    const testFile = path.join(outputDir, 'fixture.test.js');
    const coverageFile = path.join(outputDir, 'coverage.json');
    const provenanceFile = provenance.provenanceFileForCoverage(coverageFile);
    const reporterPath = path.resolve(__dirname, '../lib/coverage_mocha_reporter.js');
    const endpointPath = path.resolve(__dirname, '../lib/endpoint_coverage.js');
    fs.writeFileSync(testFile, [
      `const coverage = require(${JSON.stringify(endpointPath)});`,
      "describe('fixture suite', function () {",
      "  it('exact case', function () {",
      "    coverage.hit('GET', 'http://127.0.0.1:9999/api/v1/device', { statusCode: 200 });",
      '  });',
      '});'
    ].join('\n'));

    try {
      const mochaCli = require.resolve('mocha/bin/mocha.js');
      const result = spawnSync(process.execPath, [
        mochaCli,
        testFile,
        '--reporter', reporterPath,
        '--reporter-options', `reportDir=${outputDir},reportFilename=fixture,overwrite=true,quiet=true`
      ], {
        encoding: 'utf8',
        env: {
          ...process.env,
          ENDPOINT_COVERAGE_FILE: coverageFile,
          COVERAGE_PROVENANCE_FILE: provenanceFile,
          AETHERLINK_COVERAGE_RUN_ID: 'mocha-run',
          AETHERLINK_COVERAGE_MODULE: 'fixture-module'
        }
      });
      expect(result.status, result.stderr || result.stdout).to.equal(0);
      const ledger = provenance.readLedger(provenanceFile);
      expect(ledger.events).to.have.length(1);
      expect(ledger.events[0].case).to.include({
        title: 'exact case',
        attributed: true
      });
      expect(ledger.events[0].case.titlePath).to.deep.equal(['fixture suite exact case']);
      expect(ledger.events[0]).to.include({
        runId: 'mocha-run',
        module: 'fixture-module',
        statusCode: 200,
        disposition: 'candidate'
      });
    } finally {
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });
});
