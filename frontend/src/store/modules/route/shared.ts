/**
 * 文件用途: route store 的纯转换 helper 集合。
 * 核心逻辑: 处理权限过滤、菜单生成、排序、缓存路由、面包屑和 locale 更新。
 * 关键注意事项: route key、meta、roles 和 menu order 的转换会影响权限边界与可见导航。
 * 重构建议: 将每类转换保持为无副作用函数，并补充后端路由缺字段、角色不匹配和缓存路由测试。
 */
import type { RouteLocationNormalizedLoaded, RouteRecordRaw, _RouteRecordBase } from 'vue-router'
import type { ElegantConstRoute, LastLevelRouteKey, RouteKey, RouteMap } from '@elegant-router/types'
import { useSvgIconRender } from '@aetherlink/hooks'
import SvgIcon from '@/components/custom/svg-icon.vue'
import { resolveRouteLabel } from '@/utils/router/resolve-route-label'

/**
 * 按角色过滤鉴权路由。
 *
 * @param routes 鉴权路由列表
 * @param roles 当前用户角色
 */
export function filterAuthRoutesByRoles(routes: ElegantConstRoute[], roles: string[]) {
  const SUPER_ROLE = 'SYS_ADMIN'
  // 超级管理员直接保留全部路由，避免重复递归裁剪。
  if (roles?.includes(SUPER_ROLE)) {
    return routes
  }

  return routes.flatMap(route => filterAuthRouteByRoles(route, roles))
}

/**
 * 递归过滤单个鉴权路由及其子路由。
 *
 * @param route 当前路由
 * @param roles 当前用户角色
 */
function filterAuthRouteByRoles(route: ElegantConstRoute, roles: string[]) {
  const routeRoles = (route.meta && route.meta.roles) || []
  const currentRoles = Array.isArray(roles) ? roles : []

  // 只要命中任一角色即可保留该路由。
  const hasPermission = !routeRoles.length || routeRoles.some(role => currentRoles.includes(role))
  if (!hasPermission) return []

  const filterRoute = { ...route }

  if (filterRoute.children?.length) {
    filterRoute.children = filterRoute.children.flatMap(item => filterAuthRouteByRoles(item, roles))

    // A public parent is useful only when it still has an authorized child.
    if (!filterRoute.children.length) return []
  }

  return [filterRoute]
}

/**
 * 递归按 `meta.order` 排序单个路由树。
 */
function sortRouteByOrder(route: ElegantConstRoute) {
  if (route.children?.length) {
    route.children.sort((next, prev) => (Number(next.meta?.order) || 0) - (Number(prev.meta?.order) || 0))
    route.children.forEach(sortRouteByOrder)
  }

  return route
}

/**
 * 按 `meta.order` 排序路由列表。
 */
export function sortRoutesByOrder(routes: ElegantConstRoute[]) {
  routes.sort((next, prev) => (Number(next.meta?.order) || 0) - (Number(prev.meta?.order) || 0))
  routes.forEach(sortRouteByOrder)
  return routes
}

/**
 * 根据鉴权路由生成全局菜单树。
 */
export function getGlobalMenusByAuthRoutes(routes: ElegantConstRoute[]) {
  const menus: App.Global.Menu[] = []

  routes.forEach(route => {
    if (!route.meta?.hideInMenu) {
      const menu = getGlobalMenuByBaseRoute(route)

      if (route.children?.some(child => !child.meta?.hideInMenu)) {
        menu.children = getGlobalMenusByAuthRoutes(route.children)
      }

      menus.push(menu)
    }
  })

  return menus
}

/**
 * 重新计算全局菜单的本地化文案。
 */
export function updateLocaleOfGlobalMenus(menus: App.Global.Menu[]) {
  const result: App.Global.Menu[] = []

  menus.forEach(menu => {
    const { i18nKey, label, children } = menu

    const newLabel = resolveRouteLabel(i18nKey, label)

    const newMenu: App.Global.Menu = {
      ...menu,
      label: newLabel
    }

    if (children?.length) {
      newMenu.children = updateLocaleOfGlobalMenus(children)
    }

    result.push(newMenu)
  })

  return result
}

