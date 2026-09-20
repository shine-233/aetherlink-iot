/**
 * Regression coverage for the Playwright worker-restart persistence seam.
 * This validates the measurement harness only; it does not prove browser
 * business behavior.
 */
const { expect } = require('chai');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const trackerPath = path.resolve(__dirname, '../lib/page_coverage.js');
const provenance = require('../lib/coverage_provenance');

function runCoverageWorker(coverageFile, route, eventId = 'event-' + route) {
  const script = [
    `const tracker = require(${JSON.stringify(trackerPath)});`,
    `tracker.hitPage(${JSON.stringify(route)}, undefined, {`,
    "  runId: 'replacement-run',",
    "  module: 'device',",
    "  case: { file: 'e2e/02_device.spec.js', title: 'renders route', caseId: 'case-1' },",
    `  eventId: ${JSON.stringify(eventId)}`,
    '});'
  ].join('\n');
  return spawnSync(process.execPath, ['-e', script], {
    encoding: 'utf8',
    env: {
      ...process.env,
      PAGE_COVERAGE_FILE: coverageFile,
      COVERAGE_PROVENANCE_FILE: provenance.provenanceFileForCoverage(coverageFile)
    }
  });
}

function runBusinessFlowWorker(coverageFile, flowId, dimensions) {
  const script = [
    `const tracker = require(${JSON.stringify(trackerPath)});`,
    `tracker.hitBusinessFlow(${JSON.stringify(flowId)}, ${JSON.stringify(dimensions)});`
  ].join('\n');
  return spawnSync(process.execPath, ['-e', script], {
    encoding: 'utf8',
    env: {
      ...process.env,
      PAGE_COVERAGE_FILE: coverageFile,
      COVERAGE_PROVENANCE_FILE: provenance.provenanceFileForCoverage(coverageFile)
    }
  });
}

describe('Page coverage persistence contract [00_page_coverage_persistence]', function () {
  it('merges routes written by replacement Playwright workers without creating flow evidence', function () {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-page-coverage-'));
    const coverageFile = path.join(outputDir, 'page.json');

    try {
      expect(runCoverageWorker(coverageFile, '/device/manage').status).to.equal(0);
      expect(runCoverageWorker(coverageFile, '/device/grouping').status).to.equal(0);

      const payload = JSON.parse(fs.readFileSync(coverageFile, 'utf8'));
      expect(payload.pages.map(item => item.key)).to.have.members([
        '/device/manage',
        '/device/grouping'
      ]);
      expect(payload.businessFlows).to.deep.equal([]);
      expect(payload.provenance.events).to.have.length(2);
      const ledger = provenance.readLedger(provenance.provenanceFileForCoverage(coverageFile));
      expect(ledger.events).to.have.length(2);
    } finally {
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('deduplicates provenance when a replacement worker repeats a stable event ID', function () {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-page-coverage-'));
    const coverageFile = path.join(outputDir, 'page.json');

    try {
      expect(runCoverageWorker(coverageFile, '/device/manage', 'replacement-event').status).to.equal(0);
      expect(runCoverageWorker(coverageFile, '/device/grouping', 'replacement-event').status).to.equal(0);

      const payload = JSON.parse(fs.readFileSync(coverageFile, 'utf8'));
      expect(payload.provenance.events).to.have.length(1);
      expect(payload.provenance.events[0].eventId).to.equal('replacement-event');
      const ledger = provenance.readLedger(provenance.provenanceFileForCoverage(coverageFile));
      expect(ledger.events).to.have.length(1);
    } finally {
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('does not promote route renders into business-flow evidence', function () {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-page-coverage-'));
    const coverageFile = path.join(outputDir, 'page.json');

    try {
      expect(runCoverageWorker(coverageFile, '/login').status).to.equal(0);
      expect(runCoverageWorker(coverageFile, '/home').status).to.equal(0);

      const payload = JSON.parse(fs.readFileSync(coverageFile, 'utf8'));
      expect(payload.pages.map(item => item.key)).to.have.members(['/login', '/home']);
      expect(payload.businessFlows).to.deep.equal([]);
    } finally {
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('preserves business-flow dimensions across replacement workers', function () {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-page-coverage-'));
    const coverageFile = path.join(outputDir, 'page.json');
    const dimensions = {
      userAction: true,
      response: true,
      visibleResult: true
    };

    try {
      expect(runBusinessFlowWorker(coverageFile, 'auth.password-login', dimensions).status).to.equal(0);
      expect(runCoverageWorker(coverageFile, '/home').status).to.equal(0);

      const payload = JSON.parse(fs.readFileSync(coverageFile, 'utf8'));
      expect(payload.businessFlows).to.have.length(1);
      expect(payload.businessFlows[0]).to.include({
        key: 'auth.password-login',
        count: 1
      });
      expect(payload.businessFlows[0].dimensions).to.deep.equal(dimensions);
    } finally {
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('keeps report route renders separate from complete report-flow evidence', function () {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-page-coverage-'));
    const coverageFile = path.join(outputDir, 'page.json');
    const complete = {
      userAction: true,
      response: true,
      stateReadback: true,
      visibleResult: true,
      cleanup: true
    };

    try {
      expect(runCoverageWorker(coverageFile, '/visualization/report').status).to.equal(0);
      let payload = JSON.parse(fs.readFileSync(coverageFile, 'utf8'));
      expect(payload.pages.map(item => item.key)).to.include('/visualization/report');
      expect(payload.businessFlows).to.deep.equal([]);

      const incomplete = runBusinessFlowWorker(coverageFile, 'report.workspace.schedule-lifecycle', {
        ...complete,
        cleanup: false
      });
      expect(incomplete.status).to.not.equal(0);
      expect(incomplete.stderr).to.include('missing required dimensions: cleanup');

      expect(runBusinessFlowWorker(coverageFile, 'report.workspace.schedule-lifecycle', complete).status).to.equal(0);
      payload = JSON.parse(fs.readFileSync(coverageFile, 'utf8'));
      expect(payload.businessFlows).to.deep.include({
        key: 'report.workspace.schedule-lifecycle',
        count: 1,
        flow: {
          id: 'report.workspace.schedule-lifecycle',
          module: 'visualization',
          name: 'Create, edit, run, inspect, and clean up a scheduled report',
          priority: 'P1',
          pages: ['/visualization/report'],
          requiredDimensions: ['userAction', 'response', 'stateReadback', 'visibleResult', 'cleanup']
        },
        dimensions: complete
      });
    } finally {
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it('requires every declared dimension before recording a business flow', function () {
    const tracker = require(trackerPath);
    tracker.reset();

    expect(() => tracker.hitBusinessFlow('auth.password-login', {
      userAction: true,
      response: true
    })).to.throw('missing required dimensions: visibleResult');

    tracker.hitBusinessFlow('auth.password-login', {
      userAction: true,
      response: true,
      visibleResult: true
    });
    expect(tracker.getStats().businessFlows.coveredList.map(item => item.id))
      .to.deep.equal(['auth.password-login']);
    tracker.reset();
  });
});
