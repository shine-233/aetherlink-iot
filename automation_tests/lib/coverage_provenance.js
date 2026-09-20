const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

const writeJsonArtifact = require('./json_artifact');

const SCHEMA = 'aetherlink.coverage.hit.v1';
const KINDS = new Set(['endpoint', 'page']);
const DISPOSITIONS = new Set(['candidate', 'effective', 'diagnostic']);
const COVERING_OUTCOMES = new Set(['passed', 'expected']);
const NON_COVERING_OUTCOMES = new Set([
  'failed',
  'skipped',
  'partial',
  'partial-skip',
  'all-skipped',
  'flaky',
  'mismatched',
  'unexpected'
]);

function cleanString(value) {
  return typeof value === 'string' && value.trim() ? value.trim() : null;
}

function finiteInteger(value) {
  const number = Number(value);
  return Number.isInteger(number) && number >= 0 ? number : null;
}

function normalizeStatusCode(value) {
  const statusCode = finiteInteger(value);
  return statusCode !== null && statusCode >= 100 && statusCode <= 599
    ? statusCode
    : null;
}

function normalizeAttempt(attempt = {}) {
  const retry = finiteInteger(attempt.retry);
  const workerIndex = finiteInteger(attempt.workerIndex);
  const parallelIndex = finiteInteger(attempt.parallelIndex);
  const repeatEachIndex = finiteInteger(attempt.repeatEachIndex);
  return {
    retry,
    workerIndex,
    parallelIndex,
    repeatEachIndex,
    transportFailure: attempt.transportFailure === true,
    responseReceived: attempt.responseReceived === true,
    errorCode: cleanString(attempt.errorCode),
    errorMessage: cleanString(attempt.errorMessage)
  };
}

function normalizeCaseContext(context = {}) {
  const file = cleanString(context.file);
  const title = cleanString(context.title);
  const titlePath = Array.isArray(context.titlePath)
    ? context.titlePath.map(cleanString).filter(Boolean)
    : [];
  const caseId = cleanString(context.caseId);
  const hook = cleanString(context.hook);
  const attributed = Boolean(file && (title || titlePath.length > 0 || caseId));
  return { file, title, titlePath, caseId, hook, attributed };
}

