/**
 * 文件用途：配置面向自动化辅助测试的 Vitest 运行环境。
 * 核心逻辑：复用前端别名和测试环境设置，限定自动化相关测试入口。
 * 关键注意事项：该配置服务特定验证链，不应与普通单元测试配置混淆。
 * 重构建议：可抽出共享 Vitest base config，减少别名和环境配置重复。
 */
import { defineConfig } from 'vitest/config';
import vue from '@vitejs/plugin-vue';
import { fileURLToPath } from 'node:url';

const srcPath = fileURLToPath(new URL('./src', import.meta.url));
const rootPath = fileURLToPath(new URL('./', import.meta.url));

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': srcPath,
      '~': rootPath,
    }
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    setupFiles: ['./src/test/setup.ts'],
    include: [
      'src/views/automation/**/__tests__/*.test.ts',
      'src/views/personal-center/**/__tests__/*.test.ts'
    ],
    exclude: ['node_modules', 'dist', 'packages/**', 'build/**']
  }
});
