/**
 * 文件用途：初始化前端国际化实例和语言资源入口。
 * 核心逻辑：注册语言包、默认语言和 i18n 运行时配置。
 * 关键注意事项：语言键缺失会影响路由标题、表单和业务文案，新增键需多语言同步。
 * 重构建议：可引入缺失键扫描和类型化消息键，降低翻译漂移。
 */
import type { App } from 'vue'
import { createI18n } from 'vue-i18n'
import { localStg } from '@/utils/storage'
import messages from './locale'

export const i18n = createI18n({
  locale: localStg.get('lang') || 'en-US',
  fallbackLocale: 'en-US',
  messages,
  // Vue I18n Composition API mode; this is a framework flag, not a product compatibility marker.
  legacy: false
})

/**
 * Setup plugin i18n
 *
 * @param app
 */
export function setupI18n(app: App) {
  app.use(i18n)
}

const i18nGlobal = i18n.global as any

export const $t = (key: string, ...args: any[]) => i18nGlobal.t(key, ...args) as string
export function setLocale(locale: App.I18n.LangType) {
  i18nGlobal.locale.value = locale
}
