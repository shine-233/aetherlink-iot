const EVIDENCE_KINDS = new Set([
  'business', 'boundary', 'contract', 'catalog', 'config', 'preflight',
  'page-smoke', 'page-coverage-only',
]);
const OPERATION_DIMENSIONS = new Set([
  'userAction', 'response', 'mutation', 'stateReadback', 'negativeControl',
  'idempotency', 'runtimeSideEffect', 'tenantScope', 'visibleResult', 'cleanup',
]);

function isPlainObject(value) {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value) &&
    Object.getPrototypeOf(value) === Object.prototype;
}

function assertExactKeys(value, allowed, label) {
  if (!isPlainObject(value)) throw new TypeError(`${label} must be a plain object`);
  for (const key of Object.keys(value)) {
    if (!allowed.includes(key)) throw new TypeError(`${label} has unknown key: ${key}`);
  }
  for (const key of allowed) {
    if (!(key in value)) throw new TypeError(`${label} is missing key: ${key}`);
  }
}

function stringArray(value, label, allowed = null) {
  if (!Array.isArray(value)) throw new TypeError(`${label} must be an array`);
  const result = value.map(item => {
    if (typeof item !== 'string' || !item.trim()) {
      throw new TypeError(`${label} entries must be non-empty strings`);
    }
    const normalized = item.trim();
    if (allowed && !allowed.has(normalized)) {
      throw new TypeError(`${label} has unsupported value: ${normalized}`);
    }
    return normalized;
  });
  if (new Set(result).size !== result.length) {
    throw new TypeError(`${label} must not contain duplicates`);
  }
  return result;
}

function requiredString(value, label) {
  if (typeof value !== 'string' || !value.trim()) {
    throw new TypeError(`${label} must be a non-empty string`);
  }
  return value.trim();
}

function normalizeV2(config) {
  const keys = [
    'schemaVersion', 'caseId', 'title', 'fullTitle', 'evidenceKind',
    'businessClosureEvidence', 'assertions', 'capabilityIds', 'operationIds',
    'operationDimensions', 'semantics',
  ];
  assertExactKeys(config, keys, 'metadataCase v2');
  if (config.schemaVersion !== 2) throw new TypeError('metadataCase schemaVersion must be 2');
  const caseId = requiredString(config.caseId, 'metadataCase.caseId');
  if (!/^[a-z0-9]+(?:[.-][a-z0-9]+)*$/.test(caseId)) {
    throw new TypeError(`metadataCase.caseId is invalid: ${caseId}`);
  }
  const evidenceKind = requiredString(config.evidenceKind, 'metadataCase.evidenceKind');
  if (!EVIDENCE_KINDS.has(evidenceKind)) {
    throw new TypeError(`metadataCase.evidenceKind is unsupported: ${evidenceKind}`);
  }
  if (typeof config.businessClosureEvidence !== 'boolean') {
    throw new TypeError('metadataCase.businessClosureEvidence must be boolean');
  }
  assertExactKeys(
    config.assertions,
    ['exactStatus', 'body', 'mutationOrSeed', 'negative'],
    'metadataCase.assertions',
  );
  for (const [key, value] of Object.entries(config.assertions)) {
    if (typeof value !== 'boolean') throw new TypeError(`metadataCase.assertions.${key} must be boolean`);
  }
  assertExactKeys(
    config.semantics,
    ['actor', 'role', 'tenant', 'idempotency', 'state', 'visibleResult', 'cleanup'],
    'metadataCase.semantics',
  );
  const semantics = {
    actor: requiredString(config.semantics.actor, 'metadataCase.semantics.actor'),
    role: requiredString(config.semantics.role, 'metadataCase.semantics.role'),
    tenant: requiredString(config.semantics.tenant, 'metadataCase.semantics.tenant'),
    idempotency: requiredString(config.semantics.idempotency, 'metadataCase.semantics.idempotency'),
    state: stringArray(config.semantics.state, 'metadataCase.semantics.state'),
    visibleResult: requiredString(config.semantics.visibleResult, 'metadataCase.semantics.visibleResult'),
    cleanup: requiredString(config.semantics.cleanup, 'metadataCase.semantics.cleanup'),
  };
  const operationIds = stringArray(config.operationIds, 'metadataCase.operationIds');
  const operationDimensions = stringArray(
    config.operationDimensions,
    'metadataCase.operationDimensions',
    OPERATION_DIMENSIONS,
  );
  if (operationDimensions.length > 0 && operationIds.length === 0) {
    throw new TypeError('metadataCase operation dimensions require operationIds');
  }
  if (config.businessClosureEvidence && (
    evidenceKind !== 'business' ||
    !config.assertions.exactStatus ||
    !config.assertions.body
  )) {
    throw new TypeError('business closure metadata requires business exact-status and body assertions');
  }
  if (operationDimensions.includes('cleanup') && semantics.cleanup !== 'verified-by-case') {
    throw new TypeError('cleanup operation evidence must be verified-by-case');
  }
  return {
    schemaVersion: 2,
    caseId,
    title: requiredString(config.title, 'metadataCase.title'),
    fullTitle: requiredString(config.fullTitle, 'metadataCase.fullTitle'),
    evidenceKind,
    businessClosureEvidence: config.businessClosureEvidence,
    hasExactStatusAssertion: config.assertions.exactStatus,
    hasBodyAssertion: config.assertions.body,
    hasMutationOrSeedAction: config.assertions.mutationOrSeed,
    hasNegativeAssertion: config.assertions.negative,
    assertions: { ...config.assertions },
    capabilityIds: stringArray(config.capabilityIds, 'metadataCase.capabilityIds'),
    operationIds,
    operationDimensions,
    semantics,
  };
}

function metadataCase(configOrTitle, ...legacyArgs) {
  if (isPlainObject(configOrTitle)) return normalizeV2(configOrTitle);
  const [evidenceKind, businessClosureEvidence, hasExactStatusAssertion,
    hasBodyAssertion, hasMutationOrSeedAction, hasNegativeAssertion,
    capabilityIds = [], options = {}] = legacyArgs;
  return {
    title: configOrTitle,
    fullTitle: options.fullTitle || configOrTitle,
    evidenceKind,
    businessClosureEvidence,
    hasExactStatusAssertion,
    hasBodyAssertion,
    hasMutationOrSeedAction,
    hasNegativeAssertion,
    capabilityIds,
  };
}

function e2eCase(title, evidenceKind, businessClosureEvidence, provesBusinessFlow, options = {}) {
  return {
    title, evidenceKind, businessClosureEvidence, provesBusinessFlow,
    capabilityIds: Array.isArray(options.capabilityIds) ? options.capabilityIds : [],
    evidenceLayer: options.evidenceLayer || 'browser-e2e',
    hasBrowserUserFlow: options.hasBrowserUserFlow !== false,
    firstDeviceOnboarding: options.firstDeviceOnboarding === true,
    readyCheckDiagnosticsBundle: options.readyCheckDiagnosticsBundle === true,
    otaSupportArchive: options.otaSupportArchive === true,
    requiresSeededDevice: options.requiresSeededDevice === true,
    requiresSeededOtaTask: options.requiresSeededOtaTask === true,
    runtimeEvidenceRequired: options.runtimeEvidenceRequired === true,
    ...(options.caseId ? { caseId: options.caseId } : {}),
    ...(options.fullTitle ? { fullTitle: options.fullTitle } : {}),
    ...(options.operationDimensions ? { operationDimensions: options.operationDimensions } : {}),
  };
}

module.exports = { metadataCase, e2eCase };
