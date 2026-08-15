/**
 * 文件用途：全局权限路由守卫。
 * 核心逻辑：协调授权路由初始化、登录重定向、外链路由、角色校验和 403 fallback。
 * 关键注意事项：`next()` 必须单次完成；auth/route store 初始化顺序错误会导致刷新白屏或权限绕过。
 * 重构建议：抽出策略判断为纯函数，并用守卫测试覆盖未登录、已登录、外链、无权限和 not-found 分支。
 */
import type { NavigationGuardNext, RouteLocationNormalized, Router } from 'vue-router'
import type { RouteKey, RoutePath } from '@elegant-router/types'
import { useAuthStore } from '@/store/modules/auth'
import { useRouteStore } from '@/store/modules/route'
import { localStg } from '@/utils/storage'

export function createPermissionGuard(router: Router) {
  router.beforeEach(async (to, from, next) => {
    const pass = await createAuthRouteGuard(to, from, next)

    if (!pass) return

    // 外链路由在新窗口打开，并终止当前导航，避免后续守卫重复调用 next()。
    if (to.meta.href) {
      window.open(to.meta.href, '_blank')
      next({ path: from.fullPath, replace: true, query: from.query, hash: to.hash })
      return
    }

    const authStore = useAuthStore()

    const isLogin = Boolean(localStg.get('token'))
    const needLogin = !to.meta.constant
    const routeRoles = to.meta.roles || []
    const rootRoute: RouteKey = 'root'
    const loginRoute: RouteKey = 'login'
    const noPermissionRoute: RouteKey = '403'

    // 角色为空表示公开给已登录用户；SYS_ADMIN 拥有全部授权路由访问权。
    const SUPER_ADMIN = 'SYS_ADMIN'
    const hasPermission =
      !routeRoles.length ||
      authStore.userInfo?.roles?.includes(SUPER_ADMIN) ||
      authStore.userInfo?.roles?.some(role => routeRoles.includes(role))
    const strategicPatterns: CommonType.StrategicPattern[] = [
      {
        condition: isLogin && to.path.startsWith('/login'),
        callback: () => {
          next({ name: rootRoute })
        }
      },
      {
        condition: !needLogin,
        callback: () => {
          next()
        }
      },
      {
        condition: !isLogin && needLogin,
        callback: () => {
          next({ name: loginRoute, query: { redirect: to.fullPath } })
        }
      },
      {
        condition: isLogin && needLogin && hasPermission,
        callback: () => {
          next()
        }
      },
      {
        condition: isLogin && needLogin && !hasPermission,
        callback: () => {
          next({ name: noPermissionRoute })
        }
      }
    ]

    strategicPatterns.some(({ condition, callback }) => {
      if (condition) {
        callback()
      }

      return condition
    })
  })
}

async function createAuthRouteGuard(
  to: RouteLocationNormalized,
  _from: RouteLocationNormalized,
  next: NavigationGuardNext
) {
  const notFoundRoute: RouteKey = 'not-found'
  const isNotFoundRoute = to.name === notFoundRoute

  if (to.meta.constant && !isNotFoundRoute) {
    return true
  }

  const routeStore = useRouteStore()
  if (routeStore.isInitAuthRoute && !isNotFoundRoute) {
    return true
  }

  if (routeStore.isInitAuthRoute && isNotFoundRoute) {
    const exist = await routeStore.getIsAuthRouteExist(to.path as RoutePath)

    if (exist) {
      const noPermissionRoute: RouteKey = '403'

      next({ name: noPermissionRoute })

      return false
    }

    return true
  }

  const isLogin = Boolean(localStg.get('token'))
  if (!isLogin) {
    const loginRoute: RouteKey = 'login'
    const redirect = to.fullPath

    next({ name: loginRoute, query: { redirect } })

    return false
  }

  const initSuccess = await routeStore.initAuthRoute()

  if (!initSuccess) {
    const authStore = useAuthStore()
    await authStore.resetStore()

    return false
  }

  if (isNotFoundRoute) {
    const rootRoute: RouteKey = 'root'
    const path = to.redirectedFrom?.name === rootRoute ? '/' : to.fullPath

    next({ path, replace: true, query: to.query, hash: to.hash })

    return false
  }

  return true
}
