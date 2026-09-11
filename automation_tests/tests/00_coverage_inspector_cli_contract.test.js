'use strict';

const { expect } = require('chai');
const { spawnSync } = require('child_process');
const path = require('path');

const script = path.resolve(__dirname, '..', 'scripts', 'inspect_coverage_evidence.js');
const projectRoot = path.resolve(__dirname, '..', '..');

function run(args) {
  return spawnSync(process.execPath, [script, ...args], {
    cwd: projectRoot,
    encoding: 'utf8'
  });
}

function parseResult(result) {
  expect(result.stdout, result.stderr).to.not.equal('');
  return JSON.parse(result.stdout);
}

describe('Coverage inspector CLI contract [00_coverage_inspector_cli_contract]', function () {
  it('fails closed with structured JSON for unknown arguments', function () {
    const result = run(['--unknown']);
    const output = parseResult(result);

    expect(result.status).to.equal(1);
    expect(output.schema).to.equal('aetherlink.coverage.inspector.v1');
    expect(output.metrics).to.equal(null);
    expect(output.archiveSelection).to.include({
      status: 'invalid',
      reasonCode: 'invalid-cli-input',
      selectedPath: null
    });
    expect(output.readiness).to.deep.equal({
      status: 'fail',
      blockingReasons: ['invalid-cli-input']
    });
  });

  it('fails closed for invalid evidence kinds and max ages', function () {
    const variants = [
      ['--require-evidence-kind', 'imaginary'],
      ['--max-age-hours', '0'],
      ['--max-age-hours', 'not-a-number']
    ];

    for (const args of variants) {
      const result = run(args);
      const output = parseResult(result);
      expect(result.status).to.equal(1);
      expect(output.readiness.status).to.equal('fail');
      expect(output.readiness.blockingReasons).to.deep.equal(['invalid-cli-input']);
      expect(output.metrics).to.equal(null);
    }
  });

  it('fails closed when value-taking arguments omit their values', function () {
    const variants = [
      ['--project-root'],
      ['--verification-root'],
      ['--max-age-hours'],
      ['--require-evidence-kind'],
      ['--project-root', '--require-clean-worktree']
    ];

    for (const args of variants) {
      const result = run(args);
      const output = parseResult(result);
      expect(result.status).to.equal(1);
      expect(output.archiveSelection).to.include({
        status: 'invalid',
        reasonCode: 'invalid-cli-input',
        selectedPath: null
      });
      expect(output.metrics).to.equal(null);
      expect(output.readiness.blockingReasons).to.deep.equal(['invalid-cli-input']);
    }
  });
});
