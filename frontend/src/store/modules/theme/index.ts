/**
 * 文件用途：定义 主题状态模块 的 Pinia 状态模块。
 * 核心逻辑：维护模块状态、计算属性和动作，并把状态变化暴露给页面、组件和路由流程。
 * 关键注意事项：状态字段、持久化键和跨模块调用属于前端契约，调整时需要同步测试与调用方。
 * 重构建议：可将副作用、接口访问和纯状态推导拆分，降低 store 文件复杂度。
 */
import { computed, effectScope, onScopeDispose, ref, toRefs, watch } from 'vue'
import type { Ref } from 'vue'
import { defineStore } from 'pinia'
import { useEventListener, usePreferredColorScheme } from '@vueuse/core'
import { SetupStoreId } from '@/enum'
import { localStg } from '@/utils/storage'
import { addThemeVarsToHtml, createThemeToken, getNaiveTheme, initThemeSettings, toggleCssDarkMode } from './shared'

/** Theme store */
export const useThemeStore = defineStore(SetupStoreId.Theme, () => {
  const scope = effectScope()
  const osTheme = usePreferredColorScheme()

  /** Theme settings */
  const settings: Ref<App.Theme.ThemeSetting> = ref(initThemeSettings())

  /** Dark mode */
  const darkMode = computed(() => {
    if (settings.value.themeScheme === 'auto') {
      return osTheme.value === 'dark'
    }
    return settings.value.themeScheme === 'dark'
  })

  /** Theme colors */
  const themeColors = computed(() => {
    const { themeColor, otherColor, isInfoFollowPrimary } = settings.value
    const colors: App.Theme.ThemeColor = {
      primary: themeColor,
      ...otherColor,
      info: isInfoFollowPrimary ? themeColor : otherColor.info
    }
    return colors
  })

  /** Naive theme */
  const naiveTheme = computed(() => getNaiveTheme(themeColors.value))

  /**
   * Settings json
   *
   * It is for copy settings
   */
  const settingsJson = computed(() => JSON.stringify(settings.value))

  /** Reset store */
  function resetStore() {
    settings.value = initThemeSettings()
  }

  /**
   * Set theme scheme
   *
   * @param themeScheme
   */
  function setThemeScheme(themeScheme: UnionKey.ThemeScheme) {
    settings.value.themeScheme = themeScheme
  }

  /** Toggle theme scheme */
  function toggleThemeScheme() {
    const themeSchemes: UnionKey.ThemeScheme[] = ['light', 'dark', 'auto']

    const index = themeSchemes.findIndex(item => item === settings.value.themeScheme)

    const nextIndex = index === themeSchemes.length - 1 ? 0 : index + 1

    const nextThemeScheme = themeSchemes[nextIndex]

    setThemeScheme(nextThemeScheme)
  }

  /**
   * Update theme colors
   *
   * @param key Theme color key
   * @param color Theme color
   */
  function updateThemeColors(key: App.Theme.ThemeColorKey, color: string) {
    if (key === 'primary') {
      settings.value.themeColor = color
    } else {
      settings.value.otherColor[key] = color
    }
  }

  /**
   * Set theme layout
   *
   * @param mode Theme layout mode
   */
  function setThemeLayout(mode: UnionKey.ThemeLayoutMode) {
    settings.value.layout.mode = mode
  }

  /** Setup theme vars to html */
  function setupThemeVarsToHtml() {
    const { themeTokens, darkThemeTokens } = createThemeToken(themeColors.value)
    addThemeVarsToHtml(themeTokens, darkThemeTokens)
  }

  /** Cache theme settings */
  function cacheThemeSettings() {
    const isProd = import.meta.env.PROD

    if (!isProd) return

    localStg.set('themeSettings', settings.value)
  }

  // cache theme settings when page is closed or refreshed
  useEventListener(window, 'beforeunload', () => {
    cacheThemeSettings()
  })

  // watch store
  scope.run(() => {
    // watch dark mode
    watch(
      darkMode,
      val => {
        toggleCssDarkMode(val)
      },
      { immediate: true }
    )

    // themeColors change, update css vars and storage theme color
    watch(
      themeColors,
      val => {
        setupThemeVarsToHtml()

        localStg.set('themeColor', val.primary)
      },
      { immediate: true }
    )
  })

  /** On scope dispose */
  onScopeDispose(() => {
    scope.stop()
  })

  return {
    ...toRefs(settings.value),
    darkMode,
    themeColors,
    naiveTheme,
    settingsJson,
    resetStore,
    setThemeScheme,
    toggleThemeScheme,
    updateThemeColors,
    setThemeLayout
  }
})
