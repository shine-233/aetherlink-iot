'use strict';

const path = require('path');
const inspector = require('../lib/coverage_inspector');

function requireValue(argv, index, argument) {
  const value = argv[index + 1];
  if (typeof value !== 'string' || !value.trim() || value.startsWith('--')) {
    throw new Error(argument + ' requires a value');
  }
  return value;
}

function parseArgs(argv) {
  const options = {};
  for (let index = 0; index < argv.length; index++) {
    const argument = argv[index];
    if (argument === '--project-root') {
      options.projectRoot = path.resolve(requireValue(argv, index, argument));
      index++;
    } else if (argument === '--verification-root') {
      options.verificationRoot = path.resolve(requireValue(argv, index, argument));
      index++;
    } else if (argument === '--max-age-hours') {
      const hours = Number(requireValue(argv, index, argument));
      index++;
      if (!Number.isFinite(hours) || hours <= 0) throw new Error('--max-age-hours must be positive');
      options.maxAgeMs = hours * 60 * 60 * 1000;
    } else if (argument === '--require-evidence-kind') {
      options.requiredEvidenceKind = requireValue(argv, index, argument);
      index++;
    } else if (argument === '--require-clean-worktree') {
      options.requireCleanCurrentWorktree = true;
    } else {
      throw new Error('Unknown argument: ' + argument);
    }
  }
  return options;
}

let result;
try {
  const options = parseArgs(process.argv.slice(2));
  if (
    options.requiredEvidenceKind &&
    !['synthetic', 'local-real', 'real-device', 'target-deploy'].includes(options.requiredEvidenceKind)
  ) {
    throw new Error('--require-evidence-kind is invalid');
  }
  result = inspector.inspect(options);
} catch (error) {
  result = {
    schema: inspector.INSPECTOR_SCHEMA,
    generatedAt: new Date().toISOString(),
    project: null,
    inputs: null,
    archiveSelection: { status: 'invalid', reasonCode: 'invalid-cli-input', selectedPath: null },
    archives: [],
    metrics: null,
    checks: [{
      id: 'cli-input',
      status: 'fail',
      severity: 'blocking',
      reasonCode: 'invalid-cli-input',
      message: error.message,
      evidenceRefs: []
    }],
    readiness: { status: 'fail', blockingReasons: ['invalid-cli-input'] }
  };
}

process.stdout.write(JSON.stringify(result, null, 2) + '\n');
process.exitCode = inspector.exitCode(result);
