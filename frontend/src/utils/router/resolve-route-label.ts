/*
 * 文件用途：解析路由展示标签，兼容国际化 key 与兜底 label。
 * 核心逻辑：优先使用 i18n key 翻译，缺失时返回 fallback。
 * 关键注意事项：标签解析结果会影响菜单、面包屑和页签展示。
 * 重构建议：可增加缺失翻译的日志或开发期检查。
 */
import { $t } from '@/locales'

export function resolveRouteLabel(i18nKey?: string | null, fallbackLabel?: string | null) {
  const normalizedKey = i18nKey?.trim()
  const normalizedFallback = fallbackLabel?.trim()

  if (!normalizedKey) {
    return normalizedFallback || ''
  }

  const translated = $t(normalizedKey)

  if (translated && translated !== normalizedKey) {
    return translated
  }

  return normalizedFallback || normalizedKey
}
