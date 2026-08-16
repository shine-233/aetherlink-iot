#!/usr/bin/env node
/**
 * 离线生成本仓库的本地 SBOM。
 *
 * 本工具只读取固定的 Go 模块清单、Go 校验和文件和前端 pnpm 锁文件，
 * 不联网、不安装依赖，也不遍历 node_modules。结果覆盖声明与仓库内锁定的
 * 组件，但不声称已完成 Go module graph 选择、registry enrichment 或部署等价性证明。
 */
'use strict';

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const TOOL_NAME = 'aetherlink-local-sbom-generator';
const TOOL_VERSION = '1.1.0';
const REPOSITORY_ROOT = path.resolve(__dirname, '..', '..');
const PREFERRED_OUTPUT = 'verification/local-sbom.json';
const IGNORED_OUTPUT = '_localrun/sbom/local-sbom.json';
const SOURCE_FILES = [
  { relativePath: 'backend/go.mod', ecosystem: 'go', lockfile: 'backend/go.sum' },
  { relativePath: 'backend/go.sum', ecosystem: 'go-lock' },
  { relativePath: 'mqtt-broker/go.mod', ecosystem: 'go', lockfile: 'mqtt-broker/go.sum' },
  { relativePath: 'mqtt-broker/go.sum', ecosystem: 'go-lock' },
  {
    relativePath: 'backend/cmd/aetherlink-device-autotest/go.mod',
    ecosystem: 'go',
    lockfile: 'backend/cmd/aetherlink-device-autotest/go.sum'
  },
  { relativePath: 'backend/cmd/aetherlink-device-autotest/go.sum', ecosystem: 'go-lock' },
  { relativePath: 'frontend/pnpm-lock.yaml', ecosystem: 'pnpm' }
];

class CliError extends Error {
  constructor(message, exitCode) {
    super(message);
    this.exitCode = exitCode;
  }
}

function printHelp() {
  process.stdout.write(`用法: node automation_tests/scripts/generate_local_sbom.js [选项]\n\n选项:\n  --output path   输出仓库内的 JSON 路径\n  --source-only   只输出四个源码/模块组件，不展开清单中声明的第三方组件\n  -h, --help      显示帮助\n\n默认输出为 ${PREFERRED_OUTPUT}；如果该路径被 Git 忽略，则自动改用\n${IGNORED_OUTPUT}，避免在源码边界产生未跟踪文件。工具全程离线，\n不会安装依赖或扫描 node_modules。\n`);
}

function parseArguments(argv) {
  const options = { output: null, sourceOnly: false, help: false };

  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === '-h' || argument === '--help') {
      options.help = true;
    } else if (argument === '--source-only') {
      if (options.sourceOnly) throw new CliError('参数 --source-only 不能重复。', 2);
      options.sourceOnly = true;
    } else if (argument === '--output') {
      if (options.output !== null) throw new CliError('参数 --output 不能重复。', 2);
      const value = argv[index + 1];
      if (!value || value.startsWith('-')) throw new CliError('参数 --output 缺少路径。', 2);
      options.output = value;
      index += 1;
    } else if (argument.startsWith('--output=')) {
      if (options.output !== null) throw new CliError('参数 --output 不能重复。', 2);
      options.output = argument.slice('--output='.length);
      if (!options.output) throw new CliError('参数 --output 缺少路径。', 2);
    } else {
      throw new CliError(`未知参数: ${argument}`, 2);
    }
  }

  return options;
}