/**
 * 根据路由记录生成单个全局菜单节点。
 */
function getGlobalMenuByBaseRoute(route: RouteLocationNormalizedLoaded | ElegantConstRoute) {
  const { SvgIconVNode } = useSvgIconRender(SvgIcon)

  const { name, path } = route
  const { title, i18nKey, icon = import.meta.env.VITE_MENU_ICON, localIcon, remark } = route.meta ?? {}

  const label = resolveRouteLabel(i18nKey, title)

  const menu: App.Global.Menu = {
    key: name as string,
    label,
    i18nKey,
    routeKey: name as RouteKey,
    routePath: path as RouteMap[RouteKey],
    icon: SvgIconVNode({ icon, localIcon, fontSize: 20 }),
    remark: remark as string
  }

  return menu
}

/**
 * 获取需要 keep-alive 的末级路由名称。
 *
 * @param routes 两级 Vue 路由
 */
export function getCacheRouteNames(routes: RouteRecordRaw[]) {
  const cacheNames: LastLevelRouteKey[] = []

  routes.forEach(route => {
    // 只缓存末级且实际挂载组件的页面路由。
    route.children?.forEach(child => {
      if (child.component && child.meta?.keepAlive) {
        cacheNames.push(child.name as LastLevelRouteKey)
      }
    })
  })

  return cacheNames
}

/**
 * 判断路由树中是否存在指定路由名。
 */
export function isRouteExistByRouteName(routeName: RouteKey, routes: ElegantConstRoute[]) {
  return routes.some(route => recursiveGetIsRouteExistByRouteName(route, routeName))
}

/**
 * 递归判断单棵路由树中是否存在指定路由名。
 */
function recursiveGetIsRouteExistByRouteName(route: ElegantConstRoute, routeName: RouteKey) {
  let isExist = route.name === routeName

  if (isExist) {
    return true
  }

  if (route.children && route.children.length) {
    isExist = route.children.some(item => recursiveGetIsRouteExistByRouteName(item, routeName))
  }

  return isExist
}

/**
 * 获取选中菜单节点的完整 key 路径。
 */
export function getSelectedMenuKeyPathByKey(selectedKey: string, menus: App.Global.Menu[]) {
  const keyPath: string[] = []

  menus.some(menu => {
    const path = findMenuPath(selectedKey, menu)

    const find = Boolean(path?.length)

    if (find) {
      keyPath.push(...path!)
    }

    return find
  })

  return keyPath
}

/**
 * 在菜单树中查找目标节点路径。
 */
function findMenuPath(targetKey: string, menu: App.Global.Menu): string[] | null {
  const path: string[] = []

  function dfs(item: App.Global.Menu): boolean {
    path.push(item.key)

    if (item.key === targetKey) {
      return true
    }

    if (item.children) {
      for (const child of item.children) {
        if (dfs(child)) {
          return true
        }
      }
    }

    path.pop()

    return false
  }

  if (dfs(menu)) {
    return path
  }

  return null
}

/**
 * 将菜单节点转换为面包屑节点。
 */
function transformMenuToBreadcrumb(menu: App.Global.Menu) {
  const { children, ...rest } = menu

  const breadcrumb: App.Global.Breadcrumb = {
    ...rest
  }

  if (children?.length) {
    breadcrumb.options = children.map(transformMenuToBreadcrumb)
  }

  return breadcrumb
}

/**
 * 根据当前路由生成面包屑链路。
 */
export function getBreadcrumbsByRoute(
  route: RouteLocationNormalizedLoaded,
  menus: App.Global.Menu[]
): App.Global.Breadcrumb[] {
  const key = route.name as string
  const activeKey = route.meta?.activeMenu

  const menuKey = activeKey || key

  for (const menu of menus) {
    if (menu.key === menuKey) {
      const breadcrumbMenu = menuKey !== activeKey ? menu : getGlobalMenuByBaseRoute(route)

      return [transformMenuToBreadcrumb(breadcrumbMenu)]
    }

    if (menu.children?.length) {
      const result = getBreadcrumbsByRoute(route, menu.children)
      if (result.length > 0) {
        return [transformMenuToBreadcrumb(menu), ...result]
      }
    }
  }

  return []
}
