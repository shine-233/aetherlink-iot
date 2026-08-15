// @unocss-include

/**
 * 文件用途：定义 AetherLink 项目的 UnoCSS 预设。
 * 核心逻辑：导出 presetAetherLink，并集中维护项目常用 shortcuts。
 * 关键注意事项：顶部 @unocss-include 指令需要保留在文件最前，避免影响 UnoCSS 静态扫描。
 * 重构建议：后续可按布局、间距、文本和状态分组 shortcuts，提升可审查性。
 */
import type { Preset } from '@unocss/core'
import type { Theme } from '@unocss/preset-uno'

export function presetAetherLink(): Preset<Theme> {
  const preset: Preset<Theme> = {
    name: 'preset-aetherlink',
    shortcuts: [
      {
        'wh-full': 'w-full h-full'
      },
      {
        'flex-center': 'flex justify-center items-center',
        'flex-x-center': 'flex justify-center',
        'flex-y-center': 'flex items-center',
        'flex-vertical': 'flex flex-col',
        'flex-vertical-center': 'flex-center flex-col',
        'flex-vertical-stretch': 'flex-vertical items-stretch',
        'i-flex-center': 'inline-flex justify-center items-center',
        'i-flex-x-center': 'inline-flex justify-center',
        'i-flex-y-center': 'inline-flex items-center',
        'i-flex-vertical': 'inline-flex flex-col',
        'i-flex-vertical-stretch': 'i-flex-vertical items-stretch',
        'flex-1-hidden': 'flex-1 overflow-hidden'
      },
      {
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
        'fixed-center': 'fixed-lt flex-center wh-full'
      },
      {
        'nowrap-hidden': 'overflow-hidden whitespace-nowrap',
        'ellipsis-text': 'nowrap-hidden text-ellipsis'
      }
    ]
  }

  return preset
}

export default presetAetherLink
