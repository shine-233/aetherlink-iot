// 文件用途：typecheck 盲区收编门禁。
// 核心逻辑：以 tsconfig.blindspots.json（主工程排除目录的补充工程）运行 vue-tsc，
//   将错误按「文件 × 错误码」聚合计数，与 blindspots-baseline.json 对比：
//   任一组合超出基线即失败（阻止新增欠账）；低于基线提示可收紧（不阻断）。
// 用法：node scripts/typecheck-blindspots.mjs [--update]
//   --update 用当前结果重写基线（仅在有意清账时使用）。
// 关键注意事项：这是 #P2「~65 文件不在任何 tsc 工程」的结构性收口——
//   盲区从此有门禁、有基线、只许下降；清零后可将目录移回主 tsconfig 并删除本脚本。

import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';

const root = process.cwd();
const update = process.argv.includes('--update');
const baselinePath = path.join(root, 'blindspots-baseline.json');

// Windows 下 pnpm/.cmd 无法被 execFileSync 直接拉起；改经 node 直调 vue-tsc 入口。
const require = createRequire(path.join(root, 'package.json'));
const vueTscEntry = require.resolve('vue-tsc/bin/vue-tsc.js');

let raw = '';
let code = 0;
try {
  const result = spawnSync(process.execPath, [vueTscEntry, '-p', 'tsconfig.blindspots.json', '--noEmit', '--skipLibCheck'], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024
  });
  raw = String(result.stdout || '') + String(result.stderr || '');
  code = result.status ?? -1;
} catch (err) {
  raw = String(err.stdout || '') + String(err.stderr || '');
  code = -1;
}

if (raw.trim() === '' && code !== 0) {
  console.error('[blindspots] vue-tsc produced no output but exited non-zero; refusing to treat as zero errors.');
  process.exit(2);
}

const counts = new Map();
for (const line of raw.split(/\r?\n/)) {
  const m = line.match(/^(.+?)\((\d+),(\d+)\):\s+(error TS\d+):/);
  if (!m) continue;
  const [, file, , , code] = m;
  const key = `${file}|${code}`;
  counts.set(key, (counts.get(key) || 0) + 1);
}

if (update) {
  const payload = { updatedAt: new Date().toISOString(), total: [...counts.values()].reduce((a, b) => a + b, 0), entries: Object.fromEntries([...counts.entries()].sort()) };
  writeFileSync(baselinePath, JSON.stringify(payload, null, 2) + '\n');
  console.log(`[blindspots] baseline updated: ${payload.total} errors across ${counts.size} keys`);
  process.exit(0);
}

if (!existsSync(baselinePath)) {
  console.error('[blindspots] missing blindspots-baseline.json; run with --update to create it deliberately.');
  process.exit(1);
}

const baseline = JSON.parse(readFileSync(baselinePath, 'utf8'));
const baseEntries = baseline.entries || {};
let newDebt = 0;
const shrunk = [];
for (const [key, count] of counts) {
  const allowed = baseEntries[key] || 0;
  if (count > allowed) {
    console.error(`[blindspots] NEW debt: ${key} = ${count} (baseline ${allowed})`);
    newDebt += count - allowed;
  } else if (count < allowed) {
    shrunk.push(`${key}: ${allowed} -> ${count}`);
  }
}
for (const key of Object.keys(baseEntries)) {
  if (!counts.has(key)) shrunk.push(`${key}: ${baseEntries[key]} -> 0`);
}
if (shrunk.length > 0) {
  console.log(`[blindspots] ${shrunk.length} key(s) shrank; consider pruning baseline with --update:`);
  for (const s of shrunk.slice(0, 10)) console.log(`  - ${s}`);
}

const total = [...counts.values()].reduce((a, b) => a + b, 0);
console.log(`[blindspots] current total=${total}, baseline total=${baseline.total}`);
if (newDebt > 0) {
  console.error(`[blindspots] FAILED: ${newDebt} error(s) exceed the frozen baseline.`);
  process.exit(1);
}
