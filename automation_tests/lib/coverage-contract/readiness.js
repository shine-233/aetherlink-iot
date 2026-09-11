const { BUSINESS_OPERATIONS } = require('./business-operations');

/**
 * Pure readiness predicates for the coverage contract.
 *
 * This module only combines audit results. Source scanning and evidence
 * collection stay in the coverage-contract facade so callers keep one stable
 * public entry point.
 */
function hasCompleteExplicitBusinessInventory(explicitBusinessInventoryAudit) {
  return explicitBusinessInventoryAudit.missingEndpoints.length === 0 &&
    explicitBusinessInventoryAudit.missingRoutes.length === 0;
}

function getMissingTraceability(traceability) {
  return traceability.filter(item => {
    return !item.hasFrontendRoute ||
      !item.hasEndpoint ||
      !item.hasAutomation ||
      !item.hasTrueAutomation ||
      !item.hasE2E ||
      !item.hasTrueE2E ||
      !item.hasBackendDeclaration ||
      !item.hasGMQTTDeclaration;
  });
}

function hasCompleteOperationInventory(operationInventoryAudit) {
  return Boolean(operationInventoryAudit && operationInventoryAudit.valid);
}

function hasCompleteOperationCoverage(operationTraceability) {
  if (!Array.isArray(operationTraceability)) return false;
  const canonicalIds = BUSINESS_OPERATIONS.map(item => item.id);
  const actualIds = operationTraceability.map(item => item && item.id);
  return actualIds.length === canonicalIds.length &&
    new Set(actualIds).size === actualIds.length &&
    canonicalIds.every(id => actualIds.includes(id)) &&
    operationTraceability.every(item =>
      item.ready === true &&
      item.staticInventoryValid === true &&
      item.runtimeStatus === 'passed' &&
      Array.isArray(item.missingDimensions) &&
      item.missingDimensions.length === 0 &&
      Array.isArray(item.runtimeOutcomeErrors) &&
      item.runtimeOutcomeErrors.length === 0
    );
}

function hasNoCatalogIdentityGaps(catalogIdentityAudit) {
  return Boolean(catalogIdentityAudit) &&
    catalogIdentityAudit.duplicateEndpoints.length === 0 &&
    catalogIdentityAudit.duplicateRoutes.length === 0 &&
    catalogIdentityAudit.duplicateMetadataFiles.length === 0;
}

function hasNoCatalogInventoryOrMappingGaps(
  catalogClassificationAudit,
  explicitBusinessInventoryComplete,
  mappedTestFileAudit
) {
  return catalogClassificationAudit.unclassifiedEndpoints.length === 0 &&
    catalogClassificationAudit.unclassifiedRoutes.length === 0 &&
    explicitBusinessInventoryComplete &&
    mappedTestFileAudit.length === 0;
}

function hasNoBusinessAssertionGaps(businessAssertionAudit, options = {}) {
  const includeWeakBodyAssertions = options.includeWeakBodyAssertions !== false;
  return businessAssertionAudit.seedBlockedReturns.length === 0 &&
    (!includeWeakBodyAssertions || businessAssertionAudit.weakBodyAssertions.length === 0) &&
    businessAssertionAudit.weakExistenceAssertions.length === 0 &&
    businessAssertionAudit.weakFlexibleShapeAssertions.length === 0 &&
    businessAssertionAudit.weakObjectOnlyAssertions.length === 0 &&
    businessAssertionAudit.weakBareObjectAssertions.length === 0 &&
    businessAssertionAudit.weakConditionalEmptyAssertions.length === 0 &&
    businessAssertionAudit.weakNullableHelperAssertions.length === 0 &&
    businessAssertionAudit.broadNon200Assertions.length === 0 &&
    businessAssertionAudit.businessE2EFallbackAssertions.length === 0 &&
    businessAssertionAudit.e2eMetadataSourceGaps.length === 0 &&
    businessAssertionAudit.e2eRouteSmokeAssertions.length === 0 &&
    businessAssertionAudit.e2eCurrentStateAssertions.length === 0 &&
    businessAssertionAudit.prohibitedCoverageMarkerTitles.length === 0 &&
    businessAssertionAudit.genericBlockedReasons.length === 0;
}

function hasNoStructuredBusinessEvidenceGaps({
  blockedReasonAudit,
  catalogClassificationAudit,
  explicitBusinessInventoryComplete,
  mappedTestFileAudit,
  businessAssertionAudit
}) {
  return blockedReasonAudit.seedableReasons.length === 0 &&
    hasNoCatalogInventoryOrMappingGaps(
      catalogClassificationAudit,
      explicitBusinessInventoryComplete,
      mappedTestFileAudit
    ) &&
    hasNoBusinessAssertionGaps(businessAssertionAudit);
}

function hasNoSourceReviewBoundaryGaps(sourceReviewBoundaryAudit) {
  if (!sourceReviewBoundaryAudit) return false;
  return sourceReviewBoundaryAudit.sourceInventoryDeclaresStaticBoundary === true &&
    sourceReviewBoundaryAudit.qualityReviewRejectsRequestWrapperClosure === true &&
    sourceReviewBoundaryAudit.qualityReviewRejectsSourceInventoryClosure === true &&
    sourceReviewBoundaryAudit.qualityReviewRejectsSmokeClosure === true &&
    sourceReviewBoundaryAudit.qualityReviewKeepsReleaseGateOpen === true &&
    sourceReviewBoundaryAudit.unsafeClosureClaims.length === 0;
}