function normalizeRelativePath(value) {
  return value.replace(/\\/g, '/').replace(/^\.\//, '');
}

function isInsideRepository(absolutePath) {
  const relative = path.relative(REPOSITORY_ROOT, absolutePath);
  return relative !== '' && relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

function isSensitiveOutput(relativePath) {
  const normalized = normalizeRelativePath(relativePath).toLowerCase();
  const parts = normalized.split('/');
  const basename = parts[parts.length - 1];
  const envFile = (basename === '.env' || basename.endsWith('.env')) &&
    !/\.(example|sample|template)$/.test(basename);

  return envFile ||
    parts.some(part => ['secret', 'secrets', 'credential', 'credentials', 'private_key', 'private-key', 'rsa_key'].includes(part)) ||
    /(^|\/)(id_rsa|private[_-]?key)(\.|$)/.test(normalized) ||
    /\.(pem|key|p12|pfx|jks|keystore)$/.test(basename);
}

function gitIgnores(relativePath) {
  const result = spawnSync('git', ['check-ignore', '-q', '--', relativePath], {
    cwd: REPOSITORY_ROOT,
    stdio: 'ignore',
    windowsHide: true
  });
  return result.status === 0;
}

function defaultOutputPath() {
  return gitIgnores(PREFERRED_OUTPUT) ? IGNORED_OUTPUT : PREFERRED_OUTPUT;
}

function validateOutputPath(value) {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new CliError('输出路径不能为空。', 2);
  }
  if (value.includes('\0')) throw new CliError('输出路径包含非法空字符。', 2);

  const absolutePath = path.resolve(REPOSITORY_ROOT, value);
  if (!isInsideRepository(absolutePath)) {
    throw new CliError('输出路径越界：必须位于 repository 内，不能指向 repository root 或 outside repository。', 2);
  }

  const relativePath = normalizeRelativePath(path.relative(REPOSITORY_ROOT, absolutePath));
  if (isSensitiveOutput(relativePath)) {
    throw new CliError('拒绝写入疑似 secrets、.env 或私钥路径。', 2);
  }
  if (SOURCE_FILES.some(source => source.relativePath === relativePath)) {
    throw new CliError('输出路径不能覆盖输入清单。', 2);
  }
  if (path.extname(absolutePath).toLowerCase() !== '.json') {
    throw new CliError('输出路径必须使用 .json 扩展名。', 2);
  }

  return { absolutePath, relativePath };
}

function assertNoSymlinkEscape(outputPath) {
  let current = path.dirname(outputPath);
  const missing = [];

  while (!fs.existsSync(current)) {
    missing.push(current);
    const parent = path.dirname(current);
    if (parent === current) break;
    current = parent;
  }

  let realParent;
  try {
    realParent = fs.realpathSync(current);
  } catch (error) {
    throw new CliError(`无法检查输出目录: ${error.message}`, 4);
  }
  if (realParent !== REPOSITORY_ROOT && !isInsideRepository(realParent)) {
    throw new CliError('输出目录经符号链接解析后位于仓库外。', 4);
  }

  if (fs.existsSync(outputPath)) {
    let realOutput;
    try {
      realOutput = fs.realpathSync(outputPath);
    } catch (error) {
      throw new CliError(`无法检查现有输出文件: ${error.message}`, 4);
    }
    if (!isInsideRepository(realOutput)) {
      throw new CliError('现有输出文件经符号链接解析后位于仓库外。', 4);
    }
  }

  return missing;
}

function readSourceFiles() {
  return SOURCE_FILES.map(source => {
    const absolutePath = path.join(REPOSITORY_ROOT, ...source.relativePath.split('/'));
    let content;
    try {
      content = fs.readFileSync(absolutePath);
    } catch (error) {
      throw new CliError(`读取 ${source.relativePath} 失败: ${error.message}`, 3);
    }
    return {
      ...source,
      content,
      text: content.toString('utf8'),
      sha256: crypto.createHash('sha256').update(content).digest('hex')
    };
  });
}

function stableCompare(left, right) {
  return left < right ? -1 : left > right ? 1 : 0;
}

function property(name, value) {
  return { name, value: String(value) };
}

function encodePurlPart(value) {
  return value.split('/').map(part => encodeURIComponent(part)).join('/');
}

function goPurl(name, version) {
  return `pkg:golang/${encodePurlPart(name)}${version ? `@${encodeURIComponent(version)}` : ''}`;
}

function npmPurl(name, version) {
  const encodedName = name.startsWith('@')
    ? `${encodeURIComponent(name.slice(0, name.indexOf('/')))}/${encodeURIComponent(name.slice(name.indexOf('/') + 1))}`
    : encodeURIComponent(name);
  return `pkg:npm/${encodedName}${version ? `@${encodeURIComponent(version)}` : ''}`;
}

function parseGoMod(source) {
  const moduleMatch = source.text.match(/^module\s+(\S+)\s*$/m);
  if (!moduleMatch) throw new CliError(`${source.relativePath} 缺少 module 声明。`, 3);

  const dependencies = [];
  const lines = source.text.split(/\r?\n/);
  let inRequireBlock = false;
  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (line === 'require (') {
      inRequireBlock = true;
      continue;
    }
    if (inRequireBlock && line === ')') {
      inRequireBlock = false;
      continue;
    }

    let declaration = null;
    if (inRequireBlock) declaration = line;
    else if (line.startsWith('require ')) declaration = line.slice('require '.length).trim();
    if (!declaration || declaration.startsWith('//')) continue;

    const indirect = /\/\/\s*indirect\s*$/.test(declaration);
    const clean = declaration.replace(/\s*\/\/.*$/, '').trim();
    const match = clean.match(/^(\S+)\s+(\S+)$/);
    if (!match) throw new CliError(`${source.relativePath} 中存在无法解析的 require 声明: ${line}`, 3);
    dependencies.push({ name: match[1], version: match[2], indirect });
  }

  return { moduleName: moduleMatch[1], dependencies };
}

