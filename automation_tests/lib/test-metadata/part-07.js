const { metadataCase } = require("./helpers");

const EDGE_NODES_SUITE = "Edge node registry and reconcile [38_edge_nodes]";
const LICENSE_STATUS_SUITE = "License status view [39_license_status]";
const TELEMETRY_ANOMALY_SUITE = "Telemetry anomaly detection [40_telemetry_anomaly]";
const MARKET_BUNDLE_SUITE = "Market bundle export / import gate [41_market_bundle_import]";
const OPERATION_LOG_EXPORT_SUITE = "Operation log audit export [42_operation_logs_export]";

function managedMetadataCase(suite, config) {
  return metadataCase({
    schemaVersion: 2,
    ...config,
    fullTitle: `${suite} ${config.title}`,
  });
}

function managedCase(caseId, title, options = {}) {
  return {
    caseId,
    title,
    evidenceKind: options.evidenceKind || "business",
    businessClosureEvidence: options.businessClosureEvidence !== false,
    assertions: {
      exactStatus: options.exactStatus !== false,
      body: options.body !== false,
      mutationOrSeed: options.mutationOrSeed !== false,
      negative: options.negative === true,
    },
    capabilityIds: options.capabilityIds || [],
    operationIds: options.operationIds || [],
    operationDimensions: options.operationDimensions || [],
    semantics: {
      actor: options.actor || "prepared-tenant-admin",
      role: options.role || "tenant_admin",
      tenant: options.tenant || "own-tenant",
      idempotency: options.idempotency || "not-applicable",
      state: options.state || [],
      visibleResult: options.visibleResult || "api-response-asserted",
      cleanup: options.cleanup || "not-applicable",
    },
  };
}

