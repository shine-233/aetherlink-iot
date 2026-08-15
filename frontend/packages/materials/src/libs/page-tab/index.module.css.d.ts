/**
 * 文件用途：PageTab CSS Modules 的 TypeScript 类型声明。
 * 核心逻辑：声明 index.module.css 中可被 TS/Vue 文件安全引用的类名。
 * 主要逻辑：导出只读 styles 对象，配合 import style from './index.module.css' 使用。
 * 关键注意事项：这是样式声明文件，修改 CSS 类名后需要同步更新本文件或重新生成声明。
 * 重构建议：建议用生成脚本维护 CSS Modules 声明，避免手写类名漂移。
 */
declare const styles: {
  readonly 'button-tab': string
  readonly 'button-tab_dark': string
  readonly 'button-tab_active': string
  readonly 'button-tab_active_dark': string
  readonly 'chrome-tab': string
  readonly 'chrome-tab_active': string
  readonly 'chrome-tab__bg': string
  readonly 'chrome-tab_active_dark': string
  readonly 'chrome-tab_dark': string
  readonly 'chrome-tab-divider': string
  readonly 'svg-close': string
}

export default styles
