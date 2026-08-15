/**
 * 文件用途：装配 UnoCSS 插件。
 * 核心逻辑：加载根目录 uno.config.ts（@unocss/vite 默认按 cwd 查找配置文件）。
 * 关键注意事项：本地 svg 雪碧图由 ./icons.ts 的 createSvgIconsPlugin 统一注册，
 *   此处**不要**重复注册，否则同一 symbolId 会被注入两次。
 * 重构建议：如需按环境切换 UnoCSS 配置，在此处传入 configFile，不要拆散图标注册链路。
 */
import type { PluginOption } from 'vite'
import unocss from '@unocss/vite'

export function setupUnocssPlugin(): PluginOption[] {
  return [unocss() as PluginOption]
}
