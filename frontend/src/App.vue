<!--
文件用途：承载前端根 Vue 组件和应用级占位。
核心逻辑：挂载全局应用容器、路由出口和基础 provider。
关键注意事项：入口层不要承载业务状态，避免影响路由和布局初始化。
重构建议：若全局壳层继续扩展，可拆出 AppShell 组件并保留根组件轻量。
-->
<!--
  Root Vue component for AetherLink IoT.
  Provides Naive UI configuration, theme/locale providers, global content
  shell, and router outlet for every route. JSON code highlighting is loaded
  after mount so it does not sit on the initial route critical path.
-->
<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { NConfigProvider, darkTheme } from 'naive-ui'
import { useAppStore } from './store/modules/app'
import { useThemeStore } from './store/modules/theme'
import { naiveDateLocales, naiveLocales } from './locales/naive'
import Content from './components/content/index.vue'

defineOptions({
  name: 'App'
})

const appStore = useAppStore()
const themeStore = useThemeStore()
const hljs = shallowRef()
const naiveDarkTheme = computed(() => (themeStore.darkMode ? darkTheme : undefined))

const naiveLocale = computed(() => {
  return naiveLocales[appStore.locale]
})

const naiveDateLocale = computed(() => {
  return naiveDateLocales[appStore.locale]
})

const setupCodeHighlight = async () => {
  try {
    const [{ default: hljsCore }, { default: jsonLanguage }] = await Promise.all([
      import('highlight.js/lib/core'),
      import('highlight.js/lib/languages/json')
    ])
    hljsCore.registerLanguage('json', jsonLanguage)
    hljs.value = hljsCore
  } catch {
    hljs.value = undefined
  }
}

onMounted(() => {
  const run = () => {
    void setupCodeHighlight()
  }

  if (typeof window !== 'undefined' && typeof window.requestIdleCallback === 'function') {
    window.requestIdleCallback(run, { timeout: 2000 })
    return
  }

  setTimeout(run, 100)
})

// 注意：此处不注册全局 fullscreenchange 监听器。
// 早期版本曾在此监听全屏变化，但退出子元素全屏时会误触发整个页面全屏，
// 因此移除该监听器，改由各组件自行管理全屏状态。
</script>

<template>
  <NConfigProvider
    :hljs="hljs"
    :theme="naiveDarkTheme"
    :theme-overrides="themeStore.naiveTheme"
    :locale="naiveLocale"
    :date-locale="naiveDateLocale"
    class="h-full"
  >
    <NMessageProvider>
      <Content />
      <AppProvider>
        <RouterView class="bg-layout" />
      </AppProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
