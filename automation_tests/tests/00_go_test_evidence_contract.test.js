'use strict';

const { expect } = require('chai');
const fs = require('fs');
const os = require('os');
const path = require('path');
const {
  GO_TEST_EVIDENCE_SCHEMA,
  evaluateGoEvidence,
  getLayer,
  getModulePackage,
  hasCompatibleDeclaredPackage,
  inspectGoFile,
  inspectGoPackage,
  validateGoEvidenceInventory
} = require('../lib/go-test-evidence');

function makeEvidence(overrides = {}) {
  return {
    evidenceId: 'go-v1-example',
    repositoryFile: 'backend/internal/example/example_test.go',
    package: 'aetherlink-iot/backend/internal/example',
    testFunction: 'TestBusinessBehavior',
    semanticAnchor: 'example.business-behavior',
    evidenceRole: 'business',
    capabilityIds: ['device-telemetry'],
    ...overrides
  };
}

function reasons(result) {
  return result.errors.map(item => item.reason);
}

function createFixture(options = {}) {
  const projectRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-go-evidence-'));
  const repositoryFile = options.repositoryFile || 'backend/internal/example/example_test.go';
  const absoluteFile = path.join(projectRoot, ...repositoryFile.split('/'));
  fs.mkdirSync(path.dirname(absoluteFile), { recursive: true });
  fs.writeFileSync(absoluteFile, options.source || [
    'package example',
    '',
    'import "testing"',
    '',
    'func TestBusinessBehavior(t *testing.T) {}',
    ''
  ].join('\n'));
  return { projectRoot, repositoryFile, absoluteFile };
}

function successfulOptions(overrides = {}) {
  return {
    listGoPackage: (_cwd, target) => ({
      status: 0,
      stdout: target === './internal/example'
        ? 'aetherlink-iot/backend/internal/example\n'
        : '',
      stderr: ''
    }),
    runInspector: () => ({
      status: 0,
      stdout: JSON.stringify({
        package: 'example',
        tests: [{
          name: 'TestBusinessBehavior',
          parameter: '*testing.T',
          exported: true
        }]
      }),
      stderr: ''
    }),
    ...overrides
  };
}

