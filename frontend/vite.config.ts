/**
 * 文件用途：配置 Vite 开发、构建、代理和插件链。
 * 核心逻辑：按环境装配插件、路径别名、构建开关和诊断 trace。
 * 关键注意事项：插件开关会影响构建体积和本地预览，变更需做 focused build 或 lint 校验。
 * 重构建议：可继续拆分插件、代理和构建诊断配置，降低根配置复杂度。
 */
import process from 'node:process'
import { URL, fileURLToPath } from 'node:url'
import type { PluginOption } from 'vite'
import { defineConfig, loadEnv } from 'vite'
import dayjs from 'dayjs'
import { visualizer } from 'rollup-plugin-visualizer'
import svgLoader from 'vite-svg-loader'
import { setupVitePlugins } from './build/plugins'
import { createViteProxy } from './build/config'

function setupBuildTracePlugin(): PluginOption[] {
  if (process.env.VITE_TRACE_BUILD !== 'Y') return []

  let transformCount = 0
  let lastLoggedAt = Date.now()

  return [
    {
      name: 'aetherlink-build-trace',
      apply: 'build',
      enforce: 'pre',
      transform(_code, id) {
        transformCount += 1
        const now = Date.now()
        if (transformCount === 1 || transformCount % 25 === 0 || now - lastLoggedAt > 10000) {
          lastLoggedAt = now
          console.log(`[build-trace] transformed=${transformCount} module=${id.replace(/\\/g, '/')}`)
        }
        return null
      },
      buildEnd(error) {
        const status = error ? 'failed' : 'completed'
        console.log(`[build-trace] ${status} transformed=${transformCount}`)
      }
    }
  ]
}

export default defineConfig(function (configEnv) {
  const viteEnv = loadEnv(configEnv.mode, process.cwd()) as unknown as Env.ImportMeta
  const isLightBuild = process.env.VITE_LIGHT_BUILD === 'Y'
  const enableBundleReport = process.env.VITE_BUNDLE_REPORT === 'Y'

  const buildTime = dayjs().format('YYYY-MM-DD HH:mm:ss')
  const workspaceRootPath = fileURLToPath(new URL('./', import.meta.url))
  const srcRootPath = fileURLToPath(new URL('./src', import.meta.url))
  const sharedScssPath = fileURLToPath(new URL('./src/styles/scss/_mixins.scss', import.meta.url)).replace(/\\/g, '/')

  return {
    base: viteEnv.VITE_BASE_URL || '/',
    resolve: {
      alias: [
        { find: '~', replacement: workspaceRootPath },
        { find: '@', replacement: srcRootPath }
      ]
    },
    css: {
      preprocessorOptions: {
        scss: {
          additionalData: `@use "${sharedScssPath}" as *;`
        }
      }
    },
    plugins: [
      ...setupBuildTracePlugin(),
      ...setupVitePlugins(viteEnv),
      svgLoader(),
      ...(enableBundleReport
        ? [
            visualizer({
              filename: 'dist/stats.html',
              template: 'treemap',
              gzipSize: true,
              brotliSize: true
            }),
            visualizer({
              filename: 'dist/stats.raw-data.json',
              template: 'raw-data',
              gzipSize: true,
              brotliSize: true
            })
          ]
        : [])
    ],
    optimizeDeps: {
      include: [
        'vue',
        'vue-router',
        'pinia',
        'vue-i18n',
        'naive-ui',
        'axios',
        'dayjs',
        'lodash-es',
        '@vueuse/core',
        'nanoid',
        'nprogress'
      ]
    },
    server: {
      host: '0.0.0.0',
      port: 5002,
      open: true,
      proxy: createViteProxy(viteEnv),
      fs: {
        cachedChecks: false
      },
      watch: {
        usePolling: true
      }
    },
    preview: {
      port: 9725
    },
    build: {
      chunkSizeWarningLimit: 1500,
      minify: isLightBuild ? false : 'esbuild',
      sourcemap: false,
      reportCompressedSize: false,
      commonjsOptions: {
        ignoreTryCatch: false
      },
      rollupOptions: {
        output: {
          manualChunks(id) {
            const normalizedId = id.replace(/\\/g, '/')

            // Keep application modules in Rollup-managed chunks. Several app
            // features import each other through route/view boundaries, and
            // forcing them into named chunks can create production TDZ errors.

            if (!normalizedId.includes('node_modules')) return
            if (normalizedId.includes('echarts') || normalizedId.includes('zrender')) return 'vendor-echarts'
            if (normalizedId.includes('@codemirror') || normalizedId.includes('codemirror')) return 'vendor-codemirror'
            if (normalizedId.includes('grid-layout-plus')) return 'vendor-grid'
            if (normalizedId.includes('naive-ui')) return 'vendor-ui'
            // Let Rollup order Vue, Vue Router, Pinia, and Vue I18n together.
            // Forcing them into one manual chunk can trigger TDZ errors in
            // production preview when circular exports initialize out of order.
            if (
              normalizedId.includes('dayjs') ||
              normalizedId.includes('lodash-es') ||
              normalizedId.includes('axios') ||
              normalizedId.includes('@vueuse')
            ) {
              return 'vendor-utils'
            }
            if (normalizedId.includes('crypto-js')) return 'vendor-crypto'
            return 'vendor'
          }
        }
      }
    },
    define: {
      BUILD_TIME: JSON.stringify(buildTime)
    },
    lintOnSave: false
  }
})