module.exports = {
  "tests/38_edge_nodes.test.js": {
    file: "tests/38_edge_nodes.test.js",
    type: "api",
    evidenceKind: "business",
    fileFlags: { runtimeEvidenceRequired: true, caseMetadataManaged: true },
    cases: [
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.register.create", "registers an edge node and returns an active node with a health classification", { capabilityIds: ["edge-governance"], operationIds: ["edge.node.register"], operationDimensions: ["response", "mutation", "stateReadback", "tenantScope"], state: ["node-created", "last-seen-touched", "health-classified"] })),
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.register.reject-missing-fields", "rejects a registration request without node_id or version", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["edge-governance"], operationIds: ["edge.node.register"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["missing-node-id-rejected", "missing-version-rejected"] })),
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.register.idempotent", "is idempotent: re-registering the same node in the same tenant does not create a duplicate", { capabilityIds: ["edge-governance"], operationIds: ["edge.node.register"], operationDimensions: ["response", "mutation", "stateReadback", "idempotency"], idempotency: "same-node-id-updates-in-place", state: ["version-updated", "no-duplicate-row"] })),
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.register.cross-tenant-squat", "rejects cross-tenant squatting of an already registered node id", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["edge-governance", "permission-tenancy"], operationIds: ["edge.node.register"], operationDimensions: ["response", "negativeControl", "tenantScope"], mutationOrSeed: false, negative: true, tenant: "own-and-other-tenant", state: ["cross-tenant-registration-rejected"] })),
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.list.readback", "lists the registered node inside the tenant", { capabilityIds: ["edge-governance"], operationIds: ["edge.node.list"], operationDimensions: ["response", "stateReadback", "tenantScope"], mutationOrSeed: false, state: ["node-visible-in-tenant-list"] })),
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.heartbeat.online", "heartbeat refreshes last_seen_at and reports online health", { capabilityIds: ["edge-governance"], operationIds: ["edge.node.heartbeat"], operationDimensions: ["response", "mutation", "stateReadback"], state: ["last-seen-refreshed", "health-online"] })),
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.reconcile.version-gate", "reconcile returns a plan and fails closed on an incompatible version", { capabilityIds: ["edge-governance"], operationIds: ["edge.node.reconcile"], operationDimensions: ["response", "stateReadback", "negativeControl"], mutationOrSeed: false, negative: true, state: ["plan-returned", "incompatible-version-blocked"] })),
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.reconcile.unregistered-node", "reconcile rejects an unregistered node id", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["edge-governance"], operationIds: ["edge.node.reconcile"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["unregistered-node-rejected"] })),
      managedMetadataCase(EDGE_NODES_SUITE, managedCase("edge.node.reconcile.foreign-gateway", "reconcile rejects a gateway device that is not in the tenant", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["edge-governance", "permission-tenancy"], operationIds: ["edge.node.reconcile"], operationDimensions: ["response", "negativeControl", "tenantScope"], mutationOrSeed: false, negative: true, state: ["foreign-gateway-rejected"] })),
    ],
  },
  "tests/39_license_status.test.js": {
    file: "tests/39_license_status.test.js",
    type: "api",
    evidenceKind: "business",
    fileFlags: { runtimeEvidenceRequired: true, caseMetadataManaged: true },
    cases: [
      managedMetadataCase(LICENSE_STATUS_SUITE, managedCase("license.status.read", "returns a status view for platform admins", { actor: "prepared-platform-admin", role: "SYS_ADMIN", capabilityIds: ["license-governance"], operationIds: ["license.status"], operationDimensions: ["response", "stateReadback"], mutationOrSeed: false, state: ["status-view-read"] })),
      managedMetadataCase(LICENSE_STATUS_SUITE, managedCase("license.status.disabled-reason", "explains itself when the license boundary is not enabled", { actor: "prepared-platform-admin", role: "SYS_ADMIN", capabilityIds: ["license-governance"], operationIds: ["license.status"], operationDimensions: ["response", "stateReadback", "negativeControl"], mutationOrSeed: false, negative: true, state: ["disabled-boundary-explained", "disabled-never-valid"] })),
      managedMetadataCase(LICENSE_STATUS_SUITE, managedCase("license.status.no-material-leak", "never returns the license material itself", { actor: "prepared-platform-admin", role: "SYS_ADMIN", capabilityIds: ["license-governance"], operationIds: ["license.status"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["no-material-keys"] })),
      managedMetadataCase(LICENSE_STATUS_SUITE, managedCase("license.status.fingerprint-gate", "reports a fingerprint only when the license is valid", { actor: "prepared-platform-admin", role: "SYS_ADMIN", capabilityIds: ["license-governance"], operationIds: ["license.status"], operationDimensions: ["response", "stateReadback"], mutationOrSeed: false, state: ["fingerprint-bound-to-validity"] })),
      managedMetadataCase(LICENSE_STATUS_SUITE, managedCase("license.status.role-gate", "rejects non-platform-admin roles", { evidenceKind: "boundary", businessClosureEvidence: false, actor: "prepared-tenant-admin", role: "tenant_admin", capabilityIds: ["license-governance", "permission-tenancy"], operationIds: ["license.status"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["non-admin-denied"] })),
      managedMetadataCase(LICENSE_STATUS_SUITE, managedCase("license.status.unauthenticated-gate", "rejects unauthenticated access", { evidenceKind: "boundary", businessClosureEvidence: false, actor: "anonymous", role: "anonymous", capabilityIds: ["license-governance", "permission-tenancy"], operationIds: ["license.status"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["unauthenticated-denied"] })),
    ],
  },
  "tests/40_telemetry_anomaly.test.js": {
    file: "tests/40_telemetry_anomaly.test.js",
    type: "api",
    evidenceKind: "business",
    fileFlags: { requiresSeededDevice: true, runtimeEvidenceRequired: true, caseMetadataManaged: true },
    cases: [
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.bounds", "accepts a bounds rule and returns one row per device", { capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.bounds"], operationDimensions: ["response", "stateReadback", "tenantScope"], mutationOrSeed: false, state: ["result-shape-readback", "one-row-per-device"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.deviation", "accepts a deviation rule and defaults k to 3", { capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.deviation"], operationDimensions: ["response", "stateReadback"], mutationOrSeed: false, state: ["k-defaulted-to-three"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.empty-window", "reports an empty window as \"no data in window\" instead of \"no anomalies\"", { capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.empty-window"], operationDimensions: ["response", "stateReadback", "negativeControl"], mutationOrSeed: false, negative: true, state: ["explicit-no-data-error", "no-fabricated-anomalies"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.reject-window-order", "rejects an end_time that is not after start_time", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.window"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["inverted-window-rejected"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.reject-window-overflow", "rejects a window_ms larger than the query window", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.window"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["oversized-bucket-rejected"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.reject-bounds-empty", "rejects a bounds rule without min and max", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.bounds"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["empty-bounds-rejected"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.reject-bounds-inverted", "rejects a bounds rule whose min is greater than max", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.bounds"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["inverted-bounds-rejected"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.reject-nonpositive-k", "rejects a deviation rule with a non-positive k", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.deviation"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["non-positive-k-rejected"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.reject-unsupported-vocabulary", "rejects an unsupported rule type and an unsupported aggregate", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.vocabulary"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["unknown-rule-type-rejected", "unknown-aggregate-rejected"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.partial-readability", "marks unreadable devices per-row without failing the whole request", { capabilityIds: ["device-telemetry", "permission-tenancy"], operationIds: ["telemetry.anomaly.partial-readability"], operationDimensions: ["response", "stateReadback", "negativeControl", "tenantScope"], mutationOrSeed: false, negative: true, state: ["unreadable-row-marked", "batch-not-aborted"] })),
      managedMetadataCase(TELEMETRY_ANOMALY_SUITE, managedCase("telemetry.anomaly.reject-empty-input", "rejects an empty device list and a missing key", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["device-telemetry"], operationIds: ["telemetry.anomaly.input"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["empty-device-list-rejected", "missing-key-rejected"] })),
    ],
  },
  "tests/41_market_bundle_import.test.js": {
    file: "tests/41_market_bundle_import.test.js",
    type: "api",
    evidenceKind: "business",
    fileFlags: { runtimeEvidenceRequired: true, caseMetadataManaged: true },
    cases: [
      managedMetadataCase(MARKET_BUNDLE_SUITE, managedCase("market.bundle.verify.unsigned", "rejects an unsigned bundle even when only previewing", { capabilityIds: ["device-telemetry"], operationIds: ["market.bundle.verify"], operationDimensions: ["response", "negativeControl", "tenantScope"], mutationOrSeed: false, negative: true, state: ["unsigned-bundle-rejected", "verify-stage-reported"] })),
      managedMetadataCase(MARKET_BUNDLE_SUITE, managedCase("market.bundle.verify.tampered", "rejects a bundle whose signature does not match its digest", { capabilityIds: ["device-telemetry"], operationIds: ["market.bundle.verify"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["tampered-signature-rejected"] })),
      managedMetadataCase(MARKET_BUNDLE_SUITE, managedCase("market.bundle.verify.missing-payload", "rejects a request without a bundle payload", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["device-telemetry"], operationIds: ["market.bundle.verify"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["missing-bundle-rejected"] })),
      managedMetadataCase(MARKET_BUNDLE_SUITE, managedCase("market.bundle.preview.readonly", "does not modify anything while previewing", { capabilityIds: ["device-telemetry"], operationIds: ["market.bundle.preview"], operationDimensions: ["response", "stateReadback"], mutationOrSeed: false, state: ["applied-false", "preview-shape-readback"] })),
      managedMetadataCase(MARKET_BUNDLE_SUITE, managedCase("market.bundle.import.idempotent", "re-importing the same signed bundle is idempotent and does not demand overwrite confirmation", { capabilityIds: ["device-telemetry"], operationIds: ["market.bundle.import"], operationDimensions: ["response", "mutation", "stateReadback", "idempotency", "tenantScope"], idempotency: "same-name-version-idempotent", state: ["applied-true", "per-template-outcomes", "same-version-not-classified-as-overwrite"] })),
    ],
  },
  "tests/42_operation_logs_export.test.js": {
    file: "tests/42_operation_logs_export.test.js",
    type: "api",
    evidenceKind: "business",
    fileFlags: { runtimeEvidenceRequired: true, caseMetadataManaged: true },
    cases: [
      managedMetadataCase(OPERATION_LOG_EXPORT_SUITE, managedCase("audit.export.reject-missing-window", "rejects an export without a time window", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["audit-log-export"], operationIds: ["audit.export.window"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["missing-window-rejected"] })),
      managedMetadataCase(OPERATION_LOG_EXPORT_SUITE, managedCase("audit.export.reject-inverted-window", "rejects a window whose end is not after its start", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["audit-log-export"], operationIds: ["audit.export.window"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["inverted-window-rejected"] })),
      managedMetadataCase(OPERATION_LOG_EXPORT_SUITE, managedCase("audit.export.reject-oversized-window", "rejects a window longer than one year", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["audit-log-export"], operationIds: ["audit.export.window"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["oversized-window-rejected"] })),
      managedMetadataCase(OPERATION_LOG_EXPORT_SUITE, managedCase("audit.export.reject-empty-window", "rejects an empty window instead of producing an empty CSV", { evidenceKind: "boundary", businessClosureEvidence: false, capabilityIds: ["audit-log-export"], operationIds: ["audit.export.window"], operationDimensions: ["response", "negativeControl"], mutationOrSeed: false, negative: true, state: ["empty-window-rejected"] })),
      managedMetadataCase(OPERATION_LOG_EXPORT_SUITE, managedCase("audit.export.csv-envelope", "exports the current tenant audit logs as a CSV envelope", { capabilityIds: ["audit-log-export"], operationIds: ["audit.export.csv"], operationDimensions: ["response", "stateReadback", "tenantScope"], mutationOrSeed: false, state: ["csv-envelope-readback", "non-zero-row-count"] })),
      managedMetadataCase(OPERATION_LOG_EXPORT_SUITE, managedCase("audit.export.no-payload-columns", "never writes request/response payload columns into the CSV header", { capabilityIds: ["audit-log-export"], operationIds: ["audit.export.csv"], operationDimensions: ["response", "stateReadback", "negativeControl"], mutationOrSeed: false, negative: true, state: ["header-free-of-payload-columns"] })),
    ],
  },
};
