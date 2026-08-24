const { expect } = require('chai');

const cliPolicy = require('../lib/runner/cli-policy');
const optionalExternals = require('../lib/runner/optional-externals');
const reporter = require('../lib/reporter');

const thingsVisServiceBlock = {
  reason: 'ThingsVis service is unavailable: 127.0.0.1:8000',
  category: 'runtime-external',
  seedable: false
};
const thingsVisMirrorBlock = {
  reason: 'THINGSVIS_MIRRORED_DASHBOARD_ID is not configured for the optional ThingsVis/local mirror integration',
  category: 'runtime-external',
  seedable: false
};

describe('Optional external allowlist contract', function() {
  it('partitions blocked reasons into optional-external and blocking subsets', function() {
    const partition = optionalExternals.partitionBlockedReasons([
      thingsVisServiceBlock,
      thingsVisMirrorBlock,
      {
        reason: 'requires runtime fixture or external dependency: remote broker unavailable',
        category: 'runtime-external',
        seedable: false
      },
      { reason: 'missing seeded scene fixture', category: 'seed-data', seedable: true }
    ]);

    expect(partition.optionalExternal).to.deep.equal([thingsVisServiceBlock, thingsVisMirrorBlock]);
    expect(partition.blocking.map(item => item.reason)).to.deep.equal([
      'requires runtime fixture or external dependency: remote broker unavailable',
      'missing seeded scene fixture'
    ]);
    expect(optionalExternals.isOptionalExternalBlockedReason(thingsVisServiceBlock)).to.equal(true);
    expect(optionalExternals.isOptionalExternalBlockedReason(null)).to.equal(false);
    expect(optionalExternals.isOptionalExternalBlockedReason('preview proxy is not configured'))
      .to.equal(false);
  });

  it('passes strict integration for pure optional-external skips and blocks', function() {
    const summary = {
      total: 20,
      passed: 20,
      failed: 0,
      skipped: 3,
      partialSkipped: 1,
      allSkipped: 0,
      optionalExternalSkipped: 3,
      partialSkippedOptionalExternal: 1,
      allSkippedOptionalExternal: 0,
      blockedReasons: [thingsVisServiceBlock, thingsVisMirrorBlock]
    };

    expect(cliPolicy.getStrictIntegrationGaps(summary)).to.deep.equal([]);
    expect(cliPolicy.getRunnerExitCode(summary, { strictIntegration: true })).to.equal(0);
  });

  it('still fails strict integration when non-allowlisted runtime-external blocks remain', function() {
    const summary = {
      failed: 0,
      skipped: 1,
      partialSkipped: 1,
      optionalExternalSkipped: 1,
      partialSkippedOptionalExternal: 0,
      allSkippedOptionalExternal: 0,
      blockedReasons: [
        thingsVisMirrorBlock,
        {
          reason: 'requires runtime fixture or external dependency: remote broker unavailable',
          category: 'runtime-external',
          seedable: false
        }
      ]
    };

    expect(cliPolicy.getStrictIntegrationGaps(summary)).to.deep.equal([
      'partial-skip module results are not allowed in strict integration mode',
      'runtime-external prerequisites are blocked in strict integration mode'
    ]);
    expect(cliPolicy.getRunnerExitCode(summary, { strictIntegration: true }))
      .to.equal(cliPolicy.EXIT_CODES.failed);
  });

  it('fails closed when exemption counters are absent or incomplete', function() {
    expect(cliPolicy.getRunnerExitCode({
      failed: 0,
      skipped: 3,
      partialSkipped: 1,
      blockedReasons: [thingsVisServiceBlock]
    }, { strictIntegration: true })).to.equal(cliPolicy.EXIT_CODES.failed);

    expect(cliPolicy.getRunnerExitCode({
      failed: 0,
      skipped: 3,
      optionalExternalSkipped: 2,
      blockedReasons: []
    }, { strictIntegration: true })).to.equal(cliPolicy.EXIT_CODES.failed);
  });

  it('reports optional-external counters from reporter end summaries', function() {
    reporter.results = [];
    reporter.parallel = false;
    reporter.startTime = new Date('2026-08-24T00:00:00.000Z');
    reporter.endTime = new Date('2026-08-24T00:00:01.000Z');

    reporter.record('visualization', '11_visualization.spec.js', true, '', 'e2e', 'business', {
      outcome: 'partial-skip',
      skipped: 3,
      blockedReasons: [thingsVisServiceBlock, thingsVisMirrorBlock]
    });
    reporter.record('device', '02_device.spec.js', true, '', 'e2e', 'business', {
      outcome: 'passed',
      skipped: 0,
      blockedReasons: []
    });

    const summary = reporter.end();

    expect(summary).to.include({
      skipped: 3,
      partialSkipped: 1,
      allSkipped: 0,
      optionalExternalSkipped: 3,
      partialSkippedOptionalExternal: 1,
      allSkippedOptionalExternal: 0
    });
    expect(cliPolicy.getRunnerExitCode(summary, { strictIntegration: true })).to.equal(0);
  });

  it('does not exempt results mixing allowlisted and blocking blocked reasons', function() {
    reporter.results = [];
    reporter.parallel = false;
    reporter.startTime = new Date('2026-08-24T00:00:00.000Z');
    reporter.endTime = new Date('2026-08-24T00:00:01.000Z');

    reporter.record('visualization', 'mixed-blocks.spec.js', true, '', 'e2e', 'business', {
      outcome: 'partial-skip',
      skipped: 3,
      blockedReasons: [
        thingsVisServiceBlock,
        {
          reason: 'requires runtime fixture or external dependency: remote broker unavailable',
          category: 'runtime-external',
          seedable: false
        }
      ]
    });

    const summary = reporter.end();

    expect(summary).to.include({
      skipped: 3,
      partialSkipped: 1,
      optionalExternalSkipped: 0,
      partialSkippedOptionalExternal: 0
    });
    expect(cliPolicy.getRunnerExitCode(summary, { strictIntegration: true }))
      .to.equal(cliPolicy.EXIT_CODES.failed);
  });
});
