/**
 * 文件用途：按需加载多语言消息目录（首屏只加载当前语言）。
 * 核心逻辑：非 eager glob 只产出加载器；loadLocaleMessages 按 lang 目录过滤后
 *   动态拉取，fr/es 复用既有 en-us 兜底合并语义，已加载目录进程内缓存复用。
 * 关键注意事项：getLangMessages 保持纯函数契约（locale-loader.test.ts 锁定）；
 *   首屏必须在挂载前完成启动语言注册（见 locales/index.ts ensureLocaleReady）。
 * 重构建议：后续可为目录加载失败增加重试与降级提示。
 */
// 非 eager：仅生成动态导入加载器，语言包不再整体打进首屏 entry。
const moduleLoaders = import.meta.glob('./langs/**/*.json')

export type LocaleFolder = 'zh-cn' | 'en-us' | 'fr-fr' | 'es-es'

const loadedCatalogs = new Map<App.I18n.LangType, App.I18n.Schema>()
const pendingLoads = new Map<App.I18n.LangType, Promise<App.I18n.Schema>>()

function langToFolder(lang: App.I18n.LangType): LocaleFolder {
  switch (lang) {
    case 'zh-CN':
      return 'zh-cn'
    case 'fr-FR':
      return 'fr-fr'
    case 'es-ES':
      return 'es-es'
    default:
      return 'en-us'
  }
}

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

/** 返回已缓存的目录；未加载过时返回 undefined。 */
export function getCachedLocaleMessages(lang: App.I18n.LangType): App.I18n.Schema | undefined {
  return loadedCatalogs.get(lang)
}

/** 按需加载目标语言目录（含 en-us 兜底合并），并发调用共享同一加载任务。 */
export async function loadLocaleMessages(lang: App.I18n.LangType): Promise<App.I18n.Schema> {
  const cached = loadedCatalogs.get(lang)
  if (cached) return cached

  const pending = pendingLoads.get(lang)
  if (pending) return pending

  const task = (async () => {
    const folder = langToFolder(lang)
    const targetPrefix = `./langs/${folder}/`
    const fallbackPrefix = './langs/en-us/'
    const modules: Record<string, any> = {}

    const jobs: Array<Promise<void>> = []
    for (const [path, loader] of Object.entries(moduleLoaders)) {
      const isTarget = path.startsWith(targetPrefix)
      const needsFallback = folder !== 'zh-cn' && folder !== 'en-us' && path.startsWith(fallbackPrefix)
      if (!isTarget && !needsFallback) continue
      jobs.push(
        loader().then(module => {
          modules[path] = module
        })
      )
    }
    await Promise.all(jobs)

    const catalog = getLangMessagesWithFallback(modules, folder) as unknown as App.I18n.Schema
    loadedCatalogs.set(lang, catalog)
    return catalog
  })()

  pendingLoads.set(lang, task)
  try {
    return await task
  } finally {
    pendingLoads.delete(lang)
  }
}
