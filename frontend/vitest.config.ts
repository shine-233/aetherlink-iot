/**
 * 文件用途：配置前端常规 Vitest 单元测试环境。
 * 核心逻辑：设置 Vue 测试插件、别名、覆盖率和测试匹配规则。
 * 关键注意事项：覆盖率阈值只证明执行范围，不等同业务闭环。
 * 重构建议：可拆出 coverage、alias 和环境三段配置，便于按测试层复用。
 */
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';
import vue from '@vitejs/plugin-vue';

// 使用 fileURLToPath 解析路径,与 vite.config.ts 保持一致(ESM 兼容)
const srcPath = fileURLToPath(new URL('./src', import.meta.url));
const rootPath = fileURLToPath(new URL('./', import.meta.url));

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': srcPath,
      '~': rootPath,
      '@aetherlink/axios': fileURLToPath(new URL('./packages/axios/src/index.ts', import.meta.url)),
      '@aetherlink/hooks': fileURLToPath(new URL('./packages/hooks/src/index.ts', import.meta.url)),
      '@aetherlink/utils': fileURLToPath(new URL('./packages/utils/src/index.ts', import.meta.url)),
      '@aetherlink/color-palette': fileURLToPath(new URL('./packages/color-palette/src/index.ts', import.meta.url)),
      '@aetherlink/materials': fileURLToPath(new URL('./packages/materials/src/index.ts', import.meta.url)),
      '@aetherlink/scripts': fileURLToPath(new URL('./packages/scripts/src/index.ts', import.meta.url)),
      '@aetherlink/uno-preset': fileURLToPath(new URL('./packages/uno-preset/src/index.ts', import.meta.url))
    }
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    hookTimeout: 60_000,
    testTimeout: 60_000,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{js,ts,jsx,tsx}'],
    exclude: ['node_modules', 'dist', '.idea', '.git', '.cache', 'packages/**', 'build/**'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html', 'lcov'],
      include: ['src/**/*.{ts,vue}'],
      exclude: [
        'node_modules/',
        'src/test/',
        'src/**/*.d.ts',
        'src/__tests__/**',
        'src/**/__tests__/**',
        'src/**/*.test.ts',
        'src/**/*.spec.ts',
        'packages/**'
      ],
      thresholds: {
        // Minimum regression gate based on the last complete profile. Passing
        // these percentages is not a claim that every business flow is closed.
        lines: 70,
        branches: 70,
        functions: 60,
        statements: 70,

        // === RDI 核心模块（P0 优先级） ===
        'src/views/device/details/modules/rdi/composables/**': {
          lines: 85, branches: 60, functions: 80, statements: 85
        },
        'src/views/device/details/modules/rdi/constants/**': {
          lines: 100, branches: 100, functions: 100, statements: 100
        },
        'src/views/device/details/modules/rdi-panel.vue': {
          lines: 95, branches: 100, functions: 20, statements: 95
        },
        'src/service/api/rdi.ts': {
          lines: 100, branches: 100, functions: 100, statements: 100
        },

        // === 遥测主链路（P0 优先级） ===
        'src/views/device/details/modules/telemetry/**': {
          lines: 85, branches: 65, functions: 55, statements: 85
        },
        'src/views/device/details/modules/telemetry/telemetry.vue': {
          lines: 95, branches: 70, functions: 60, statements: 95
        },
        'src/views/device/details/modules/telemetry/modules/history-data.vue': {
          lines: 95, branches: 85, functions: 65, statements: 95
        },
        'src/views/device/details/modules/telemetry/modules/time-series-data.vue': {
          lines: 75, branches: 60, functions: 50, statements: 75
        },
        'src/views/device/details/modules/telemetry/modules/AggregationSelector.vue': {
          lines: 80, branches: 65, functions: 55, statements: 80
        },
        'src/views/device/details/modules/telemetry/modules/ChartComponent.vue': {
          lines: 70, branches: 50, functions: 50, statements: 70
        },

        // === 设备详情扩展页（P1 优先级） ===
        'src/views/device/details/index.vue': {
          lines: 80, branches: 60, functions: 60, statements: 80
        },
        'src/views/device/details/modules/message.vue': {
          lines: 70, branches: 80, functions: 80, statements: 70
        },
        'src/views/device/details/modules/device-status.vue': {
          lines: 65, branches: 50, functions: 50, statements: 65
        },
        'src/views/device/details/modules/give-an-alarm.vue': {
          lines: 65, branches: 50, functions: 50, statements: 65
        },
        'src/views/device/manage/index.vue': {
          lines: 90, branches: 30, functions: 0, statements: 90
        },
        'src/views/device/shared-with-me/index.vue': {
          lines: 80, branches: 70, functions: 30, statements: 80
        },

        // === 告警模块（P1 优先级） ===
        'src/views/alarm/warning-message/components/alarm-configuration.vue': {
          lines: 90, branches: 80, functions: 30, statements: 90
        },
        'src/views/alarm/warning-message/components/alarm-configuration.helpers.ts': {
          lines: 100, branches: 85, functions: 100, statements: 100
        },
        'src/views/alarm/warning-message/components/pop-up.vue': {
          lines: 90, branches: 70, functions: 50, statements: 90
        },
        'src/views/alarm/warning-message/components/new-information.vue': {
          lines: 75, branches: 60, functions: 50, statements: 75
        },

        // === 仪表盘（P1 优先级） ===
        'src/views/dashboard/rdi-overview/index.vue': {
          lines: 90, branches: 80, functions: 65, statements: 90
        },

        // === 通用组件与核心库 ===
        'src/components/common/grid/errorHandler.ts': {
          lines: 95, branches: 70, functions: 100, statements: 95
        },
        'src/components/common/grid/utils-enhanced.ts': {
          lines: 95, branches: 45, functions: 100, statements: 95
        },
        'src/core/data-architecture/DataWarehouse.ts': {
          lines: 85, branches: 70, functions: 85, statements: 85
        },
        'src/core/data-architecture/executors/MultiLayerExecutorChain.ts': {
          lines: 80, branches: 55, functions: 90, statements: 80
        },
        'src/core/data-architecture/types/enhanced-types.ts': {
          lines: 80, branches: 100, functions: 5, statements: 80
        }
      }
    }
  }
});
