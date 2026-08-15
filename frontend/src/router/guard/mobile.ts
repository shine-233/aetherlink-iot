/**
 * 文件用途：移动端布局路由守卫。
 * 核心逻辑：在移动端访问业务路由时识别是否需要移动端布局处理，并保留路由层适配入口。
 * 关键注意事项：当前守卫只做候选路由识别和导航放行，实际布局切换仍由布局组件根据 appStore.isMobile 渲染。
 * 重构建议：如果未来需要真正替换路由组件，应抽出明确的布局解析函数并补充重定向/组件替换测试。
 */
import type { RouteLocationNormalized, Router } from 'vue-router'
import { useAppStore } from '@/store/modules/app'

export function createMobileLayoutGuard(router: Router) {
  router.beforeEach((to, _from, next) => {
    const appStore = useAppStore()

    if (appStore.isMobile && shouldUseMobileLayout(to)) {
      const routeMatch = router.getRoutes().find(route => route.name === to.name)

      // Vue Router 不适合在守卫中直接替换已注册组件；这里仅保留移动端候选识别。
      if (routeMatch && isBaseLayoutComponent(routeMatch.components?.default)) {
        // 实际渲染差异交给布局组件根据 isMobile 状态处理。
      }
    }

    next()
  })
}

/**
 * 判断当前路由是否应参与移动端布局候选判断。
 */
function shouldUseMobileLayout(route: RouteLocationNormalized): boolean {
  const excludeRoutes = ['login', '403', '404', '500']

  if (route.meta?.constant) {
    return false
  }

  if (excludeRoutes.includes(route.name as string)) {
    return false
  }

  if (route.meta?.disableMobileLayout) {
    return false
  }

  return true
}

/**
 * 检查路由组件是否为基础布局组件。
 */
function isBaseLayoutComponent(component: any): boolean {
  if (!component) return false

  const componentName = component.name || component.__name || component.displayName
  return (
    componentName === 'BaseLayout' || (component.__asyncResolved && component.__asyncResolved.name === 'BaseLayout')
  )
}
