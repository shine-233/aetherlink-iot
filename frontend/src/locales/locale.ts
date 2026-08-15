/**
 * 文件用途：维护语言切换和本地化状态辅助逻辑。
 * 核心逻辑：封装当前语言、持久化语言和动态切换流程。
 * 关键注意事项：语言持久化会影响首屏展示，变更需覆盖刷新和默认语言场景。
 * 重构建议：可统一浏览器语言探测、用户设置和默认语言的优先级策略。
 */
// Eagerly load the locale JSON modules so the message catalog is complete before app startup.
const modules = import.meta.glob('./langs/**/*.json', { eager: true })

export type LocaleFolder = 'zh-cn' | 'en-us' | 'fr-fr' | 'es-es'

export function getLangMessages(modules: Record<string, any>, lang: LocaleFolder) {
  const messages: Record<string, any> = {}
  const prefix = `./langs/${lang}/`

  for (const path in modules) {
    if (path.startsWith(prefix)) {
      const content = modules[path].default

      // 提取文件名作为命名空间
      const fileName = path.replace(prefix, '').replace('.json', '')

      // 特殊处理：某些文件保持扁平化结构，供当前扁平 key 读取路径使用
      const flatFiles = [
        'common',
        'card',
        'page',
        'device_template',
        'basic',
        'buttons',
        'custom',
        'dropdown',
        'form',
        'generate',
        'grouping_details',
        'icon',
        'interaction',
        'others',
        'route',
        'script',
        'test',
        'theme',
        'time',
        'visual-editor',
        'market'
      ]

      if (flatFiles.includes(fileName)) {
        // 扁平化合并（保持现有行为）
        Object.assign(messages, content)
      } else {
        // 使用文件名作为命名空间
        messages[fileName] = content
      }
    }
  }
  return messages
}

function getLangMessagesWithFallback(modules: Record<string, any>, lang: LocaleFolder) {
  if (lang === 'zh-cn' || lang === 'en-us') {
    return getLangMessages(modules, lang)
  }

  return {
    ...getLangMessages(modules, 'en-us'),
    ...getLangMessages(modules, lang)
  }
}

const locales: Record<App.I18n.LangType, App.I18n.Schema> = {
  'zh-CN': getLangMessages(modules, 'zh-cn') as unknown as App.I18n.Schema,
  'en-US': getLangMessages(modules, 'en-us') as unknown as App.I18n.Schema,
  'fr-FR': getLangMessagesWithFallback(modules, 'fr-fr') as unknown as App.I18n.Schema,
  'es-ES': getLangMessagesWithFallback(modules, 'es-es') as unknown as App.I18n.Schema
}

export default locales
