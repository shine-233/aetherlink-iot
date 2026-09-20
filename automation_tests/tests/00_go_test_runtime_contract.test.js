'use strict';

const { expect } = require('chai');
const {
  buildGoTestGroups,
  collectGoTestEvidence,
  evaluatePackageRun,
  parseEventStream
} = require('../lib/go-test-runtime');
const { GO_TEST_EVIDENCE_SCHEMA } = require('../lib/go-test-evidence');

const backendEvidence = {
  evidenceId: 'go-v1-backend-example',
  repositoryFile: 'backend/internal/example/example_test.go',
  package: 'aetherlink-iot/backend/internal/example',
  testFunction: 'TestBusinessBehavior'
};

function event(action, testFunction = null, extra = {}) {
  return JSON.stringify({
    Time: new Date().toISOString(),
    Action: action,
    Package: backendEvidence.package,
    ...(testFunction ? { Test: testFunction } : {}),
    ...extra
  });
}

function output(lines) {
  return lines.join('\n') + '\n';
}

function successfulResult() {
  return {
    status: 0,
    stdout: output([
      event('start'),
      event('run', backendEvidence.testFunction),
      event('pass', backendEvidence.testFunction),
      event('pass')
    ]),
    stderr: ''
  };
}

function makeGroup() {
  return buildGoTestGroups([backendEvidence]).groups[0];
}

function reasons(value) {
  return value.errors.map(item => item.reason);
}

describe('Go runtime evidence contract [00_go_test_runtime_contract]', function () {
  it('builds exact uncached JSON commands per module package', function () {
    const gmqttEvidence = {
      ...backendEvidence,
      evidenceId: 'go-v1-gmqtt-example',
      repositoryFile: 'mqtt-broker/plugin/example/example_test.go',
      package: 'github.com/DrmagicE/gmqtt/plugin/example',
      testFunction: 'TestBrokerBehavior'
    };
    const grouped = buildGoTestGroups([gmqttEvidence, backendEvidence]);
    expect(grouped.errors).to.deep.equal([]);
    expect(grouped.groups).to.have.length(2);
    expect(grouped.groups.map(item => item.args)).to.deep.include.members([
      ['test', '-json', '-count=1', '-run', '^(?:TestBusinessBehavior)$', './internal/example'],
      ['test', '-json', '-count=1', '-run', '^(?:TestBrokerBehavior)$', './plugin/example']
    ]);
  });

  it('rejects evidence outside the known Go module roots', function () {
    const grouped = buildGoTestGroups([{
      ...backendEvidence,
      repositoryFile: '../outside_test.go',
      package: 'outside/module'
    }]);
    expect(grouped.groups).to.deep.equal([]);
    expect(reasons(grouped)).to.deep.equal(['invalid-go-test-target']);
  });

  it('fails closed on malformed event lines', function () {
    const parsed = parseEventStream(`${event('start')}\n{\n`);
    expect(parsed.events).to.have.length(1);
    expect(parsed.errors[0]).to.include({
      reason: 'malformed-go-test-json',
      line: 2
    });
    expect(parsed.errors[0].detail).to.be.a('string').and.not.equal('');
  });

  it('accepts one exact passing test and package terminal', function () {
    const result = evaluatePackageRun(makeGroup(), successfulResult(), {
      startedAt: '2026-09-09T00:00:00.000Z',
      finishedAt: '2026-09-09T00:00:01.000Z'
    });
    expect(result.errors).to.deep.equal([]);
    expect(result.packageOutcome).to.equal('pass');
    expect(result.outcomes).to.deep.equal([{
      evidenceId: backendEvidence.evidenceId,
      package: backendEvidence.package,
      testFunction: backendEvidence.testFunction,
      outcome: 'passed'
    }]);
  });

  it('rejects missing, duplicate, and unexpected top-level test terminals', function () {
    const variants = [
      [output([event('start'), event('pass')]), 'missing-test-terminal'],
      [output([event('pass', backendEvidence.testFunction), event('pass', backendEvidence.testFunction), event('pass')]), 'duplicate-test-terminal'],
      [output([event('run', 'TestUnexpected'), event('pass', 'TestUnexpected'), event('pass')]), 'unexpected-test-identity']
    ];
    for (const [stdout, reason] of variants) {
      const result = evaluatePackageRun(makeGroup(), { status: 0, stdout, stderr: '' }, {
        startedAt: '2026-09-09T00:00:00.000Z',
        finishedAt: '2026-09-09T00:00:01.000Z'
      });
      expect(reasons(result)).to.include(reason);
    }
  });

  it('rejects package failures, duplicate package terminals, and command failure', function () {
    const variants = [
      [{ status: 1, stdout: output([event('fail', backendEvidence.testFunction), event('fail')]), stderr: '' }, ['package-failed', 'go-test-command-failed']],
      [{ status: 0, stdout: output([event('pass', backendEvidence.testFunction), event('pass'), event('pass')]), stderr: '' }, ['duplicate-package-terminal']],
      [{ status: 1, stdout: '', stderr: 'process failed' }, ['go-test-command-failed', 'missing-package-terminal']]
    ];
    for (const [execution, expected] of variants) {
      const result = evaluatePackageRun(makeGroup(), execution, {
        startedAt: '2026-09-09T00:00:00.000Z',
        finishedAt: '2026-09-09T00:00:01.000Z'
      });
      expect(reasons(result)).to.include.members(expected);
    }
  });

  it('rejects setup, build, cached, and foreign-package output', function () {
    const stdout = output([
      event('run', backendEvidence.testFunction),
      event('pass', backendEvidence.testFunction, { Output: 'ok (cached)' }),
      event('pass'),
      JSON.stringify({ Action: 'fail', Package: 'foreign/module', Output: '[setup failed]' }),
      JSON.stringify({ Action: 'output', Package: backendEvidence.package, Output: '[build failed]' })
    ]);
    const result = evaluatePackageRun(makeGroup(), { status: 1, stdout, stderr: '' }, {
      startedAt: '2026-09-09T00:00:00.000Z',
      finishedAt: '2026-09-09T00:00:01.000Z'
    });
    expect(reasons(result)).to.include.members([
      'cached-go-test-result',
      'unexpected-package-event',
      'go-test-setup-failed',
      'go-test-build-failed'
    ]);
  });

  it('emits revision-bound exact outcomes and preserves dirty provenance', function () {
    let invocation;
    const source = {
      gitCommit: 'a'.repeat(40),
      gitRef: 'main',
      dirty: true,
      diffHash: 'b'.repeat(64)
    };
    const report = collectGoTestEvidence({
      projectRoot: 'C:/project',
      evidenceItems: [backendEvidence],
      startedAt: '2026-09-09T00:00:00.000Z',
      finishedAt: '2026-09-09T00:00:02.000Z',
      source,
      runGoTest: (cwd, args) => {
        invocation = { cwd, args };
        return successfulResult();
      }
    });
    expect(report).to.include({
      schema: GO_TEST_EVIDENCE_SCHEMA,
      revision: source.gitCommit,
      startedAt: '2026-09-09T00:00:00.000Z',
      finishedAt: '2026-09-09T00:00:02.000Z',
      passed: true
    });
    expect(report.source).to.deep.equal(source);
    expect(invocation.args).to.deep.equal([
      'test', '-json', '-count=1', '-run', '^(?:TestBusinessBehavior)$', './internal/example'
    ]);
    expect(invocation.cwd.replace(/\\/g, '/')).to.equal('C:/project/backend');
  });
});
