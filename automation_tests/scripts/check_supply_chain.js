#!/usr/bin/env node
/**
 * 文件用途：执行不依赖网络或托管平台的供应链输入检查。
 * 核心逻辑：核对 Go module、Docker builder、pnpm 版本、锁文件与组件许可证边界。
 * 关键注意事项：仅核对项目声明与标准位置，不替代依赖级许可证扫描；外部能力不在此伪装成功。
 */
const fs = require('fs');
const path = require('path');

const projectRoot = path.resolve(__dirname, '..', '..');

function read(relativePath) {
  return fs.readFileSync(path.join(projectRoot, relativePath), 'utf8').replace(/\r\n/g, '\n');
}

function exists(relativePath) {
  return fs.existsSync(path.join(projectRoot, relativePath));
}

function parseVersion(value) {
  const match = String(value).match(/^(\d+)\.(\d+)(?:\.(\d+))?/);
  if (!match) throw new Error(`invalid version: ${value}`);
  return match.slice(1).map(part => Number(part || 0));
}

function compareVersions(left, right) {
  const a = parseVersion(left);
  const b = parseVersion(right);
  for (let index = 0; index < 3; index += 1) {
    if (a[index] !== b[index]) return a[index] - b[index];
  }
  return 0;
}

function goModuleVersion(relativePath) {
  const match = read(relativePath).match(/^go\s+(\d+\.\d+(?:\.\d+)?)$/m);
  if (!match) throw new Error(`${relativePath}: missing go directive`);
  return match[1];
}

function dockerGoVersion(relativePath) {
  const match = read(relativePath).match(/^FROM\s+golang:(\d+\.\d+(?:\.\d+)?)-/m);
  if (!match) throw new Error(`${relativePath}: missing pinned golang builder version`);
  return match[1];
}

function inspectSupplyChain() {
  const checks = [];
  const add = (id, ok, detail) => checks.push({ id, mode: 'local-default', status: ok ? 'pass' : 'fail', detail });

  for (const module of [
    ['backend/go.mod', 'backend/go.sum'],
    ['mqtt-broker/go.mod', 'mqtt-broker/go.sum'],
    ['backend/cmd/aetherlink-device-autotest/go.mod', 'backend/cmd/aetherlink-device-autotest/go.sum']
  ]) {
    add(`module:${module[0]}`, exists(module[0]) && exists(module[1]), `${module[0]} and ${module[1]} must coexist`);
  }

  for (const pair of [
    ['backend/go.mod', 'backend/Dockerfile'],
    ['mqtt-broker/go.mod', 'mqtt-broker/Dockerfile']
  ]) {
    const moduleVersion = goModuleVersion(pair[0]);
    const builderVersion = dockerGoVersion(pair[1]);
    add(
      `toolchain:${pair[1]}`,
      compareVersions(builderVersion, moduleVersion) >= 0,
      `builder Go ${builderVersion} must not be older than module Go ${moduleVersion}`
    );
  }

  const frontendManifest = JSON.parse(read('frontend/package.json'));
  const packageManager = frontendManifest.packageManager || '';
  add('frontend:package-manager', /^pnpm@\d+\.\d+\.\d+\+sha512\./.test(packageManager), 'packageManager must pin pnpm and integrity hash');
  add('frontend:lock-boundary', exists('frontend/pnpm-lock.yaml') && exists('frontend/pnpm-workspace.yaml'), 'pnpm lockfile and workspace manifest must coexist');
  add('license:frontend-declaration', frontendManifest.license === 'Apache-2.0', 'frontend package.json must use the SPDX identifier Apache-2.0');

  for (const component of [
    ['frontend', 'Apache License', 'Apache-2.0'],
    ['backend', 'Apache License', 'Apache-2.0'],
    ['mqtt-broker', 'MIT License', 'MIT']
  ]) {
    const licensePath = `${component[0]}/LICENSE`;
    add(
      `license:${component[0]}-file`,
      exists(licensePath) && read(licensePath).includes(component[1]),
      `${licensePath} must contain the declared ${component[2]} license text`
    );
  }

  const frontendDockerfile = read('frontend/Dockerfile');
  add('frontend:corepack', /RUN corepack enable/.test(frontendDockerfile), 'Docker build must enable Corepack');
  add('frontend:frozen-lockfile', /pnpm install --frozen-lockfile/.test(frontendDockerfile), 'Docker build must use the frozen lockfile');

  const external = [
    { id: 'vulnerability-database', mode: 'optional-external', status: 'not-run', detail: 'govulncheck and package audits require tools and current advisory data' },
    { id: 'dependency-license-analysis', mode: 'optional-external', status: 'not-run', detail: 'transitive dependency license detection and policy evaluation require an explicit SCA toolchain' },
    { id: 'sbom-generation', mode: 'optional-external', status: 'not-run', detail: 'CycloneDX/SPDX generation requires an explicit release toolchain' },
    { id: 'hosted-dependency-review', mode: 'blocked-external', status: 'not-run', detail: 'GitHub Dependency Review requires repository hosting and service configuration' }
  ];

  return { checks, external, ok: checks.every(check => check.status === 'pass') };
}

if (require.main === module) {
  const result = inspectSupplyChain();
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  if (!result.ok) process.exitCode = 1;
}

module.exports = { compareVersions, inspectSupplyChain, projectRoot };
