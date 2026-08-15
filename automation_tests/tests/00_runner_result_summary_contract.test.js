const fs = require('fs');
const { expect } = require('chai');

const runner = require('../run_tests');
const resultSummary = require('../lib/runner/result-summary');

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