function parseGoSum(source) {
  const entries = new Map();

  for (const rawLine of source.text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith('//')) continue;

    const match = line.match(/^(\S+)\s+(\S+)\s+(h1:\S+)$/);
    if (!match) continue;

    const moduleName = match[1];
    const isGoModChecksum = match[2].endsWith('/go.mod');
    const version = isGoModChecksum ? match[2].slice(0, -'/go.mod'.length) : match[2];
    const key = `${moduleName}@${version}`;
    const existing = entries.get(key) || {
      name: moduleName,
      version,
      integrities: new Set()
    };
    existing.integrities.add(`${isGoModChecksum ? 'go.mod' : 'module'}:${match[3]}`);
    entries.set(key, existing);
  }

  return [...entries.values()];
}

function unquoteYamlKey(value) {
  const trimmed = value.trim();
  if ((trimmed.startsWith("'") && trimmed.endsWith("'")) ||
      (trimmed.startsWith('"') && trimmed.endsWith('"'))) {
    return trimmed.slice(1, -1).replace(/''/g, "'");
  }
  return trimmed;
}

function yamlKeyAtIndent(line, spaces) {
  const match = line.match(new RegExp(`^ {${spaces}}(.+):\\s*$`));
  return match ? unquoteYamlKey(match[1]) : null;
}

