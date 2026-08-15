/**
 * 文件用途：装配 elegant-router 的路由生成插件（默认关闭）。
 * 核心逻辑：`src/router/elegant/*` 已被**手工拆分**为 systemRoutes / automationRoutes /
 *   visualizationRoutes / deviceRoutes 四个子模块，`routes.ts` 只用 spread 聚合它们。
 *   elegant-router 的生成器通过 magicast 回读 `routes.ts`，而 magicast 无法 cast SpreadElement，
 *   一旦启用就在 configResolved 阶段抛 `MagicastError: Casting "SpreadElement" is not supported`，
 *   导致 `pnpm dev` 连 vite server 都起不来（不是配置写错，是生成器与手工产物不兼容）。
 * 关键注意事项：因此默认**不注册**该插件；`src/router/elegant/*` 与 `src/typings/elegant-router.d.ts`
 *   按已提交的生成产物对待，由人工维护。仅在确实要重新生成、并且愿意先把 `routes.ts` 还原成
 *   生成器可解析的扁平数组时，才设 `AETHERLINK_ELEGANT_ROUTER=1` 显式开启。
 *   路径映射必须与 `src/service/api/management.adapter.ts` 的 `ROUTE_DISPLAY_PATH_MAP` 保持一致。
 * 重构建议：若要恢复自动生成能力，需先取消 routes.ts 的 spread 聚合（或升级到支持 SpreadElement
 *   的 magicast），两者缺一不可。
 */
import process from 'node:process'
import type { PluginOption } from 'vite'
import ElegantVueRouter from '@elegant-router/vue/vite'

/**
 * 目录键 → 浏览器展示路径。
 * 与 `src/service/api/management.adapter.ts` 的 ROUTE_DISPLAY_PATH_MAP 同源，改动需两边同步。
 */
const ROUTE_DISPLAY_PATH_MAP: Record<string, string> = {
  device_config: '/device/template',
  device_template: '/device/thingsmodel',
  'automation_space-management': '/automation/scene-manage'
}

/** 是否显式要求启用路由生成器（默认关闭，见文件头说明） */
function isGeneratorEnabled() {
  // 兼容 package.json 里历史的 dev:skip-router-gen 脚本：该变量为 '1' 时同样保持关闭。
  if (process.env.AETHERLINK_SKIP_ELEGANT_ROUTER === '1') return false

  return process.env.AETHERLINK_ELEGANT_ROUTER === '1'
}

export function setupRouterPlugin(): PluginOption[] {
  if (!isGeneratorEnabled()) return []

  return [
    ElegantVueRouter({
      alias: {
        '@': 'src'
      },
      layouts: {
        base: 'src/layouts/base-layout/index.vue',
        blank: 'src/layouts/blank-layout/index.vue'
      },
      routePathTransformer(routeName, routePath) {
        return ROUTE_DISPLAY_PATH_MAP[routeName] ?? routePath
      }
    }) as PluginOption[]
  ]
}
