/**
 * Release preflight contract: the default gate stays offline and testable
 * without executing the repository's full checks.
 */
const { expect } = require('chai');
const {
  createLocalChecks,
  runReleasePreflight
} = require('../scripts/release_preflight');

describe('release preflight contract [00_release_preflight_contract]', function () {
  it('passes all injected local checks without running real scripts', function () {
    const calls = [];
    const result = runReleasePreflight({
      projectRoot: process.cwd(),
      runner(command, args, options) {
        calls.push({ command, args: [...args], options: { ...options } });
        return { status: 0, stdout: 'fixture pass', stderr: '' };
      }
    });

    expect(result.kind).to.equal('aetherlink-release-preflight-local');
    expect(result.ok).to.equal(true);
    expect(result.checks).to.have.length(10);
    expect(result.checks.every(check => check.status === 'pass')).to.equal(true);
    expect(calls).to.have.length(10);
  });

  it('fails when a required local check fails', function () {
    const result = runReleasePreflight({
      checks: [{ id: 'fixture-required', mode: 'local-default', command: 'fixture', args: [] }],
      runner() {
        return { status: 3, stdout: 'partial output', stderr: 'contract failed' };
      }
    });

    expect(result.ok).to.equal(false);
    expect(result.checks[0]).to.include({ id: 'fixture-required', mode: 'local-default', status: 'fail', exitCode: 3 });
    expect(result.checks[0].stderr).to.include('contract failed');
  });

  it('does not count external not-run items as failures or passes', function () {
    const result = runReleasePreflight({
      checks: [{ id: 'fixture-required', mode: 'local-default', command: 'fixture', args: [] }],
      runner: () => ({ status: 0 })
    });

    expect(result.ok).to.equal(true);
    expect(result.external.map(item => [item.id, item.mode, item.status])).to.deep.equal([
      ['runtime-api-e2e', 'blocked-external', 'not-run'],
      ['vulnerability-database', 'optional-external', 'not-run'],
      ['sbom-generation', 'optional-external', 'not-run'],
      ['hosted-dependency-review', 'blocked-external', 'not-run']
    ]);
  });

  it('includes both local scripts and the eight deploy contracts', function () {
    const checks = createLocalChecks();
    expect(checks.map(check => check.id)).to.deep.equal([
      'supply-chain',
      'generated-artifacts',
      'deploy:optional-integrations-contract',
      'deploy:docker-build-context-contract',
      'deploy:package-source-boundary-contract',
      'deploy:backend-readiness-contract',
      'deploy:redis-memory-contract',
      'deploy:container-runtime-security-contract',
      'deploy:network-segmentation-contract',
      'deploy:backup-restore-contract'
    ]);
    expect(checks.slice(2).every(check => check.command === 'sh')).to.equal(true);
  });

  it('does not change cwd or mutate caller-provided check arguments', function () {
    const cwdBefore = process.cwd();
    const args = ['fixture-argument'];
    const checks = [{ id: 'fixture', mode: 'local-default', command: 'fixture', args }];
    runReleasePreflight({
      projectRoot: '.',
      checks,
      runner: () => ({ status: 0 })
    });

    expect(process.cwd()).to.equal(cwdBefore);
    expect(args).to.deep.equal(['fixture-argument']);
    expect(checks[0].args).to.equal(args);
  });
});
