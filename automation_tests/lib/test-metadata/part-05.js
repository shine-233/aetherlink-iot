const { metadataCase, e2eCase } = require("./helpers");

module.exports = {
  "e2e/07_dashboard.spec.js": {
    file: "e2e/07_dashboard.spec.js",
    type: "e2e",
    evidenceKind: "boundary",
    fileFlags: {},
    cases: [
      e2eCase(
        "tenant admin receives an explicit 403 boundary for the dashboard root",
        "boundary",
        false,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "tenant admin receives an explicit 403 boundary for the dashboard RDI route",
        "boundary",
        false,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          runtimeEvidenceRequired: true,
        },
      ),
    ],
  },
  "e2e/23_exception_routes.spec.js": {
    file: "e2e/23_exception_routes.spec.js",
    type: "e2e",
    evidenceKind: "boundary",
    fileFlags: {},
    cases: [
      e2eCase(
        "exception page 403 renders its recovery action and returns home",
        "boundary",
        false,
        false,
        { evidenceLayer: "browser-e2e", runtimeEvidenceRequired: true },
      ),
      e2eCase(
        "exception page 404 renders its recovery action and returns home",
        "boundary",
        false,
        false,
        { evidenceLayer: "browser-e2e", runtimeEvidenceRequired: true },
      ),
      e2eCase(
        "exception page 500 renders its recovery action and returns home",
        "boundary",
        false,
        false,
        { evidenceLayer: "browser-e2e", runtimeEvidenceRequired: true },
      ),
    ],
  },
  "e2e/09_management.spec.js": {
    file: "e2e/09_management.spec.js",
    type: "e2e",
    evidenceKind: "business",
    fileFlags: {},
    cases: [
      e2eCase(
        "super-admin tenant list in the browser matches authenticated user API state",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["permission-tenancy"],
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "super-admin role API access is separated from its unassigned UI route permission",
        "boundary",
        false,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["permission-tenancy"],
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "notification email form matches the persisted service configuration returned by API",
        "contract",
        false,
        false,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["alarm-notification"],
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "renders the selected plugin access list from API",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["ota-script-openapi-service"],
          runtimeEvidenceRequired: true,
        },
      ),
    ],
  },
  "e2e/11_visualization.spec.js": {
    file: "e2e/11_visualization.spec.js",
    type: "e2e",
    evidenceKind: "business",
    fileFlags: {},
    cases: [
      e2eCase(
        "native board CRUD is persisted by the local provider across list viewer and editor routes",
        "business",
        true,
        true,
        {
          capabilityIds: ["visualization"],
          evidenceLayer: "browser-e2e-with-api-setup",
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "native board flows through the local provider on all ThingsVis compatibility routes",
        "business",
        true,
        true,
        {
          capabilityIds: ["visualization"],
          evidenceLayer: "browser-e2e-with-api-setup",
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "seeded ThingsVis project and dashboard render across project list editor preview and menu routes",
        "business",
        true,
        true,
        {
          capabilityIds: ["visualization"],
          evidenceLayer: "browser-e2e-with-api-setup",
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "dashboard menu API rejects missing dashboard ownership and the visualization route stays stable",
        "boundary",
        false,
        false,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          hasBrowserUserFlow: true,
          capabilityIds: ["visualization"],
        },
      ),
      e2eCase(
        "dashboard menu persists for a real ThingsVis dashboard when the mirror is available",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["visualization"],
        },
      ),
    ],
  },
  "e2e/13_write_flows.spec.js": {
    file: "e2e/13_write_flows.spec.js",
    type: "e2e",
    evidenceKind: "business",
    fileFlags: {},
    cases: [
      e2eCase(
        "seeded device template is searchable in the UI and matches the list/detail APIs",
        "business",
        true,
        true,
        {
          capabilityIds: ["device-telemetry"],
          evidenceLayer: "browser-e2e-with-api-setup",
        },
      ),
    ],
  },
  "e2e/15_apply_marketplace.spec.js": {
    file: "e2e/15_apply_marketplace.spec.js",
    type: "e2e",
    evidenceKind: "boundary",
    fileFlags: {},
    cases: [
      e2eCase(
        "tenant admin is denied service-plugin APIs and both apply management routes",
        "boundary",
        false,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
        },
      ),
      e2eCase(
        "super admin service-plugin API remains available while the legacy apply route is explicitly denied",
        "boundary",
        false,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
        },
      ),
    ],
  },
  "e2e/19_device_details_app.spec.js": {
    file: "e2e/19_device_details_app.spec.js",
    type: "e2e",
    // This route is intentionally read-only: it proves the authenticated
    // detail/telemetry payload and rendered identity, but it exposes no
    // operator control that a browser user can exercise. Keep it as boundary
    // evidence instead of inflating business E2E closure from page load alone.
    evidenceKind: "boundary",
    fileFlags: {},
    cases: [
      e2eCase(
        "seeded device opens in the standalone app with matching detail API identity and status",
        "boundary",
        false,
        false,
        {
          capabilityIds: ["device-telemetry"],
          evidenceLayer: "browser-e2e-with-api-setup",
          hasBrowserUserFlow: false,
          requiresSeededDevice: true,
          runtimeEvidenceRequired: true,
        },
      ),
    ],
  },
  "e2e/20_command_jobs.spec.js": {
    file: "e2e/20_command_jobs.spec.js",
    type: "e2e",
    evidenceKind: "business",
    fileFlags: {
      requiresSeededDevice: true,
      runtimeEvidenceRequired: true,
      commandJobLifecycle: true,
    },
    cases: [
      e2eCase(
        "browser draft previews, submits, and finds the persisted job in history",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["command-jobs"],
          requiresSeededDevice: true,
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "scheduled command job is canceled from the browser and persists the canceled state",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["command-jobs"],
          requiresSeededDevice: true,
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "failed device acknowledgement is retried from the browser through the real broker path",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["command-jobs"],
          requiresSeededDevice: true,
          runtimeEvidenceRequired: true,
        },
      ),
      e2eCase(
        "selected-device command job stays visible with result and support evidence",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["command-jobs"],
        },
      ),
      e2eCase(
        "selected-device command job downloads support bundle JSON for operator handoff",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["command-jobs"],
        },
      ),
      e2eCase(
        "support preview failed devices link directly to Ready Check and Job detail",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["command-jobs"],
          requiresSeededDevice: true,
          runtimeEvidenceRequired: true,
        },
      ),
    ],
  },
  "e2e/21_ready_check_command_draft.spec.js": {
    file: "e2e/21_ready_check_command_draft.spec.js",
    type: "e2e",
    evidenceKind: "business",
    fileFlags: {
      requiresSeededDevice: true,
      runtimeEvidenceRequired: true,
      commandRouteDraft: true,
      commandJobLifecycle: true,
    },
    cases: [
      e2eCase(
        "Ready Check route draft previews, submits, and persists the same command job",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["command-jobs"],
          requiresSeededDevice: true,
          runtimeEvidenceRequired: true,
        },
      ),
    ],
  },
  "e2e/22_ota_support_archive.spec.js": {
    file: "e2e/22_ota_support_archive.spec.js",
    type: "e2e",
    evidenceKind: "business",
    fileFlags: {
      requiresSeededOtaTask: true,
      runtimeEvidenceRequired: true,
      otaSupportArchive: true,
    },
    cases: [
      e2eCase(
        "OTA task detail downloads support archive with task-level counts and conditional Ready Check handoff fields",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["ota-script-openapi-service"],
          otaSupportArchive: true,
          requiresSeededOtaTask: true,
          runtimeEvidenceRequired: true,
        },
      ),
    ],
  },
  "e2e/23_home.spec.js": {
    file: "e2e/23_home.spec.js",
    type: "e2e",
    evidenceKind: "business",
    fileFlags: {
      requiresSeededDevice: true,
      runtimeEvidenceRequired: true,
    },
    cases: [
      e2eCase(
        "home renders live first-device state and refreshes the workbench from the browser",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["system-deployment"],
          requiresSeededDevice: true,
          runtimeEvidenceRequired: true,
        },
      ),
    ],
  },
  "e2e/23_native_board_super_admin.spec.js": {
    file: "e2e/23_native_board_super_admin.spec.js",
    type: "e2e",
    evidenceKind: "business",
    fileFlags: {
      runtimeEvidenceRequired: true,
    },
    cases: [
      e2eCase(
        "uses the selected tenant filter as the create context",
        "business",
        true,
        true,
        {
          evidenceLayer: "browser-e2e-with-api-setup",
          capabilityIds: ["visualization", "permission-tenancy"],
          runtimeEvidenceRequired: true,
        },
      ),
    ],
  },
  "tests/23_seeded_automation_scene.test.js": {
    file: "tests/23_seeded_automation_scene.test.js",
    type: "api",
    evidenceKind: "business",
    fileFlags: {},
    cases: [
      {
        title:
          "creates a seeded scene and verifies detail, list, and activation surfaces",
        evidenceKind: "business",
        businessClosureEvidence: true,
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasMutationOrSeedAction: true,
        hasNegativeAssertion: false,
        capabilityIds: ["automation-scene"],
      },
      {
        title:
          "asserts scene and automation negative branches with explicit product errors",
        evidenceKind: "boundary",
        businessClosureEvidence: false,
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasMutationOrSeedAction: true,
        hasNegativeAssertion: true,
        capabilityIds: ["automation-scene"],
      },
    ],
  },
  "tests/24_seeded_scene_automations.test.js": {
    file: "tests/24_seeded_scene_automations.test.js",
    type: "api",
    evidenceKind: "business",
    fileFlags: {},
    cases: [
      {
        title:
          "creates, updates, switches, filters, and deletes a seeded scene automation",
        evidenceKind: "business",
        businessClosureEvidence: true,
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasMutationOrSeedAction: true,
        hasNegativeAssertion: true,
        capabilityIds: ["automation-scene"],
      },
      {
        title:
          "rejects non-existent scene automation ids with explicit product errors",
        evidenceKind: "boundary",
        businessClosureEvidence: false,
        hasExactStatusAssertion: true,
        hasBodyAssertion: false,
        hasMutationOrSeedAction: true,
        hasNegativeAssertion: true,
        capabilityIds: ["automation-scene"],
      },
      {
        title:
          "keeps create response id shape explicit for seeded automation helpers",
        evidenceKind: "contract",
        businessClosureEvidence: false,
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasMutationOrSeedAction: true,
        hasNegativeAssertion: false,
        capabilityIds: ["automation-scene"],
      },
      {
        title:
          "dry-runs a seeded scene automation payload without saving or executing it",
        evidenceKind: "business",
        businessClosureEvidence: true,
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasMutationOrSeedAction: true,
        hasNegativeAssertion: false,
        capabilityIds: ["automation-scene"],
      },
      {
        title:
          "dry-run explains invalid action references without accepting the payload",
        evidenceKind: "boundary",
        businessClosureEvidence: false,
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasMutationOrSeedAction: true,
        hasNegativeAssertion: true,
        capabilityIds: ["automation-scene"],
      },
    ],
  },
};
