/**
 * 文件用途：前端路由定义和授权路由生成入口。
 * 核心逻辑：合并 root、兼容 redirect、fallback、独立预览路由和 generated elegant routes，并转换为 Vue Router route records。
 * 关键注意事项：route name/path、constant meta 和兼容 redirect 会影响菜单、权限守卫和旧链接可达性。
 * 重构建议：将兼容 redirect 与生成路由分区维护，并用 route coverage contract 测试锁定关键路径。
 */
import type { CustomRoute, ElegantConstRoute, ElegantRoute } from '@elegant-router/types'
import { generatedRoutes } from '../elegant/routes'
import { layouts, views } from '../elegant/imports'
import { transformElegantRoutesToVueRoutes } from '../elegant/transform'

export const ROOT_ROUTE: CustomRoute = {
  name: 'root',
  path: '/',
  redirect: '/home',
  meta: {
    title: 'root',
    constant: true
  }
}

const customRoutes = [
  ROOT_ROUTE as unknown as ElegantRoute,
  {
    name: 'first-device-onboarding',
    path: '/first-device',
    redirect: {
      path: '/home',
      query: {
        onboarding: 'first-device',
        focus: 'quickstart'
      }
    },
    meta: {
      title: 'first-device-onboarding',
      constant: true
    }
  } as unknown as ElegantRoute,
  {
    name: 'terms',
    path: '/terms',
    component: 'layout.blank$view.legal',
    props: {
      type: 'terms'
    },
    meta: {
      title: 'terms',
      i18nKey: 'legal.termsTitle',
      constant: true
    }
  } as unknown as ElegantRoute,
  {
    name: 'privacy',
    path: '/privacy',
    component: 'layout.blank$view.legal',
    props: {
      type: 'privacy'
    },
    meta: {
      title: 'privacy',
      i18nKey: 'legal.privacyTitle',
      constant: true
    }
  } as unknown as ElegantRoute,
  {
    name: 'device-config-bridge-redirect',
    path: '/device/config',
    redirect: '/device/template',
    meta: {
      title: 'device-config-bridge-redirect',
      constant: true
    }
  } as unknown as ElegantRoute,
  {
    name: 'not-found',
    path: '/:pathMatch(.*)*',
    component: 'layout.blank$view.404',
    meta: {
      title: 'not-found',
      constant: true
    }
  } as unknown as ElegantRoute,
  {
    name: 'device-details-app',
    path: '/device-details-app',
    component: 'layout.blank$view.device-details-app',
    meta: {
      title: 'device-details-app',
      i18nKey: 'route.device-details-app',
      constant: true
    }
  } as unknown as ElegantRoute
] as unknown as ElegantRoute[]

// The standalone preview route is public. It can render either the local
// provider or the explicitly selected legacy provider without requiring the
// optional compatibility profile just to resolve the page.
const thingsvisPreviewRoute = {
  name: 'thingsvis-preview-standalone',
  path: '/tv-preview',
  component: 'layout.blank$view.visualization_thingsvis-preview',
  meta: {
    title: 'thingsvis-preview',
    constant: true
  }
} as any

/** 创建常量路由和授权路由集合。 */
export function createRoutes() {
  const constantRoutes: ElegantRoute[] = []
  const authRoutes: ElegantRoute[] = []

  // Keep every compatibility page resolvable. Provider selection happens in
  // the page/provider seam: an unqualified route uses Native by default,
  // while provider=legacy-thingsvis remains an explicit external opt-in.
  constantRoutes.push(thingsvisPreviewRoute)

  ;[...customRoutes, ...(generatedRoutes as ElegantRoute[])].forEach(item => {
    if (item.meta?.constant) {
      constantRoutes.push(item)
    } else {
      authRoutes.push(item)
    }
  })

  const constantVueRoutes = transformElegantRoutesToVueRoutes(constantRoutes, layouts, views)

  return {
    constantVueRoutes,
    authRoutes
  }
}

/**
 * 将授权 Elegant routes 转换为 Vue Router routes。
 *
 * @param routes - Elegant routes
 */
export function getAuthVueRoutes(routes: ElegantConstRoute[]) {
  return transformElegantRoutesToVueRoutes(routes, layouts, views)
}
