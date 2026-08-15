import { execSync } from 'child_process';
import { writeFileSync } from 'fs';
import { dirname } from 'path';
import { fileURLToPath } from 'url';

const frontendRoot = dirname(fileURLToPath(import.meta.url));

const testFiles = [
  'src/views/automation/linkage-edit/__tests__/linkage-edit-index.test.ts',
  'src/views/automation/linkage-edit/modules/__tests__/edit-premise.test.ts',
  'src/views/automation/linkage-edit/modules/__tests__/edit-action.test.ts',
  'src/views/automation/scene-edit/__tests__/scene-edit-index.test.ts',
  'src/views/personal-center/__tests__/personal-center-index.test.ts',
  'src/views/personal-center/components/__tests__/change-information.test.ts'
];

const filesArg = testFiles.join(' ');
const cmd = `node ./node_modules/vitest/vitest.mjs run ${filesArg} --reporter verbose --config vitest.automation.config.ts`;

try {
  const output = execSync(cmd, {
    encoding: 'utf-8',
    cwd: frontendRoot,
    timeout: 120000,
    maxBuffer: 10 * 1024 * 1024
  });
  writeFileSync('test-result.txt', output, 'utf-8');
  console.log('Tests passed!');
  console.log(output.slice(-2000));
} catch (e) {
  const output = e.stdout || '' + e.stderr || '' + e.message || '';
  writeFileSync('test-result.txt', output, 'utf-8');
  console.log('Tests failed. Output saved to test-result.txt');
  console.log(output.slice(-3000));
}