describe('Exact Go test evidence contract [00_go_test_evidence_contract]', function () {
  it('classifies only safe repository paths and package roots', function () {
    expect(getLayer(makeEvidence())).to.equal('backend');
    expect(getLayer(makeEvidence({
      repositoryFile: 'mqtt-broker/plugin/example/example_test.go'
    }))).to.equal('gmqtt');
    for (const repositoryFile of [
      '../backend/internal/example/example_test.go',
      'backend/../../outside_test.go',
      '/backend/internal/example/example_test.go',
      'C:/backend/internal/example/example_test.go',
      'other/example_test.go'
    ]) {
      expect(getLayer(makeEvidence({ repositoryFile }))).to.equal(null);
    }
    expect(getModulePackage('backend', 'aetherlink-iot/backend/internal/example')).to.equal('./internal/example');
    expect(getModulePackage('backend', 'aetherlink-iot/backend-evil/internal/example')).to.equal(null);
    expect(hasCompatibleDeclaredPackage(makeEvidence({
      repositoryFile: 'backend/mqtt/simulation_publish/simulation_publish_test.go',
      package: 'aetherlink-iot/backend/mqtt/simulation_publish'
    }), '/project', 'simulationpublish')).to.equal(true);
    expect(hasCompatibleDeclaredPackage(makeEvidence(), '/project', 'unrelated')).to.equal(false);
  });

  it('resolves exact import paths and fails closed on mismatch or command failure', function () {
    const evidence = makeEvidence();
    expect(inspectGoPackage('/project', evidence, successfulOptions())).to.deep.equal({
      resolved: true,
      error: null,
      importPath: evidence.package
    });
    expect(inspectGoPackage('/project', evidence, successfulOptions({
      listGoPackage: () => ({ status: 0, stdout: 'wrong/module\n', stderr: '' })
    }))).to.deep.equal({
      resolved: false,
      error: 'package-import-path-mismatch',
      importPath: 'wrong/module'
    });
    const failed = inspectGoPackage('/project', evidence, successfulOptions({
      listGoPackage: () => ({ status: 1, stdout: '', stderr: 'package missing' })
    }));
    expect(failed).to.include({ resolved: false, importPath: null });
    expect(failed.error).to.equal('package missing');
  });

  it('uses the real AST helper for normal, aliased, and dot testing imports', function () {
    this.timeout(30000);
    for (const [importLine, parameter] of [
      ['import "testing"', '*testing.T'],
      ['import check "testing"', '*check.T'],
      ['import . "testing"', '*T']
    ]) {
      const fixture = createFixture({
        source: [
          'package example',
          '',
          importLine,
          '',
          `func TestBusinessBehavior(t ${parameter}) {}`,
          ''
        ].join('\n')
      });
      try {
        const source = inspectGoFile(fixture.projectRoot, fixture.repositoryFile);
        expect(source).to.deep.equal({
          exists: true,
          parseError: null,
          packageName: 'example',
          tests: [{
            name: 'TestBusinessBehavior',
            parameter: '*testing.T',
            exported: true
          }]
        });
      } finally {
        fs.rmSync(fixture.projectRoot, { recursive: true, force: true });
      }
    }
  });

  it('uses the injected AST inspector result and rejects malformed output', function () {
    const fixture = createFixture();
    try {
      const source = inspectGoFile(
        fixture.projectRoot,
        fixture.repositoryFile,
        successfulOptions()
      );
      expect(source).to.deep.equal({
        exists: true,
        parseError: null,
        packageName: 'example',
        tests: [{
          name: 'TestBusinessBehavior',
          parameter: '*testing.T',
          exported: true
        }]
      });
      const malformed = inspectGoFile(
        fixture.projectRoot,
        fixture.repositoryFile,
        successfulOptions({
          runInspector: () => ({ status: 0, stdout: '{', stderr: '' })
        })
      );
      expect(malformed.exists).to.equal(true);
      expect(malformed.parseError).to.be.a('string').and.not.equal('');
      expect(inspectGoFile(fixture.projectRoot, '../escape_test.go', successfulOptions()).exists).to.equal(false);
    } finally {
      fs.rmSync(fixture.projectRoot, { recursive: true, force: true });
    }
  });

  it('accepts a complete exact declaration inventory including shared ownership', function () {
    const fixture = createFixture();
    try {
      const evidence = makeEvidence({
        capabilityIds: ['device-telemetry', 'mqtt-broker-pipeline']
      });
      const result = validateGoEvidenceInventory(
        [evidence],
        fixture.projectRoot,
        {
          capabilityIds: ['device-telemetry', 'mqtt-broker-pipeline'],
          ...successfulOptions()
        }
      );
      expect(result).to.deep.equal({ valid: true, errors: [] });
    } finally {
      fs.rmSync(fixture.projectRoot, { recursive: true, force: true });
    }
  });

  it('rejects duplicate evidence, anchor, and package/test identities', function () {
    const fixture = createFixture();
    try {
      const first = makeEvidence();
      const duplicate = makeEvidence({ capabilityIds: ['mqtt-broker-pipeline'] });
      const result = validateGoEvidenceInventory(
        [first, duplicate],
        fixture.projectRoot,
        {
          capabilityIds: ['device-telemetry', 'mqtt-broker-pipeline'],
          ...successfulOptions()
        }
      );
      expect(reasons(result)).to.include.members([
        'duplicate-evidence-id',
        'duplicate-semantic-anchor',
        'duplicate-runtime-identity'
      ]);
    } finally {
      fs.rmSync(fixture.projectRoot, { recursive: true, force: true });
    }
  });

  it('rejects unsafe files, invalid metadata, and capability ownership gaps', function () {
    const invalidItems = [
      makeEvidence({ evidenceId: '', semanticAnchor: '', repositoryFile: '../escape_test.go' }),
      makeEvidence({ evidenceId: 'role', semanticAnchor: 'role', evidenceRole: 'placeholder' }),
      makeEvidence({ evidenceId: 'name', semanticAnchor: 'name', testFunction: 'BenchmarkOnly' }),
      makeEvidence({ evidenceId: 'missing-capability', semanticAnchor: 'missing-capability', capabilityIds: [] }),
      makeEvidence({ evidenceId: 'invalid-capability', semanticAnchor: 'invalid-capability', capabilityIds: [''] }),
      makeEvidence({ evidenceId: 'duplicate-capability', semanticAnchor: 'duplicate-capability', capabilityIds: ['device-telemetry', 'device-telemetry'] }),
      makeEvidence({ evidenceId: 'unknown-capability', semanticAnchor: 'unknown-capability', capabilityIds: ['unknown'] }),
      makeEvidence({ evidenceId: 'module', semanticAnchor: 'module', package: 'aetherlink-iot/backend-evil/internal/example' }),
      makeEvidence({ evidenceId: 'suffix', semanticAnchor: 'suffix', repositoryFile: 'backend/internal/example/example.go' })
    ];
    const result = validateGoEvidenceInventory(invalidItems, '/not-used', {
      capabilityIds: ['device-telemetry'],
      ...successfulOptions()
    });
    expect(reasons(result)).to.include.members([
      'missing-evidence-id',
      'missing-semantic-anchor',
      'unsupported-repository-file',
      'invalid-evidence-role',
      'invalid-test-function-name',
      'missing-capability-ids',
      'invalid-capability-id',
      'duplicate-capability-id',
      'unknown-capability-id',
      'package-outside-module',
      'not-go-test-file'
    ]);
  });

  it('rejects package resolution, missing files, parse errors, and declaration mismatches', function () {
    const fixture = createFixture();
    try {
      const baseOptions = successfulOptions();
      const packageFailure = validateGoEvidenceInventory([makeEvidence()], fixture.projectRoot, {
        capabilityIds: ['device-telemetry'],
        ...baseOptions,
        listGoPackage: () => ({ status: 0, stdout: 'wrong/module\n', stderr: '' })
      });
      expect(reasons(packageFailure)).to.include('go-package-resolution-failed');

      const missing = validateGoEvidenceInventory([
        makeEvidence({ repositoryFile: 'backend/internal/example/missing_test.go' })
      ], fixture.projectRoot, {
        capabilityIds: ['device-telemetry'],
        ...baseOptions
      });
      expect(reasons(missing)).to.include('missing-test-file');

      const parseFailure = validateGoEvidenceInventory([makeEvidence()], fixture.projectRoot, {
        capabilityIds: ['device-telemetry'],
        ...baseOptions,
        runInspector: () => ({ status: 1, stdout: '', stderr: 'parse failed' })
      });
      expect(reasons(parseFailure)).to.include('go-ast-parse-failed');

      const mismatchOptions = expected => successfulOptions({
        runInspector: () => ({
          status: 0,
          stdout: JSON.stringify(expected),
          stderr: ''
        })
      });
      const wrongPackage = validateGoEvidenceInventory([makeEvidence()], fixture.projectRoot, {
        capabilityIds: ['device-telemetry'],
        ...mismatchOptions({ package: 'wrong', tests: [{ name: 'TestBusinessBehavior', parameter: '*testing.T', exported: true }] })
      });
      expect(reasons(wrongPackage)).to.include('package-mismatch');

      const missingFunction = validateGoEvidenceInventory([makeEvidence()], fixture.projectRoot, {
        capabilityIds: ['device-telemetry'],
        ...mismatchOptions({ package: 'example', tests: [] })
      });
      expect(reasons(missingFunction)).to.include('missing-test-function');

      const duplicateFunction = validateGoEvidenceInventory([makeEvidence()], fixture.projectRoot, {
        capabilityIds: ['device-telemetry'],
        ...mismatchOptions({ package: 'example', tests: [
          { name: 'TestBusinessBehavior', parameter: '*testing.T', exported: true },
          { name: 'TestBusinessBehavior', parameter: '*testing.T', exported: true }
        ] })
      });
      expect(reasons(duplicateFunction)).to.include('duplicate-test-function');

      const invalidSignature = validateGoEvidenceInventory([makeEvidence()], fixture.projectRoot, {
        capabilityIds: ['device-telemetry'],
        ...mismatchOptions({ package: 'example', tests: [
          { name: 'TestBusinessBehavior', parameter: '*testing.B', exported: true }
        ] })
      });
      expect(reasons(invalidSignature)).to.include('invalid-test-signature');
    } finally {
      fs.rmSync(fixture.projectRoot, { recursive: true, force: true });
    }
  });

  it('keeps declarations runtime-unknown without a current report', function () {
    const result = evaluateGoEvidence([makeEvidence()]);
    expect(result.schemaValid).to.equal(false);
    expect(result.errors).to.deep.equal([]);
    expect(result.evaluations[0]).to.include({
      runtimeOutcome: 'unknown',
      runtimePassed: false
    });
    expect(result.passed).to.equal(false);
  });

  it('accepts one exact passing outcome and rejects every ambiguous runtime shape', function () {
    const evidence = makeEvidence();
    const passingOutcome = {
      package: evidence.package,
      testFunction: evidence.testFunction,
      outcome: 'passed'
    };
    const passed = evaluateGoEvidence([evidence], {
      schema: GO_TEST_EVIDENCE_SCHEMA,
      outcomes: [passingOutcome]
    });
    expect(passed.errors).to.deep.equal([]);
    expect(passed.evaluations[0]).to.include({
      runtimeOutcome: 'passed',
      runtimePassed: true
    });
    expect(passed.passed).to.equal(true);

    const cases = [
      [{ schema: 'wrong', outcomes: [passingOutcome] }, 'invalid-runtime-schema'],
      [{ schema: GO_TEST_EVIDENCE_SCHEMA }, 'missing-runtime-outcomes'],
      [{ schema: GO_TEST_EVIDENCE_SCHEMA, outcomes: [] }, 'missing-runtime-identity'],
      [{ schema: GO_TEST_EVIDENCE_SCHEMA, outcomes: [{ ...passingOutcome, outcome: 'crashed' }] }, 'invalid-runtime-outcome'],
      [{ schema: GO_TEST_EVIDENCE_SCHEMA, outcomes: [passingOutcome, passingOutcome] }, 'duplicate-runtime-identity'],
      [{ schema: GO_TEST_EVIDENCE_SCHEMA, outcomes: [{ package: 'unexpected/package', testFunction: 'TestOther', outcome: 'passed' }] }, 'unexpected-runtime-identity'],
      [{ schema: GO_TEST_EVIDENCE_SCHEMA, outcomes: [{ package: '', testFunction: '', outcome: 'passed' }] }, 'malformed-runtime-identity']
    ];
    for (const [report, expectedReason] of cases) {
      const result = evaluateGoEvidence([evidence], report);
      expect(reasons(result)).to.include(expectedReason);
      expect(result.passed).to.equal(false);
      expect(result.evaluations[0].runtimePassed).to.equal(false);
    }
    for (const outcome of ['failed', 'skipped', 'unknown']) {
      const result = evaluateGoEvidence([evidence], {
        schema: GO_TEST_EVIDENCE_SCHEMA,
        outcomes: [{ ...passingOutcome, outcome }]
      });
      expect(result.evaluations[0]).to.include({ runtimeOutcome: outcome, runtimePassed: false });
      expect(result.passed).to.equal(false);
    }
  });

  it('rejects duplicate expected package/test identities even with one passing result', function () {
    const first = makeEvidence();
    const second = makeEvidence({
      evidenceId: 'go-v1-second',
      semanticAnchor: 'example.second'
    });
    const result = evaluateGoEvidence([first, second], {
      schema: GO_TEST_EVIDENCE_SCHEMA,
      outcomes: [{
        package: first.package,
        testFunction: first.testFunction,
        outcome: 'passed'
      }]
    });
    expect(reasons(result)).to.include('duplicate-expected-runtime-identity');
    expect(result.evaluations.every(item => !item.runtimePassed)).to.equal(true);
    expect(result.passed).to.equal(false);
  });
});
