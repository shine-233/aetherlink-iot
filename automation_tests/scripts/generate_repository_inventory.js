#!/usr/bin/env node
/**
 * 生成当前仓库的可复核文件台账。
 *
 * 台账覆盖 Git tracked（含工作树缺失）、modified 和未忽略的 untracked 文件；
 * ignored 依赖或运行产物仅记录 Git 返回的折叠边界路径，不遍历或读取其内容。
 * 台账输出文件本身不计入输入集合，避免输出大小和行数造成不可收敛的自引用。
 * 敏感候选只记录文件元数据，不读取内容。
 */
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

const projectRoot = path.resolve(__dirname, '..', '..');
const csvRelativePath = 'references/repository-file-inventory.csv';
const summaryRelativePath = 'references/repository-file-inventory-summary.md';
const excludedOutputPaths = new Set([csvRelativePath, summaryRelativePath]);
const reviewedDeletionReasons = new Map([
  [
    'automation_tests/e2e/route_smoke.js',
    'removed weak route-shell helper; API-backed browser assertions now live in the owning E2E specs'
  ],
  [
    'frontend/src/core/SystemInitializer.ts',
    'removed unused orchestration layer; frontend/src/main.ts is the active direct bootstrap and no source import remains'
  ]
]);

function runGit(args) {
  return execFileSync('git', args, {
    cwd: projectRoot,
    encoding: 'buffer',
    maxBuffer: 64 * 1024 * 1024
  });
}

function parseNullSeparated(buffer) {
  return buffer
    .toString('utf8')
    .split('\0')
    .filter(Boolean)
    .map(value => value.replace(/\\/g, '/'));
}

function listRepositoryPaths() {
  const tracked = new Set(parseNullSeparated(runGit(['ls-files', '-z'])));
  const untracked = new Set(parseNullSeparated(runGit(['ls-files', '--others', '--exclude-standard', '-z'])));
  const changed = new Set(parseNullSeparated(runGit(['diff', 'HEAD', '--name-only', '-z'])));
  const paths = new Set([...tracked, ...untracked]);

  for (const outputPath of excludedOutputPaths) paths.delete(outputPath);

  return [...paths].sort((left, right) => left.localeCompare(right, 'en')).map(relativePath => ({
    path: relativePath,
    tracked: tracked.has(relativePath),
    untracked: untracked.has(relativePath),
    changed: changed.has(relativePath)
  }));
}

function ignoredBoundaryCategory(relativePath) {
  const normalized = relativePath.replace(/\/$/, '');
  const parts = normalized.toLowerCase().split('/');
  const basename = parts[parts.length - 1];

  if (isSensitivePath(normalized) || parts.includes('.auth') || parts.includes('certs')) {
    return 'local-sensitive-config';
  }
  if (parts.includes('node_modules') || parts.includes('vendor')) return 'dependency-tree';
  if (/^project_(cleanup_plan|folder_(audit|contents))/.test(basename)) return 'local-audit-evidence';
  if (
    parts.some(part => ['_localrun', '_localrun_instance_b', 'reports', 'verification', 'playwright-report', 'test-results', '.playwright-cli'].includes(part))
  ) return 'runtime-or-report';
  return 'generated-output';
}

function ignoredBoundaryRecommendation(category) {
  if (category === 'dependency-tree') return 'reinstall-from-lockfile-do-not-review-as-source';
  if (category === 'local-sensitive-config') return 'keep-local-or-template-only';
  if (category === 'local-audit-evidence') return 'preserve-at-captured-path-until-closeout';
  if (category === 'runtime-or-report') return 'preserve-needed-evidence-then-review-cleanup';
  return 'regenerate-do-not-commit';
}

function listIgnoredBoundaries() {
  const paths = new Set(parseNullSeparated(runGit([
    'ls-files',
    '--others',
    '--ignored',
    '--exclude-standard',
    '--directory',
    '-z'
  ])));

  return [...paths]
    .sort((left, right) => left.localeCompare(right, 'en'))
    .map(relativePath => {
      const category = ignoredBoundaryCategory(relativePath);
      return {
        path: relativePath,
        module: moduleFor(relativePath),
        category,
        sensitive: isSensitivePath(relativePath) || category === 'local-sensitive-config',
        recommendation: ignoredBoundaryRecommendation(category)
      };
    });
}

function isSensitivePath(relativePath) {
  const basename = path.posix.basename(relativePath).toLowerCase();
  return (
    ((basename.startsWith('.env') || basename.endsWith('.env')) && !/\.(example|sample|template)$/.test(basename)) ||
    /(^|\/)(credentials?|secrets?)(\/|\.|$)/i.test(relativePath) ||
    /(^|\/)(private[_-]?key|id_rsa)(\.|$)/i.test(relativePath) ||
    /\.(p12|pfx|jks|keystore)$/i.test(relativePath)
  );
}

