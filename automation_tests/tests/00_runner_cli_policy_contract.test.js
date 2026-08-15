const { expect } = require('chai');

const runner = require('../run_tests');
const cliPolicy = require('../lib/runner/cli-policy');

describe('Automation runner CLI policy contract', function() {
  it('keeps the facade references identical to the extracted policy module', function() {
    expect(runner.EXIT_CODES).to.equal(cliPolicy.EXIT_CODES);
    expect(runner.parseCliArgs).to.equal(cliPolicy.parseCliArgs);
    expect(runner.parseArgs).to.equal(cliPolicy.parseArgs);
    expect(runner.getRunnerExitCode).to.equal(cliPolicy.getRunnerExitCode);
  });

  it('parses mixed arguments and comma-separated modules', function() {
    expect(cliPolicy.parseCliArgs([
      '--module', 'device, alarm', '-m', 'config', '--parallel', '--workers', '4',
      '--e2e', '--include-e2e', '--archive', '--list', '-h'
    ])).to.deep.equal({
      modules: ['device', 'alarm', 'config'],
      parallel: true,
      workers: 4,
      e2e: true,
      includeE2e: true,
      archive: true,
      list: true,
      help: true
    });
  });

  it('parses equals forms for module and workers', function() {
    expect(cliPolicy.parseCliArgs(['--module=device,alarm', '--workers=3'])).to.deep.include({
      modules: ['device', 'alarm'],
      workers: 3
    });
  });

  it('reports unknown arguments and sets help', function() {
    const originalError = console.error;
    const errors = [];
    console.error = message => errors.push(message);
    try {
      const result = cliPolicy.parseCliArgs(['--unknown']);
      expect(result.help).to.equal(true);
      expect(errors).to.deep.equal(['Unknown argument: --unknown']);
    } finally {
      console.error = originalError;
    }
  });

  it('preserves missing-value behavior', function() {
    const originalError = console.error;
    const errors = [];
    console.error = message => errors.push(message);
    try {
      const expected = {
        modules: [],
        parallel: false,
        workers: null,
        e2e: false,
        includeE2e: false,
        archive: false,
        list: false,
        help: true
      };
      expect(cliPolicy.parseCliArgs(['--module'])).to.deep.equal(expected);
      expect(cliPolicy.parseCliArgs(['--workers'])).to.deep.equal(expected);
      expect(cliPolicy.parseCliArgs(['--module', '--parallel'])).to.deep.equal({
        ...expected,
        parallel: true
      });
      expect(cliPolicy.parseCliArgs(['--workers', '--list'])).to.deep.equal({
        ...expected,
        list: true
      });
      expect(errors).to.deep.equal([
        'Unknown argument: --module',
        'Unknown argument: --workers',
        'Unknown argument: --module',
        'Unknown argument: --workers'
      ]);
    } finally {
      console.error = originalError;
    }
  });

  it('uses process.argv.slice(2) when parseArgs receives no argument', function() {
    const originalArgv = process.argv;
    process.argv = ['node', 'runner', '--workers=7'];
    try {
      expect(cliPolicy.parseArgs()).to.deep.include({ workers: 7 });
    } finally {
      process.argv = originalArgv;
    }
  });

  it('returns a failure code only when summary.failed is positive', function() {
    expect(cliPolicy.getRunnerExitCode({ failed: 0 })).to.equal(0);
    expect(cliPolicy.getRunnerExitCode({ failed: 2 })).to.equal(cliPolicy.EXIT_CODES.failed);
  });

  it('archives only explicit archive or full include-e2e runs', function() {
    expect(cliPolicy.shouldArchiveReports({ archive: true, includeE2e: false, modules: ['device'] })).to.equal(true);
    expect(cliPolicy.shouldArchiveReports({ archive: false, includeE2e: true, modules: [] })).to.equal(true);
    expect(cliPolicy.shouldArchiveReports({ archive: false, includeE2e: true, modules: ['device'] })).to.equal(false);
    expect(cliPolicy.shouldArchiveReports({ archive: false, includeE2e: false, modules: [] })).to.equal(false);
  });
});
