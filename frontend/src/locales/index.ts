/**
 * 文件用途：初始化前端国际化实例和语言资源入口。
 * 核心逻辑：启动时只异步加载当前语言目录，切换语言时按需加载目标目录；
 *   注册完成后才挂载应用，保证渲染期 $t 始终有完整目录可用。
 * 关键注意事项：语言键缺失会影响路由标题、表单和业务文案，新增键需多语言同步；
 *   setLocale 现为异步——调用方无需等待，UI 在目录就绪后自动更新。
 * 重构建议：可引入缺失键扫描和类型化消息键，降低翻译漂移。
 */
import type { App } from 'vue'
import { createI18n } from 'vue-i18n'
import { localStg } from '@/utils/storage'
import { loadLocaleMessages } from './locale'

const initialLocale: App.I18n.LangType = localStg.get('lang') || 'en-US'

export const i18n = createI18n({
  locale: initialLocale,
  fallbackLocale: 'en-US',
  messages: {},
  // Vue I18n Composition API mode; this is a framework flag, not a product compatibility marker.
  legacy: false
})

let readyPromise: Promise<void> | null = null

/** 加载并注册启动语言；重复调用共享同一任务。挂载前必须 await。 */
export async function ensureLocaleReady(): Promise<void> {
  if (!readyPromise) {
    const current = i18n.global.locale.value as App.I18n.LangType
    readyPromise = loadLocaleMessages(current).then(messages => {
      i18n.global.setLocaleMessage(current, messages)
    })
  }
  return readyPromise
}

/**
 * Setup plugin i18n
 *
 * @param app
 */
export async function setupI18n(app: App) {
  await ensureLocaleReady()
  app.use(i18n)
}

const i18nGlobal = i18n.global as any

export const $t = (key: string, ...args: any[]) => i18nGlobal.t(key, ...args) as string

/** 异步切换语言：目标目录按需加载，加载完成后切换生效。 */
export async function setLocale(locale: App.I18n.LangType) {
  if (i18n.global.locale.value !== locale) {
    const messages = await loadLocaleMessages(locale)
    i18n.global.setLocaleMessage(locale, messages)
  }
  i18n.global.locale.value = locale
}
