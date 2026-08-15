const fs = require('fs');

const coverageContract = require('../coverage_contract');

function readJsonIfPresent(filePath) {
  if (!filePath || !fs.existsSync(filePath)) {
    return null;
  }
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch (_error) {
    return null;
  }
}

function extractBlockedReasons(...texts) {
  const structuredReasons = [];
  const legacyReasons = [];
  texts.forEach(text => {
    String(text || '')
      .split(/\r?\n/)
      .forEach(line => {
        const metadataMatch = line.match(/integration-blocked-meta:\s*(\{.*\})\s*$/i);
        if (metadataMatch && metadataMatch[1]) {
          try {
            structuredReasons.push(
              coverageContract.classifyBlockedReason(JSON.parse(metadataMatch[1]))
            );
            return;
          } catch (_error) {
            // Fall through to the legacy text parser for malformed metadata.
          }
        }
        const textMatch = line.match(/integration-blocked:\s*(.+)$/i);
        if (textMatch && textMatch[1]) {
          legacyReasons.push(coverageContract.classifyBlockedReason(textMatch[1]));
        }
      });
  });

  if (structuredReasons.length === 0) {
    return legacyReasons;
  }

  const structuredReasonsByText = new Set(structuredReasons.map(item => item.reason));
  return structuredReasons.concat(
    legacyReasons.filter(item => !structuredReasonsByText.has(item.reason))
  );
}

function hasOnlyRuntimeExternalBlocks(blockedReasons) {
  return Array.isArray(blockedReasons) &&
    blockedReasons.length > 0 &&
    blockedReasons.every(item => item && item.category === 'runtime-external' && item.seedable === false);
}

function createRunnerSummary({
  passed,
  outcome,
  skipped = 0,
  blockedReasons = [],
  reason = ''
}) {
  return {
    passed: Boolean(passed),
    outcome: outcome || (passed ? 'passed' : 'failed'),
    skipped: Number(skipped || 0),
    blockedReasons: Array.isArray(blockedReasons) ? blockedReasons : [],
    reason: reason || ''
  };
}

function summarizeMochaResult(result, explicitReport) {
  const report = explicitReport || readJsonIfPresent(result.reportJson);
  const stats = report && report.stats ? report.stats : null;
  const defaultReason = result.stderr || result.stdout || 'exit code ' + result.code;
  const blockedReasons = extractBlockedReasons(result.stdout, result.stderr);

  if (!stats) {
    return createRunnerSummary({
      passed: result.code === 0,
      outcome: result.code === 0 ? 'passed' : 'failed',
      skipped: 0,
      blockedReasons,
      reason: result.code === 0 ? '' : defaultReason
    });
  }

  const tests = Number(stats.tests || 0);
  const passes = Number(stats.passes || 0);
  const pending = Number(stats.pending || 0) + Number(stats.skipped || 0);
  const failures = Number(stats.failures || 0);
  const allSkipped = tests > 0 && passes === 0 && failures === 0 && pending >= tests;

  if (result.code !== 0 || failures > 0) {
    return createRunnerSummary({
      passed: false,
      outcome: 'failed',
      skipped: pending,
      blockedReasons,
      reason: defaultReason
    });
  }
  if (allSkipped) {
    const runtimeExternalOnly = hasOnlyRuntimeExternalBlocks(blockedReasons);
    return createRunnerSummary({
      // A structured runtime-external skip is an honest partial result, not a
      // failed assertion. Keep unannotated/all-seedable skips failing so a
      // silent fake test cannot hide behind the same branch.
      passed: runtimeExternalOnly,
      outcome: runtimeExternalOnly ? 'partial-skip' : 'all-skipped',
      skipped: pending,
      blockedReasons,
      reason: runtimeExternalOnly
        ? 'all tests skipped because a declared runtime-external prerequisite was unavailable'
        : 'all tests skipped; environment/data preconditions were not satisfied'
    });
  }
  return createRunnerSummary({
    passed: true,
    outcome: pending > 0 ? 'partial-skip' : 'passed',
    skipped: pending,
    blockedReasons,
    reason: pending > 0 ? `skipped ${pending}/${tests}; check fixture readiness if this was unexpected` : ''
  });
}

function summarizePlaywrightResult(result, explicitReport) {
  const report = explicitReport || readJsonIfPresent(result.reportJson);
  const stats = report && report.stats ? report.stats : null;
  const defaultReason = result.stderr || result.stdout || 'exit code ' + result.code;
  const blockedReasons = extractBlockedReasons(result.stdout, result.stderr);

  if (!stats) {
    return createRunnerSummary({
      passed: result.code === 0,
      outcome: result.code === 0 ? 'passed' : 'failed',
      skipped: 0,
      blockedReasons,
      reason: result.code === 0 ? '' : defaultReason
    });
  }

  const expected = Number(stats.expected || 0);
  const skipped = Number(stats.skipped || 0);
  const unexpected = Number(stats.unexpected || 0);
  const allSkipped = expected === 0 && skipped > 0 && unexpected === 0;

  if (result.code !== 0 || unexpected > 0 || (report.errors && report.errors.length)) {
    return createRunnerSummary({
      passed: false,
      outcome: 'failed',
      skipped,
      blockedReasons,
      reason: defaultReason
    });
  }
  if (allSkipped) {
    const runtimeExternalOnly = hasOnlyRuntimeExternalBlocks(blockedReasons);
    return createRunnerSummary({
      passed: runtimeExternalOnly,
      outcome: runtimeExternalOnly ? 'partial-skip' : 'all-skipped',
      skipped,
      blockedReasons,
      reason: runtimeExternalOnly
        ? 'all E2E tests skipped because a declared runtime-external prerequisite was unavailable'
        : 'all E2E tests skipped; environment/data preconditions were not satisfied'
    });
  }
  return createRunnerSummary({
    passed: true,
    outcome: skipped > 0 ? 'partial-skip' : 'passed',
    skipped,
    blockedReasons,
    reason: skipped > 0 ? `skipped ${skipped}; check fixture readiness if this was unexpected` : ''
  });
}

module.exports = {
  extractBlockedReasons,
  hasOnlyRuntimeExternalBlocks,
  createRunnerSummary,
  summarizeMochaResult,
  summarizePlaywrightResult
};
