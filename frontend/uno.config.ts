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
import { presetAetherLink, aetherlinkShortcuts } from '@aetherlink/uno-preset'
import { themeVars } from './src/theme/vars'

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
    ...aetherlinkShortcuts,
    'card-wrapper': 'rd-8px shadow-sm'
  },
  transformers: [transformerDirectives(), transformerVariantGroup()],
  presets: [presetUno({ dark: 'class' }), presetAetherLink()]
})
