const { spawnSync } = require('child_process');
const path = require('path');
const { expect } = require('chai');
const runner = require('../run_tests');

const runnerPath = path.join(__dirname, '..', 'run_tests.js');

function runCli(...args) {
  return spawnSync(process.execPath, [runnerPath, ...args], {
    cwd: path.join(__dirname, '..'),
    encoding: 'utf8'
  });
}

describe('Automation runner module filter contract', function() {
  it('exits non-zero before execution when --module matches nothing', function() {
    const result = runCli('--module', 'definitely-not-a-real-module');

    expect(result.status).to.equal(runner.EXIT_CODES.failed);
    expect(result.stderr).to.include('No module matched: definitely-not-a-real-module');
    expect(result.stdout).not.to.include('AetherLink IoT automation suite');
  });

  it('still selects a known module and preserves --list as informational', function() {
    const knownModule = runner.API_MODULES[0];
    const args = runner.parseCliArgs(['--module', knownModule.key]);
    const plan = runner.buildExecutionPlan(args);
    const listResult = runCli('--list', '--module', 'definitely-not-a-real-module');

    expect(plan.apiModulesToRun.map(mod => mod.key)).to.deep.equal([knownModule.key]);
    expect(plan.e2eModulesToRun).to.deep.equal([]);
    expect(listResult.status).to.equal(0);
    expect(listResult.stdout).to.include('API modules (');
    expect(listResult.stderr).not.to.include('No module matched:');
  });
});
