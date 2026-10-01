/**
 * 文件用途：白标（TB-47）租户翻译覆盖的前端合并逻辑（纯函数，唯一判定点）。
 * 核心逻辑：把后端按 lang 分组的 key→value 覆盖映射，合并进 vue-i18n 的平铺键目录——
 *   本仓库语言目录为平铺点号键形态（custom.json "page.x.y"），vue-i18n 在嵌套路径
 *   未命中时按字面量平铺键回退解析（@intlify/core-base resolveMessageFormat），
 *   因此覆盖必须保持平铺键原样合并，不得拆点号成嵌套对象。
 * 关键注意事项：本模块不得引入 store/request 等有副作用的依赖（保持可单测纯度）；
 *   合并永不改写入参（静态目录是共享引用），同键时覆盖值恒胜出。
 * 重构建议：若未来语言目录改为嵌套结构，本模块需同步提供深合并变体并补负向用例。
 */
import type { LocaleFolder } from './locale'

/** 白标覆盖允许的语言目录集合（与后端 SupportedTenantTranslationLangs 一致） */
export type WhitelabelFolder = LocaleFolder

/** i18n locale（zh-CN 形态）→ 白标语言目录（zh-cn 形态）；未知 locale 返回 null */
export function whitelabelFolderForLocale(locale: string): WhitelabelFolder | null {
  switch (locale) {
    case 'zh-CN':
      return 'zh-cn'
    case 'en-US':
      return 'en-us'
    case 'es-ES':
      return 'es-es'
    case 'fr-FR':
      return 'fr-fr'
    default:
      return null
  }
}

/**
 * 把租户覆盖合并进语言目录：返回新对象，不改写 messages / overrides 入参。
 * messages 为已加载目录（可能同时含平铺键与命名空间嵌套）；overrides 为
 * { "<平铺键>": "<译文>" } 映射，同键覆盖恒胜出。
 */
export function applyTranslationOverrides(
  messages: Record<string, unknown>,
  overrides: Record<string, string>
): Record<string, unknown> {
  const merged: Record<string, unknown> = { ...messages }
  for (const key of Object.keys(overrides)) {
    const value = overrides[key]
    if (typeof value === 'string' && value.length > 0) {
      merged[key] = value
    }
  }
  return merged
}

/**
 * 从后端覆盖载荷中取出指定语言目录的覆盖映射；载荷缺失/形态非法时返回空对象（fail-open 到静态目录）。
 */
export function pickOverridesForFolder(translations: unknown, folder: WhitelabelFolder): Record<string, string> {
  if (!translations || typeof translations !== 'object') return {}
  const byLang = translations as Record<string, unknown>
  const langMap = byLang[folder]
  if (!langMap || typeof langMap !== 'object') return {}
  const result: Record<string, string> = {}
  for (const [key, value] of Object.entries(langMap as Record<string, unknown>)) {
    if (typeof value === 'string' && value.length > 0) {
      result[key] = value
    }
  }
  return result
}
