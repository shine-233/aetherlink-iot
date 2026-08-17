// CI contract: keep the required gate local, least-privileged, and immutable.
const fs = require('fs');
const path = require('path');
const { expect } = require('chai');

const workflowPath = path.resolve(__dirname, '..', '..', '.github', 'workflows', 'minimum-quality-gate.yml');

function workflow() {
  return fs.readFileSync(workflowPath, 'utf8').replace(/\r\n/g, '\n');
}

describe('minimum CI quality gate contract [00_minimum_quality_gate_contract]', function () {
  it('runs for pull requests, pushes, and manual verification', function () {
    const source = workflow();
    expect(source).to.match(/^on:\n  pull_request:\n  push:\n  workflow_dispatch:/m);
  });

  it('grants only read access to repository contents', function () {
    const source = workflow();
    expect(source).to.match(/^permissions:\n  contents: read$/m);
    expect(source).not.to.match(/^\s+[\w-]+: write$/m);
  });

  it('pins checkout and the Node runtime before executing the offline gate', function () {
    const source = workflow();
    expect(source).to.match(/uses: actions\/checkout@[0-9a-f]{40}(?:\s+#.*)?$/m);
    expect(source).to.match(/uses: actions\/setup-node@[0-9a-f]{40}(?:\s+#.*)?$/m);
    expect(source).to.include('node-version: 22');
    expect(source).to.include('run: node automation_tests/scripts/release_preflight.js');
    expect(source).not.to.include('pull_request_target');
    expect(source).not.to.match(/\bsecrets\./);
  });

  it('bounds execution and cancels stale runs', function () {
    const source = workflow();
    expect(source).to.include('timeout-minutes: 10');
    expect(source).to.include('cancel-in-progress: true');
  });
});
