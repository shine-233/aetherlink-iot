/**
 * 文件用途：按环境装配 Vite 插件链，供 `vite.config.ts` 的 `setupVitePlugins` 调用。
 * 核心逻辑：聚合 vue/jsx、UnoCSS、图标、路由生成与构建进度插件，顺序即插件生效顺序。
 * 关键注意事项：插件顺序会影响组件解析与样式生成；`unplugin-vue-components` 必须在 vue 插件之后注册。
 *   本目录是**必需构建源码**（非生成产物），删除会导致 `pnpm dev` / `pnpm build` 直接无法加载配置。
 * 重构建议：新增插件时优先落到独立文件，再在此聚合，保持根配置文件为薄壳。
 */
import type { PluginOption } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
import Components from 'unplugin-vue-components/vite'
import { NaiveUiResolver } from 'unplugin-vue-components/resolvers'
import IconsResolver from 'unplugin-icons/resolver'
import progress from 'vite-plugin-progress'
import { setupUnocssPlugin } from './unocss'
import { setupIconPlugins } from './icons'
import { setupRouterPlugin } from './router'

/**
 * 自动注册组件的扫描范围。
 *
 * 必须与 tsconfig.json 的 `exclude` 保持一致：`src/typings/components.d.ts` 是本插件生成的产物，
 * 一旦扫描到被 tsconfig 排除的目录（grid / gridv2 / 备份目录），生成的 `import()` 会把这些目录
 * 重新拖回 `vue-tsc` 的检查范围，凭空产生 typecheck 报错。新增 tsconfig 排除项时同步此处。
 */
const COMPONENT_GLOBS = [
  'src/components/**/*.vue',
  '!src/components/DeviceSelectSingle.vue',
  '!src/components/common/grid/**',
  '!src/components/common/gridv2/**',
  '!src/components/common/gridv2_backup_*/**',
  '!src/components/**/backup/**',
  '!src/components/**/examples/**',
  '!src/components/**/__tests__/**',
  '!src/components/**/* copy.vue'
]

export function setupVitePlugins(viteEnv: Env.ImportMeta): PluginOption[] {
  // Keep a clean checkout buildable without requiring a developer-only .env file.
  const iconPrefix = viteEnv.VITE_ICON_PREFIX || 'icon'
  const localPrefix = viteEnv.VITE_ICON_LOCAL_PREFIX || 'local-icon'

  return [
    vue(),
    vueJsx(),
    ...setupRouterPlugin(),
    ...setupUnocssPlugin(),
    Components({
      dts: 'src/typings/components.d.ts',
      types: [],
      globs: COMPONENT_GLOBS,
      resolvers: [
        NaiveUiResolver(),
        IconsResolver({
          prefix: iconPrefix,
          // 让 `icon-local-avatar` / `IconLocalAvatar` 解析到本地 svg 集合
          customCollections: [localPrefix.replace(`${iconPrefix}-`, '')]
        })
      ]
    }),
    ...setupIconPlugins(viteEnv),
    progress()
  ]
}
