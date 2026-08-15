/**
 * 文件用途：配置 UnoCSS 原子化样式预设和扫描规则。
 * 核心逻辑：定义主题 token、快捷规则、图标和内容扫描范围。
 * 关键注意事项：扫描范围会影响样式生成体积，排除项变更需确认页面样式不丢失。
 * 重构建议：可把业务色板和快捷类拆到独立主题模块，便于设计系统维护。
 */
import { defineConfig } from '@unocss/vite'
import transformerDirectives from '@unocss/transformer-directives'
import transformerVariantGroup from '@unocss/transformer-variant-group'
import presetUno from '@unocss/preset-uno'
import type { Theme } from '@unocss/preset-uno'
import { presetAetherLink } from '@aetherlink/uno-preset'
import { themeVars } from './src/theme/vars'

const sharedShortcuts = {
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

const contentExclude = ['node_modules', 'dist', '.git', '.husky', '.vscode', 'public', 'build', 'mock', './stats.html']

export default defineConfig<Theme>({
  content: {
    pipeline: {
      exclude: [...contentExclude, 'node_modules', 'dist']
    }
  },
  theme: {
    ...themeVars,
    fontSize: {
      'icon-xs': '0.875rem',
      'icon-small': '1rem',
      icon: '1.125rem',
      'icon-large': '1.5rem',
      'icon-xl': '2rem'
    }
  },
  shortcuts: {
    ...sharedShortcuts,
    'card-wrapper': 'rd-8px shadow-sm'
  },
  transformers: [transformerDirectives(), transformerVariantGroup()],
  presets: [presetUno({ dark: 'class' }), presetAetherLink()]
})
