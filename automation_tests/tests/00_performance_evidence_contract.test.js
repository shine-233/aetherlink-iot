/**
 * Performance evidence contract: the current runner is an evidence scaffold,
 * not a load generator or a source of capacity claims.
 */
const { expect } = require('chai');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const projectRoot = path.resolve(__dirname, '..', '..');
const runnerPath = path.join(projectRoot, 'performance', 'scripts', 'run-tier-benchmark.ps1');
const summarizerPath = path.join(projectRoot, 'performance', 'scripts', 'summarize-tier-report.js');

describe('performance evidence boundary [00_performance_evidence_contract]', function () {
  it('marks the PowerShell runner as an evidence scaffold without load generation', function () {
    const source = fs.readFileSync(runnerPath, 'utf8').replace(/\r\n/g, '\n');

    expect(source).to.include('execution_mode = "evidence-scaffold"');
    expect(source).to.include('load_generation_executed = $false');
    expect(source).to.include('executed_scenarios = @()');
    expect(source).to.include('does not execute the tier duration, API concurrency, or MQTT client load');
    expect(source).to.include('catalog inclusion is not scenario execution evidence');
  });

  it('preserves unknown capacity status in generated summaries and reports', function () {
    const archiveRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-performance-contract-'));
    const rawRoot = path.join(archiveRoot, 'raw');
    fs.mkdirSync(rawRoot);
    fs.writeFileSync(path.join(rawRoot, 'resource-snapshot.json'), '{}\n');
    fs.writeFileSync(path.join(archiveRoot, 'manifest.json'), `${JSON.stringify({
      schema: 'aetherlink.performance.benchmark.v1',
      tier: '1c2g',
      tier_profile: {
        cpu: 1,
        memoryMb: 2048,
        durationSeconds: 300,
        apiConcurrentUsers: 10,
        mqttClients: 100
      },
      execution_mode: 'evidence-scaffold',
      load_generation_executed: false,
      executed_scenarios: [],
      capacity_claim_status: 'unknown',
      commands: [{ name: 'backend-health', exit_code: 0 }],
      blocking_gaps: [
        'Tier load was not executed.',
        'Scenario catalog is not execution evidence.'
      ]
    }, null, 2)}\n`);

    try {
      const result = spawnSync(process.execPath, [summarizerPath, '--archive', archiveRoot], {
        cwd: projectRoot,
        encoding: 'utf8'
      });
      expect(result.error).to.equal(undefined);
      expect(result.status, result.stderr).to.equal(0);

      const summary = JSON.parse(fs.readFileSync(path.join(archiveRoot, 'summary.json'), 'utf8'));
      const report = fs.readFileSync(path.join(archiveRoot, 'report.md'), 'utf8');

      expect(summary).to.include({
        verdict: 'unknown',
        capacity_claim_status: 'unknown',
        execution_mode: 'evidence-scaffold',
        load_generation_executed: false
      });
      expect(summary.executed_scenarios).to.deep.equal([]);
      expect(summary.blocking_gaps).to.include('Tier load was not executed.');
      expect(report).to.include('Load generation executed: false');
      expect(report).to.include('Execution mode: evidence-scaffold');
      expect(report).to.include('- Tier load was not executed.');
    } finally {
      fs.rmSync(archiveRoot, { recursive: true, force: true });
    }
  });
});
