/**
 * Offline contract for the default Compose healthcheck startup boundary.
 *
 * Docker does not count probe failures during start_period toward retries. Keep
 * this local contract so a slow first boot cannot silently lose that allowance.
 */

const { expect } = require('chai');
const fs = require('fs');
const path = require('path');

const composeSource = fs.readFileSync(
  path.resolve(__dirname, '..', '..', 'docker-compose.yml'),
  'utf8'
).replace(/\r\n/g, '\n');

function serviceBlocks(source) {
  const blocks = {};
  const lines = source.split('\n');
  let currentService = null;

  for (const line of lines.slice(lines.indexOf('services:') + 1)) {
    if (/^[^\s#][^:]*:/.test(line)) {
      break;
    }

    const service = line.match(/^  ([A-Za-z0-9_-]+):\s*$/);
    if (service) {
      currentService = service[1];
      blocks[currentService] = [];
    } else if (currentService) {
      blocks[currentService].push(line);
    }
  }

  return Object.fromEntries(
    Object.entries(blocks).map(([name, linesForService]) => [name, linesForService.join('\n')])
  );
}

describe('Docker Compose healthcheck contract [00_docker_compose_healthcheck_contract]', function() {
  const services = serviceBlocks(composeSource);

  it('gives every default service a bounded startup grace period', function() {
    expect(Object.keys(services).sort()).to.deep.equal([
      'backend',
      'frontend',
      'mqtt-broker',
      'postgres',
      'redis'
    ]);

    for (const [name, source] of Object.entries(services)) {
      expect(source, `${name} requires a healthcheck`).to.match(/(?:^|\n)    healthcheck:/);
      expect(source, `${name} requires an executable probe`).to.match(
        /(?:^|\n)      test: \["CMD(?:-SHELL)?", ".+"\](?:\n|$)/
      );
      expect(source, `${name} requires an interval`).to.match(/(?:^|\n)      interval: \S+/);
      expect(source, `${name} requires a timeout`).to.match(/(?:^|\n)      timeout: \S+/);
      expect(source, `${name} requires bounded retries`).to.match(
        /(?:^|\n)      retries: [1-9]\d*/
      );
      expect(source, `${name} requires startup grace`).to.match(
        /(?:^|\n)      start_period: [1-9]\d*(?:ns|us|ms|s|m|h)/
      );
    }
  });
});
