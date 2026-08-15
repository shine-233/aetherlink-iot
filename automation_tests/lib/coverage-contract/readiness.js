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
      !item.hasBackend ||
      !item.hasGMQTT;
  });
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
    endpointComparison,
    frontendWeakAssertionAudit,
    goSourceStringContractAudit,
    mappedTestFileAudit,
    missingCapabilityEndpoints,
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
      goSourceStringContractAudit.length === 0 &&
      frontendWeakAssertionAudit.length === 0,
    businessClosureReady: hasNoBusinessClosureGaps({
      skipAudit,
      catalogClassificationAudit,
      explicitBusinessInventoryComplete,
      mappedTestFileAudit,
      missingTraceability,
      businessAssertionAudit,
      sourceReviewBoundaryAudit
    }),
    skipAudit,
    traceability,
    missingTraceability,
    trustworthy: hasTrustworthyCoverageHarness({
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
  hasNoBusinessAssertionGaps,
  hasNoCatalogInventoryOrMappingGaps,
  hasNoSourceReviewBoundaryGaps
};