function hasNoBusinessClosureGaps({
  skipAudit,
  catalogClassificationAudit,
  explicitBusinessInventoryComplete,
  mappedTestFileAudit,
  missingTraceability,
  businessAssertionAudit,
  sourceReviewBoundaryAudit
}) {
  return skipAudit.explicitBlockedHelpers === 0 &&
    hasNoCatalogInventoryOrMappingGaps(
      catalogClassificationAudit,
      explicitBusinessInventoryComplete,
      mappedTestFileAudit
    ) &&
    missingTraceability.length === 0 &&
    hasNoBusinessAssertionGaps(businessAssertionAudit) &&
    hasNoSourceReviewBoundaryGaps(sourceReviewBoundaryAudit);
}

function hasTrustworthyCoverageHarness({
  routeComparison,
  endpointComparison,
  missingCapabilityEndpoints,
  catalogClassificationAudit,
  explicitBusinessInventoryComplete,
  mappedTestFileAudit,
  missingTraceability,
  skipAudit,
  businessAssertionAudit,
  sourceReviewBoundaryAudit
}) {
  return routeComparison.missingFromCatalog.length === 0 &&
    endpointComparison.missingFromCatalog.length === 0 &&
    missingCapabilityEndpoints.length === 0 &&
    hasNoCatalogInventoryOrMappingGaps(
      catalogClassificationAudit,
      explicitBusinessInventoryComplete,
      mappedTestFileAudit
    ) &&
    missingTraceability.length === 0 &&
    skipAudit.rawMochaSkips.length === 0 &&
    skipAudit.rawPlaywrightSkips.length === 0 &&
    hasNoSourceReviewBoundaryGaps(sourceReviewBoundaryAudit) &&
    hasNoBusinessAssertionGaps(businessAssertionAudit, {
      includeWeakBodyAssertions: false
    });
}

function getSelfCheckReadiness(audits) {
  const {
    blockedReasonAudit,
    businessAssertionAudit,
    catalogClassificationAudit,
    catalogIdentityAudit,
    endpointComparison,
    frontendWeakAssertionAudit,
    goEvidenceInventoryAudit,
    goSourceStringContractAudit,
    mappedTestFileAudit,
    missingCapabilityEndpoints,
    operationInventoryAudit,
    operationTraceability,
    routeComparison,
    skipAudit,
    sourceReviewBoundaryAudit,
    traceability
  } = audits;
  const explicitBusinessInventoryComplete = hasCompleteExplicitBusinessInventory(audits.explicitBusinessInventoryAudit);
  const missingTraceability = getMissingTraceability(traceability);
  const batchStructureReady = hasNoStructuredBusinessEvidenceGaps({
    blockedReasonAudit,
    catalogClassificationAudit,
    explicitBusinessInventoryComplete,
    mappedTestFileAudit,
    businessAssertionAudit
  });

  return {
    runtimeBlockedHelpers: blockedReasonAudit.runtimeReasons.length,
    seedableBlockedHelpers: blockedReasonAudit.seedableReasons.length,
    batchStructureReady,
    allLayerStructureReady: batchStructureReady &&
      hasNoCatalogIdentityGaps(catalogIdentityAudit) &&
      Boolean(goEvidenceInventoryAudit && goEvidenceInventoryAudit.valid) &&
      goSourceStringContractAudit.length === 0 &&
      frontendWeakAssertionAudit.length === 0,
    businessClosureReady: hasNoCatalogIdentityGaps(catalogIdentityAudit) &&
      hasNoBusinessClosureGaps({
      skipAudit,
      catalogClassificationAudit,
      explicitBusinessInventoryComplete,
      mappedTestFileAudit,
      missingTraceability,
      businessAssertionAudit,
      sourceReviewBoundaryAudit
    }) &&
      hasCompleteOperationInventory(operationInventoryAudit) &&
      hasCompleteOperationCoverage(operationTraceability),
    skipAudit,
    traceability,
    missingTraceability,
    trustworthy: hasNoCatalogIdentityGaps(catalogIdentityAudit) &&
      hasTrustworthyCoverageHarness({
      routeComparison,
      endpointComparison,
      missingCapabilityEndpoints,
      catalogClassificationAudit,
      explicitBusinessInventoryComplete,
      mappedTestFileAudit,
      missingTraceability,
      skipAudit,
      businessAssertionAudit,
      sourceReviewBoundaryAudit
    })
  };
}

module.exports = {
  getMissingTraceability,
  getSelfCheckReadiness,
  hasCompleteExplicitBusinessInventory,
  hasCompleteOperationCoverage,
  hasCompleteOperationInventory,
  hasNoBusinessAssertionGaps,
  hasNoCatalogIdentityGaps,
  hasNoCatalogInventoryOrMappingGaps,
  hasNoSourceReviewBoundaryGaps
};
