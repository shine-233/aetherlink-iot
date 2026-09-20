const OPERATION_DIMENSIONS = Object.freeze([
  'userAction',
  'response',
  'mutation',
  'stateReadback',
  'negativeControl',
  'idempotency',
  'runtimeSideEffect',
  'tenantScope',
  'visibleResult',
  'cleanup'
]);

function mappedCase(file, title, dimensions, evidenceRole = 'business') {
  return { file, title, dimensions, evidenceRole };
}

const BUSINESS_OPERATIONS = Object.freeze([
  {
    id: 'automation.rule-chain.lifecycle',
    name: 'Rule-chain lifecycle',
    capabilityIds: ['automation-scene'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'negativeControl', 'tenantScope', 'cleanup'],
    cases: [
      mappedCase('tests/29_rule_chain_business.test.js', 'creates a tenant-owned disabled rule chain with the exact graph', ['response', 'mutation', 'tenantScope']),
      mappedCase('tests/29_rule_chain_business.test.js', 'lists and gets the created chain with exact persisted state', ['response', 'stateReadback', 'tenantScope']),
      mappedCase('tests/29_rule_chain_business.test.js', 'updates name and enabled state and persists both changes', ['response', 'mutation', 'stateReadback']),
      mappedCase('tests/29_rule_chain_business.test.js', 'rejects a disconnected action root with an exact parameter error and no row', ['response', 'stateReadback', 'negativeControl']),
      mappedCase('tests/29_rule_chain_business.test.js', 'deletes the chain and returns exact not-found state afterwards', ['response', 'mutation', 'stateReadback', 'cleanup'])
    ]
  },
  {
    id: 'automation.rule-chain.telemetry-command-runtime',
    name: 'Rule-chain telemetry command runtime',
    capabilityIds: ['automation-scene'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'negativeControl', 'runtimeSideEffect', 'cleanup'],
    cases: [
      mappedCase('tests/29_rule_chain_business.test.js', 'executes telemetry rules only above threshold and records an acknowledged automatic MQTT command', ['response', 'mutation', 'stateReadback', 'negativeControl', 'runtimeSideEffect', 'cleanup'])
    ]
  },
  {
    id: 'automation.scene.action-20-runtime',
    name: 'Scene action 20 runtime',
    capabilityIds: ['automation-scene'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'runtimeSideEffect', 'cleanup'],
    cases: [
      mappedCase('tests/31_scene_action_20_runtime.test.js', 'executes scene action 20 from a real MQTT online transition and exposes automation/scene/alarm logs', ['response', 'mutation', 'stateReadback', 'runtimeSideEffect', 'cleanup'])
    ]
  },
  {
    id: 'ota.rollout-success',
    name: 'OTA successful rollout',
    capabilityIds: ['ota-script-openapi-service'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'runtimeSideEffect', 'cleanup'],
    cases: [
      mappedCase('tests/32_ota_runtime.test.js', 'creates a task through the public API and persists a successful device-reported OTA rollout', ['response', 'mutation', 'stateReadback', 'runtimeSideEffect', 'cleanup'])
    ]
  },
  {
    id: 'ota.rollout-failure-support',
    name: 'OTA failed rollout support evidence',
    capabilityIds: ['ota-script-openapi-service'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'negativeControl', 'runtimeSideEffect', 'cleanup'],
    cases: [
      mappedCase('tests/32_ota_runtime.test.js', 'persists a device-reported OTA failure and exposes it through the support bundle', ['response', 'mutation', 'stateReadback', 'negativeControl', 'runtimeSideEffect', 'cleanup'])
    ]
  },
  {
    id: 'template.market.portable-idempotent-import',
    name: 'Portable idempotent template import',
    capabilityIds: ['device-telemetry'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'negativeControl', 'idempotency', 'cleanup'],
    cases: [
      mappedCase('tests/36_template_market.test.js', 'exports a portable template descriptor without id/tenant fields', ['response']),
      mappedCase('tests/36_template_market.test.js', 'imports the payload as a new tenant template (created=true)', ['response', 'mutation']),
      mappedCase('tests/36_template_market.test.js', 're-imports the same name+version idempotently (created=false, same id)', ['response', 'stateReadback', 'idempotency']),
      mappedCase('tests/36_template_market.test.js', 'rejects unsupported template kinds via expectBusinessError(100002)', ['response', 'negativeControl'], 'boundary'),
      mappedCase('tests/36_template_market.test.js', 'deletes both templates and returns exact not-found state afterwards', ['response', 'mutation', 'stateReadback', 'cleanup'])
    ]
  },
  {
    id: 'report.schedule.lifecycle-concurrency',
    name: 'Report schedule lifecycle and optimistic concurrency',
    capabilityIds: ['visualization'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'negativeControl', 'tenantScope', 'cleanup'],
    cases: [
      mappedCase('tests/37_report_schedule.test.js', 'creates a schedule with omitted enabled defaulting to true', ['response', 'mutation', 'tenantScope']),
      mappedCase('tests/37_report_schedule.test.js', 'lists and gets the tenant schedule with exact persisted state', ['response', 'stateReadback', 'tenantScope']),
      mappedCase('tests/37_report_schedule.test.js', 'updates by route identity and persists the next revision', ['response', 'mutation', 'stateReadback']),
      mappedCase('tests/37_report_schedule.test.js', 'rejects a stale revision and proves the schedule was not mutated', ['response', 'stateReadback', 'negativeControl']),
      mappedCase('tests/37_report_schedule.test.js', 'isolates report reads by tenant and denies non-admin write roles', ['response', 'stateReadback', 'negativeControl', 'tenantScope']),
      mappedCase('tests/37_report_schedule.test.js', 'deletes with the current revision and verifies exact not-found cleanup', ['response', 'mutation', 'stateReadback', 'cleanup'])
    ]
  },
  {
    id: 'report.run.manual-idempotent-acceptance',
    name: 'Durable manual report run acceptance and replay',
    capabilityIds: ['visualization'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'idempotency', 'runtimeSideEffect', 'tenantScope'],
    cases: [
      mappedCase('tests/37_report_schedule.test.js', 'accepts one durable manual run with exact 202 Location and stable replay', ['response', 'mutation', 'stateReadback', 'idempotency', 'runtimeSideEffect', 'tenantScope'])
    ]
  },
  {
    id: 'report.run.durable-identity',
    name: 'Durable report run identity and terminal lifecycle readback',
    capabilityIds: ['visualization'],
    requiredDimensions: ['response', 'stateReadback', 'runtimeSideEffect'],
    cases: [
      mappedCase('tests/37_report_schedule.test.js', 'exposes durable run identity and immutable window without assuming a queued race', ['response', 'stateReadback', 'runtimeSideEffect'])
    ]
  },
  {
    id: 'report.run.retry',
    name: 'Report run retry eligibility and immutable child',
    capabilityIds: ['visualization'],
    requiredDimensions: ['response', 'mutation', 'stateReadback', 'idempotency', 'negativeControl'],
    cases: [
      mappedCase('tests/37_report_schedule.test.js', 'enforces retry eligibility for a terminal parent and rejects ineligible targets', ['response', 'mutation', 'stateReadback', 'idempotency', 'negativeControl'])
    ]
  },
  {
    id: 'report.run.nested-tenant-isolation',
    name: 'Nested report run tenant isolation',
    capabilityIds: ['visualization'],
    requiredDimensions: ['response', 'negativeControl', 'tenantScope'],
    cases: [
      mappedCase('tests/37_report_schedule.test.js', 'isolates nested run list and run detail by tenant', ['response', 'negativeControl', 'tenantScope'])
    ]
  },
  {
    id: 'visualization.native-board.local-crud',
    name: 'Native board local CRUD',
    capabilityIds: ['visualization'],
    requiredDimensions: ['userAction', 'response', 'mutation', 'stateReadback', 'visibleResult', 'cleanup'],
    cases: [
      mappedCase('e2e/11_visualization.spec.js', 'native board CRUD is persisted by the local provider across list viewer and editor routes', ['userAction', 'response', 'mutation', 'stateReadback', 'visibleResult', 'cleanup'])
    ]
  },
  {
    id: 'visualization.native-board.super-admin-tenant-context',
    name: 'Native board super-admin tenant context',
    capabilityIds: ['visualization', 'permission-tenancy'],
    requiredDimensions: ['userAction', 'response', 'mutation', 'tenantScope', 'visibleResult', 'cleanup'],
    cases: [
      mappedCase('e2e/23_native_board_super_admin.spec.js', 'uses the selected tenant filter as the create context', ['userAction', 'response', 'mutation', 'tenantScope', 'visibleResult', 'cleanup'])
    ]
  }
]);

module.exports = { OPERATION_DIMENSIONS, BUSINESS_OPERATIONS };
