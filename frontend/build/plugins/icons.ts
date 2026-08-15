/**
 * 文件用途：装配图标相关的 Vite 插件（本地 svg 雪碧图 + unplugin-icons 组件化图标）。
 * 核心逻辑：注册 `virtual:svg-icons-register` 雪碧图，并把 `Icon<Collection><Name>` 组件名解析为按需图标。
 * 关键注意事项：symbolId 必须与 `src/components/custom/svg-icon.vue` 的 `#${VITE_ICON_LOCAL_PREFIX}-${name}` 一致；
 *   `local` 自定义集合指向 `src/assets/svg-icon`，对应模板里的 `IconLocalAvatar` / `IconLocalLogo`。
 * 重构建议：新增图标集合时同步 package.json 依赖，避免出现模板引用了未安装的集合。
 */
import path from 'node:path'
import process from 'node:process'
import type { PluginOption } from 'vite'
import { createSvgIconsPlugin } from 'vite-plugin-svg-icons'
import Icons from 'unplugin-icons/vite'
import { FileSystemIconLoader } from 'unplugin-icons/loaders'

/** 本地 svg 图标目录，同时用于雪碧图与 `local` 自定义集合 */
export const localIconPath = path.join(process.cwd(), 'src/assets/svg-icon')

export function setupIconPlugins(viteEnv: Env.ImportMeta): PluginOption[] {
  const { VITE_ICON_LOCAL_PREFIX: localPrefix } = viteEnv

  return [
    createSvgIconsPlugin({
      iconDirs: [localIconPath],
      // 与 svg-icon.vue 的 `#${prefix}-${icon}` 保持一致，图标为平铺目录故不含 [dir]
      symbolId: `${localPrefix}-[name]`,
      inject: 'body-last',
      customDomId: '__SVG_ICON_LOCAL__'
    }),
    Icons({
      compiler: 'vue3',
      customCollections: {
        [localPrefix.replace(`${viteEnv.VITE_ICON_PREFIX}-`, '')]: FileSystemIconLoader(localIconPath, svg =>
          svg.replace(/^<svg\s/, '<svg width="1em" height="1em" ')
        )
      },
      scale: 1,
      defaultClass: 'inline-block'
    })
  ]
}
