/**
 * Keeps every tracked Docker Compose manifest on an explicit Dependabot
 * version-update path. Dockerfile entries alone do not maintain image tags
 * declared in Compose files.
 */
const { expect } = require('chai');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', '..');
const configPath = path.join(root, '.github', 'dependabot.yml');

function readConfig() {
  return fs.readFileSync(configPath, 'utf8').replace(/\r\n/g, '\n');
}

function entryFor(source, ecosystem, directory) {
  const escapedDirectory = directory.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const pattern = new RegExp(
    `- package-ecosystem: ${ecosystem}\\n\\s+directory: ${escapedDirectory}(?:\\n|$)`
  );
  return pattern.test(source);
}

describe('Dependabot configuration contract [00_dependabot_config_contract]', function () {
  it('maintains Dockerfiles and all tracked Compose manifests', function () {
    const source = readConfig();

    for (const directory of ['/backend', '/frontend', '/mqtt-broker']) {
      expect(entryFor(source, 'docker', directory), `Docker entry ${directory}`).to.equal(true);
    }

    for (const directory of ['/', '/deploy', '/backend/test/multidb']) {
      expect(
        entryFor(source, 'docker-compose', directory),
        `Docker Compose entry ${directory}`
      ).to.equal(true);
    }
  });

  it('covers every maintained dependency manifest and keeps device updates regular', function () {
    const source = readConfig();
    const expected = [
      ['github-actions', '/'],
      ['npm', '/frontend'],
      ['npm', '/automation_tests'],
      ['gomod', '/backend'],
      ['gomod', '/mqtt-broker'],
      ['gomod', '/backend/cmd/aetherlink-device-autotest'],
      ['docker', '/backend'],
      ['docker', '/frontend'],
      ['docker', '/mqtt-broker'],
      ['docker-compose', '/'],
      ['docker-compose', '/deploy'],
      ['docker-compose', '/backend/test/multidb']
    ];

    for (const [ecosystem, directory] of expected) {
      expect(entryFor(source, ecosystem, directory), `${ecosystem} entry ${directory}`).to.equal(true);
    }

    expect(source).to.include('device-autotest-minor-patch:');
    expect(source).to.include('device-autotest-security:');
  });

  it('keeps Compose image maintenance grouped and bounded', function () {
    const source = readConfig();
    const composeEntries = source.match(/package-ecosystem: docker-compose/g) || [];

    expect(composeEntries).to.have.length(3);
    expect(source.match(/open-pull-requests-limit: 5/g) || []).to.have.length.at.least(6);
    expect(source).to.include('root-compose-images:');
    expect(source).to.include('optional-compose-images:');
    expect(source).to.include('multidb-compose-images:');
  });
});