function moduleFor(relativePath) {
  const first = relativePath.split('/')[0];
  const knownModules = new Set([
    'frontend',
    'backend',
    'mqtt-broker',
    'automation_tests',
    'deploy',
    'references',
    'verification',
    'audit_reports',
    'performance'
  ]);
  return knownModules.has(first) ? first : 'root';
}

function categoryFor(relativePath) {
  const extension = path.posix.extname(relativePath).toLowerCase();
  const basename = path.posix.basename(relativePath).toLowerCase();

  // Coverage reporters commonly emit `lcov.info`. It is generated evidence,
  // not an ambiguous source/configuration file, even when it lives outside a
  // module's conventional `coverage/` directory (for example in a clean-room
  // evidence bundle).
  if (basename === 'lcov.info' || extension === '.lcov') return 'generated-output';
  if (/\.gen\.go$/.test(relativePath) || /\.pb\.go$/.test(relativePath) || /(^|\/)backend\/docs\/(docs\.go|swagger\.(json|yaml))$/.test(relativePath)) return 'generated-source';
  if (/(^|\/)(third_party|vendor)(\/|$)/.test(relativePath)) return 'third-party';
  if (/(^|\/)(__tests__|tests?|e2e)(\/|$)/.test(relativePath) || /\.(test|spec)\.[^.]+$/.test(relativePath)) return 'test';
  if (/(^|\/)(docs?|references|verification|audit_reports)(\/|$)/.test(relativePath) || extension === '.md') return 'documentation';
  if (['package-lock.json', 'pnpm-lock.yaml', 'go.sum'].includes(basename)) return 'lockfile';
  if (
    ['.yml', '.yaml', '.toml', '.ini', '.conf'].includes(extension) ||
    /(^|\/)(dockerfile|makefile)$/i.test(relativePath) ||
    /(^|\/)(package\.json|go\.mod|tsconfig[^/]*\.json)$/.test(relativePath) ||
    /(^|\/)(\.dockerignore|\.editorconfig|\.gitattributes|\.gitignore|\.npmrc|\.prettierrc)$/.test(relativePath) ||
    /(^|\/)[^/]*\.env\.(example|sample|template)$/.test(relativePath) ||
    /(^|\/)\.env\.(example|sample|template)$/.test(relativePath)
  ) return 'configuration';
  if (['.go', '.ts', '.tsx', '.vue', '.js', '.mjs', '.cjs', '.sh', '.ps1', '.sql', '.proto', '.cmd'].includes(extension)) return 'source';
  if (['.png', '.jpg', '.jpeg', '.gif', '.ico', '.woff', '.woff2', '.ttf', '.pdf', '.doc', '.docx', '.xls', '.xlsx'].includes(extension)) return 'binary-asset';
  if (['.json', '.csv', '.svg', '.html', '.css', '.scss', '.less', '.txt'].includes(extension) || basename === '.gitkeep') return 'data-or-asset';
  if (basename === 'license') return 'documentation';
  return 'other';
}

function recommendationFor(relativePath, category, gitStatus, sensitive) {
  if (gitStatus === 'deleted') {
    return reviewedDeletionReasons.has(relativePath) ? 'reviewed-deletion' : 'review-deletion';
  }
  if (sensitive) return 'keep-local-or-template-only';
  if (category === 'generated-source') return 'retain-until-reproducible';
  if (category === 'third-party') return 'preserve-contract-review-upstream';
  if (category === 'lockfile') return 'retain-generated-by-package-manager';
  if (category === 'binary-asset') return 'retain-if-referenced-review-size';
  return 'retain-and-review-in-module';
}

function inspectEntry(entry) {
  const absolutePath = path.join(projectRoot, ...entry.path.split('/'));
  const exists = fs.existsSync(absolutePath);
  const sensitive = isSensitivePath(entry.path);
  let bytes = 0;
  let lines = '';
  let contentKind = exists ? 'metadata-only' : 'missing';

  if (exists) {
    const stat = fs.statSync(absolutePath);
    bytes = stat.size;
    if (!sensitive && stat.isFile()) {
      const content = fs.readFileSync(absolutePath);
      if (content.includes(0)) {
        contentKind = 'binary';
      } else {
        contentKind = 'text';
        lines = content.length === 0 ? 0 : content.toString('utf8').split(/\r\n|\n|\r/).length;
      }
    } else if (sensitive) {
      contentKind = 'sensitive-metadata-only';
    }
  }

  const gitStatus = !exists && entry.tracked
    ? 'deleted'
    : entry.untracked
      ? 'untracked'
      : entry.changed
        ? 'modified'
        : 'tracked';
  const category = categoryFor(entry.path);

  return {
    path: entry.path,
    gitStatus,
    bytes,
    lines,
    contentKind,
    module: moduleFor(entry.path),
    category,
    sensitive,
    recommendation: recommendationFor(entry.path, category, gitStatus, sensitive),
    reviewReason: gitStatus === 'deleted' ? (reviewedDeletionReasons.get(entry.path) || '') : ''
  };
}

function csvCell(value) {
  const text = String(value ?? '');
  return `"${text.replace(/"/g, '""')}"`;
}