function normalizeFile(value) {
  const file = cleanString(value);
  if (!file) return null;

  let normalized = file.replace(/\\/g, '/').replace(/^\.\//, '');
  const automationIndex = normalized.toLowerCase().lastIndexOf('/automation_tests/');
  if (automationIndex >= 0) {
    normalized = normalized.slice(automationIndex + '/automation_tests/'.length);
  }
  return normalized.replace(/^automation_tests\//i, '');
}

function titlePathKey(value) {
  const titlePath = Array.isArray(value) ? value.map(cleanString).filter(Boolean) : [];
  return titlePath.length > 0 ? JSON.stringify(titlePath) : null;
}

function authoritativeTitlePath(item = {}) {
  if (Array.isArray(item.titlePath)) {
    const explicit = item.titlePath.map(cleanString).filter(Boolean);
    if (explicit.length > 0) return explicit;
  }
  const fullTitle = cleanString(item.fullTitle);
  if (!fullTitle) return [];
  return fullTitle.includes(' › ')
    ? fullTitle.split(' › ').map(cleanString).filter(Boolean)
    : [fullTitle];
}

function addUniqueCase(index, key, item) {
  if (!key) return;
  if (!index.has(key)) {
    index.set(key, item);
  } else {
    index.set(key, false);
  }
}

function defaultRunId() {
  return cleanString(process.env.AETHERLINK_COVERAGE_RUN_ID) || 'unattributed-run';
}

function createEvent(input = {}) {
  const kind = cleanString(input.kind);
  if (!KINDS.has(kind)) throw new Error('coverage provenance kind must be endpoint or page');
  const target = cleanString(input.target);
  if (!target) throw new Error('coverage provenance target is required');
  const moduleName = cleanString(input.module) ||
    cleanString(process.env.AETHERLINK_COVERAGE_MODULE) ||
    'unknown';
  const caseContext = normalizeCaseContext(input.case);
  const attempt = normalizeAttempt(input.attempt);
  const requestedDisposition = cleanString(input.disposition) || 'candidate';
  if (!DISPOSITIONS.has(requestedDisposition)) {
    throw new Error('invalid coverage provenance disposition: ' + requestedDisposition);
  }
  const outcome = cleanString(input.outcome) || 'pending';
  const observedAt = cleanString(input.observedAt) || new Date().toISOString();
  if (!Number.isFinite(Date.parse(observedAt))) {
    throw new Error('coverage provenance observedAt must be an ISO timestamp');
  }
  const disposition = requestedDisposition === 'candidate' && (
    !caseContext.attributed || attempt.transportFailure || NON_COVERING_OUTCOMES.has(outcome)
  ) ? 'diagnostic' : requestedDisposition;

  return {
    schema: SCHEMA,
    eventId: cleanString(input.eventId) || crypto.randomUUID(),
    runId: cleanString(input.runId) || defaultRunId(),
    module: moduleName,
    kind,
    target,
    case: caseContext,
    attempt,
    outcome,
    statusCode: normalizeStatusCode(input.statusCode),
    observedAt: new Date(observedAt).toISOString(),
    disposition,
    diagnostics: input.diagnostics && typeof input.diagnostics === 'object'
      ? { ...input.diagnostics }
      : {}
  };
}

function validateEvent(event) {
  if (!event || event.schema !== SCHEMA) return false;
  if (!cleanString(event.eventId) || !cleanString(event.runId)) return false;
  if (!cleanString(event.module) || !KINDS.has(event.kind) || !cleanString(event.target)) return false;
  if (!event.case || typeof event.case.attributed !== 'boolean') return false;
  if (!event.attempt || typeof event.attempt.transportFailure !== 'boolean') return false;
  if (!cleanString(event.outcome) || !DISPOSITIONS.has(event.disposition)) return false;
  if (!cleanString(event.observedAt) || !Number.isFinite(Date.parse(event.observedAt))) return false;
  if (event.statusCode !== null && normalizeStatusCode(event.statusCode) === null) return false;
  return true;
}

function eventIdentity(event) {
  return event && cleanString(event.eventId);
}

function mergeEvents(...collections) {
  const rank = { candidate: 1, diagnostic: 2, effective: 3 };
  const byId = new Map();
  for (const collection of collections) {
    for (const event of Array.isArray(collection) ? collection : []) {
      if (!validateEvent(event)) continue;
      const id = eventIdentity(event);
      if (!id) continue;
      const current = byId.get(id);
      if (!current || rank[event.disposition] >= rank[current.disposition]) {
        byId.set(id, event);
      }
    }
  }
  return Array.from(byId.values()).sort((left, right) => (
    left.observedAt.localeCompare(right.observedAt) ||
    (eventIdentity(left) || '').localeCompare(eventIdentity(right) || '')
  ));
}

function readLedger(filePath) {
  if (!filePath || !fs.existsSync(filePath)) return { schema: SCHEMA, events: [] };
  try {
    const payload = JSON.parse(fs.readFileSync(filePath, 'utf8'));
    return { schema: SCHEMA, events: mergeEvents(payload && payload.events) };
  } catch (_error) {
    return { schema: SCHEMA, events: [] };
  }
}

function writeLedger(filePath, events) {
  if (!filePath) return;
  const persisted = readLedger(filePath);
  writeJsonArtifact(filePath, {
    schema: SCHEMA,
    events: mergeEvents(persisted.events, events)
  });
}

function replaceLedger(filePath, events) {
  if (!filePath) return;
  writeJsonArtifact(filePath, {
    schema: SCHEMA,
    events: mergeEvents(events)
  });
}

function appendEvent(filePath, event) {
  writeLedger(filePath, [event]);
  return event;
}

function outcomeFromCaseResult(result = {}) {
  const status = cleanString(result.status) || 'unknown';
  const retry = finiteInteger(result.retry) || 0;
  if (status === 'skipped') return 'skipped';
  if (status !== 'passed' && status !== 'expected') return status === 'timedOut' ? 'failed' : status;
  if (retry > 0) return 'flaky';
  return status === 'expected' ? 'expected' : 'passed';
}

function qualifyEvents(events, authoritative = {}) {
  const runId = cleanString(authoritative.runId);
  const moduleName = cleanString(authoritative.module);
  const moduleOutcome = cleanString(authoritative.moduleOutcome) || 'unknown';
  const modulePassed = authoritative.modulePassed === true && moduleOutcome === 'passed';
  const cases = Array.isArray(authoritative.cases) ? authoritative.cases : [];
  const casesById = new Map();
  const casesByFileTitlePath = new Map();
  const casesByFileTitle = new Map();
  for (const item of cases) {
    if (!item || typeof item !== 'object') continue;
    const normalized = {
      caseId: cleanString(item.caseId),
      file: normalizeFile(item.file),
      title: cleanString(item.title),
      titlePath: authoritativeTitlePath(item),
      outcome: cleanString(item.outcome) || outcomeFromCaseResult(item),
      attempts: Array.isArray(item.attempts) ? item.attempts.map(normalizeAttempt) : [],
      mismatched: item.mismatched === true
    };
    addUniqueCase(casesById, normalized.caseId, normalized);
    const pathKey = titlePathKey(normalized.titlePath);
    addUniqueCase(
      casesByFileTitlePath,
      normalized.file && pathKey ? normalized.file + '::' + pathKey : null,
      normalized
    );
    addUniqueCase(
      casesByFileTitle,
      normalized.file && normalized.title ? normalized.file + '::' + normalized.title : null,
      normalized
    );
  }

  return mergeEvents(events).map(event => {
    const runMatches = !runId || event.runId === runId;
    const moduleMatches = !moduleName || event.module === moduleName;
    if (!runMatches || !moduleMatches) {
      return {
        ...event,
        disposition: 'diagnostic',
        diagnostics: {
          ...event.diagnostics,
          qualifiedByAuthoritativeResult: false,
          authoritativeModuleOutcome: moduleOutcome,
          authoritativeCaseOutcome: null,
          qualificationScopeMatched: false
        }
      };
    }
    const eventFile = normalizeFile(event.case.file);
    const eventPathKey = titlePathKey(event.case.titlePath);
    let matched = event.case.caseId
      ? casesById.get(event.case.caseId)
      : undefined;
    if (matched === undefined && eventFile && eventPathKey) {
      matched = casesByFileTitlePath.get(eventFile + '::' + eventPathKey);
    }
    if (matched === undefined && eventFile && event.case.title) {
      matched = casesByFileTitle.get(eventFile + '::' + event.case.title);
    }
    let outcome = event.outcome;
    let disposition = 'diagnostic';
    const statusEligible = event.kind !== 'endpoint' || event.statusCode !== null;
    if (matched) {
      outcome = matched.mismatched ? 'mismatched' : matched.outcome;
      if (modulePassed && COVERING_OUTCOMES.has(outcome) && statusEligible && !event.attempt.transportFailure) {
        disposition = 'effective';
      }
    }
    return {
      ...event,
      outcome,
      disposition,
      diagnostics: {
        ...event.diagnostics,
        qualifiedByAuthoritativeResult: Boolean(matched),
        authoritativeModuleOutcome: moduleOutcome,
        authoritativeCaseOutcome: matched ? outcome : null,
        qualificationScopeMatched: true
      }
    };
  });
}

function summarizeEvents(events) {
  const valid = mergeEvents(events);
  return {
    schema: SCHEMA,
    total: valid.length,
    candidate: valid.filter(event => event.disposition === 'candidate').length,
    effective: valid.filter(event => event.disposition === 'effective').length,
    diagnostic: valid.filter(event => event.disposition === 'diagnostic').length,
    effectiveEvents: valid.filter(event => event.disposition === 'effective')
  };
}

function provenanceFileForCoverage(coverageFile) {
  if (!coverageFile) return '';
  const extension = path.extname(coverageFile);
  const stem = extension ? coverageFile.slice(0, -extension.length) : coverageFile;
  return stem + '-provenance.json';
}

module.exports = {
  SCHEMA,
  createEvent,
  validateEvent,
  mergeEvents,
  readLedger,
  writeLedger,
  replaceLedger,
  appendEvent,
  qualifyEvents,
  summarizeEvents,
  outcomeFromCaseResult,
  provenanceFileForCoverage,
  normalizeCaseContext,
  normalizeAttempt
};
