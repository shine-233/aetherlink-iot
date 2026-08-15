/**
 * 本地 source-manifest SBOM 契约。
 *
 * 该测试只使用 Node.js 标准库和临时输出目录，不联网、不调用外部 SBOM 工具；
 * 完整 resolved dependency/image SBOM 仍属于发布环境的 optional-external 能力。
 */
const { expect } = require('chai');
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const projectRoot = path.resolve(__dirname, '..', '..');
const automationRoot = path.join(projectRoot, 'automation_tests');
const generatorPath = path.join(automationRoot, 'scripts', 'generate_local_sbom.js');
const manifestPaths = [
  'backend/go.mod',
  'backend/cmd/aetherlink-device-autotest/go.mod',
  'mqtt-broker/go.mod',
  'frontend/pnpm-lock.yaml'
];

function sha256(relativePath) {
  return crypto
    .createHash('sha256')
    .update(fs.readFileSync(path.join(projectRoot, ...relativePath.split('/'))))
    .digest('hex');
}

function runGenerator(outputArgument) {
  return spawnSync(process.execPath, [generatorPath, '--source-only', '--output', outputArgument], {
    cwd: projectRoot,
    encoding: 'utf8',
    windowsHide: true
  });
}

function sourcePath(component) {
  const property = (component.properties || []).find(item => item.name === 'source.file');
  return property ? property.value : component.name;
}

function normalizedBom(bom) {
  const copy = JSON.parse(JSON.stringify(bom));
  if (copy.metadata) delete copy.metadata.timestamp;
  return copy;
}

describe('local source-manifest SBOM [00_local_sbom]', function () {
  this.timeout(30000);

  let temporaryDirectory;

  beforeEach(function () {
    temporaryDirectory = fs.mkdtempSync(path.join(automationRoot, '.local-sbom-test-'));
  });

  afterEach(function () {
    fs.rmSync(temporaryDirectory, { recursive: true, force: true });
  });

  it('writes a CycloneDX-like source-manifest-only BOM with SHA-256 hashes', function () {
    const outputPath = path.join(temporaryDirectory, 'local-sbom.json');
    const result = runGenerator(path.relative(projectRoot, outputPath));

    expect(result.status, result.stderr || result.stdout).to.equal(0);
    expect(fs.existsSync(outputPath)).to.equal(true);

    const bom = JSON.parse(fs.readFileSync(outputPath, 'utf8'));
    expect(bom).to.include({ bomFormat: 'CycloneDX', specVersion: '1.6', version: 1 });
    expect(bom.metadata).to.be.an('object');
    expect(new Date(bom.metadata.timestamp).toISOString()).to.equal(bom.metadata.timestamp);
    expect(bom.metadata.properties).to.deep.include({
      name: 'completeness',
      value: 'source-manifest-only'
    });
    expect(bom.metadata.properties).to.deep.include({
      name: 'scope',
      value: 'source-components-only'
    });

    expect(bom.components).to.be.an('array').with.length(manifestPaths.length);
    const componentsByPath = new Map(bom.components.map(component => [sourcePath(component), component]));
    expect([...componentsByPath.keys()].sort()).to.deep.equal([...manifestPaths].sort());

    for (const relativePath of manifestPaths) {
      const component = componentsByPath.get(relativePath);
      expect(component, relativePath).to.be.an('object');
      expect(component.hashes).to.deep.include({ alg: 'SHA-256', content: sha256(relativePath) });
    }

    expect([...componentsByPath.keys()].filter(value => value.endsWith('/go.mod'))).to.have.length(3);
    expect(componentsByPath.has('frontend/pnpm-lock.yaml')).to.equal(true);
  });

  it('rejects an output path that escapes the repository root', function () {
    const escapedName = `aetherlink-sbom-escape-${process.pid}-${Date.now()}.json`;
    const escapedPath = path.resolve(projectRoot, '..', escapedName);
    const result = runGenerator(`../${escapedName}`);

    try {
      expect(result.status).not.to.equal(0);
      expect(fs.existsSync(escapedPath)).to.equal(false);
      expect(`${result.stderr}\n${result.stdout}`).to.match(/outside|escape|repository|越界/i);
    } finally {
      fs.rmSync(escapedPath, { force: true });
    }
  });

  it('rejects an absolute output path outside the repository root', function () {
    const escapedName = `aetherlink-sbom-absolute-${process.pid}-${Date.now()}.json`;
    const escapedPath = path.resolve(projectRoot, '..', escapedName);
    const result = runGenerator(escapedPath);

    try {
      expect(result.status).not.to.equal(0);
      expect(fs.existsSync(escapedPath)).to.equal(false);
      expect(`${result.stderr}\n${result.stdout}`).to.match(/outside|escape|repository|越界/i);
    } finally {
      fs.rmSync(escapedPath, { force: true });
    }
  });

  it('rejects the repository root itself as an output path', function () {
    const result = runGenerator('.');

    expect(result.status).not.to.equal(0);
    expect(`${result.stderr}\n${result.stdout}`).to.match(/outside|escape|repository|越界/i);
  });

  it('is stable across repeated runs except for metadata.timestamp', function () {
    const firstPath = path.join(temporaryDirectory, 'first.json');
    const secondPath = path.join(temporaryDirectory, 'second.json');
    const first = runGenerator(path.relative(projectRoot, firstPath));
    const second = runGenerator(path.relative(projectRoot, secondPath));

    expect(first.status, first.stderr || first.stdout).to.equal(0);
    expect(second.status, second.stderr || second.stdout).to.equal(0);

    const firstBom = JSON.parse(fs.readFileSync(firstPath, 'utf8'));
    const secondBom = JSON.parse(fs.readFileSync(secondPath, 'utf8'));
    expect(normalizedBom(secondBom)).to.deep.equal(normalizedBom(firstBom));
  });
});
