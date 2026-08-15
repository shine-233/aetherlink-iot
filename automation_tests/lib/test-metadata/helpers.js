function metadataCase(
  title,
  evidenceKind,
  businessClosureEvidence,
  hasExactStatusAssertion,
  hasBodyAssertion,
  hasMutationOrSeedAction,
  hasNegativeAssertion,
  capabilityIds = [],
) {
  return {
    title,
    evidenceKind,
    businessClosureEvidence,
    hasExactStatusAssertion,
    hasBodyAssertion,
    hasMutationOrSeedAction,
    hasNegativeAssertion,
    capabilityIds,
  };
}

function e2eCase(
  title,
  evidenceKind,
  businessClosureEvidence,
  provesBusinessFlow,
  options = {},
) {
  return {
    title,
    evidenceKind,
    businessClosureEvidence,
    provesBusinessFlow,
    capabilityIds: Array.isArray(options.capabilityIds)
      ? options.capabilityIds
      : [],
    evidenceLayer: options.evidenceLayer || "browser-e2e",
    hasBrowserUserFlow: options.hasBrowserUserFlow !== false,
    firstDeviceOnboarding: options.firstDeviceOnboarding === true,
    readyCheckDiagnosticsBundle: options.readyCheckDiagnosticsBundle === true,
    otaSupportArchive: options.otaSupportArchive === true,
    requiresSeededDevice: options.requiresSeededDevice === true,
    requiresSeededOtaTask: options.requiresSeededOtaTask === true,
    runtimeEvidenceRequired: options.runtimeEvidenceRequired === true,
  };
}

module.exports = { metadataCase, e2eCase };
