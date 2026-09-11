'use strict';

const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const {
  GO_TEST_EVIDENCE_SCHEMA,
  getLayer,
  getModulePackage
} = require('./go-test-evidence');

const TERMINAL_TEST_ACTIONS = new Set(['pass', 'fail', 'skip']);
const TERMINAL_PACKAGE_ACTIONS = new Set(['pass', 'fail', 'skip']);

function escapeRegex(value) {
  return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function buildGoTestGroups(evidenceItems) {
  const groups = new Map();
  const errors = [];
  for (const evidence of evidenceItems || []) {
    const layer = getLayer(evidence);
    const target = getModulePackage(layer, evidence && evidence.package);
    if (!layer || !target) {
      errors.push({
        evidenceId: evidence && evidence.evidenceId || null,
        reason: 'invalid-go-test-target'
      });
      continue;
    }
    const key = `${layer}:${evidence.package}`;
    const group = groups.get(key) || {
      layer,
      package: evidence.package,
      target,
      evidence: []
    };
    group.evidence.push(evidence);
    groups.set(key, group);
  }
  for (const group of groups.values()) {
    const names = group.evidence.map(item => escapeRegex(item.testFunction)).sort();
    group.args = ['test', '-json', '-count=1', '-run', `^(?:${names.join('|')})$`, group.target];
  }
  return { groups: [...groups.values()], errors };
}

function parseEventStream(stdout) {
  const events = [];
  const errors = [];
  for (const [index, line] of String(stdout || '').split(/\r?\n/).entries()) {
    if (!line.trim()) continue;
    try {
      const event = JSON.parse(line);
      if (!event || typeof event !== 'object' || Array.isArray(event)) throw new Error('event is not an object');
      events.push(event);
    } catch (error) {
      errors.push({ reason: 'malformed-go-test-json', line: index + 1, detail: error.message });
    }
  }
  return { events, errors };
}

function evaluatePackageRun(group, result, interval) {
  const parsed = parseEventStream(result.stdout);
  const errors = [...parsed.errors];
  const expected = new Map(group.evidence.map(item => [item.testFunction, item]));
  const terminals = new Map();
  const packageTerminals = [];
  const topLevelStarted = new Set();
  for (const event of parsed.events) {
    if (event.Package && event.Package !== group.package) {
      errors.push({ reason: 'unexpected-package-event', package: event.Package });
    }
    if (event.Cached === true || /\(cached\)/.test(String(event.Output || ''))) {
      errors.push({ reason: 'cached-go-test-result' });
    }
    if (event.Test && !event.Test.includes('/')) {
      if (event.Action === 'run') topLevelStarted.add(event.Test);
      if (TERMINAL_TEST_ACTIONS.has(event.Action)) {
        const terminalGroup = terminals.get(event.Test) || [];
        terminalGroup.push(event);
        terminals.set(event.Test, terminalGroup);
      }
    } else if (!event.Test && TERMINAL_PACKAGE_ACTIONS.has(event.Action)) {
      packageTerminals.push(event);
    }
  }
  for (const testFunction of new Set([...topLevelStarted, ...terminals.keys()])) {
    if (!expected.has(testFunction)) {
      errors.push({ reason: 'unexpected-test-identity', testFunction });
    }
  }
  if (result.error || result.status !== 0) {
    errors.push({ reason: 'go-test-command-failed', detail: String(result.stderr || result.error?.message || '').trim() });
  }
  if (packageTerminals.length === 0) errors.push({ reason: 'missing-package-terminal' });
  if (packageTerminals.length > 1) errors.push({ reason: 'duplicate-package-terminal' });
  if (packageTerminals.length === 1 && packageTerminals[0].Action !== 'pass') {
    errors.push({ reason: packageTerminals[0].Action === 'skip' ? 'package-skipped' : 'package-failed' });
  }
  const output = `${result.stdout || ''}\n${result.stderr || ''}`;
  if (/\[setup failed\]|setup failed/i.test(output)) errors.push({ reason: 'go-test-setup-failed' });
  if (/\[build failed\]|build failed/i.test(output)) errors.push({ reason: 'go-test-build-failed' });
  const outcomes = group.evidence.map(evidence => {
    const terminalGroup = terminals.get(evidence.testFunction) || [];
    if (terminalGroup.length === 0) errors.push({ reason: 'missing-test-terminal', testFunction: evidence.testFunction });
    if (terminalGroup.length > 1) errors.push({ reason: 'duplicate-test-terminal', testFunction: evidence.testFunction });
    const action = terminalGroup.length === 1 ? terminalGroup[0].Action : 'unknown';
    return {
      evidenceId: evidence.evidenceId,
      package: evidence.package,
      testFunction: evidence.testFunction,
      outcome: action === 'pass' ? 'passed' : action === 'fail' ? 'failed' : action === 'skip' ? 'skipped' : 'unknown'
    };
  });
  return {
    package: group.package,
    layer: group.layer,
    command: ['go', ...group.args],
    startedAt: interval.startedAt,
    finishedAt: interval.finishedAt,
    exitCode: Number.isInteger(result.status) ? result.status : null,
    packageOutcome: packageTerminals.length === 1 ? packageTerminals[0].Action : 'unknown',
    outcomes,
    errors
  };
}

function collectGoTestEvidence(options) {
  const grouped = buildGoTestGroups(options.evidenceItems);
  const packageResults = [];
  const runGoTest = options.runGoTest || ((cwd, args) => spawnSync('go', args, {
    cwd,
    encoding: 'utf8',
    windowsHide: true,
    maxBuffer: 64 * 1024 * 1024
  }));
  for (const group of grouped.groups) {
    const moduleRoot = path.join(options.projectRoot, group.layer === 'backend' ? 'backend' : 'mqtt-broker');
    const startedAt = new Date();
    const result = runGoTest(moduleRoot, group.args, group);
    packageResults.push(evaluatePackageRun(group, result, {
      startedAt: startedAt.toISOString(),
      finishedAt: new Date().toISOString()
    }));
  }
  const outcomes = packageResults.flatMap(item => item.outcomes);
  const errors = [
    ...grouped.errors,
    ...packageResults.flatMap(item => item.errors.map(error => ({ package: item.package, ...error })))
  ];
  const source = options.source && typeof options.source === 'object'
    ? {
        gitCommit: options.source.gitCommit || null,
        gitRef: options.source.gitRef || null,
        dirty: options.source.dirty === true,
        diffHash: options.source.diffHash || null
      }
    : null;
  return {
    schema: GO_TEST_EVIDENCE_SCHEMA,
    revision: source && source.gitCommit,
    startedAt: options.startedAt,
    finishedAt: options.finishedAt || new Date().toISOString(),
    source,
    commandPolicy: { json: true, count: 1, cacheDisabled: true },
    packages: packageResults,
    outcomes,
    errors,
    passed: outcomes.length > 0 && outcomes.every(item => item.outcome === 'passed') && errors.length === 0
  };
}

function writeGoTestEvidenceReport(reportDir, report) {
  const reportPath = path.join(reportDir, 'go-test-evidence.json');
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2), 'utf8');
  return reportPath;
}

module.exports = {
  buildGoTestGroups,
  collectGoTestEvidence,
  evaluatePackageRun,
  parseEventStream,
  writeGoTestEvidenceReport
};
