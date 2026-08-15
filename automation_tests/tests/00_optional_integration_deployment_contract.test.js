/**
 * Offline deployment contract for the lightweight stack and optional integrations.
 *
 * This suite intentionally reads committed templates rather than a real `.env` or
 * running containers. It fails closed when optional services leak into the default
 * stack, lose their profile/readiness boundary, or Nginx upstreams drift from the
 * Compose service ports.
 */

const { expect } = require('chai');
const fs = require('fs');
const path = require('path');

const projectRoot = path.resolve(__dirname, '..', '..');

function readProjectFile(relativePath) {
  return fs.readFileSync(path.join(projectRoot, relativePath), 'utf8');
}

function serviceBlocks(composeSource) {
  const lines = composeSource.replace(/\r\n/g, '\n').split('\n');
  const servicesLine = lines.findIndex((line) => line === 'services:');
  if (servicesLine === -1) {
    throw new Error('Compose source has no top-level services section');
  }

  const blocks = {};
  let currentName = null;
  for (let index = servicesLine + 1; index < lines.length; index += 1) {
    const line = lines[index];
    if (/^[^\s#][^:]*:/.test(line)) {
      break;
    }

    const serviceMatch = line.match(/^  ([A-Za-z0-9_-]+):\s*$/);
    if (serviceMatch) {
      currentName = serviceMatch[1];
      blocks[currentName] = [];
      continue;
    }
    if (currentName !== null) {
      blocks[currentName].push(line);
    }
  }

  return Object.fromEntries(
    Object.entries(blocks).map(([name, linesForService]) => [name, linesForService.join('\n')])
  );
}

function envExampleValues(source) {
  const values = {};
  for (const line of source.replace(/\r\n/g, '\n').split('\n')) {
    const match = line.match(/^([A-Z][A-Z0-9_]*)=(.*)$/);
    if (match) {
      values[match[1]] = match[2];
    }
  }
  return values;
}

function expectDependency(serviceSource, dependency, condition) {
  const dependencyPattern = new RegExp(
    `(?:^|\\n)    depends_on:\\n(?:      .+\\n)*?      ${dependency}:\\n        condition: ${condition}(?:\\n|$)`
  );
  expect(serviceSource).to.match(dependencyPattern);
}

function expectContainerPort(serviceSource, port) {
  const portPattern = new RegExp(
    `(?:^|\\n)      - "(?:\\$\\{AETHERLINK_BIND_ADDRESS:-127\\.0\\.0\\.1\\}:)?\\$\\{[A-Z0-9_]+:-${port}\\}:${port}"(?:\\n|$)`
  );
  expect(serviceSource).to.match(portPattern);
}

function nginxUpstreams(source) {
  return [...source.matchAll(/proxy_pass\s+http:\/\/([A-Za-z0-9_-]+):(\d+)/g)]
    .map((match) => `${match[1]}:${match[2]}`);
}

describe('Optional integration deployment contract [00_optional_integration_deployment_contract]', function() {
  const defaultCompose = readProjectFile('docker-compose.yml');
  const optionalCompose = readProjectFile('deploy/docker-compose.optional-integrations.yml');
  const defaultNginx = readProjectFile('frontend/nginx.conf');
  const optionalNginx = readProjectFile('frontend/nginx.thingsvis.conf');
  const frontendDockerfile = readProjectFile('frontend/Dockerfile');
  const envExample = readProjectFile('.env.example');
  const deployReadme = readProjectFile('deploy/README.md');
  const defaultServices = serviceBlocks(defaultCompose);
  const optionalServices = serviceBlocks(optionalCompose);

  it('keeps the default Compose service set lightweight and profile-free', function() {
    expect(Object.keys(defaultServices).sort()).to.deep.equal([
      'backend',
      'frontend',
      'mqtt-broker',
      'postgres',
      'redis'
    ]);

    for (const [name, source] of Object.entries(defaultServices)) {
      expect(source, `${name} must remain in the default profile`).not.to.match(/(?:^|\n)    profiles:/);
    }
    expect(defaultCompose).not.to.match(/thingsvis|http_adapter|optional-integrations/i);
  });

  it('gates every optional service and gateway override behind one explicit profile', function() {
    expect(Object.keys(optionalServices).sort()).to.deep.equal([
      'backend',
      'frontend',
      'http_adapter',
      'thingsvis-server',
      'thingsvis-studio'
    ]);

    for (const [name, source] of Object.entries(optionalServices)) {
      expect(source, `${name} must require explicit opt-in`).to.match(
        /(?:^|\n)    profiles: \[optional-integrations\](?:\n|$)/
      );
    }

    expect(optionalServices.frontend).to.include(
      './frontend/nginx.thingsvis.conf:/etc/nginx/nginx.conf:ro'
    );
    for (const key of [
      'GOTP_INTEGRATIONS_THINGSVIS_ENABLED',
      'GOTP_INTEGRATIONS_THINGSVIS_CONFIGURED',
      'GOTP_INTEGRATIONS_HTTP_ADAPTER_ENABLED',
      'GOTP_INTEGRATIONS_HTTP_ADAPTER_CONFIGURED'
    ]) {
      expect(optionalServices.backend).to.include(`${key}: "true"`);
    }
    expect(deployReadme).to.include('-f deploy/docker-compose.optional-integrations.yml');
    expect(deployReadme).to.include('--profile optional-integrations up -d');
  });

  it('keeps legacy frontend routes off by default and enables them only in the optional build', function() {
    expect(frontendDockerfile).to.match(/ARG VITE_ENABLE_THINGSVIS_COMPAT=N/);
    // Dockerfile keeps public build variables in one multiline ENV declaration.
    expect(frontendDockerfile).to.match(
      /ENV[\s\S]*VITE_ENABLE_THINGSVIS_COMPAT=\$\{VITE_ENABLE_THINGSVIS_COMPAT\}/
    );
    expect(defaultServices.frontend).not.to.include('VITE_ENABLE_THINGSVIS_COMPAT');
    expect(optionalServices.frontend).to.match(
      /(?:^|\n)    build:\n      args:\n        VITE_ENABLE_THINGSVIS_COMPAT: "Y"(?:\n|$)/
    );
    expect(deployReadme).to.include('VITE_ENABLE_THINGSVIS_COMPAT=Y');
  });

  it('preserves the default and optional dependency readiness graph', function() {
    expectDependency(defaultServices['mqtt-broker'], 'postgres', 'service_healthy');
    expectDependency(defaultServices['mqtt-broker'], 'redis', 'service_healthy');
    expectDependency(defaultServices.backend, 'postgres', 'service_healthy');
    expectDependency(defaultServices.backend, 'redis', 'service_healthy');
    expectDependency(defaultServices.backend, 'mqtt-broker', 'service_healthy');
    expectDependency(defaultServices.frontend, 'backend', 'service_healthy');

    expectDependency(optionalServices.http_adapter, 'backend', 'service_started');
    expectDependency(optionalServices.http_adapter, 'mqtt-broker', 'service_started');
    expectDependency(optionalServices['thingsvis-server'], 'postgres', 'service_healthy');
    expectDependency(optionalServices['thingsvis-studio'], 'thingsvis-server', 'service_healthy');
    expectDependency(optionalServices.frontend, 'thingsvis-studio', 'service_healthy');
  });

  it('requires secrets and keeps checked-in example credentials non-deployable', function() {
    const requiredSecretExpressions = [
      'POSTGRES_PASSWORD',
      'REDIS_PASSWORD',
      'GOTP_DB_PSQL_PASSWORD',
      'GOTP_DB_REDIS_PASSWORD',
      'MQTT_ROOT_PASSWORD',
      'MQTT_PLUGIN_PASSWORD',
      'MQTT_BROKER_ID',
      'GOTP_JWT_KEY'
    ];
    for (const key of requiredSecretExpressions) {
      expect(defaultCompose, `${key} must use required Compose interpolation`)
        .to.match(new RegExp(`\\$\\{${key}:\\?[^}]+\\}`));
    }
    expect(optionalServices['thingsvis-server'])
      .to.match(/AUTH_SECRET: \$\{THINGSVIS_AUTH_SECRET:\?[^}]+\}/);

    const example = envExampleValues(envExample);
    for (const key of [
      'POSTGRES_PASSWORD',
      'REDIS_PASSWORD',
      'MQTT_ROOT_PASSWORD',
      'MQTT_PLUGIN_PASSWORD',
      'GOTP_JWT_KEY',
      'GOTP_DB_PSQL_PASSWORD',
      'GOTP_DB_REDIS_PASSWORD',
      'GOTP_MQTT_PASS'
    ]) {
      expect(example[key], `${key} must be an obvious placeholder`).to.match(/^change_me_/);
    }
    expect(example.GOTP_DB_PSQL_PASSWORD).to.equal(example.POSTGRES_PASSWORD);
    expect(example.GOTP_DB_REDIS_PASSWORD).to.equal(example.REDIS_PASSWORD);
    expect(example.GOTP_MQTT_PASS).to.equal(example.MQTT_ROOT_PASSWORD);
    expect(example.MQTT_PLUGIN_PASSWORD).not.to.equal(example.MQTT_ROOT_PASSWORD);
    expect(deployReadme).to.match(/THINGSVIS_AUTH_SECRET=replace-with-[^\s]+/);
  });

  it('gives every default service a bounded healthcheck', function() {
    for (const [name, source] of Object.entries(defaultServices)) {
      expect(source, `${name} requires a healthcheck`).to.match(/(?:^|\n)    healthcheck:/);
      expect(source, `${name} healthcheck requires an interval`).to.match(/(?:^|\n)      interval: \S+/);
      expect(source, `${name} healthcheck requires a timeout`).to.match(/(?:^|\n)      timeout: \S+/);
      expect(source, `${name} healthcheck requires bounded retries`).to.match(/(?:^|\n)      retries: [1-9]\d*/);
    }
  });

  it('makes readiness machine-verifiable for every optional integration', function() {
    for (const name of ['http_adapter', 'thingsvis-server', 'thingsvis-studio']) {
      const source = optionalServices[name];
      expect(source, `${name} requires a container healthcheck`).to.match(/(?:^|\n)    healthcheck:/);
      expect(source, `${name} healthcheck requires an interval`).to.match(/(?:^|\n)      interval: \S+/);
      expect(source, `${name} healthcheck requires a timeout`).to.match(/(?:^|\n)      timeout: \S+/);
      expect(source, `${name} healthcheck requires bounded retries`).to.match(/(?:^|\n)      retries: [1-9]\d*/);
    }
    expect(optionalServices['thingsvis-server']).to.include('http://127.0.0.1:8000/api/health');
    expect(optionalServices['thingsvis-studio']).to.include('http://127.0.0.1:3000/');
  });

  it('keeps the default Nginx boundary fail-closed for ThingsVis routes', function() {
    expect(nginxUpstreams(defaultNginx)).to.deep.equal(['backend:9999', 'backend:9999']);
    expect(defaultNginx).not.to.match(/proxy_pass\s+http:\/\/thingsvis-/);
    expect(defaultNginx.match(/return 503/g) || []).to.have.length.at.least(6);
    expect(defaultNginx.match(/THINGSVIS_OPTIONAL_SERVICE_DISABLED/g) || [])
      .to.have.length.at.least(4);
  });

  it('maps every optional Nginx upstream to a declared Compose container port', function() {
    expectContainerPort(defaultServices.backend, 9999);
    expectContainerPort(defaultServices.frontend, 8080);
    expectContainerPort(optionalServices['thingsvis-server'], 8000);
    expectContainerPort(optionalServices['thingsvis-studio'], 3000);
    expectContainerPort(optionalServices.http_adapter, 19090);
    expectContainerPort(optionalServices.http_adapter, 19091);

    expect([...new Set(nginxUpstreams(optionalNginx))].sort()).to.deep.equal([
      'backend:9999',
      'thingsvis-server:8000',
      'thingsvis-studio:3000'
    ]);
    expect(optionalNginx).not.to.match(/THINGSVIS_OPTIONAL_SERVICE_DISABLED|return 503/);
    expect(optionalServices.http_adapter).to.include('P_PLATFORM_URL: http://backend:9999');
    expect(optionalServices.http_adapter).to.include(
      'P_PLATFORM_MQTT_BROKER: tcp://mqtt-broker:1883'
    );
  });
});
