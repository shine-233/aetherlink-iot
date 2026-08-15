/**
 * 文件用途：配置 Day.js 的语言环境和时间本地化。
 * 核心逻辑：按当前语言加载日期语言包并同步时间展示。
 * 关键注意事项：日期语言与 i18n 切换需保持一致，避免组件显示混合语言。
 * 重构建议：可把日期格式和语言包加载策略抽成 locale adapter。
 */
import { locale } from 'dayjs'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/en'
import 'dayjs/locale/fr'
import 'dayjs/locale/es'
import { localStg } from '@/utils/storage'

/**
 * Set dayjs locale
 *
 * @param lang
 */
export function setDayjsLocale(lang: App.I18n.LangType = 'en-US') {
  const localMap = {
    'zh-CN': 'zh-cn',
    'en-US': 'en',
    'fr-FR': 'fr',
    'es-ES': 'es'
  } satisfies Record<App.I18n.LangType, string>

  const l = lang || localStg.get('lang') || 'en-US'

  locale(localMap[l])
}
