// @unocss-include

/**
 * 文件用途：定义 AetherLink 项目的 UnoCSS 预设与唯一快捷类来源。
 * 核心逻辑：导出 presetAetherLink 与 aetherlinkShortcuts；uno.config.ts 与本预设共用同一份定义，禁止两处各自维护。
 * 关键注意事项：顶部 @unocss-include 指令需要保留在文件最前，避免影响 UnoCSS 静态扫描。
 * 重构建议：新增 shortcut 时先确认没有语义重复项（历史上有 flex-col/flex-vertical 两族并存）。
 */
import type { Preset } from '@unocss/core'
import type { Theme } from '@unocss/preset-uno'

/**
 * 项目级 shortcuts 唯一来源（uno.config.ts 也从这里引用）。
 * flex-col 与 flex-vertical 两族历史上都被页面使用（flex-vertical 族现存 9 处引用），保留别名关系。
 */
export const aetherlinkShortcuts: Record<string, string> = {
  'wh-full': 'w-full h-full',
  'flex-center': 'flex justify-center items-center',
  'flex-col-center': 'flex-center flex-col',
  'flex-x-center': 'flex justify-center',
  'flex-y-center': 'flex items-center',
  'i-flex-center': 'inline-flex justify-center items-center',
  'i-flex-x-center': 'inline-flex justify-center',
  'i-flex-y-center': 'inline-flex items-center',
  'flex-col': 'flex flex-col',
  'flex-col-stretch': 'flex-col items-stretch',
  'i-flex-col': 'inline-flex flex-col',
  'i-flex-col-stretch': 'i-flex-col items-stretch',
  // flex-vertical 族：flex-col 的历史别名，保持向后兼容
  'flex-vertical': 'flex flex-col',
  'flex-vertical-center': 'flex-center flex-col',
  'flex-vertical-stretch': 'flex-vertical items-stretch',
  'i-flex-vertical': 'inline-flex flex-col',
  'i-flex-vertical-stretch': 'i-flex-vertical items-stretch',
  'flex-1-hidden': 'flex-1 overflow-hidden',
  'absolute-lt': 'absolute left-0 top-0',
  'absolute-lb': 'absolute left-0 bottom-0',
  'absolute-rt': 'absolute right-0 top-0',
  'absolute-rb': 'absolute right-0 bottom-0',
  'absolute-tl': 'absolute-lt',
  'absolute-tr': 'absolute-rt',
  'absolute-bl': 'absolute-lb',
  'absolute-br': 'absolute-rb',
  'absolute-center': 'absolute-lt flex-center wh-full',
  'fixed-lt': 'fixed left-0 top-0',
  'fixed-lb': 'fixed left-0 bottom-0',
  'fixed-rt': 'fixed right-0 top-0',
  'fixed-rb': 'fixed right-0 bottom-0',
  'fixed-tl': 'fixed-lt',
  'fixed-tr': 'fixed-rt',
  'fixed-bl': 'fixed-lb',
  'fixed-br': 'fixed-rb',
  'fixed-center': 'fixed-lt flex-center wh-full',
  'nowrap-hidden': 'whitespace-nowrap overflow-hidden',
  'ellipsis-text': 'nowrap-hidden text-ellipsis',
  'transition-base': 'transition-all duration-300 ease-in-out'
}

export function presetAetherLink(): Preset<Theme> {
  const preset: Preset<Theme> = {
    name: 'preset-aetherlink',
    shortcuts: [aetherlinkShortcuts]
  }

  return preset
}

export default presetAetherLink
