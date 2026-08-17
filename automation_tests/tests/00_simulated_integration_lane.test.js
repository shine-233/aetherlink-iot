/**
 * Contract tests for the offline simulated integration lane.
 *
 * These assertions protect the evidence boundary: a green simulation is
 * useful, but it must never be serialized as real API, broker, device, or
 * production deployment evidence.
 */
'use strict';

const { expect } = require('chai');
const {
  SYNTHETIC_PROVENANCE,
  parseArguments,
  topicMatches,
  runSimulatedIntegration,
  main
} = require('../scripts/run_simulated_integration_lane');

function memoryStream() {
  let value = '';
  return {
    write(chunk) {
      value += String(chunk);
      return true;
    },
    text() {
      return value;
    }
  };
}

describe('simulated integration lane [00_simulated_integration_lane]', function () {
  this.timeout(30000);

  let report;

  before(async function () {
    report = await runSimulatedIntegration(parseArguments([]));
  });

  it('passes every requested offline simulation lane', function () {
    expect(report.status).to.equal('simulated_pass');
    expect(Object.keys(report.lanes).sort()).to.deep.equal([
      'api_login',
      'business_e2e',
      'deployment',
      'mqtt_broker',
      'rdi',
      'sbom'
    ]);
    for (const lane of Object.values(report.lanes)) {
      expect(lane.status, lane.name).to.equal('simulated_pass');
      expect(lane.started_at, lane.name).to.be.a('string');
      expect(lane.finished_at, lane.name).to.be.a('string');
      expect(lane.checks, lane.name).to.be.an('array').that.is.not.empty;
      const requiredChecks = lane.checks.filter(check => check.status !== 'not_run');
      expect(requiredChecks.every(check => check.ok), lane.name).to.equal(true);
    }
  });

  it('keeps real external claims explicitly unproven or not-run', function () {
    expect(report.claims).to.deep.include({
      real_api_login: 'not-proven',
      real_business_e2e: 'not-proven',
      real_mqtt_broker: 'not-proven',
      real_physical_rdi: 'not-proven',
      target_server_deployment: 'not-run',
      registry_enrichment: 'not-run',
      deployment_equivalence: 'not-proven'
    });
  });

  it('marks the Docker boundary without pretending that Compose started', function () {
    expect(report.lanes.deployment.static_dry_run).to.equal('simulated_pass');
    expect(report.lanes.deployment.deployment_equivalence).to.equal('not-proven');
    expect(['not-run', 'not-validated']).to.include(report.lanes.deployment.docker_runtime);
  });

  it('marks source SBOM enrichment as not-run', function () {
    expect(report.lanes.sbom.component_count).to.be.greaterThan(0);
    expect(report.lanes.sbom.registry_enrichment).to.equal('not-run');
    expect(report.lanes.sbom.deployment_image_sbom).to.equal('not-proven');
  });

  it('preserves the synthetic RDI provenance boundary', function () {
    expect(report.lanes.rdi.evidence).to.include({
      fixture_provenance: SYNTHETIC_PROVENANCE,
      device_execution: 'not-proven',
      real_rdi_status: 'not-tested',
      production_signoff: 'not-ready'
    });
    expect(JSON.stringify(report)).not.to.match(/real[-_ ]?rdi[-_ ]?passed/i);
  });

  it('records business cleanup and does not expose the in-memory token', function () {
    expect(report.lanes.business_e2e.cleanup).to.deep.include({
      device_disconnected: true,
      api_delete_attempted: true,
      api_delete_succeeded: true
    });
    const serialized = JSON.stringify(report);
    expect(serialized).not.to.include('sim-token-');
    expect(serialized).not.to.include('simulated-password-not-a-secret');
    expect(report.cleanup.status).to.equal('passed');
  });

  it('fails closed when a caller asks for real RDI mode', async function () {
    const stdout = memoryStream();
    const stderr = memoryStream();
    const exitCode = await main(['--rdi-mode=real', '--json'], stdout, stderr);
    expect(exitCode).to.equal(2);
    const blocked = JSON.parse(stdout.text());
    expect(blocked.status).to.equal('external_blocked');
    expect(blocked.lanes).to.deep.equal({});
    expect(blocked.claims.real_physical_rdi).to.equal('not-run');
    expect(blocked.blockers.join('\n')).to.match(/device|voucher|physical/i);
    expect(stderr.text()).to.equal('');
  });

  it('matches the canonical MQTT wildcard contract', function () {
    expect(topicMatches('devices/command/SYN123/+', 'devices/command/SYN123/message-1')).to.equal(true);
    expect(topicMatches('devices/command/SYN123/+', 'devices/command/OTHER/message-1')).to.equal(false);
    expect(topicMatches('devices/status/+', 'devices/status/device-1')).to.equal(true);
    expect(topicMatches('devices/status/+', 'devices/status/device-1/extra')).to.equal(false);
  });
});