function renderCsv(entries) {
  const headers = ['path', 'git_status', 'bytes', 'lines', 'content_kind', 'module', 'category', 'sensitive', 'recommendation', 'review_reason'];
  const rows = entries.map(entry => [
    entry.path,
    entry.gitStatus,
    entry.bytes,
    entry.lines,
    entry.contentKind,
    entry.module,
    entry.category,
    entry.sensitive,
    entry.recommendation,
    entry.reviewReason
  ]);
  return [headers, ...rows].map(row => row.map(csvCell).join(',')).join('\n') + '\n';
}

function countBy(entries, field) {
  return entries.reduce((counts, entry) => {
    counts[entry[field]] = (counts[entry[field]] || 0) + 1;
    return counts;
  }, {});
}

function markdownCounts(counts) {
  return Object.entries(counts)
    .sort(([left], [right]) => left.localeCompare(right, 'en'))
    .map(([name, count]) => `| ${name} | ${count} |`)
    .join('\n');
}

function renderSummary(entries, ignoredBoundaries) {
  const generatedAt = new Date().toISOString();
  const deletedEntries = entries.filter(entry => entry.gitStatus === 'deleted');
  const unresolvedDeletionCount = deletedEntries.filter(entry => entry.recommendation === 'review-deletion').length;
  const deletionRows = deletedEntries.length === 0
    ? '- 当前工作树没有 tracked 删除项。'
    : deletedEntries.map(entry => `- \`${entry.path}\`：${entry.reviewReason || '尚未审查删除原因'}`).join('\n');

  return `# 当前仓库文件台账摘要\n\n` +
    `> 由 \`automation_tests/scripts/generate_repository_inventory.js\` 于 ${generatedAt} 生成。不要手工修改计数。\n\n` +
    `完整逐文件台账见 [repository-file-inventory.csv](./repository-file-inventory.csv)。台账覆盖 Git tracked（含缺失文件）和未忽略的 untracked 文件；ignored 依赖/运行产物仅登记 Git 折叠边界，不遍历或读取内容。两个台账输出文件自身不计入输入集合，以避免自引用。敏感候选只读取元数据。\n\n` +
    `源码边界总计：**${entries.length}** 个文件；ignored 折叠边界：**${ignoredBoundaries.length}** 项。\n\n` +
    `## Git 状态\n\n| 状态 | 数量 |\n| --- | ---: |\n${markdownCounts(countBy(entries, 'gitStatus'))}\n\n` +
    `## 模块\n\n| 模块 | 数量 |\n| --- | ---: |\n${markdownCounts(countBy(entries, 'module'))}\n\n` +
    `## 分类\n\n| 分类 | 数量 |\n| --- | ---: |\n${markdownCounts(countBy(entries, 'category'))}\n\n` +
    `## Ignored 折叠边界\n\n这些路径不计入源码文件总数，且台账生成器不会读取其内容。\n\n` +
    `### 按模块\n\n| 模块 | 数量 |\n| --- | ---: |\n${markdownCounts(countBy(ignoredBoundaries, 'module'))}\n\n` +
    `### 按类别\n\n| 类别 | 数量 |\n| --- | ---: |\n${markdownCounts(countBy(ignoredBoundaries, 'category'))}\n\n` +
    `## 删除项审查\n\n未闭环删除项：**${unresolvedDeletionCount}**。\n\n${deletionRows}\n\n` +
    `## 审查规则\n\n` +
    `- \`generated-source\` 在完整再生成链路验证前继续保留。\n` +
    `- \`third-party\` 保留外部接口契约，按上游来源单独审查。\n` +
    `- \`binary-asset\` 核对引用和体积，不因无法逐行读取而直接删除。\n` +
    `- \`deleted\` 和 \`untracked\` 均保留在台账中，避免漏掉当前工作树成果。\n` +
    `- ignored 边界只用于证明依赖、运行态、敏感配置、生成输出和本地审计证据已被分类，不代表允许删除。\n` +
    `- 此台账是文件级完整性证据，不等同于每个业务行为已经过运行验证。\n`;
}

function generateInventory() {
  const entries = listRepositoryPaths().map(inspectEntry);
  const ignoredBoundaries = listIgnoredBoundaries();
  fs.writeFileSync(path.join(projectRoot, csvRelativePath), renderCsv(entries), 'utf8');
  fs.writeFileSync(path.join(projectRoot, summaryRelativePath), renderSummary(entries, ignoredBoundaries), 'utf8');
  return entries;
}

if (require.main === module) {
  const entries = generateInventory();
  const statuses = countBy(entries, 'gitStatus');
  console.log(`Repository inventory generated: ${entries.length} files`);
  console.log(JSON.stringify(statuses));
}

module.exports = {
  categoryFor,
  excludedOutputPaths,
  generateInventory,
  ignoredBoundaryCategory,
  inspectEntry,
  isSensitivePath,
  listIgnoredBoundaries,
  listRepositoryPaths,
  moduleFor,
  projectRoot,
  reviewedDeletionReasons
};
