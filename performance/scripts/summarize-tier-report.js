import fs from 'node:fs'
import path from 'node:path'

const archiveArgIndex = process.argv.indexOf('--archive')
const archiveRoot = archiveArgIndex >= 0 ? process.argv[archiveArgIndex + 1] : ''

if (!archiveRoot) {
  console.error('Usage: node summarize-tier-report.js --archive <path>')
  process.exit(1)
}

const manifestPath = path.join(archiveRoot, 'manifest.json')
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
const failedCommands = (manifest.commands || []).filter(command => Number(command.exit_code) !== 0)
const resourceSnapshotPath = path.join(archiveRoot, 'raw', 'resource-snapshot.json')
const resourceSnapshotExists = fs.existsSync(resourceSnapshotPath)
const tierProfile = manifest.tier_profile || {}
const slo = tierProfile.slo || {}
const commandStatus = failedCommands.length === 0 ? 'complete' : 'failed'
const verdict = failedCommands.length === 0 ? 'unknown' : 'fail'
const blockingGaps = Array.isArray(manifest.blocking_gaps) ? manifest.blocking_gaps : []
const executionMode = manifest.execution_mode || 'unknown'
const loadGenerationExecuted = manifest.load_generation_executed === true
const executedScenarios = Array.isArray(manifest.executed_scenarios) ? manifest.executed_scenarios : []

const summary = {
  schema: manifest.schema || 'aetherlink.performance.benchmark.v1',
  tier: manifest.tier,
  tier_profile: tierProfile,
  execution_mode: executionMode,
  load_generation_executed: loadGenerationExecuted,
  executed_scenarios: executedScenarios,
  target_url: manifest.target_url,
  backend_url: manifest.backend_url,
  started_at: manifest.started_at,
  finished_at: manifest.finished_at,
  command_count: (manifest.commands || []).length,
  failed_command_count: failedCommands.length,
  command_status: commandStatus,
  verdict,
  capacity_claim_status: 'unknown',
  resource_snapshot: resourceSnapshotExists ? resourceSnapshotPath : null,
  failed_commands: failedCommands.map(command => command.name),
  blocking_gaps: [
    ...(resourceSnapshotExists ? [] : ['raw/resource-snapshot.json is missing.']),
    ...blockingGaps
  ]
}

fs.writeFileSync(path.join(archiveRoot, 'summary.json'), `${JSON.stringify(summary, null, 2)}\n`)

const report = [
  `# Performance Tier Report: ${summary.tier}`,
  '',
  `Verdict: ${summary.verdict}`,
  `Command status: ${summary.command_status}`,
  `Capacity claim status: ${summary.capacity_claim_status}`,
  `Execution mode: ${summary.execution_mode}`,
  `Load generation executed: ${summary.load_generation_executed}`,
  `Executed scenarios: ${summary.executed_scenarios.length ? summary.executed_scenarios.join(', ') : 'none'}`,
  `Target URL: ${summary.target_url}`,
  `Backend URL: ${summary.backend_url}`,
  `Started: ${summary.started_at}`,
  `Finished: ${summary.finished_at}`,
  `Commands: ${summary.command_count}`,
  `Failed commands: ${summary.failed_command_count}`,
  `Resource snapshot: ${summary.resource_snapshot || 'missing'}`,
  '',
  '## Tier Profile',
  '',
  '| Field | Value |',
  '| --- | --- |',
  `| CPU | ${tierProfile.cpu ?? 'unknown'} |`,
  `| Memory MB | ${tierProfile.memoryMb ?? 'unknown'} |`,
  `| Duration seconds | ${tierProfile.durationSeconds ?? 'unknown'} |`,
  `| API concurrent users | ${tierProfile.apiConcurrentUsers ?? 'unknown'} |`,
  `| MQTT clients | ${tierProfile.mqttClients ?? 'unknown'} |`,
  `| API p95 SLO ms | ${slo.apiP95Ms ?? 'unknown'} |`,
  `| Error-rate max | ${slo.errorRateMax ?? 'unknown'} |`,
  `| Frontend first-load p95 SLO ms | ${slo.frontendFirstLoadP95Ms ?? 'unknown'} |`,
  '',
  '## Required Measured Results',
  '',
  '| Result | Value | Raw Evidence |',
  '| --- | --- | --- |',
  '| Device count sustained | TODO | TODO |',
  '| MQTT messages per second sustained | TODO | TODO |',
  '| API p95 latency | TODO | TODO |',
  '| API error rate | TODO | TODO |',
  '| Frontend first load p95 | TODO | TODO |',
  '| Backend CPU and memory peak | TODO | TODO |',
  '| Broker CPU and memory peak | TODO | TODO |',
  '| PostgreSQL CPU, memory, DB size | TODO | TODO |',
  '| Redis CPU and memory peak | TODO | TODO |',
  '',
  '## Boundary',
  '',
  'This report summarizes archived command evidence. It is not a capacity promise until the required measured results are filled from raw evidence and reviewed.',
  ''
]

if (summary.blocking_gaps.length) {
  report.push(
    '## Blocking Gaps',
    '',
    ...summary.blocking_gaps.map(gap => `- ${gap}`),
    ''
  )
}

report.push(
  '## Command Evidence',
  '',
  '| Command | Exit Code | Stdout | Stderr |',
  '| --- | --- | --- | --- |',
  ...(manifest.commands || []).map(command => {
    return `| ${command.name} | ${command.exit_code} | ${command.stdout || ''} | ${command.stderr || ''} |`
  }),
  ''
)

if (failedCommands.length) {
  report.push('## Failed Commands', '', ...failedCommands.map(command => `- ${command.name}`), '')
}

fs.writeFileSync(path.join(archiveRoot, 'report.md'), `${report.join('\n')}\n`)
