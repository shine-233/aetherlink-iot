import { existsSync, readdirSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { join } from 'node:path';
import process from 'node:process';

const packagesDir = join(process.cwd(), 'packages');
const packageDirs = readdirSync(packagesDir, { withFileTypes: true })
  .filter(entry => entry.isDirectory())
  .map(entry => entry.name)
  .sort();

for (const packageDir of packageDirs) {
  const config = join('packages', packageDir, 'tsconfig.json');
  if (!existsSync(config)) continue;
  const pnpmEntry = process.env.npm_execpath;
  if (!pnpmEntry) {
    console.error('typecheck:packages must be run through pnpm');
    process.exit(1);
  }
  const result = spawnSync(process.execPath, [pnpmEntry, 'exec', 'tsc', '--project', config, '--noEmit', '--skipLibCheck'], {
    cwd: process.cwd(),
    encoding: 'utf8'
  });
  process.stdout.write(result.stdout ?? '');
  process.stderr.write(result.stderr ?? '');
  if (result.status !== 0) process.exit(result.status ?? 1);
}
