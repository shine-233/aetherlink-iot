/*
 * 文件用途：提供路由跳转 Hook，统一首页、登录、登录模块切换和重定向处理。
 * 核心逻辑：根据是否处于 setup 选择 router 实例，并组装 route key、query、redirect 等参数。
 * 关键注意事项：该文件会影响登录闭环和动态路由跳转，改动需验证权限路由场景。
 * 重构建议：可把 URL/query 构造拆成纯函数，降低路由副作用测试成本。
 */
import { useRouter } from 'vue-router'
import type { RouteLocationRaw } from 'vue-router'
import type { LastLevelRouteKey, RouteKey } from '@elegant-router/types'
import { router as globalRouter } from '@/router'

/**
 * Router push
 *
 * Jump to the specified route, it can replace function router.push
 *
 * @param inSetup Whether is in vue script setup
 */
export function useRouterPush(inSetup = true) {
  const router = inSetup ? useRouter() : globalRouter
  const route = globalRouter.currentRoute

  const routerPush = router.push

  const routerBack = router.back

  interface RouterPushOptions {
    query?: Record<string, string>
    params?: Record<string, string>
  }

  async function routerPushByKey(key: LastLevelRouteKey | RouteKey, options?: RouterPushOptions) {
    const { query, params } = options || {}

    const routeLocation: RouteLocationRaw = {
      name: key
    }
    if (query) {
      routeLocation.query = query
    }

    if (params) {
      routeLocation.params = params
    }
    return routerPush(routeLocation)
  }

  async function toHome() {
    const home: LastLevelRouteKey = 'home'

    return routerPushByKey(home)
  }

  /**
   * Navigate to login page
   *
   * @param loginModule The login module
   * @param redirectUrl The redirect url, if not specified, it will be the current route fullPath
   */
  async function toLogin(loginModule?: UnionKey.LoginModule, redirectUrl?: string) {
    const module = loginModule || 'pwd-login'

    const options: RouterPushOptions = {
      params: {
        module
      }
    }
    let redirect = ''
    const isRememberPath = localStorage.getItem('isRememberPath')

    if (isRememberPath === '1') {
      redirect = redirectUrl || route.value.fullPath
    }

    if (redirect) {
      options.query = {
        redirect
      }
    }

    return routerPushByKey('login', options)
  }

  /**
   * Toggle login module
   *
   * @param module
   */
  async function toggleLoginModule(module: UnionKey.LoginModule) {
    const query = route.value.query as Record<string, string>

    return routerPushByKey('login', { query, params: { module } })
  }

  /** Redirect from login */
  async function redirectFromLogin() {
    const redirect = route.value.query?.redirect as string

    if (redirect) {
      routerPush(redirect)
    } else {
      toHome()
    }
  }

  return {
    route,
    routerPush,
    routerBack,
    routerPushByKey,
    toLogin,
    toggleLoginModule,
    redirectFromLogin
  }
}
