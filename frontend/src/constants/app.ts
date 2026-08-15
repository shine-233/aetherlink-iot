/**
 * 文件用途：定义应用级常量和公共运行标识。
 * 核心逻辑：集中维护标题、命名空间、缓存前缀等入口常量。
 * 关键注意事项：这些值可能参与持久化或外部展示，改名需考虑兼容迁移。
 * 重构建议：可按展示、缓存、运行时分组拆分，减少常量文件职责混杂。
 */
/**
 * 文件：应用级常量。
 * 作用：维护主题、登录模块、布局、滚动、标签页和页面动画选项。
 * 依赖：依赖 transformRecordToOption 将国际化映射转为 UI 选项。
 * 维护：新增主题或登录模块时同步国际化 key 与选项导出。
 */

import { transformRecordToOption } from '@/utils/common/options'

export const themeSchemaRecord: Record<UnionKey.ThemeScheme, App.I18n.I18nKey> = {
  light: 'theme.themeSchema.light',
  dark: 'theme.themeSchema.dark',
  auto: 'theme.themeSchema.auto'
}

export const themeSchemaOptions = transformRecordToOption(themeSchemaRecord)

export const loginModuleRecord: Record<UnionKey.LoginModule, App.I18n.I18nKey> = {
  'pwd-login': 'page.login.pwdLogin.title',
  register: 'page.login.register.title',
  'register-email': 'page.login.register.title',
  'register-super-admin': 'page.login.register.title',
  'reset-pwd': 'page.login.resetPwd.title',
  'bind-wechat': 'page.login.bindWeChat.title'
}

export const themeLayoutModeRecord: Record<UnionKey.ThemeLayoutMode, App.I18n.I18nKey> = {
  vertical: 'theme.layoutMode.vertical',
  'vertical-mix': 'theme.layoutMode.vertical-mix',
  horizontal: 'theme.layoutMode.horizontal',
  'horizontal-mix': 'theme.layoutMode.horizontal-mix'
}

export const themeLayoutModeOptions = transformRecordToOption(themeLayoutModeRecord)

export const themeScrollModeRecord: Record<UnionKey.ThemeScrollMode, App.I18n.I18nKey> = {
  wrapper: 'theme.scrollMode.wrapper',
  content: 'theme.scrollMode.content'
}

export const themeScrollModeOptions = transformRecordToOption(themeScrollModeRecord)

export const themeTabModeRecord: Record<UnionKey.ThemeTabMode, App.I18n.I18nKey> = {
  chrome: 'theme.tab.mode.chrome',
  button: 'theme.tab.mode.button'
}

export const themeTabModeOptions = transformRecordToOption(themeTabModeRecord)

export const themePageAnimationModeRecord: Record<UnionKey.ThemePageAnimateMode, App.I18n.I18nKey> = {
  'fade-slide': 'theme.page.mode.fade-slide',
  fade: 'theme.page.mode.fade',
  'fade-bottom': 'theme.page.mode.fade-bottom',
  'fade-scale': 'theme.page.mode.fade-scale',
  'zoom-fade': 'theme.page.mode.zoom-fade',
  'zoom-out': 'theme.page.mode.zoom-out',
  none: 'theme.page.mode.none'
}

export const themePageAnimationModeOptions = transformRecordToOption(themePageAnimationModeRecord)
