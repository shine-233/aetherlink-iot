#!/usr/bin/env node
/**
 * 文件用途：只读检查本地运行产物、构建输出、缓存、归档和二进制的版本控制边界。
 * 核心逻辑：枚举已知生成物，通过 Git tracked/ignored 状态区分本地默认产物与需人工复核项。
 * 关键注意事项：本脚本不删除、移动或改写任何文件；外部归档内容与保留期限不在本地门禁中猜测。
 */
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const projectRoot = path.resolve(__dirname, '..', '..');
const skippedDirectories = new Set(['.git', 'node_modules']);
const archiveExtensions = new Set(['.zip', '.tar', '.tgz', '.gz', '.7z', '.rar']);
const binaryExtensions = new Set(['.exe', '.dll', '.so', '.dylib']);

function toRelative(absolutePath) {
  return path.relative(projectRoot, absolutePath).split(path.sep).join('/');
}

function isGeneratedCandidate(relativePath) {
  const normalized = `/${relativePath}`;
  const extension = path.extname(relativePath).toLowerCase();
  const publicInstanceFiles = new Set([
    '/_localrun_instance_b/README.md',
    '/_localrun_instance_b/instance-b.env.example'
  ]);
  if (publicInstanceFiles.has(normalized)) return false;
  return normalized.includes('/_localrun/')
    || normalized.startsWith('/_localrun/')
    || normalized.startsWith('/_localrun_instance_b/')
    || normalized.startsWith('/.playwright-cli/')
    || normalized.startsWith('/frontend/dist/')
    || normalized.startsWith('/frontend/dist-lite/')
    || normalized.startsWith('/frontend/output/')
    || normalized.startsWith('/automation_tests/output/')
    || relativePath.endsWith('.tsbuildinfo')
    || (normalized.startsWith('/verification/') && archiveExtensions.has(extension))
    || binaryExtensions.has(extension);
}

function collectCandidates(directory = projectRoot, output = []) {
  if (path.resolve(directory) === projectRoot && output.length === 0) {
    const gitCandidates = collectCandidatesFromGit();
    if (gitCandidates) return gitCandidates;
  }

  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.isSymbolicLink() || (entry.isDirectory() && skippedDirectories.has(entry.name))) continue;
    const absolutePath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      collectCandidates(absolutePath, output);
      continue;
    }
    if (!entry.isFile()) continue;
    const relativePath = toRelative(absolutePath);
    if (isGeneratedCandidate(relativePath)) {
      output.push({ path: relativePath, size: fs.statSync(absolutePath).size });
    }
  }
  return output.sort((left, right) => left.path.localeCompare(right.path));
}

function runGit(args, input) {
  return spawnSync('git', args, {
    cwd: projectRoot,
    encoding: 'utf8',
    input,
    maxBuffer: 128 * 1024 * 1024
  });
}

function splitNul(value) {
  return String(value || '').split('\0').filter(Boolean).map(item => item.replaceAll('\\', '/'));
}

function collectCandidatesFromGit() {
  const candidatePaths = new Set();
  const inventories = [
    ['ls-files', '--cached', '--others', '--exclude-standard', '-z'],
    ['ls-files', '--others', '--ignored', '--exclude-standard', '-z']
  ];

  for (const args of inventories) {
    const result = runGit(args);
    if (result.error || result.status !== 0) return null;
    for (const relativePath of splitNul(result.stdout)) {
      const pathParts = relativePath.split('/');
      if (pathParts.includes('.git') || pathParts.includes('node_modules')) continue;
      if (isGeneratedCandidate(relativePath)) candidatePaths.add(relativePath);
    }
  }

  const candidates = [];
  for (const relativePath of candidatePaths) {
    try {
      const stat = fs.statSync(path.join(projectRoot, relativePath));
      if (stat.isFile()) candidates.push({ path: relativePath, size: stat.size });
    } catch (error) {
      if (error && error.code === 'ENOENT') continue;
      return null;
    }
  }

  return candidates.sort((left, right) => left.path.localeCompare(right.path));
}

function inspectGeneratedArtifacts() {
  const candidates = collectCandidates();
  const trackedResult = runGit(['ls-files', '-z']);
  if (trackedResult.error || trackedResult.status !== 0) {
    return {
      ok: false,
      summary: { candidates: candidates.length, localDefault: 0, reviewRequired: 0 },
      localDefault: [],
      reviewRequired: [],
      external: [{
        id: 'git-metadata',
        mode: 'blocked-external',
        status: 'not-run',
        detail: trackedResult.error?.message || trackedResult.stderr || 'git ls-files failed'
      }]
    };
  }

  const tracked = new Set(splitNul(trackedResult.stdout));
  const candidatePaths = candidates.map(candidate => candidate.path);
  const ignoredResult = candidatePaths.length
    ? runGit(['check-ignore', '-z', '--stdin'], `${candidatePaths.join('\0')}\0`)
    : { status: 1, stdout: '', stderr: '' };
  const gitIgnoreFailed = ignoredResult.error || ![0, 1].includes(ignoredResult.status);
  if (gitIgnoreFailed) {
    return {
      ok: false,
      summary: { candidates: candidates.length, localDefault: 0, reviewRequired: 0 },
      localDefault: [],
      reviewRequired: [],
      external: [{
        id: 'git-ignore-metadata',
        mode: 'blocked-external',
        status: 'not-run',
        detail: ignoredResult.error?.message || ignoredResult.stderr || 'git check-ignore failed'
      }]
    };
  }

  const ignored = new Set(splitNul(ignoredResult.stdout));
  const localDefault = [];
  const reviewRequired = [];
  for (const candidate of candidates) {
    const item = {
      ...candidate,
      tracked: tracked.has(candidate.path),
      ignored: ignored.has(candidate.path)
    };
    if (!item.tracked && item.ignored) localDefault.push({ ...item, mode: 'local-default', status: 'pass' });
    else reviewRequired.push({ ...item, mode: 'review-required', status: 'fail' });
  }

  return {
    ok: reviewRequired.length === 0,
    summary: {
      candidates: candidates.length,
      bytes: candidates.reduce((total, candidate) => total + candidate.size, 0),
      localDefault: localDefault.length,
      reviewRequired: reviewRequired.length
    },
    localDefault,
    reviewRequired,
    external: [{
      id: 'artifact-retention-and-archive-content',
      mode: 'optional-external',
      status: 'not-run',
      detail: 'retention approval and archive-content review require an explicit owner and release context'
    }]
  };
}

if (require.main === module) {
  const result = inspectGeneratedArtifacts();
  const output = {
    ok: result.ok,
    summary: result.summary,
    reviewRequired: result.reviewRequired,
    external: result.external
  };
  process.stdout.write(`${JSON.stringify(output, null, 2)}\n`);
  if (!result.ok) process.exitCode = 1;
}

module.exports = { collectCandidates, inspectGeneratedArtifacts, isGeneratedCandidate, projectRoot };
