/**
 * 文件用途：初始化 Vue 应用并挂载到页面根节点。
 * 核心逻辑：创建应用实例，注册插件、全局样式和运行时依赖后启动。
 * 关键注意事项：启动顺序会影响路由、状态和 UI 插件可用性，变更需做启动回归。
 * 重构建议：可把插件装配拆成更细的 bootstrap 模块，降低入口文件耦合。
 */
/**
 * Frontend application bootstrap.
 *
 * Registers global assets, store, router, i18n, dayjs, Iconify, loading state,
 * NProgress, and recently visited route persistence before mounting the Vue
 * application. Keep bootstrap side effects small and documented because they
 * run for every frontend entry.
 */
import { createApp, watch } from 'vue'
import './plugins/assets'
import { useTitle } from '@vueuse/core'
import { useSysSettingStore } from '@/store/modules/sys-setting'
import { $t } from '@/locales'
import { resolveDocumentTitle } from '@/router/guard/title-helper'
import { setupDayjs, setupLoading, setupNProgress } from './plugins'
import { setupStore } from './store'
import { router, setupRouter } from './router'
import { i18n, setupI18n } from './locales'
import App from './App.vue'

/** 最近访问路由在 localStorage 中的存储键 */
const RECENTLY_VISITED_ROUTES_KEY = 'RECENTLY_VISITED_ROUTES'
/** 最近访问路由的最大保留数量 */
const MAX_RECENT_ROUTES = 8

/** 需要排除记录的路由路径模式，支持以 /* 结尾的通配符 */
const excludedPaths = ['/login/*', '/404', '/home']

/** 最近访问路由的单条记录结构 */
interface RecentRoute {
  path: string
  name: unknown
  title: unknown
  i18nKey: unknown
  icon: unknown
  query: Record<string, unknown>
}

// 防抖函数 - 减少 localStorage 写入频率
function debounce<T extends (...args: any[]) => void>(func: T, wait: number): T {
  let timeout: ReturnType<typeof setTimeout> | null = null
  return ((...args: Parameters<T>) => {
    if (timeout) clearTimeout(timeout)
    timeout = setTimeout(() => func(...args), wait)
  }) as T
}

// 内存缓存最近访问的路由，减少 localStorage 读取
let recentRoutesCache: RecentRoute[] | null = null

async function setupApp() {
  const app = createApp(App)

  setupStore(app)
  // 启动语言包按需加载：挂载前注册当前语言，其余语言切换时动态拉取。
  await setupI18n(app)
  setupNProgress()
  setupLoading()

  // 2. 系统设置延迟加载 - 避免阻塞应用启动
  const sysSettingStore = useSysSettingStore()

  // 使用 Promise 但不等待，让系统设置并行加载
  sysSettingStore.initSysSetting().then(() => {
    const syncCurrentDocumentTitle = () => {
      const appTitle = sysSettingStore.system_name || $t('title')
      useTitle(resolveDocumentTitle(router.currentRoute.value, appTitle, $t))
    }

    // 监听 system_name 的变化，并根据变化动态更新国际化消息
    watch(
      () => sysSettingStore.system_name,
      (newSystemName) => {
        const locales = i18n.global.availableLocales

        locales.forEach((locale) => {
          i18n.global.mergeLocaleMessage(locale, {
            system: {
              title: newSystemName
            }
          })
        })
        syncCurrentDocumentTitle()
      },
      { immediate: true }
    )
  })

  const setupIdlePlugins = async () => {
    const { setupIconifyOffline } = await import('./plugins/iconify')
    setupIconifyOffline()
    setupDayjs()
  }

  // 3. 非关键初始化 - 使用 requestIdleCallback 延迟执行
  if (typeof requestIdleCallback !== 'undefined') {
    requestIdleCallback(
      () => {
        void setupIdlePlugins()
      },
      { timeout: 2000 }
    )
  } else {
    // requestIdleCallback 不可用时使用延迟初始化
    setTimeout(() => {
      void setupIdlePlugins()
    }, 100)
  }

  // 4. 路由初始化 - 应用启动必需
  await setupRouter(app)

  // 路由记录功能：将最近访问的路由防抖写入 localStorage
  const debouncedSaveRoutes = debounce((routes: RecentRoute[]) => {
    try {
      localStorage.setItem(RECENTLY_VISITED_ROUTES_KEY, JSON.stringify(routes))
      recentRoutesCache = routes
    } catch {
      /* localStorage 写入失败时静默忽略，避免阻塞路由跳转 */
    }
  }, 1000)

  // 初始化缓存
  try {
    const routesRaw = localStorage.getItem(RECENTLY_VISITED_ROUTES_KEY)
    recentRoutesCache = routesRaw ? (JSON.parse(routesRaw) as RecentRoute[]) : []
  } catch {
    recentRoutesCache = []
  }

  // 路由记录功能的后置守卫
  router.afterEach((to) => {
    const isExcluded = excludedPaths.some((pattern) => {
      if (pattern.endsWith('/*')) {
        // 通配符模式：确保匹配 /login/ 而不是 /login-other
        const prefix = pattern.slice(0, -1) // /login/
        return to.path.startsWith(prefix)
      }
      // 精确匹配模式
      return to.path === pattern
    })

    if (isExcluded) {
      return
    }

    // 过滤掉没有名称或 title 的路由，以及重定向的路由
    if (!to.name || !to.meta?.title || to.redirectedFrom) {
      return
    }

    if (!recentRoutesCache) {
      return
    }

    try {
      let recentRoutes = [...recentRoutesCache]

      // 已存在相同路由时先移除，避免重复
      const existingIndex = recentRoutes.findIndex((route) => route.path === to.path)
      if (existingIndex === 0) {
        return
      }

      if (existingIndex > 0) {
        recentRoutes.splice(existingIndex, 1)
      }

      const newRoute: RecentRoute = {
        path: to.path,
        name: to.name,
        title: to.meta.title,
        i18nKey: to.meta.i18nKey,
        icon: to.meta.icon,
        query: to.query
      }

      recentRoutes.unshift(newRoute)

      if (recentRoutes.length > MAX_RECENT_ROUTES) {
        recentRoutes = recentRoutes.slice(0, MAX_RECENT_ROUTES)
      }

      debouncedSaveRoutes(recentRoutes)
    } catch {
      /* 路由记录失败时静默忽略，不影响正常导航 */
    }
  })

  app.config.globalProperties.getPlatform = () => {
    const { appVersion } = window.navigator
    if (['iPhone', 'Android', 'iPad'].includes(appVersion) || window.innerWidth < 680) {
      return true
    }
    return false
  }

  app.mount('#app')
}

setupApp()
