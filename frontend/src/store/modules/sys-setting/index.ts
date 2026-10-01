/**
 * 文件用途：定义 系统设置状态模块 的 Pinia 状态模块（含白标运行时：品牌资产、
 *   租户翻译覆盖合并与自定义 CSS 注入，TB-47）。
 * 核心逻辑：维护模块状态、计算属性和动作，并把状态变化暴露给页面、组件和路由流程；
 *   initWhitelabelOverrides 在登录后拉取覆盖，CSS 用 textContent 写入独立 style 标签，
 *   翻译覆盖按当前 locale 平铺键合并进 vue-i18n。
 * 关键注意事项：状态字段、持久化键和跨模块调用属于前端契约，调整时需要同步测试与调用方；
 *   CSS 注入只允许 textContent（禁止 innerHTML，防 XSS）；覆盖接口需登录态，
 *   失败时静默降级到静态语言目录与默认样式（fail-open，不阻塞路由初始化）。
 * 重构建议：可将副作用、接口访问和纯状态推导拆分，降低 store 文件复杂度。
 */
import { defineStore } from 'pinia'
import { fetchThemeSetting } from '@/service/api/setting'
import { fetchWhitelabelOverrides } from '@/service/api/whitelabel'
import { localStg } from '@/utils/storage'
import { createServiceConfig } from '~/env.config'
import { applyTranslationOverrides, pickOverridesForFolder, whitelabelFolderForLocale } from '@/locales/whitelabel-override'
import { i18n } from '@/locales'

const { otherBaseURL } = createServiceConfig(import.meta.env)
const platformApiUrl = new URL(otherBaseURL.platform ? otherBaseURL.platform : `${window.location.origin}/api/v1`)
const defaultFavicon = '/rdi/logo.png'

type SysSetting = Omit<Api.GeneralSetting.ThemeSetting, 'id'>

const emptySysSetting: SysSetting = {
  system_name: '',
  logo_background: '',
  logo_loading: '',
  logo_cache: '',
  home_background: '',
  theme_color: '',
  favicon: ''
}

/** 租户自定义 CSS 注入的 style 标签 id（幂等复用，全局唯一） */
const TENANT_CSS_STYLE_ID = 'aetherlink-tenant-custom-css'

// i18n 全局实例的结构化窄类型（与 locales/index.ts 的 i18nGlobal as any 口径一致，
// 这里给出最小可用形状以便 store 内可测、可查错）。
const i18nGlobal = i18n.global as unknown as {
  locale: { value: string }
  messages: { value: Record<string, Record<string, unknown>> }
  setLocaleMessage: (locale: string, message: Record<string, unknown>) => void
}

function resolveAssetUrl(value?: string | null) {
  const path = String(value || '').trim()
  if (!path) return ''
  if (/^(https?:)?\/\//i.test(path) || path.startsWith('data:')) return path
  return `${platformApiUrl.origin}${path.startsWith('/') ? path : `/${path}`}`
}

function applyFavicon(url?: string | null) {
  if (typeof document === 'undefined') return

  const href = String(url || '').trim() || defaultFavicon
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.href = href
}

// applyThemeColor 将租户级主题色注入全局 CSS 变量（C5 白标）。
// 空值/非法 hex 回退为空字符串，由 CSS 的 fallback 保持默认主题。
function applyThemeColor(color?: string | null) {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  const value = String(color || '').trim()
  root.style.setProperty('--aetherlink-brand-color', value || '')
}

function normalizeSetting(setting?: Partial<Api.GeneralSetting.ThemeSetting> | null): SysSetting {
  return {
    system_name: String(setting?.system_name || '').trim(),
    logo_background: resolveAssetUrl(setting?.logo_background),
    logo_loading: resolveAssetUrl(setting?.logo_loading),
    logo_cache: resolveAssetUrl(setting?.logo_cache),
    home_background: resolveAssetUrl(setting?.home_background),
    theme_color: String(setting?.theme_color || '').trim(),
    favicon: resolveAssetUrl(setting?.favicon)
  }
}

function syncBrandingRuntime(setting: SysSetting) {
  applyFavicon(setting.favicon || setting.logo_cache)
  applyThemeColor(setting.theme_color)
  localStg.set('logoLoading', setting.logo_loading || '')
  localStg.set('systemName', setting.system_name || '')
}

// applyCustomCSS 把租户自定义 CSS 用 textContent 写入独立 style 标签（TB-47 白标）。
// 空串时移除标签（清除语义）。必须用 textContent 而非 innerHTML——CSS 文本来自
// 租户输入，textContent 不触发 HTML 解析，是防 XSS 的主防线（后端另拒绝 </style 序列）。
function applyCustomCSS(css?: string | null) {
  if (typeof document === 'undefined') return

  const value = String(css || '')
  let style = document.getElementById(TENANT_CSS_STYLE_ID) as HTMLStyleElement | null
  if (!value.trim()) {
    style?.remove()
    return
  }
  if (!style) {
    style = document.createElement('style')
    style.id = TENANT_CSS_STYLE_ID
    document.head.appendChild(style)
  }
  style.textContent = value
}

export const useSysSettingStore = defineStore('sys-setting', {
  state: (): SysSetting & { customCSS: string } => ({ ...emptySysSetting, customCSS: '' }),
  actions: {
    async initSysSetting() {
      const { error, data } = await fetchThemeSetting()
      if (!error && data) {
        const list: Api.GeneralSetting.ThemeSetting[] = data.list
        const setting = normalizeSetting(list[0] || emptySysSetting)
        syncBrandingRuntime(setting)
        Object.assign(this.$state, setting)
      }
    },
    /**
     * 登录后拉取白标覆盖（TB-47）：翻译覆盖合并进当前 locale 的 vue-i18n 目录，
     * 自定义 CSS 用 textContent 注入 style 标签。接口需登录态；网络级失败或业务
     * 错误一律静默降级（fail-open：静态语言目录 + 默认样式仍可用），本动作绝不
     * 抛错/拒绝——调用方（路由初始化）用 void 触发，不能产生未处理拒绝。
     * 调用时机：路由守卫 initAuthRoute（覆盖登录成功与带 token 刷新两种场景）；
     * 切换语言走整页刷新，刷新后本动作会重新对目标 locale 应用覆盖。
     */
    async initWhitelabelOverrides() {
      try {
        const { error, data } = await fetchWhitelabelOverrides()
        if (error || !data) return

        this.customCSS = String(data.css || '')
        applyCustomCSS(this.customCSS)

        const locale = i18nGlobal.locale.value as string
        const folder = whitelabelFolderForLocale(locale)
        if (!folder) return
        const overrides = pickOverridesForFolder(data.translations, folder)
        if (Object.keys(overrides).length === 0) return
        const catalog = i18nGlobal.messages.value[locale] || {}
        i18nGlobal.setLocaleMessage(locale, applyTranslationOverrides(catalog as Record<string, unknown>, overrides))
      } catch {
        // 网络级失败/任何异常：静默降级到静态目录与默认样式，绝不向调用方传播拒绝。
        return
      }
    },
    /** 登出/重置时调用：清除内存中的 CSS 与覆盖（已注入的 style 标签同步移除）。 */
    resetWhitelabelRuntime() {
      this.customCSS = ''
      applyCustomCSS('')
    }
  }
})