function packageNameAndVersion(lockKey) {
  const normalized = lockKey.replace(/^\//, '');
  const separator = normalized.lastIndexOf('@');
  if (separator <= 0) return null;

  const name = normalized.slice(0, separator);
  const version = normalized.slice(separator + 1).replace(/\(.+$/, '');
  if (!name || !version || /^(link|workspace|file):/.test(version)) return null;
  return { name, version };
}

function parsePnpmLock(source) {
  const lines = source.text.split(/\r?\n/);
  const direct = new Map();
  const packages = [];
  let topLevel = '';
  let importer = '';
  let dependencyGroup = '';

  for (const line of lines) {
    if (/^[A-Za-z][^:]*:\s*$/.test(line)) {
      topLevel = line.slice(0, line.indexOf(':'));
      importer = '';
      dependencyGroup = '';
      continue;
    }

    if (topLevel === 'importers') {
      const importerKey = yamlKeyAtIndent(line, 2);
      if (importerKey !== null) {
        importer = importerKey;
        dependencyGroup = '';
        continue;
      }
      const group = yamlKeyAtIndent(line, 4);
      if (group !== null) {
        dependencyGroup = ['dependencies', 'devDependencies', 'optionalDependencies'].includes(group) ? group : '';
        continue;
      }
      const dependencyName = yamlKeyAtIndent(line, 6);
      if (importer && dependencyGroup && dependencyName !== null) {
        const key = dependencyName;
        const existing = direct.get(key) || new Set();
        existing.add(`${importer}:${dependencyGroup}`);
        direct.set(key, existing);
      }
    } else if (topLevel === 'packages') {
      const packageKey = yamlKeyAtIndent(line, 2);
      if (packageKey !== null) {
        const parsed = packageNameAndVersion(packageKey);
        if (parsed) packages.push(parsed);
      }
    }
  }

  if (packages.length === 0) throw new CliError(`${source.relativePath} 未解析到 packages 组件。`, 3);
  return { direct, packages };
}

function sourceComponent(name, source, purl) {
  return {
    type: 'application',
    'bom-ref': `source:${source.relativePath}`,
    name,
    scope: 'required',
    hashes: [{ alg: 'SHA-256', content: source.sha256 }],
    purl,
    properties: [
      property('source.file', source.relativePath),
      property('source.sha256', source.sha256),
      property('component.origin', 'source-manifest')
    ]
  };
}

function dependencyComponent(type, name, version, purl, sourceFiles, extraProperties) {
  return {
    type,
    'bom-ref': purl,
    name,
    version,
    scope: 'required',
    purl,
    properties: [
      property('source.files', [...sourceFiles].sort(stableCompare).join(',')),
      property('component.origin', 'declared-or-locked'),
      ...extraProperties
    ].sort((left, right) => stableCompare(left.name, right.name))
  };
}

function buildComponents(sources, sourceOnly) {
  const components = [];
  const dependencyMap = new Map();

  for (const source of sources.filter(item => item.ecosystem === 'go')) {
    const parsed = parseGoMod(source);
    const lockSource = sources.find(item => item.relativePath === source.lockfile);
    if (!lockSource) throw new CliError(`${source.relativePath} 缺少对应的 Go 校验和文件。`, 3);
    const lockEntries = parseGoSum(lockSource);
    components.push(sourceComponent(parsed.moduleName, source, goPurl(parsed.moduleName, '')));
    if (sourceOnly) continue;

    for (const dependency of parsed.dependencies) {
      const purl = goPurl(dependency.name, dependency.version);
      const existing = dependencyMap.get(purl) || {
        type: 'library', name: dependency.name, version: dependency.version, purl,
        sourceFiles: new Set(), relationships: new Set(), integrities: new Set()
      };
      existing.sourceFiles.add(source.relativePath);
      existing.relationships.add(dependency.indirect ? 'indirect-declaration' : 'direct-declaration');
      dependencyMap.set(purl, existing);
    }

    for (const dependency of lockEntries) {
      const purl = goPurl(dependency.name, dependency.version);
      const existing = dependencyMap.get(purl) || {
        type: 'library', name: dependency.name, version: dependency.version, purl,
        sourceFiles: new Set(), relationships: new Set(), integrities: new Set()
      };
      existing.sourceFiles.add(lockSource.relativePath);
      existing.relationships.add('go.sum-entry');
      for (const integrity of dependency.integrities) existing.integrities.add(integrity);
      dependencyMap.set(purl, existing);
    }
  }

  const pnpmSource = sources.find(item => item.ecosystem === 'pnpm');
  const pnpm = parsePnpmLock(pnpmSource);
  components.push(sourceComponent('aetherlink-frontend', pnpmSource, 'pkg:npm/aetherlink-frontend'));

  if (!sourceOnly) {
    for (const dependency of pnpm.packages) {
      const purl = npmPurl(dependency.name, dependency.version);
      const existing = dependencyMap.get(purl) || {
        type: 'library', name: dependency.name, version: dependency.version, purl,
        sourceFiles: new Set(), relationships: new Set(), integrities: new Set()
      };
      existing.sourceFiles.add(pnpmSource.relativePath);
      const directDeclarations = pnpm.direct.get(dependency.name);
      if (directDeclarations) {
        for (const declaration of directDeclarations) existing.relationships.add(`direct:${declaration}`);
      } else {
        existing.relationships.add('lockfile-entry');
      }
      dependencyMap.set(purl, existing);
    }

    for (const item of dependencyMap.values()) {
      const properties = [
        property('declaration.relationship', [...item.relationships].sort(stableCompare).join(','))
      ];
      if (item.integrities && item.integrities.size > 0) {
        properties.push(property('go.sum.integrity', [...item.integrities].sort(stableCompare).join(',')));
      }
      components.push(dependencyComponent(
        item.type,
        item.name,
        item.version,
        item.purl,
        item.sourceFiles,
        properties
      ));
    }
  }

  return components.sort((left, right) =>
    stableCompare(left['bom-ref'], right['bom-ref']) || stableCompare(left.name, right.name)
  );
}

function buildBom(sources, sourceOnly) {
  const sourceSummary = sources
    .map(source => `${source.relativePath}=sha256:${source.sha256}`)
    .sort(stableCompare)
    .join(';');
  const completeness = sourceOnly ? 'source-manifest-only' : 'declared-and-locked-components';

  return {
    bomFormat: 'CycloneDX',
    specVersion: '1.6',
    version: 1,
    metadata: {
      timestamp: new Date().toISOString(),
      tools: {
        components: [{
          type: 'application',
          name: TOOL_NAME,
          version: TOOL_VERSION,
          properties: [
            property('execution.network', 'not-run'),
            property('execution.package-install', 'not-run'),
            property('execution.node_modules-scan', 'not-run')
          ]
        }]
      },
      properties: [
        property('aetherlink:sbom:scope', completeness),
        property('completeness', completeness),
        property('scope', sourceOnly ? 'source-components-only' : 'declared-and-locked-components'),
        property('external.dependency-resolution', 'not-run'),
        property('external.registry-enrichment', 'not-run'),
        property('external.container-attestation', 'not-run'),
        property('source.files.sha256', sourceSummary),
        property('limitations', 'No claim of complete Go module graph selection, registry enrichment, or deployed artifact equivalence.')
      ]
    },
    components: buildComponents(sources, sourceOnly)
  };
}

function writeBom(output, bom) {
  assertNoSymlinkEscape(output.absolutePath);
  try {
    fs.mkdirSync(path.dirname(output.absolutePath), { recursive: true });
    fs.writeFileSync(output.absolutePath, `${JSON.stringify(bom, null, 2)}\n`, {
      encoding: 'utf8',
      flag: 'w',
      mode: 0o600
    });
  } catch (error) {
    throw new CliError(`写入 ${output.relativePath} 失败: ${error.message}`, 4);
  }
}

function main() {
  const options = parseArguments(process.argv.slice(2));
  if (options.help) {
    printHelp();
    return;
  }

  const output = validateOutputPath(options.output || defaultOutputPath());
  const sources = readSourceFiles();
  const bom = buildBom(sources, options.sourceOnly);
  writeBom(output, bom);
  const completeness = bom.metadata.properties.find(item => item.name === 'completeness');
  process.stdout.write(`已生成 ${output.relativePath}（${bom.components.length} 个组件，completeness=${completeness.value}）。\n`);
}

try {
  main();
} catch (error) {
  const exitCode = error instanceof CliError ? error.exitCode : 1;
  process.stderr.write(`generate_local_sbom: ${error.message}\n`);
  process.exitCode = exitCode;
}
