/**
 * 文件用途：定义 应用全局状态模块 的 Pinia 状态模块。
 * 核心逻辑：维护模块状态、计算属性和动作，并把状态变化暴露给页面、组件和路由流程。
 * 关键注意事项：状态字段、持久化键和跨模块调用属于前端契约，调整时需要同步测试与调用方。
 * 重构建议：可将副作用、接口访问和纯状态推导拆分，降低 store 文件复杂度。
 */
import { effectScope, onScopeDispose, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { breakpointsTailwind, useBreakpoints, useTitle } from '@vueuse/core'
import { useBoolean } from '@aetherlink/hooks'
import { message } from '@/utils/common/discrete'
import { router } from '@/router'
import { resolveDocumentTitle } from '@/router/guard/title-helper'
import { SetupStoreId } from '@/enum'
import { $t, setLocale } from '@/locales'
import { setDayjsLocale } from '@/locales/dayjs'
import { localStg } from '@/utils/storage'
import { getToken } from '../auth/shared'
import { useRouteStore } from '../route'
import { useSysSettingStore } from '../sys-setting'
import { useTabStore } from '../tab'
import { useThemeStore } from '../theme'

export const useAppStore = defineStore(SetupStoreId.App, () => {
  const themeStore = useThemeStore()
  const routeStore = useRouteStore()
  const sysSettingStore = useSysSettingStore()
  const tabStore = useTabStore()
  const scope = effectScope()
  const breakpoints = useBreakpoints(breakpointsTailwind)
  const { bool: themeDrawerVisible, setTrue: openThemeDrawer, setFalse: closeThemeDrawer } = useBoolean()
  const { bool: reloadFlag, setBool: setReloadFlag } = useBoolean(true)
  const { bool: fullContent, setBool: setFullContent, toggle: toggleFullContent } = useBoolean()
  const { bool: contentXScrollable, setBool: setContentXScrollable } = useBoolean()
  const { bool: siderCollapse, setBool: setSiderCollapse, toggle: toggleSiderCollapse } = useBoolean()
  const { bool: mixSiderFixed, setBool: setMixSiderFixed, toggle: toggleMixSiderFixed } = useBoolean()

  /** Is mobile layout */
  const isMobile = breakpoints.smaller('sm')

  /**
   * Reload page
   *
   * @param duration Duration time
   */
  async function reloadPage(duration = 0) {
    setReloadFlag(false)

    if (duration > 0) {
      await new Promise(resolve => {
        setTimeout(resolve, duration)
      })
    }

    setReloadFlag(true)
  }

  const locale = ref<App.I18n.LangType>(localStg.get('lang') || 'en-US')
  // 启动即对齐 <html lang> 与实际语言（index.html 静态值不随用户偏好变化）。
  document.documentElement.lang = locale.value

  const localeOptions: App.I18n.LangOption[] = [
    {
      label: '中文',
      key: 'zh-CN'
    },
    {
      label: 'English',
      key: 'en-US'
    },
    {
      label: 'Français',
      key: 'fr-FR'
    },
    {
      label: 'Español',
      key: 'es-ES'
    }
  ]

  function changeLocale(lang: App.I18n.LangType, options: { persistRemote?: boolean } = {}) {
    const { persistRemote = true } = options
    locale.value = lang
    setLocale(lang)
    // 同步 <html lang>，保证可访问性（屏幕阅读器发音）与浏览器翻译提示正确。
    document.documentElement.lang = lang
    localStg.set('lang', lang)
    if (persistRemote) {
      void persistPreferredLanguage(lang)
    }
    message.success($t('common.languageSwitched'))
    // Force reload page to ensure all components update with new locale
    reloadPage(100)
  }

  async function persistPreferredLanguage(lang: App.I18n.LangType) {
    if (!getToken()) return

    try {
      const { savePreferredLanguage } = await import('@/service/api/personal-center')
      await savePreferredLanguage({ prefer_lang: lang, default_language: lang })
    } catch {
      // Keep language switching local if saving the preference fails.
    }
  }

  /** Update document title by locale */
  function updateDocumentTitleByLocale() {
    const appTitle = sysSettingStore.system_name === '' ? $t('title') : sysSettingStore.system_name
    const documentTitle = resolveDocumentTitle(router.currentRoute.value, appTitle || $t('title'), $t)
    useTitle(documentTitle)
  }

  function init() {
    setDayjsLocale(locale.value)
  }

  // watch store
  scope.run(() => {
    // watch isMobile, if is mobile, collapse sider
    watch(
      isMobile,
      newValue => {
        if (newValue) {
          setSiderCollapse(true)

          themeStore.setThemeLayout('vertical')
        }
      },
      { immediate: true }
    )

    // watch locale
    watch(locale, () => {
      // update document title by locale
      updateDocumentTitleByLocale()

      // update global menus by locale
      routeStore.updateGlobalMenusByLocale()

      // update tabs by locale
      tabStore.updateTabsByLocale()

      // set dayjs locale
      setDayjsLocale(locale.value)
    })
  })

  /** On scope dispose */
  onScopeDispose(() => {
    scope.stop()
  })

  // init
  init()

  return {
    isMobile,
    reloadFlag,
    reloadPage,
    fullContent,
    locale,
    localeOptions,
    changeLocale,
    themeDrawerVisible,
    openThemeDrawer,
    closeThemeDrawer,
    setFullContent,
    toggleFullContent,
    contentXScrollable,
    setContentXScrollable,
    siderCollapse,
    setSiderCollapse,
    toggleSiderCollapse,
    mixSiderFixed,
    setMixSiderFixed,
    toggleMixSiderFixed
  }
})
