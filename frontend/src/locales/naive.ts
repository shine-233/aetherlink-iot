/**
 * 文件用途：桥接 Naive UI 组件库的本地化配置。
 * 核心逻辑：根据当前语言返回 Naive UI locale 与 dateLocale。
 * 关键注意事项：组件库语言需与应用 i18n 同步，避免日期和组件文案不一致。
 * 重构建议：可把第三方 UI 本地化统一到一个 provider 配置层。
 */
import { dateEnUS, dateZhCN, enUS, zhCN } from 'naive-ui'
import type { NDateLocale, NLocale } from 'naive-ui'

export const naiveLocales: Record<App.I18n.LangType, NLocale> = {
  'zh-CN': zhCN,
  'en-US': enUS,
  'fr-FR': enUS,
  'es-ES': enUS
}

export const naiveDateLocales: Record<App.I18n.LangType, NDateLocale> = {
  'zh-CN': dateZhCN,
  'en-US': dateEnUS,
  'fr-FR': dateEnUS,
  'es-ES': dateEnUS
}
